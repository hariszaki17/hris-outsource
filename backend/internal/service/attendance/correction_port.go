// Package attendance — the correction repository port (consumed by CorrectionRepo
// in repository/attendance and by CorrectionService). Kept in its own file so the
// Task-1 repository layer and the Task-2 service layer share one definition.
package attendance

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
)

// CorrectionRepository is the data dependency for the correction service.
type CorrectionRepository interface {
	ListCorrections(ctx context.Context, f CorrectionFilter) ([]att.Correction, error)
	GetCorrection(ctx context.Context, id string) (att.Correction, error)
	GetCorrectionForUpdate(ctx context.Context, tx pgx.Tx, id string) (att.Correction, error)
	ApproveCorrection(ctx context.Context, tx pgx.Tx, id string, decidedBy *string) (att.Correction, int64, error)
	RejectCorrection(ctx context.Context, tx pgx.Tx, id string, decidedBy *string, reason string) (att.Correction, int64, error)
	// CreateCorrection inserts a new PENDING correction (in tx) and returns its id.
	CreateCorrection(ctx context.Context, tx pgx.Tx, p CreateCorrectionParams) (string, error)
	// GetPendingCorrectionForAttendance returns the active PENDING correction id for
	// a target attendance (found=false when none) — the CREATE-path dedupe guard.
	GetPendingCorrectionForAttendance(ctx context.Context, attendanceID string) (id string, found bool, err error)
}

// CreateCorrectionParams is the repo-layer insert payload for a new correction
// (company_id + attendance_shift_date denormalized from the target attendance).
type CreateCorrectionParams struct {
	AttendanceID             string
	RequesterID              string
	CompanyID                string
	Type                     string
	ProposedCheckInAt        *time.Time
	ProposedCheckOutAt       *time.Time
	ProposedAttendanceCodeID *string
	Reason                   string
	EvidenceFileID           *string
	AttendanceShiftDate      time.Time
}
