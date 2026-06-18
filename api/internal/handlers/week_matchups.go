package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type weekMatchupProvider interface {
	GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error)
	GetLeague(ctx context.Context, leagueID string) (provider.League, error)
}

func HandleGetWeekMatchups(p weekMatchupProvider, store weekLockStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leagueID := r.PathValue("leagueId")
		week, ok := parseWeek(r.PathValue("week"))
		if !ok {
			http.Error(w, "invalid week", http.StatusBadRequest)
			return
		}
		// The matchups and the league (for the lock's season) are independent
		// Sleeper calls, so fetch them concurrently.
		var (
			wg         sync.WaitGroup
			matchups   []provider.WeekMatchup
			matchupErr error
			league     provider.League
			leagueErr  error
		)
		wg.Add(2)
		go func() {
			defer wg.Done()
			matchups, matchupErr = p.GetWeekMatchups(r.Context(), leagueID, week)
		}()
		go func() {
			defer wg.Done()
			league, leagueErr = p.GetLeague(r.Context(), leagueID)
		}()
		wg.Wait()

		if matchupErr != nil {
			http.Error(w, matchupErr.Error(), http.StatusInternalServerError)
			return
		}

		// Resolve the season to look up the week's lock. If the league can't be
		// resolved, degrade to unlocked rather than failing the whole view — the
		// write gate remains authoritative.
		season := ""
		if leagueErr != nil {
			slog.WarnContext(r.Context(), "could not resolve league for lock state; defaulting unlocked",
				slog.String("league_id", leagueID), slog.Any("err", leagueErr))
		} else {
			season = league.Season
		}

		locked, locksAt, _ := weekLocked(r.Context(), store, season, week)

		_ = encode(w, r, http.StatusOK, provider.WeekMatchupsResponse{
			Locked:   locked,
			LocksAt:  locksAt,
			Matchups: matchups,
		})
	})
}
