// Command cron runs the periodic maintenance sweeps as ONE-SHOT, triggerable
// jobs — one invocation runs the named sweep once and exits. This replaces the
// former in-process cron runners that lived inside cmd/api (which mutated shared
// state on a timer and raced with anything else touching the DB, e.g. tests).
//
// Scheduling is now external: an OS/k8s cron (or a manual run) invokes this
// binary, e.g.
//
//	cron absence-sweep        # F5.2 — mark overdue unclocked shifts ABSENT
//	cron leave-expiry-sweep   # F6.1 — release pending on lapsed grant-lots
//	cron all                  # run every sweep once, in order
//
// Each sweep's business logic is the SAME service method the API exposes, so the
// jobs stay trivially unit-testable and directly callable. Exit code is non-zero
// on failure so a scheduler can alert.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/config"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	applog "github.com/hariszaki17/hris-outsource/backend/internal/platform/log"
	attendancerepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/attendance"
	attendancesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// defaultAutoCloseGrace is the grace period for auto-closing stale open attendance
// records (48h default). Mirrors the constant in auto_close_sweep.go.
const defaultAutoCloseGrace = 48 * time.Hour

// autoCloseSweepRepoAdapter combines AttendanceRepo (ListStaleOpenAttendances)
// and ClockRepo (AutoCloseAttendance) to satisfy AutoCloseSweepRepository.
type autoCloseSweepRepoAdapter struct {
	att *attendancerepo.AttendanceRepo
	clk *attendancerepo.ClockRepo
}

func (a *autoCloseSweepRepoAdapter) ListStaleOpenAttendances(ctx context.Context, cutoff time.Time, limit int) ([]attendancesvc.StaleOpenAttendance, error) {
	return a.att.ListStaleOpenAttendances(ctx, cutoff, limit)
}

func (a *autoCloseSweepRepoAdapter) AutoCloseAttendance(ctx context.Context, tx pgx.Tx, p attendancesvc.AutoCloseRow) (string, bool, error) {
	return a.clk.AutoCloseAttendance(ctx, tx, p)
}

// maxRunDuration bounds a single sweep invocation so a runaway job can't hang a
// scheduler slot forever.
const maxRunDuration = 10 * time.Minute

func main() {
	if err := run(); err != nil {
		slog.Error("cron fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: cron <absence-sweep|leave-expiry-sweep|all>")
	}
	job := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	applog.Setup(cfg.Env, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, maxRunDuration)
	defer cancel()

	pool, err := db.Open(ctx, cfg.DB.URL, cfg.DB.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()
	txm := db.NewTxManager(pool)

	// absence-sweep (F5.2): write ABSENT rows for scheduled shifts that ended past
	// the grace window with no clock-in. Idempotent via the partial unique index on
	// attendance(schedule_id).
	runAbsence := func() error {
		svc := attendancesvc.NewAbsenceSweepService(attendancerepo.NewAbsenceSweepRepo(pool), txm, cfg.Cron.AbsenceGrace, 0)
		n, err := svc.Sweep(ctx)
		if err != nil {
			return fmt.Errorf("absence-sweep: %w", err)
		}
		slog.Info("absence-sweep done", "absent_created", n)
		return nil
	}

	runAutoClose := func() error {
		attRepo := attendancerepo.NewAttendanceRepo(pool)
		clkRepo := attendancerepo.NewClockRepo(pool)
		svc := attendancesvc.NewAutoCloseSweepService(&autoCloseSweepRepoAdapter{att: attRepo, clk: clkRepo}, txm, defaultAutoCloseGrace, 0)
		n, err := svc.Sweep(ctx)
		if err != nil {
			return fmt.Errorf("auto-close-sweep: %w", err)
		}
		slog.Info("auto-close-sweep done", "closed", n)
		return nil
	}

	runAll := func() error {
		if err := runAbsence(); err != nil {
			return err
		}
		if err := runAutoClose(); err != nil {
			return err
		}
		return nil
	}

	switch job {
	case "absence-sweep":
		return runAbsence()
	case "auto-close-sweep":
		return runAutoClose()
	case "all":
		return runAll()
	default:
		return fmt.Errorf("unknown job %q (want absence-sweep|auto-close-sweep|all)", job)
	}
}
