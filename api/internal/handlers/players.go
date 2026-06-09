package handlers

import (
	"context"
	"net/http"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type playerStore interface {
	ListActiveFantasyPlayers(ctx context.Context) ([]provider.SlimPlayer, error)
}

func HandleGetPlayers(s playerStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		players, err := s.ListActiveFantasyPlayers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = encode(w, r, http.StatusOK, players)
	})
}
