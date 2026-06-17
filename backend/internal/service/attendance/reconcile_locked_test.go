// Package attendance — F8.5 PC-11 test: F5.7 auto-reconcile is INERT on a LOCKED
// payroll month. A late roster still inserts the shift (the caller does that), but
// ReconcileForEntry must short-circuit before touching any attendance record. Uses
// the same fakeReconcileRepo as reconcile_test.go + a fake LockedMonthChecker.
package attendance

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// fakeLockedChecker reports a fixed locked state (and records the (year, month) asked).
type fakeLockedChecker struct {
	locked    bool
	err       error
	gotYear   int
	gotMonth  int
	wasCalled bool
}

func (f *fakeLockedChecker) IsMonthLocked(_ context.Context, year, month int) (bool, error) {
	f.wasCalled = true
	f.gotYear, f.gotMonth = year, month
	return f.locked, f.err
}

func TestReconcile_InertOnLockedMonth(t *testing.T) {
	repo := &fakeReconcileRepo{candidate: machineCand(6, 50), candFound: true}
	chk := &fakeLockedChecker{locked: true}
	svc := NewReconcileService(repo)
	svc.SetLockedMonthChecker(chk)

	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err != nil {
		t.Fatalf("ReconcileForEntry err = %v, want nil (no-op)", err)
	}
	if !chk.wasCalled {
		t.Fatalf("locked checker not consulted")
	}
	// The shift is dated 2026-06-17 (reconcileEntry) → June 2026.
	if chk.gotYear != 2026 || chk.gotMonth != 6 {
		t.Fatalf("checked (%d, %d), want (2026, 6)", chk.gotYear, chk.gotMonth)
	}
	// PC-11: NO candidate lookup, NO reconcile, NO relink on a locked month.
	if repo.findCalled {
		t.Fatalf("FindCandidate called on a LOCKED month — reconcile must be inert (PC-11)")
	}
	if repo.reconciled != nil || repo.relinked != nil {
		t.Fatalf("a locked-month record was mutated — reconcile must be inert (PC-11)")
	}
}

func TestReconcile_RunsOnOpenMonth(t *testing.T) {
	repo := &fakeReconcileRepo{candidate: machineCand(6, 50), candFound: true}
	chk := &fakeLockedChecker{locked: false}
	svc := NewReconcileService(repo)
	svc.SetLockedMonthChecker(chk)

	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err != nil {
		t.Fatalf("ReconcileForEntry err = %v, want nil", err)
	}
	if !repo.findCalled {
		t.Fatalf("FindCandidate not called on an OPEN month — reconcile must run (PC-11)")
	}
	if repo.reconciled == nil {
		t.Fatalf("machine-owned candidate not reconciled on an OPEN month")
	}
}

func TestReconcile_NoCheckerStillRuns(t *testing.T) {
	// Pre-F8.5 behavior: a service with no locked-month checker always reconciles.
	repo := &fakeReconcileRepo{candidate: machineCand(6, 50), candFound: true}
	svc := NewReconcileService(repo) // no SetLockedMonthChecker

	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err != nil {
		t.Fatalf("ReconcileForEntry err = %v, want nil", err)
	}
	if !repo.findCalled || repo.reconciled == nil {
		t.Fatalf("reconcile did not run without a locked-month checker (regression)")
	}
}

func TestReconcile_CheckerErrorPropagates(t *testing.T) {
	repo := &fakeReconcileRepo{candidate: machineCand(6, 50), candFound: true}
	chk := &fakeLockedChecker{err: errors.New("db down")}
	svc := NewReconcileService(repo)
	svc.SetLockedMonthChecker(chk)

	err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry())
	if err == nil {
		t.Fatalf("ReconcileForEntry err = nil, want the checker error to propagate (fail-closed)")
	}
	if repo.findCalled {
		t.Fatalf("FindCandidate called despite a checker error — must fail closed (PC-11)")
	}
}

var _ = pgx.ErrNoRows
