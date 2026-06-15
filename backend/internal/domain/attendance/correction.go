package attendance

import "time"

// CorrectionType is the kind of correction requested (openapi CorrectionType).
type CorrectionType string

const (
	CorrectionTypeCheckIn  CorrectionType = "CHECK_IN"
	CorrectionTypeCheckOut CorrectionType = "CHECK_OUT"
	CorrectionTypeCode     CorrectionType = "CODE"
	CorrectionTypeOther    CorrectionType = "OTHER"
	// CorrectionTypeNewEntry creates an attendance record for a day with no existing
	// record (and possibly no configured shift). Carries WorkDate instead of an
	// AttendanceID; on approval the OnApproved hook inserts the attendance row.
	CorrectionTypeNewEntry CorrectionType = "NEW_ENTRY"
)

// CorrectionStatus is the correction state machine (openapi CorrectionStatus):
// PENDING → APPROVED|APPLIED|REJECTED|CANCELLED. Only PENDING is decidable.
type CorrectionStatus string

const (
	CorrectionStatusPending   CorrectionStatus = "PENDING"
	CorrectionStatusApproved  CorrectionStatus = "APPROVED"
	CorrectionStatusApplied   CorrectionStatus = "APPLIED"
	CorrectionStatusRejected  CorrectionStatus = "REJECTED"
	CorrectionStatusCancelled CorrectionStatus = "CANCELLED"
)

// DiffRow is one field-by-field difference between original_snapshot and the
// proposed/applied state (openapi Correction.diff[] item). Only rendered on
// GET /corrections/{id}.
type DiffRow struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// Correction is the domain entity for one attendance correction (openapi
// Correction). Nullable openapi fields are pointers; OriginalSnapshot is the
// frozen pre-application copy (CR-5); Diff is server-rendered on detail only.
type Correction struct {
	ID string
	// AttendanceID is the target record. Empty ("") for a still-PENDING NEW_ENTRY
	// (no record exists yet); set to the created record once a NEW_ENTRY is APPLIED.
	AttendanceID string
	// WorkDate is set for NEW_ENTRY corrections (the day the record is created for).
	WorkDate    *time.Time
	RequesterID string
	CompanyID   string // denormalized from attendance (or active placement for NEW_ENTRY) for leader-scope queries
	Type        CorrectionType
	// ApprovalInstanceID is the E11 instance opened on submit (SWP-APV-*).
	ApprovalInstanceID *string

	ProposedCheckInAt        *time.Time
	ProposedCheckOutAt       *time.Time
	ProposedAttendanceCodeID *string

	Reason         string
	EvidenceFileID *string

	Status       CorrectionStatus
	DecidedBy    *string
	DecidedAt    *time.Time
	RejectReason *string

	OriginalSnapshot map[string]any
	// AttendanceShiftDate is the target shift date — basis for the
	// OUTSIDE_CORRECTION_WINDOW 7-day check (not in the openapi DTO; internal).
	AttendanceShiftDate time.Time

	CreatedAt time.Time
	UpdatedAt time.Time

	// Diff is populated only on GET /corrections/{id} (server-rendered).
	Diff []DiffRow

	// Denormalized for display (filled via JOINs).
	RequesterName *string
	CompanyName   *string
}
