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

// QuotaFilter is the decoded GET /leave-quotas query (cursor-paged) — DEPRECATED
// 2026-06-08; the live path is GrantFilter / leave-grants.
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

// BalanceListFilter is the decoded GET /leave-balances query (cursor-paged, one row
// per employee). Q mirrors ListEmployees: ILIKE over full_name/nik/nip. Cursor is
// keyset on (full_name, employee_id).
type BalanceListFilter struct {
	Q              *string
	Limit          int
	CursorFullName *string
	CursorID       *string
}

// GrantFilter is the decoded GET /leave-grants query (cursor-paged, FIFO-ordered).
type GrantFilter struct {
	EmployeeID     *string
	Earmark        *string // "__null" sentinel = unearmarked only (openapi `earmark=__null`)
	Source         *string
	CompanyID      *string
	IncludeExpired bool
	Limit          int
	CursorExpires  *time.Time
	CursorID       *string
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
	IsAnnual bool   // the quota-tracked gate (the real leave_types.is_annual column)
	Earmark  string // purpose code for earmarked allocation (LQ-10); "" = flat pool

	// F6.2 create-time validation gates (openapi LeaveType). IsDocumentRequired maps
	// to the leave_types.requires_document column (MISSING_REQUIRED_DOCUMENT). There is
	// no allows_backdated column yet, so AllowsBackdated is currently always false (any
	// start_date < today on any type → BACKDATED_LEAVE).
	// TODO: attachment upload + edit-draft + document-required leave types; add a real
	// leave_types.allows_backdated column to source AllowsBackdated.
	IsDocumentRequired bool
	AllowsBackdated    bool
}

// --- leave-request repository port ---

// LeaveRepository is the data dependency for the leave request + calendar services.
type LeaveRepository interface {
	ListLeaveRequests(ctx context.Context, f RequestFilter) ([]dom.LeaveRequest, error)
	GetLeaveRequest(ctx context.Context, id string) (dom.LeaveRequest, error)
	GetLeaveRequestForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.LeaveRequest, error)

	// CreateLeaveRequest inserts a DRAFT request (F6.2 agent file-a-request). The
	// caller (Create) computes duration + routing + the validation gates first.
	CreateLeaveRequest(ctx context.Context, tx pgx.Tx, p CreateLeaveRequestParams) (dom.LeaveRequest, error)
	// CheckOverlappingLeave reports whether the employee already holds a live
	// (non-REJECTED/non-CANCELLED) leave overlapping [start,end] (LR-5).
	CheckOverlappingLeave(ctx context.Context, employeeID string, start, end time.Time) (bool, error)

	UpdateLeaveRequestStatus(ctx context.Context, tx pgx.Tx, p UpdateStatusParams) (dom.LeaveRequest, error)
	UpdateLeaveRequestDates(ctx context.Context, tx pgx.Tx, id string, start, end time.Time, durationDays int) (dom.LeaveRequest, error)

	InsertLeaveApproval(ctx context.Context, tx pgx.Tx, p ApprovalRow) (dom.LeaveApproval, error)
	ListLeaveApprovalsForRequest(ctx context.Context, id string) ([]dom.LeaveApproval, error)

	GetLeaveType(ctx context.Context, id string) (LeaveTypeInfo, error)

	// SetBalanceSnapshot persists the FIFO reservation snapshot (openapi BalanceCheck)
	// at reserve/commit; clearing passes a zero BalanceSnapshot.
	SetBalanceSnapshot(ctx context.Context, tx pgx.Tx, p BalanceSnapshotParams) error

	ListCalendarEntries(ctx context.Context, f CalendarFilter, statusIn []string, from, to time.Time) ([]dom.LeaveCalendarEntry, error)
}

// BalanceSnapshotParams writes the openapi BalanceCheck snapshot columns. allocation
// is the marshalled jsonb FIFO split (nil clears).
type BalanceSnapshotParams struct {
	ID               string
	RequestedDays    *int
	RemainingAtCheck *int
	RequiresOverride *bool
	Earmark          *string
	Allocation       []byte
}

// UpdateStatusParams carries the state transition + routing/balance snapshot write.
// BalanceQuotaID is retained as a nullable column (always nil under the grant-lot
// ledger) for migration/rollback compatibility; the live snapshot is requested/
// remaining/requires_override + the leave_consumptions rows (the allocation).
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

// CreateLeaveRequestParams carries one DRAFT leave_requests insert (F6.2). The id is
// allocated by the column DEFAULT; duration_days / backdated / routing are computed by
// the service before the insert. Nullable columns are pointers.
type CreateLeaveRequestParams struct {
	EmployeeID       string
	PlacementID      *string
	CompanyID        *string
	ServiceLineID    *string
	LeaveTypeID      string
	StartDate        time.Time
	EndDate          time.Time
	DurationDays     int
	Reason           *string
	Notes            *string
	Status           dom.LeaveStatus
	DelegateID       *string
	DocumentFileID   *string
	Backdated        bool
	NoLeader         bool
	AssignedLeaderID *string
	CreatedBy        *string
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

// --- grant-lot ledger repository port (F6.1, the live balance path) ---

// CreateGrantParams carries one HR grant-lot insert (POST /leave-grants).
type CreateGrantParams struct {
	EmployeeID    string
	Amount        int
	Source        dom.LeaveGrantSource
	Earmark       *string
	Remark        *string
	EffectiveFrom time.Time
	ExpiresAt     time.Time
	CreatedBy     *string
}

// PatchGrantParams carries one HR lot adjustment (PATCH /leave-grants/{id}). nil
// scalar fields leave the column unchanged; SetEarmark distinguishes "set earmark to
// null" from "leave earmark unchanged".
type PatchGrantParams struct {
	ID         string
	Amount     *int
	ExpiresAt  *time.Time
	SetEarmark bool
	Earmark    *string
	Remark     string
}

// EarmarkBalanceGroup is one (earmark, remaining, pending, next_expiry) aggregate from
// the balance query. Earmark==nil ⇒ the flat pool group.
type EarmarkBalanceGroup struct {
	Earmark    *string
	Remaining  int
	Pending    int
	NextExpiry *time.Time
}

// ExpiredLot is one expired lot still holding dangling pending_days (expiry sweep).
type ExpiredLot struct {
	ID          string
	EmployeeID  string
	ExpiresAt   time.Time
	PendingDays int
}

// GrantRepository is the data dependency for the grant/balance/allocation service.
type GrantRepository interface {
	CreateLeaveGrant(ctx context.Context, tx pgx.Tx, p CreateGrantParams) (dom.LeaveGrant, error)
	GetLeaveGrant(ctx context.Context, id string) (dom.LeaveGrant, error)
	GetLeaveGrantForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.LeaveGrant, error)
	ListLeaveGrants(ctx context.Context, f GrantFilter, now time.Time) ([]dom.LeaveGrant, error)
	ListLeaveBalances(ctx context.Context, f BalanceListFilter, now time.Time) ([]dom.EmployeeLeaveBalance, error)
	PatchLeaveGrant(ctx context.Context, tx pgx.Tx, p PatchGrantParams) (dom.LeaveGrant, error)

	ListConsumptionsForGrant(ctx context.Context, grantID string) ([]dom.LeaveConsumption, error)
	ListConsumptionsForRequest(ctx context.Context, requestID string) ([]dom.LeaveConsumption, error)

	// Allocation lifecycle (all in-tx, FOR UPDATE on the lots).
	GetActiveLotsForAllocation(ctx context.Context, tx pgx.Tx, employeeID, earmarkMatch string, now time.Time) ([]dom.LeaveGrant, error)
	ReservePending(ctx context.Context, tx pgx.Tx, grantID string, days int) error
	CommitReservation(ctx context.Context, tx pgx.Tx, grantID string, days int) error
	ReleasePending(ctx context.Context, tx pgx.Tx, grantID string, days int) error
	ReverseConsumption(ctx context.Context, tx pgx.Tx, grantID string, days int) error
	ApplyConsumption(ctx context.Context, tx pgx.Tx, requestID, grantID string, days int) (dom.LeaveConsumption, error)
	DeleteConsumptionsForRequest(ctx context.Context, tx pgx.Tx, requestID string) error

	// Balance read model.
	SumActiveBalanceByEarmark(ctx context.Context, employeeID string, now time.Time) ([]EarmarkBalanceGroup, error)

	// Expiry sweep.
	FindExpiredLotsWithPending(ctx context.Context, today time.Time, limit int) ([]ExpiredLot, error)
	ZeroLotPending(ctx context.Context, tx pgx.Tx, grantID string) error
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

	// CountLeaveDuration is the server-authoritative F6.2 duration: the number of days
	// in [start,end] the agent would otherwise be rostered for a shift (E4 schedule
	// entries) MINUS public holidays (E7). Reuses the scheduling repo's schedule_entries
	// + holidays access, so the leave service never re-implements a naive day-count.
	CountLeaveDuration(ctx context.Context, employeeID string, start, end time.Time) (int, error)
}
