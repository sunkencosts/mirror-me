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

	sleeperClient := sleeper.New(cfg.SleeperBaseURL, store, cfg.CurrentWeek)
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

	srv := NewServer(sleeperClient, cfg, store, googleClient, logger)

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

func NewServer(sleeperClient sleeperDeps, cfg config.Config, store *db.Store, googleClient *googleauth.Client, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, sleeperClient, store, cfg, googleClient)

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

func corsMiddleware(frontendURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Origin") != "" {
				w.Header().Set("Access-Control-Allow-Origin", frontendURL)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if r.Method == http.MethodOptions {
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
