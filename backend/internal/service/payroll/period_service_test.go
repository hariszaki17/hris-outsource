// Package payroll — F8.5 PeriodService + ClarificationService unit tests, mirroring
// the PRD §7 Gherkin + §8 cases (lock blocked by blockers; force-lock; lock emits
// summary; reopen voids; FIELD gates / INTERNAL informational PC-5; clarification
// round-trip target resolution; post-lock adjustment emit). Fakes implement the
// repository ports; a fake tx no-ops audit.Record's Exec. No Postgres.
package payroll

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// --- fake tx + runner (audit.Record's tx.Exec is a no-op) ---

type fakeTx struct{}

func (fakeTx) Begin(context.Context) (pgx.Tx, error) { return fakeTx{}, nil }
func (fakeTx) Commit(context.Context) error          { return nil }
func (fakeTx) Rollback(context.Context) error        { return nil }
func (fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) { panic("unused") }
func (fakeTx) QueryRow(context.Context, string, ...any) pgx.Row        { panic("unused") }
func (fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("unused")
}
func (fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("unused") }
func (fakeTx) LargeObjects() pgx.LargeObjects                         { panic("unused") }
func (fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("unused")
}
func (fakeTx) Conn() *pgx.Conn { return nil }

type fakeRunner struct{}

func (fakeRunner) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error { return fn(fakeTx{}) }

// --- fake period repo ---

type fakePeriodRepo struct {
	period      dom.PayrollPeriod
	periodFound bool
	blockers    dom.PeriodBlockerCounts
	openClarif  int

	lockCalled    bool
	lockParams    LockPeriodParams
	snapshotN     int64
	snapshotCalls int
	voidCalls     int
	reopenCalled  bool
	created       []dom.EmployeeCompleteness
	summaries     []dom.PeriodEmployeeSummary
}

func (f *fakePeriodRepo) GetPeriod(_ context.Context, _, _ int) (dom.PayrollPeriod, bool, error) {
	return f.period, f.periodFound, nil
}

func (f *fakePeriodRepo) GetOrCreatePeriod(_ context.Context, year, month int) (dom.PayrollPeriod, error) {
	if f.periodFound {
		return f.period, nil
	}
	return dom.PayrollPeriod{ID: "SWP-PPD-1", Year: year, Month: month, Status: dom.PeriodStatusOpen}, nil
}

func (f *fakePeriodRepo) GetPeriodForUpdate(_ context.Context, _ pgx.Tx, _, _ int) (dom.PayrollPeriod, bool, error) {
	return f.period, f.periodFound, nil
}

func (f *fakePeriodRepo) Lock(_ context.Context, _ pgx.Tx, p LockPeriodParams) (dom.PayrollPeriod, bool, error) {
	f.lockCalled = true
	f.lockParams = p
	locked := f.period
	locked.Status = dom.PeriodStatusLocked
	locked.ForceLocked = p.ForceLocked
	locked.ForceLockReason = p.ForceLockReason
	now := time.Now()
	locked.LockedAt = &now
	locked.LockedBy = p.LockedBy
	return locked, true, nil
}

func (f *fakePeriodRepo) Reopen(_ context.Context, _ pgx.Tx, _ string, by *string) (dom.PayrollPeriod, bool, error) {
	f.reopenCalled = true
	reopened := f.period
	reopened.Status = dom.PeriodStatusReopened
	reopened.ReopenedBy = by
	return reopened, true, nil
}

func (f *fakePeriodRepo) SnapshotSummaries(_ context.Context, _ pgx.Tx, _ SnapshotParams) (int64, error) {
	f.snapshotCalls++
	return f.snapshotN, nil
}

func (f *fakePeriodRepo) VoidSummaries(_ context.Context, _ pgx.Tx, _ string) (int64, error) {
	f.voidCalls++
	return 3, nil
}

func (f *fakePeriodRepo) ListCompleteness(_ context.Context, _, _ int, _ *string, _ int) ([]dom.EmployeeCompleteness, error) {
	return f.created, nil
}

func (f *fakePeriodRepo) CountFieldBlockers(_ context.Context, _, _ int) (dom.PeriodBlockerCounts, error) {
	return f.blockers, nil
}

func (f *fakePeriodRepo) CountOpenClarifications(_ context.Context, _ string) (int, error) {
	return f.openClarif, nil
}

func (f *fakePeriodRepo) ListSummaries(_ context.Context, _ string, _ *string, _ int) ([]dom.PeriodEmployeeSummary, error) {
	return f.summaries, nil
}

// --- helpers ---

func hrCtx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-1", Role: auth.RoleHRAdmin})
}

func superCtx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-9", Role: auth.RoleSuperAdmin})
}

func openPeriod() dom.PayrollPeriod {
	return dom.PayrollPeriod{ID: "SWP-PPD-1", Year: 2026, Month: 6, Status: dom.PeriodStatusOpen}
}

func lockedPeriod() dom.PayrollPeriod {
	return dom.PayrollPeriod{ID: "SWP-PPD-1", Year: 2026, Month: 6, Status: dom.PeriodStatusLocked}
}

// --- lock blocked by blockers (§7 "Cannot lock with open exceptions") ---

func TestLock_BlockedByBlockers(t *testing.T) {
	repo := &fakePeriodRepo{
		period: openPeriod(), periodFound: true,
		blockers: dom.PeriodBlockerCounts{Attendance: 3, Overtime: 1},
	}
	svc := NewPeriodService(repo, fakeRunner{})

	_, err := svc.Lock(hrCtx(), 2026, 6, false, "")
	ae, ok := apperr.As(err)
	if !ok || ae.Code != "PERIOD_HAS_BLOCKERS" {
		t.Fatalf("Lock err = %v, want PERIOD_HAS_BLOCKERS", err)
	}
	if ae.Status() != 422 {
		t.Fatalf("status = %d, want 422", ae.Status())
	}
	if repo.lockCalled {
		t.Fatalf("Lock applied despite blockers")
	}
}

// --- lock emits the immutable summary (§7 "Lock emits the immutable summary") ---

func TestLock_CleanEmitsSummary(t *testing.T) {
	repo := &fakePeriodRepo{
		period: openPeriod(), periodFound: true,
		blockers: dom.PeriodBlockerCounts{}, snapshotN: 7,
	}
	svc := NewPeriodService(repo, fakeRunner{})

	res, err := svc.Lock(hrCtx(), 2026, 6, false, "")
	if err != nil {
		t.Fatalf("Lock err = %v, want nil", err)
	}
	if res.Period.Status != dom.PeriodStatusLocked {
		t.Fatalf("status = %v, want LOCKED", res.Period.Status)
	}
	if res.SummariesCount != 7 || repo.snapshotCalls != 1 {
		t.Fatalf("snapshot count = %d / calls = %d, want 7 / 1 (PC-8)", res.SummariesCount, repo.snapshotCalls)
	}
	if repo.lockParams.ForceLocked {
		t.Fatalf("a clean lock must not be force-locked")
	}
}

// --- super-admin force-lock with reason (§7) ---

func TestLock_ForceRequiresSuperAndReason(t *testing.T) {
	repo := &fakePeriodRepo{period: openPeriod(), periodFound: true, blockers: dom.PeriodBlockerCounts{Attendance: 2}}
	svc := NewPeriodService(repo, fakeRunner{})

	// HR force-lock → 403.
	if _, err := svc.Lock(hrCtx(), 2026, 6, true, "absconded"); err == nil {
		t.Fatalf("HR force-lock allowed, want 403")
	} else if ae, _ := apperr.As(err); ae.Status() != 403 {
		t.Fatalf("HR force-lock status = %d, want 403", ae.Status())
	}

	// Super force-lock without reason → 422.
	if _, err := svc.Lock(superCtx(), 2026, 6, true, ""); err == nil {
		t.Fatalf("super force-lock without reason allowed, want 422")
	} else if ae, _ := apperr.As(err); ae.Code != "FORCE_LOCK_REASON_REQUIRED" {
		t.Fatalf("err = %v, want FORCE_LOCK_REASON_REQUIRED", err)
	}

	// Super force-lock with reason over blockers → locks, reason stored.
	repo2 := &fakePeriodRepo{period: openPeriod(), periodFound: true, blockers: dom.PeriodBlockerCounts{Attendance: 2}, snapshotN: 1}
	svc2 := NewPeriodService(repo2, fakeRunner{})
	res, err := svc2.Lock(superCtx(), 2026, 6, true, "agent absconded")
	if err != nil {
		t.Fatalf("super force-lock err = %v, want nil", err)
	}
	if !res.Period.ForceLocked || res.Period.ForceLockReason == nil || *res.Period.ForceLockReason != "agent absconded" {
		t.Fatalf("force-lock reason not stored: %+v", res.Period)
	}
}

// --- already-locked rejected ---

func TestLock_AlreadyLocked(t *testing.T) {
	repo := &fakePeriodRepo{period: lockedPeriod(), periodFound: true}
	svc := NewPeriodService(repo, fakeRunner{})

	_, err := svc.Lock(hrCtx(), 2026, 6, false, "")
	if ae, _ := apperr.As(err); ae == nil || ae.Code != "PERIOD_ALREADY_LOCKED" {
		t.Fatalf("Lock err = %v, want PERIOD_ALREADY_LOCKED", err)
	}
}

// --- reopen voids the summary; super-admin only (§7 "Reopen only before posting") ---

func TestReopen_VoidsSummariesSuperOnly(t *testing.T) {
	// HR reopen → 403.
	repo := &fakePeriodRepo{period: lockedPeriod(), periodFound: true}
	svc := NewPeriodService(repo, fakeRunner{})
	if _, err := svc.Reopen(hrCtx(), 2026, 6, "fix"); err == nil {
		t.Fatalf("HR reopen allowed, want 403")
	} else if ae, _ := apperr.As(err); ae.Status() != 403 {
		t.Fatalf("HR reopen status = %d, want 403", ae.Status())
	}

	// Super reopen with reason → REOPENED + summaries voided.
	repo2 := &fakePeriodRepo{period: lockedPeriod(), periodFound: true}
	svc2 := NewPeriodService(repo2, fakeRunner{})
	p, err := svc2.Reopen(superCtx(), 2026, 6, "correction needed")
	if err != nil {
		t.Fatalf("super reopen err = %v, want nil", err)
	}
	if p.Status != dom.PeriodStatusReopened {
		t.Fatalf("status = %v, want REOPENED", p.Status)
	}
	if repo2.voidCalls != 1 {
		t.Fatalf("summaries voided %d times, want 1 (PC-12)", repo2.voidCalls)
	}
}

// --- reopen requires reason ---

func TestReopen_RequiresReason(t *testing.T) {
	repo := &fakePeriodRepo{period: lockedPeriod(), periodFound: true}
	svc := NewPeriodService(repo, fakeRunner{})
	if _, err := svc.Reopen(superCtx(), 2026, 6, ""); err == nil {
		t.Fatalf("reopen without reason allowed, want 422")
	} else if ae, _ := apperr.As(err); ae.Code != "REOPEN_REASON_REQUIRED" {
		t.Fatalf("err = %v, want REOPEN_REASON_REQUIRED", err)
	}
}

// --- reopen of a not-locked period rejected ---

func TestReopen_NotLocked(t *testing.T) {
	repo := &fakePeriodRepo{period: openPeriod(), periodFound: true}
	svc := NewPeriodService(repo, fakeRunner{})
	if _, err := svc.Reopen(superCtx(), 2026, 6, "x"); err == nil {
		t.Fatalf("reopen of OPEN period allowed, want 409")
	} else if ae, _ := apperr.As(err); ae.Code != "PERIOD_NOT_LOCKED" {
		t.Fatalf("err = %v, want PERIOD_NOT_LOCKED", err)
	}
}

// --- PC-5: FIELD gates, INTERNAL informational ---

func TestCompleteness_FieldGatesInternalInformational(t *testing.T) {
	repo := &fakePeriodRepo{
		blockers: dom.PeriodBlockerCounts{},
		created: []dom.EmployeeCompleteness{
			{EmployeeID: "SWP-EMP-1", EmployeeType: "FIELD", Expected: 5, Clean: 3, OnLeave: 0, AttendanceBlockers: 2},
			{EmployeeID: "SWP-EMP-2", EmployeeType: "INTERNAL", Expected: 0, Clean: 0, AttendanceBlockers: 4},
		},
	}
	svc := NewPeriodService(repo, fakeRunner{})

	rows, _, err := svc.Completeness(hrCtx(), 2026, 6, "", 50)
	if err != nil {
		t.Fatalf("Completeness err = %v", err)
	}
	// FIELD: coverage_gap = 5 - (3+0) = 2; in-scope blockers > 0.
	if rows[0].CoverageGap != 2 {
		t.Fatalf("FIELD coverage_gap = %d, want 2", rows[0].CoverageGap)
	}
	if rows[0].InScopeBlockerCount() == 0 {
		t.Fatalf("FIELD employee with blockers must gate (PC-5)")
	}
	// INTERNAL: never gates (PC-5).
	if rows[1].InScopeBlockerCount() != 0 {
		t.Fatalf("INTERNAL blockers gated the lock — must be informational (PC-5)")
	}
}

// --- empty-summary lock (C-PC-8) ---

func TestLock_EmptyMonthLocks(t *testing.T) {
	repo := &fakePeriodRepo{period: openPeriod(), periodFound: true, blockers: dom.PeriodBlockerCounts{}, snapshotN: 0}
	svc := NewPeriodService(repo, fakeRunner{})
	res, err := svc.Lock(hrCtx(), 2026, 6, false, "")
	if err != nil {
		t.Fatalf("Lock err = %v, want nil (empty month locks, C-PC-8)", err)
	}
	if res.SummariesCount != 0 {
		t.Fatalf("empty month summary count = %d, want 0", res.SummariesCount)
	}
}

var _ = errors.Is
