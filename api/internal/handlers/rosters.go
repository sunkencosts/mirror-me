package handlers

import (
	"context"
	"net/http"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type rosterProvider interface {
	GetRosters(ctx context.Context, leagueID string) ([]provider.Roster, error)
}

func HandleGetRosters(p rosterProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leagueID := r.PathValue("leagueId")
		rosters, err := p.GetRosters(r.Context(), leagueID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, rosters)
	})
}
