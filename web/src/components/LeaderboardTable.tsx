import type { LeaderboardRow } from "../types";
import { avatarBg, initials } from "../utils/avatar";
import {
	formatEdge,
	formatEfficiency,
	formatWinRate,
	scorebarWidth,
} from "../utils/leaderboardFormat";

export function Monogram({ username }: { username: string }) {
	return (
		<div
			className="pav r-dim"
			style={{ background: avatarBg(username), width: 28, height: 28, fontSize: 11 }}
		>
			{initials(username)}
		</div>
	);
}

function PodiumCard({ row, place }: { row: LeaderboardRow; place: number }) {
	const placeLabels: Record<number, string> = { 1: "1st", 2: "2nd", 3: "3rd" };
	return (
		<div className={`pod${place === 1 ? " first" : ""}`}>
			<div className="place">{placeLabels[place]}</div>
			<Monogram username={row.username} />
			<div className="name">{row.username}</div>
			<div className="eff">{formatEfficiency(row.mean_efficiency)}</div>
			<div className="wks">{row.weeks_played} wks</div>
		</div>
	);
}

interface LeaderboardTableProps {
	rows: LeaderboardRow[];
	/** Provisional rows have no rank yet — render a dash and dimmed styling. */
	provisional?: boolean;
	/** Show a top-3 podium above the table (ranked boards only, when ≥3 rows). */
	showPodium?: boolean;
}

export default function LeaderboardTable({
	rows,
	provisional = false,
	showPodium = false,
}: LeaderboardTableProps) {
	// Podium order is 2nd · 1st · 3rd so the winner sits centered and tallest.
	const topThree = rows.filter((row) => row.rank >= 1 && row.rank <= 3);
	const podiumOrder =
		showPodium && topThree.length === 3
			? [
					topThree.find((r) => r.rank === 2),
					topThree.find((r) => r.rank === 1),
					topThree.find((r) => r.rank === 3),
				]
			: [];

	return (
		<>
			{podiumOrder.length === 3 && (
				<div className="podium">
					{podiumOrder.map(
						(row) => row && <PodiumCard key={row.user_id} row={row} place={row.rank} />,
					)}
				</div>
			)}

			<div className="panel">
				<div className="table-wrap">
					<table className="rk-table">
						<thead>
							<tr>
								<th>Rank</th>
								<th>Manager</th>
								<th>Efficiency</th>
								<th className="num-col">Edge</th>
								<th className="num-col">Win rate</th>
								<th className="num-col">Weeks</th>
							</tr>
						</thead>
						<tbody>
							{rows.map((row) => (
								<tr key={row.user_id}>
									<td className="nm">{provisional ? "—" : `#${row.rank}`}</td>
									<td>
										<div className="lb-who">
											<Monogram username={row.username} />
											<span className="nm">{row.username}</span>
										</div>
									</td>
									<td>
										<div className="lb-eff">
											<span className="v">{formatEfficiency(row.mean_efficiency)}</span>
											<span className="scorebar">
												<span
													className="fill"
													style={{ width: scorebarWidth(row.mean_efficiency) }}
												/>
											</span>
										</div>
									</td>
									<td className="num-col">
										<span className={`lb-edge ${row.edge >= 0 ? "pos" : "neg"}`}>
											{formatEdge(row.edge)}
										</span>
									</td>
									<td className="num-col">{formatWinRate(row.win_rate)}</td>
									<td className="num-col">{row.weeks_played}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			</div>
		</>
	);
}
