import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router";
import { type ApiError, bookmarksKey, fetchJson, postJson } from "../api";
import LeagueBookmarks from "../components/LeagueBookmarks";
import { useAuth } from "../context/AuthContext";
import type { League, LeagueBookmark } from "../types";
import styles from "./HomePage.module.css";

export default function HomePage() {
	const [leagueId, setLeagueId] = useState("");
	const [label, setLabel] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
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
		navigate(`/league/${id}`);
	}

	return (
		<>
			<section className={styles.hero}>
				<p className={styles.tagline}>Same roster. Same matchups. Different manager.</p>
				<p className={styles.intro}>
					Mirror League copies any public Sleeper fantasy football roster into your own lineup. Set
					your starters for the week, then compare your score to what the real manager actually
					played to see who is the better manager.
				</p>
				<details className={styles.howItWorks}>
					<summary>How it works</summary>
					<ol className={styles.steps}>
						<li>Paste a Sleeper league ID below</li>
						<li>Pick a team to mirror</li>
						<li>Set your lineup for the week</li>
						<li>Compare your score to theirs</li>
					</ol>
				</details>
			</section>

			<LeagueBookmarks userId={userId} />

			<h2 className={styles.formHeading}>Connect League</h2>
			<form onSubmit={handleSubmit} className={styles.leagueForm}>
				<input
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
					type="text"
					placeholder="Label (optional)"
					value={label}
					onChange={(e) => setLabel(e.target.value)}
					className={styles.labelInput}
				/>
				<button type="submit" disabled={submitting}>
					Load League
				</button>
				{error && <p className={styles.error}>{error}</p>}
			</form>
			<p className={styles.help}>
				Don't know your league ID? Open your league at{" "}
				<a href="https://sleeper.com" target="_blank" rel="noreferrer">
					sleeper.com
				</a>{" "}
				— it's the long number in the page URL, e.g.{" "}
				<code>
					sleeper.com/leagues/<strong>1182073403987832832</strong>/team
				</code>
				.
			</p>
		</>
	);
}
