// Package leave — E6 leave services (F6.1/F6.2/F6.3 / LVE-01..03). The web surface
// is HR/leader APPROVAL + quota management + calendar (agent leave-request CREATE is
// mobile-only and out of scope; requests are seeded Pending). This package owns the
// two-level approval state machine (PENDING_L1 → PENDING_HR → APPROVED; reject →
// REJECTED), the soft-reservation quota balance model + QUOTA_EXCEEDED /
// BALANCE_RECHECK_FAILED guards, the bulk-grant partial-success engine, and the
// cross-epic INV-3 loop-closer (cancel overlapping schedule_entries +
// INSERT approved_leave_days in the same tx).
//
// Mirrors the Phase-7 attendance slice for the state machine / scope / audit-in-tx /
// bulk partial-success shape, and the Phase-6 scheduling slice for the schedule
// cancel + approved_leave_days write path.
package leave

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
)

// TxRunner runs a closure inside a DB transaction (db.TxManager satisfies it).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock supplies the current time (overridable in tests).
type Clock func() time.Time

// --- filters ---

// RequestFilter is the decoded GET /leave-requests query (cursor-paged).
type RequestFilter struct {
	CompanyID     *string
	EmployeeID    *string
	LeaveTypeID   *string
	ServiceLine   *string
	Status        *string
	StatusIn      []string
	StartFrom     *time.Time
	StartTo       *time.Time
	Q             *string
	Limit         int
	CursorCreated *time.Time
	CursorID      *string
}

// QuotaFilter is the decoded GET /leave-quotas query (cursor-paged).
type QuotaFilter struct {
	EmployeeID    *string
	LeaveTypeID   *string
	Period        *int
	CompanyID     *string
	ServiceLine   *string
	IncludeClosed bool
	Limit         int
	CursorCreated *time.Time
	CursorID      *string
}

// CalendarFilter is the decoded GET /leave-calendar query.
type CalendarFilter struct {
	CompanyID   *string
	ServiceLine *string
	LeaveTypeID *string
	Period      int
	Month       *int
	ShowPending bool
}

// --- leave-type port (read-through to E2 master for is_annual) ---

// LeaveTypeInfo is the subset of the E2 leave-type master E6 needs.
type LeaveTypeInfo struct {
	ID       string
	Code     string
	Name     string
	IsAnnual bool // the quota-tracked gate (the real leave_types.is_annual column)
}

// --- leave-request repository port ---

// LeaveRepository is the data dependency for the leave request + calendar services.
type LeaveRepository interface {
	ListLeaveRequests(ctx context.Context, f RequestFilter) ([]dom.LeaveRequest, error)
	GetLeaveRequest(ctx context.Context, id string) (dom.LeaveRequest, error)
	GetLeaveRequestForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.LeaveRequest, error)
	UpdateLeaveRequestStatus(ctx context.Context, tx pgx.Tx, p UpdateStatusParams) (dom.LeaveRequest, error)

	InsertLeaveApproval(ctx context.Context, tx pgx.Tx, p ApprovalRow) (dom.LeaveApproval, error)
	ListLeaveApprovalsForRequest(ctx context.Context, id string) ([]dom.LeaveApproval, error)

	GetLeaveType(ctx context.Context, id string) (LeaveTypeInfo, error)

	ListCalendarEntries(ctx context.Context, f CalendarFilter, statusIn []string, from, to time.Time) ([]dom.LeaveCalendarEntry, error)
}

// UpdateStatusParams carries the state transition + routing/balance snapshot write.
type UpdateStatusParams struct {
	ID                      string
	Status                  dom.LeaveStatus
	NoLeader                bool
	AssignedLeaderID        *string
	ClockInConflict         bool
	BalanceQuotaID          *string
	BalanceRequestedDays    *int
	BalanceRemainingAtCheck *int
	BalanceRequiresOverride *bool
}

// ApprovalRow carries one leave_approvals decision-trail insert.
type ApprovalRow struct {
	LeaveRequestID string
	Stage          dom.LeaveStage
	Decision       dom.LeaveDecision
	ActorID        *string
	ActorRole      *string
	DecisionNote   *string
	RejectReason   *string
	IsOverride     bool
	OverrideReason *string
}

// --- quota repository port ---

// QuotaRepository is the data dependency for the quota service.
type QuotaRepository interface {
	ListLeaveQuotas(ctx context.Context, f QuotaFilter) ([]dom.LeaveQuota, error)
	GetLeaveQuota(ctx context.Context, id string) (dom.LeaveQuota, error)
	GetLeaveQuotaForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.LeaveQuota, error)
	FindQuotaForEmployeeTypePeriod(ctx context.Context, employeeID, leaveTypeID string, period int) (dom.LeaveQuota, error)

	UpsertLeaveQuota(ctx context.Context, tx pgx.Tx, p UpsertQuotaParams) (dom.LeaveQuota, error)
	AdjustLeaveQuotaTotal(ctx context.Context, tx pgx.Tx, id string, delta int, adj dom.LeaveQuotaAdjustment) (dom.LeaveQuota, error)
	DeductLeaveQuota(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error)
	RestoreLeaveQuota(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error)
	SetLeaveQuotaOverride(ctx context.Context, tx pgx.Tx, id string, ov dom.LeaveQuotaOverride) (dom.LeaveQuota, error)

	CountPendingLeaveDaysForQuota(ctx context.Context, employeeID, leaveTypeID string, periodStart, periodEnd time.Time) (int, error)
	ListActivePlacedEmployeesForGrant(ctx context.Context, periodStart, periodEnd time.Time) ([]GrantCandidate, error)
}

// UpsertQuotaParams carries one bulk-grant total upsert (does NOT touch used/pending).
type UpsertQuotaParams struct {
	EmployeeID    string
	LeaveTypeID   string
	Period        int
	PeriodStart   time.Time
	PeriodEnd     time.Time
	Total         int
	IsProrated    bool
	ProrateMonths int
}

// GrantCandidate is one employee in the bulk-grant set (+ join date for pro-rate).
type GrantCandidate struct {
	EmployeeID     string
	EmployeeName   *string
	PlacementStart time.Time
}

// --- scheduling INV-3 port (satisfied by the existing scheduling repo) ---

// ScheduleImpact is one cancelled E4 schedule entry returned by the loop-closer
// (carries the DB status 'CANCELLED_BY_LEAVE'; the service maps it to the DTO
// new_status='LEAVE').
type ScheduleImpact struct {
	ScheduleID string
	Date       time.Time
	NewStatus  string // DB value: 'CANCELLED_BY_LEAVE'
}

// SchedulePort is the INV-3 write surface the leave service calls inside its
// approval tx. Implemented by the scheduling repo (avoids an import cycle — the
// port lives here, in service/leave).
type SchedulePort interface {
	CancelScheduleEntriesForLeave(ctx context.Context, tx pgx.Tx, employeeID string, start, end time.Time) ([]ScheduleImpact, error)
	InsertApprovedLeaveDay(ctx context.Context, tx pgx.Tx, employeeID string, date time.Time, leaveRequestID, leaveType string) error
}
