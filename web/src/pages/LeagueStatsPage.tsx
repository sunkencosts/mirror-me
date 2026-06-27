import { useParams } from "react-router";
import WeeklyResults from "../components/weekly/WeeklyResults";

// League Stats is a single, personal question: "how did my lineup do this week vs the original
// owner and everyone else who mirrored the team?" It is the weekly results browser — a week
// picker, your personal headline ("you set the Nth best lineup"), and the ranked field. The
// season-aggregate leaderboard lives on the global /leaderboard page, not here.
export default function LeagueStatsPage() {
	const { leagueId = "" } = useParams();

	return (
		<div className="fade-in">
			<WeeklyResults leagueId={leagueId} />
		</div>
	);
}
