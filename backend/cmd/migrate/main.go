// Command migrate applies/rolls back goose migrations embedded in the binary.
// Usage: migrate [up|down|status|version]  (default: up)
//
// Special subcommand: `migrate river-up` applies River's own job-queue migrations
// PROGRAMMATICALLY via rivermigrate (no external `river` CLI needed). The E2E harness
// uses this so the worker's queue tables (river_queue/river_job/…) always exist even
// when the `river` CLI is not installed — without them the worker crashes on boot
// ("relation river_queue does not exist") and the async export job never completes.
package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	migrations "github.com/hariszaki17/hris-outsource/backend/db/migrations"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	// River queue migrations — applied programmatically (no `river` CLI dependency).
	if command == "river-up" {
		return riverMigrateUp(context.Background(), cfg.DB.URL)
	}

	db, err := sql.Open("pgx", cfg.DB.URL)
	if err != nil {
		return err
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.RunContext(context.Background(), command, db, ".", os.Args[2:]...)
}

// riverMigrateUp creates River's job-queue tables via rivermigrate over a pgx pool.
// Equivalent to `river migrate-up` but with no external CLI install required.
func riverMigrateUp(ctx context.Context, dbURL string) error {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return err
	}
	slog.Info("river migrations applied", "count", len(res.Versions))
	return nil
}
