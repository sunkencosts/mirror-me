// Shared formatting for efficiency/edge figures and the efficiency scorebar, used by the
// season leaderboard and the weekly results browser.

export const formatEfficiency = (value: number) => `${Math.round(value * 100)}%`;
export const formatEdge = (value: number) => `${value >= 0 ? "+" : ""}${(value * 100).toFixed(1)}%`;
export const formatWinRate = (value: number) => `${Math.round(value * 100)}%`;

// ordinal(1) => "1st", ordinal(2) => "2nd", ordinal(11) => "11th" — for "you set the 10th
// best lineup". Handles the 11/12/13 exception.
export const ordinal = (n: number) => {
	const rem100 = n % 100;
	if (rem100 >= 11 && rem100 <= 13) {
		return `${n}th`;
	}
	switch (n % 10) {
		case 1:
			return `${n}st`;
		case 2:
			return `${n}nd`;
		case 3:
			return `${n}rd`;
		default:
			return `${n}th`;
	}
};

// Real efficiencies cluster high (~85–100%), so a raw 0–100% bar reads as "all full" and
// hides the ranking. Zoom the bar into the meaningful range: SCOREBAR_FLOOR maps to empty,
// 100% to full (clamped). The headline % shown beside it stays the true, unscaled value.
const SCOREBAR_FLOOR = 0.75;
export const scorebarWidth = (value: number) => {
	const scaled = (value - SCOREBAR_FLOOR) / (1 - SCOREBAR_FLOOR);
	return `${Math.round(Math.max(0, Math.min(1, scaled)) * 100)}%`;
};
