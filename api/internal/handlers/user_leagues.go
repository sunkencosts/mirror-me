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

// resolveUserID returns the acting user's id. Signed-in requests (OptionalAuth found a
// valid JWT) always use the claims subject, ignoring any client-supplied id — otherwise a
// signed-in user could read or mutate another signed-in user's bookmarks by passing their
// id. Anonymous requests fall back to the client-supplied id (the unguessable localStorage
// UUID is the anon trust model).
func resolveUserID(r *http.Request, clientUserID string) string {
	if claims, ok := ClaimsFromContext(r.Context()); ok {
		return claims.Subject
	}
	return clientUserID
}

func HandleSaveUserLeague(store userLeagueStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[saveUserLeagueRequest](w, r)
		if err != nil {
			writeDecodeError(w, err)
			return
		}
		userID := resolveUserID(r, req.UserID)
		if userID == "" || req.LeagueID == "" || req.Source == "" {
			http.Error(w, "missing user_id or league_id or source", http.StatusBadRequest)
			return
		}
		if _, ok := sourceIcons[req.Source]; !ok {
			http.Error(w, "unknown source", http.StatusBadRequest)
			return
		}
		ul, err := store.SaveUserLeague(r.Context(), userID, req.LeagueID, req.Source, req.Label)
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
		userID := resolveUserID(r, r.URL.Query().Get("user_id"))
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
			// ListUserLeagues fills IconURL with the cached league avatar when known; fall back
			// to the generic per-source icon for leagues not yet cached (or with no avatar set).
			if leagues[i].IconURL == "" {
				leagues[i].IconURL = iconForSource(leagues[i].Source)
			}
		}

		_ = encode(w, r, http.StatusOK, leagues)
	})
}

func HandleUpdateUserLeague(store userLeagueStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leagueID := r.PathValue("leagueId")
		req, err := decode[updateUserLeagueRequest](w, r)
		if err != nil {
			writeDecodeError(w, err)
			return
		}
		source := r.URL.Query().Get("source")
		userID := resolveUserID(r, req.UserID)
		if userID == "" || source == "" {
			http.Error(w, "missing user_id or source", http.StatusBadRequest)
			return
		}
		if _, ok := sourceIcons[source]; !ok {
			http.Error(w, "unknown source", http.StatusBadRequest)
			return
		}
		ul, err := store.UpdateUserLeague(r.Context(), userID, leagueID, source, req.Label)
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
		userID := resolveUserID(r, r.URL.Query().Get("user_id"))
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
