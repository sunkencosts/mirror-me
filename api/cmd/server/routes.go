package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sunkencosts/mirrorleague/internal/db"
	"github.com/sunkencosts/mirrorleague/internal/googleauth"
	"github.com/sunkencosts/mirrorleague/internal/handlers"
	"github.com/sunkencosts/mirrorleague/internal/provider"
	"github.com/sunkencosts/mirrorleague/pkg/config"
)

type sleeperDeps interface {
	provider.Provider
	InvalidateRosters()
	GetWeekMatchups(ctx context.Context, leagueID string, week int) ([]provider.WeekMatchup, error)
}

// Update api/api.md when adding or removing routes here.
func addRoutes(mux *http.ServeMux, sleeperClient sleeperDeps, store *db.Store, cfg config.Config, googleClient *googleauth.Client) {
	jwtSecret := []byte(cfg.JWTSecret)
	requireAuth := handlers.RequireAuth(jwtSecret)
	adminMux := http.NewServeMux()
	adminMux.Handle("POST /admin/sync-players", handlers.HandleSyncPlayers(store, sleeperClient, cfg.SleeperBaseURL, cfg.RankingsCSVURL))
	mux.Handle("/admin/", handlers.RequireAdminSecret(cfg.AdminSecret)(adminMux))

	mux.Handle("GET /auth/google", handlers.HandleGoogleLogin(googleClient))
	mux.Handle("GET /auth/google/callback", handlers.HandleGoogleCallback(googleClient, store, jwtSecret, cfg.FrontendURL))
	mux.Handle("GET /auth/me", requireAuth(handlers.HandleAuthMe()))
	mux.Handle("POST /auth/merge", requireAuth(handlers.HandleMerge(store)))
	mux.Handle("DELETE /auth/logout", handlers.HandleLogout(googleClient.IsSecure()))
	if cfg.AppEnv == "development" {
		mux.Handle("GET /dev/login", handlers.HandleDevLogin(jwtSecret, cfg.FrontendURL))
	}

	mux.Handle("POST /league-bookmarks", handlers.HandleSaveUserLeague(store))
	mux.Handle("GET /league-bookmarks", handlers.HandleListUserLeagues(store))
	mux.Handle("PATCH /league-bookmarks/{leagueId}", handlers.HandleUpdateUserLeague(store))
	mux.Handle("DELETE /league-bookmarks/{leagueId}", handlers.HandleDeleteUserLeague(store))
	mux.Handle("POST /lineups", requireAuth(handlers.HandleCreateLineup(store, sleeperClient)))
	mux.Handle("PATCH /lineups/{id}", requireAuth(handlers.HandleUpdateLineup(store, sleeperClient)))
	mux.Handle("GET /lineups", handlers.HandleListLineups(store))
	mux.Handle("GET /lineups/{id}", handlers.HandleGetLineupByID(store))
	mux.Handle("GET /players", handlers.HandleGetPlayers(store))
	mux.Handle("GET /league/{leagueId}/rosters", handlers.HandleGetRosters(sleeperClient))
	mux.Handle("GET /league/{leagueId}/week/{week}", handlers.HandleGetWeekMatchups(sleeperClient))
	mux.Handle("GET /league/{leagueId}/week/{week}/roster/{rosterId}/compare", handlers.HandleGetCompare(sleeperClient, store))
	mux.Handle("GET /league/{leagueId}", handlers.HandleGetLeague(sleeperClient))
	mux.HandleFunc("GET /healthz", handleHealthz(store))
	mux.Handle("/", spaHandler("web/dist"))
}

func handleHealthz(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func spaHandler(distDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(distDir, filepath.Clean("/"+r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
			return
		}
		http.FileServer(http.Dir(distDir)).ServeHTTP(w, r)
	}
}
