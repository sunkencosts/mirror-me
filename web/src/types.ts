export interface Player {
	player_id: string;
	first_name: string;
	last_name: string;
	number: number;
	age: number;
	team: string;
	active: boolean;
	fantasy_positions: string[];
	image_url: string;
	rarity?: import("./rarity").Rarity;
}

export interface SwapOption {
	player: Player;
	isBench: boolean;
}

export interface Roster {
	roster_id: number;
	owner_id: string;
	team_name: string;
	owner_avatar_url: string;
	players: Player[];
	starters: Player[];
	reserve: Player[];
	taxi: Player[];
}

export interface League {
	roster_positions: string[];
	name: string;
	avatar_url: string;
	scoring_settings: {
		bonus_rec_te: number;
		rec: number;
	};
	settings: {
		reserve_slots: number;
		taxi_slots: number;
		num_teams: number;
		leg: number;
	};
}

export interface Lineup {
	id: string;
	roster_id: number;
	week_number: number;
	starters: string[];
	locked?: boolean;
	locks_at?: string;
}

export interface LeagueConfig {
	starterSlots: string[];
	irSlots: number;
	taxiSlots: number;
}

export interface WeekMatchup {
	roster_id: number;
	matchup_id: number;
	owner_id: string;
	team_name: string;
	owner_avatar_url: string;
	points: number;
	custom_points: number | null;
	players: Player[];
	starters: Player[];
	player_points: Record<string, number>;
}

// Envelope returned by GET /league/:leagueId/week/:week. `locked` is true once the
// week's first game has kicked off; `locks_at` is that kickoff time (UTC, omitted
// when no lock is seeded). This is the lineup editor's source of truth for whether
// edits are still allowed.
export interface WeekMatchupsResponse {
	locked: boolean;
	locks_at?: string;
	matchups: WeekMatchup[];
}

export interface AuthUser {
	id: string;
	email: string;
	username: string;
}

export interface SlimPlayer {
	player_id: string;
	first_name: string;
	last_name: string;
	team: string;
	fantasy_positions: string[];
	image_url: string;
	rarity: string;
}

export interface LeagueBookmark {
	user_id: string;
	league_id: string;
	label: string;
	created_at: string;
	source: string;
	icon_url: string;
}

// A player with their points for one scored week (compare / per-setter lineup detail).
export interface ScoredPlayer extends Player {
	points: number;
}

export interface ScoredLineup {
	lineup_id?: string;
	starters: ScoredPlayer[];
	total_points: number;
}

// One user's lineup scored against the real manager's official lineup + the roster's optimal
// lineup, for a week. Returned by both the auth'd compare and the public per-setter endpoint.
export interface CompareResponse {
	roster_id: number;
	week: number;
	official: ScoredLineup;
	user: ScoredLineup;
	winner: "user" | "official" | "tie";
	optimal_points: number;
	user_efficiency: number;
	official_efficiency: number;
	edge: number;
	final: boolean;
}

// One setter's row in the weekly results browser (GET .../week/{week}/results).
export interface WeeklySetterResult {
	user_id: string;
	username: string;
	user_total: number;
	efficiency: number; // 0..1
	edge: number; // efficiency − official efficiency
	result: "user" | "official" | "tie";
	rank: number; // standing within the roster
}

// Weekly results for one roster: the official/optimal baseline + ranked setters who mirrored
// it. setter_count is the unfiltered total; setters may be a searched/paginated subset.
export interface WeeklyRosterResults {
	roster_id: number;
	official_total: number;
	optimal_total: number;
	official_efficiency: number;
	setter_count: number;
	beat_official_count: number; // how many setters outscored the original manager
	setters: WeeklySetterResult[];
}

// A single row on the global or per-league leaderboard. Rows arrive already sorted:
// ranked rows first (by mean_efficiency desc), then any provisional rows (global board
// only) with rank 0. Render in array order.
export interface LeaderboardRow {
	user_id: string;
	username: string;
	rank: number; // 1-based among ranked rows; 0 for provisional rows
	mean_efficiency: number; // 0..1 — the headline number and the sort key
	edge: number; // mean (you − manager)/optimal; can be negative
	win_rate: number; // 0..1 — wins/(wins+losses), ties excluded (secondary)
	weeks_played: number;
	provisional: boolean; // true → not yet ranked (global board only)
}
