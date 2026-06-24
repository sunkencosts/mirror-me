package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sunkencosts/mirrorleague/internal/provider"
)

// Scenario sets E (global) + F (per-league) leaderboards.

func getLeaderboard(t *testing.T, url string) []provider.LeaderboardRow {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("leaderboard request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var rows []provider.LeaderboardRow
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatalf("decode leaderboard: %v", err)
	}
	return rows
}

func findRow(rows []provider.LeaderboardRow, userID string) (provider.LeaderboardRow, int) {
	count := 0
	var found provider.LeaderboardRow
	for _, r := range rows {
		if r.UserID == userID {
			found = r
			count++
		}
	}
	return found, count
}

// gradedWorld boots the server, seeds the canonical world, and runs grading so the
// week_results table is populated for the leaderboard to aggregate.
func gradedWorld(t *testing.T) (string, world) {
	t.Helper()
	w := buildWorld()
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_WEEK": "5", "CURRENT_SEASON": "2026"})
	seedWorld(t, w)
	runGrading(t, baseURL)
	return baseURL, w
}

func TestGlobalLeaderboard_OneRowPerUser(t *testing.T) {
	baseURL, _ := gradedWorld(t)
	rows := getLeaderboard(t, baseURL+"/leaderboard?season=2026")

	// u9 mirrors two leagues but must appear exactly once.
	if _, count := findRow(rows, uid(9)); count != 1 {
		t.Fatalf("expected u9 once, found %d times", count)
	}
}

func TestGlobalLeaderboard_RankedByMeanEfficiency(t *testing.T) {
	baseURL, _ := gradedWorld(t)
	rows := getLeaderboard(t, baseURL+"/leaderboard?season=2026")

	prev := 2.0
	for _, r := range rows {
		if r.Provisional {
			continue // provisional rows trail the ranked ones
		}
		if r.MeanEfficiency > prev+1e-9 {
			t.Errorf("ranked rows not sorted by mean efficiency desc: %v after %v", r.MeanEfficiency, prev)
		}
		prev = r.MeanEfficiency
	}
}

func TestGlobalLeaderboard_PooledEfficiencyEqualWeekWeight(t *testing.T) {
	baseURL, _ := gradedWorld(t)
	rows := getLeaderboard(t, baseURL+"/leaderboard?season=2026")

	// u9 = 4 weeks on league-std r1 + 4 on league-sf r1 = 8 pooled graded weeks.
	row, count := findRow(rows, uid(9))
	if count != 1 {
		t.Fatalf("expected u9 once, got %d", count)
	}
	if row.WeeksPlayed != 8 {
		t.Errorf("expected u9 weeks_played 8 (pooled across mirrors), got %d", row.WeeksPlayed)
	}
}

func TestGlobalLeaderboard_ProvisionalBelowThreeWeeks(t *testing.T) {
	baseURL, _ := gradedWorld(t)
	rows := getLeaderboard(t, baseURL+"/leaderboard?season=2026")

	// u12 has 1 graded week → provisional, Rank 0, and after every ranked user.
	row, count := findRow(rows, uid(12))
	if count != 1 {
		t.Fatalf("expected u12 once, got %d", count)
	}
	if !row.Provisional || row.Rank != 0 {
		t.Errorf("expected u12 provisional with rank 0, got provisional=%v rank=%d", row.Provisional, row.Rank)
	}
	// A 3-week user (u3) must be ranked, not provisional.
	u3, _ := findRow(rows, uid(3))
	if u3.Provisional {
		t.Errorf("u3 has 3 graded weeks and should be ranked, not provisional")
	}
}

func TestGlobalLeaderboard_RowReturnsEdgeAndWeeksPlayed(t *testing.T) {
	baseURL, _ := gradedWorld(t)
	rows := getLeaderboard(t, baseURL+"/leaderboard?season=2026")

	// u1 played the optimal lineup every graded week → positive edge, 4 weeks, rank set.
	u1, _ := findRow(rows, uid(1))
	if u1.WeeksPlayed != 4 {
		t.Errorf("expected u1 weeks_played 4, got %d", u1.WeeksPlayed)
	}
	if u1.Edge <= 0 {
		t.Errorf("expected u1 positive edge, got %v", u1.Edge)
	}
	if u1.Rank == 0 {
		t.Errorf("expected u1 to be ranked")
	}
}

func TestGlobalLeaderboard_CurrentSeasonOnly(t *testing.T) {
	baseURL, _ := gradedWorld(t)

	// Inject a 2025 result for u1; the 2026 board must ignore it (u1 stays at 4 weeks).
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	_, err = pool.Exec(context.Background(),
		`INSERT INTO week_results (user_id, league_id, roster_id, week, season, user_total, official_total, optimal_total, result)
		 VALUES ($1, $2, 1, 1, '2025', 100, 90, 100, 'user')`, uid(1), leagueStd)
	if err != nil {
		t.Fatalf("insert 2025 result: %v", err)
	}

	rows := getLeaderboard(t, baseURL+"/leaderboard?season=2026")
	u1, _ := findRow(rows, uid(1))
	if u1.WeeksPlayed != 4 {
		t.Errorf("expected u1 weeks_played 4 (2025 excluded), got %d", u1.WeeksPlayed)
	}
}

func TestGlobalLeaderboard_ExcludesUsersWithoutAccount(t *testing.T) {
	baseURL, _ := gradedWorld(t)

	// A graded result for a user with no users row (anonymous) must not appear (D15).
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	_, err = pool.Exec(context.Background(),
		`INSERT INTO week_results (user_id, league_id, roster_id, week, season, user_total, official_total, optimal_total, result)
		 VALUES ($1, $2, 1, 1, '2026', 100, 90, 100, 'user')`, uid(99), leagueStd)
	if err != nil {
		t.Fatalf("insert anon result: %v", err)
	}

	rows := getLeaderboard(t, baseURL+"/leaderboard?season=2026")
	if _, count := findRow(rows, uid(99)); count != 0 {
		t.Errorf("expected anonymous user (no account) excluded, found %d times", count)
	}
}

func TestLeagueLeaderboard_ScopedToLeagueMembers(t *testing.T) {
	baseURL, _ := gradedWorld(t)
	rows := getLeaderboard(t, baseURL+"/league/"+leagueStd+"/leaderboard?season=2026")

	// u4 only mirrors league-sf → absent from the league-std board.
	if _, count := findRow(rows, uid(4)); count != 0 {
		t.Errorf("expected u4 (league-sf only) absent from league-std board, found %d", count)
	}
	// u1 mirrors league-std → present.
	if _, count := findRow(rows, uid(1)); count != 1 {
		t.Errorf("expected u1 present once on league-std board, got %d", count)
	}
}

func TestLeagueLeaderboard_RanksFromWeekOneNoMinimum(t *testing.T) {
	baseURL, _ := gradedWorld(t)
	rows := getLeaderboard(t, baseURL+"/league/"+leagueSF+"/leaderboard?season=2026")

	// u12 has a single graded week in league-sf — ranked immediately, NOT provisional.
	row, count := findRow(rows, uid(12))
	if count != 1 {
		t.Fatalf("expected u12 on league-sf board once, got %d", count)
	}
	if row.Provisional || row.Rank == 0 {
		t.Errorf("expected u12 ranked from week 1 (no minimum), got provisional=%v rank=%d", row.Provisional, row.Rank)
	}
}

func TestLeagueLeaderboard_EmptyReturns200(t *testing.T) {
	baseURL, _ := gradedWorld(t)
	rows := getLeaderboard(t, baseURL+"/league/nobody-mirrored-this/leaderboard?season=2026")
	if len(rows) != 0 {
		t.Errorf("expected empty board, got %d rows", len(rows))
	}
}

// TestLeaderboards_FullWorldSweep is the headline integration test the plan called for:
// one realistic seeded world (12 users, 2 leagues, multiple lineups), graded, then the
// whole leaderboard asserted at once.
func TestLeaderboards_FullWorldSweep(t *testing.T) {
	baseURL, _ := gradedWorld(t)
	rows := getLeaderboard(t, baseURL+"/leaderboard?season=2026")

	// All 12 seeded users appear exactly once.
	if len(rows) != 12 {
		t.Fatalf("expected 12 rows (one per user), got %d", len(rows))
	}
	for n := 1; n <= 12; n++ {
		if _, count := findRow(rows, uid(n)); count != 1 {
			t.Errorf("expected user %d exactly once, got %d", n, count)
		}
	}

	// Exactly one provisional user (u12, 1 week); the rest are ranked 1..11 in order.
	provisionalCount := 0
	lastRank := 0
	for _, r := range rows {
		if r.Provisional {
			provisionalCount++
			if r.Rank != 0 {
				t.Errorf("provisional row should have rank 0, got %d", r.Rank)
			}
			continue
		}
		if r.Rank != lastRank+1 {
			t.Errorf("ranked rows not contiguous: got rank %d after %d", r.Rank, lastRank)
		}
		lastRank = r.Rank
	}
	if provisionalCount != 1 {
		t.Errorf("expected exactly 1 provisional user (u12), got %d", provisionalCount)
	}

	// u11 pooled two same-league mirrors → 8 weeks, one row.
	if u11, _ := findRow(rows, uid(11)); u11.WeeksPlayed != 8 {
		t.Errorf("expected u11 weeks_played 8 (two same-league mirrors pooled), got %d", u11.WeeksPlayed)
	}
}
