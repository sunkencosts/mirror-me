import { Navigate, useParams } from "react-router";

/** Redirects pre-redesign URLs (/league/:id[/week/:week]) to the new scoped scheme. */
export default function LegacyLeagueRedirect() {
	const { leagueId = "", week } = useParams();
	const to = week ? `/${leagueId}/lineups?week=${week}` : `/${leagueId}/lineups`;
	return <Navigate to={to} replace />;
}
