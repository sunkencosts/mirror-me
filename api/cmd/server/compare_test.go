package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sunkencosts/mirrorleague/internal/provider"
)

// Scenario set A — compare hardening (SCORING_LEADERBOARD_PLAN.md §5).

func decodeCompare(t *testing.T, resp *http.Response) provider.CompareResponse {
	t.Helper()
	var result provider.CompareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode compare: %v", err)
	}
	return result
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// seedOneLineup inserts a single lineup via a fresh pool (for cases the canonical world
// doesn't cover, like a departed-player lineup).
func seedOneLineup(t *testing.T, userID, leagueID string, rosterID, week int, season string, starters []string) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("seedOneLineup: connect: %v", err)
	}
	defer pool.Close()
	seedLineup(t, pool, userID, leagueID, rosterID, week, season, starters)
}

func TestCompare_UsesJWTUserIgnoringQueryParam(t *testing.T) {
	w := buildWorld()
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_WEEK": "5"})
	seedWorld(t, w)

	// Signed in as u1 (who has a roster-1 lineup) but passing ?user_id=u2 (who does not).
	// The handler must use the JWT subject (u1) and ignore the query param.
	token := signTestJWT(uid(1), "u1@test.example", username(1))
	resp, err := authedGet(token, baseURL+"/league/"+leagueStd+"/week/1/roster/1/compare?user_id="+uid(2))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (u1's lineup graded), got %d", resp.StatusCode)
	}
	result := decodeCompare(t, resp)
	if result.User.TotalPoints != 150 {
		t.Errorf("expected u1's user total 150, got %v", result.User.TotalPoints)
	}
}

func TestCompare_EfficiencyAndEdge(t *testing.T) {
	w := buildWorld()
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_WEEK": "5"})
	seedWorld(t, w)

	token := signTestJWT(uid(1), "u1@test.example", username(1))
	resp, err := authedGet(token, baseURL+"/league/"+leagueStd+"/week/1/roster/1/compare")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	result := decodeCompare(t, resp)

	// u1 played the optimal lineup (150); manager played 141; optimal is 150.
	if result.OptimalPoints != 150 {
		t.Errorf("expected optimal 150, got %v", result.OptimalPoints)
	}
	if !near(result.UserEfficiency, 1.0) {
		t.Errorf("expected user efficiency 1.0, got %v", result.UserEfficiency)
	}
	if !near(result.OfficialEfficiency, 141.0/150.0) {
		t.Errorf("expected official efficiency %v, got %v", 141.0/150.0, result.OfficialEfficiency)
	}
	if !near(result.Edge, 1.0-141.0/150.0) {
		t.Errorf("expected edge %v, got %v", 1.0-141.0/150.0, result.Edge)
	}
	if result.Winner != "user" {
		t.Errorf("expected winner user, got %q", result.Winner)
	}
}

func TestCompare_PastWeekIsFinal(t *testing.T) {
	w := buildWorld()
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_WEEK": "5"})
	seedWorld(t, w)

	token := signTestJWT(uid(1), "u1@test.example", username(1))
	resp, err := authedGet(token, baseURL+"/league/"+leagueStd+"/week/1/roster/1/compare")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if result := decodeCompare(t, resp); !result.Final {
		t.Error("week 1 with CURRENT_WEEK 5 should be final")
	}
}

func TestCompare_LiveWeekIsNotFinal(t *testing.T) {
	w := buildWorld()
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_WEEK": "5"})
	seedWorld(t, w)

	// u1 has a week-5 lineup; week 5 == CURRENT_WEEK → live → not final.
	token := signTestJWT(uid(1), "u1@test.example", username(1))
	resp, err := authedGet(token, baseURL+"/league/"+leagueStd+"/week/5/roster/1/compare")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if result := decodeCompare(t, resp); result.Final {
		t.Error("week 5 with CURRENT_WEEK 5 should NOT be final")
	}
}

func TestCompare_DepartedStarterScoresZeroNo500(t *testing.T) {
	w := buildWorld()
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_WEEK": "5"})

	// A lineup whose 9th starter (999) is not on the roster at grade time → that slot
	// scores 0, the rest score normally, and there is no 500. 150 optimal − DEF 811(12) = 138.
	departedUser := uid(50)
	seedOneLineup(t, departedUser, leagueStd, 1, 1, "2026",
		[]string{"111", "333", "334", "222", "555", "444", "335", "711", "999"})

	token := signTestJWT(departedUser, "departed@test.example", "departed_user")
	resp, err := authedGet(token, baseURL+"/league/"+leagueStd+"/week/1/roster/1/compare")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (no 500), got %d", resp.StatusCode)
	}
	if result := decodeCompare(t, resp); result.User.TotalPoints != 138 {
		t.Errorf("expected user total 138 (999 scores 0), got %v", result.User.TotalPoints)
	}
}
