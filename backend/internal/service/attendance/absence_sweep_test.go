// Package attendance — unit tests for the absence-sweep service. A fake repo holds
// the candidate set + records created rows (and simulates ON CONFLICT no-ops on a
// re-run); a fake clock + fakeRunner (reused tx stub) let Sweep run without Postgres.
package attendance

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- fakeTx / fakeRunner (only Exec is exercised, by audit.Record) ---

type sweepFakeTx struct{}

func (sweepFakeTx) Begin(context.Context) (pgx.Tx, error) { return sweepFakeTx{}, nil }
func (sweepFakeTx) Commit(context.Context) error          { return nil }
func (sweepFakeTx) Rollback(context.Context) error        { return nil }
func (sweepFakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (sweepFakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) { panic("Query unused") }
func (sweepFakeTx) QueryRow(context.Context, string, ...any) pgx.Row       { panic("QueryRow unused") }
func (sweepFakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("CopyFrom unused")
}
func (sweepFakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("SendBatch unused") }
func (sweepFakeTx) LargeObjects() pgx.LargeObjects                         { panic("LargeObjects unused") }
func (sweepFakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("Prepare unused")
}
func (sweepFakeTx) Conn() *pgx.Conn { panic("Conn unused") }

type sweepFakeRunner struct{}

func (sweepFakeRunner) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return fn(sweepFakeTx{})
}

// --- fake absence-sweep repo ---

// fakeAbsenceRepo serves a fixed candidate list and tracks inserts. existing models
// the partial-unique-index state: a schedule_id already in `existing` collides
// (CreateAbsentAttendance returns created=false, mirroring ON CONFLICT DO NOTHING).
type fakeAbsenceRepo struct {
	candidates    []AbsenceCandidate
	existing      map[string]bool      // schedule_ids that already have an attendance row
	created       []CreateAbsentParams // inserts that actually happened
	findErr       error
	forceConflict bool // every insert hits ON CONFLICT (concurrent-write simulation)
	nextID        int
}

// newConflictingAbsenceRepo returns candidates from Find (existing empty so they are
// not pre-filtered) but makes every insert a no-op, modeling a row that appeared
// between Find and the insert.
func newConflictingAbsenceRepo(cs ...AbsenceCandidate) *fakeAbsenceRepo {
	return &fakeAbsenceRepo{candidates: cs, forceConflict: true}
}

func (f *fakeAbsenceRepo) FindUnreportedAbsences(_ context.Context, cutoff time.Time, limit int) ([]AbsenceCandidate, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	out := make([]AbsenceCandidate, 0, len(f.candidates))
	for _, c := range f.candidates {
		// Mirror the SQL: ended before cutoff AND not already reported.
		if !c.ShiftEndAt.Before(cutoff) {
			continue
		}
		if f.existing[c.ScheduleID] {
			continue
		}
		out = append(out, c)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeAbsenceRepo) CreateAbsentAttendance(_ context.Context, _ pgx.Tx, p CreateAbsentParams) (string, bool, error) {
	if f.forceConflict || f.existing[p.ScheduleID] {
		return "", false, nil // ON CONFLICT DO NOTHING
	}
	if f.existing == nil {
		f.existing = map[string]bool{}
	}
	f.existing[p.ScheduleID] = true
	f.created = append(f.created, p)
	f.nextID++
	return "SWP-ATT-9000" + string(rune('0'+f.nextID)), true, nil
}

// --- helpers ---

func fixedClock(t time.Time) Clock { return func() time.Time { return t } }

func candidate(scheduleID string, endAt time.Time) AbsenceCandidate {
	return AbsenceCandidate{
		ScheduleID:   scheduleID,
		EmployeeID:   "SWP-EMP-0001",
		PlacementID:  "SWP-PL-0001",
		CompanyID:    "SWP-CMP-0021",
		SiteID:       "SWP-SITE-0001",
		PositionID:   "SWP-POS-014",
		ServiceLine:  "building_management",
		ShiftStartAt: endAt.Add(-8 * time.Hour),
		ShiftEndAt:   endAt,
	}
}

func TestSweep(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	grace := 30 * time.Minute
	// cutoff = now - 30m = 11:30. A shift ending before 11:30 is overdue.
	overdueEnd := now.Add(-2 * time.Hour) // ended 10:00 → > grace ago
	withinGrace := now.Add(-5 * time.Minute)

	t.Run("creates one ABSENT row for an overdue unreported shift", func(t *testing.T) {
		repo := &fakeAbsenceRepo{candidates: []AbsenceCandidate{candidate("SWP-SCH-1", overdueEnd)}}
		svc := NewAbsenceSweepService(repo, sweepFakeRunner{}, grace, 0)
		svc.SetClock(fixedClock(now))

		created, err := svc.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if created != 1 {
			t.Fatalf("created = %d, want 1", created)
		}
		if len(repo.created) != 1 {
			t.Fatalf("inserts = %d, want 1", len(repo.created))
		}
		got := repo.created[0]
		if got.ScheduleID != "SWP-SCH-1" {
			t.Errorf("schedule_id = %q, want SWP-SCH-1", got.ScheduleID)
		}
		if got.CompanyID != "SWP-CMP-0021" || got.SiteID != "SWP-SITE-0001" || got.PositionID != "SWP-POS-014" {
			t.Errorf("denorm fields not set: %+v", got)
		}
		if got.ServiceLine != "building_management" {
			t.Errorf("service_line = %q, want building_management", got.ServiceLine)
		}
		if got.ShiftStartAt.IsZero() || got.ShiftEndAt.IsZero() {
			t.Errorf("shift window not set: %+v", got)
		}
	})

	t.Run("idempotent: a second sweep over the same state creates 0", func(t *testing.T) {
		repo := &fakeAbsenceRepo{candidates: []AbsenceCandidate{candidate("SWP-SCH-1", overdueEnd)}}
		svc := NewAbsenceSweepService(repo, sweepFakeRunner{}, grace, 0)
		svc.SetClock(fixedClock(now))

		first, err := svc.Sweep(context.Background())
		if err != nil || first != 1 {
			t.Fatalf("first sweep: created=%d err=%v", first, err)
		}
		second, err := svc.Sweep(context.Background())
		if err != nil {
			t.Fatalf("second sweep: %v", err)
		}
		if second != 0 {
			t.Fatalf("second sweep created = %d, want 0 (idempotent)", second)
		}
		if len(repo.created) != 1 {
			t.Fatalf("total inserts = %d, want 1", len(repo.created))
		}
	})

	t.Run("ON CONFLICT no-op (row already exists) creates 0", func(t *testing.T) {
		// A row already exists for this schedule (e.g. a concurrent clock-in). The
		// insert hits the partial unique index → DO NOTHING → created=false, counted 0.
		repo := newConflictingAbsenceRepo(candidate("SWP-SCH-1", overdueEnd))
		svc := NewAbsenceSweepService(repo, sweepFakeRunner{}, grace, 0)
		svc.SetClock(fixedClock(now))

		created, err := svc.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if created != 0 {
			t.Fatalf("created = %d, want 0", created)
		}
		if len(repo.created) != 0 {
			t.Fatalf("inserts = %d, want 0", len(repo.created))
		}
	})

	t.Run("day-off and within-grace shifts are NOT swept", func(t *testing.T) {
		// The day-off shift never appears as a candidate (Find excludes is_day_off),
		// so it is represented by simply not being in the candidate list. The
		// within-grace shift IS a candidate row but ends after cutoff → filtered out.
		repo := &fakeAbsenceRepo{candidates: []AbsenceCandidate{
			candidate("SWP-SCH-WITHIN-GRACE", withinGrace),
		}}
		svc := NewAbsenceSweepService(repo, sweepFakeRunner{}, grace, 0)
		svc.SetClock(fixedClock(now))

		created, err := svc.Sweep(context.Background())
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if created != 0 {
			t.Fatalf("created = %d, want 0 (within grace must not be swept)", created)
		}
		if len(repo.created) != 0 {
			t.Fatalf("inserts = %d, want 0", len(repo.created))
		}
	})
}
