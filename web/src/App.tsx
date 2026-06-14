import { createBrowserRouter, Navigate } from "react-router";
import Layout from "./components/Layout";
import LeagueError from "./components/LeagueError";
import LegacyLeagueRedirect from "./components/LegacyLeagueRedirect";
import RootError from "./components/RootError";
import BestSettersPage from "./pages/BestSettersPage";
import LeaderboardPage from "./pages/LeaderboardPage";
import LeagueStatsPage from "./pages/LeagueStatsPage";
import LineupsPage from "./pages/LineupsPage";
import TeamsPage from "./pages/TeamsPage";
import MyLeaguesPage from "./pages/MyLeaguesPage";
import RookieRankingsPage from "./pages/RookieRankingsPage";

export const router = createBrowserRouter([
	{
		path: "/",
		element: <Layout />,
		errorElement: <RootError />,
		children: [
			// Global scope
			{ index: true, element: <MyLeaguesPage /> },
			{ path: "leaderboard", element: <LeaderboardPage /> },
			{ path: "rankings/rookies", element: <RookieRankingsPage /> },

			// Legacy URL redirects (pre-redesign bookmarks/links)
			{ path: "league/:leagueId", element: <LegacyLeagueRedirect /> },
			{ path: "league/:leagueId/week/:week", element: <LegacyLeagueRedirect /> },

			// League scope
			{
				errorElement: <LeagueError />,
				children: [
					{ path: ":leagueId/lineups", element: <LineupsPage /> },
					{ path: ":leagueId/stats", element: <LeagueStatsPage /> },
					{ path: ":leagueId/best-setters", element: <BestSettersPage /> },
					{ path: ":leagueId/teams", element: <TeamsPage /> },
				],
			},

			{ path: "*", element: <Navigate to="/" replace /> },
		],
	},
]);
