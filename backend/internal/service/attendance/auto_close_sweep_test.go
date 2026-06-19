// Package attendance — unit tests for the auto-close sweep service. A fake repo holds
// the candidate set + tracks closed rows; a fake clock + fake runner let Sweep run
// without Postgres.
package attendance

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
)

// --- fakeTx / fakeRunner ---

type autoCloseFakeTx struct{}

func (autoCloseFakeTx) Begin(context.Context) (pgx.Tx, error) { return autoCloseFakeTx{}, nil }
func (autoCloseFakeTx) Commit(context.Context) error          { return nil }
func (autoCloseFakeTx) Rollback(context.Context) error        { return nil }
func (autoCloseFakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (autoCloseFakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) { panic("Query unused") }
func (autoCloseFakeTx) QueryRow(context.Context, string, ...any) pgx.Row       { panic("QueryRow unused") }
func (autoCloseFakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("CopyFrom unused")
}
func (autoCloseFakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("SendBatch unused") }
func (autoCloseFakeTx) LargeObjects() pgx.LargeObjects                         { panic("LargeObjects unused") }
func (autoCloseFakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("Prepare unused")
}
func (autoCloseFakeTx) Conn() *pgx.Conn { panic("Conn unused") }

type autoCloseFakeRunner struct{}

func (autoCloseFakeRunner) InTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(autoCloseFakeTx{})
}

// --- fake auto-close repo ---

type fakeAutoCloseRepo struct {
	stale   []StaleOpenAttendance
	closed  map[string]AutoCloseRow
	findErr error
}

func (f *fakeAutoCloseRepo) ListStaleOpenAttendances(_ context.Context, cutoff time.Time, limit int) ([]StaleOpenAttendance, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	out := make([]StaleOpenAttendance, 0, len(f.stale))
	for _, c := range f.stale {
		if c.ShiftEndAt != nil && c.ShiftEndAt.Before(cutoff) {
			if _, already := f.closed[c.ID]; already {
				continue
			}
			out = append(out, c)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeAutoCloseRepo) AutoCloseAttendance(_ context.Context, _ pgx.Tx, p AutoCloseRow) (string, bool, error) {
	if f.closed == nil {
		f.closed = make(map[string]AutoCloseRow)
	}
	if _, exists := f.closed[p.ID]; exists {
		return "", false, nil
	}
	f.closed[p.ID] = p
	return p.ID, true, nil
}

// --- helpers ---

func autoCloseCandidate(id string, checkInAt, shiftEndAt time.Time) StaleOpenAttendance {
	return StaleOpenAttendance{
		ID:         id,
		CheckInAt:  &checkInAt,
		ShiftEndAt: &shiftEndAt,
	}
}

func TestAutoCloseSweep(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	grace := 48 * time.Hour
	staleEnd := now.Add(-50 * time.Hour)
	checkInAt := staleEnd.Add(-8 * time.Hour)

	t.Run("closes stale records with AUTO_CLOSED flag", func(t *testing.T) {
		repo := &fakeAutoCloseRepo{
			stale: []StaleOpenAttendance{
				autoCloseCandidate("SWP-ATT-1", checkInAt, staleEnd),
				autoCloseCandidate("SWP-ATT-2", checkInAt, staleEnd),
			},
		}
		svc := NewAutoCloseSweepService(repo, autoCloseFakeRunner{}, grace, 0)
		svc.SetClock(func() time.Time { return now })

		closed, err := svc.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if closed != 2 {
			t.Fatalf("closed = %d, want 2", closed)
		}
		if len(repo.closed) != 2 {
			t.Fatalf("closed map size = %d, want 2", len(repo.closed))
		}
		for _, row := range repo.closed {
			if row.Status != string(att.StatusIncomplete) {
				t.Errorf("status = %s, want INCOMPLETE", row.Status)
			}
			if row.VerificationStatus != string(att.VerificationPending) {
				t.Errorf("verification_status = %s, want PENDING", row.VerificationStatus)
			}
			hasAutoClosed := false
			for _, f := range row.Flags {
				if f == string(att.FlagAutoClosed) {
					hasAutoClosed = true
					break
				}
			}
			if !hasAutoClosed {
				t.Errorf("flags %v should contain AUTO_CLOSED", row.Flags)
			}
		}
	})

	t.Run("skips recent records within grace", func(t *testing.T) {
		recentEnd := now.Add(-1 * time.Hour)
		repo := &fakeAutoCloseRepo{
			stale: []StaleOpenAttendance{
				autoCloseCandidate("SWP-ATT-3", recentEnd.Add(-8*time.Hour), recentEnd),
			},
		}
		svc := NewAutoCloseSweepService(repo, autoCloseFakeRunner{}, grace, 0)
		svc.SetClock(func() time.Time { return now })

		closed, err := svc.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if closed != 0 {
			t.Errorf("closed = %d, want 0 (within grace)", closed)
		}
	})

	t.Run("already closed idempotent", func(t *testing.T) {
		repo := &fakeAutoCloseRepo{
			stale: []StaleOpenAttendance{
				autoCloseCandidate("SWP-ATT-4", checkInAt, staleEnd),
			},
			closed: map[string]AutoCloseRow{
				"SWP-ATT-4": {ID: "SWP-ATT-4"},
			},
		}
		svc := NewAutoCloseSweepService(repo, autoCloseFakeRunner{}, grace, 0)
		svc.SetClock(func() time.Time { return now })

		closed, err := svc.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if closed != 0 {
			t.Errorf("closed = %d, want 0 (already closed)", closed)
		}
	})

	t.Run("empty results no error", func(t *testing.T) {
		repo := &fakeAutoCloseRepo{}
		svc := NewAutoCloseSweepService(repo, autoCloseFakeRunner{}, grace, 0)
		svc.SetClock(func() time.Time { return now })

		closed, err := svc.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if closed != 0 {
			t.Errorf("closed = %d, want 0", closed)
		}
	})

	t.Run("worked minutes correct", func(t *testing.T) {
		checkIn := time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC)
		shiftEnd := time.Date(2026, 6, 5, 17, 0, 0, 0, time.UTC)
		repo := &fakeAutoCloseRepo{
			stale: []StaleOpenAttendance{
				{ID: "SWP-ATT-5", CheckInAt: &checkIn, ShiftEndAt: &shiftEnd},
			},
		}
		svc := NewAutoCloseSweepService(repo, autoCloseFakeRunner{}, grace, 0)
		svc.SetClock(func() time.Time { return now })

		closed, err := svc.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if closed != 1 {
			t.Fatalf("closed = %d, want 1", closed)
		}
		row := repo.closed["SWP-ATT-5"]
		if row.WorkedMinutes != 540 {
			t.Errorf("WorkedMinutes = %d, want 540", row.WorkedMinutes)
		}
		if !row.CheckOutAt.Equal(shiftEnd) {
			t.Errorf("CheckOutAt = %v, want %v", row.CheckOutAt, shiftEnd)
		}
	})
}
