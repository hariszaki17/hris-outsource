// Package leave holds the dependency-free domain types for the E6 slice
// (F6.1/F6.2/F6.3 / SWP-LR-* / SWP-LQ-*). These structs are shared between the
// leave service and repository and map 1:1 onto the openapi LeaveRequest /
// LeaveQuota / LeaveCalendarEntry shapes (08-02 maps sqlc rows → these → DTOs).
//
// Convention (mirrors internal/domain/attendance): nullable columns are pointers;
// denormalized read-time fields (employee_name, company_name, …) are pointers too.
// remaining = total - used - pending is a DERIVED method (LeaveQuota.Remaining),
// never a stored column. The two-level approval state machine
// (PENDING_L1 → PENDING_HR → APPROVED; reject → REJECTED; withdraw → CANCELLED)
// is enforced in the 08-02 service; these types only carry state.
package leave

import "time"

// LeaveStatus is the persisted request lifecycle state. Values are pinned to
// openapi schemas.LeaveStatus (AUTHORITATIVE) — byte-for-byte.
type LeaveStatus string

const (
	LeaveStatusDraft     LeaveStatus = "DRAFT"
	LeaveStatusPendingL1 LeaveStatus = "PENDING_L1"
	LeaveStatusPendingHR LeaveStatus = "PENDING_HR"
	LeaveStatusApproved  LeaveStatus = "APPROVED"
	LeaveStatusRejected  LeaveStatus = "REJECTED"
	LeaveStatusCancelled LeaveStatus = "CANCELLED"
)

// LeaveStage is the approval stage at which a decision was recorded (openapi
// schemas.LeaveStage).
type LeaveStage string

const (
	StageL1 LeaveStage = "L1"
	StageHR LeaveStage = "HR"
)

// LeaveDecision is the recorded decision on a stage (openapi schemas.LeaveDecision).
type LeaveDecision string

const (
	DecisionApproved         LeaveDecision = "APPROVED"
	DecisionRejected         LeaveDecision = "REJECTED"
	DecisionOverrideApproved LeaveDecision = "OVERRIDE_APPROVED"
)

// TimelineStatus is the per-timeline-entry status the FE renders (openapi
// LeaveTimelineEntry.status).
type TimelineStatus string

const (
	TimelineStatusPending          TimelineStatus = "PENDING"
	TimelineStatusApproved         TimelineStatus = "APPROVED"
	TimelineStatusRejected         TimelineStatus = "REJECTED"
	TimelineStatusOverrideApproved TimelineStatus = "OVERRIDE_APPROVED"
)

// LeaveRouting is the openapi LeaveRequest.routing object (LA-2 no-leader routing).
type LeaveRouting struct {
	NoLeader         bool    `json:"no_leader"`
	AssignedLeaderID *string `json:"assigned_leader_id,omitempty"`
	AssignedLeader   *string `json:"assigned_leader_name,omitempty"`
}

// BalanceCheck is the openapi LeaveRequest.balance_check snapshot taken at
// submit-reserve or final-approve-commit. RequestedDays / RemainingDaysAtCheck are the
// recorded-at-check values (per-type ledger; no per-lot allocation).
type BalanceCheck struct {
	RequestedDays        *int       `json:"requested_days,omitempty"`
	RemainingDaysAtCheck *int       `json:"remaining_days_at_check,omitempty"`
	CheckedAt            *time.Time `json:"checked_at,omitempty"`
	RequiresOverride     bool       `json:"requires_override"`
}

// LeaveTimelineEntry is one decision in the request's approval timeline (openapi
// LeaveTimelineEntry). Assembled from the leave_approvals rows in 08-02.
type LeaveTimelineEntry struct {
	Stage          LeaveStage     `json:"stage"`
	Status         TimelineStatus `json:"status"`
	ActorID        *string        `json:"actor_id,omitempty"`
	ActorName      *string        `json:"actor_name,omitempty"`
	ActorRole      *string        `json:"actor_role,omitempty"`
	Decision       *LeaveDecision `json:"decision,omitempty"`
	DecisionNote   *string        `json:"decision_note,omitempty"`
	RejectReason   *string        `json:"reject_reason,omitempty"`
	Override       bool           `json:"override"`
	OverrideReason *string        `json:"override_reason,omitempty"`
	OccurredAt     time.Time      `json:"occurred_at"`
}

// ScheduleImpactEntry is one cancelled/affected E4 schedule entry caused by an
// approved leave (openapi LeaveRequest.schedule_impact[]). Populated by the INV-3
// loop-closer in 08-02.
type ScheduleImpactEntry struct {
	ScheduleID      string `json:"schedule_id"`
	Date            string `json:"date"`
	PriorStatus     string `json:"prior_status"`
	NewStatus       string `json:"new_status"` // LEAVE | UNASSIGNED
	ClockInConflict bool   `json:"clock_in_conflict"`
}

// LeaveRequest is the domain entity for one leave request (openapi LeaveRequest).
// Nullable openapi fields are pointers; *Name fields are denormalized via JOINs.
// Timeline / ScheduleImpact are assembled by 08-02 from the leave_approvals +
// schedule-cancel side-effects.
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

	// routing.* + balance_check.* snapshot columns.
	Routing      LeaveRouting
	BalanceCheck BalanceCheck

	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Denormalized for display (server-authoritative; filled via JOINs).
	EmployeeName  *string
	CompanyName   *string
	LeaveTypeName *string
	LeaveTypeCode *string

	// Assembled read-time aggregates (08-02).
	Timeline       []LeaveTimelineEntry
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

// LeaveQuota is the domain entity for one (employee, leave_type, period) quota row
// (openapi LeaveQuota). remaining is the DERIVED Remaining() method, NOT a column.
type LeaveQuota struct {
	ID            string
	EmployeeID    string
	LeaveTypeID   string
	Period        int
	PeriodStart   time.Time
	PeriodEnd     time.Time
	Total         int
	Used          int
	Pending       int
	IsProrated    bool
	ProrateMonths int
	Closed        bool

	// Per-type ledger (2026-06-12, EPICS §8). PeriodKey generalizes Period across
	// cap_basis windows (year | year-month | "EMP"). EntitledDays/UsedDays/
	// PendingDays supersede Total/Used/Pending once the service is rewired (Phase 4).
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

// Remaining is the derived balance: total - used - pending. May go negative after
// an HR :approve-override (LA-8); last_override is set in that case.
func (q LeaveQuota) Remaining() int { return q.Total - q.Used - q.Pending }

// RemainingPerType is the per-type-ledger derived balance (entitled - used -
// pending). Used by the per-type model (Phase 4 onward); never negative by design
// (allocation only ever draws available, INV-6).
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

// QuotaWindowSpec opens (or upserts) a per-type quota window. Legacy Period/
// PeriodStart/PeriodEnd are supplied transitionally (NOT NULL until Phase 8).
type QuotaWindowSpec struct {
	EmployeeID   string
	LeaveTypeID  string
	PeriodKey    string
	Period       int
	PeriodStart  time.Time
	PeriodEnd    time.Time
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

// LeaveApproval is one immutable decision-trail row (the leave_approvals table /
// the FEATURE ER decision log). 08-02 maps these into LeaveRequest.timeline[].
type LeaveApproval struct {
	ID             int64
	LeaveRequestID string
	Stage          LeaveStage
	Decision       LeaveDecision
	ActorID        *string
	ActorRole      *string
	DecisionNote   *string
	RejectReason   *string
	IsOverride     bool
	OverrideReason *string
	OccurredAt     time.Time
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

// LeaveQuotaAdjustParams carries the :adjust inputs (08-02 service → repo).
type LeaveQuotaAdjustParams struct {
	QuotaID    string
	Delta      int
	Reason     string
	AdjustedBy string
}

// LeaveQuotaBulkGrantParams carries the :bulk-grant inputs (08-02 service → repo).
type LeaveQuotaBulkGrantParams struct {
	LeaveTypeID     string
	Period          int
	PeriodStart     time.Time
	PeriodEnd       time.Time
	EntitlementDays int
	EmployeeIDs     []string // ["all"] sentinel resolved in the service
	ProRate         bool
	Preview         bool
}
