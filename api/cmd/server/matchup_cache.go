package main

import (
	"context"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type matchupCacheStore interface {
	GetCachedWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, bool, error)
	SaveWeekMatchups(ctx context.Context, leagueID string, week int, matchups []provider.WeekMatchup) error
	GetCachedLeague(ctx context.Context, leagueID string) (provider.League, bool, error)
	SaveLeague(ctx context.Context, league provider.League) error
}

// cachingMatchups wraps the Sleeper client and serves FINAL-week matchups (week < current
// week) from our own DB — writing through on a cache miss — so repeated reads of past weeks
// don't re-hit Sleeper and risk its rate limit. The live week passes straight through (its
// points still change). It satisfies sleeperDeps via the embedded client; only GetWeekMatchups
// is intercepted. Seeding week_matchups rows lets dev serve real per-player scores with no
// live Sleeper data behind them.
type cachingMatchups struct {
	sleeperDeps
	store       matchupCacheStore
	currentWeek func(context.Context) int
}

func newCachingMatchups(client sleeperDeps, store matchupCacheStore, currentWeek func(context.Context) int) *cachingMatchups {
	return &cachingMatchups{sleeperDeps: client, store: store, currentWeek: currentWeek}
}

func (c *cachingMatchups) GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error) {
	final := week < c.currentWeek(ctx)
	if final {
		if cached, ok, err := c.store.GetCachedWeekMatchups(ctx, leagueID, week); err == nil && ok {
			return cached, nil
		}
	}
	matchups, err := c.sleeperDeps.GetWeekMatchups(ctx, leagueID, week)
	if err != nil {
		return nil, err
	}
	if final {
		_ = c.store.SaveWeekMatchups(ctx, leagueID, week, matchups) // best-effort cache fill
	}
	return matchups, nil
}

// GetLeague serves a league's shape from our DB cache when present (roster_positions are
// static per season), writing through on a miss. This keeps grading and the per-setter
// compare off Sleeper's /league endpoint, and lets the dev seeder pre-populate league shape
// so offline/synthetic leagues still grade and render.
func (c *cachingMatchups) GetLeague(ctx context.Context, leagueID string) (provider.League, error) {
	if cached, ok, err := c.store.GetCachedLeague(ctx, leagueID); err == nil && ok {
		return cached, nil
	}
	league, err := c.sleeperDeps.GetLeague(ctx, leagueID)
	if err != nil {
		return provider.League{}, err
	}
	_ = c.store.SaveLeague(ctx, league) // best-effort cache fill
	return league, nil
}
