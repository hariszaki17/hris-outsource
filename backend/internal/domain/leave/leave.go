// Package leave holds the dependency-free domain types for the E6 slice
// (F6.1/F6.2/F6.3 / SWP-LR-* / SWP-LQ-*). These structs are shared between the
// leave service and repository and map 1:1 onto the openapi LeaveRequest /
// LeaveQuota / LeaveCalendarEntry shapes (08-02 maps sqlc rows → these → DTOs).
//
// Convention (mirrors internal/domain/attendance): nullable columns are pointers;
// denormalized read-time fields (employee_name, company_name, …) are pointers too.
// remaining = total - used - pending is a DERIVED method (LeaveQuota.Remaining),
// never a stored column.
//
// Approval routing + the chain-progress timeline are OWNED BY E11 (the configurable
// approval engine, 2026-06-xx): the leave request only carries an
// ApprovalInstanceID linking to the E11 ApprovalInstance. The old two-level
// (PENDING_L1 → PENDING_HR) state machine + the leave_approvals decision trail were
// ripped out — clients read the chain via GET /approval-instances/{id}.
package leave

import "time"

// LeaveStatus is the persisted request lifecycle state. Values are pinned to
// openapi schemas.LeaveStatus (AUTHORITATIVE) — byte-for-byte. The intermediate
// PENDING_L1 / PENDING_HR states were collapsed into a single PENDING (E11 owns the
// chain progress).
type LeaveStatus string

const (
	LeaveStatusDraft     LeaveStatus = "DRAFT"
	LeaveStatusPending   LeaveStatus = "PENDING"
	LeaveStatusApproved  LeaveStatus = "APPROVED"
	LeaveStatusRejected  LeaveStatus = "REJECTED"
	LeaveStatusCancelled LeaveStatus = "CANCELLED"
)

// BalanceCheck is the openapi LeaveRequest.balance_check snapshot taken at
// submit-reserve or the OnApproved-hook commit re-check. RequestedDays /
// RemainingDaysAtCheck are the recorded-at-check values (per-type ledger; no
// per-lot allocation).
type BalanceCheck struct {
	RequestedDays        *int       `json:"requested_days,omitempty"`
	RemainingDaysAtCheck *int       `json:"remaining_days_at_check,omitempty"`
	CheckedAt            *time.Time `json:"checked_at,omitempty"`
	RequiresOverride     bool       `json:"requires_override"`
}

// ScheduleImpactEntry is one cancelled/affected E4 schedule entry caused by an
// approved leave (openapi LeaveRequest.schedule_impact[]). Populated by the INV-3
// loop-closer in the OnApproved hook.
type ScheduleImpactEntry struct {
	ScheduleID      string `json:"schedule_id"`
	Date            string `json:"date"`
	PriorStatus     string `json:"prior_status"`
	NewStatus       string `json:"new_status"` // LEAVE | UNASSIGNED
	ClockInConflict bool   `json:"clock_in_conflict"`
}

// LeaveRequest is the domain entity for one leave request (openapi LeaveRequest).
// Nullable openapi fields are pointers; *Name fields are denormalized via JOINs.
//
// NoLeader / AssignedLeaderID remain as persisted columns (the DB still carries
// them for migration/rollback compatibility), but the routing object is no longer
// exposed at the DTO — E11 owns routing. ApprovalInstanceID links to the E11
// ApprovalInstance tracking this request's approval chain.
type LeaveRequest struct {
	ID          string
	EmployeeID  string
	PlacementID *string
	CompanyID   *string
	LeaveTypeID string

	StartDate    time.Time
	EndDate      time.Time
	DurationDays int

	Reason          *string
	Notes           *string
	Status          LeaveStatus
	DelegateID      *string
	DocumentFileID  *string
	Backdated       bool
	ClockInConflict bool

	// Persisted routing columns (not exposed at the DTO; E11 owns routing).
	NoLeader         bool
	AssignedLeaderID *string

	// ApprovalInstanceID links the E11 ApprovalInstance (null while DRAFT, set at
	// :submit). Clients call GET /approval-instances/{id} to render the chain.
	ApprovalInstanceID *string

	// balance_check.* snapshot columns.
	BalanceCheck BalanceCheck

	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Denormalized for display (server-authoritative; filled via JOINs).
	EmployeeName  *string
	CompanyName   *string
	LeaveTypeName *string
	LeaveTypeCode *string

	// Assembled read-time aggregate (set by the OnApproved hook on the approve path).
	ScheduleImpact []ScheduleImpactEntry
}

// LeaveQuotaAdjustment is the openapi LeaveQuota.last_adjustment embedded object.
type LeaveQuotaAdjustment struct {
	Delta      int       `json:"delta"`
	Reason     string    `json:"reason"`
	AdjustedBy string    `json:"adjusted_by"`
	AdjustedAt time.Time `json:"adjusted_at"`
}

// LeaveQuotaOverride is the openapi LeaveQuota.last_override embedded object.
type LeaveQuotaOverride struct {
	LeaveRequestID string    `json:"leave_request_id"`
	OverrideReason string    `json:"override_reason"`
	OverriddenBy   string    `json:"overridden_by"`
	OverriddenAt   time.Time `json:"overridden_at"`
}

// LeaveQuota is the domain entity for one per-type quota window (openapi LeaveQuota):
// one (employee, leave_type, period_key) row under the cap_basis ledger. PeriodKey is
// the window key (year | year-month | "EMP"). remaining = entitled - used - pending is
// the DERIVED RemainingPerType() method, never a column.
type LeaveQuota struct {
	ID           string
	EmployeeID   string
	LeaveTypeID  string
	PeriodKey    string
	EntitledDays int
	UsedDays     int
	PendingDays  int
	Source       QuotaSource
	Remark       string
	ExpiresAt    *time.Time
	CreatedBy    *string

	LastAdjustment *LeaveQuotaAdjustment
	LastOverride   *LeaveQuotaOverride

	CreatedAt time.Time
	UpdatedAt time.Time

	// Denormalized for display.
	EmployeeName  *string
	LeaveTypeName *string
	LeaveTypeCode *string
}

// RemainingPerType is the derived balance (entitled - used - pending). Never negative
// by design (allocation only ever draws available, INV-6).
func (q LeaveQuota) RemainingPerType() int { return q.EntitledDays - q.UsedDays - q.PendingDays }

// LeaveTypeCap is a leave type's metering mechanics (leave_types cap_* columns,
// migr. 00050) — read by the QuotaMeter to pick the window + apply gates.
type LeaveTypeCap struct {
	ID               string
	Code             string
	Name             string
	CapBasis         LeaveTypeCapBasis
	CapValue         *int // nil = uncapped / variable
	CapUnit          string // DAYS | COUNT
	Paid             bool
	Gender           string // ANY | FEMALE | MALE
	RequiresDocument bool
	NoticeDays       int
	MinServiceYears  int
	LeadDays         int
	TrailDays        int
}

// EmployeeGateInfo is the employee data the request-time gates need.
type EmployeeGateInfo struct {
	Gender *string   // MALE | FEMALE | nil
	JoinAt time.Time // tenure source for min_service_years
}

// TypeBalance is one leave type's current-window balance for an employee (F6.5 /
// mobile "Saldo per jenis"). HasWindow=false for PER_EVENT/UNCAPPED types (and for
// quota-bearing types not yet opened); Entitled defaults to the type cap then.
type TypeBalance struct {
	LeaveTypeID      string
	Code             string
	Name             string
	CapBasis         LeaveTypeCapBasis
	CapValue         *int
	CapUnit          string
	Paid             bool
	Gender           string
	RequiresDocument bool
	Color            string

	HasWindow bool
	Entitled  int
	Used      int
	Pending   int
	ExpiresAt *time.Time
}

// Remaining is the displayed remaining for quota-bearing capped types
// (entitled-used-pending). nil for UNCAPPED / nil-cap types ("sesuai ketentuan").
func (b TypeBalance) Remaining() *int {
	if !b.CapBasis.QuotaBearing() && b.CapBasis != CapBasisPerEvent {
		return nil // UNCAPPED
	}
	if b.CapBasis == CapBasisAnnualPool || b.CapValue != nil {
		entitled := b.Entitled
		if !b.HasWindow && b.CapValue != nil {
			entitled = *b.CapValue
		}
		r := entitled - b.Used - b.Pending
		return &r
	}
	return nil // quota-bearing but no day cap (e.g. hajj)
}

// QuotaWindowSpec opens (or upserts) a per-type quota window keyed by PeriodKey.
type QuotaWindowSpec struct {
	EmployeeID   string
	LeaveTypeID  string
	PeriodKey    string
	EntitledDays int
	Source       QuotaSource
	Remark       string
	ExpiresAt    *time.Time
	CreatedBy    *string
}

// QuotaSource is how a leave_quotas row was created (leave_quotas.source).
type QuotaSource string

const (
	QuotaSourceAuto       QuotaSource = "AUTO"       // annual auto-grant / window auto-open
	QuotaSourceAdjustment QuotaSource = "ADJUSTMENT" // HR manual adjust
	QuotaSourceMigration  QuotaSource = "MIGRATION"  // E9 backfill from lumen_swp
)

// LeaveTypeCapBasis is how a leave type meters its entitlement (leave_types.cap_basis,
// migr. 00050). Drives the per-type ledger window (F6.1 LQ-13).
type LeaveTypeCapBasis string

const (
	CapBasisAnnualPool   LeaveTypeCapBasis = "ANNUAL_POOL"    // accruing yearly pool, expires year-end, no carryover
	CapBasisPerEvent     LeaveTypeCapBasis = "PER_EVENT"      // fixed days per occurrence, no standing row
	CapBasisPerMonth     LeaveTypeCapBasis = "PER_MONTH"      // resets each calendar month
	CapBasisPerYearCount LeaveTypeCapBasis = "PER_YEAR_COUNT" // max occurrences per year
	CapBasisUncapped     LeaveTypeCapBasis = "UNCAPPED"       // doc-bounded, no standing row
	CapBasisLifetimeOnce LeaveTypeCapBasis = "LIFETIME_ONCE"  // once per employment
	CapBasisServiceUnpaid LeaveTypeCapBasis = "SERVICE_UNPAID" // eligibility-gated, unpaid, once
)

// QuotaBearing reports whether a cap_basis holds a standing leave_quotas window row
// (vs PER_EVENT / UNCAPPED, which are validated at request time without a row).
func (c LeaveTypeCapBasis) QuotaBearing() bool {
	switch c {
	case CapBasisAnnualPool, CapBasisPerMonth, CapBasisPerYearCount, CapBasisLifetimeOnce, CapBasisServiceUnpaid:
		return true
	default:
		return false
	}
}

// LeaveCalendarEntry is one calendar cell-spanning entry (openapi
// LeaveCalendarEntry — what leave-calendar-screen.tsx renders).
type LeaveCalendarEntry struct {
	LeaveRequestID string
	EmployeeID     string
	EmployeeName   *string
	CompanyID      *string
	CompanyName    *string
	LeaveTypeID    string
	LeaveTypeCode  *string
	LeaveTypeName  *string
	StartDate      time.Time
	EndDate        time.Time
	Status         LeaveStatus
	DelegateID     *string
	DelegateName   *string
}
