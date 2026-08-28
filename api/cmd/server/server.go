package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sunkencosts/mirrorleague/internal/db"
	"github.com/sunkencosts/mirrorleague/internal/googleauth"
	"github.com/sunkencosts/mirrorleague/internal/sleeper"
	"github.com/sunkencosts/mirrorleague/pkg/config"
	"github.com/sunkencosts/mirrorleague/pkg/logger"
)

func run(ctx context.Context, getenv func(string) string, stdout, stderr io.Writer) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	cfg := config.Load(getenv)

	if cfg.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET must be set")
	}
	if cfg.AdminSecret == "" {
		return fmt.Errorf("ADMIN_SECRET must be set")
	}

	logger, closeLog, err := logger.New(cfg.AppEnv, cfg.LogFile, stdout, stderr)
	if err != nil {
		return fmt.Errorf("initializing logger: %w", err)
	}
	// Make the configured logger the slog default so handlers can log (e.g. the
	// fail-open warning when a week_locks row is missing) without threading it through.
	slog.SetDefault(logger)
	defer func() {
		if err := closeLog(); err != nil {
			fmt.Fprintf(stderr, "closing log: %v\n", err)
		}
	}()

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parsing database URL: %w", err)
	}
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.MaxConnIdleTime = 5 * time.Minute

	dbpool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("creating connection pool: %w", err)
	}
	defer dbpool.Close()

	store := db.NewStore(dbpool)
	if err := store.Ping(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}
	logger.Info("database connected")

	currentWeek := newCurrentWeekResolver(store, cfg)
	sleeperClient := sleeper.New(cfg.SleeperBaseURL, store, currentWeek)
	// Persist final-week matchups in our DB so repeated reads (compare, per-setter lineup,
	// grading, weekly browser) don't re-hit Sleeper's rate limit; the live week still passes
	// through to Sleeper. Seeding week_matchups rows lets dev render real scores offline.
	cachingSleeper := newCachingMatchups(sleeperClient, store, currentWeek)
	googleClient := googleauth.New(googleauth.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		AuthURL:      cfg.GoogleAuthURL,
		TokenURL:     cfg.GoogleTokenURL,
		UserInfoURL:  cfg.GoogleUserInfoURL,
	})

	migrateURL := strings.Replace(cfg.DatabaseURL, "postgresql://", "pgx5://", 1)
	migrateURL = strings.Replace(migrateURL, "postgres://", "pgx5://", 1)
	m, err := migrate.New(cfg.MigrationsURL, migrateURL)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}
	// Release the migrator's dedicated DB connection now that migrations are done —
	// it is separate from dbpool and would otherwise leak for the process lifetime.
	if srcErr, dbErr := m.Close(); srcErr != nil {
		return fmt.Errorf("closing migration source: %w", srcErr)
	} else if dbErr != nil {
		return fmt.Errorf("closing migration db: %w", dbErr)
	}

	srv := NewServer(cachingSleeper, cfg, store, googleClient, logger, currentWeek)

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", ":"+cfg.Port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	httpServer := &http.Server{
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("server listening", slog.String("addr", listener.Addr().String()))
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.Any("err", err))
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { //nolint:gosec // intentional: shutdown goroutine uses context.Background so it can outlive the cancelled parent ctx
		defer wg.Done()
		<-ctx.Done()
		shutdownCtx := context.Background()
		shutdownCtx, cancel := context.WithTimeout(shutdownCtx, 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown error", slog.Any("err", err))
		}
	}()
	wg.Wait()
	return nil
}

// newCurrentWeekResolver returns a function that yields the live NFL week at call time.
// A non-zero CURRENT_WEEK override (tests, manual control) wins; otherwise it is inferred
// from week_locks + the current date and auto-advances at each week's kickoff. Falls back
// to week 1 if inference fails or no week has kicked off yet.
func newCurrentWeekResolver(store *db.Store, cfg config.Config) func(context.Context) int {
	return func(ctx context.Context) int {
		if cfg.CurrentWeek > 0 {
			return cfg.CurrentWeek
		}
		week, err := store.CurrentWeekFromLocks(ctx, cfg.CurrentSeason, time.Now())
		if err != nil {
			// A transient inference failure silently rewinds the live week to 1 (wrong
			// cache TTLs, Final flags, grading window). Fall back, but make it loud so it
			// is not mistaken for genuine pre-season.
			slog.ErrorContext(ctx, "current-week inference failed; falling back to week 1",
				slog.String("season", cfg.CurrentSeason), slog.Any("err", err))
			return 1
		}
		if week == 0 {
			return 1 // pre-season: no week has kicked off yet
		}
		return week
	}
}

func NewServer(sleeperClient sleeperDeps, cfg config.Config, store *db.Store, googleClient *googleauth.Client, logger *slog.Logger, currentWeek func(context.Context) int) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, sleeperClient, store, cfg, googleClient, currentWeek)

	var handler http.Handler = mux
	handler = timeoutMiddleware(20 * time.Second)(handler)
	handler = corsMiddleware(cfg.FrontendURL)(handler)
	handler = requestLogger(logger)(handler)
	return handler
}

func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// csrfProtect guards a mutating handler that trusts the auth cookie. The cookie is
// SameSite=None; Secure in production (required by the split-origin frontend/API setup), and
// decode() does not enforce Content-Type — together that lets a cross-site "simple" request
// (a plain <form> POST or navigator.sendBeacon, neither of which triggers a CORS preflight)
// carry the user's cookie to a mutating endpoint. Close that gap two ways: reject a present
// Origin that doesn't match the configured frontend, and require Content-Type:
// application/json so a cross-site POST is forced through preflight. A request with no Origin
// header (same-origin browser navigation, curl, server-to-server) is allowed through — only a
// present, mismatched Origin is rejected. See GH #12.
func csrfProtect(frontendURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); origin != "" && origin != frontendURL {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
			if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
				http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func corsMiddleware(frontendURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				// Vary: Origin tells any cache in front of this server that the response
				// depends on the request's Origin header, so it must not serve a response
				// built for one origin to a request from another.
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Origin", frontendURL)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			// Only a real CORS preflight (Origin + Access-Control-Request-Method both
			// present, per the Fetch spec) short-circuits into the bare 204. A plain
			// OPTIONS request with neither is not a preflight and must fall through to
			// normal routing.
			if r.Method == http.MethodOptions && origin != "" && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := newResponseWriter(w)
			start := time.Now()
			next.ServeHTTP(rw, r)
			logger.Info("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}
