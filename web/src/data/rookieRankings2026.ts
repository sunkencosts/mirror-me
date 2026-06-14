import csv from "./rookieRankings2026.csv?raw";

export type RookiePosition = "QB" | "RB" | "WR" | "TE";
export type RookieModel = "no_pick" | "with_pick";

export interface RookieRanking {
	player: string;
	position: RookiePosition;
	/** Predicted rookie-season fantasy points, standard scoring (season total, not per-game). */
	points: number;
	/** Predicted rookie-season fantasy points, PPR scoring. Only present for the with-pick model. */
	pointsPpr: number | null;
	conference: string;
	/** NFL team the player was drafted to, e.g. "ARI". Empty if not yet drafted. */
	team: string;
	model: RookieModel;
	/** Rank within position for this model, standard scoring. */
	positionRank: number;
	/** Rank within position, PPR scoring. Only present for the with-pick model. */
	positionRankPpr: number | null;
}

function parseRookieRankings(raw: string): RookieRanking[] {
	const [header, ...rows] = raw.trim().split("\n");
	const columns = header.split(",");
	const columnIndex = Object.fromEntries(columns.map((name, i) => [name, i]));

	return rows.map((row) => {
		const values = row.split(",");
		const get = (name: string) => values[columnIndex[name]];
		const getNumberOrNull = (name: string) => {
			const value = get(name);
			return value === "" ? null : Number(value);
		};

		return {
			player: get("player"),
			position: get("position") as RookiePosition,
			points: Number(get("predicted_fantasy_points")),
			pointsPpr: getNumberOrNull("predicted_fantasy_points_ppr"),
			conference: get("conference"),
			team: get("team"),
			model: get("model") as RookieModel,
			positionRank: Number(get("position_rank")),
			positionRankPpr: getNumberOrNull("position_rank_ppr"),
		};
	});
}

export const rookieRankings2026: RookieRanking[] = parseRookieRankings(csv);
