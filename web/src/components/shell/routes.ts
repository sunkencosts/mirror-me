import type { IconName } from "../icons";

export type Scope = "global" | "league";

export interface RouteDef {
	key: string;
	title: string;
	scope: Scope;
	navIcon: IconName;
	navLabel: string;
	/** Build the URL for this route. League routes require a leagueId. */
	to: (leagueId?: string) => string;
}

export const GLOBAL_ROUTES: RouteDef[] = [
	{
		key: "home",
		title: "My Leagues",
		scope: "global",
		navIcon: "grid",
		navLabel: "My Leagues",
		to: () => "/",
	},
	{
		key: "leaderboard",
		title: "Leaderboard",
		scope: "global",
		navIcon: "trophy",
		navLabel: "Leaderboard",
		to: () => "/leaderboard",
	},
	{
		key: "rookies",
		title: "Rookie Rankings",
		scope: "global",
		navIcon: "list",
		navLabel: "Rookie Rankings",
		to: () => "/rankings/rookies",
	},
];

export const LEAGUE_ROUTES: RouteDef[] = [
	{
		key: "lineups",
		title: "Lineups",
		scope: "league",
		navIcon: "stack",
		navLabel: "Lineups",
		to: (id) => `/${id}/lineups`,
	},
	{
		key: "stats",
		title: "League Stats",
		scope: "league",
		navIcon: "chart",
		navLabel: "League Stats",
		to: (id) => `/${id}/stats`,
	},
	{
		key: "setters",
		title: "Best Setters",
		scope: "league",
		navIcon: "medal",
		navLabel: "Best Setters",
		to: (id) => `/${id}/best-setters`,
	},
	{
		key: "members",
		title: "Members",
		scope: "league",
		navIcon: "users",
		navLabel: "Members",
		to: (id) => `/${id}/members`,
	},
];

export const ALL_ROUTES: RouteDef[] = [...GLOBAL_ROUTES, ...LEAGUE_ROUTES];

const LEAGUE_SEGMENT_TO_KEY: Record<string, string> = {
	lineups: "lineups",
	stats: "stats",
	"best-setters": "setters",
	members: "members",
};

/** Resolve the active route key from a pathname. */
export function activeRouteKey(pathname: string): string {
	if (pathname === "/") {
		return "home";
	}
	if (pathname.startsWith("/leaderboard")) {
		return "leaderboard";
	}
	if (pathname.startsWith("/rankings/rookies")) {
		return "rookies";
	}
	const segments = pathname.split("/").filter(Boolean); // [leagueId, page]
	return LEAGUE_SEGMENT_TO_KEY[segments[1] ?? ""] ?? "home";
}

export function routeByKey(key: string): RouteDef | undefined {
	return ALL_ROUTES.find((r) => r.key === key);
}
