import type { League } from "../types";
import { pprLabel } from "../utils/league";
import WeekSelector from "./WeekSelector";

interface Props {
	league: League;
	weekNumber: number;
	onWeekChange: (week: number) => void;
}

export default function LeagueSummary({
	league: {
		name,
		roster_positions,
		scoring_settings,
		settings: { num_teams },
	},
	weekNumber,
	onWeekChange,
}: Props) {
	const ppr = pprLabel(scoring_settings.rec);
	const tep = scoring_settings.bonus_rec_te > 0;
	const superflex = roster_positions.includes("SUPER_FLEX");

	return (
		<div className="league-hero">
			<div className="lg-badge">⛳</div>
			<div>
				<h2>{name}</h2>
				<div className="meta">
					<span className="tag">{num_teams} Teams</span>
					{ppr && <span className="tag green">{ppr}</span>}
					{tep && <span className="tag green">TEP +{scoring_settings.bonus_rec_te}</span>}
					{superflex && <span className="tag purple">Superflex</span>}
				</div>
			</div>
			<WeekSelector weekNumber={weekNumber} onWeekChange={onWeekChange} />
		</div>
	);
}
