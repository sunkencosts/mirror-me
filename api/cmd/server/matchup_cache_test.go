package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sunkencosts/mirrorleague/internal/db"
	"github.com/sunkencosts/mirrorleague/internal/provider"
)

// countingSleeper is a sleeperDeps that records how many times GetWeekMatchups is called and
// returns a canned set of matchups. It lets the cache tests prove when Sleeper is (not) hit.
type countingSleeper struct {
	matchups    []provider.WeekMatchup
	league      provider.League
	calls       int
	leagueCalls int
	err         error
}

func (c *countingSleeper) GetRosters(ctx context.Context, leagueID string) ([]provider.Roster, error) {
	return nil, nil
}
func (c *countingSleeper) GetLeague(ctx context.Context, leagueID string) (provider.League, error) {
	c.leagueCalls++
	if c.err != nil {
		return provider.League{}, c.err
	}
	return c.league, nil
}
func (c *countingSleeper) InvalidateRosters() {}
func (c *countingSleeper) GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return c.matchups, nil
}

// sampleMatchups is one roster with three started players and their weekly points, built from
// the reference testPlayers so the IDs resolve to real names when read back from the cache.
func sampleMatchups() []provider.WeekMatchup {
	players := []provider.Player{
		{PlayerID: "111"}, {PlayerID: "333"}, {PlayerID: "222"},
	}
	return []provider.WeekMatchup{{
		RosterID: 1, MatchupID: 1, OwnerID: "mgr-1", TeamName: "Alpha", Points: 67,
		Players:      players,
		Starters:     players,
		PlayerPoints: map[string]float64{"111": 25, "333": 20, "222": 22},
	}}
}

// fixedWeek returns a currentWeek resolver pinned to w (so "final" = week < w is deterministic).
func fixedWeek(w int) func(context.Context) int {
	return func(context.Context) int { return w }
}

func cacheTestStore(t *testing.T, leagueID string) *db.Store {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(pool.Close)
	// Isolate this test's league from any rows other tests left behind.
	if _, err := pool.Exec(context.Background(), "DELETE FROM week_matchups WHERE league_id = $1", leagueID); err != nil {
		t.Fatalf("clean week_matchups: %v", err)
	}
	if _, err := pool.Exec(context.Background(), "DELETE FROM leagues WHERE league_id = $1", leagueID); err != nil {
		t.Fatalf("clean leagues: %v", err)
	}
	return db.NewStore(pool)
}

func TestCachingMatchups(t *testing.T) {
	ctx := context.Background()

	t.Run("FinalWeekServedFromCacheWithoutSleeper", func(t *testing.T) {
		const leagueID = "cache-final"
		store := cacheTestStore(t, leagueID)
		// Pre-seed the cache as though week 2 was already fetched and persisted.
		if err := store.SaveWeekMatchups(ctx, leagueID, 2, sampleMatchups()); err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		sleeper := &countingSleeper{matchups: sampleMatchups()}
		caching := newCachingMatchups(sleeper, store, fixedWeek(5)) // week 2 < 5 → final

		got, err := caching.GetWeekMatchups(ctx, leagueID, 2)
		if err != nil {
			t.Fatalf("GetWeekMatchups: %v", err)
		}
		if sleeper.calls != 0 {
			t.Errorf("expected Sleeper to be skipped on cache hit, got %d calls", sleeper.calls)
		}
		if len(got) != 1 || got[0].RosterID != 1 {
			t.Fatalf("unexpected matchups: %+v", got)
		}
		if got[0].PlayerPoints["111"] != 25 {
			t.Errorf("expected player 111 to score 25, got %v", got[0].PlayerPoints["111"])
		}
		// Players are re-resolved from the players table, so names come back populated.
		if len(got[0].Starters) != 3 || got[0].Starters[0].FirstName != "Josh" {
			t.Errorf("expected resolved starters with names, got %+v", got[0].Starters)
		}
	})

	t.Run("FinalWeekCacheMissWritesThrough", func(t *testing.T) {
		const leagueID = "cache-miss"
		store := cacheTestStore(t, leagueID)

		sleeper := &countingSleeper{matchups: sampleMatchups()}
		caching := newCachingMatchups(sleeper, store, fixedWeek(5)) // week 3 < 5 → final

		// First read: cache empty → Sleeper hit once, result persisted.
		if _, err := caching.GetWeekMatchups(ctx, leagueID, 3); err != nil {
			t.Fatalf("first GetWeekMatchups: %v", err)
		}
		if sleeper.calls != 1 {
			t.Fatalf("expected 1 Sleeper call on miss, got %d", sleeper.calls)
		}
		cached, ok, err := store.GetCachedWeekMatchups(ctx, leagueID, 3)
		if err != nil || !ok {
			t.Fatalf("expected write-through to persist matchups (ok=%v, err=%v)", ok, err)
		}
		if len(cached) != 1 || cached[0].PlayerPoints["222"] != 22 {
			t.Errorf("persisted matchups wrong: %+v", cached)
		}

		// Second read: now served from cache, Sleeper not hit again.
		if _, err := caching.GetWeekMatchups(ctx, leagueID, 3); err != nil {
			t.Fatalf("second GetWeekMatchups: %v", err)
		}
		if sleeper.calls != 1 {
			t.Errorf("expected cache hit on second read, got %d Sleeper calls", sleeper.calls)
		}
	})

	t.Run("LiveWeekBypassesCache", func(t *testing.T) {
		const leagueID = "cache-live"
		store := cacheTestStore(t, leagueID)
		// Even with stale data sitting in the cache for week 5...
		if err := store.SaveWeekMatchups(ctx, leagueID, 5, sampleMatchups()); err != nil {
			t.Fatalf("seed cache: %v", err)
		}

		live := []provider.WeekMatchup{{
			RosterID: 1, Points: 99, PlayerPoints: map[string]float64{"111": 30},
		}}
		sleeper := &countingSleeper{matchups: live}
		caching := newCachingMatchups(sleeper, store, fixedWeek(5)) // week 5 == 5 → live, not final

		got, err := caching.GetWeekMatchups(ctx, leagueID, 5)
		if err != nil {
			t.Fatalf("GetWeekMatchups: %v", err)
		}
		if sleeper.calls != 1 {
			t.Errorf("expected live week to always hit Sleeper, got %d calls", sleeper.calls)
		}
		// Returns the live Sleeper data, not the stale cached row.
		if got[0].Points != 99 {
			t.Errorf("expected live points 99, got %v", got[0].Points)
		}
	})

	t.Run("LeagueServedFromCacheWithoutSleeper", func(t *testing.T) {
		const leagueID = "cache-league"
		store := cacheTestStore(t, leagueID)
		positions := []string{"QB", "RB", "RB", "WR", "WR", "TE", "FLEX", "K", "DEF", "BN"}
		if err := store.SaveLeague(ctx, provider.League{
			LeagueID: leagueID, Name: "Cached", Season: "2026", RosterPositions: positions,
		}); err != nil {
			t.Fatalf("seed league: %v", err)
		}

		sleeper := &countingSleeper{}
		caching := newCachingMatchups(sleeper, store, fixedWeek(5))

		got, err := caching.GetLeague(ctx, leagueID)
		if err != nil {
			t.Fatalf("GetLeague: %v", err)
		}
		if sleeper.leagueCalls != 0 {
			t.Errorf("expected Sleeper /league to be skipped on cache hit, got %d calls", sleeper.leagueCalls)
		}
		if len(got.RosterPositions) != len(positions) || got.RosterPositions[6] != "FLEX" {
			t.Errorf("unexpected roster positions: %+v", got.RosterPositions)
		}
	})

	t.Run("LeagueCacheMissWritesThrough", func(t *testing.T) {
		const leagueID = "cache-league-miss"
		store := cacheTestStore(t, leagueID)

		sleeper := &countingSleeper{league: provider.League{
			LeagueID: leagueID, Name: "Live", Season: "2026",
			RosterPositions: []string{"QB", "RB", "WR", "FLEX", "K", "DEF", "BN"},
		}}
		caching := newCachingMatchups(sleeper, store, fixedWeek(5))

		if _, err := caching.GetLeague(ctx, leagueID); err != nil {
			t.Fatalf("first GetLeague: %v", err)
		}
		if sleeper.leagueCalls != 1 {
			t.Fatalf("expected 1 Sleeper call on miss, got %d", sleeper.leagueCalls)
		}
		if _, ok, err := store.GetCachedLeague(ctx, leagueID); err != nil || !ok {
			t.Fatalf("expected league write-through (ok=%v, err=%v)", ok, err)
		}
		if _, err := caching.GetLeague(ctx, leagueID); err != nil {
			t.Fatalf("second GetLeague: %v", err)
		}
		if sleeper.leagueCalls != 1 {
			t.Errorf("expected cache hit on second read, got %d Sleeper calls", sleeper.leagueCalls)
		}
	})

	t.Run("SleeperErrorPropagatesOnMiss", func(t *testing.T) {
		const leagueID = "cache-err"
		store := cacheTestStore(t, leagueID)

		sleeper := &countingSleeper{err: errors.New("sleeper down")}
		caching := newCachingMatchups(sleeper, store, fixedWeek(5))

		if _, err := caching.GetWeekMatchups(ctx, leagueID, 2); err == nil {
			t.Error("expected error to propagate when Sleeper fails on a cache miss")
		}
	})
}
