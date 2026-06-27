// Package grading turns submitted lineups for finished weeks into cached week_results
// rows (the leaderboard's input). It is run when CURRENT_WEEK advances and re-run to
// backfill anything still ungraded (D30). It is idempotent: already-graded lineups are
// skipped by the store, and a lineup it can't grade this run (e.g. a transient Sleeper
// failure, or the roster not yet in the week's matchups) is simply left for a later run.
package grading

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/sunkencosts/mirrorleague/internal/provider"
	"github.com/sunkencosts/mirrorleague/internal/scoring"
)

type LineupLister interface {
	ListGradableLineups(ctx context.Context, currentWeek int) ([]provider.Lineup, error)
}

type ResultWriter interface {
	UpsertWeekResult(ctx context.Context, r provider.WeekResult) error
}

type MatchupProvider interface {
	GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error)
	GetLeague(ctx context.Context, leagueID string) (provider.League, error)
}

// GradeSeason grades every past-week lineup lacking a result row and returns the number
// written. Per-lineup failures are logged and skipped (the lineup stays ungraded and is
// retried on the next run — G1 backfill), so one bad league/week never blocks the rest.
func GradeSeason(ctx context.Context, lineups LineupLister, results ResultWriter, prov MatchupProvider, currentWeek int) (int, error) {
	pending, err := lineups.ListGradableLineups(ctx, currentWeek)
	if err != nil {
		return 0, err
	}

	// Cache matchups + league per (league, week) so we fetch Sleeper once per group.
	matchupCache := map[string][]provider.WeekMatchup{}
	leagueCache := map[string]provider.League{}

	graded := 0
	for _, lineup := range pending {
		key := lineup.LeagueID + "/" + strconv.Itoa(lineup.WeekNumber)
		matchups, ok := matchupCache[key]
		if !ok {
			matchups, err = prov.GetWeekMatchups(ctx, lineup.LeagueID, lineup.WeekNumber)
			if err != nil {
				slog.WarnContext(ctx, "grading: matchups fetch failed; will retry next run",
					slog.String("league", lineup.LeagueID), slog.Int("week", lineup.WeekNumber), slog.Any("err", err))
				continue
			}
			matchupCache[key] = matchups
		}

		official := provider.FindMatchup(matchups, lineup.RosterID)
		if official == nil {
			slog.WarnContext(ctx, "grading: roster not in matchups; will retry next run",
				slog.String("league", lineup.LeagueID), slog.Int("week", lineup.WeekNumber), slog.Int("roster", lineup.RosterID))
			continue
		}

		league, ok := leagueCache[lineup.LeagueID]
		if !ok {
			league, err = prov.GetLeague(ctx, lineup.LeagueID)
			if err != nil {
				slog.WarnContext(ctx, "grading: league fetch failed; will retry next run",
					slog.String("league", lineup.LeagueID), slog.Any("err", err))
				continue
			}
			leagueCache[lineup.LeagueID] = league
		}

		grade := scoring.GradeWeek(BuildGradeInput(league, official, lineup, currentWeek))

		if err := results.UpsertWeekResult(ctx, provider.WeekResult{
			UserID:        lineup.UserID,
			LeagueID:      lineup.LeagueID,
			RosterID:      lineup.RosterID,
			Week:          lineup.WeekNumber,
			Season:        lineup.Season,
			UserTotal:     grade.UserTotal,
			OfficialTotal: grade.OfficialTotal,
			OptimalTotal:  grade.OptimalTotal,
			Result:        grade.Result,
		}); err != nil {
			slog.WarnContext(ctx, "grading: upsert failed; will retry next run",
				slog.String("user", lineup.UserID), slog.Any("err", err))
			continue
		}
		graded++
	}
	return graded, nil
}

// BuildGradeInput assembles the scoring.GradeInput for one (user, roster, week) from the
// official matchup, the user's lineup, and league settings. Shared by the season grader and
// the live compare handler so the two never derive grade inputs differently.
func BuildGradeInput(league provider.League, official *provider.WeekMatchup, lineup provider.Lineup, currentWeek int) scoring.GradeInput {
	playerPositions := make(map[string][]string, len(official.Players))
	for _, pl := range official.Players {
		playerPositions[pl.PlayerID] = pl.FantasyPositions
	}

	return scoring.GradeInput{
		RosterPositions: league.RosterPositions,
		RosterPlayers:   provider.PlayerIDs(official.Players),
		UserStarters:    lineup.Starters,
		OfficialTotal:   official.OfficialTotal(),
		Points:          official.PlayerPoints,
		PlayerPositions: playerPositions,
		Week:            lineup.WeekNumber,
		CurrentWeek:     currentWeek,
	}
}
