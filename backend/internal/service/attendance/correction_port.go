// Package attendance — the correction repository port (consumed by CorrectionRepo
// in repository/attendance and by CorrectionService). Kept in its own file so the
// Task-1 repository layer and the Task-2 service layer share one definition.
package attendance

import (
	"context"

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
}
