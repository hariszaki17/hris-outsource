// Package overtime holds the dependency-free domain types for the E7 slice
// (F7.1/F7.4 / SWP-OT-* / SWP-HOL-*). These structs are shared between the
// overtime service and repository and map 1:1 onto the openapi Overtime /
// OvertimeCalculation / Holiday shapes (09-02 maps sqlc rows → these → DTOs).
//
// Convention (mirrors internal/domain/leave + internal/domain/attendance):
// nullable columns are pointers; denormalized read-time fields (EmployeeName,
// CompanyName) are pointers too.
//
// Approval routing + the chain-progress timeline are OWNED BY E11 (the configurable
// approval engine): the OT record only carries an ApprovalInstanceID linking to the
// E11 ApprovalInstance. The old two-level (PENDING_L1 → PENDING_HR) state machine +
// the overtime_approvals decision trail were ripped out — clients read the chain via
// GET /approval-instances/{id}.
//
// V1 records HOURS/MINUTES ONLY (INV-2): ReferenceMultiplier is a STORED reference
// from the applied E2 OvertimeRule — it is NEVER applied (no monetary method).
package overtime

import "time"

// OvertimeStatus is the persisted OT lifecycle state. Values are pinned to
// openapi schemas.OvertimeStatus (AUTHORITATIVE) — byte-for-byte. The intermediate
// PENDING_L1 / PENDING_HR states collapsed into a single PENDING (E11 owns chain
// progress); WITHDRAWN collapsed into CANCELLED.
type OvertimeStatus string

const (
	OvertimeStatusPendingAgentConfirm OvertimeStatus = "PENDING_AGENT_CONFIRM"
	OvertimeStatusPending             OvertimeStatus = "PENDING"
	OvertimeStatusApproved            OvertimeStatus = "APPROVED"
	OvertimeStatusRejected            OvertimeStatus = "REJECTED"
	OvertimeStatusCancelled           OvertimeStatus = "CANCELLED"
)

// OvertimeSource is how the OT entered the system (openapi schemas.OvertimeSource).
type OvertimeSource string

const (
	OvertimeSourceRequested            OvertimeSource = "REQUESTED"
	OvertimeSourceAutoDetected         OvertimeSource = "AUTO_DETECTED"
	OvertimeSourceWorkedWithoutRequest OvertimeSource = "WORKED_WITHOUT_REQUEST"
)

// OvertimeTier is the day-type tier / tier_indicator (openapi schemas.OvertimeTier).
// Precedence: HOLIDAY > RESTDAY > WORKDAY.
type OvertimeTier string

const (
	OvertimeTierWorkday OvertimeTier = "WORKDAY"
	OvertimeTierRestday OvertimeTier = "RESTDAY"
	OvertimeTierHoliday OvertimeTier = "HOLIDAY"
)

// tierRank ranks tiers for precedence resolution (higher = wins).
func tierRank(t OvertimeTier) int {
	switch t {
	case OvertimeTierHoliday:
		return 3
	case OvertimeTierRestday:
		return 2
	case OvertimeTierWorkday:
		return 1
	default:
		return 0
	}
}

// TierPrecedence returns the higher of two tiers per HOLIDAY > RESTDAY > WORKDAY
// (used by the 09-02 day_type classification when both a schedule lookup and the
// holiday calendar apply to a work_date).
func TierPrecedence(a, b OvertimeTier) OvertimeTier {
	if tierRank(a) >= tierRank(b) {
		return a
	}
	return b
}

// HolidayCategory is the public-holiday category (openapi schemas.HolidayCategory).
type HolidayCategory string

const (
	HolidayCategoryNational HolidayCategory = "NATIONAL"
	HolidayCategoryRegional HolidayCategory = "REGIONAL"
	HolidayCategoryCustom   HolidayCategory = "CUSTOM"
)

// Overtime is the domain entity for one OT record (openapi Overtime). Nullable
// openapi fields are pointers; *Name fields are denormalized via JOINs. The approval
// chain is OWNED BY E11 — clients read it via ApprovalInstanceID, not an inline
// approvals[] trail.
//
// ReferenceMultiplier is STORED reference only, NOT applied (INV-2): there is no
// monetary method on this type.
type Overtime struct {
	ID           string
	EmployeeID   string
	EmployeeName *string
	CompanyID    *string
	CompanyName  *string
	PlacementID  string
	AttendanceID *string

	WorkDate         time.Time
	PlannedStartTime *string
	PlannedEndTime   *string
	ActualStartTime  *string
	ActualEndTime    *string
	CrossMidnight    bool

	Source  OvertimeSource
	Status  OvertimeStatus
	DayType OvertimeTier

	WorkedMinutes       int
	CountedMinutes      int
	MinMinutesThreshold int
	SkippedTooShort     bool

	ReferenceMultiplier *float64 // STORED reference from the applied E2 rule — NOT applied (INV-2)
	OvertimeRuleID      *string
	HolidayID           *string

	FlaggedNoPreapproval bool
	Reason               *string

	// ApprovalInstanceID links the E11 ApprovalInstance tracking this OT's approval
	// chain (null while PENDING_AGENT_CONFIRM, set at :confirm / direct create).
	ApprovalInstanceID *string

	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CountedFromWorked rounds worked minutes DOWN to the nearest 30-minute increment
// (openapi OvertimeCalculation rule 2): counted = floor(worked / 30) * 30.
func (o Overtime) CountedFromWorked() int { return (o.WorkedMinutes / 30) * 30 }

// Holiday is the domain entity for one public-holiday calendar row (openapi
// Holiday). Holidays are GLOBAL ONLY (decision 2026-06-12 — applicable_service_lines
// dropped). InUseByOvertime is the server-computed flag (true if any APPROVED OT
// references this holiday — the HOLIDAY_IN_USE guard surface).
type Holiday struct {
	ID              string
	Name            string
	Date            time.Time
	Category        HolidayCategory
	Recurring       bool
	InUseByOvertime bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
