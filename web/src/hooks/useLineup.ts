import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { ApiError, patchJson, postJson } from "../api";
import type { Lineup, Player } from "../types";

interface Params {
	userId: string;
	leagueId: string;
	rosterId: number;
	weekNumber: number;
	players: Player[];
	initialStarters: Player[];
	slotCount: number;
	existingLineup: Lineup | null;
}

interface LineupState {
	seenWeek: number;
	lineupId: string | null;
	overrides: Record<number, Player>;
}

function buildOverrides(
	existingLineup: Lineup | null,
	players: Player[],
	initialStarters: Player[],
): Record<number, Player> {
	if (!existingLineup) {
		return {};
	}
	const playerMap = new Map(players.map((p) => [p.player_id, p]));
	return existingLineup.starters.reduce(
		(acc, id, i) => {
			const official = initialStarters[i];
			if (official?.player_id !== id) {
				const player = playerMap.get(id);
				if (player) {
					acc[i] = player;
				}
			}
			return acc;
		},
		{} as Record<number, Player>,
	);
}

export function useLineup({
	userId,
	leagueId,
	rosterId,
	weekNumber,
	players,
	initialStarters,
	slotCount,
	existingLineup,
}: Params) {
	const queryKey = ["lineups", userId, leagueId, weekNumber];
	const currentLineupId = existingLineup?.id ?? null;

	const [state, setState] = useState<LineupState>(() => ({
		seenWeek: weekNumber,
		lineupId: currentLineupId,
		overrides: buildOverrides(existingLineup, players, initialStarters),
	}));

	// getDerivedStateFromProps pattern — reset overrides when:
	// 1. The week changes (clear any pending picks)
	// 2. A saved lineup appears for the first time for this week (initialize from server)
	const weekChanged = state.seenWeek !== weekNumber;
	const lineupFirstAppeared = !weekChanged && state.lineupId === null && currentLineupId !== null;
	if (weekChanged || lineupFirstAppeared) {
		setState({
			seenWeek: weekNumber,
			lineupId: currentLineupId,
			overrides: buildOverrides(existingLineup, players, initialStarters),
		});
	}

	const queryClient = useQueryClient();

	const mutation = useMutation({
		mutationFn: (starters: string[]) => {
			if (state.lineupId === null) {
				return postJson<Lineup>("/lineups", {
					user_id: userId,
					league_id: leagueId,
					source: "sleeper",
					roster_id: rosterId,
					week_number: weekNumber,
					starters,
				});
			}
			return patchJson<Lineup>(`/lineups/${state.lineupId}`, {
				user_id: userId,
				starters,
			});
		},
		onSuccess: (data) => {
			if (state.lineupId === null) {
				// Setting lineupId immediately prevents a refetch from triggering
				// lineupFirstAppeared and overwriting picks made before the query lands.
				setState((prev) => ({ ...prev, lineupId: data.id }));
			}
			queryClient.setQueryData(queryKey, (old: Lineup[] | undefined) =>
				old ? old.map((l) => (l.id === data.id ? data : l)) : [data],
			);
		},
		onError: (error) => {
			// Defensive: the UI disables editing once locked, but if a save races past
			// kickoff the server returns 409. Refetch the week so its lock state flips
			// to true and the editor disables itself.
			if (error instanceof ApiError && error.status === 409) {
				queryClient.invalidateQueries({ queryKey: ["week-matchups", leagueId, weekNumber] });
			}
		},
	});

	function save(next: Record<number, Player>) {
		const merged = Array.from(
			{ length: slotCount },
			(_, i) => next[i] ?? initialStarters[i] ?? null,
		);
		if (merged.some((p) => p === null)) {
			return;
		}
		mutation.mutate((merged as Player[]).map((p) => p.player_id));
	}

	function applyOverride(index: number, player: Player | null) {
		const officialId = initialStarters[index]?.player_id;
		let next: Record<number, Player>;
		if (!player || player.player_id === officialId) {
			next = { ...state.overrides };
			delete next[index];
		} else {
			next = { ...state.overrides, [index]: player };
		}
		setState((prev) => ({ ...prev, overrides: next }));
		save(next);
	}

	return {
		overrides: state.overrides,
		applyOverride,
		saveStatus: mutation.isPending
			? "saving"
			: mutation.isError
				? "error"
				: mutation.isSuccess
					? "saved"
					: "idle",
	} as const;
}
