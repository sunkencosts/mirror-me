import { useState } from "react";
import AboutTheModel from "../components/rookie/AboutTheModel";
import RankingsHero, { type ScoringMode } from "../components/rookie/RankingsHero";
import RankingsList from "../components/rookie/RankingsList";
import type { RookiePosition } from "../data/rookieRankings2026";
import "./RookieRankingsPage.css";

export default function RookieRankingsPage() {
	const [scoringMode, setScoringMode] = useState<ScoringMode>("standard");
	const [posFilter, setPosFilter] = useState<"ALL" | RookiePosition>("ALL");

	return (
		<div className="fade-in">
			<RankingsHero scoringMode={scoringMode} onScoringModeChange={setScoringMode} />
			<RankingsList
				scoringMode={scoringMode}
				posFilter={posFilter}
				onPosFilterChange={setPosFilter}
			/>
			<AboutTheModel />
		</div>
	);
}
