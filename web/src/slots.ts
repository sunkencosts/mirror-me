import type { Player } from "./types";

export const SLOT_DISPLAY: Record<string, string> = {
	SUPER_FLEX: "SF",
};

export function slotLabel(slot: string): string {
	return SLOT_DISPLAY[slot] ?? slot;
}

// Design's `.pos` badge variant: green default, blue flex, purple superflex.
export function slotPosClass(slot: string): string {
	if (slot === "FLEX") {
		return "flex";
	}
	if (slot === "SUPER_FLEX" || slot === "SF") {
		return "sf";
	}
	return "";
}

export const SLOT_ELIGIBILITY: Record<string, string[]> = {
	QB: ["QB"],
	RB: ["RB"],
	WR: ["WR"],
	TE: ["TE"],
	K: ["K"],
	DEF: ["DEF"],
	FLEX: ["RB", "WR", "TE"],
	SUPER_FLEX: ["QB", "RB", "WR", "TE"],
	IDP_FLEX: ["DL", "LB", "DB"],
	DL: ["DL"],
	LB: ["LB"],
	DB: ["DB"],
};

export function canFillSlot(slot: string, player: Player): boolean {
	const eligible = SLOT_ELIGIBILITY[slot];
	if (!eligible) {
		return false;
	}
	return player.fantasy_positions.some((pos) => eligible.includes(pos));
}
