// Package domain — placement types for the E3 slice (F3.1/F3.2 / PLC-* / SL-*).
// These dependency-free structs are shared between the placement service and
// repository. Mirrors the shape of domain/people.go.
package domain

import "time"

// Placement is the domain entity for an agent placement (F3.1 / PLC-*).
//
// LifecycleStatus is the persisted DB value; date-derived statuses (EXPIRING,
// PENDING_START) are resolved at the DTO boundary (Asia/Jakarta TZ layer).
// The *Name fields are denormalized at read time via LEFT JOINs.
// Nullable columns are pointers.
type Placement struct {
	ID                string
	EmployeeID        string
	AgreementID       *string // nil = pending agreement (awaiting_agreement = true)
	ClientCompanyID   string
	SiteID            string // INV-5: required
	ServiceLineID     string
	PositionID        string
	StartDate         time.Time
	EndDate           *time.Time // nil = open-ended (PKWTT)
	Notes             *string
	LifecycleStatus   string // PENDING_START|ACTIVE|EXTENDED|EXPIRING|ENDED|TRANSFERRED|TERMINATED|RESIGNED|SUPERSEDED
	StatusChangedAt   time.Time
	EndedReason       *string
	EndedAt           *time.Time
	TerminationReason *string
	ResignAt          *time.Time
	PredecessorID     *string
	SuccessorID       *string
	BackdateReason    *string
	CreatedBy         *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	// AwaitingAgreement is the orthogonal compliance flag (= AgreementID == nil),
	// derived in SQL and carried through every read/write row. NOT a lifecycle state.
	AwaitingAgreement bool
	// Denormalized for display (server-authoritative; filled via JOINs).
	EmployeeName      *string
	ClientCompanyName *string
	SiteName          *string
	ServiceLineName   *string
	PositionName      *string
	AgreementType     *string
	// Warnings are non-blocking soft warnings attached at write time (not persisted).
	Warnings []string
}

// PlacementHistory is one row of a placement's lifecycle audit trail.
// One row per transition (create/renew/transfer/end/resign/terminate/supersede).
type PlacementHistory struct {
	ID            int64
	PlacementID   string
	Action        string
	ActorUserID   *string
	Reason        *string
	EffectiveDate *time.Time
	StatusBefore  *string
	StatusAfter   *string
	Notes         *string
	CreatedAt     time.Time
}

// ShiftLeaderAssignment is the domain entity for a shift-leader assignment (F3.4 / SL-*).
//
// The leadership unit is ClientCompanyID (always) + SiteID (nullable; set only
// when the company's leader_scope='site'). Active is derived (UnassignedAt == nil).
type ShiftLeaderAssignment struct {
	ID              string
	ClientCompanyID string
	SiteID          *string // null when leader_scope=company; set when =site
	EmployeeID      string
	AssignedAt      time.Time
	UnassignedAt    *time.Time // null while active
	AssignedBy      *string    // SWP-USR-<N>; "system" for auto-vacate
	VacatedReason   *string    // REASSIGNED|PLACEMENT_ENDED|MANUAL|COMPANY_ARCHIVED
	Notes           *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	// Denormalized for display.
	ClientCompanyName *string
	EmployeeName      *string
}

// Active reports whether the assignment is currently in force.
func (s ShiftLeaderAssignment) Active() bool { return s.UnassignedAt == nil }

// CompanyRosterSummary holds the aggregate counts returned with a company roster (RO-5).
type CompanyRosterSummary struct {
	TotalActive    int
	TotalScheduled int
	TotalExpiring  int
	ByServiceLine  []RosterServiceLineCount
	ByStatus       []RosterStatusCount
}

// RosterServiceLineCount is one by_service_line bucket of CompanyRosterSummary.
type RosterServiceLineCount struct {
	ServiceLineID   string
	ServiceLineName string
	Count           int
}

// RosterStatusCount is one by_status bucket of CompanyRosterSummary.
type RosterStatusCount struct {
	Status string
	Count  int
}

// PlacementStats holds the global placement aggregates backing the /placements
// dashboard stat cards (F3.1 / C2SSLA). Counts are over non-deleted placements,
// optionally scoped to a single company (shift-leader RBAC).
type PlacementStats struct {
	ClientCompanyCount int64
	ActiveCount        int64
	ExpiringCount      int64
	PendingCount       int64
}

// PlacementFilter holds the decoded query parameters for GET /placements and the
// company roster. All fields optional; cursor fields are set when paginating.
//
// Status (single) and StatusIn (CSV) both filter the lifecycle_status column.
type PlacementFilter struct {
	CompanyID             *string
	ServiceLineID         *string
	EmployeeID            *string
	AgreementID           *string
	Status                *string  // single → lifecycle_status =
	StatusIn              []string // CSV → lifecycle_status = ANY
	Q                     *string  // ILIKE over agent name / employee_id / company name
	EndDateLTE            *time.Time
	AwaitingAgreement     *bool // nil = no filter; true/false = pending-agreement filter
	IncludeHistory        bool
	Limit                 int
	CursorStatusChangedAt *time.Time
	CursorID              *string
}

// ExpiringFilter holds the decoded params for GET /placements/expiring.
// Cutoff = today(Asia/Jakarta) + WithinDays (computed in the service).
type ExpiringFilter struct {
	Cutoff        time.Time
	CompanyID     *string
	Limit         int
	CursorEndDate *time.Time
	CursorID      *string
}

// ShiftLeaderFilter holds the decoded query parameters for listing shift-leader
// assignments.
type ShiftLeaderFilter struct {
	CompanyID  *string
	EmployeeID *string
	ActiveOnly bool
}
