import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { bookmarksKey, fetchJson, postJson } from "../api";
import { useAuth } from "../context/AuthContext";
import type { League, LeagueBookmark } from "../types";

// Landing page for a shared deeplink (/add/:leagueId). Adds the league to the
// visitor's bookmarks, then sends them straight to its lineups page.
export default function AddLeaguePage() {
	const { leagueId } = useParams();
	const { userId } = useAuth();
	const navigate = useNavigate();
	const queryClient = useQueryClient();
	const [error, setError] = useState(false);
	const ranRef = useRef(false);

	useEffect(() => {
		if (ranRef.current || !leagueId || !userId) {
			return;
		}
		ranRef.current = true;

		void (async () => {
			try {
				// fetchQuery (not raw fetchJson) seeds the ["league", id] cache the
				// destination LineupsPage reads. retry:false so an invalid/private
				// league id surfaces the error immediately instead of after backoff.
				const league = await queryClient.fetchQuery({
					queryKey: ["league", leagueId],
					queryFn: () => fetchJson<League>(`/league/${leagueId}`),
					retry: false,
				});
				await postJson<LeagueBookmark>("/league-bookmarks", {
					user_id: userId,
					league_id: leagueId,
					label: league.name,
					source: "sleeper",
				});
				// Fire-and-forget: the My Leagues list isn't mounted here, so don't
				// block the redirect on its refetch.
				void queryClient.invalidateQueries({ queryKey: bookmarksKey(userId) });
				navigate(`/${leagueId}/lineups`, { replace: true });
			} catch {
				setError(true);
			}
		})();
	}, [leagueId, userId, navigate, queryClient]);

	if (error) {
		return (
			<div className="panel">
				<div className="sub" style={{ margin: 0 }}>
					Couldn't add that league. It may not exist, or something went wrong.{" "}
					<button type="button" className="btn btn-ghost" onClick={() => navigate("/")}>
						Back to My Leagues
					</button>
				</div>
			</div>
		);
	}

	return <p>Adding league…</p>;
}
