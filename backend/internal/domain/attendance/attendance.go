// Package attendance holds the dependency-free domain types for the E5 slice
// (F5.1/F5.2/F5.3 / SWP-ATT-* / SWP-COR-*). These structs are shared between the
// attendance service and repository and map 1:1 onto the openapi Attendance /
// Correction shapes (07-02 maps sqlc rows → these → DTOs).
//
// Convention (mirrors internal/domain/placement.go): nullable columns are
// pointers; date-derived/denormalized read-time fields are pointers too. The web
// surface is exceptions-only verification — geofence/lateness/auto-close are STORED
// (no runtime compute), so GeofenceIn/Out are assembled from stored columns.
package attendance

import "time"

// AttendanceStatus is the persisted shift outcome (openapi AttendanceStatus).
type AttendanceStatus string

const (
	StatusPresent    AttendanceStatus = "PRESENT"
	StatusLate       AttendanceStatus = "LATE"
	StatusIncomplete AttendanceStatus = "INCOMPLETE"
	StatusAbsent     AttendanceStatus = "ABSENT"
	StatusOnLeave    AttendanceStatus = "ON_LEAVE"
)

// VerificationStatus is the verification-queue state (openapi VerificationStatus).
// Only PENDING/ESCALATED are verifiable; AUTO_APPROVED never enters the queue
// (INV-3 exceptions-only); VERIFIED/REJECTED are terminal.
type VerificationStatus string

const (
	VerificationAutoApproved VerificationStatus = "AUTO_APPROVED"
	VerificationPending      VerificationStatus = "PENDING"
	VerificationVerified     VerificationStatus = "VERIFIED"
	VerificationRejected     VerificationStatus = "REJECTED"
	VerificationEscalated    VerificationStatus = "ESCALATED"
)

// Flag is one verification-queue driver (openapi AttendanceFlag). Stored in the
// flags text[] column; any flag → PENDING (or ESCALATED for leader-own).
type Flag string

const (
	FlagLate                  Flag = "LATE"
	FlagEarly                 Flag = "EARLY"
	FlagOutsideGeofence       Flag = "OUTSIDE_GEOFENCE"
	FlagUnscheduled           Flag = "UNSCHEDULED"
	FlagEscalated             Flag = "ESCALATED"
	FlagCorrected             Flag = "CORRECTED"
	FlagAutoClosed            Flag = "AUTO_CLOSED"
	FlagAbsent                Flag = "ABSENT"
	FlagNeedsCodeVerification Flag = "NEEDS_CODE_VERIFICATION"
	FlagManualEntry           Flag = "MANUAL_ENTRY"
)

// Flexible check-out window (F5.1). An open record (check_out_at IS NULL) only means
// "currently clocked in" — i.e. the next toggle is a CHECK OUT — while now is within
// the shift's checkout window: shift_end + CheckoutWindowGrace. Past that, the row is a
// STALE forgotten clock-out: the next toggle is a fresh CHECK IN (the stale row is
// auto-closed server-side). This is the synchronous complement to the absence sweep —
// it fires inside the clock-in request even for users/companies the cron skips.
const (
	// CheckoutWindowGrace extends shift_end before an open row stops counting as
	// "checked in". Distinct from the 15-min lateness/early grace.
	CheckoutWindowGrace = 4 * time.Hour
	// FallbackShiftHours is the assumed shift length when the record has no usable
	// scheduled shift (unscheduled clock-in, or a 00:00==00:00 sentinel).
	FallbackShiftHours = 18 * time.Hour
)

// ShiftEndTimestamp returns the instant the record's shift is considered to end —
// schedule-aware with an overnight-correct snapshot, falling back to check_in +
// FallbackShiftHours when there is no usable shift. ShiftEndAt is the timestamptz the
// clock-in snapshotted from the schedule (cross_midnight already added a day), so
// overnight shifts need no extra handling here. Returns the zero time only when the
// record has neither a usable shift nor a clock-in (a true ABSENT row).
func ShiftEndTimestamp(a Attendance) time.Time {
	if hasUsableShift(a) {
		return *a.ShiftEndAt
	}
	if a.CheckInAt != nil {
		return a.CheckInAt.Add(FallbackShiftHours)
	}
	return time.Time{}
}

// hasUsableShift reports whether the snapshotted shift window is meaningful: a non-nil
// end that is not equal to the start (start==end is the 00:00:00 "no fixed time"
// sentinel → treat as no shift, per the fallback rule).
func hasUsableShift(a Attendance) bool {
	if a.ShiftEndAt == nil {
		return false
	}
	if a.ShiftStartAt != nil && a.ShiftEndAt.Equal(*a.ShiftStartAt) {
		return false
	}
	return true
}

// IsWithinCheckoutWindow reports whether the record is still an open clock-in whose
// checkout window has not elapsed (now <= shift_end + CheckoutWindowGrace) — i.e. the
// mobile toggle should show CHECK OUT. False when the row is already checked out, has
// no clock-in (true ABSENT), or its window has passed (a stale forgotten clock-out, for
// which the mobile toggle should show CHECK IN instead).
func IsWithinCheckoutWindow(a Attendance, now time.Time) bool {
	if a.CheckOutAt != nil {
		return false
	}
	if a.CheckInAt == nil {
		return false
	}
	end := ShiftEndTimestamp(a)
	if end.IsZero() {
		return false
	}
	return !now.After(end.Add(CheckoutWindowGrace))
}

// ServiceLine enumerates the placement service lines carried on the record.
const (
	ServiceLineFacilityServices   = "facility_services"
	ServiceLineBuildingManagement = "building_management"
	ServiceLineParking            = "parking"
)

// GeofenceCheck is the openapi GeofenceCheck — the stored geofence result for one
// clock event (assembled from the *_geofence / *_distance_m / geofence_radius_m
// columns; nil when the underlying inside flag is absent).
type GeofenceCheck struct {
	Inside    bool `json:"inside"`
	DistanceM int  `json:"distance_m"`
	RadiusM   int  `json:"radius_m"`
}

// Attendance is the domain entity for one attendance record (openapi Attendance).
// Nullable openapi fields are pointers; *Name fields are denormalized at read time
// via LEFT JOINs. Flags is the parsed flags[] column.
type Attendance struct {
	ID               string
	EmployeeID       string
	PlacementID      string
	ScheduleID       *string // nil = unscheduled
	CompanyID        string
	ServiceLine      string
	SiteID           string // denormalized from placement → site (E2 F2.6 / INV-5)
	PositionID       string // denormalized from placement → position (E2)
	AttendanceCodeID *string

	ShiftStartAt *time.Time
	ShiftEndAt   *time.Time

	CheckInAt  *time.Time // nil for a true ABSENT record (no clock-in by shift end)
	CheckOutAt *time.Time // nil while open / until auto-close
	LatIn      *float64   // nil for a true ABSENT record (no clock-in GPS)
	LngIn      *float64
	LatOut     *float64
	LngOut     *float64
	PhotoInID  *string
	PhotoOutID *string

	WFO           bool
	IsLate        bool
	LateMinutes   int
	WorkedMinutes *int // nil while open
	AutoClosed    bool

	// Geofence assembled from stored columns (nil when no inside flag present).
	GeofenceIn  *GeofenceCheck
	GeofenceOut *GeofenceCheck

	Status             AttendanceStatus
	VerificationStatus VerificationStatus
	Flags              []Flag

	VerifiedBy       *string
	VerifiedAt       *time.Time
	RejectedBy       *string
	RejectedAt       *time.Time
	RejectReason     *string
	LastCorrectionID *string

	CreatedBy *string // SWP-EMP-* of who created the record (nil = system)

	CreatedAt time.Time
	UpdatedAt time.Time

	// Denormalized for display (server-authoritative; filled via JOINs).
	EmployeeName *string
	CompanyName  *string
	SiteName     *string
	PositionName *string
}
