import { useEffect } from "react";
import { useLocation } from "react-router";
import { activeRouteKey, LEAGUE_ROUTES } from "../components/shell/routes";

const LAST_LEAGUE_KEY = "ml_last_league";

export interface CurrentLeague {
	/** League id from the URL when on a league route, else the last-visited league. */
	leagueId: string | null;
	/** Whether the current route is league-scoped. */
	isLeagueRoute: boolean;
	/** Active route key (home, lineups, stats, …). */
	routeKey: string;
}

export function useCurrentLeague(): CurrentLeague {
	const { pathname } = useLocation();
	const segments = pathname.split("/").filter(Boolean);
	const routeKey = activeRouteKey(pathname);
	const isLeagueRoute = LEAGUE_ROUTES.some((r) => r.key === routeKey);
	// Legacy URLs (/league/:leagueId[/week/:week]) carry the leagueId in segments[1]
	// while LegacyLeagueRedirect computes its <Navigate> target.
	const legacyLeagueId = segments[0] === "league" ? (segments[1] ?? null) : null;
	const routeLeagueId = legacyLeagueId ?? (isLeagueRoute ? (segments[0] ?? null) : null);

	useEffect(() => {
		if (routeLeagueId) {
			localStorage.setItem(LAST_LEAGUE_KEY, routeLeagueId);
		}
	}, [routeLeagueId]);

	const remembered = typeof window !== "undefined" ? localStorage.getItem(LAST_LEAGUE_KEY) : null;
	return { leagueId: routeLeagueId ?? remembered, isLeagueRoute, routeKey };
}
