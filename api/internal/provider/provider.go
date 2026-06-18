package provider

import (
	"context"
	"errors"
	"time"
)

var ErrUsernameConflict = errors.New("username already taken")
var ErrLeagueNotFound = errors.New("league not found")

type User struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type AuthUser struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type League struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Metadata struct {
		AutoContinue   string `json:"auto_continue"`
		KeeperDeadline string `json:"keeper_deadline"`
	} `json:"metadata"`
	Settings struct {
		BestBall                 int `json:"best_ball"`
		WaiverBudget             int `json:"waiver_budget"`
		DisableAdds              int `json:"disable_adds"`
		CapacityOverride         int `json:"capacity_override"`
		WaiverBidMin             int `json:"waiver_bid_min"`
		TaxiDeadline             int `json:"taxi_deadline"`
		DraftRounds              int `json:"draft_rounds"`
		ReserveAllowNa           int `json:"reserve_allow_na"`
		StartWeek                int `json:"start_week"`
		PlayoffSeedType          int `json:"playoff_seed_type"`
		PlayoffTeams             int `json:"playoff_teams"`
		VetoVotesNeeded          int `json:"veto_votes_needed"`
		NumTeams                 int `json:"num_teams"`
		DailyWaiversHour         int `json:"daily_waivers_hour"`
		PlayoffType              int `json:"playoff_type"`
		TaxiSlots                int `json:"taxi_slots"`
		SubStartTimeEligibility  int `json:"sub_start_time_eligibility"`
		DailyWaiversDays         int `json:"daily_waivers_days"`
		SubLockIfStarterActive   int `json:"sub_lock_if_starter_active"`
		PlayoffWeekStart         int `json:"playoff_week_start"`
		WaiverClearDays          int `json:"waiver_clear_days"`
		ReserveAllowDoubtful     int `json:"reserve_allow_doubtful"`
		CommissionerDirectInvite int `json:"commissioner_direct_invite"`
		VetoAutoPoll             int `json:"veto_auto_poll"`
		ReserveAllowDnr          int `json:"reserve_allow_dnr"`
		TaxiAllowVets            int `json:"taxi_allow_vets"`
		WaiverDayOfWeek          int `json:"waiver_day_of_week"`
		PlayoffRoundType         int `json:"playoff_round_type"`
		ReserveAllowOut          int `json:"reserve_allow_out"`
		ReserveAllowSus          int `json:"reserve_allow_sus"`
		VetoShowVotes            int `json:"veto_show_votes"`
		TradeDeadline            int `json:"trade_deadline"`
		TaxiYears                int `json:"taxi_years"`
		DailyWaivers             int `json:"daily_waivers"`
		FaabSuggestions          int `json:"faab_suggestions"`
		DisableTrades            int `json:"disable_trades"`
		PickTrading              int `json:"pick_trading"`
		Type                     int `json:"type"`
		MaxKeepers               int `json:"max_keepers"`
		WaiverType               int `json:"waiver_type"`
		MaxSubs                  int `json:"max_subs"`
		LeagueAverageMatch       int `json:"league_average_match"`
		TradeReviewDays          int `json:"trade_review_days"`
		BenchLock                int `json:"bench_lock"`
		OffseasonAdds            int `json:"offseason_adds"`
		Leg                      int `json:"leg"`
		ReserveSlots             int `json:"reserve_slots"`
		ReserveAllowCov          int `json:"reserve_allow_cov"`
		DailyWaiversLastRan      int `json:"daily_waivers_last_ran"`
	} `json:"settings"`
	Avatar          string      `json:"avatar"`
	CompanyID       interface{} `json:"company_id"`
	LastMessageID   string      `json:"last_message_id"`
	Shard           int         `json:"shard"`
	Season          string      `json:"season"`
	SeasonType      string      `json:"season_type"`
	Sport           string      `json:"sport"`
	ScoringSettings struct {
		Sack         float64 `json:"sack"`
		Fgm4049      float64 `json:"fgm_40_49"`
		BonusRecTe   float64 `json:"bonus_rec_te"`
		PassInt      float64 `json:"pass_int"`
		PtsAllow0    float64 `json:"pts_allow_0"`
		Pass2Pt      float64 `json:"pass_2pt"`
		StTd         float64 `json:"st_td"`
		RecTd        float64 `json:"rec_td"`
		IdpBlkKick   float64 `json:"idp_blk_kick"`
		Fgm3039      float64 `json:"fgm_30_39"`
		Xpmiss       float64 `json:"xpmiss"`
		RushTd       float64 `json:"rush_td"`
		IdpTkl       float64 `json:"idp_tkl"`
		DefStTklSolo float64 `json:"def_st_tkl_solo"`
		Rec2Pt       float64 `json:"rec_2pt"`
		IdpTklLoss   float64 `json:"idp_tkl_loss"`
		IdpTklSolo   float64 `json:"idp_tkl_solo"`
		StFumRec     float64 `json:"st_fum_rec"`
		Fgmiss       float64 `json:"fgmiss"`
		Ff           float64 `json:"ff"`
		IdpInt       float64 `json:"idp_int"`
		Rec          float64 `json:"rec"`
		IdpSafe      float64 `json:"idp_safe"`
		PtsAllow1420 float64 `json:"pts_allow_14_20"`
		Fgm019       float64 `json:"fgm_0_19"`
		IdpDefTd     float64 `json:"idp_def_td"`
		Int          float64 `json:"int"`
		DefStFumRec  float64 `json:"def_st_fum_rec"`
		FumLost      float64 `json:"fum_lost"`
		PtsAllow16   float64 `json:"pts_allow_1_6"`
		RecFd        float64 `json:"rec_fd"`
		StTklSolo    float64 `json:"st_tkl_solo"`
		IdpSack      float64 `json:"idp_sack"`
		Fgm2029      float64 `json:"fgm_20_29"`
		PtsAllow2127 float64 `json:"pts_allow_21_27"`
		Xpm          float64 `json:"xpm"`
		Rush2Pt      float64 `json:"rush_2pt"`
		FumRec       float64 `json:"fum_rec"`
		IdpPassDef   float64 `json:"idp_pass_def"`
		DefStTd      float64 `json:"def_st_td"`
		Fgm50P       float64 `json:"fgm_50p"`
		DefTd        float64 `json:"def_td"`
		IdpFumRec    float64 `json:"idp_fum_rec"`
		Safe         float64 `json:"safe"`
		PassYd       float64 `json:"pass_yd"`
		BlkKick      float64 `json:"blk_kick"`
		PassTd       float64 `json:"pass_td"`
		IdpQbHit     float64 `json:"idp_qb_hit"`
		RushYd       float64 `json:"rush_yd"`
		Fum          float64 `json:"fum"`
		PtsAllow2834 float64 `json:"pts_allow_28_34"`
		PtsAllow35P  float64 `json:"pts_allow_35p"`
		FumRecTd     float64 `json:"fum_rec_td"`
		RecYd        float64 `json:"rec_yd"`
		DefStFf      float64 `json:"def_st_ff"`
		PtsAllow713  float64 `json:"pts_allow_7_13"`
		IdpFf        float64 `json:"idp_ff"`
		StFf         float64 `json:"st_ff"`
		IdpTklAst    float64 `json:"idp_tkl_ast"`
	} `json:"scoring_settings"`
	LastAuthorAvatar        string      `json:"last_author_avatar"`
	LastAuthorDisplayName   string      `json:"last_author_display_name"`
	LastAuthorID            string      `json:"last_author_id"`
	LastAuthorIsBot         bool        `json:"last_author_is_bot"`
	LastMessageAttachment   interface{} `json:"last_message_attachment"`
	LastMessageTextMap      interface{} `json:"last_message_text_map"`
	LastMessageTime         int64       `json:"last_message_time"`
	LastPinnedMessageID     interface{} `json:"last_pinned_message_id"`
	LastReadID              interface{} `json:"last_read_id"`
	DraftID                 string      `json:"draft_id"`
	LeagueID                string      `json:"league_id"`
	PreviousLeagueID        string      `json:"previous_league_id"`
	RosterPositions         []string    `json:"roster_positions"`
	BracketID               interface{} `json:"bracket_id"`
	BracketOverridesID      interface{} `json:"bracket_overrides_id"`
	GroupID                 interface{} `json:"group_id"`
	LoserBracketID          interface{} `json:"loser_bracket_id"`
	LoserBracketOverridesID interface{} `json:"loser_bracket_overrides_id"`
	TotalRosters            int         `json:"total_rosters"`
}

type Player struct {
	PlayerID         string   `json:"player_id"`
	FirstName        string   `json:"first_name"`
	LastName         string   `json:"last_name"`
	Number           int      `json:"number"`
	Age              int      `json:"age"`
	Team             string   `json:"team"`
	Active           bool     `json:"active"`
	FantasyPositions []string `json:"fantasy_positions"`
	ImageURL         string   `json:"image_url"`
	Rarity           string   `json:"rarity"`
}

type SlimPlayer struct {
	PlayerID         string   `json:"player_id"`
	FirstName        string   `json:"first_name"`
	LastName         string   `json:"last_name"`
	Team             string   `json:"team"`
	FantasyPositions []string `json:"fantasy_positions"`
	ImageURL         string   `json:"image_url"`
	Rarity           string   `json:"rarity"`
}

type Roster struct {
	RosterID int      `json:"roster_id"`
	OwnerID  string   `json:"owner_id"`
	TeamName string   `json:"team_name"`
	Players  []Player `json:"players"`
	Starters []Player `json:"starters"`
	Reserve  []Player `json:"reserve"`
	Taxi     []Player `json:"taxi"`
}
type WeekMatchup struct {
	RosterID     int                `json:"roster_id"`
	MatchupID    int                `json:"matchup_id"`
	OwnerID      string             `json:"owner_id"`
	TeamName     string             `json:"team_name"`
	Points       float64            `json:"points"`
	CustomPoints *float64           `json:"custom_points"`
	Players      []Player           `json:"players"`
	Starters     []Player           `json:"starters"`
	PlayerPoints map[string]float64 `json:"player_points"`
}

type ScoredPlayer struct {
	Player
	Points float64 `json:"points"`
}

type ScoredLineup struct {
	LineupID    string         `json:"lineup_id,omitempty"`
	Starters    []ScoredPlayer `json:"starters"`
	TotalPoints float64        `json:"total_points"`
}

type CompareResponse struct {
	RosterID int          `json:"roster_id"`
	Week     int          `json:"week"`
	Official ScoredLineup `json:"official"`
	User     ScoredLineup `json:"user"`
	Winner   string       `json:"winner"`
}

type Lineup struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	LeagueID   string     `json:"league_id"`
	RosterID   int        `json:"roster_id"`
	WeekNumber int        `json:"week_number"`
	Season     string     `json:"season"`
	Source     string     `json:"source"`
	Starters   []string   `json:"starters"`
	Locked     bool       `json:"locked"`
	LocksAt    *time.Time `json:"locks_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// WeekMatchupsResponse envelopes the week's matchups together with the lock state
// for that (season, week). It is the lineup editor's source of truth for whether
// edits are still allowed. Locked is true once now() >= LocksAt; LocksAt is nil when
// no week_locks row has been seeded (fail open — not locked).
type WeekMatchupsResponse struct {
	Locked   bool          `json:"locked"`
	LocksAt  *time.Time    `json:"locks_at,omitempty"`
	Matchups []WeekMatchup `json:"matchups"`
}
type UserLeague struct {
	UserID    string    `json:"user_id"`
	LeagueID  string    `json:"league_id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IconURL   string    `json:"icon_url"`
	Source    string    `json:"source"`
}

type Provider interface {
	GetRosters(ctx context.Context, leagueID string) ([]Roster, error)
	GetLeague(ctx context.Context, leagueID string) (League, error)
}
