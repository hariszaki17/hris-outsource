// Package overtime holds the dependency-free domain types for the E7 slice
// (F7.1/F7.4 / SWP-OT-* / SWP-HOL-*). These structs are shared between the
// overtime service and repository and map 1:1 onto the openapi Overtime /
// OvertimeCalculation / Holiday shapes (09-02 maps sqlc rows → these → DTOs).
//
// Convention (mirrors internal/domain/leave + internal/domain/attendance):
// nullable columns are pointers; denormalized read-time fields (EmployeeName,
// CompanyName) are pointers too. The workflow state machine
// (PENDING_AGENT_CONFIRM → PENDING_L1 → PENDING_HR → APPROVED; reject at either
// level → REJECTED; withdraw → WITHDRAWN) is enforced in the 09-02 service; these
// types only carry state.
//
// V1 records HOURS/MINUTES ONLY (INV-2): ReferenceMultiplier is a STORED reference
// from the applied E2 OvertimeRule — it is NEVER applied (no monetary method).
package overtime

import "time"

// OvertimeStatus is the persisted OT lifecycle state. Values are pinned to
// openapi schemas.OvertimeStatus (AUTHORITATIVE) — byte-for-byte.
type OvertimeStatus string

const (
	OvertimeStatusPendingAgentConfirm OvertimeStatus = "PENDING_AGENT_CONFIRM"
	OvertimeStatusPendingL1           OvertimeStatus = "PENDING_L1"
	OvertimeStatusPendingHR           OvertimeStatus = "PENDING_HR"
	OvertimeStatusApproved            OvertimeStatus = "APPROVED"
	OvertimeStatusRejected            OvertimeStatus = "REJECTED"
	OvertimeStatusWithdrawn           OvertimeStatus = "WITHDRAWN"
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

// OvertimeApproval is one immutable decision-trail row (the overtime_approvals
// table). 09-02 maps these into Overtime.Approvals[] (openapi Overtime.approvals).
type OvertimeApproval struct {
	Level        int     // 1 = leader/L1, 2 = HR final
	Decision     string  // APPROVED | REJECTED | OVERRIDE_APPROVED
	ApproverID   *string // SWP-USR-* / SWP-EMP-*
	ApproverName *string
	Reason       *string
	DecidedAt    time.Time
}

// Overtime is the domain entity for one OT record (openapi Overtime). Nullable
// openapi fields are pointers; *Name fields are denormalized via JOINs. Approvals
// is assembled by 09-02 from the overtime_approvals rows.
//
// ReferenceMultiplier is STORED reference only, NOT applied (INV-2): there is no
// monetary method on this type.
type Overtime struct {
	ID            string
	EmployeeID    string
	EmployeeName  *string
	CompanyID     *string
	CompanyName   *string
	PlacementID   string
	AttendanceID  *string
	ServiceLineID *string

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

	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Assembled read-time aggregate (09-02).
	Approvals []OvertimeApproval
}

// CountedFromWorked rounds worked minutes DOWN to the nearest 30-minute increment
// (openapi OvertimeCalculation rule 2): counted = floor(worked / 30) * 30.
func (o Overtime) CountedFromWorked() int { return (o.WorkedMinutes / 30) * 30 }

// Holiday is the domain entity for one public-holiday calendar row (openapi
// Holiday). InUseByOvertime is the server-computed flag (true if any APPROVED OT
// references this holiday — the HOLIDAY_IN_USE guard surface).
type Holiday struct {
	ID                     string
	Name                   string
	Date                   time.Time
	Category               HolidayCategory
	Recurring              bool
	ApplicableServiceLines []string
	InUseByOvertime        bool
	CreatedAt              time.Time
	UpdatedAt              time.Time
}
