// Package scheduling — F5.7 trigger test: scheduling.CreateEntry invokes the
// attendance auto-reconcile side-effect inside the create tx, but ONLY for a
// workable shift (AR-1). A day-off entry and a force_replace (MODIFIED) entry must
// NOT trigger reconcile. Mirrors the §7 "Bulk roster reconciles each cell" /
// "Day-off entry does not adopt" scenarios at the trigger boundary.
package scheduling

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	attsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// --- fakes (scheduling service package) ---

type recFakeTx struct{}

func (recFakeTx) Begin(context.Context) (pgx.Tx, error) { return recFakeTx{}, nil }
func (recFakeTx) Commit(context.Context) error          { return nil }
func (recFakeTx) Rollback(context.Context) error        { return nil }
func (recFakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (recFakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) { panic("Query unused") }
func (recFakeTx) QueryRow(context.Context, string, ...any) pgx.Row        { panic("QueryRow unused") }
func (recFakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (recFakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("SendBatch unused") }
func (recFakeTx) LargeObjects() pgx.LargeObjects                         { panic("LargeObjects unused") }
func (recFakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (recFakeTx) Conn() *pgx.Conn { panic("Conn unused") }

type recFakeRunner struct{}

func (recFakeRunner) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return fn(recFakeTx{})
}

// recFakeReconciler records the ReconcileForEntry calls.
type recFakeReconciler struct{ calls []attsvc.ReconcileEntry }

func (r *recFakeReconciler) ReconcileForEntry(_ context.Context, _ pgx.Tx, e attsvc.ReconcileEntry) error {
	r.calls = append(r.calls, e)
	return nil
}

// recFakeRepo is a minimal ScheduleRepository for CreateEntry. The agent has an
// active placement covering the date at company SWP-CMP-1; shift master SWP-SM-1 is
// active 07:00–15:00. No live entry exists (unless preLive is set, for the replace
// path).
type recFakeRepo struct {
	preLive *LiveEntry // FindLiveEntryForAgentDate result (nil ⇒ not found)
	seq     int
}

func (r *recFakeRepo) FindActivePlacementForAgentDate(_ context.Context, _ string, _ time.Time) (PlacementCover, error) {
	return PlacementCover{PlacementID: "SWP-PL-1", CompanyID: "SWP-CMP-1", StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, nil
}

func (r *recFakeRepo) GetShiftMaster(_ context.Context, _ string) (domain.ShiftMaster, error) {
	return domain.ShiftMaster{ID: "SWP-SM-1", IsActive: true, StartTime: "07:00", EndTime: "15:00"}, nil
}

func (r *recFakeRepo) FindApprovedLeaveForAgentDate(_ context.Context, _ string, _ time.Time) (ApprovedLeave, error) {
	return ApprovedLeave{}, domain.ErrNotFound
}

func (r *recFakeRepo) FindLiveEntryForAgentDate(_ context.Context, _ string, _ time.Time) (LiveEntry, error) {
	if r.preLive != nil {
		return *r.preLive, nil
	}
	return LiveEntry{}, domain.ErrNotFound
}

func (r *recFakeRepo) ListSchedule(_ context.Context, _ domain.ScheduleFilter) ([]domain.ScheduleEntry, error) {
	return nil, nil
}
func (r *recFakeRepo) ListScheduleByAgent(_ context.Context, _ string, _, _ time.Time) ([]domain.ScheduleEntry, error) {
	return nil, nil
}
func (r *recFakeRepo) GetActivePlacementCompanyForEmployee(_ context.Context, _ string) (string, error) {
	return "SWP-CMP-1", nil
}

func (r *recFakeRepo) GetScheduleEntry(_ context.Context, id string) (domain.ScheduleEntry, error) {
	return domain.ScheduleEntry{}, domain.ErrNotFound // forces CreateEntry to return `created`
}
func (r *recFakeRepo) GetScheduleEntryForUpdate(_ context.Context, _ pgx.Tx, id string) (domain.ScheduleEntry, error) {
	return domain.ScheduleEntry{}, domain.ErrNotFound
}

// FindLiveEntryForAgentDateTx: the in-tx DOUBLE_SHIFT re-check. Not found unless
// preLive is set AND not force-replacing.
func (r *recFakeRepo) FindLiveEntryForAgentDateTx(_ context.Context, _ pgx.Tx, _ string, _ time.Time) (LiveEntry, error) {
	return LiveEntry{}, domain.ErrNotFound
}

func (r *recFakeRepo) CreateScheduleEntry(_ context.Context, _ pgx.Tx, p CreateScheduleEntryParams) (domain.ScheduleEntry, error) {
	r.seq++
	return domain.ScheduleEntry{
		ID:            "SWP-SCH-100" + strconv.Itoa(r.seq),
		EmployeeID:    p.EmployeeID,
		PlacementID:   p.PlacementID,
		CompanyID:     "SWP-CMP-1",
		ShiftMasterID: p.ShiftMasterID,
		StartTime:     p.StartTime,
		EndTime:       p.EndTime,
		CrossMidnight: p.CrossMidnight,
		WorkDate:      p.WorkDate,
		Status:        p.Status,
		IsDayOff:      p.IsDayOff,
	}, nil
}

func (r *recFakeRepo) UpdateScheduleEntry(_ context.Context, _ pgx.Tx, _ UpdateScheduleEntryParams) (domain.ScheduleEntry, error) {
	return domain.ScheduleEntry{}, nil
}
func (r *recFakeRepo) SoftDeleteScheduleEntry(_ context.Context, _ pgx.Tx, _ string) (int64, error) {
	return 1, nil
}

func (r *recFakeRepo) CancelFutureSchedulesForEmployee(_ context.Context, _ pgx.Tx, employeeID string, afterDate time.Time) error {
	return nil
}

func (r *recFakeRepo) ListScheduleForAggregate(_ context.Context, companyID string, start, end time.Time) ([]domain.ScheduleEntry, error) {
	return nil, nil
}

var _ ScheduleRepository = (*recFakeRepo)(nil)

func hrCtx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "SWP-USR-HR", Role: auth.RoleHRAdmin,
	})
}

func newReconcileSvc(repo ScheduleRepository) (*ScheduleService, *recFakeReconciler) {
	rec := &recFakeReconciler{}
	s := NewScheduleService(repo, recFakeRunner{})
	s.SetClock(func() time.Time { return time.Date(2026, 6, 17, 3, 0, 0, 0, time.UTC) })
	s.SetReconciler(rec)
	return s, rec
}

func TestCreateEntry_WorkableShift_TriggersReconcile(t *testing.T) {
	repo := &recFakeRepo{}
	s, rec := newReconcileSvc(repo)
	sm := "SWP-SM-1"
	date := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)

	_, err := s.CreateEntry(hrCtx(), CreateEntryRequest{EmployeeID: "SWP-EMP-1", ShiftMasterID: &sm, Date: date})
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("reconcile calls = %d, want 1 (workable shift)", len(rec.calls))
	}
	got := rec.calls[0]
	if got.EmployeeID != "SWP-EMP-1" || got.PlacementID != "SWP-PL-1" {
		t.Errorf("reconcile entry = %+v, want employee SWP-EMP-1 / placement SWP-PL-1", got)
	}
	if got.StartTime != "07:00" || got.EndTime != "15:00" {
		t.Errorf("reconcile window = %s-%s, want 07:00-15:00", got.StartTime, got.EndTime)
	}
	if got.ScheduleID == "" {
		t.Error("reconcile entry missing schedule_id")
	}
}

func TestCreateEntry_DayOff_SkipsReconcile(t *testing.T) {
	// AR-1 / C-AR-2: an is_day_off entry is not a workable shift — no reconcile.
	repo := &recFakeRepo{}
	s, rec := newReconcileSvc(repo)
	date := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)

	_, err := s.CreateEntry(hrCtx(), CreateEntryRequest{EmployeeID: "SWP-EMP-1", Date: date, IsDayOff: true})
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("reconcile calls = %d, want 0 (day-off)", len(rec.calls))
	}
}

func TestCreateEntry_ForceReplace_SkipsReconcile(t *testing.T) {
	// AR-1 / C-AR-6: force_replace produces a MODIFIED entry superseding an existing
	// one — that day was already linked, so no reconcile.
	repo := &recFakeRepo{preLive: &LiveEntry{ID: "SWP-SCH-OLD", Status: "SCHEDULED"}}
	s, rec := newReconcileSvc(repo)
	sm := "SWP-SM-1"
	date := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)

	entry, err := s.CreateEntry(hrCtx(), CreateEntryRequest{EmployeeID: "SWP-EMP-1", ShiftMasterID: &sm, Date: date, ForceReplace: true})
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	if entry.Status != "MODIFIED" {
		t.Fatalf("status = %q, want MODIFIED (replace path)", entry.Status)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("reconcile calls = %d, want 0 (force_replace)", len(rec.calls))
	}
}
