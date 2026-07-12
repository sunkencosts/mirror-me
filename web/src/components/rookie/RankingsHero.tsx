import { useCopyToClipboard } from "../../hooks/useCopyToClipboard";
import { Icon } from "../icons";

export type ScoringMode = "standard" | "ppr";

interface RankingsHeroProps {
	scoringMode: ScoringMode;
	onScoringModeChange: (mode: ScoringMode) => void;
}

interface R2Entry {
	value: string;
	pm: string;
	label: string;
}

const R2: Record<ScoringMode, R2Entry> = {
	standard: {
		value: "0.373",
		pm: "±0.101",
		label: "Standard scoring · draft slot as feature",
	},
	ppr: {
		value: "0.331",
		pm: "±0.073",
		label: "PPR scoring · draft slot as feature",
	},
};

export default function RankingsHero({ scoringMode, onScoringModeChange }: RankingsHeroProps) {
	const r2 = R2[scoringMode];
	const { copied, copy } = useCopyToClipboard();

	return (
		<div className="home-hero rr-hero">
			<div className="tags">
				<span className="tag green">
					<Icon name="bolt" />
					ROOKIE RANKER
				</span>
				<span className="tag">2026 CLASS</span>
				<span className="mini">Random Forest · scikit-learn · 5-fold CV</span>
			</div>
			<h1>2026 NFL Rookie Rankings</h1>
			<p>
				Projected year-1 fantasy points for every skill-position rookie, ranked by a supervised ML
				pipeline trained on college production and draft data.
			</p>
			<div className="controls">
				<div>
					<div className="seg-label">SCORING</div>
					<div className="seg">
						<button
							type="button"
							className={scoringMode === "standard" ? "on" : ""}
							onClick={() => onScoringModeChange("standard")}
						>
							Standard
						</button>
						<button
							type="button"
							className={scoringMode === "ppr" ? "on" : ""}
							onClick={() => onScoringModeChange("ppr")}
						>
							PPR
						</button>
					</div>
				</div>
				<div className="r2">
					<div className="lbl">{r2.label}</div>
					<div className="val">
						CV R² {r2.value}
						<span className="pm">{r2.pm}</span>
					</div>
				</div>
				<button
					type="button"
					className="btn btn-ghost btn-sm"
					style={{ marginLeft: "auto" }}
					onClick={() => copy(window.location.href)}
				>
					<Icon name="share" />
					{copied ? "Copied!" : "Share"}
				</button>
			</div>
		</div>
	);
}
