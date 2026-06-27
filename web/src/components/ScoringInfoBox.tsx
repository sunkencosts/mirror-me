import { Icon } from "./icons";

interface ScoringInfoBoxProps {
	global?: boolean;
}

/** Ranking rules panel — shared by the per-league and global leaderboards. */
export default function ScoringInfoBox({ global = false }: ScoringInfoBoxProps) {
	return (
		<div className="infobox">
			<div className="ic">
				<Icon name="info" />
			</div>
			<div>
				<h4>How ranking works</h4>
				<ol>
					<li>
						All picks <b>lock at the first kickoff</b> each week — no retroactive editing.
					</li>
					<li>
						Each week you score an <b className="k">efficiency</b> = your starters' points ÷ the
						best possible lineup from that roster.
					</li>
					<li>
						Your rank is your <b>average efficiency</b> across graded weeks.
					</li>
					<li>
						<b>Edge</b> = how much your efficiency beats the real manager's, averaged — the brag.
					</li>
					{global && (
						<li>
							<b>3 graded weeks</b> are required to qualify for the leaderboard.
						</li>
					)}
					<li>
						<b>Win rate</b> (weeks you outscored the real manager) is shown as a secondary stat.
					</li>
				</ol>
			</div>
		</div>
	);
}
