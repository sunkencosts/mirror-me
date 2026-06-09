package handlers

import (
	"context"
	"net/http"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type weekMatchupProvider interface {
	GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error)
}

func HandleGetWeekMatchups(p weekMatchupProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leagueID := r.PathValue("leagueId")
		week, ok := parseWeek(r.PathValue("week"))
		if !ok {
			http.Error(w, "invalid week", http.StatusBadRequest)
			return
		}
		matchups, err := p.GetWeekMatchups(r.Context(), leagueID, week)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, matchups)
	})
}
