import { Icon } from "../icons";

function SectionDivider({ label }: { label: string }) {
	return (
		<div className="section-divider">
			<div className="section-label" style={{ whiteSpace: "nowrap", color: "var(--mute)" }}>
				{label}
			</div>
			<div className="line" />
		</div>
	);
}

const KPIS = [
	{ label: "CV R²", value: "0.373", pm: "±0.101", note: "5-fold CV", bar: 37.3 },
	{ label: "CV R² (PPR)", value: "0.331", pm: "±0.073", note: "5-fold CV", bar: 33.1 },
	{ label: "Training samples", value: "264", pm: null, note: "matched players · 2014–24", bar: null },
	{ label: "CV folds", value: "5", pm: null, note: "train 4/5 · test 1/5 · rotate", bar: null },
];

const PIPELINE = [
	{
		step: "DATA",
		title: "API Ingestion + Caching",
		body: (
			<>
				College stats from the CollegeFootballData API (2014–2025). NFL rookie fantasy points and
				draft picks via nflreadpy.
			</>
		),
	},
	{
		step: "FEATURES",
		title: "Feature Engineering",
		body: (
			<>
				Derived features <code>total_touchdowns</code> and <code>yards_from_scrimmage</code> package signal the model would otherwise discover slowly. Position-aware zeroing prevents
				gadget-play noise. StandardScaler + OneHotEncoder in a ColumnTransformer.
			</>
		),
	},
	{
		step: "MODEL",
		title: "Random Forest Regressor",
		body: (
			<>
				200-tree RandomForestRegressor wrapped in a full scikit-learn Pipeline, trained on college
				production stats plus actual draft slot. Draft slot is one of the strongest single predictors
				of NFL success because it encodes combine data, injury history, and scheme fit that box
				scores can't.
			</>
		),
	},
	{
		step: "VALIDATION",
		title: "5-Fold Cross-Validation",
		body: (
			<>
				Train on 4/5 of data, test on the held-out 1/5, rotate through all folds, average the
				results. The resulting CV R² (0.373 standard, 0.331 PPR) measures how much of the variance in
				rookie fantasy output the model explains on players it never saw during training.
			</>
		),
	},
	{
		step: "SHAP",
		title: "Interpretability + Debugging",
		body: (
			<>
				SHAP (SHapley Additive exPlanations) explains individual predictions and was used to{" "}
				<em>actively debug</em> the model, not just explain it after the fact. It caught a spurious
				feature driving rankings for the wrong reasons, leading to a direct model improvement.
			</>
		),
	},
];

const LIMITATIONS = [
	"No combine / athletic testing data: 40-yd dash, vertical, etc.",
	"No birth date or breakout-age data because the CollegeFootballData API doesn't expose player DOBs.",
	"Can't model scheme fit (e.g. triple-option QBs whose college stats don't translate) or landing-spot opportunity.",
	"Explicitly framed as a complement to expert consensus rankings, not a replacement. Useful for flagging undervalued high-volume producers, not as a sole source of truth.",
];

const TECH_STACK = [
	{ name: "Python 3.13", note: "runtime" },
	{ name: "pandas", note: "data pipeline" },
	{ name: "scikit-learn", note: "Pipeline · RF · CV · encoders" },
	{ name: "SHAP", note: "interpretability" },
	{ name: "rapidfuzz", note: "entity resolution" },
	{ name: "nflreadpy", note: "NFL data" },
	{ name: "requests", note: "API ingestion" },
	{ name: "joblib", note: "model persistence" },
];

export default function AboutTheModel() {
	return (
		<>
			<SectionDivider label="ABOUT THE MODEL" />

			<div className="kpi-row">
				{KPIS.map((k) => (
					<div className="kpi" key={k.label}>
						<div className="l">{k.label}</div>
						<div className="v tnum">
							{k.value}
							{k.pm && <span className="pm">{k.pm}</span>}
						</div>
						<div className="d">{k.note}</div>
						{k.bar !== null && (
							<div className="bar">
								<div className="fill" style={{ width: `${k.bar}%` }} />
							</div>
						)}
					</div>
				))}
			</div>

			<div className="method">
				{PIPELINE.map((c) => (
					<div className="mcard" key={c.step}>
						<div className="step">{c.step}</div>
						<h5>{c.title}</h5>
						<p>{c.body}</p>
					</div>
				))}
			</div>

			<div className="panel" style={{ marginBottom: "var(--gap)" }}>
				<div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }}>
					<span className="tag green">POST-DRAFT</span>
					<span style={{ fontFamily: "var(--display)", fontWeight: 700, fontSize: 15 }}>
						The Model
					</span>
				</div>
				<div className="sub" style={{ marginBottom: 14 }}>
					College stats + draft slot · CV R² = 0.373 ±0.101
				</div>
				<p style={{ margin: "0 0 10px", fontSize: 13, color: "var(--dim)", lineHeight: 1.65 }}>
					Ranks prospects on college production plus their actual draft slot. Draft position is one
					of the strongest single predictors of NFL success because it's a compressed encoding of
					combine numbers, injury history, and scheme fit that college box scores simply can't see.
				</p>
				<p style={{ margin: 0, fontSize: 13, color: "var(--dim)", lineHeight: 1.65 }}>
					Trained on 11 years of data (2014–2024 college → 2015–2025 NFL) since draft position is
					a stable signal across eras.
				</p>
			</div>

			<div className="infobox" style={{ marginBottom: "var(--gap)" }}>
				<div className="ic">
					<Icon name="bolt" />
				</div>
				<div>
					<h4>SHAP debugging: the Sun Belt RB story</h4>
					<p style={{ margin: "0 0 8px", fontSize: 13, color: "var(--dim)", lineHeight: 1.65 }}>
						An early model version ranked a Sun Belt running back unusually high. Inspecting his
						SHAP waterfall revealed that his projected score was driven almost entirely by his
						conference, a spurious correlation with no causal relationship to NFL success. His
						college production was ordinary; the model had just learned that Sun Belt backs
						happened to over-perform in its small training window.
					</p>
					<p style={{ margin: 0, fontSize: 13, color: "var(--dim)", lineHeight: 1.65 }}>
						The fix was removing conference as a feature entirely. This is how the model actually
						improved: not by tuning hyperparameters in the dark, but by using feature attributions
						to catch exactly what the model was learning — and pruning it when the answer was
						wrong for the right reasons.
					</p>
				</div>
			</div>

			<SectionDivider label="LIMITATIONS" />

			<div className="grid-2" style={{ marginBottom: "var(--gap)" }}>
				<div className="panel">
					<h4 style={{ marginBottom: 4 }}>Known Limitations</h4>

					<ul style={{ margin: 0, paddingLeft: 18, fontSize: 13, color: "var(--dim)", lineHeight: 1.85 }}>
						{LIMITATIONS.map((l) => (
							<li key={l}>{l}</li>
						))}
					</ul>
				</div>

			</div>

			<div className="panel">
				<h4 style={{ marginBottom: 4 }}>Tech Stack</h4>
				<div className="sub" style={{ marginBottom: 14 }}>
					End-to-end Python ML pipeline
				</div>
				<div className="tech-row">
					{TECH_STACK.map((t) => (
						<div className="tech-chip" key={t.name}>
							<span className="name">{t.name}</span>
							<span className="note">{t.note}</span>
						</div>
					))}
				</div>
			</div>
		</>
	);
}
