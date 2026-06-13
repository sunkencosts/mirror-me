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

export default function MembersPage() {
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

	return (
		<div className="fade-in">
			<div className="panel">
				<h4>League members</h4>
				<div className="sub">{rosters.length} managers</div>
				<div
					className="lg-cards"
					style={{ gridTemplateColumns: "repeat(auto-fill,minmax(240px,1fr))" }}
				>
					{rosters.map((roster, i) => {
						const name = roster.team_name || `Team ${roster.roster_id}`;
						const power = scores[i];
						const tier = getTierRelative(power, scores);
						return (
							<div key={roster.roster_id} className="lg-card" style={{ gap: 10 }}>
								<div className="top">
									<div
										className={`pav ${TIER_RING[tier]}`}
										style={{ background: avatarBg(name), width: 40, height: 40, fontSize: 14 }}
									>
										{initials(name)}
									</div>
									<div style={{ flex: 1, minWidth: 0 }}>
										<div
											className="nm"
											style={{
												fontSize: 14,
												whiteSpace: "nowrap",
												overflow: "hidden",
												textOverflow: "ellipsis",
											}}
										>
											{name}
										</div>
									</div>
								</div>
								<div className="row2">
									<span className="tag green">Power {power.toFixed(1)}</span>
									<Link to={`/${leagueId}/lineups`} className="go">
										Mirror <Icon name="chevR" />
									</Link>
								</div>
							</div>
						);
					})}
				</div>
			</div>
		</div>
	);
}
