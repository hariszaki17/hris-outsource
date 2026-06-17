// Package attendance — F5.7 Attendance Auto-Reconcile (AR-1..AR-10). When a
// workable shift is created (scheduling.CreateEntry), inside the SAME tx, an
// existing machine-owned UNSCHEDULED attendance record the shift covers is adopted:
// linked to the new schedule_id, its shift window snapshotted, and its truth
// (lateness / status / flags / verification / payability) RE-DERIVED with the exact
// clock-in rules (clock_service.go) — so a reconciled record is indistinguishable
// from one captured against a shift that existed all along. A human-decided record
// (already VERIFIED/REJECTED/ESCALATED) is never re-derived; it only gets the
// schedule_id + shift snapshot for lineage (AR-9, audited RELINKED, not RECONCILED).
//
// This is the E4→E5 reconcile counterpart to schedule_propagation.go's E4→E5 shift-
// end sync: scheduling depends on the Reconciler PORT (consumer-defined in this
// package, satisfied by ReconcileService) and calls it in-tx. The derivation is a
// pure function (deriveReconcile) so it is unit-testable without a DB.
package attendance

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
)

// Reconcile audit actions distinguish the two adoption outcomes (AR-10).
const (
	// ActionReconciled — a machine-owned UNSCHEDULED record was adopted AND re-derived.
	ActionReconciled audit.Action = "ATTENDANCE_RECONCILED"
	// ActionRelinked — a human-decided record got schedule_id lineage only (AR-9).
	ActionRelinked audit.Action = "ATTENDANCE_RELINKED"
)

// reconcileWindowPre / reconcileWindowPost are the match window margins (AR-3):
// [shift_start − 2h, shift_end + 4h]. The 4h tail reuses the flexible-checkout grace
// (att.CheckoutWindowGrace).
const reconcileWindowPre = 2 * time.Hour

var reconcileWindowPost = att.CheckoutWindowGrace // 4h

// ReconcileEntry is the just-created workable schedule entry that may adopt an
// existing UNSCHEDULED attendance record. Scheduling builds this from the entry it
// inserted (AR-1 guarantees both times are set + status SCHEDULED + not day-off).
type ReconcileEntry struct {
	ScheduleID    string
	EmployeeID    string
	PlacementID   string
	WorkDate      time.Time
	StartTime     string // HH:MM (Asia/Jakarta wall-clock)
	EndTime       string // HH:MM
	CrossMidnight bool
}

// Reconciler is the port scheduling.CreateEntry depends on. ReconcileService
// satisfies it; the call runs inside the schedule-create tx (AR-10 atomicity:
// an unexpected error rolls back the schedule create).
type Reconciler interface {
	ReconcileForEntry(ctx context.Context, tx pgx.Tx, e ReconcileEntry) error
}

// ReconcileCandidate is the adoptee row FindCandidate returns (the columns reconcile
// needs to classify + re-derive). CheckInAt is always non-nil (the finder filters it).
type ReconcileCandidate struct {
	ID                 string
	CheckInAt          time.Time
	Flags              []string
	Status             string
	VerificationStatus string
	IsPayable          *bool
	VerifiedBy         *string
	RejectedBy         *string
}

// ReconcileParams is the machine-owned re-derivation UPDATE payload (AR-5/6/7/8).
type ReconcileParams struct {
	ID                 string
	ScheduleID         string
	ShiftStartAt       time.Time
	ShiftEndAt         time.Time
	Status             string
	IsLate             bool
	LateMinutes        int
	Flags              []string
	VerificationStatus string
	IsPayable          *bool // nil ⇒ leave as-is; the query COALESCEs NULL→true
}

// RelinkParams is the lineage-only UPDATE payload for a human-decided record (AR-9).
type RelinkParams struct {
	ID           string
	ScheduleID   string
	ShiftStartAt time.Time
	ShiftEndAt   time.Time
}

// ReconcileRepo is the data surface the reconcile service needs. The finder reads +
// row-locks under the tx; the two writes are count-guarded (schedule_id IS NULL) so
// a concurrent link cannot double-apply — found=false (no error) when the guard
// no-ops (AR-4).
type ReconcileRepo interface {
	// FindCandidate returns the earliest UNSCHEDULED row in [windowStart, windowEnd]
	// for (employeeID, placementID), FOR UPDATE. found=false (no error) when none.
	FindCandidate(ctx context.Context, tx pgx.Tx, employeeID, placementID string, windowStart, windowEnd time.Time) (ReconcileCandidate, bool, error)
	// Reconcile re-derives a machine-owned record. found=false when the guard no-ops.
	Reconcile(ctx context.Context, tx pgx.Tx, p ReconcileParams) (found bool, err error)
	// Relink attaches schedule_id lineage to a human-decided record. found=false
	// when the guard no-ops.
	Relink(ctx context.Context, tx pgx.Tx, p RelinkParams) (found bool, err error)
}

// LockedMonthChecker reports whether the PayrollPeriod for a (year, month) is
// LOCKED (E8 F8.5 / PC-11). Defined here (consumer side) to avoid an import cycle
// with the payroll service; satisfied by the payroll repo and injected via
// SetLockedMonthChecker. When a checker is wired, ReconcileForEntry is a NO-OP for a
// shift whose work-date month is locked — a late roster still inserts the shift, but
// F5.7 never re-derives a locked-month attendance record (PC-11). A nil checker
// preserves the pre-F8.5 behavior (always reconcile).
type LockedMonthChecker interface {
	IsMonthLocked(ctx context.Context, year, month int) (bool, error)
}

// ReconcileService implements the auto-reconcile business logic.
type ReconcileService struct {
	repo   ReconcileRepo
	grace  time.Duration
	locked LockedMonthChecker // PC-11 guard; nil = always reconcile (pre-F8.5)
}

// NewReconcileService wires the reconcile service with the 15-min clock-in grace
// (defaultGrace) — identical to clock-in so a reconciled record is indistinguishable.
func NewReconcileService(repo ReconcileRepo) *ReconcileService {
	return &ReconcileService{repo: repo, grace: defaultGrace}
}

// SetLockedMonthChecker wires the F8.5 locked-period guard (PC-11). Additive +
// nil-safe: without it, reconcile runs as before.
func (s *ReconcileService) SetLockedMonthChecker(c LockedMonthChecker) { s.locked = c }

var _ Reconciler = (*ReconcileService)(nil)

// ReconcileForEntry finds and adopts the UNSCHEDULED record the new shift covers.
// Runs inside the caller's (schedule-create) tx. No candidate ⇒ no-op (the common
// case). An unexpected DB error propagates and rolls back the schedule create
// (AR-10 fail-closed).
func (s *ReconcileService) ReconcileForEntry(ctx context.Context, tx pgx.Tx, e ReconcileEntry) error {
	// PC-11: a late roster in a LOCKED payroll month still inserts the shift (the
	// caller already did), but reconcile is INERT — it must not silently re-derive a
	// locked-month attendance record. The shift's work-date month gates this (the
	// adoptee belongs to the shift's month, C-PC-2).
	if s.locked != nil {
		locked, err := s.locked.IsMonthLocked(ctx, e.WorkDate.Year(), int(e.WorkDate.Month()))
		if err != nil {
			return apperr.Internal(err)
		}
		if locked {
			return nil
		}
	}

	shiftStart := shiftInstant(e.WorkDate, e.StartTime, false)
	shiftEnd := shiftInstant(e.WorkDate, e.EndTime, e.CrossMidnight)
	windowStart := shiftStart.Add(-reconcileWindowPre)
	windowEnd := shiftEnd.Add(reconcileWindowPost)

	cand, found, err := s.repo.FindCandidate(ctx, tx, e.EmployeeID, e.PlacementID, windowStart, windowEnd)
	if err != nil {
		return apperr.Internal(err)
	}
	if !found {
		// No UNSCHEDULED record in the window — nothing to adopt (AR-3 no fallback).
		return nil
	}

	if isMachineOwned(cand) {
		return s.reconcileMachineOwned(ctx, tx, e, cand, shiftStart, shiftEnd)
	}
	return s.relinkHumanDecided(ctx, tx, e, cand, shiftStart, shiftEnd)
}

// reconcileMachineOwned re-derives + links a machine-owned record (AR-5/6/7/8) and
// audits it RECONCILED (AR-10).
func (s *ReconcileService) reconcileMachineOwned(
	ctx context.Context, tx pgx.Tx, e ReconcileEntry, cand ReconcileCandidate, shiftStart, shiftEnd time.Time,
) error {
	d := deriveReconcile(cand, shiftStart, s.grace)

	found, err := s.repo.Reconcile(ctx, tx, ReconcileParams{
		ID:                 cand.ID,
		ScheduleID:         e.ScheduleID,
		ShiftStartAt:       shiftStart,
		ShiftEndAt:         shiftEnd,
		Status:             d.status,
		IsLate:             d.isLate,
		LateMinutes:        d.lateMinutes,
		Flags:              d.flags,
		VerificationStatus: d.verification,
		IsPayable:          reconcilePayable(cand.IsPayable),
	})
	if err != nil {
		return apperr.Internal(err)
	}
	if !found {
		// Guard no-op: a concurrent link won the race (AR-4) — nothing to audit.
		return nil
	}
	return audit.Record(ctx, tx, audit.Entry{
		Action:     ActionReconciled,
		EntityType: "attendance",
		EntityID:   cand.ID,
		Before: map[string]any{
			"schedule_id":         nil,
			"status":              cand.Status,
			"verification_status": cand.VerificationStatus,
			"flags":               cand.Flags,
		},
		After: map[string]any{
			"schedule_id":         e.ScheduleID,
			"status":              d.status,
			"verification_status": d.verification,
			"flags":               d.flags,
			"source":              "auto_reconcile",
		},
	})
}

// relinkHumanDecided attaches schedule_id lineage to a human-decided record WITHOUT
// re-deriving it (AR-9) and audits it RELINKED (AR-10).
func (s *ReconcileService) relinkHumanDecided(
	ctx context.Context, tx pgx.Tx, e ReconcileEntry, cand ReconcileCandidate, shiftStart, shiftEnd time.Time,
) error {
	found, err := s.repo.Relink(ctx, tx, RelinkParams{
		ID:           cand.ID,
		ScheduleID:   e.ScheduleID,
		ShiftStartAt: shiftStart,
		ShiftEndAt:   shiftEnd,
	})
	if err != nil {
		return apperr.Internal(err)
	}
	if !found {
		return nil
	}
	return audit.Record(ctx, tx, audit.Entry{
		Action:     ActionRelinked,
		EntityType: "attendance",
		EntityID:   cand.ID,
		Before: map[string]any{
			"schedule_id":         nil,
			"status":              cand.Status,
			"verification_status": cand.VerificationStatus,
			"flags":               cand.Flags,
		},
		// status / verification_status / flags are the human's decision — UNCHANGED.
		After: map[string]any{
			"schedule_id": e.ScheduleID,
			"source":      "auto_reconcile_lineage",
		},
	})
}

// reconcileDerivation is the pure re-derivation result (AR-6/7).
type reconcileDerivation struct {
	status       string
	isLate       bool
	lateMinutes  int
	flags        []string
	verification string
}

// deriveReconcile recomputes one machine-owned record's truth from the now-known
// shift, using the SAME rules as clock-in (AR-6/7):
//   - lateness: mins = check_in − shift_start; mins > grace ⇒ LATE (status LATE,
//     LATE flag, late_minutes=mins); else PRESENT (is_late=false, late_minutes=0).
//     An INCOMPLETE record (auto-closed open row) keeps its status (AR-6).
//   - flags: remove UNSCHEDULED (+ add LATE when derived); OUTSIDE_GEOFENCE /
//     AUTO_CLOSED etc. are preserved (AR-7).
//   - verification: no flags remain ⇒ AUTO_APPROVED; any flag remains ⇒ PENDING.
func deriveReconcile(cand ReconcileCandidate, shiftStart time.Time, grace time.Duration) reconcileDerivation {
	// Strip UNSCHEDULED; keep every other fact (OUTSIDE_GEOFENCE, AUTO_CLOSED, …).
	flags := removeFlag(cand.Flags, string(att.FlagUnscheduled))

	incomplete := cand.Status == string(att.StatusIncomplete)

	var (
		status      = string(att.StatusPresent)
		isLate      bool
		lateMinutes int
	)
	// Mirror clock_service.go lateness: strictly-after-grace ⇒ LATE.
	if mins := int(cand.CheckInAt.Sub(shiftStart).Minutes()); mins > int(grace.Minutes()) {
		isLate = true
		lateMinutes = mins
		flags = appendUnique(flags, string(att.FlagLate))
		status = string(att.StatusLate)
	}
	// An auto-closed open record stays INCOMPLETE — its incompleteness survives the
	// re-derivation (AR-6); lateness flag/minutes are still recomputed above.
	if incomplete {
		status = string(att.StatusIncomplete)
	}

	// Verification mirrors clock-in: any remaining flag ⇒ PENDING; none ⇒ AUTO_APPROVED.
	verification := string(att.VerificationAutoApproved)
	if len(flags) > 0 {
		verification = string(att.VerificationPending)
	}

	return reconcileDerivation{
		status:       status,
		isLate:       isLate,
		lateMinutes:  lateMinutes,
		flags:        flags,
		verification: verification,
	}
}

// isMachineOwned reports whether a candidate is machine-owned (AR-2): PENDING and
// never touched by a human (no verified_by / rejected_by). Anything else is
// human-decided (VERIFIED/REJECTED/ESCALATED) → lineage-only (AR-9).
func isMachineOwned(c ReconcileCandidate) bool {
	return c.VerificationStatus == string(att.VerificationPending) &&
		c.VerifiedBy == nil && c.RejectedBy == nil
}

// reconcilePayable maps the existing is_payable to the UPDATE narg: an explicit
// true/false is never overridden (pass nil → query leaves it), a NULL is promoted to
// true (AR-8: the day is now shift-backed). The query COALESCEs NULL→true, so we pass
// true only when the existing value is NULL.
func reconcilePayable(existing *bool) *bool {
	if existing != nil {
		return nil // explicit value present — query's COALESCE keeps it.
	}
	t := true
	return &t
}

// shiftInstant computes the absolute instant for (work_date + HH:MM) in Asia/Jakarta,
// adding one day when the shift crosses midnight. Mirrors clock.sql
// GetTodayScheduleForEmployee's shift_start_at / shift_end_at and
// schedule_propagation.go shiftEndAt.
func shiftInstant(workDate time.Time, hhmm string, cross bool) time.Time {
	h, m := parseHHMM(hhmm)
	loc := jakartaLoc()
	t := time.Date(workDate.Year(), workDate.Month(), workDate.Day(), h, m, 0, 0, loc)
	if cross {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

// parseHHMM splits a zero-padded "HH:MM"; malformed degrades to 00:00 (entry times
// are service-validated upstream).
func parseHHMM(s string) (int, int) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	return h, m
}

// removeFlag returns a copy of in without v (preserving order).
func removeFlag(in []string, v string) []string {
	out := make([]string, 0, len(in))
	for _, x := range in {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}
