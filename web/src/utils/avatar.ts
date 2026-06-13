// Monogram avatar helpers — ported from design_handoff_mirror_league/core.js.
// Used by the `.pav` rarity-ring avatar system when no real headshot is shown.

export function initials(name: string): string {
	const parts = name
		.replace(/[^A-Za-z .'-]/g, "")
		.split(/\s+/)
		.filter(Boolean);
	return ((parts[0]?.[0] ?? "") + (parts[1]?.[0] ?? "")).toUpperCase();
}

/** Deterministic gradient background per name (name chars → HSL hue). */
export function avatarBg(name: string): string {
	let h = 0;
	for (let i = 0; i < name.length; i++) {
		h = (h * 31 + name.charCodeAt(i)) % 360;
	}
	return `linear-gradient(150deg,hsl(${h} 24% 78%),hsl(${(h + 40) % 360} 20% 60%))`;
}
