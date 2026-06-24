package scoring

// Result values for a head-to-head week (the per-week record; not the leaderboard sort).
const (
	ResultUser     = "user"
	ResultOfficial = "official"
	ResultTie      = "tie"
)

// GradeInput is everything needed to grade one (user, roster, week) head-to-head.
type GradeInput struct {
	RosterPositions []string            // league roster_positions (incl BN)
	RosterPlayers   []string            // all rostered player IDs (for the optimal denominator)
	UserStarters    []string            // the user's submitted starters
	OfficialTotal   float64             // Sleeper-authoritative official total: custom_points else points
	Points          map[string]float64  // playerID -> weekly points (missing == 0, D12)
	PlayerPositions map[string][]string // playerID -> fantasy positions
	Week            int
	CurrentWeek     int
}

// WeekGrade is the scored outcome of one head-to-head week.
type WeekGrade struct {
	UserTotal          float64 `json:"user_total"`
	OfficialTotal      float64 `json:"official_total"`
	OptimalTotal       float64 `json:"optimal_total"`
	Result             string  `json:"result"`              // user | official | tie
	UserEfficiency     float64 `json:"user_efficiency"`     // 0 when optimal is 0 (excluded)
	OfficialEfficiency float64 `json:"official_efficiency"` // 0 when optimal is 0
	Edge               float64 `json:"edge"`                // user_efficiency - official_efficiency (D27)
	Final              bool    `json:"final"`               // week < CurrentWeek (D9)
}

// GradeWeek scores the user's lineup against the manager's, against the roster's optimal
// lineup. A started player not in Points (e.g. traded off the roster before kickoff)
// contributes 0 (D12). Efficiency is undefined when optimal is 0, surfaced as 0 so the
// caller can exclude the week (H5).
func GradeWeek(in GradeInput) WeekGrade {
	userTotal := sumPoints(in.UserStarters, in.Points)
	officialTotal := in.OfficialTotal
	_, optimalTotal := OptimalLineup(in.RosterPositions, in.RosterPlayers, in.Points, in.PlayerPositions)

	g := WeekGrade{
		UserTotal:     userTotal,
		OfficialTotal: officialTotal,
		OptimalTotal:  optimalTotal,
		Final:         in.Week < in.CurrentWeek,
	}

	if optimalTotal > 0 {
		g.UserEfficiency = clamp01(userTotal / optimalTotal)
		g.OfficialEfficiency = clamp01(officialTotal / optimalTotal)
		g.Edge = g.UserEfficiency - g.OfficialEfficiency
	}

	switch {
	case userTotal > officialTotal:
		g.Result = ResultUser
	case officialTotal > userTotal:
		g.Result = ResultOfficial
	default:
		g.Result = ResultTie
	}
	return g
}

func sumPoints(ids []string, points map[string]float64) float64 {
	var total float64
	for _, id := range ids {
		total += points[id]
	}
	return total
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
