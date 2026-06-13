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
	players: Player[];
	starters: Player[];
	reserve: Player[];
	taxi: Player[];
}

export interface League {
	roster_positions: string[];
	name: string;
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
	starters: string[];
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
	points: number;
	custom_points: number | null;
	players: Player[];
	starters: Player[];
	player_points: Record<string, number>;
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
