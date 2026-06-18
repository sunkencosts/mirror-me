import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams, useSearchParams } from "react-router";
import { bookmarksKey, fetchJson, patchJson } from "../api";
import { Icon } from "../components/icons";
import LeagueSummary from "../components/LeagueSummary";
import PlayerSearch from "../components/PlayerSearch";
import RosterCard from "../components/RosterCard";
import { useAuth } from "../context/AuthContext";
import { useDismissed } from "../hooks/useDismissed";
import { computePowerScore } from "../scoring";
import type {
	League,
	LeagueBookmark,
	LeagueConfig,
	Lineup,
	Roster,
	WeekMatchupsResponse,
} from "../types";
import styles from "./LineupsPage.module.css";

export default function LineupsPage() {
	const { leagueId = "" } = useParams();
	const [searchParams, setSearchParams] = useSearchParams();
	const week = searchParams.get("week");
	const weekNumber = week ? parseInt(week, 10) : 1;
	const { userId } = useAuth();
	const queryClient = useQueryClient();

	const leagueQueryKey = ["league", leagueId] as const;

	const { data: league, isLoading: leagueLoading } = useQuery<League>({
		queryKey: leagueQueryKey,
		queryFn: () => fetchJson(`/league/${leagueId}`),
		enabled: !!leagueId,
		throwOnError: true,
	});

	const { data: rosters = [], isLoading: rostersLoading } = useQuery<Roster[]>({
		queryKey: ["rosters", leagueId],
		queryFn: () => fetchJson(`/league/${leagueId}/rosters`),
		select: (data) => data ?? [],
		enabled: !!leagueId,
		throwOnError: true,
	});

	const { data: weekData, isLoading: weekMatchupsLoading } = useQuery<WeekMatchupsResponse>({
		queryKey: ["week-matchups", leagueId, weekNumber],
		queryFn: () => fetchJson(`/league/${leagueId}/week/${weekNumber}`),
		placeholderData: keepPreviousData,
		enabled: !!leagueId,
		retry: false,
	});

	const weekMatchups = weekData?.matchups ?? [];
	const weekLocked = weekData?.locked ?? false;

	const matchupByRosterId = useMemo(
		() => new Map(weekMatchups.map((m) => [m.roster_id, m])),
		[weekMatchups],
	);

	const { data: lineups = [] } = useQuery<Lineup[]>({
		queryKey: ["lineups", userId, leagueId, weekNumber],
		queryFn: () =>
			fetchJson(`/lineups?user_id=${userId}&league_id=${leagueId}&week_number=${weekNumber}`),
		enabled: !!leagueId,
	});

	const { data: bookmarks = [] } = useQuery<LeagueBookmark[]>({
		queryKey: bookmarksKey(userId),
		queryFn: () => fetchJson(`/league-bookmarks?user_id=${userId}`),
	});

	const { mutate: patchLabel } = useMutation({
		mutationFn: ({ label, source }: { label: string; source: string }) =>
			patchJson<LeagueBookmark>(`/league-bookmarks/${leagueId}?source=${source}`, {
				user_id: userId,
				label,
			}),
		onSuccess: () => queryClient.invalidateQueries({ queryKey: bookmarksKey(userId) }),
	});

	const patched = useRef(false);
	const highlightTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
	useEffect(() => () => clearTimeout(highlightTimer.current), []);

	useEffect(() => {
		if (patched.current || !league?.name) {
			return;
		}
		const bookmark = bookmarks.find((b) => b.league_id === leagueId);
		if (!bookmark || bookmark.label) {
			return;
		}
		patched.current = true;
		patchLabel({ label: league.name, source: bookmark.source });
	}, [league, bookmarks, leagueId, patchLabel]);

	const leagueConfig = useMemo<LeagueConfig | null>(() => {
		if (!league) {
			return null;
		}
		const starterSlots = league.roster_positions.filter((p) => p !== "BN");
		return {
			starterSlots,
			irSlots: league.settings.reserve_slots,
			taxiSlots: league.settings.taxi_slots,
		};
	}, [league]);

	const allScores = useMemo(
		() => rosters.map((r) => computePowerScore(r.players, r.starters)),
		[rosters],
	);

	const [teamMenuOpen, setTeamMenuOpen] = useState(false);

	const [dismissedToEnd, setDismissedToEnd] = useDismissed(leagueId);

	const dismissedSet = useMemo(() => new Set(dismissedToEnd), [dismissedToEnd]);

	const activeRosters = useMemo(
		() => rosters.filter((r) => !dismissedSet.has(r.roster_id)),
		[rosters, dismissedSet],
	);

	const rosterById = useMemo(() => new Map(rosters.map((r) => [r.roster_id, r])), [rosters]);

	const dismissedRosters = useMemo(
		() =>
			dismissedToEnd.map((id) => rosterById.get(id)).filter((r): r is Roster => r !== undefined),
		[dismissedToEnd, rosterById],
	);

	function handleToggleDismiss(rosterId: number) {
		setDismissedToEnd((prev) =>
			prev.includes(rosterId) ? prev.filter((id) => id !== rosterId) : [...prev, rosterId],
		);
	}

	const scrollToRoster = useCallback((rosterId: number) => {
		const el = document.getElementById(`roster-${rosterId}`);
		if (!el) {
			return;
		}
		el.scrollIntoView({ behavior: "smooth", block: "start" });
		el.classList.add(styles.highlighted);
		clearTimeout(highlightTimer.current);
		highlightTimer.current = setTimeout(() => el.classList.remove(styles.highlighted), 1500);
	}, []);

	const focusRosterId = searchParams.get("roster");
	const focusedRosterRef = useRef<string | null>(null);

	useEffect(() => {
		if (!focusRosterId || focusedRosterRef.current === focusRosterId) {
			return;
		}
		if (leagueLoading || rostersLoading || weekMatchupsLoading) {
			return;
		}
		const el = document.getElementById(`roster-${focusRosterId}`);
		if (!el) {
			return;
		}
		focusedRosterRef.current = focusRosterId;
		scrollToRoster(Number(focusRosterId));
		// Strip the param via the history API directly (not setSearchParams) so this
		// doesn't register as a navigation and trigger ScrollRestoration back to top.
		const url = new URL(window.location.href);
		url.searchParams.delete("roster");
		window.history.replaceState(null, "", url);
	}, [focusRosterId, leagueLoading, rostersLoading, weekMatchupsLoading, scrollToRoster]);

	if (leagueLoading || rostersLoading || weekMatchupsLoading) {
		return <p>Loading…</p>;
	}
	if (!leagueConfig || !league) {
		return null;
	}

	function renderCard(roster: Roster, isDismissed: boolean) {
		return (
			<div
				key={roster.roster_id}
				id={`roster-${roster.roster_id}`}
				className={styles.rosterWrapper}
			>
				<RosterCard
					roster={roster}
					weekMatchup={matchupByRosterId.get(roster.roster_id) ?? null}
					{...(leagueConfig as LeagueConfig)}
					allScores={allScores}
					leagueId={leagueId}
					userId={userId}
					lineups={lineups}
					weekNumber={weekNumber}
					currentWeek={league!.settings.leg}
					locked={weekLocked}
					isDismissed={isDismissed}
					onToggleDismiss={() => handleToggleDismiss(roster.roster_id)}
				/>
			</div>
		);
	}

	return (
		<div className="fade-in">
			<LeagueSummary
				league={league}
				weekNumber={weekNumber}
				onWeekChange={(w) => setSearchParams({ week: String(w) })}
			/>
			<div className="lineup-toolbar">
				<PlayerSearch rosters={rosters} onScrollToRoster={scrollToRoster} />
				<div className="select">
					<button
						type="button"
						className="select-trigger"
						aria-label="Jump to team"
						aria-expanded={teamMenuOpen}
						onClick={() => setTeamMenuOpen((open) => !open)}
					>
						<Icon name="users" />
						<span>Jump to team…</span>
						<Icon name="chevDown" />
					</button>
					{teamMenuOpen && (
						<>
							<button
								type="button"
								aria-label="Close"
								className="pop-backdrop"
								onMouseDown={() => setTeamMenuOpen(false)}
							/>
							<div className="pop team-pop">
								<div className="pop-list">
									{[...activeRosters, ...dismissedRosters].map((r) => (
										<button
											key={r.roster_id}
											type="button"
											className="pop-item"
											onClick={() => {
												scrollToRoster(r.roster_id);
												setTeamMenuOpen(false);
											}}
										>
											<span className="pname">{r.team_name || `Team ${r.roster_id}`}</span>
										</button>
									))}
								</div>
							</div>
						</>
					)}
				</div>
			</div>
			<div className="teams-grid">{activeRosters.map((roster) => renderCard(roster, false))}</div>
			{dismissedRosters.length > 0 && (
				<div className="teams-grid dismissed-grid">
					{dismissedRosters.map((roster) => renderCard(roster, true))}
				</div>
			)}
		</div>
	);
}
