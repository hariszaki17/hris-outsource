// Command migrate applies/rolls back goose migrations embedded in the binary.
// Usage: migrate [up|down|status|version]  (default: up)
package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

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
