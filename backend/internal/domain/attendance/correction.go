package attendance

import "time"

// CorrectionType is the kind of correction requested (openapi CorrectionType).
type CorrectionType string

const (
	CorrectionTypeCheckIn  CorrectionType = "CHECK_IN"
	CorrectionTypeCheckOut CorrectionType = "CHECK_OUT"
	CorrectionTypeCode     CorrectionType = "CODE"
	CorrectionTypeOther    CorrectionType = "OTHER"
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
	ID           string
	AttendanceID string
	RequesterID  string
	CompanyID    string // denormalized from attendance for leader-scope queries
	Type         CorrectionType

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
