import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { fetchJson } from "../api";
import LeaderboardTable from "../components/LeaderboardTable";
import ScoringInfoBox from "../components/ScoringInfoBox";
import type { LeaderboardRow } from "../types";

// Only 2026 has data today; the backend defaults to 2026 and accepts ?season=.
const SEASONS = ["2026"];
const QUALIFYING_WEEKS = 3; // global board: < 3 graded weeks → provisional

export default function LeaderboardPage() {
	const [season, setSeason] = useState(SEASONS[0]);

	const { data: rows = [], isLoading } = useQuery<LeaderboardRow[]>({
		queryKey: ["leaderboard", season],
		queryFn: () => fetchJson(`/leaderboard?season=${season}`),
		select: (data) => data ?? [],
		throwOnError: true,
	});

	if (isLoading) {
		return <p>Loading…</p>;
	}

	const ranked = rows.filter((row) => !row.provisional);
	const provisional = rows.filter((row) => row.provisional);

	return (
		<div className="fade-in">
			<ScoringInfoBox global />

			{SEASONS.length > 1 && (
				<div className="filters">
					{SEASONS.map((s) => (
						<button
							key={s}
							type="button"
							className={`fchip${s === season ? " active" : ""}`}
							onClick={() => setSeason(s)}
						>
							{s}
						</button>
					))}
				</div>
			)}

			{rows.length === 0 ? (
				<div className="panel">
					<div className="lb-empty">No ranked managers yet for {season}.</div>
				</div>
			) : (
				<>
					<LeaderboardTable rows={ranked} showPodium />

					{provisional.length > 0 && (
						<div className="provisional">
							<div className="lead">
								Almost there — {QUALIFYING_WEEKS} graded weeks are needed to qualify for the
								leaderboard.
							</div>
							<LeaderboardTable rows={provisional} provisional />
						</div>
					)}
				</>
			)}
		</div>
	);
}
