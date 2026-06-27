package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sunkencosts/mirrorleague/internal/provider"
)

// Weekly Results Browser (WEEKLY_RESULTS_DESIGN.md): per (league, week, roster) setter
// standings (endpoint A, DB-only) + one setter's scored lineup (endpoint B, public compare).
// Each top-level test boots ONE server and runs subtests against it (the suite shares a
// single Postgres instance, so we don't boot a server per assertion).

func getWeeklyResults(t *testing.T, url string) provider.WeeklyRosterResults {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("weekly results request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out provider.WeeklyRosterResults
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode weekly results: %v", err)
	}
	return out
}

// seedSetter inserts a user + their graded week_results row for one (league, week, roster).
func seedSetter(t *testing.T, userID, uname, leagueID string, rosterID, week int, season string, userTotal, officialTotal, optimalTotal float64, result string) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("seedSetter: connect: %v", err)
	}
	defer pool.Close()
	seedUser(t, pool, userID, uname)
	_, err = pool.Exec(context.Background(),
		`INSERT INTO week_results (user_id, league_id, roster_id, week, season, user_total, official_total, optimal_total, result)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (user_id, league_id, roster_id, week, season)
		 DO UPDATE SET user_total=EXCLUDED.user_total, official_total=EXCLUDED.official_total,
		               optimal_total=EXCLUDED.optimal_total, result=EXCLUDED.result`,
		userID, leagueID, rosterID, week, season, userTotal, officialTotal, optimalTotal, result)
	if err != nil {
		t.Fatalf("seedSetter %s: %v", uname, err)
	}
}

// --- endpoint A: GET /league/{id}/week/{week}/results?roster_id=&season=&q=&limit=&offset=
func TestWeeklyResults(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	// Three setters on (wl, week 3, roster 7), efficiencies .95 / .80 / .90 over a shared
	// official 85 / optimal 100 baseline...
	seedSetter(t, uid(21), "alpha", "wl", 7, 3, "2026", 95, 85, 100, "user")
	seedSetter(t, uid(22), "bravo", "wl", 7, 3, "2026", 80, 85, 100, "official")
	seedSetter(t, uid(23), "charlie", "wl", 7, 3, "2026", 90, 85, 100, "user")
	// ...plus noise that must never appear on (roster 7, week 3, 2026):
	seedSetter(t, uid(24), "delta", "wl", 8, 3, "2026", 99, 85, 100, "user") // other roster
	seedSetter(t, uid(25), "echo", "wl", 7, 3, "2025", 99, 85, 100, "user")  // other season

	const base = "/league/wl/week/3/results?roster_id=7&season=2026"

	t.Run("RanksByEfficiency", func(t *testing.T) {
		res := getWeeklyResults(t, baseURL+base)
		if len(res.Setters) != 3 {
			t.Fatalf("expected 3 setters, got %d", len(res.Setters))
		}
		wantOrder := []string{uid(21), uid(23), uid(22)} // .95, .90, .80
		for i, want := range wantOrder {
			if res.Setters[i].UserID != want {
				t.Errorf("position %d: expected %s, got %s", i, want, res.Setters[i].UserID)
			}
			if res.Setters[i].Rank != i+1 {
				t.Errorf("position %d: expected rank %d, got %d", i, i+1, res.Setters[i].Rank)
			}
		}
	})

	t.Run("Baseline", func(t *testing.T) {
		res := getWeeklyResults(t, baseURL+base)
		if res.OfficialTotal != 85 {
			t.Errorf("official_total: want 85, got %v", res.OfficialTotal)
		}
		if res.OptimalTotal != 100 {
			t.Errorf("optimal_total: want 100, got %v", res.OptimalTotal)
		}
		if !near(res.OfficialEfficiency, 0.85) {
			t.Errorf("official_efficiency: want .85, got %v", res.OfficialEfficiency)
		}
		if res.SetterCount != 3 {
			t.Errorf("setter_count: want 3, got %d", res.SetterCount)
		}
		if res.BeatOfficialCount != 2 { // alpha (95) + charlie (90) > official 85; bravo (80) did not
			t.Errorf("beat_official_count: want 2, got %d", res.BeatOfficialCount)
		}
		if !near(res.Setters[0].Edge, 0.10) { // alpha: .95 - .85
			t.Errorf("alpha edge: want +.10, got %v", res.Setters[0].Edge)
		}
	})

	t.Run("SearchByUsername", func(t *testing.T) {
		res := getWeeklyResults(t, baseURL+base+"&q=brav")
		if len(res.Setters) != 1 || res.Setters[0].UserID != uid(22) {
			t.Fatalf("expected only bravo, got %+v", res.Setters)
		}
		if res.Setters[0].Rank != 3 {
			t.Errorf("bravo's true standing is rank 3, got %d", res.Setters[0].Rank)
		}
		if res.SetterCount != 3 {
			t.Errorf("setter_count should be the unfiltered total 3, got %d", res.SetterCount)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		page1 := getWeeklyResults(t, baseURL+base+"&limit=2&offset=0")
		if len(page1.Setters) != 2 || page1.Setters[0].UserID != uid(21) || page1.Setters[1].UserID != uid(23) {
			t.Fatalf("page 1 (limit 2): %+v", page1.Setters)
		}
		if page1.SetterCount != 3 {
			t.Errorf("setter_count: want 3, got %d", page1.SetterCount)
		}
		page2 := getWeeklyResults(t, baseURL+base+"&limit=2&offset=2")
		if len(page2.Setters) != 1 || page2.Setters[0].UserID != uid(22) {
			t.Errorf("page 2: %+v", page2.Setters)
		}
	})

	t.Run("EmptyRoster", func(t *testing.T) {
		res := getWeeklyResults(t, baseURL+"/league/wl/week/3/results?roster_id=999&season=2026")
		if len(res.Setters) != 0 || res.SetterCount != 0 {
			t.Errorf("expected empty roster, got %+v", res)
		}
	})

	t.Run("FilterByRosterAndSeason", func(t *testing.T) {
		res := getWeeklyResults(t, baseURL+base)
		if len(res.Setters) != 3 {
			t.Fatalf("expected 3 (roster 7 / 2026 only), got %d", len(res.Setters))
		}
		for _, s := range res.Setters {
			if s.UserID == uid(24) || s.UserID == uid(25) {
				t.Errorf("leaked other roster/season row: %s", s.UserID)
			}
		}
	})
}

// --- endpoint B: GET /league/{id}/week/{week}/roster/{rosterId}/lineup?user_id= (public)
func TestSetterLineup(t *testing.T) {
	baseURL, _ := gradedWorld(t)
	const lineupURL = "/league/" + leagueStd + "/week/1/roster/1/lineup"

	t.Run("ScoredUserAndOfficial", func(t *testing.T) {
		resp, err := http.Get(baseURL + lineupURL + "?user_id=" + uid(1))
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		res := decodeCompare(t, resp)
		if len(res.User.Starters) == 0 {
			t.Error("expected user starters")
		}
		if len(res.Official.Starters) == 0 {
			t.Error("expected official starters")
		}
		if res.User.TotalPoints <= 0 {
			t.Error("expected user total > 0")
		}
		if res.Winner == "" {
			t.Error("expected a winner")
		}
		if !res.Final {
			t.Error("week 1 < current week 5 should be final")
		}
	})

	t.Run("PublicNoAuth", func(t *testing.T) {
		// No auth cookie/header at all — the per-setter view is public.
		resp, err := http.Get(baseURL + lineupURL + "?user_id=" + uid(1))
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("public per-setter lineup should not require auth, got %d", resp.StatusCode)
		}
	})

	t.Run("404NoLineup", func(t *testing.T) {
		resp, err := http.Get(baseURL + lineupURL + "?user_id=" + uid(77))
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 for a user with no lineup, got %d", resp.StatusCode)
		}
	})

	t.Run("MissingUserID", func(t *testing.T) {
		resp, err := http.Get(baseURL + lineupURL)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 when user_id missing, got %d", resp.StatusCode)
		}
	})

	// The public per-setter view must equal the user's own authed compare for the same week.
	t.Run("MatchesAuthedCompare", func(t *testing.T) {
		pubResp, err := http.Get(baseURL + lineupURL + "?user_id=" + uid(1))
		if err != nil {
			t.Fatalf("public request: %v", err)
		}
		defer pubResp.Body.Close()
		pub := decodeCompare(t, pubResp)

		token := signTestJWT(uid(1), "u1@test.example", username(1))
		cmpResp, err := authedGet(token, baseURL+"/league/"+leagueStd+"/week/1/roster/1/compare")
		if err != nil {
			t.Fatalf("authed compare request: %v", err)
		}
		defer cmpResp.Body.Close()
		cmp := decodeCompare(t, cmpResp)

		if pub.Winner != cmp.Winner || !near(pub.UserEfficiency, cmp.UserEfficiency) || !near(pub.Edge, cmp.Edge) {
			t.Errorf("public per-setter view != authed compare:\n public=%+v\n compare=%+v", pub, cmp)
		}
	})
}
