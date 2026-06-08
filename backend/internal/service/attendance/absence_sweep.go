// Package attendance — E5 absence-sweep (F5.2 true-ABSENT). A single-binary,
// in-process cron (wired in cmd/api, NOT cmd/worker) that periodically writes an
// ABSENT attendance row for every scheduled shift that ended (plus a grace) with no
// clock-in. The row lands verification_status=PENDING and enters the leader
// verification queue naturally — no notification needed for v1. Idempotency is the
// partial unique index on attendance(schedule_id) (migration 00043): the INSERT's
// ON CONFLICT DO NOTHING makes a re-run (or a concurrent real clock-in) a no-op, so a
// row is created at most once. A later phase may graduate this to River.
package attendance

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
)

// AbsenceCandidate is one scheduled shift that ended past the grace with no
// attendance row — the unit the sweep turns into an ABSENT record. The shift window
// is computed (schedule_entries stores HH:MM, not a timestamptz); company/site/
// position/service_line are resolved from the placement for the denormalized columns.
type AbsenceCandidate struct {
	ScheduleID   string
	EmployeeID   string
	PlacementID  string
	CompanyID    string
	SiteID       string
	PositionID   string
	ServiceLine  string
	ShiftStartAt time.Time
	ShiftEndAt   time.Time
}

// CreateAbsentParams is the INSERT payload for one ABSENT row (no clock-in).
type CreateAbsentParams struct {
	EmployeeID   string
	PlacementID  string
	ScheduleID   string
	CompanyID    string
	SiteID       string
	PositionID   string
	ServiceLine  string
	ShiftStartAt time.Time
	ShiftEndAt   time.Time
}

// AbsenceSweepRepository is the data dependency for the sweep. CreateAbsentAttendance
// returns created=false (no error) when the row already existed (ON CONFLICT no-op),
// so the service counts only real inserts.
type AbsenceSweepRepository interface {
	FindUnreportedAbsences(ctx context.Context, cutoff time.Time, limit int) ([]AbsenceCandidate, error)
	CreateAbsentAttendance(ctx context.Context, tx pgx.Tx, p CreateAbsentParams) (id string, created bool, err error)
}

// defaultBatch bounds a single sweep tick (one batch per tick; the next tick drains
// any remainder). 500 is comfortably above a realistic per-interval absence count.
const defaultBatch = 500

// AbsenceSweepService marks overdue, unreported scheduled shifts ABSENT.
type AbsenceSweepService struct {
	repo  AbsenceSweepRepository
	txm   TxRunner
	now   Clock
	grace time.Duration
	batch int
}

// NewAbsenceSweepService wires the sweep. grace is how long after shift-end a shift
// must remain unreported before it is marked ABSENT; batch bounds one tick (<=0 ⇒
// defaultBatch).
func NewAbsenceSweepService(repo AbsenceSweepRepository, txm TxRunner, grace time.Duration, batch int) *AbsenceSweepService {
	if batch <= 0 {
		batch = defaultBatch
	}
	return &AbsenceSweepService{repo: repo, txm: txm, now: time.Now, grace: grace, batch: batch}
}

// SetClock overrides the time source (tests only).
func (s *AbsenceSweepService) SetClock(c Clock) { s.now = c }

// Sweep marks one batch of overdue, unreported scheduled shifts ABSENT and returns
// the number of rows ACTUALLY created (ON CONFLICT no-ops are not counted). Each row
// is inserted + audited in its own tx, so one bad row does not roll back the batch's
// successes — though a hard insert error aborts the tick and is returned. Single
// batch per tick (documented): the scheduler's next tick continues any drain.
func (s *AbsenceSweepService) Sweep(ctx context.Context) (int, error) {
	cutoff := s.now().Add(-s.grace)
	candidates, err := s.repo.FindUnreportedAbsences(ctx, cutoff, s.batch)
	if err != nil {
		return 0, err
	}

	created := 0
	for _, c := range candidates {
		c := c
		var didCreate bool
		txErr := s.txm.InTx(ctx, func(tx pgx.Tx) error {
			id, ok, ierr := s.repo.CreateAbsentAttendance(ctx, tx, CreateAbsentParams{
				EmployeeID:   c.EmployeeID,
				PlacementID:  c.PlacementID,
				ScheduleID:   c.ScheduleID,
				CompanyID:    c.CompanyID,
				SiteID:       c.SiteID,
				PositionID:   c.PositionID,
				ServiceLine:  c.ServiceLine,
				ShiftStartAt: c.ShiftStartAt,
				ShiftEndAt:   c.ShiftEndAt,
			})
			if ierr != nil {
				return ierr
			}
			if !ok {
				// ON CONFLICT no-op (a row already exists for this schedule) — nothing
				// to audit, skip silently. Not counted as a create.
				return nil
			}
			didCreate = true
			return audit.Record(ctx, tx, audit.Entry{
				Action:     audit.ActionCreate,
				EntityType: "attendance",
				EntityID:   id,
				Before:     nil,
				After: map[string]any{
					"schedule_id":         c.ScheduleID,
					"employee_id":         c.EmployeeID,
					"status":              "ABSENT",
					"verification_status": "PENDING",
					"source":              "absence_sweep",
				},
			})
		})
		if txErr != nil {
			slog.Error("absence-sweep: insert failed",
				"schedule_id", c.ScheduleID, "employee_id", c.EmployeeID, "err", txErr)
			return created, txErr
		}
		if didCreate {
			created++
		}
	}

	slog.Info("absence-sweep complete",
		"candidates", len(candidates), "created", created, "cutoff", cutoff.Format(time.RFC3339))
	return created, nil
}
