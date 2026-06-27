package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type weeklyResultsStore interface {
	WeeklyResults(ctx context.Context, season, leagueID string, rosterID, week int, q string, limit, offset int) (provider.WeeklyRosterResults, error)
}

// defaultWeeklyResultsLimit caps the setter rows returned when the client doesn't ask for a
// specific page — enough for the default view; the long tail is reached via limit/offset/q.
const defaultWeeklyResultsLimit = 100

// HandleWeeklyResults serves the per-(league, week, roster) setter standings for the weekly
// results browser. Public, DB-only (reads cached week_results). roster_id is required (the UI
// is Week -> Team -> setters). season defaults to the configured current season. A roster
// nobody mirrored returns 200 with an empty setter list.
func HandleWeeklyResults(store weeklyResultsStore, defaultSeason string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leagueID := r.PathValue("leagueId")
		week, ok := parseWeek(r.PathValue("week"))
		if !ok {
			http.Error(w, "invalid week", http.StatusBadRequest)
			return
		}
		rosterID, ok := parsePositiveInt(r.URL.Query().Get("roster_id"))
		if !ok {
			http.Error(w, "roster_id is required", http.StatusBadRequest)
			return
		}

		season := r.URL.Query().Get("season")
		if season == "" {
			season = defaultSeason
		}
		q := r.URL.Query().Get("q")
		limit := defaultWeeklyResultsLimit
		if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
			limit = v
		}
		offset := 0
		if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
			offset = v
		}

		results, err := store.WeeklyResults(r.Context(), season, leagueID, rosterID, week, q, limit, offset)
		if err != nil {
			http.Error(w, "failed to load weekly results", http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, results)
	})
}
