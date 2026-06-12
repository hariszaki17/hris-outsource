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

// ServiceLine enumerates the placement service lines carried on the request.
const (
	ServiceLineFacilityServices   = "facility_services"
	ServiceLineBuildingManagement = "building_management"
	ServiceLineParking            = "parking"
)

// LeaveRouting is the openapi LeaveRequest.routing object (LA-2 no-leader routing).
type LeaveRouting struct {
	NoLeader         bool    `json:"no_leader"`
	AssignedLeaderID *string `json:"assigned_leader_id,omitempty"`
	AssignedLeader   *string `json:"assigned_leader_name,omitempty"`
}

// BalanceCheck is the openapi LeaveRequest.balance_check snapshot taken at
// submit-reserve or final-approve-commit. RequestedDays / RemainingDaysAtCheck are the
// recorded-at-check values; Allocation is the per-lot FIFO split that was reserved /
// committed (LQ-9). Earmark is the earmark the request draws against (nil = pool).
type BalanceCheck struct {
	RequestedDays        *int             `json:"requested_days,omitempty"`
	RemainingDaysAtCheck *int             `json:"remaining_days_at_check,omitempty"`
	Earmark              *string          `json:"earmark,omitempty"`
	Allocation           []AllocationLine `json:"allocation,omitempty"`
	CheckedAt            *time.Time       `json:"checked_at,omitempty"`
	RequiresOverride     bool             `json:"requires_override"`
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
	ID            string
	EmployeeID    string
	PlacementID   *string
	CompanyID     *string
	ServiceLineID *string
	LeaveTypeID   string

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

// --- grant-lot ledger (F6.1, resolved 2026-06-08) ---

// LeaveGrantSource is the provenance of a grant-lot (openapi LeaveGrantSource).
type LeaveGrantSource string

const (
	GrantSourceAnnual     LeaveGrantSource = "ANNUAL"
	GrantSourceAdjustment LeaveGrantSource = "ADJUSTMENT"
	GrantSourceMaternity  LeaveGrantSource = "MATERNITY"
	GrantSourceStatutory  LeaveGrantSource = "STATUTORY"
	GrantSourceMigration  LeaveGrantSource = "MIGRATION"
	GrantSourceBonus      LeaveGrantSource = "BONUS"
)

// ValidGrantSource reports whether s is one of the openapi LeaveGrantSource values.
func ValidGrantSource(s string) bool {
	switch LeaveGrantSource(s) {
	case GrantSourceAnnual, GrantSourceAdjustment, GrantSourceMaternity,
		GrantSourceStatutory, GrantSourceMigration, GrantSourceBonus:
		return true
	}
	return false
}

// LeaveGrant is one grant-lot in the per-employee leave-balance ledger (openapi
// LeaveGrant). One row per insert, each with its own hard ExpiresAt (no carryover).
// Remaining() = Amount - Consumed - Pending is DERIVED, never a stored column. A lot
// is ACTIVE while now < ExpiresAt. Earmark==nil ⇒ the flat pool (ordinary FIFO); a
// non-nil Earmark restricts the lot to a request of that purpose (LQ-10).
type LeaveGrant struct {
	ID            string
	EmployeeID    string
	Amount        int
	Source        LeaveGrantSource
	Earmark       *string
	Remark        *string
	GrantedAt     time.Time
	EffectiveFrom time.Time
	ExpiresAt     time.Time
	Consumed      int
	Pending       int
	CreatedBy     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Denormalized for display.
	EmployeeName *string

	// Assembled read-time (GET /leave-grants/{id} with include_consumptions).
	Consumptions []LeaveConsumption
}

// Remaining is the derived per-lot balance: Amount - Consumed - Pending. Never driven
// negative by allocation (LQ-5).
func (g LeaveGrant) Remaining() int { return g.Amount - g.Consumed - g.Pending }

// IsActive reports whether the lot is still drawable at t (now < ExpiresAt).
func (g LeaveGrant) IsActive(t time.Time) bool { return t.Before(g.ExpiresAt) }

// LeaveConsumption is one lot-drawdown row linking a LeaveRequest to the LeaveGrant it
// drew from (openapi LeaveConsumption). A request that FIFO-spans multiple lots
// produces one row per lot; reversed exactly on cancel/shorten (LQ-3).
type LeaveConsumption struct {
	ID             string
	LeaveRequestID string
	GrantID        string
	Days           int
	CreatedAt      time.Time
}

// LeaveBalance is the computed per-employee balance over ACTIVE lots (openapi
// LeaveBalance). PoolRemaining is the flat (unearmarked) pool; Earmarked is one line
// per active earmarked lot.
type LeaveBalance struct {
	EmployeeID    string
	EmployeeName  *string
	PoolRemaining int
	PendingTotal  int
	NextExpiry    *time.Time
	Earmarked     []LeaveBalanceEarmarkLine
	AllLots       []LeaveGrant
}

// EmployeeLeaveBalance is one row in the aggregate per-employee balance LIST
// (openapi EmployeeLeaveBalance — the /leave/quotas screen). It rolls up ALL of an
// employee's ACTIVE lots into a single line: the flat pool totals (unearmarked) plus
// the combined earmarked remaining, the soonest expiry, and the active lot count.
// Drill-in to individual lots is GET /leave-grants?employee_id +
// GET /leave-balances/by-employee/{id}.
type EmployeeLeaveBalance struct {
	EmployeeID         string
	FullName           string
	NIK                string
	NIP                string
	PoolTotal          int // Σ amount_days over active unearmarked lots
	PoolConsumed       int // Σ consumed_days, unearmarked
	PoolPending        int // Σ pending_days, unearmarked
	PoolRemaining      int // PoolTotal - PoolConsumed - PoolPending
	EarmarkedRemaining int // Σ(amount-consumed-pending) over active earmarked lots
	NextExpiry         *time.Time
	LotCount           int // count of active lots (earmarked + pool)
}

// LeaveBalanceEarmarkLine is one active earmarked lot in the balance view (openapi
// LeaveBalanceEarmarkLine).
type LeaveBalanceEarmarkLine struct {
	GrantID   string
	Earmark   string
	Source    LeaveGrantSource
	Remaining int
	ExpiresAt time.Time
}

// AllocationLine is one per-lot FIFO split entry in a BalanceCheck snapshot (openapi
// BalanceCheck.allocation[]).
type AllocationLine struct {
	GrantID   string
	Days      int
	ExpiresAt time.Time
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
	ServiceLine    *string
	LeaveTypeID    string
	LeaveTypeCode  *string
	LeaveTypeName  *string
	StartDate      time.Time
	EndDate        time.Time
	Status         LeaveStatus
	DelegateID     *string
	DelegateName   *string
}

// LeaveCalendarClash flags two overlapping leaves at the same company (openapi
// LeaveCalendarEntry clash surface — the FE renders an overlap warning).
type LeaveCalendarClash struct {
	CompanyID  string   `json:"company_id"`
	Date       string   `json:"date"`
	RequestIDs []string `json:"request_ids"`
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
