import { Icon } from "./icons";

interface ScoringInfoBoxProps {
	global?: boolean;
}

/** Manager Score rules panel — shared by Best Setters and the global Leaderboard. */
export default function ScoringInfoBox({ global = false }: ScoringInfoBoxProps) {
	return (
		<div className="infobox">
			<div className="ic">
				<Icon name="info" />
			</div>
			<div>
				<h4>How Manager Score works</h4>
				<ol>
					<li>
						All picks <b>lock at the first kickoff</b> each week — no retroactive editing.
					</li>
					<li>
						You <b>win a week</b> if your mirrored starters outscore what the real manager started.
					</li>
					<li>
						<b className="k">Manager Score</b> = the percentage of weeks you beat the real manager.
					</li>
					<li>
						<b>4 locked weeks</b> are required to qualify{global ? " for the leaderboard" : ""}.
					</li>
					<li>
						Raw win rate is <b>regressed toward 50%</b> so small-sample flukes can't top a long
						record.
					</li>
					<li>Tiebreak: average points-gained margin per week.</li>
				</ol>
			</div>
		</div>
	);
}
