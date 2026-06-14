import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router";
import { fetchJson } from "../api";
import { Icon } from "../components/icons";
import { computePowerScore, getTierRelative, type Tier } from "../scoring";
import type { Roster } from "../types";
import { avatarBg, initials } from "../utils/avatar";

const TIER_RING: Record<Tier, string> = {
	S: "r-myt",
	A: "r-leg",
	B: "r-epic",
	C: "r-rare",
	D: "r-unc",
};

export default function TeamsPage() {
	const { leagueId = "" } = useParams();

	const { data: rosters = [], isLoading } = useQuery<Roster[]>({
		queryKey: ["rosters", leagueId],
		queryFn: () => fetchJson(`/league/${leagueId}/rosters`),
		select: (data) => data ?? [],
		enabled: !!leagueId,
		throwOnError: true,
	});

	if (isLoading) {
		return <p>Loading…</p>;
	}

	const scores = rosters.map((r) => computePowerScore(r.players, r.starters));

	const sortedRosters = rosters
		.map((roster, i) => ({ roster, power: scores[i] }))
		.sort((a, b) => b.power - a.power);

	return (
		<div className="fade-in">
			<div className="panel">
				<h4>Teams</h4>
				<div className="sub">{rosters.length} managers</div>
				<div className="lg-cards">
					{sortedRosters.map(({ roster, power }) => {
						const name = roster.team_name || `Team ${roster.roster_id}`;
						const tier = getTierRelative(power, scores);
						return (
							<Link
								key={roster.roster_id}
								to={`/${leagueId}/lineups?roster=${roster.roster_id}`}
								className="lg-card"
								style={{ padding: 12, gap: 8 }}
							>
								<div className="top">
									<div
										className={`pav ${TIER_RING[tier]}`}
										style={{ background: avatarBg(name), width: 32, height: 32, fontSize: 12 }}
									>
										{initials(name)}
									</div>
									<div style={{ flex: 1, minWidth: 0 }}>
										<div
											className="nm"
											style={{
												fontSize: 13,
												whiteSpace: "nowrap",
												overflow: "hidden",
												textOverflow: "ellipsis",
											}}
										>
											{name}
										</div>
									</div>
									<span className="go" aria-hidden="true">
										<Icon name="chevR" />
									</span>
								</div>
								<div className="power-top" style={{ margin: 0 }}>
									<span className={`tier-badge tier-${tier}`}>{tier}-TIER</span>
									<span className="power-num">
										ROSTER POWER<b>{power.toFixed(1)}</b>/10
									</span>
								</div>
							</Link>
						);
					})}
				</div>
			</div>
		</div>
	);
}
