// Package attendance (repository) — CorrectionRepo implements the correction
// service port over the 07-01 sqlc queries. Reads on the pool; locked re-checks +
// writes via q.WithTx(tx). Approve/Reject return the affected-row count (0 ⇒
// terminal-state, the service maps to 409).
package attendance

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	domain "github.com/hariszaki17/hris-outsource/backend/internal/domain"
	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// CorrectionRepo is the sqlc-backed implementation of svc.CorrectionRepository.
type CorrectionRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.CorrectionRepository = (*CorrectionRepo)(nil)

// NewCorrectionRepo returns a CorrectionRepo backed by pool.
func NewCorrectionRepo(pool *db.Pool) *CorrectionRepo {
	return &CorrectionRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func (r *CorrectionRepo) ListCorrections(ctx context.Context, f svc.CorrectionFilter) ([]att.Correction, error) {
	rows, err := r.q.ListCorrections(ctx, sqlcgen.ListCorrectionsParams{
		CompanyID:       f.CompanyID,
		EmployeeID:      f.EmployeeID,
		StatusIn:        f.Status,
		TypeIn:          f.Type,
		DateFrom:        timePtrToPgDate(f.DateFrom),
		DateTo:          timePtrToPgDate(f.DateTo),
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		PageLimit:       int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]att.Correction, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapCorrectionFromList(row))
	}
	return out, nil
}

func (r *CorrectionRepo) CreateCorrection(ctx context.Context, tx pgx.Tx, p svc.CreateCorrectionParams) (string, error) {
	id, err := r.q.WithTx(tx).CreateCorrection(ctx, sqlcgen.CreateCorrectionParams{
		AttendanceID:             p.AttendanceID,
		RequesterID:              p.RequesterID,
		CompanyID:                p.CompanyID,
		Type:                     p.Type,
		ProposedCheckInAt:        p.ProposedCheckInAt,
		ProposedCheckOutAt:       p.ProposedCheckOutAt,
		ProposedAttendanceCodeID: p.ProposedAttendanceCodeID,
		Reason:                   p.Reason,
		EvidenceFileID:           p.EvidenceFileID,
		AttendanceShiftDate:      pgtype.Date{Time: p.AttendanceShiftDate, Valid: true},
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *CorrectionRepo) GetPendingCorrectionForAttendance(ctx context.Context, attendanceID string) (string, bool, error) {
	id, err := r.q.GetPendingCorrectionForAttendance(ctx, attendanceID)
	if err != nil {
		if mapErr(err) == domain.ErrNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return id, true, nil
}

func (r *CorrectionRepo) GetCorrection(ctx context.Context, id string) (att.Correction, error) {
	row, err := r.q.GetCorrection(ctx, id)
	if err != nil {
		return att.Correction{}, mapErr(err)
	}
	return mapCorrectionFromGet(row), nil
}

func (r *CorrectionRepo) GetCorrectionForUpdate(ctx context.Context, tx pgx.Tx, id string) (att.Correction, error) {
	row, err := r.q.WithTx(tx).GetCorrectionForUpdate(ctx, id)
	if err != nil {
		return att.Correction{}, mapErr(err)
	}
	return mapCorrectionFromForUpdate(row), nil
}

func (r *CorrectionRepo) ApproveCorrection(ctx context.Context, tx pgx.Tx, id string, decidedBy *string) (att.Correction, int64, error) {
	row, err := r.q.WithTx(tx).ApproveCorrection(ctx, sqlcgen.ApproveCorrectionParams{
		DecidedBy: decidedBy,
		ID:        id,
	})
	if err != nil {
		if isNoRows(err) {
			return att.Correction{}, 0, nil // terminal — service emits 409
		}
		return att.Correction{}, 0, err
	}
	return mapCorrectionFromApprove(row), 1, nil
}

func (r *CorrectionRepo) RejectCorrection(ctx context.Context, tx pgx.Tx, id string, decidedBy *string, reason string) (att.Correction, int64, error) {
	rsn := reason
	row, err := r.q.WithTx(tx).RejectCorrection(ctx, sqlcgen.RejectCorrectionParams{
		DecidedBy:    decidedBy,
		RejectReason: &rsn,
		ID:           id,
	})
	if err != nil {
		if isNoRows(err) {
			return att.Correction{}, 0, nil
		}
		return att.Correction{}, 0, err
	}
	return mapCorrectionFromReject(row), 1, nil
}
