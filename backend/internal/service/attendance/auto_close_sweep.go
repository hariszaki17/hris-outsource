// Package attendance — E5 auto-close sweep. Closes stale open attendance records
// whose shift_end_at has elapsed past a grace period with no clock-out. Each row
// is marked autoclosed + AUTO_CLOSED flag, status INCOMPLETE, verification PENDING
// (enters the leader queue as an anomaly). Idempotent via the guarded UPDATE.
package attendance

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
)

// StaleOpenAttendance is a detected stale open record for the auto-close sweep.
type StaleOpenAttendance struct {
	ID          string
	CheckInAt   *time.Time
	ShiftEndAt  *time.Time
	Flags       []string
}

// AutoCloseSweepRepository is the data dependency for the auto-close sweep.
type AutoCloseSweepRepository interface {
	ListStaleOpenAttendances(ctx context.Context, cutoff time.Time, limit int) ([]StaleOpenAttendance, error)
	AutoCloseAttendance(ctx context.Context, tx pgx.Tx, p AutoCloseRow) (id string, found bool, err error)
}

// defaultAutoCloseGrace is the grace duration after shift_end before a stale open
// record is auto-closed (48h default).
const defaultAutoCloseGrace = 48 * time.Hour

// defaultAutoCloseBatch bounds a single sweep tick.
const defaultAutoCloseBatch = 500

// AutoCloseSweepService closes stale open attendance records.
type AutoCloseSweepService struct {
	repo  AutoCloseSweepRepository
	txm   TxRunner
	now   Clock
	grace time.Duration
	batch int
}

// NewAutoCloseSweepService wires the auto-close sweep.
func NewAutoCloseSweepService(repo AutoCloseSweepRepository, txm TxRunner, grace time.Duration, batch int) *AutoCloseSweepService {
	if grace <= 0 {
		grace = defaultAutoCloseGrace
	}
	if batch <= 0 {
		batch = defaultAutoCloseBatch
	}
	return &AutoCloseSweepService{repo: repo, txm: txm, now: time.Now, grace: grace, batch: batch}
}

// SetClock overrides the time source (tests only).
func (s *AutoCloseSweepService) SetClock(c Clock) { s.now = c }

// Sweep closes one batch of stale open attendance records and returns the count
// actually closed. Each row is autoclosed + audited in its own tx so one bad row
// does not roll back the batch.
func (s *AutoCloseSweepService) Sweep(ctx context.Context) (int, error) {
	cutoff := s.now().Add(-s.grace)
	candidates, err := s.repo.ListStaleOpenAttendances(ctx, cutoff, s.batch)
	if err != nil {
		return 0, err
	}

	closed := 0
	for _, c := range candidates {
		c := c
		end := time.Now()
		if c.ShiftEndAt != nil {
			end = *c.ShiftEndAt
		}
		worked := 0
		if c.CheckInAt != nil {
			worked = int(end.Sub(*c.CheckInAt).Minutes())
		}
		if worked < 0 {
			worked = 0
		}

		flags := make([]string, 0, len(c.Flags)+1)
		for _, f := range c.Flags {
			flags = append(flags, f)
		}
		flags = appendUnique(flags, string(att.FlagAutoClosed))

		row := AutoCloseRow{
			ID:                 c.ID,
			CheckOutAt:         end,
			WorkedMinutes:      worked,
			Flags:              flags,
			Status:             string(att.StatusIncomplete),
			VerificationStatus: string(att.VerificationPending),
		}

		txErr := s.txm.InTx(ctx, func(tx pgx.Tx) error {
			_, found, ierr := s.repo.AutoCloseAttendance(ctx, tx, row)
			if ierr != nil {
				return ierr
			}
			if !found {
				return nil // already closed
			}
			return audit.Record(ctx, tx, audit.Entry{
				Action:     audit.ActionUpdate,
				EntityType: "attendance",
				EntityID:   c.ID,
				Before:     map[string]any{"check_out_at": nil},
				After: map[string]any{
					"check_out_at":        end.UTC(),
					"worked_minutes":      worked,
					"status":              string(att.StatusIncomplete),
					"verification_status": string(att.VerificationPending),
					"auto_closed":         true,
					"flags":               flags,
					"source":              "auto_close_sweep",
				},
			})
		})
		if txErr != nil {
			slog.Error("auto-close-sweep: update failed",
				"attendance_id", c.ID, "err", txErr)
			return closed, txErr
		}
		closed++
	}

	slog.Info("auto-close-sweep complete",
		"candidates", len(candidates), "closed", closed, "cutoff", cutoff.Format(time.RFC3339))
	return closed, nil
}
