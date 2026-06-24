package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sunkencosts/mirrorleague/internal/provider"
)

// fixtures_test.go is the shared test-fixture harness for the scoring + leaderboard
// work (see SCORING_LEADERBOARD_PLAN.md). It provides:
//
//   - basePoints / fxPoints: a deterministic per-player weekly scoring table that is the
//     single source of truth for BOTH the fake Sleeper players_points AND the expected
//     math in assertions.
//   - fakeSleeper(world): one programmatic Sleeper mock that serves league / matchups /
//     rosters / users for every league + week in a world (replaces hand-written closures).
//   - seedUser / seedLineup: direct-DB seeders (hybrid strategy — bulk world state is
//     seeded directly; create/legality/lock paths still go through the real endpoints).
//   - buildWorld / seedWorld: the canonical fixture — 12 users, 2 leagues, weeks 1–5.
//
// The points are constant across weeks by default; weekPointOverrides allows a specific
// (week, player) cell to differ when a scenario needs week-to-week variation.

// basePoints is each player's default weekly fantasy points. Values are chosen so that
// several rosters have a benched player who outscores a starter (non-trivial optimal vs.
// actual), which is what makes efficiency/edge assertions meaningful.
var basePoints = map[string]float64{
	// QB
	"111": 25, "112": 22, "113": 20,
	// RB
	"333": 20, "334": 16, "335": 14, "336": 12,
	// WR
	"222": 22, "555": 19, "223": 10, "224": 15, "225": 17,
	// TE
	"444": 14, "445": 9, "446": 11,
	// K
	"711": 8, "712": 10,
	// DEF
	"811": 12, "812": 7,
}

// weekPointOverrides[week][playerID] overrides basePoints for a single (week, player).
// Empty by default; populate in a scenario that needs week-to-week variation.
var weekPointOverrides = map[int]map[string]float64{}

// fxPoints returns a player's points for a given week (override else base).
func fxPoints(playerID string, week int) float64 {
	if wk, ok := weekPointOverrides[week]; ok {
		if v, ok := wk[playerID]; ok {
			return v
		}
	}
	return basePoints[playerID]
}

// --- world model -----------------------------------------------------------

type fxRoster struct {
	rosterID int
	ownerID  string
	teamName string
	players  []string // all rostered player IDs (starters + bench)
	starters []string // the real manager's official starters
}

type fxLeague struct {
	id        string
	season    string
	positions []string // roster_positions (incl BN)
	rosters   []fxRoster
}

type fxUser struct {
	id       string
	username string
}

type fxLineup struct {
	userID   string
	leagueID string
	rosterID int
	week     int
	starters []string
}

type world struct {
	leagues map[string]fxLeague
	users   []fxUser
	lineups []fxLineup
}

// --- canonical world -------------------------------------------------------

func uid(n int) string      { return fmt.Sprintf("00000000-0000-0000-0000-%012d", n) }
func username(n int) string { return fmt.Sprintf("brag_%02d", n) }

const (
	leagueStd = "league-std" // standard: QB,RB,RB,WR,WR,TE,FLEX,K,DEF
	leagueSF  = "league-sf"  // superflex: QB,RB,RB,WR,WR,TE,SUPER_FLEX,K,DEF
)

// improvedStarters holds, per (league, roster), a sit/start improvement over the official
// lineup (starting a higher-scoring benched player). Rosters absent here are already
// optimal, so seeded users simply replay the official lineup (a tie).
var improvedStarters = map[string]map[int][]string{
	leagueStd: {
		1: {"111", "333", "334", "222", "555", "444", "335", "711", "811"}, // 223->555 (optimal, 150)
	},
	leagueSF: {
		1: {"111", "333", "334", "222", "555", "444", "112", "711", "811"}, // 223->555
		2: {"113", "335", "336", "222", "225", "445", "334", "712", "812"}, // 224->222
	},
}

func buildWorld() world {
	stdPos := []string{"QB", "RB", "RB", "WR", "WR", "TE", "FLEX", "K", "DEF", "BN", "BN", "BN"}
	sfPos := []string{"QB", "RB", "RB", "WR", "WR", "TE", "SUPER_FLEX", "K", "DEF", "BN", "BN"}

	leagues := map[string]fxLeague{
		leagueStd: {
			id: leagueStd, season: "2026", positions: stdPos,
			rosters: []fxRoster{
				{1, "mgr-1a", "Alpha", []string{"111", "333", "334", "222", "223", "444", "335", "711", "811", "555", "336", "445"}, []string{"111", "333", "334", "222", "223", "444", "335", "711", "811"}},
				{2, "mgr-1b", "Bravo", []string{"112", "335", "336", "224", "225", "445", "334", "712", "812", "113", "223"}, []string{"112", "335", "336", "224", "225", "445", "334", "712", "812"}},
				{3, "mgr-1c", "Charlie", []string{"113", "333", "334", "222", "555", "446", "335", "711", "811", "225", "336"}, []string{"113", "333", "334", "222", "555", "446", "335", "711", "811"}},
			},
		},
		leagueSF: {
			id: leagueSF, season: "2026", positions: sfPos,
			rosters: []fxRoster{
				{1, "mgr-2a", "Sigma", []string{"111", "112", "333", "334", "222", "223", "444", "711", "811", "113", "555"}, []string{"111", "333", "334", "222", "223", "444", "112", "711", "811"}},
				{2, "mgr-2b", "Tau", []string{"113", "335", "336", "224", "225", "445", "712", "812", "222", "334"}, []string{"113", "335", "336", "224", "225", "445", "334", "712", "812"}},
			},
		},
	}

	users := make([]fxUser, 0, 12)
	for n := 1; n <= 12; n++ {
		users = append(users, fxUser{id: uid(n), username: username(n)})
	}

	// mirror describes one (user → team) mirror and which weeks the user set a lineup.
	// CURRENT_WEEK is 5 in tests, so weeks 1–4 grade and week 5 is the live week.
	type mirror struct {
		user   int
		league string
		roster int
		weeks  []int
	}
	mirrors := []mirror{
		{1, leagueStd, 1, []int{1, 2, 3, 4, 5}}, // incl. live wk 5
		{2, leagueStd, 2, []int{1, 2, 3, 4}},
		{3, leagueStd, 3, []int{1, 3, 4}}, // wk 2 skipped (D8)
		{4, leagueSF, 1, []int{1, 2, 3, 4}},
		{5, leagueSF, 2, []int{1, 2, 3, 4}},
		{6, leagueStd, 1, []int{1, 2, 3, 4}},
		{7, leagueStd, 2, []int{1, 2, 3, 4}},
		{8, leagueSF, 1, []int{1, 2, 3, 4}},
		{9, leagueStd, 1, []int{1, 2, 3, 4}}, // u9 mirrors two leagues (pooled global)
		{9, leagueSF, 1, []int{1, 2, 3, 4}},
		{10, leagueStd, 2, []int{1, 2, 3, 4}}, // u10 mirrors two leagues
		{10, leagueSF, 2, []int{1, 2, 3, 4}},
		{11, leagueStd, 1, []int{1, 2, 3, 4}}, // u11 mirrors two teams in the SAME league (F3)
		{11, leagueStd, 2, []int{1, 2, 3, 4}},
		{12, leagueSF, 1, []int{1}}, // u12 has 1 graded week → provisional on global board
	}

	var lineups []fxLineup
	for _, m := range mirrors {
		starters := startersFor(leagues, m.league, m.roster)
		for _, wk := range m.weeks {
			lineups = append(lineups, fxLineup{
				userID: uid(m.user), leagueID: m.league, rosterID: m.roster, week: wk, starters: starters,
			})
		}
	}

	return world{leagues: leagues, users: users, lineups: lineups}
}

// startersFor returns the improved lineup for a (league, roster) if one exists, else the
// official starters (which means the seeded user ties the manager that week).
func startersFor(leagues map[string]fxLeague, leagueID string, rosterID int) []string {
	if byRoster, ok := improvedStarters[leagueID]; ok {
		if s, ok := byRoster[rosterID]; ok {
			return s
		}
	}
	for _, r := range leagues[leagueID].rosters {
		if r.rosterID == rosterID {
			return r.starters
		}
	}
	return nil
}

// --- fake Sleeper ----------------------------------------------------------

// fakeSleeper serves the Sleeper read API for every league/week in the world. Player
// points come from fxPoints, so the mock and the expected assertions never drift.
func fakeSleeper(w world) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 2 || parts[0] != "league" {
			http.NotFound(rw, r)
			return
		}
		lg, ok := w.leagues[parts[1]]
		if !ok {
			fmt.Fprint(rw, "null")
			return
		}

		switch {
		case len(parts) == 2: // /league/{id}
			json.NewEncoder(rw).Encode(map[string]any{
				"league_id":        lg.id,
				"name":             "League " + lg.id,
				"season":           lg.season,
				"roster_positions": lg.positions,
			})
		case len(parts) == 4 && parts[2] == "matchups": // /league/{id}/matchups/{week}
			week, err := strconv.Atoi(parts[3])
			if err != nil {
				http.Error(rw, "bad week", http.StatusBadRequest)
				return
			}
			out := make([]map[string]any, 0, len(lg.rosters))
			for _, ros := range lg.rosters {
				pp := make(map[string]float64, len(ros.players))
				for _, p := range ros.players {
					pp[p] = fxPoints(p, week)
				}
				var officialPoints float64
				for _, s := range ros.starters {
					officialPoints += fxPoints(s, week)
				}
				out = append(out, map[string]any{
					"roster_id":      ros.rosterID,
					"matchup_id":     ros.rosterID,
					"players":        ros.players,
					"starters":       ros.starters,
					"points":         officialPoints,
					"players_points": pp,
				})
			}
			json.NewEncoder(rw).Encode(out)
		case len(parts) == 3 && parts[2] == "rosters": // /league/{id}/rosters
			out := make([]map[string]any, 0, len(lg.rosters))
			for _, ros := range lg.rosters {
				out = append(out, map[string]any{
					"roster_id": ros.rosterID,
					"owner_id":  ros.ownerID,
					"players":   ros.players,
					"starters":  ros.starters,
				})
			}
			json.NewEncoder(rw).Encode(out)
		case len(parts) == 3 && parts[2] == "users": // /league/{id}/users
			out := make([]map[string]any, 0, len(lg.rosters))
			for _, ros := range lg.rosters {
				out = append(out, map[string]any{
					"user_id":  ros.ownerID,
					"metadata": map[string]string{"team_name": ros.teamName},
				})
			}
			json.NewEncoder(rw).Encode(out)
		default:
			http.NotFound(rw, r)
		}
	})
}

// --- DB seeders (hybrid) ---------------------------------------------------

// seedUser inserts a users row whose id equals the user_id used on that user's lineups,
// so the leaderboard can join usernames. Mirrors how an authenticated user's JWT subject
// is their users.id.
func seedUser(t *testing.T, pool *pgxpool.Pool, id, uname string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, oauth_provider, oauth_id, email, username)
		 VALUES ($1, 'google', $2, $3, $4)
		 ON CONFLICT (oauth_provider, oauth_id) DO NOTHING`,
		id, "seed-"+id, uname+"@test.example", uname)
	if err != nil {
		t.Fatalf("seedUser %s: %v", uname, err)
	}
}

// seedLineup inserts a lineup directly, bypassing the kickoff lock so past-week lineups
// can exist in the fixture world.
func seedLineup(t *testing.T, pool *pgxpool.Pool, userID, leagueID string, rosterID, week int, season string, starters []string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO lineups (user_id, league_id, roster_id, week_number, season, source, starters)
		 VALUES ($1, $2, $3, $4, $5, 'sleeper', $6)
		 ON CONFLICT (user_id, league_id, roster_id, week_number, source)
		 DO UPDATE SET starters = EXCLUDED.starters`,
		userID, leagueID, rosterID, week, season, starters)
	if err != nil {
		t.Fatalf("seedLineup %s/%s/r%d/w%d: %v", userID, leagueID, rosterID, week, err)
	}
}

// seedWorld inserts the world's users and lineups into the (already-truncated) test DB.
// Call it AFTER newTestServer, which truncates users/lineups on boot.
func seedWorld(t *testing.T, w world) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("seedWorld: connect db: %v", err)
	}
	defer pool.Close()

	for _, u := range w.users {
		seedUser(t, pool, u.id, u.username)
	}
	for _, ln := range w.lineups {
		season := w.leagues[ln.leagueID].season
		seedLineup(t, pool, ln.userID, ln.leagueID, ln.rosterID, ln.week, season, ln.starters)
	}
}

// --- validation test (PR 0): the world flows through the real server -------

// TestFixtureWorld_CompareKnownResult proves the fixture harness wires end-to-end: it
// seeds the world, then hits the real compare endpoint for a user whose lineup we know
// (u1 mirrors league-std roster 1 and started 555 over 223), and checks the math.
//
//	official starters total = 141; user (with 555) total = 150; winner = user.
func TestFixtureWorld_CompareKnownResult(t *testing.T) {
	w := buildWorld()
	baseURL := newTestServer(t, fakeSleeper(w), map[string]string{"CURRENT_WEEK": "5"})
	seedWorld(t, w)

	token := signTestJWT(uid(1), "u1@test.example", username(1))
	resp, err := authedGet(token, baseURL+"/league/"+leagueStd+"/week/1/roster/1/compare")
	if err != nil {
		t.Fatalf("compare request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result provider.CompareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode compare: %v", err)
	}
	if result.Official.TotalPoints != 141 {
		t.Errorf("expected official total 141, got %v", result.Official.TotalPoints)
	}
	if result.User.TotalPoints != 150 {
		t.Errorf("expected user total 150, got %v", result.User.TotalPoints)
	}
	if result.Winner != "user" {
		t.Errorf("expected winner %q, got %q", "user", result.Winner)
	}
}
