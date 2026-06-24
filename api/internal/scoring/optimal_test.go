package scoring

import (
	"math"
	"slices"
	"testing"
)

// roster-1 (league-std) shape, mirroring the fixture world so the numbers cross-check:
//
//	official starters total = 141, optimal = 150.
var (
	p1Players = []string{"111", "333", "334", "222", "223", "444", "335", "711", "811", "555", "336", "445"}
	p1Pos     = map[string][]string{
		"111": {"QB"},
		"333": {"RB"}, "334": {"RB"}, "335": {"RB"}, "336": {"RB"},
		"222": {"WR"}, "555": {"WR"}, "223": {"WR"},
		"444": {"TE"}, "445": {"TE"},
		"711": {"K"}, "811": {"DEF"},
	}
	p1Pts = map[string]float64{
		"111": 25,
		"333": 20, "334": 16, "335": 14, "336": 12,
		"222": 22, "555": 19, "223": 10,
		"444": 14, "445": 9,
		"711": 8, "811": 12,
	}
	stdPositions     = []string{"QB", "RB", "RB", "WR", "WR", "TE", "FLEX", "K", "DEF", "BN", "BN", "BN"}
	p1Official       = []string{"111", "333", "334", "222", "223", "444", "335", "711", "811"} // 141
	p1OptimalLineup  = []string{"111", "333", "334", "222", "555", "444", "335", "711", "811"} // 150
	p1OptimalPoints  = 150.0
	p1OfficialPoints = 141.0
)

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestOptimalLineup_PicksBestLegalSet(t *testing.T) {
	lineup, total := OptimalLineup(stdPositions, p1Players, p1Pts, p1Pos)
	if !almostEqual(total, p1OptimalPoints) {
		t.Fatalf("expected optimal total %v, got %v", p1OptimalPoints, total)
	}
	if !slices.Contains(lineup, "555") {
		t.Errorf("optimal should start the high benched WR 555, got %v", lineup)
	}
	if slices.Contains(lineup, "223") {
		t.Errorf("optimal should drop the weak WR 223, got %v", lineup)
	}
	if len(lineup) != 9 {
		t.Errorf("expected 9 starters, got %d", len(lineup))
	}
}

// TestOptimalLineup_CorrectUnderFlexNotNaiveGreedy: a broad SUPER_FLEX greedily taking the
// RB would strand the FLEX (which can't take the QB). The correct optimum fills both.
func TestOptimalLineup_CorrectUnderFlexNotNaiveGreedy(t *testing.T) {
	slots := []string{"FLEX", "SUPER_FLEX"}
	players := []string{"rb", "qb"}
	pos := map[string][]string{"rb": {"RB"}, "qb": {"QB"}}
	pts := map[string]float64{"rb": 20, "qb": 19}

	lineup, total := OptimalLineup(slots, players, pts, pos)
	if !almostEqual(total, 39) {
		t.Fatalf("expected 39 (both slots filled), got %v (lineup %v)", total, lineup)
	}
	if len(lineup) != 2 {
		t.Errorf("expected both slots filled, got %v", lineup)
	}
}

func TestOptimalLineup_ExoticSlotReturnsZero(t *testing.T) {
	slots := []string{"QB", "IDP_FLEX"}
	if _, total := OptimalLineup(slots, p1Players, p1Pts, p1Pos); total != 0 {
		t.Fatalf("exotic slots should yield optimal 0, got %v", total)
	}
}

func gradeP1(user []string, week, current int) WeekGrade {
	return GradeWeek(GradeInput{
		RosterPositions: stdPositions,
		RosterPlayers:   p1Players,
		UserStarters:    user,
		OfficialTotal:   p1OfficialPoints,
		Points:          p1Pts,
		PlayerPositions: p1Pos,
		Week:            week,
		CurrentWeek:     current,
	})
}

func TestEfficiency_UserOverOptimal(t *testing.T) {
	g := gradeP1(p1Official, 1, 5) // user replays official: 141/150
	if !almostEqual(g.UserEfficiency, p1OfficialPoints/p1OptimalPoints) {
		t.Fatalf("expected efficiency %v, got %v", p1OfficialPoints/p1OptimalPoints, g.UserEfficiency)
	}
}

func TestEfficiency_PerfectLineupIsOne(t *testing.T) {
	g := gradeP1(p1OptimalLineup, 1, 5)
	if !almostEqual(g.UserEfficiency, 1.0) {
		t.Fatalf("expected efficiency 1.0, got %v", g.UserEfficiency)
	}
}

func TestEfficiency_ZeroOptimalExcludesWeek(t *testing.T) {
	zero := map[string]float64{} // everyone scores 0 → optimal 0
	g := GradeWeek(GradeInput{
		RosterPositions: stdPositions,
		RosterPlayers:   p1Players,
		UserStarters:    p1Official,
		OfficialTotal:   0,
		Points:          zero,
		PlayerPositions: p1Pos,
		Week:            1, CurrentWeek: 5,
	})
	if g.OptimalTotal != 0 || g.UserEfficiency != 0 {
		t.Fatalf("expected optimal 0 / efficiency 0, got %v / %v", g.OptimalTotal, g.UserEfficiency)
	}
}

func TestGradeWeek_DepartedStarterScoresZero(t *testing.T) {
	// User started "999", which has no points (off the roster at kickoff) → contributes 0.
	user := []string{"111", "333", "334", "222", "555", "444", "335", "711", "999"}
	g := gradeP1(user, 1, 5)
	// 150 optimal lineup minus DEF 811(12) replaced by 999(0) = 138.
	if !almostEqual(g.UserTotal, 138) {
		t.Fatalf("expected user total 138 (999 scores 0), got %v", g.UserTotal)
	}
}

func TestEdge_PositiveWhenUserMoreEfficient(t *testing.T) {
	g := gradeP1(p1OptimalLineup, 1, 5) // user 150 vs official 141
	if g.Edge <= 0 {
		t.Fatalf("expected positive edge, got %v", g.Edge)
	}
	if g.Result != ResultUser {
		t.Errorf("expected result user, got %q", g.Result)
	}
}

func TestEdge_NegativeWhenManagerMoreEfficient(t *testing.T) {
	// User leaves points on the bench: weak-but-legal subset (total 123 < official 141).
	// QB,RB(335),RB(336),WR(555),WR(223),TE(445),FLEX(444),K,DEF.
	weak := []string{"111", "335", "336", "555", "223", "445", "444", "711", "811"}
	g := gradeP1(weak, 1, 5)
	if g.Edge >= 0 {
		t.Fatalf("expected negative edge (worse than manager), got %v (user %v vs official %v)", g.Edge, g.UserTotal, g.OfficialTotal)
	}
	if g.Result != ResultOfficial {
		t.Errorf("expected result official, got %q", g.Result)
	}
}

func TestGradeWeek_FinalFlag(t *testing.T) {
	if g := gradeP1(p1Official, 3, 5); !g.Final {
		t.Error("week 3 with CURRENT_WEEK 5 should be final")
	}
	if g := gradeP1(p1Official, 5, 5); g.Final {
		t.Error("week 5 with CURRENT_WEEK 5 should NOT be final")
	}
}
