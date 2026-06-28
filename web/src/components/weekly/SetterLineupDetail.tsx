import { useQuery } from "@tanstack/react-query";
import { fetchJson } from "../../api";
import { slotPosClass } from "../../slots";
import type { CompareResponse, ScoredPlayer } from "../../types";
import PlayerCard from "../PlayerCard";

interface Props {
	leagueId: string;
	week: number;
	rosterId: number;
	userId: string;
	username: string;
}

// A starter who left the roster before kickoff comes back with no name and 0 points.
function isDeparted(player: ScoredPlayer): boolean {
	return !player.first_name && !player.last_name;
}

function EmptySlot() {
	return (
		<div className="empty-slot">
			<div className="ring" />
			<span>Off roster</span>
		</div>
	);
}

// One slot: position chip on the far left, the manager's official starter, the points delta in
// the middle, then the setter's pick on the right.
function StarterRow({ official, pick }: { official: ScoredPlayer; pick: ScoredPlayer }) {
	const changed = official.player_id !== pick.player_id;
	const pos = pick.fantasy_positions[0] ?? official.fantasy_positions[0] ?? "";
	const delta = pick.points - official.points;

	return (
		<div className="sd-row">
			<span className={`pos ${slotPosClass(pos)}`}>{pos}</span>

			{isDeparted(official) ? (
				<EmptySlot />
			) : (
				<PlayerCard player={official} points={official.points} dimmed={changed} />
			)}

			<div className="sd-delta">
				{changed && (
					<span className={`delta ${delta >= 0 ? "pos" : "neg"} tnum`}>
						{delta >= 0 ? "+" : ""}
						{delta.toFixed(1)}
					</span>
				)}
			</div>

			<div className="sd-pick">
				{isDeparted(pick) ? (
					<EmptySlot />
				) : (
					<PlayerCard player={pick} reversed points={pick.points} />
				)}
			</div>
		</div>
	);
}

// SetterLineupDetail lazily fetches one setter's scored lineup vs the official lineup (the
// public per-setter endpoint) — rendered only when a row is expanded, so a 100-setter list
// never loads 100 lineups up front.
export default function SetterLineupDetail({ leagueId, week, rosterId, userId, username }: Props) {
	const { data, isLoading, isError } = useQuery<CompareResponse>({
		queryKey: ["setter-lineup", leagueId, week, rosterId, userId],
		queryFn: () =>
			fetchJson(`/league/${leagueId}/week/${week}/roster/${rosterId}/score?user_id=${userId}`),
	});

	if (isLoading) {
		return <div className="setter-detail mini">Loading lineup…</div>;
	}
	if (isError || !data) {
		return <div className="setter-detail mini">Couldn't load this lineup.</div>;
	}

	return (
		<div className="setter-detail">
			<div className="roster-head">
				<span className="o">
					Manager started · <b>{data.official.total_points.toFixed(1)}</b>
				</span>
				<span className="y">
					{username}'s picks · <b>{data.user.total_points.toFixed(1)}</b>
				</span>
			</div>
			{data.user.starters.map((pick, i) => (
				<StarterRow
					// biome-ignore lint/suspicious/noArrayIndexKey: lineup slots are a fixed ordered list (departed starters share an empty id)
					key={`${pick.player_id}-${i}`}
					official={data.official.starters[i]}
					pick={pick}
				/>
			))}
		</div>
	);
}
