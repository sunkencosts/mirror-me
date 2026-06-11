import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef } from "react";
import { useNavigate, useParams } from "react-router";
import { bookmarksKey, fetchJson, patchJson } from "../api";
import LeagueSummary from "../components/LeagueSummary";
import PlayerSearch from "../components/PlayerSearch";
import RosterCard from "../components/RosterCard";
import { useAuth } from "../context/AuthContext";
import { useDismissed } from "../hooks/useDismissed";
import { computePowerScore } from "../scoring";
import type { League, LeagueBookmark, LeagueConfig, Lineup, Roster, WeekMatchup } from "../types";
import styles from "./LeaguePage.module.css";

export default function LeaguePage() {
	const { leagueId = "", week } = useParams();
	const weekNumber = week ? parseInt(week, 10) : 1;
	const navigate = useNavigate();
	const { userId } = useAuth();
	const queryClient = useQueryClient();

	const leagueQueryKey = ["league", leagueId] as const;

	const {
		data: league,
		isLoading: leagueLoading,
	} = useQuery<League>({
		queryKey: leagueQueryKey,
		queryFn: () => fetchJson(`/league/${leagueId}`),
		enabled: !!leagueId,
		throwOnError: true,
	});

	const {
		data: rosters = [],
		isLoading: rostersLoading,
	} = useQuery<Roster[]>({
		queryKey: ["rosters", leagueId],
		queryFn: () => fetchJson(`/league/${leagueId}/rosters`),
		select: (data) => data ?? [],
		enabled: !!leagueId,
		throwOnError: true,
	});

	const {
		data: weekMatchups = [],
		isLoading: weekMatchupsLoading,
	} = useQuery<WeekMatchup[]>({
		queryKey: ["week-matchups", leagueId, weekNumber],
		queryFn: () => fetchJson(`/league/${leagueId}/week/${weekNumber}`),
		select: (data) => data ?? [],
		placeholderData: keepPreviousData,
		enabled: !!leagueId,
		retry: false,
	});

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
			benchSlots: league.roster_positions.length - starterSlots.length,
			irSlots: league.settings.reserve_slots,
			taxiSlots: league.settings.taxi_slots,
		};
	}, [league]);

	const allScores = useMemo(
		() => rosters.map((r) => computePowerScore(r.players, r.starters)),
		[rosters],
	);

	const [dismissedToEnd, setDismissedToEnd] = useDismissed(leagueId);

	const dismissedSet = useMemo(() => new Set(dismissedToEnd), [dismissedToEnd]);

	const activeRosters = useMemo(
		() => rosters.filter((r) => !dismissedSet.has(r.roster_id)),
		[rosters, dismissedSet],
	);

	const rosterById = useMemo(
		() => new Map(rosters.map((r) => [r.roster_id, r])),
		[rosters],
	);

	const dismissedRosters = useMemo(
		() => dismissedToEnd.map((id) => rosterById.get(id)).filter((r): r is Roster => r !== undefined),
		[dismissedToEnd, rosterById],
	);

	function handleToggleDismiss(rosterId: number) {
		setDismissedToEnd((prev) =>
			prev.includes(rosterId) ? prev.filter((id) => id !== rosterId) : [...prev, rosterId],
		);
	}

	if (leagueLoading || rostersLoading || weekMatchupsLoading) {
		return <p>Loading…</p>;
	}
	if (!leagueConfig || !league) {
		return null;
	}

	function renderCard(roster: Roster, isDismissed: boolean) {
		return (
			<div key={roster.roster_id} id={`roster-${roster.roster_id}`} className={styles.rosterWrapper}>
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
					isDismissed={isDismissed}
					onToggleDismiss={() => handleToggleDismiss(roster.roster_id)}
				/>
			</div>
		);
	}

	function scrollToRoster(rosterId: number) {
		const el = document.getElementById(`roster-${rosterId}`);
		if (!el) return;
		el.scrollIntoView({ behavior: "smooth", block: "start" });
		el.classList.add(styles.highlighted);
		clearTimeout(highlightTimer.current);
		highlightTimer.current = setTimeout(() => el.classList.remove(styles.highlighted), 1500);
	}

	return (
		<>
			<LeagueSummary
				league={league}
				weekNumber={weekNumber}
				onWeekChange={(w) => navigate(`/league/${leagueId}/week/${w}`)}
			/>
			<div className={styles.controls}>
				<PlayerSearch rosters={rosters} onScrollToRoster={scrollToRoster} />
				<select
					className={styles.teamSelect}
					value=""
					onChange={(e) => {
						const id = Number(e.target.value);
						if (id) scrollToRoster(id);
					}}
				>
					<option value="" disabled>Jump to team…</option>
					{[...activeRosters, ...dismissedRosters].map((r) => (
						<option key={r.roster_id} value={r.roster_id}>
							{r.team_name || `Team ${r.roster_id}`}
						</option>
					))}
				</select>
			</div>
			<div className={styles.rosterList}>
				{activeRosters.map((roster) => renderCard(roster, false))}
			</div>
			{dismissedRosters.length > 0 && (
				<div className={styles.dismissedList}>
					{dismissedRosters.map((roster) => renderCard(roster, true))}
				</div>
			)}
		</>
	);
}
