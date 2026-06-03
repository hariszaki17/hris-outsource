// Command worker runs the River job processor: notification dispatch, export
// jobs, and cron "expiring-soon" detection. Runs as a separate process from the
// API so async load scales independently. Requires River's tables —
// `make river-migrate` (or `river migrate-up`).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/config"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	applog "github.com/hariszaki17/hris-outsource/backend/internal/platform/log"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	applog.Setup(cfg.Env, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DB.URL, cfg.DB.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()

	client, err := jobs.NewWorkerClient(pool)
	if err != nil {
		return err
	}

	if err := client.Start(ctx); err != nil {
		return err
	}
	slog.Info("worker started")

	<-ctx.Done()
	slog.Info("worker shutting down")

	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return client.Stop(stopCtx)
}
