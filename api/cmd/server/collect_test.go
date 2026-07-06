package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// noopSleeper is a Sleeper stub for tests that never touch the Sleeper API.
func noopSleeper() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}

type visitRow struct {
	visitorID string
	userID    *string
	path      string
	referrer  *string
	country   *string
	isBot     bool
}

// queryVisits returns every row in the visits table, newest first.
func queryVisits(t *testing.T) []visitRow {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("queryVisits: connect: %v", err)
	}
	defer pool.Close()
	rows, err := pool.Query(context.Background(),
		"SELECT visitor_id, user_id, path, referrer, country, is_bot FROM visits ORDER BY id DESC")
	if err != nil {
		t.Fatalf("queryVisits: query: %v", err)
	}
	defer rows.Close()
	var out []visitRow
	for rows.Next() {
		var v visitRow
		if err := rows.Scan(&v.visitorID, &v.userID, &v.path, &v.referrer, &v.country, &v.isBot); err != nil {
			t.Fatalf("queryVisits: scan: %v", err)
		}
		out = append(out, v)
	}
	return out
}

func TestCollectAnonymousInsertsVisit(t *testing.T) {
	baseURL := newTestServer(t, noopSleeper())

	body := `{"path":"/leagues","referrer":"https://google.com","visitor_id":"anon-abc"}`
	resp, err := http.Post(baseURL+"/collect", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	visits := queryVisits(t)
	if len(visits) != 1 {
		t.Fatalf("got %d visits, want 1", len(visits))
	}
	v := visits[0]
	if v.visitorID != "anon-abc" {
		t.Errorf("visitor_id = %q, want anon-abc", v.visitorID)
	}
	if v.userID != nil {
		t.Errorf("user_id = %v, want NULL for anonymous visit", *v.userID)
	}
	if v.path != "/leagues" {
		t.Errorf("path = %q, want /leagues", v.path)
	}
	if v.referrer == nil || *v.referrer != "https://google.com" {
		t.Errorf("referrer = %v, want https://google.com", v.referrer)
	}
	if v.isBot {
		t.Errorf("is_bot = true, want false for a normal request")
	}
}

func TestCollectLoggedInStampsUserID(t *testing.T) {
	baseURL := newTestServer(t, noopSleeper())

	token := signTestJWT(testUserID, "user@test.example", "wise_owl01")
	body := `{"path":"/lineups","visitor_id":"anon-xyz"}`
	req := authedJSONRequest(http.MethodPost, baseURL+"/collect", token, body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	visits := queryVisits(t)
	if len(visits) != 1 {
		t.Fatalf("got %d visits, want 1", len(visits))
	}
	if visits[0].userID == nil || *visits[0].userID != testUserID {
		t.Errorf("user_id = %v, want %s", visits[0].userID, testUserID)
	}
}

func TestCollectFlagsBotUserAgent(t *testing.T) {
	baseURL := newTestServer(t, noopSleeper())

	body := `{"path":"/","visitor_id":"crawler-1"}`
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/collect", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	visits := queryVisits(t)
	if len(visits) != 1 {
		t.Fatalf("got %d visits, want 1", len(visits))
	}
	if !visits[0].isBot {
		t.Errorf("is_bot = false, want true for Googlebot user-agent")
	}
}

func TestCollectStoresCountryFromCloudflareHeader(t *testing.T) {
	baseURL := newTestServer(t, noopSleeper())

	body := `{"path":"/","visitor_id":"anon-geo"}`
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/collect", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CF-IPCountry", "US")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	visits := queryVisits(t)
	if len(visits) != 1 {
		t.Fatalf("got %d visits, want 1", len(visits))
	}
	if visits[0].country == nil || *visits[0].country != "US" {
		t.Errorf("country = %v, want US", visits[0].country)
	}
}

func TestCollectRejectsMissingVisitorID(t *testing.T) {
	baseURL := newTestServer(t, noopSleeper())

	body := `{"path":"/leagues"}`
	resp, err := http.Post(baseURL+"/collect", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if visits := queryVisits(t); len(visits) != 0 {
		t.Errorf("got %d visits, want 0 — nothing should be stored on a bad request", len(visits))
	}
}
