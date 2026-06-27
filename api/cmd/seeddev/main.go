// Command seeddev builds comprehensive, deterministic dev test data: a full season's worth
// of weekly matchups, submitted lineups, and graded results for a field of managers who
// mirror a real league's rosters. It simulates "what the database looks like after 8 weeks."
//
// Unlike a pure-SQL seed, it derives week_results by running the REAL grader over seeded
// week_matchups + lineups, so the weekly-results LIST and the per-setter lineup DRILL-DOWN
// are guaranteed consistent (one scorer, one source of truth) and every result is clickable.
//
// It fetches the target league + rosters from Sleeper ONCE (so roster IDs/players match the
// live Teams page), caches that league shape in our DB (leg pinned to currentWeek so the UI
// treats weeks 1..8 as final), then grades fully offline against our own cache tables.
//
// Run via `make seed-dev` (after `make seed-players`), or directly:
//
//	cd api && go run ./cmd/seeddev
package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"os"
	"slices"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sunkencosts/mirrorleague/internal/db"
	"github.com/sunkencosts/mirrorleague/internal/grading"
	"github.com/sunkencosts/mirrorleague/internal/provider"
	"github.com/sunkencosts/mirrorleague/internal/sleeper"
	"github.com/sunkencosts/mirrorleague/pkg/config"
)

// devLeagueID is the real Sleeper league mirrored in dev (the dev user's bookmarked league).
// Its league shape + rosters are fetched live once; the competitive data layered on top is
// synthetic but consistent.
const (
	devLeagueID    = "1182073403987832832"
	devLeagueLabel = "The Mirror Keepers"
	season         = "2026"
	weeksPlayed    = 8
	seededLeg      = weeksPlayed + 1 // currentWeek = 9 → weeks 1..8 are final/graded
)

// devUser is one seeded account. The first id matches the fixed subject minted by /dev/login,
// so the dev-login session "is" dev_user and sees its own lineups highlighted.
type devUser struct {
	id       string
	username string
}

var devUsers = []devUser{
	{"00000000-0000-0000-0000-000000000001", "dev_user"},
	{"00000000-0000-0000-0000-0000000000d1", "ana_sharp"},
	{"00000000-0000-0000-0000-0000000000d2", "ben_steady"},
	{"00000000-0000-0000-0000-0000000000d3", "cora_clutch"},
	{"00000000-0000-0000-0000-0000000000d4", "dan_dart"},
	{"00000000-0000-0000-0000-0000000000d5", "evan_eh"},
	{"00000000-0000-0000-0000-0000000000d6", "fiona_fresh"},
	{"00000000-0000-0000-0000-0000000000d7", "gabe_grit"},
	{"00000000-0000-0000-0000-0000000000d8", "hana_heat"},
	{"00000000-0000-0000-0000-0000000000d9", "iggy_iso"},
	{"00000000-0000-0000-0000-0000000000da", "juno_jet"},
	{"00000000-0000-0000-0000-0000000000db", "kai_klutch"},
}

// mirror describes one user mirroring a roster. rosterIdx indexes into the league's sorted
// roster list (0 = first roster = the "popular" team). quality is the setter's skill: 0 starts
// the best available player at each slot (≈ optimal), higher numbers start progressively worse
// players. weeks is how many of weeks 1..8 they submitted (fewer ⇒ provisional on the global
// board). The spread of quality across a roster's setters is what produces the "you set the
// Nth best lineup" ranking and the "X% of lineups beat the original" headline.
type mirror struct {
	userIdx   int
	rosterIdx int
	quality   int
	weeks     int
}

var mirrors = []mirror{
	// Roster 0 — the popular team (7 setters spanning a full quality range).
	{0, 0, 2, weeksPlayed}, // dev_user (mid)
	{1, 0, 0, weeksPlayed}, // ana_sharp (elite)
	{3, 0, 1, weeksPlayed}, // cora_clutch
	{5, 0, 2, weeksPlayed}, // evan_eh
	{7, 0, 3, weeksPlayed}, // gabe_grit
	{8, 0, 1, weeksPlayed}, // hana_heat
	{9, 0, 4, weeksPlayed}, // iggy_iso (weak)
	// Roster 1 — two setters, one provisional.
	{2, 1, 1, weeksPlayed}, // ben_steady
	{6, 1, 0, 2},           // fiona_fresh (provisional: 2 weeks)
	// Roster 2 — two setters, one provisional.
	{4, 2, 3, weeksPlayed},  // dan_dart (often below the manager)
	{11, 2, 1, 1},           // kai_klutch (provisional: 1 week)
	{10, 2, 2, weeksPlayed}, // juno_jet
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seeddev: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	cfg := config.Load(os.Getenv)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to db: %w", err)
	}
	defer pool.Close()
	store := db.NewStore(pool)

	currentWeek := func(context.Context) int { return seededLeg }
	sleeperClient := sleeper.New(cfg.SleeperBaseURL, store, currentWeek)

	// Fetch the real league shape + rosters once so roster IDs/players match the live app.
	league, err := sleeperClient.GetLeague(ctx, devLeagueID)
	if err != nil {
		return fmt.Errorf("fetching league %s (is the dev stack online?): %w", devLeagueID, err)
	}
	rosters, err := sleeperClient.GetRosters(ctx, devLeagueID)
	if err != nil {
		return fmt.Errorf("fetching rosters for %s: %w", devLeagueID, err)
	}
	if len(rosters) == 0 {
		return fmt.Errorf("league %s returned no rosters", devLeagueID)
	}
	sort.Slice(rosters, func(i, j int) bool { return rosters[i].RosterID < rosters[j].RosterID })

	// Cache the league shape with leg pinned so the UI defaults to the latest graded week and
	// treats weeks 1..8 as final. roster_positions come from the live league (real scoring shape).
	league.Settings.Leg = seededLeg
	league.Season = season
	if err := store.SaveLeague(ctx, league); err != nil {
		return fmt.Errorf("caching league: %w", err)
	}

	if err := seedUsers(ctx, pool); err != nil {
		return err
	}

	// Seed week_matchups for every roster × week (the per-player scores everything derives from).
	for _, roster := range rosters {
		for week := 1; week <= weeksPlayed; week++ {
			matchup := buildMatchup(roster, week)
			if err := store.SaveWeekMatchups(ctx, devLeagueID, week, []provider.WeekMatchup{matchup}); err != nil {
				return fmt.Errorf("saving matchup r%d w%d: %w", roster.RosterID, week, err)
			}
		}
	}

	// Seed each mirror's submitted lineups (a quality-graded legal selection per week).
	lineupCount := 0
	for _, m := range mirrors {
		if m.rosterIdx >= len(rosters) {
			continue // league has fewer rosters than the script assumes; skip gracefully
		}
		roster := rosters[m.rosterIdx]
		user := devUsers[m.userIdx]
		for week := 1; week <= m.weeks; week++ {
			starters := buildStarters(league.RosterPositions, roster.Players, week, m.quality)
			if _, err := store.CreateLineup(ctx, user.id, devLeagueID, season, "sleeper", roster.RosterID, week, starters); err != nil {
				return fmt.Errorf("creating lineup %s r%d w%d: %w", user.username, roster.RosterID, week, err)
			}
			lineupCount++
		}
	}

	// Bookmark the league for the dev user so it shows on My Leagues.
	if _, err := store.SaveUserLeague(ctx, devUsers[0].id, devLeagueID, "sleeper", devLeagueLabel); err != nil {
		return fmt.Errorf("bookmarking league: %w", err)
	}

	// Grade offline: the store-backed provider reads our seeded matchups + cached league, so the
	// real grader writes week_results with no live Sleeper calls — and identical math to the
	// runtime per-setter compare drill-down.
	graded, err := grading.GradeSeason(ctx, store, store, storeProvider{store}, seededLeg)
	if err != nil {
		return fmt.Errorf("grading: %w", err)
	}

	log.Printf("seeddev: %d users, %d rosters, %d lineups, %d graded results",
		len(devUsers), len(rosters), lineupCount, graded)
	return nil
}

// seedUsers upserts the fixed dev accounts with their stable UUIDs (dev_user's id is the one
// /dev/login mints). Raw SQL because we need explicit ids, not generated ones.
func seedUsers(ctx context.Context, pool *pgxpool.Pool) error {
	for _, u := range devUsers {
		_, err := pool.Exec(ctx,
			`INSERT INTO users (id, oauth_provider, oauth_id, email, username)
			 VALUES ($1, 'dev', $2, $3, $4)
			 ON CONFLICT (oauth_provider, oauth_id) DO UPDATE SET username = EXCLUDED.username`,
			u.id, "dev-"+u.username, u.username+"@dev.local", u.username)
		if err != nil {
			return fmt.Errorf("seeding user %s: %w", u.username, err)
		}
	}
	return nil
}

// buildMatchup turns a real roster into one week's cached matchup: every rostered player's
// deterministic score, with the manager's official starters and their summed points.
func buildMatchup(roster provider.Roster, week int) provider.WeekMatchup {
	points := make(map[string]float64, len(roster.Players))
	for _, player := range roster.Players {
		points[player.PlayerID] = playerWeekPoints(player.PlayerID, week)
	}
	var officialTotal float64
	for _, starter := range roster.Starters {
		officialTotal += points[starter.PlayerID]
	}
	return provider.WeekMatchup{
		RosterID:     roster.RosterID,
		MatchupID:    roster.RosterID,
		OwnerID:      roster.OwnerID,
		TeamName:     roster.TeamName,
		Points:       round1(officialTotal),
		Players:      roster.Players,
		Starters:     roster.Starters,
		PlayerPoints: points,
	}
}

// playerWeekPoints is a stable per-(player, week) fantasy score. Pure function of the IDs, so
// the data is identical on every reseed (no random()). The base is the player's "talent"; the
// sine term is weekly variance, so benched players sometimes outscore starters (making optimal
// > official, which is what gives efficiency/edge meaning).
func playerWeekPoints(playerID string, week int) float64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(playerID))
	seed := h.Sum32()
	base := 4 + float64(seed%18)                                // 4..21
	variance := 5 * math.Sin(float64(week)*0.9+float64(seed%7)) // ±5
	return round1(math.Max(0, base+variance))
}

// buildStarters picks a legal lineup for a setter of the given quality. quality 0 greedily
// starts the best available player for each slot (≈ the optimal lineup); higher quality starts
// the q-th best, degrading the lineup so setters spread out in the standings. A small weekly
// wobble shifts the offset so ranks change week to week.
func buildStarters(positions []string, rosterPlayers []provider.Player, week, quality int) []string {
	pointsOf := func(id string) float64 { return playerWeekPoints(id, week) }

	// Process specific slots before FLEX/SUPER_FLEX so flex picks from the leftovers.
	type slot struct {
		name  string
		order int
	}
	slots := make([]slot, 0, len(positions))
	for _, pos := range positions {
		if pos == "BN" || pos == "IR" || pos == "TAXI" {
			continue
		}
		order := 0
		if isFlex(pos) {
			order = 1
		}
		slots = append(slots, slot{pos, order})
	}
	sort.SliceStable(slots, func(i, j int) bool { return slots[i].order < slots[j].order })

	offset := quality + (week % 2) // weekly wobble in the quality offset
	used := map[string]bool{}
	starters := make([]string, 0, len(slots))
	for _, s := range slots {
		candidates := make([]provider.Player, 0)
		for _, player := range rosterPlayers {
			if used[player.PlayerID] || !eligible(s.name, player.FantasyPositions) {
				continue
			}
			candidates = append(candidates, player)
		}
		if len(candidates) == 0 {
			continue
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			pi, pj := pointsOf(candidates[i].PlayerID), pointsOf(candidates[j].PlayerID)
			if pi != pj {
				return pi > pj // best first
			}
			return candidates[i].PlayerID < candidates[j].PlayerID
		})
		idx := offset
		if idx >= len(candidates) {
			idx = len(candidates) - 1
		}
		pick := candidates[idx]
		used[pick.PlayerID] = true
		starters = append(starters, pick.PlayerID)
	}
	return starters
}

func isFlex(pos string) bool {
	switch pos {
	case "FLEX", "SUPER_FLEX", "WRRB_FLEX", "REC_FLEX", "WRRB_WRT", "IDP_FLEX":
		return true
	}
	return false
}

// eligible reports whether a player can fill a roster slot.
func eligible(slot string, playerPositions []string) bool {
	has := func(p string) bool { return slices.Contains(playerPositions, p) }
	switch slot {
	case "FLEX", "WRRB_WRT", "REC_FLEX":
		return has("RB") || has("WR") || has("TE")
	case "WRRB_FLEX":
		return has("RB") || has("WR")
	case "SUPER_FLEX":
		return has("QB") || has("RB") || has("WR") || has("TE")
	default:
		return has(slot)
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// storeProvider adapts the DB cache to grading.MatchupProvider so the grader runs fully
// offline: matchups and league shape both come from tables this seeder just populated.
type storeProvider struct{ store *db.Store }

func (p storeProvider) GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error) {
	matchups, _, err := p.store.GetCachedWeekMatchups(ctx, leagueID, week)
	return matchups, err
}

func (p storeProvider) GetLeague(ctx context.Context, leagueID string) (provider.League, error) {
	league, _, err := p.store.GetCachedLeague(ctx, leagueID)
	return league, err
}
