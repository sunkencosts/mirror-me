import { useQuery } from "@tanstack/react-query";
import { Fragment, useState } from "react";
import { fetchJson } from "../../api";
import { useAuth } from "../../context/AuthContext";
import type {
	CompareResponse,
	League,
	Lineup,
	Roster,
	WeeklyRosterResults,
	WeeklySetterResult,
} from "../../types";
import { ordinal } from "../../utils/leaderboardFormat";
import { Icon } from "../icons";
import { Monogram } from "../LeaderboardTable";
import TeamSelect from "../TeamSelect";
import WeekSelector from "../WeekSelector";
import SetterLineupDetail from "./SetterLineupDetail";

const SEASON = "2026";
const RESULT_LABEL: Record<string, string> = { user: "Won", official: "Lost", tie: "Tie" };

// Points margin over the manager, e.g. "+12.4" / "-3.1".
function formatMargin(margin: number): string {
	return `${margin >= 0 ? "+" : ""}${margin.toFixed(1)}`;
}

export default function WeeklyResults({ leagueId }: { leagueId: string }) {
	const { userId, user } = useAuth();
	const isLoggedIn = !!user;

	const { data: league } = useQuery<League>({
		queryKey: ["league", leagueId],
		queryFn: () => fetchJson(`/league/${leagueId}`),
		enabled: !!leagueId,
	});
	const { data: rosters = [] } = useQuery<Roster[]>({
		queryKey: ["rosters", leagueId],
		queryFn: () => fetchJson(`/league/${leagueId}/rosters`),
		enabled: !!leagueId,
	});

	// Only past weeks are graded.
	const currentWeek = league?.settings.leg ?? 1;
	const latestGraded = Math.max(1, currentWeek - 1);

	// Every lineup the signed-in user set in this league (all weeks, newest first). Used to land
	// them on the week/team they actually played and to score that lineup live.
	const { data: myLineups = [] } = useQuery<Lineup[]>({
		queryKey: ["my-lineups-all", leagueId, userId],
		queryFn: () => fetchJson(`/lineups?user_id=${userId}&league_id=${leagueId}`),
		enabled: !!leagueId && !!userId,
	});
	const myPlayedWeeks = myLineups
		.map((l) => l.week_number)
		.filter((w) => w >= 1 && w <= latestGraded);
	const myLatestPlayedWeek = myPlayedWeeks.length ? Math.max(...myPlayedWeeks) : null;

	const [weekOverride, setWeekOverride] = useState<number | null>(null);
	const week = weekOverride ?? myLatestPlayedWeek ?? latestGraded;

	const myRosterForWeek = myLineups.find((l) => l.week_number === week)?.roster_id ?? null;
	const [rosterOverride, setRosterOverride] = useState<number | null>(null);
	const activeRosterId = rosterOverride ?? myRosterForWeek ?? rosters[0]?.roster_id ?? null;

	const [query, setQuery] = useState("");
	const [expanded, setExpanded] = useState<string | null>(null);

	const { data: results, isLoading } = useQuery<WeeklyRosterResults>({
		queryKey: ["weekly-results", leagueId, week, activeRosterId, query],
		queryFn: () =>
			fetchJson(
				`/league/${leagueId}/week/${week}/results?roster_id=${activeRosterId}&season=${SEASON}&q=${encodeURIComponent(query)}`,
			),
		enabled: !!leagueId && activeRosterId !== null,
	});

	// Score the signed-in user's own lineup live (same path as the Lineups page) so it appears the
	// instant they set it — before the batch grader writes week_results. A 404 (they didn't set a
	// lineup for this team/week) just means "no personal result", so don't retry.
	const { data: myResult } = useQuery<CompareResponse>({
		queryKey: ["my-week-result", leagueId, week, activeRosterId, userId],
		queryFn: () =>
			fetchJson(
				`/league/${leagueId}/week/${week}/roster/${activeRosterId}/lineup?user_id=${userId}`,
			),
		enabled: isLoggedIn && !!leagueId && activeRosterId !== null,
		retry: false,
	});

	const roster = rosters.find((r) => r.roster_id === activeRosterId);
	const teamName = roster?.team_name || (activeRosterId ? `Team ${activeRosterId}` : "");

	const gradedSetters = results?.setters ?? [];
	const gradedMe = gradedSetters.find((setter) => setter.user_id === userId);
	// Inject the user's live result into the field only when they have one, aren't already graded,
	// and aren't searching (search filters the field server-side, so injecting would be confusing).
	const injectLiveMe = isLoggedIn && !!myResult && !gradedMe && query.trim() === "";
	const liveMeRow: WeeklySetterResult | null =
		injectLiveMe && myResult
			? {
					user_id: userId,
					username: user?.username ?? "you",
					user_total: myResult.user.total_points,
					efficiency: myResult.user_efficiency,
					edge: myResult.edge,
					result: myResult.winner,
					rank: 0,
				}
			: null;

	// The ranked field: the graded setters plus the live "you" row, re-ranked by total points.
	const displaySetters: WeeklySetterResult[] = liveMeRow
		? [...gradedSetters, liveMeRow]
				.sort((a, b) => b.user_total - a.user_total || (a.user_id < b.user_id ? -1 : 1))
				.map((setter, index) => ({ ...setter, rank: index + 1 }))
		: gradedSetters;

	// Baseline (manager/optimal) comes from the graded field; if nobody else is graded for this
	// roster-week, fall back to the user's own live compare so a solo setter still sees a baseline.
	const hasField = !!results && results.setter_count > 0;
	const officialTotal = hasField ? results.official_total : (myResult?.official.total_points ?? 0);
	const optimalTotal = hasField ? results.optimal_total : (myResult?.optimal_points ?? 0);

	const totalLineups = (hasField ? results.setter_count : 0) + (injectLiveMe ? 1 : 0);
	const beatCount =
		(hasField ? results.beat_official_count : 0) +
		(injectLiveMe && myResult?.winner === "user" ? 1 : 0);
	const beatPct = totalLineups ? Math.round((beatCount / totalLineups) * 100) : 0;
	const youRow = displaySetters.find((setter) => setter.user_id === userId);

	function toggleExpanded(setter: WeeklySetterResult) {
		setExpanded((current) => (current === setter.user_id ? null : setter.user_id));
	}

	return (
		<div className="wr">
			<div className="wr-controls">
				<TeamSelect
					rosters={rosters}
					value={activeRosterId}
					onSelect={(rosterId) => {
						setRosterOverride(rosterId);
						setExpanded(null);
					}}
					placeholder="Team"
					ariaLabel="Team"
				/>
				<WeekSelector
					weekNumber={week}
					onWeekChange={(w) => {
						setWeekOverride(w);
						setExpanded(null);
					}}
					max={latestGraded}
				/>
				<label className="wr-search">
					<Icon name="search" />
					<input
						type="text"
						placeholder="Find a manager"
						value={query}
						onChange={(e) => setQuery(e.target.value)}
					/>
				</label>
			</div>

			{isLoading ? (
				<p>Loading…</p>
			) : !hasField && !injectLiveMe ? (
				<div className="panel">
					<div className="lb-empty">
						No one mirrored {teamName} in week {week} yet.
					</div>
				</div>
			) : (
				<>
					<div className="wr-headline">
						{!isLoggedIn ? (
							<p className="wr-you wr-you-empty">
								Log in to see how your lineup ranked for week {week}.
							</p>
						) : youRow ? (
							<p className="wr-you">
								You set the <strong>{ordinal(youRow.rank)}</strong> best lineup of {totalLineups}{" "}
								for {teamName} ·{" "}
								<span
									className={`lb-edge ${youRow.user_total - officialTotal >= 0 ? "pos" : "neg"}`}
								>
									{formatMargin(youRow.user_total - officialTotal)}
								</span>{" "}
								vs the manager
							</p>
						) : (
							<p className="wr-you wr-you-empty">
								You didn't set a lineup for {teamName} in week {week}.
							</p>
						)}
						<p className="wr-field">
							<strong>{beatPct}%</strong> of lineups scored higher than the original owner.
						</p>
					</div>

					<div className="kpi-row wr-baseline">
						<div className="kpi">
							<div className="l">Manager (official)</div>
							<div className="v">{officialTotal.toFixed(1)}</div>
							<div className="d">official lineup</div>
						</div>
						<div className="kpi">
							<div className="l">Best possible</div>
							<div className="v">{optimalTotal.toFixed(1)}</div>
							<div className="d">optimal lineup</div>
						</div>
						<div className="kpi">
							<div className="l">Mirrored by</div>
							<div className="v">{totalLineups}</div>
							<div className="d">{totalLineups === 1 ? "manager" : "managers"}</div>
						</div>
					</div>

					<div className="panel">
						<div className="table-wrap">
							<table className="rk-table">
								<thead>
									<tr>
										<th>Rank</th>
										<th>Manager</th>
										<th className="num-col">Points</th>
										<th className="num-col">vs Owner</th>
										<th className="num-col">Result</th>
										<th aria-label="Expand" />
									</tr>
								</thead>
								<tbody>
									{displaySetters.map((setter) => {
										const isYou = setter.user_id === userId;
										const isOpen = expanded === setter.user_id;
										return (
											<Fragment key={setter.user_id}>
												<tr
													className={`wr-row${isYou ? " you" : ""}`}
													onClick={() => toggleExpanded(setter)}
												>
													<td className="nm">#{setter.rank}</td>
													<td>
														<div className="lb-who">
															<Monogram username={setter.username} />
															<span className="nm">{setter.username}</span>
															{isYou && <span className="you-tag">you</span>}
														</div>
													</td>
													<td className="num-col">{setter.user_total.toFixed(1)}</td>
													<td className="num-col">
														<span
															className={`lb-edge ${setter.user_total - officialTotal >= 0 ? "pos" : "neg"}`}
														>
															{formatMargin(setter.user_total - officialTotal)}
														</span>
													</td>
													<td className="num-col">
														{RESULT_LABEL[setter.result] ?? setter.result}
													</td>
													<td className="num-col">
														<Icon name={isOpen ? "chevDown" : "chevR"} />
													</td>
												</tr>
												{isOpen && activeRosterId !== null && (
													<tr className="wr-detail-row">
														<td colSpan={6}>
															<SetterLineupDetail
																leagueId={leagueId}
																week={week}
																rosterId={activeRosterId}
																userId={setter.user_id}
																username={setter.username}
															/>
														</td>
													</tr>
												)}
											</Fragment>
										);
									})}
								</tbody>
							</table>
						</div>
					</div>
				</>
			)}
		</div>
	);
}
