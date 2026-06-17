// Package attendance — unit tests for F5.7 Attendance Auto-Reconcile (AR-1..AR-10),
// mirroring the PRD §7 Gherkin + §8 cases. Uses a fake ReconcileRepo + the shared
// sweepFakeRunner/sweepFakeTx (audit.Record Exec is a no-op). No Postgres. The
// re-derivation is checked against the SAME 15-min grace clock-in uses, so a
// reconciled record is indistinguishable from a shift-backed one.
package attendance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
)

// fakeReconcileRepo records the Reconcile / Relink calls so the test asserts on the
// derived payload. candidate is what FindCandidate returns (found=candFound).
type fakeReconcileRepo struct {
	candidate  ReconcileCandidate
	candFound  bool
	findErr    error
	findCalled bool
	winStart   time.Time
	winEnd     time.Time

	reconciled *ReconcileParams
	relinked   *RelinkParams
	guardNoop  bool // when true, the write returns found=false (concurrent link)
}

func (f *fakeReconcileRepo) FindCandidate(_ context.Context, _ pgx.Tx, _, _ string, ws, we time.Time) (ReconcileCandidate, bool, error) {
	f.findCalled = true
	f.winStart, f.winEnd = ws, we
	if f.findErr != nil {
		return ReconcileCandidate{}, false, f.findErr
	}
	return f.candidate, f.candFound, nil
}

func (f *fakeReconcileRepo) Reconcile(_ context.Context, _ pgx.Tx, p ReconcileParams) (bool, error) {
	f.reconciled = &p
	return !f.guardNoop, nil
}

func (f *fakeReconcileRepo) Relink(_ context.Context, _ pgx.Tx, p RelinkParams) (bool, error) {
	f.relinked = &p
	return !f.guardNoop, nil
}

// entry is a workable 07:00–15:00 shift dated 2026-06-17 (Asia/Jakarta).
func reconcileEntry() ReconcileEntry {
	return ReconcileEntry{
		ScheduleID:    "SWP-SCH-0001",
		EmployeeID:    "SWP-EMP-0001",
		PlacementID:   "SWP-PL-0001",
		WorkDate:      time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC),
		StartTime:     "07:00",
		EndTime:       "15:00",
		CrossMidnight: false,
	}
}

// wib is Asia/Jakarta for building check-in instants at local wall-clock.
var wib = jakartaLoc()

// machineCand builds a machine-owned UNSCHEDULED candidate clocked in at the given
// Asia/Jakarta wall-clock time on 2026-06-17, with the given extra flags.
func machineCand(hh, mm int, extraFlags ...string) ReconcileCandidate {
	ci := time.Date(2026, 6, 17, hh, mm, 0, 0, wib)
	flags := append([]string{string(att.FlagUnscheduled)}, extraFlags...)
	return ReconcileCandidate{
		ID:                 "SWP-ATT-0001",
		CheckInAt:          ci,
		Flags:              flags,
		Status:             string(att.StatusPresent),
		VerificationStatus: string(att.VerificationPending),
	}
}

func TestReconcile_OnTime_AutoApproved(t *testing.T) {
	// AR-6/7/8: 07:03 (within 15-min grace) → PRESENT, AUTO_APPROVED, UNSCHEDULED
	// stripped, is_payable promoted.
	repo := &fakeReconcileRepo{candidate: machineCand(7, 3), candFound: true}
	svc := NewReconcileService(repo)

	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err != nil {
		t.Fatalf("ReconcileForEntry: %v", err)
	}
	if repo.reconciled == nil {
		t.Fatal("no reconcile UPDATE happened")
	}
	got := repo.reconciled
	if got.Status != string(att.StatusPresent) {
		t.Errorf("status = %q, want PRESENT", got.Status)
	}
	if got.VerificationStatus != string(att.VerificationAutoApproved) {
		t.Errorf("verification = %q, want AUTO_APPROVED", got.VerificationStatus)
	}
	if got.IsLate || got.LateMinutes != 0 {
		t.Errorf("late = %v/%d, want false/0", got.IsLate, got.LateMinutes)
	}
	if hasFlag(got.Flags, string(att.FlagUnscheduled)) {
		t.Errorf("flags %v still contain UNSCHEDULED", got.Flags)
	}
	if got.IsPayable == nil || !*got.IsPayable {
		t.Errorf("is_payable = %v, want promoted true (AR-8)", got.IsPayable)
	}
	if got.ScheduleID != "SWP-SCH-0001" {
		t.Errorf("schedule_id = %q, want SWP-SCH-0001", got.ScheduleID)
	}
}

func TestReconcile_Late_PendingWithLateMinutes(t *testing.T) {
	// AR-6: 07:40 (40 min, > 15 grace) → LATE, late_minutes=40, [LATE], PENDING.
	repo := &fakeReconcileRepo{candidate: machineCand(7, 40), candFound: true}
	svc := NewReconcileService(repo)

	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err != nil {
		t.Fatalf("ReconcileForEntry: %v", err)
	}
	got := repo.reconciled
	if got == nil {
		t.Fatal("no reconcile UPDATE happened")
	}
	if got.Status != string(att.StatusLate) {
		t.Errorf("status = %q, want LATE", got.Status)
	}
	if !got.IsLate || got.LateMinutes != 40 {
		t.Errorf("late = %v/%d, want true/40", got.IsLate, got.LateMinutes)
	}
	if !hasFlag(got.Flags, string(att.FlagLate)) {
		t.Errorf("flags %v missing LATE", got.Flags)
	}
	if got.VerificationStatus != string(att.VerificationPending) {
		t.Errorf("verification = %q, want PENDING", got.VerificationStatus)
	}
}

func TestReconcile_OutsideGeofence_Survives(t *testing.T) {
	// AR-7: on-time but OUTSIDE_GEOFENCE → UNSCHEDULED removed, OUTSIDE_GEOFENCE kept,
	// stays PENDING.
	repo := &fakeReconcileRepo{candidate: machineCand(7, 3, string(att.FlagOutsideGeofence)), candFound: true}
	svc := NewReconcileService(repo)

	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err != nil {
		t.Fatalf("ReconcileForEntry: %v", err)
	}
	got := repo.reconciled
	if got == nil {
		t.Fatal("no reconcile UPDATE happened")
	}
	if hasFlag(got.Flags, string(att.FlagUnscheduled)) {
		t.Errorf("UNSCHEDULED not removed: %v", got.Flags)
	}
	if !hasFlag(got.Flags, string(att.FlagOutsideGeofence)) {
		t.Errorf("OUTSIDE_GEOFENCE stripped, must survive: %v", got.Flags)
	}
	if got.VerificationStatus != string(att.VerificationPending) {
		t.Errorf("verification = %q, want PENDING (flag remains)", got.VerificationStatus)
	}
}

func TestReconcile_IncompleteSurvives(t *testing.T) {
	// AR-6: an AUTO_CLOSED INCOMPLETE record keeps INCOMPLETE status; AUTO_CLOSED flag
	// preserved; stays PENDING.
	cand := machineCand(7, 3, string(att.FlagAutoClosed))
	cand.Status = string(att.StatusIncomplete)
	repo := &fakeReconcileRepo{candidate: cand, candFound: true}
	svc := NewReconcileService(repo)

	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err != nil {
		t.Fatalf("ReconcileForEntry: %v", err)
	}
	got := repo.reconciled
	if got == nil || got.Status != string(att.StatusIncomplete) {
		t.Fatalf("status = %v, want INCOMPLETE preserved", got)
	}
	if !hasFlag(got.Flags, string(att.FlagAutoClosed)) {
		t.Errorf("AUTO_CLOSED stripped, must survive: %v", got.Flags)
	}
	if got.VerificationStatus != string(att.VerificationPending) {
		t.Errorf("verification = %q, want PENDING", got.VerificationStatus)
	}
}

func TestReconcile_OutOfWindow_NoMatch(t *testing.T) {
	// AR-3: check-in at 03:00 for a 14:00–22:00 shift is outside [12:00, 02:00+1d].
	// The finder (window SQL) returns nothing; service no-ops. We assert the window
	// bounds the service passed exclude 03:00.
	repo := &fakeReconcileRepo{candFound: false}
	svc := NewReconcileService(repo)
	e := reconcileEntry()
	e.StartTime, e.EndTime = "14:00", "22:00"

	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, e); err != nil {
		t.Fatalf("ReconcileForEntry: %v", err)
	}
	if repo.reconciled != nil || repo.relinked != nil {
		t.Error("a write happened, want none (no candidate)")
	}
	// Window = [14:00-2h, 22:00+4h] = [12:00, 02:00 next day] in Asia/Jakarta.
	wantStart := time.Date(2026, 6, 17, 12, 0, 0, 0, wib)
	wantEnd := time.Date(2026, 6, 18, 2, 0, 0, 0, wib)
	if !repo.winStart.Equal(wantStart) {
		t.Errorf("window start = %v, want %v", repo.winStart, wantStart)
	}
	if !repo.winEnd.Equal(wantEnd) {
		t.Errorf("window end = %v, want %v", repo.winEnd, wantEnd)
	}
	// 03:00 is before 12:00 → genuinely out of window.
	threeAM := time.Date(2026, 6, 17, 3, 0, 0, 0, wib)
	if !threeAM.Before(repo.winStart) {
		t.Errorf("03:00 should be before window start %v", repo.winStart)
	}
}

func TestReconcile_CrossMidnightWindow(t *testing.T) {
	// AR-3 / C-AR-4: a 23:00→07:00 cross-midnight shift dated 2026-06-17 has
	// window [21:00 (17th), 11:00 (18th)] — a 00:30 (18th) clock-in is inside.
	repo := &fakeReconcileRepo{candFound: false}
	svc := NewReconcileService(repo)
	e := reconcileEntry()
	e.StartTime, e.EndTime, e.CrossMidnight = "23:00", "07:00", true

	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, e); err != nil {
		t.Fatalf("ReconcileForEntry: %v", err)
	}
	wantStart := time.Date(2026, 6, 17, 21, 0, 0, 0, wib) // 23:00 - 2h
	wantEnd := time.Date(2026, 6, 18, 11, 0, 0, 0, wib)   // (07:00 +1d) + 4h
	if !repo.winStart.Equal(wantStart) {
		t.Errorf("window start = %v, want %v", repo.winStart, wantStart)
	}
	if !repo.winEnd.Equal(wantEnd) {
		t.Errorf("window end = %v, want %v", repo.winEnd, wantEnd)
	}
	clockIn := time.Date(2026, 6, 18, 0, 30, 0, 0, wib)
	if clockIn.Before(repo.winStart) || clockIn.After(repo.winEnd) {
		t.Errorf("00:30 (18th) should be inside [%v, %v]", repo.winStart, repo.winEnd)
	}
}

func TestReconcile_HumanDecided_LineageOnly(t *testing.T) {
	// AR-9: a record already VERIFIED by a human is NOT re-derived — only schedule_id
	// lineage is attached, audited RELINKED. No Reconcile UPDATE happens.
	verifier := "SWP-EMP-9000"
	cand := machineCand(7, 40) // would be LATE if re-derived
	cand.VerificationStatus = string(att.VerificationVerified)
	cand.VerifiedBy = &verifier
	repo := &fakeReconcileRepo{candidate: cand, candFound: true}
	svc := NewReconcileService(repo)

	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err != nil {
		t.Fatalf("ReconcileForEntry: %v", err)
	}
	if repo.reconciled != nil {
		t.Error("a machine-owned reconcile happened on a human-decided record")
	}
	if repo.relinked == nil {
		t.Fatal("no lineage relink happened")
	}
	if repo.relinked.ScheduleID != "SWP-SCH-0001" {
		t.Errorf("relink schedule_id = %q, want SWP-SCH-0001", repo.relinked.ScheduleID)
	}
}

func TestReconcile_GuardNoop_NoError(t *testing.T) {
	// AR-4: a concurrent link won the race — the count-guarded UPDATE returns
	// found=false; the service treats it as a harmless no-op (no error).
	repo := &fakeReconcileRepo{candidate: machineCand(7, 3), candFound: true, guardNoop: true}
	svc := NewReconcileService(repo)
	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err != nil {
		t.Fatalf("ReconcileForEntry: %v", err)
	}
}

func TestReconcile_FindError_FailsClosed(t *testing.T) {
	// AR-10: an unexpected finder error propagates (rolls back the schedule create).
	repo := &fakeReconcileRepo{findErr: errors.New("boom")}
	svc := NewReconcileService(repo)
	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err == nil {
		t.Fatal("want error to propagate (fail-closed), got nil")
	}
}

func TestReconcile_ExplicitPayablePreserved(t *testing.T) {
	// AR-8: an explicit is_payable (false) is never overridden — the narg stays nil so
	// the query's COALESCE keeps the existing value.
	cand := machineCand(7, 3)
	no := false
	cand.IsPayable = &no
	repo := &fakeReconcileRepo{candidate: cand, candFound: true}
	svc := NewReconcileService(repo)
	if err := svc.ReconcileForEntry(context.Background(), sweepFakeTx{}, reconcileEntry()); err != nil {
		t.Fatalf("ReconcileForEntry: %v", err)
	}
	if repo.reconciled.IsPayable != nil {
		t.Errorf("is_payable narg = %v, want nil (explicit value preserved)", repo.reconciled.IsPayable)
	}
}
