import { useMemo } from "react";
import { RARITY_LABELS, RARITY_ORDER, RARITY_SEG } from "../rarity";
import { computePowerScore, getTierRelative } from "../scoring";
import type { Player } from "../types";

interface Props {
	players: Player[];
	starters: Player[];
	allScores: number[];
}

export default function RosterRarity({ players, starters, allScores }: Props) {
	const rarityCounts = useMemo(
		() =>
			players.reduce<Record<string, number>>((acc, p) => {
				const key = p.rarity ?? "grey";
				acc[key] = (acc[key] ?? 0) + 1;
				return acc;
			}, {}),
		[players],
	);

	const score = useMemo(() => computePowerScore(players, starters), [players, starters]);
	const tier = useMemo(() => getTierRelative(score, allScores), [score, allScores]);
	const present = RARITY_ORDER.filter((r) => rarityCounts[r] > 0);

	return (
		<div className="power">
			<div className="power-top">
				<span className={`tier-badge tier-${tier}`}>{tier}-TIER</span>
				<span className="power-num">
					ROSTER POWER<b>{score.toFixed(1)}</b>/10
				</span>
			</div>

			<div className="power-bar">
				{present.map((r) => (
					<span
						key={r}
						className={RARITY_SEG[r]}
						style={{ flex: rarityCounts[r] }}
						title={`${RARITY_LABELS[r]}: ${rarityCounts[r]}`}
					>
						{rarityCounts[r]}
					</span>
				))}
			</div>

			<div className="power-legend">
				{present.map((r) => (
					<span key={r}>
						<i className={RARITY_SEG[r]} />
						{RARITY_LABELS[r]}
					</span>
				))}
			</div>
		</div>
	);
}
