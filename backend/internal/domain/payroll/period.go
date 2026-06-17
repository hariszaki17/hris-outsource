// Package payroll — F8.5 Payroll Period Close domain types (PC-1..PC-15 /
// SWP-PPD-* / SWP-PES-* / SWP-CLR-* / SWP-PAJ-*). Dependency-free structs + enums
// + invariant helpers shared by the period service and repository. Mirrors the
// per-epic domain convention (internal/domain/attendance, internal/domain/leave):
// nullable columns are pointers (nil = null on the wire).
package payroll

import "time"

// PeriodStatus is the PayrollPeriod state-machine value (PC-1). OPEN (upstreams
// mutable) -> LOCKED -> (REOPENED -> OPEN). Pinned to the F8.5 openapi.
type PeriodStatus string

const (
	PeriodStatusOpen     PeriodStatus = "OPEN"
	PeriodStatusLocked   PeriodStatus = "LOCKED"
	PeriodStatusReopened PeriodStatus = "REOPENED"
)

// ClarificationStatus is the clarification-request lifecycle (PC-13). One round:
// OPEN -> ANSWERED -> RESOLVED (or CANCELLED).
type ClarificationStatus string

const (
	ClarificationStatusOpen      ClarificationStatus = "OPEN"
	ClarificationStatusAnswered  ClarificationStatus = "ANSWERED"
	ClarificationStatusResolved  ClarificationStatus = "RESOLVED"
	ClarificationStatusCancelled ClarificationStatus = "CANCELLED"
)

// ClarificationSourceType is the kind of record a clarification is raised on
// (PC-13). Pinned to the F8.5 openapi.
type ClarificationSourceType string

const (
	ClarificationSourceAttendance ClarificationSourceType = "ATTENDANCE"
	ClarificationSourceOvertime   ClarificationSourceType = "OVERTIME"
	ClarificationSourceLeave      ClarificationSourceType = "LEAVE"
)

// AdjustmentStatus is the carry-forward ledger state (PC-9). PENDING (event
// recorded, amount unvalued) -> APPLIED (the F8.3 run valued + applied it).
type AdjustmentStatus string

const (
	AdjustmentStatusPending AdjustmentStatus = "PENDING"
	AdjustmentStatusApplied AdjustmentStatus = "APPLIED"
)

// AdjustmentSourceType is the origin of a post-lock change (PC-9).
type AdjustmentSourceType string

const (
	AdjustmentSourceAttendance AdjustmentSourceType = "ATTENDANCE"
	AdjustmentSourceOvertime   AdjustmentSourceType = "OVERTIME"
	AdjustmentSourceCorrection AdjustmentSourceType = "CORRECTION"
)

// PayrollPeriod is one month-end gate (PC-1). LockedBy/At + ReopenedBy/At + the
// force-lock fields are nil until the matching transition stamps them.
type PayrollPeriod struct {
	ID              string
	Year            int
	Month           int
	Status          PeriodStatus
	LockedBy        *string
	LockedAt        *time.Time
	ForceLocked     bool
	ForceLockReason *string
	ReopenedBy      *string
	ReopenedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsLocked reports whether the period is in the LOCKED terminal-of-the-month state
// (PC-11 F5.7 guard / PC-10 run precondition read this).
func (p PayrollPeriod) IsLocked() bool { return p.Status == PeriodStatusLocked }

// CanLock reports whether a lock transition is permitted from the current state
// (PC-7): only OPEN or REOPENED periods may be locked.
func (p PayrollPeriod) CanLock() bool {
	return p.Status == PeriodStatusOpen || p.Status == PeriodStatusReopened
}

// CanReopen reports whether a reopen transition is permitted (PC-12): only a LOCKED
// period may be reopened.
func (p PayrollPeriod) CanReopen() bool { return p.Status == PeriodStatusLocked }

// PeriodEmployeeSummary is the IMMUTABLE per-FIELD-employee snapshot materialized
// on lock (PC-8). Authoritative payroll input (PC-10).
type PeriodEmployeeSummary struct {
	ID                 string
	PeriodID           string
	EmployeeID         string
	EmployeeName       *string
	PayableDays        int
	PresentDays        int
	LateDays           int
	AbsentDays         int
	NoShiftPayableDays int
	WorkedMinutes      int
	ApprovedOTMinutes  int
	PaidLeaveDays      int
	UnpaidLeaveDays    int
	CreatedAt          time.Time
}

// EmployeeCompleteness is one per-employee completeness row across the three
// upstreams (PC-2..PC-5). FIELD employees gate the lock; INTERNAL is informational
// (PC-5). Blockers is the in-scope unresolved count (attendance + OT + leave).
type EmployeeCompleteness struct {
	EmployeeID   string
	EmployeeName *string
	EmployeeType string // FIELD | INTERNAL (PC-5)

	// Attendance completeness (PC-2).
	Expected int // E4 workable entries
	Recorded int // attendance rows in-month
	Clean    int // verification_status in (VERIFIED, AUTO_APPROVED)
	OnLeave  int // E6 approved leave days

	// Blocker breakdown (PC-3 / PC-4).
	AttendanceBlockers int // PENDING/ESCALATED + open + is_payable-NULL no-shift
	CoverageGap        int // expected - (clean + on_leave), floored at 0
	PendingOT          int // E7 OT in-period with status PENDING
	PendingLeave       int // E6 leave overlapping the period with status PENDING
}

// InScopeBlockerCount is the lock-gating blocker count for this row (PC-5/PC-7).
// FIELD employees count their attendance + coverage-gap + OT + leave blockers;
// INTERNAL staff are informational and never gate (return 0).
func (c EmployeeCompleteness) InScopeBlockerCount() int {
	if c.EmployeeType != "FIELD" {
		return 0
	}
	return c.AttendanceBlockers + c.CoverageGap + c.PendingOT + c.PendingLeave
}

// PeriodBlockerCounts is the per-tab unresolved-exception tally for the cockpit
// header (PC-7 gate / §9 GET period). Only FIELD blockers are counted (PC-5).
type PeriodBlockerCounts struct {
	Attendance int
	Overtime   int
	Leave      int
}

// Total is the sum across the three tabs — zero means a clean lock is permitted
// (PC-7).
func (c PeriodBlockerCounts) Total() int { return c.Attendance + c.Overtime + c.Leave }

// ClarificationRequest is one thin back-channel question (PC-13).
type ClarificationRequest struct {
	ID               string
	SourceType       ClarificationSourceType
	SourceID         string
	PeriodID         *string
	RaisedBy         *string
	TargetEmployeeID string
	Question         string
	Status           ClarificationStatus
	Answer           *string
	AnsweredBy       *string
	AnsweredAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// PayrollAdjustment is one carry-forward ledger entry (PC-9). Amount is nil until
// the F8.3 run values it; AppliedRunID is nil until applied.
type PayrollAdjustment struct {
	ID           string
	EmployeeID   string
	SourceType   AdjustmentSourceType
	SourceID     string
	OriginYear   int
	OriginMonth  int
	Note         *string
	Amount       *string
	Status       AdjustmentStatus
	AppliedRunID *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
