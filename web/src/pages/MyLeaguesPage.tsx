import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router";
import { type ApiError, bookmarksKey, fetchJson, postJson } from "../api";
import { Icon } from "../components/icons";
import LeagueBookmarks from "../components/LeagueBookmarks";
import { useAuth } from "../context/AuthContext";
import type { League, LeagueBookmark } from "../types";

export default function MyLeaguesPage() {
	const [leagueId, setLeagueId] = useState("");
	const [label, setLabel] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const [connectOpen, setConnectOpen] = useState(false);
	const [howOpen, setHowOpen] = useState(false);
	const navigate = useNavigate();
	const { userId } = useAuth();
	const queryClient = useQueryClient();

	const saveBookmark = useMutation({
		mutationFn: (validatedId: string) =>
			postJson<LeagueBookmark>("/league-bookmarks", {
				user_id: userId,
				league_id: validatedId,
				label: label.trim(),
				source: "sleeper",
			}),
		onSuccess: () => queryClient.invalidateQueries({ queryKey: bookmarksKey(userId) }),
	});

	async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
		e.preventDefault();
		const id = leagueId.trim();
		if (!id || submitting) {
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await fetchJson<League>(`/league/${id}`);
			await saveBookmark.mutateAsync(id);
		} catch (err) {
			if ((err as ApiError)?.status === 404) {
				setError("League not found. Check that the league ID is correct.");
			} else {
				setError("Something went wrong. Try again.");
			}
			setSubmitting(false);
			return;
		}
		navigate(`/${id}/lineups`);
	}

	return (
		<div className="fade-in">
			<div className="home-hero">
				<h1>Same roster. Same matchups. Different manager.</h1>
				<p>
					Mirror League copies any public Sleeper roster into your own lineup. Set your starters for
					the week, then compare your score to what the real manager actually played and see who's
					the better GM.
				</p>
				<div className="cta">
					<button
						type="button"
						className="btn btn-primary"
						onClick={() => setConnectOpen((o) => !o)}
					>
						<Icon name="plus" />
						Connect a league
					</button>
					<button type="button" className="btn btn-ghost" onClick={() => setHowOpen((o) => !o)}>
						<Icon name="info" />
						How it works
					</button>
				</div>

				<div className={`connect-inline${connectOpen ? " open" : ""}`}>
					<div className="connect-inner">
						<div className="section-label" style={{ marginBottom: 12 }}>
							Connect a Sleeper league
						</div>
						<form onSubmit={handleSubmit}>
							<div className="connect-grid">
								<input
									className="field"
									type="text"
									placeholder="Sleeper league ID"
									value={leagueId}
									onChange={(e) => {
										setLeagueId(e.target.value);
										setError(null);
									}}
									required
								/>
								<input
									className="field"
									type="text"
									placeholder="Label (optional)"
									value={label}
									onChange={(e) => setLabel(e.target.value)}
								/>
							</div>
							<button
								type="submit"
								className="btn btn-primary"
								style={{ marginTop: 12 }}
								disabled={submitting}
							>
								Load League
							</button>
							{error && (
								<p className="mini" style={{ color: "var(--lose)", marginTop: 8 }}>
									{error}
								</p>
							)}
							<p className="mini" style={{ marginTop: 12, lineHeight: 1.6, maxWidth: 520 }}>
								Don't know your league ID? Open your league on{" "}
								<a
									href="https://sleeper.com"
									target="_blank"
									rel="noreferrer"
									style={{ color: "var(--accent)" }}
								>
									sleeper.com
								</a>{" "}
								— it's the long number in the URL, e.g. sleeper.com/leagues/
								<span className="mono">1182073403987832832</span>/team.
							</p>
						</form>
					</div>
				</div>

				<div className={`connect-inline${howOpen ? " open" : ""}`}>
					<div className="connect-inner">
						<div className="section-label" style={{ marginBottom: 12 }}>
							How it works
						</div>
						<ol className="how-steps">
							<li>Paste a public Sleeper league ID above.</li>
							<li>Pick a team to mirror from that league.</li>
							<li>Set your own lineup from the same roster before kickoff.</li>
							<li>Compare your score to what the real manager actually started.</li>
						</ol>
					</div>
				</div>
			</div>

			<LeagueBookmarks userId={userId} />
		</div>
	);
}
