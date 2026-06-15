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
	schedulingsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
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
	Status        *string
	StatusIn      []string
	StartFrom     *time.Time
	StartTo       *time.Time
	Q             *string
	Limit         int
	CursorCreated *time.Time
	CursorID      *string
}

// CalendarFilter is the decoded GET /leave-calendar query.
type CalendarFilter struct {
	CompanyID   *string
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

	// Paid is leave_types.paid: an UNPAID type (cuti di luar tanggungan perusahaan)
	// makes every day of the request non-payable for payroll, regardless of shift.
	Paid bool
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

	// SetApprovalInstanceID links the freshly-created E11 ApprovalInstance to the
	// request (called inside the submit tx after engine.CreateInstance).
	SetApprovalInstanceID(ctx context.Context, tx pgx.Tx, id, instanceID string) error

	GetLeaveType(ctx context.Context, id string) (LeaveTypeInfo, error)

	// SetBalanceSnapshot persists the FIFO reservation snapshot (openapi BalanceCheck)
	// at reserve/commit; clearing passes a zero BalanceSnapshot.
	SetBalanceSnapshot(ctx context.Context, tx pgx.Tx, p BalanceSnapshotParams) error

	ListCalendarEntries(ctx context.Context, f CalendarFilter, statusIn []string, from, to time.Time) ([]dom.LeaveCalendarEntry, error)
}

// BalanceSnapshotParams writes the openapi BalanceCheck snapshot columns (per-type
// ledger: requested/remaining/requires_override only).
type BalanceSnapshotParams struct {
	ID               string
	RequestedDays    *int
	RemainingAtCheck *int
	RequiresOverride *bool
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

// --- quota repository port ---

// QuotaRepository is the data dependency for the per-type quota balance read (F6.5).
// The window-mutating side (reserve/commit/...) is the QuotaMeterStore interface,
// implemented by the same concrete QuotaRepo.
type QuotaRepository interface {
	// Per-type ledger (2026-06-12): every active leave type + the employee's
	// current-window quota (F6.5 balance). curYear="2026", curMonth="2026-06".
	ListEmployeeTypeBalances(ctx context.Context, employeeID, curYear, curMonth string) ([]dom.TypeBalance, error)
}

// --- entitlement repository port (HR-assigned per-employee leave) ---

// EntitlementWrite carries one assign/update (POST/PATCH) of an employee leave
// entitlement. EntitledDays is nil for event/uncapped types (toggle-on, no fixed
// quota). Note is the create value; NotePtr is the PATCH value (nil = leave as-is).
type EntitlementWrite struct {
	EmployeeID   string
	LeaveTypeID  string
	EntitledDays *int
	Note         string
	NotePtr      *string
	AssignedBy   *string
}

// EntitlementRepository is the data dependency for the HR leave-entitlement CRUD
// (PRD leave-entitlement-assignment). Mutations take a tx so the audit row commits
// atomically with the change (CONVENTIONS §16.1).
type EntitlementRepository interface {
	ListEntitlements(ctx context.Context, employeeID string) ([]dom.LeaveEntitlement, error)
	UpsertEntitlement(ctx context.Context, tx pgx.Tx, p EntitlementWrite) (dom.LeaveEntitlement, error)
	UpdateEntitlement(ctx context.Context, tx pgx.Tx, p EntitlementWrite) (dom.LeaveEntitlement, error)
	DeactivateEntitlement(ctx context.Context, tx pgx.Tx, employeeID, leaveTypeID string, assignedBy *string) (dom.LeaveEntitlement, error)
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
	// InsertApprovedLeaveDay upserts one approved-leave day. hadShift = the day cancelled a
	// live scheduled shift; isPayable = resolved payability (nil = pending an SL/HR flag for
	// a no-shift day). Migr. 00064.
	InsertApprovedLeaveDay(ctx context.Context, tx pgx.Tx, employeeID string, date time.Time, leaveRequestID, leaveType string, hadShift bool, isPayable *bool) error
	// ListApprovedLeaveDays returns the per-day payability breakdown for a request (detail UI).
	ListApprovedLeaveDays(ctx context.Context, leaveRequestID string) ([]dom.LeaveDayPayability, error)
	// SetLeaveDayPayable flags a NO-SHIFT approved-leave day payable/non-payable. Returns
	// domain.ErrNotFound when the (request, date) is not a flaggable no-shift day.
	SetLeaveDayPayable(ctx context.Context, tx pgx.Tx, leaveRequestID string, date time.Time, payable bool) (dom.LeaveDayPayability, error)

	// CountLeaveDuration is the server-authoritative F6.2 duration: the number of days
	// in [start,end] the agent would otherwise be rostered for a shift (E4 schedule
	// entries) MINUS public holidays (E7). Reuses the scheduling repo's schedule_entries
	// + holidays access, so the leave service never re-implements a naive day-count.
	CountLeaveDuration(ctx context.Context, employeeID string, start, end time.Time) (int, error)

	// FindActivePlacementForAgentDate resolves the agent's ACTIVE/EXPIRING placement
	// covering the given date. Its company denormalizes onto the leave_request and
	// selects the E11 approval template (CreateInstance requires a non-empty company);
	// the placement_id is denormalized too. Returns domain.ErrNotFound when no active
	// placement covers the date (INV-2 OUTSIDE_PLACEMENT_PERIOD).
	FindActivePlacementForAgentDate(ctx context.Context, employeeID string, date time.Time) (schedulingsvc.PlacementCover, error)
}
