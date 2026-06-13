export const RARITY_ORDER = ["mythic", "orange", "purple", "blue", "green", "grey"] as const;
export type Rarity = (typeof RARITY_ORDER)[number];

const HEX: Record<Rarity, string> = {
	mythic: "#ffd700",
	orange: "#f97316",
	purple: "#a855f7",
	blue: "#3b82f6",
	green: "#22c55e",
	grey: "#4b5563",
};

export const RARITY_COLORS: Record<Rarity, string> = { ...HEX };

export const RARITY_POINTS: Record<Rarity, number> = {
	grey: 1,
	green: 2,
	blue: 4,
	purple: 7,
	orange: 11,
	mythic: 16,
};

export const RARITY_LABELS: Record<Rarity, string> = {
	mythic: "MYT",
	orange: "LEG",
	purple: "EPIC",
	blue: "RARE",
	green: "UNC",
	grey: "COM",
};

// Maps our rarity tiers to the design's `.pav` ring CSS classes.
export const RARITY_RING: Record<Rarity, string> = {
	mythic: "r-myt",
	orange: "r-leg",
	purple: "r-epic",
	blue: "r-rare",
	green: "r-unc",
	grey: "r-com",
};

// Segment background class for the roster-power bar.
export const RARITY_SEG: Record<Rarity, string> = {
	mythic: "seg-myt",
	orange: "seg-leg",
	purple: "seg-epic",
	blue: "seg-rare",
	green: "seg-unc",
	grey: "seg-com",
};

export const RARITY_GLOW: Record<Rarity, string> = {
	mythic: `0 0 0 2px ${HEX.mythic}, 0 0 14px rgba(255, 215, 0, 0.7)`,
	orange: `0 0 0 2px ${HEX.orange}, 0 0 10px rgba(249, 115, 22, 0.6)`,
	purple: `0 0 0 2px ${HEX.purple}, 0 0 10px rgba(168, 85, 247, 0.5)`,
	blue: `0 0 0 2px ${HEX.blue},   0 0  8px rgba(59, 130, 246, 0.4)`,
	green: `0 0 0 2px ${HEX.green},  0 0  6px rgba(34, 197, 94, 0.3)`,
	grey: `0 0 0 2px ${HEX.grey}`,
};
