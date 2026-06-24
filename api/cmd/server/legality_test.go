package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

// Scenario set B — lineup legality (SCORING_LEADERBOARD_PLAN.md §6), exercised through
// the real POST/PATCH /lineups endpoints against the fixture world.
//
// All POSTs target week 12: 2026 weeks 12 & 18 are intentionally unseeded in week_locks
// (fail-open unlocked), so the kickoff lock never interferes regardless of wall-clock.

const openWeek = 12

// postLineup posts a lineup authenticated as token and returns the status + body.
func postLineup(t *testing.T, baseURL, token, leagueID string, rosterID, week int, starters []string) (int, string) {
	t.Helper()
	payload := map[string]any{
		"source":      "sleeper",
		"league_id":   leagueID,
		"roster_id":   rosterID,
		"week_number": week,
		"starters":    starters,
	}
	b, _ := json.Marshal(payload)
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/lineups", token, string(b)))
	if err != nil {
		t.Fatalf("postLineup: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// legal roster-1 (league-std) lineup: QB,RB,RB,WR,WR,TE,FLEX(RB 335),K,DEF.
var legalStdStarters = []string{"111", "333", "334", "222", "223", "444", "335", "711", "811"}

func TestCreateLineup_LegalLineupAccepted(t *testing.T) {
	baseURL := newTestServer(t, fakeSleeper(buildWorld()))
	token := signTestJWT(uid(1), "b1@test.example", "b1_user")

	status, body := postLineup(t, baseURL, token, leagueStd, 1, openWeek, legalStdStarters)
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", status, body)
	}
}

func TestCreateLineup_RejectsWrongStarterCount(t *testing.T) {
	baseURL := newTestServer(t, fakeSleeper(buildWorld()))
	token := signTestJWT(uid(2), "b2@test.example", "b2_user")

	t.Run("too few", func(t *testing.T) {
		status, body := postLineup(t, baseURL, token, leagueStd, 1, openWeek, legalStdStarters[:8])
		if status != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", status, body)
		}
	})
	t.Run("too many", func(t *testing.T) {
		status, body := postLineup(t, baseURL, token, leagueStd, 1, openWeek, append([]string{"555"}, legalStdStarters...))
		if status != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", status, body)
		}
	})
}

func TestCreateLineup_RejectsBadPositionMix(t *testing.T) {
	baseURL := newTestServer(t, fakeSleeper(buildWorld()))
	token := signTestJWT(uid(3), "b3@test.example", "b3_user")

	// 4 RB + only 1 WR — can't fill WR×2.
	starters := []string{"111", "333", "334", "335", "336", "444", "222", "711", "811"}
	status, body := postLineup(t, baseURL, token, leagueStd, 1, openWeek, starters)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", status, body)
	}
}

func TestCreateLineup_FlexAbsorbsExtraRB(t *testing.T) {
	baseURL := newTestServer(t, fakeSleeper(buildWorld()))
	token := signTestJWT(uid(4), "b4@test.example", "b4_user")

	// 3 RB (333,334,335) — the third legally fills FLEX.
	starters := []string{"111", "333", "334", "222", "223", "444", "335", "711", "811"}
	status, body := postLineup(t, baseURL, token, leagueStd, 1, openWeek, starters)
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", status, body)
	}
}

func TestCreateLineup_RejectsQBInStandardFlex(t *testing.T) {
	baseURL := newTestServer(t, fakeSleeper(buildWorld()))
	token := signTestJWT(uid(5), "b6@test.example", "b6_user")

	// league-std roster 2 has QBs 112 & 113; starting both forces a QB into FLEX → illegal.
	starters := []string{"112", "335", "336", "224", "225", "445", "113", "712", "812"}
	status, body := postLineup(t, baseURL, token, leagueStd, 2, openWeek, starters)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", status, body)
	}
}

func TestCreateLineup_SuperFlexAllowsSecondQB(t *testing.T) {
	baseURL := newTestServer(t, fakeSleeper(buildWorld()))
	token := signTestJWT(uid(6), "b7@test.example", "b7_user")

	// league-sf roster 1: QB 111 + QB 112 in SUPER_FLEX is legal.
	starters := []string{"111", "333", "334", "222", "223", "444", "112", "711", "811"}
	status, body := postLineup(t, baseURL, token, leagueSF, 1, openWeek, starters)
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", status, body)
	}
}

func TestCreateLineup_RejectsDuplicateStarter(t *testing.T) {
	baseURL := newTestServer(t, fakeSleeper(buildWorld()))
	token := signTestJWT(uid(7), "b8@test.example", "b8_user")

	// 711 twice, DEF slot unfilled.
	starters := []string{"111", "333", "334", "222", "223", "444", "335", "711", "711"}
	status, body := postLineup(t, baseURL, token, leagueStd, 1, openWeek, starters)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", status, body)
	}
}

func TestCreateLineup_RejectsNonRosteredPlayer(t *testing.T) {
	baseURL := newTestServer(t, fakeSleeper(buildWorld()))
	token := signTestJWT(uid(8), "b9@test.example", "b9_user")

	// 999 is not on the roster — membership check rejects before positional checks.
	starters := []string{"111", "333", "334", "222", "223", "444", "335", "711", "999"}
	status, body := postLineup(t, baseURL, token, leagueStd, 1, openWeek, starters)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", status, body)
	}
}

func TestCreateLineup_RejectsMissingDefense(t *testing.T) {
	baseURL := newTestServer(t, fakeSleeper(buildWorld()))
	token := signTestJWT(uid(9), "b11@test.example", "b11_user")

	// 555 (WR) where the DEF should be — no DEF in the lineup.
	starters := []string{"111", "333", "334", "222", "223", "444", "335", "711", "555"}
	status, body := postLineup(t, baseURL, token, leagueStd, 1, openWeek, starters)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", status, body)
	}
}

func TestCreateLineup_SkipsValidationWhenNoMatchupData(t *testing.T) {
	// A league the world doesn't know → empty matchups → fail-open skip (D17/B12).
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/league/empty-league":
			json.NewEncoder(w).Encode(map[string]any{
				"league_id":        "empty-league",
				"name":             "Empty",
				"season":           "2099",
				"roster_positions": []string{"QB", "RB", "RB", "WR", "WR", "TE", "FLEX", "K", "DEF", "BN"},
			})
		default:
			// matchups / rosters / users all empty.
			fmt.Fprint(w, "[]")
		}
	})
	baseURL := newTestServer(t, handler)
	token := signTestJWT(uid(10), "b12@test.example", "b12_user")

	// Wrong count, but no matchup data published → validation skipped → 201.
	status, body := postLineup(t, baseURL, token, "empty-league", 1, 1, []string{"111", "222"})
	if status != http.StatusCreated {
		t.Fatalf("expected 201 (validation skipped), got %d (%s)", status, body)
	}
}

func TestUpdateLineup_RejectsIllegalAndKeepsExisting(t *testing.T) {
	baseURL := newTestServer(t, fakeSleeper(buildWorld()))
	token := signTestJWT(uid(11), "b13@test.example", "b13_user")

	// Create a legal lineup, capture its id.
	status, body := postLineup(t, baseURL, token, leagueStd, 1, openWeek, legalStdStarters)
	if status != http.StatusCreated {
		t.Fatalf("setup create expected 201, got %d (%s)", status, body)
	}
	var created provider.Lineup
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	// PATCH with an illegal lineup (too few) → 400.
	patch := `{"starters":["111","222"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/lineups/"+created.ID, token, patch))
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on illegal PATCH, got %d", resp.StatusCode)
	}

	// The stored lineup must be unchanged.
	getResp, err := http.Get(baseURL + "/lineups/" + created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer getResp.Body.Close()
	var after provider.Lineup
	if err := json.NewDecoder(getResp.Body).Decode(&after); err != nil {
		t.Fatalf("decode after: %v", err)
	}
	if len(after.Starters) != len(legalStdStarters) {
		t.Errorf("expected unchanged %d starters, got %d", len(legalStdStarters), len(after.Starters))
	}
}
