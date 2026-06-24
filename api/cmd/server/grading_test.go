package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Scenario set G (+ D30) — the grading step writes week_results and backfills.

func runGrading(t *testing.T, baseURL string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/admin/grade", nil)
	req.Header.Set("X-Admin-Secret", testAdminSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("runGrading: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("runGrading: expected 200, got %d", resp.StatusCode)
	}
	var out map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("runGrading decode: %v", err)
	}
	return out["graded"]
}

func countWeekResults(t *testing.T) int {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("countWeekResults: connect: %v", err)
	}
	defer pool.Close()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM week_results").Scan(&n); err != nil {
		t.Fatalf("countWeekResults: %v", err)
	}
	return n
}

// gradableCount returns how many of the world's lineups are for a past week (< current).
func gradableCount(w world, currentWeek int) int {
	n := 0
	for _, ln := range w.lineups {
		if ln.week < currentWeek {
			n++
		}
	}
	return n
}

func TestGradingStep_WritesWeekResultsOnAdvance(t *testing.T) {
	w := buildWorld()
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_WEEK": "5"})
	seedWorld(t, w)

	want := gradableCount(w, 5)
	graded := runGrading(t, baseURL)
	if graded != want {
		t.Fatalf("expected %d graded, got %d", want, graded)
	}
	if got := countWeekResults(t); got != want {
		t.Fatalf("expected %d week_results rows, got %d", want, got)
	}

	// Spot-check a known row: u1, league-std roster 1, week 1 → user 150 / official 141 /
	// optimal 150 / result user.
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	var userTotal, officialTotal, optimalTotal float64
	var result string
	err = pool.QueryRow(context.Background(),
		`SELECT user_total, official_total, optimal_total, result FROM week_results
		 WHERE user_id=$1 AND league_id=$2 AND roster_id=1 AND week=1`, uid(1), leagueStd).
		Scan(&userTotal, &officialTotal, &optimalTotal, &result)
	if err != nil {
		t.Fatalf("query u1 result: %v", err)
	}
	if userTotal != 150 || officialTotal != 141 || optimalTotal != 150 || result != "user" {
		t.Errorf("u1 wk1: got user=%v official=%v optimal=%v result=%q; want 150/141/150/user",
			userTotal, officialTotal, optimalTotal, result)
	}
}

func TestGradingStep_Idempotent(t *testing.T) {
	w := buildWorld()
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_WEEK": "5"})
	seedWorld(t, w)

	first := runGrading(t, baseURL)
	countAfterFirst := countWeekResults(t)
	second := runGrading(t, baseURL)

	if second != 0 {
		t.Errorf("second grading run should write 0 (all already graded), got %d", second)
	}
	if got := countWeekResults(t); got != countAfterFirst {
		t.Errorf("row count changed on re-run: %d -> %d", countAfterFirst, got)
	}
	if first == 0 {
		t.Error("first run should have graded something")
	}
}

func TestGradingStep_BackfillsUngradedPastWeek(t *testing.T) {
	w := buildWorld()
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_WEEK": "5"})
	seedWorld(t, w)

	_ = runGrading(t, baseURL)
	before := countWeekResults(t)

	// A new ungraded past-week lineup appears (e.g. a user who set it late, or a row that
	// failed to grade on an earlier run). A subsequent grading run must pick it up.
	seedOneLineup(t, uid(60), leagueStd, 1, 2, "2026", legalStdStarters)

	graded := runGrading(t, baseURL)
	if graded != 1 {
		t.Fatalf("expected backfill to grade exactly 1, got %d", graded)
	}
	if got := countWeekResults(t); got != before+1 {
		t.Errorf("expected %d rows after backfill, got %d", before+1, got)
	}
}
