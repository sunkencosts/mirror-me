import { useEffect, useMemo, useState } from "react";
import { type RookiePosition, rookieRankings2026 } from "../../data/rookieRankings2026";
import type { ScoringMode } from "./RankingsHero";

interface RankingsListProps {
	scoringMode: ScoringMode;
	posFilter: "ALL" | RookiePosition;
	onPosFilterChange: (pos: "ALL" | RookiePosition) => void;
}

const POSITIONS: ("ALL" | RookiePosition)[] = ["ALL", "QB", "RB", "WR", "TE"];

const TOP_N = 25;

const DRAFTED_STORAGE_KEY = "rookieRankings2026:drafted";

function loadDrafted(): Set<string> {
	try {
		const raw = localStorage.getItem(DRAFTED_STORAGE_KEY);
		return raw ? new Set(JSON.parse(raw)) : new Set();
	} catch {
		return new Set();
	}
}

export default function RankingsList({ scoringMode, posFilter, onPosFilterChange }: RankingsListProps) {
	const usePpr = scoringMode === "ppr";
	const [drafted, setDrafted] = useState<Set<string>>(loadDrafted);

	useEffect(() => {
		localStorage.setItem(DRAFTED_STORAGE_KEY, JSON.stringify([...drafted]));
	}, [drafted]);

	function toggleDrafted(player: string) {
		setDrafted((prev) => {
			const next = new Set(prev);
			if (next.has(player)) {
				next.delete(player);
			} else {
				next.add(player);
			}
			return next;
		});
	}

	const filtered = useMemo(
		() =>
			rookieRankings2026
				.filter((r) => r.model === "with_pick")
				.filter((r) => posFilter === "ALL" || r.position === posFilter)
				.map((r) => ({
					...r,
					points: usePpr && r.pointsPpr !== null ? r.pointsPpr : r.points,
					positionRank: usePpr && r.positionRankPpr !== null ? r.positionRankPpr : r.positionRank,
				}))
				.sort((a, b) => b.points - a.points)
				.slice(0, TOP_N),
		[posFilter, usePpr],
	);

	const subtitle = `Post-draft · with-pick model · draft slot included as feature · sorted by projected ${usePpr ? "PPR " : ""}points`;

	return (
		<>
			<div
				className="filters"
				style={{ display: "flex", gap: 8, marginBottom: 16, flexWrap: "wrap" }}
			>
				{POSITIONS.map((pos) => (
					<button
						key={pos}
						type="button"
						className={`fchip${posFilter === pos ? " active" : ""}`}
						onClick={() => onPosFilterChange(pos)}
					>
						{pos}
					</button>
				))}
				<span className="mini" style={{ marginLeft: "auto", alignSelf: "center" }}>
					{filtered.length} prospect{filtered.length !== 1 ? "s" : ""}
				</span>
			</div>

			<div className="panel" style={{ marginBottom: "var(--gap)" }}>
				<h4 style={{ marginBottom: 4 }}>2026 Rookie Projections</h4>
				<div className="sub" style={{ marginBottom: 20 }}>
					{subtitle}
				</div>

				{filtered.length === 0 ? (
					<div style={{ textAlign: "center", padding: "40px 20px" }}>
						<span className="mini">No prospects match the current filter.</span>
					</div>
				) : (
					<div className="table-wrap">
						<table className="rk-table">
							<thead>
								<tr>
									<th />
									<th>#</th>
									<th>Player</th>
									<th>Pos</th>
									<th>Team</th>
									<th>Conference</th>
									<th>Rank</th>
									<th className="num-col">Proj pts{usePpr ? " (PPR)" : ""}</th>
								</tr>
							</thead>
							<tbody>
								{filtered.map((r, i) => {
									const rank = i + 1;
									const isDrafted = drafted.has(r.player);
									return (
										<tr key={r.player} className={isDrafted ? "drafted" : ""}>
											<td className="check-col">
												<input
													type="checkbox"
													checked={isDrafted}
													onChange={() => toggleDrafted(r.player)}
												/>
											</td>
											<td className="tnum">{rank}</td>
											<td className="nm">{r.player}</td>
											<td>
												<span className="pos">{r.position}</span>
											</td>
											<td>{r.team || "\u2014"}</td>
											<td>{r.conference}</td>
											<td className="tnum">
												{r.position}
												{r.positionRank}
											</td>
											<td className="num-col tnum">{r.points.toFixed(1)}</td>
										</tr>
									);
								})}
							</tbody>
						</table>
					</div>
				)}
			</div>
		</>
	);
}
