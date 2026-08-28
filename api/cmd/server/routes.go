package main

import (
	"context"
	"net/http"
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
func addRoutes(mux *http.ServeMux, sleeperClient sleeperDeps, store *db.Store, cfg config.Config, googleClient *googleauth.Client, currentWeek func(context.Context) int) {
	jwtSecret := []byte(cfg.JWTSecret)
	requireAuth := handlers.RequireAuth(jwtSecret)
	// rateLimit guards the routes that are both unauthenticated and either write to our DB
	// (POST /collect) or proxy an arbitrary caller-supplied league ID through to Sleeper —
	// the two abuse paths described in GH #13. It is not applied blanket-wide: authenticated
	// routes already require a signed-in user, and DB-only public reads (GET /players,
	// leaderboards) don't risk Sleeper's rate limit.
	rateLimit := newIPRateLimiter().middleware
	adminMux := http.NewServeMux()
	adminMux.Handle("POST /admin/sync-players", handlers.HandleSyncPlayers(store, sleeperClient, cfg.SleeperBaseURL, cfg.RankingsCSVURL))
	adminMux.Handle("POST /admin/grade", handlers.HandleGradeSeason(store, sleeperClient, currentWeek))
	mux.Handle("/admin/", handlers.RequireAdminSecret(cfg.AdminSecret)(adminMux))

	mux.Handle("GET /auth/google", handlers.HandleGoogleLogin(googleClient))
	mux.Handle("GET /auth/google/callback", handlers.HandleGoogleCallback(googleClient, store, jwtSecret, cfg.FrontendURL))
	mux.Handle("GET /auth/me", requireAuth(handlers.HandleAuthMe()))
	mux.Handle("PATCH /auth/profile", requireAuth(handlers.HandleUpdateProfile(store, jwtSecret, googleClient.IsSecure())))
	mux.Handle("POST /auth/merge", requireAuth(handlers.HandleMerge(store)))
	mux.Handle("DELETE /auth/logout", handlers.HandleLogout(googleClient.IsSecure()))
	if cfg.AppEnv == "development" {
		mux.Handle("GET /dev/login", handlers.HandleDevLogin(jwtSecret, cfg.FrontendURL, cfg.DevLoginUserID, cfg.DevLoginEmail, cfg.DevLoginUsername))
	}

	optionalAuth := handlers.OptionalAuth(jwtSecret)
	mux.Handle("POST /league-bookmarks", optionalAuth(handlers.HandleSaveUserLeague(store)))
	mux.Handle("GET /league-bookmarks", optionalAuth(handlers.HandleListUserLeagues(store)))
	mux.Handle("PATCH /league-bookmarks/{leagueId}", optionalAuth(handlers.HandleUpdateUserLeague(store)))
	mux.Handle("DELETE /league-bookmarks/{leagueId}", optionalAuth(handlers.HandleDeleteUserLeague(store)))
	mux.Handle("POST /lineups", requireAuth(handlers.HandleCreateLineup(store, sleeperClient)))
	mux.Handle("PATCH /lineups/{id}", requireAuth(handlers.HandleUpdateLineup(store, sleeperClient)))
	mux.Handle("GET /lineups", optionalAuth(handlers.HandleListLineups(store)))
	mux.Handle("GET /lineups/{id}", handlers.HandleGetLineupByID(store))
	mux.Handle("POST /collect", rateLimit(handlers.OptionalAuth(jwtSecret)(handlers.HandleCollect(store))))
	mux.Handle("GET /players", handlers.HandleGetPlayers(store))
	mux.Handle("GET /league/{leagueId}/rosters", rateLimit(handlers.HandleGetRosters(sleeperClient)))
	mux.Handle("GET /league/{leagueId}/week/{week}", rateLimit(handlers.HandleGetWeekMatchups(sleeperClient, store)))
	mux.Handle("GET /league/{leagueId}/week/{week}/roster/{rosterId}/compare", requireAuth(handlers.HandleGetCompare(sleeperClient, store, currentWeek)))
	mux.Handle("GET /league/{leagueId}/week/{week}/roster/{rosterId}/score", rateLimit(handlers.HandleSetterLineup(sleeperClient, store, currentWeek)))
	mux.Handle("GET /league/{leagueId}/week/{week}/results", handlers.HandleWeeklyResults(store, cfg.CurrentSeason))
	mux.Handle("GET /leaderboard", handlers.HandleGlobalLeaderboard(store, cfg.CurrentSeason))
	mux.Handle("GET /league/{leagueId}/leaderboard", handlers.HandleLeagueLeaderboard(store, cfg.CurrentSeason))
	mux.Handle("GET /league/{leagueId}", rateLimit(handlers.HandleGetLeague(sleeperClient)))
	mux.HandleFunc("GET /healthz", handleHealthz(store))
	mux.Handle("/", handleNotFound())
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

func handleNotFound() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}
}
