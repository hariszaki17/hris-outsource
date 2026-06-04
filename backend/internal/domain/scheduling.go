// Package domain — scheduling types for the E4 slice (F4.1/F4.2/F4.3 / SM-*).
// These dependency-free structs are shared between the scheduling service and
// repository. Mirrors the shape of domain/placement.go.
//
// Times (start_time/end_time/break_*) are HH:MM strings (Asia/Jakarta,
// CONVENTIONS §10) — the DB columns are text, not time. WorkDate is a time.Time;
// the DTO layer in 06-02 formats it to "YYYY-MM-DD" via the Asia/Jakarta TZ layer
// (mirrors Phase-5 lifecycle date formatting). The repository converts the sqlc
// pgtype.Date ↔ time.Time at its boundary (same pattern as placements).
package domain

import "time"

// ShiftMaster is the domain entity for a reusable shift template (F4.1 / SM-*).
//
// ServiceLineID nil = untagged (applies to all service lines, SM-3). CrossMidnight
// is server-derived (end_time <= start_time). InUseCount = live schedule_entries
// referencing this master. IsActive drives the ACTIVE/INACTIVE status at the DTO
// boundary. *Name fields are denormalized at read time via LEFT JOINs.
type ShiftMaster struct {
	ID         string
	Name       string
	StartTime  string // HH:MM
	EndTime    string // HH:MM
	BreakStart *string
	BreakEnd   *string

	ServiceLineID   *string
	ServiceLineName *string // denormalized via JOIN

	CrossMidnight bool
	IsActive      bool
	InUseCount    int64

	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ScheduleEntry is one scheduled shift for one agent on one work_date (F4.2/F4.3).
//
// Anchored to a placement (INV-2). StartTime/EndTime are a snapshot of the shift
// master's window at write time. Status is the persisted value; CANCELLED_BY_LEAVE
// / MODIFIED reflect leave-driven transitions. ShiftMasterID is nil only when
// IsDayOff. ReplacedEntryID links a MODIFIED replacement to its predecessor.
// CompanyID/CompanyName are denormalized from the linked placement's company.
type ScheduleEntry struct {
	ID          string
	EmployeeID  string
	PlacementID string
	CompanyID   string // from placement.client_company_id

	ServiceLineID   *string
	ShiftMasterID   *string
	ShiftMasterName *string // denormalized via JOIN
	EmployeeName    *string // denormalized via JOIN
	CompanyName     *string // denormalized via JOIN

	StartTime *string // HH:MM snapshot
	EndTime   *string // HH:MM snapshot

	CrossMidnight bool
	WorkDate      time.Time // formatted "YYYY-MM-DD" at the DTO boundary
	Status        string    // SCHEDULED|CANCELLED_BY_LEAVE|MODIFIED
	IsDayOff      bool

	ReplacedEntryID *string

	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ShiftMasterFilter holds the decoded query parameters for GET /shift-masters.
// All fields optional; Cursor is set when paginating.
//
// Status is the API ACTIVE/INACTIVE token; the repository maps it to the
// is_active bool narg.
type ShiftMasterFilter struct {
	ServiceLineID *string
	Status        *string // ACTIVE | INACTIVE
	Q             *string
	Limit         int32
	Cursor        *string
}

// ScheduleFilter holds the decoded query parameters for GET /schedule.
//
// CompanyID + the work_date range are required (the grid always loads a company
// over a window). EmployeeID and StatusIn narrow the result.
type ScheduleFilter struct {
	CompanyID  string
	StartDate  time.Time
	EndDate    time.Time
	EmployeeID *string
	StatusIn   []string // SCHEDULED|CANCELLED_BY_LEAVE|MODIFIED
}
