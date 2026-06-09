package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type leagueProvider interface {
	GetLeague(ctx context.Context, leagueID string) (provider.League, error)
}

func HandleGetLeague(p leagueProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leagueID := r.PathValue("leagueId")
		league, err := p.GetLeague(r.Context(), leagueID)
		if err != nil {
			if errors.Is(err, provider.ErrLeagueNotFound) {
				http.Error(w, "league not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, league)
	})
}
