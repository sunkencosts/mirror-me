package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/sunkencosts/mirrorleague/internal/provider"
	"github.com/sunkencosts/mirrorleague/internal/sleeper"
)

type userLeagueStore interface {
	SaveUserLeague(ctx context.Context, userID, leagueID, source, label string) (provider.UserLeague, error)
	ListUserLeagues(ctx context.Context, userID string) ([]provider.UserLeague, error)
	UpdateUserLeague(ctx context.Context, userID, leagueID, source, label string) (provider.UserLeague, error)
	DeleteUserLeague(ctx context.Context, userID, leagueID, source string) error
}

type saveUserLeagueRequest struct {
	UserID   string `json:"user_id"`
	LeagueID string `json:"league_id"`
	Label    string `json:"label"`
	Source   string `json:"source"`
}

type updateUserLeagueRequest struct {
	UserID string `json:"user_id"`
	Label  string `json:"label"`
}

var sourceIcons = map[string]string{
	"sleeper": sleeper.IconURL,
}

func iconForSource(source string) string {
	return sourceIcons[source]
}

func HandleSaveUserLeague(store userLeagueStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[saveUserLeagueRequest](r)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.UserID == "" || req.LeagueID == "" || req.Source == "" {
			http.Error(w, "missing user_id or league_id or source", http.StatusBadRequest)
			return
		}
		if _, ok := sourceIcons[req.Source]; !ok {
			http.Error(w, "unknown source", http.StatusBadRequest)
			return
		}
		ul, err := store.SaveUserLeague(r.Context(), req.UserID, req.LeagueID, req.Source, req.Label)
		if err != nil {
			http.Error(w, "failed to save bookmark", http.StatusInternalServerError)
			return
		}
		ul.IconURL = iconForSource(ul.Source)
		_ = encode(w, r, http.StatusOK, ul)
	})
}

func HandleListUserLeagues(store userLeagueStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}
		leagues, err := store.ListUserLeagues(r.Context(), userID)
		if err != nil {
			http.Error(w, "failed to list bookmarks", http.StatusInternalServerError)
			return
		}
		for i := range leagues {
			leagues[i].IconURL = iconForSource(leagues[i].Source)
		}

		_ = encode(w, r, http.StatusOK, leagues)
	})
}

func HandleUpdateUserLeague(store userLeagueStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leagueID := r.PathValue("leagueId")
		req, err := decode[updateUserLeagueRequest](r)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		source := r.URL.Query().Get("source")
		if req.UserID == "" || source == "" {
			http.Error(w, "missing user_id or source", http.StatusBadRequest)
			return
		}
		if _, ok := sourceIcons[source]; !ok {
			http.Error(w, "unknown source", http.StatusBadRequest)
			return
		}
		ul, err := store.UpdateUserLeague(r.Context(), req.UserID, leagueID, source, req.Label)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.Error(w, "bookmark not found", http.StatusNotFound)
				return
			}
			http.Error(w, "failed to update bookmark", http.StatusInternalServerError)
			return
		}
		ul.IconURL = iconForSource(ul.Source)
		_ = encode(w, r, http.StatusOK, ul)
	})
}

func HandleDeleteUserLeague(store userLeagueStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leagueID := r.PathValue("leagueId")
		userID := r.URL.Query().Get("user_id")
		source := r.URL.Query().Get("source")
		if userID == "" || source == "" {
			http.Error(w, "missing user_id or source", http.StatusBadRequest)
			return
		}
		if err := store.DeleteUserLeague(r.Context(), userID, leagueID, source); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.Error(w, "bookmark not found", http.StatusNotFound)
				return
			}
			http.Error(w, "failed to delete bookmark", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
