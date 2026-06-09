package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sunkencosts/mirrorleague/pkg/config"
)

func main() {
	if err := runMigrate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runMigrate() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: migrate <up|down|version>")
	}

	cfg := config.Load(os.Getenv)

	migrateURL := strings.Replace(cfg.DatabaseURL, "postgresql://", "pgx5://", 1)
	migrateURL = strings.Replace(migrateURL, "postgres://", "pgx5://", 1)

	m, err := migrate.New(cfg.MigrationsURL, migrateURL)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	switch os.Args[1] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate up: %w", err)
		}
		fmt.Fprintln(os.Stdout, "migrations applied")
	case "down":
		if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate down: %w", err)
		}
		fmt.Fprintln(os.Stdout, "rolled back one migration")
	case "version":
		v, dirty, err := m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Fprintln(os.Stdout, "version: none")
			return nil
		}
		if err != nil {
			return fmt.Errorf("version: %w", err)
		}
		fmt.Fprintf(os.Stdout, "version: %d, dirty: %v\n", v, dirty)
	default:
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
	return nil
}
