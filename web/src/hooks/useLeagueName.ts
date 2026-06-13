import { useQuery } from "@tanstack/react-query";
import { bookmarksKey, fetchJson } from "../api";
import { useAuth } from "../context/AuthContext";
import type { LeagueBookmark } from "../types";
import { useCurrentLeague } from "./useCurrentLeague";

/** Display name for the current league, resolved from the user's bookmarks. */
export function useLeagueName(): string | null {
	const { userId } = useAuth();
	const { leagueId } = useCurrentLeague();

	const { data: bookmarks = [] } = useQuery<LeagueBookmark[]>({
		queryKey: bookmarksKey(userId),
		queryFn: () => fetchJson(`/league-bookmarks?user_id=${userId}`),
	});

	if (!leagueId) {
		return null;
	}
	return bookmarks.find((b) => b.league_id === leagueId)?.label ?? null;
}
