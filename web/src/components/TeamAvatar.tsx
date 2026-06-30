import { useState } from "react";
import { avatarBg, initials } from "../utils/avatar";

interface Props {
	/** Team/owner name — drives the monogram and its deterministic gradient. */
	name: string;
	/** Resolved Sleeper avatar URL. Empty/missing falls back to the monogram. */
	avatarUrl?: string | null;
	/** Rarity ring class (e.g. "r-leg", "r-dim"). Defaults to the neutral ring. */
	ring?: string;
	/** Diameter in px. Omit to inherit sizing from the CSS context (e.g. `.pop-item .pav`). */
	size?: number;
}

/** A `.pav` avatar that shows the real Sleeper headshot when available and falls back to a
    monogram (on missing URL or load error). Shared by Teams, the roster header, and the
    team picker. */
export default function TeamAvatar({ name, avatarUrl, ring = "r-dim", size }: Props) {
	const [errored, setErrored] = useState(false);
	const showImage = !!avatarUrl && !errored;

	const style: React.CSSProperties = showImage ? {} : { background: avatarBg(name) };
	if (size != null) {
		style.width = size;
		style.height = size;
		style.fontSize = Math.round(size * 0.375);
	}

	return (
		<div className={`pav ${ring}`} style={style}>
			{showImage ? (
				<img src={avatarUrl ?? ""} alt="" onError={() => setErrored(true)} />
			) : (
				initials(name)
			)}
		</div>
	);
}
