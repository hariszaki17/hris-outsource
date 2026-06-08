// Package attendance (repository) — AttendanceRepo implements the attendance
// service port over the 07-01 sqlc queries. Reads on the pool; locked re-checks +
// writes via q.WithTx(tx). pgx.ErrNoRows → domain.ErrNotFound. Verify/Reject
// return the affected-row count (0 ⇒ terminal-state, the service maps to 409).
package attendance

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	domain "github.com/hariszaki17/hris-outsource/backend/internal/domain"
	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// AttendanceRepo is the sqlc-backed implementation of svc.AttendanceRepository.
type AttendanceRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.AttendanceRepository = (*AttendanceRepo)(nil)

// NewAttendanceRepo returns an AttendanceRepo backed by pool.
func NewAttendanceRepo(pool *db.Pool) *AttendanceRepo {
	return &AttendanceRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func timePtrToPgDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// intPtrToI32Ptr narrows a *int to *int32 for sqlc nullable integer params.
func intPtrToI32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	n := int32(*v)
	return &n
}

func (r *AttendanceRepo) ListAttendance(ctx context.Context, f svc.AttendanceFilter) ([]att.Attendance, error) {
	var exceptions *bool
	if f.ExceptionsOnly {
		t := true
		exceptions = &t
	}
	rows, err := r.q.ListAttendance(ctx, sqlcgen.ListAttendanceParams{
		CompanyID:            f.CompanyID,
		EmployeeID:           f.EmployeeID,
		ServiceLine:          f.ServiceLine,
		SiteID:               f.SiteID,
		PositionID:           f.PositionID,
		VerificationStatusIn: f.VerificationStatus,
		StatusIn:             f.Status,
		DateFrom:             timePtrToPgDate(f.DateFrom),
		DateTo:               timePtrToPgDate(f.DateTo),
		Exceptions:           exceptions,
		CursorCheckInAt:      f.CursorCheckInAt,
		CursorID:             f.CursorID,
		PageLimit:            int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]att.Attendance, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAttendanceFromList(row))
	}
	return out, nil
}

func (r *AttendanceRepo) GetAttendance(ctx context.Context, id string) (att.Attendance, error) {
	row, err := r.q.GetAttendance(ctx, id)
	if err != nil {
		return att.Attendance{}, mapErr(err)
	}
	return mapAttendanceFromGet(row), nil
}

func (r *AttendanceRepo) GetAttendanceForUpdate(ctx context.Context, tx pgx.Tx, id string) (att.Attendance, error) {
	row, err := r.q.WithTx(tx).GetAttendanceForUpdate(ctx, id)
	if err != nil {
		return att.Attendance{}, mapErr(err)
	}
	return mapAttendanceFromForUpdate(row), nil
}

func (r *AttendanceRepo) VerifyAttendance(ctx context.Context, tx pgx.Tx, id string, verifiedBy *string) (att.Attendance, int64, error) {
	row, err := r.q.WithTx(tx).VerifyAttendance(ctx, sqlcgen.VerifyAttendanceParams{
		VerifiedBy: verifiedBy,
		ID:         id,
	})
	if err != nil {
		if isNoRows(err) {
			return att.Attendance{}, 0, nil // terminal state — service emits 409
		}
		return att.Attendance{}, 0, err
	}
	return mapAttendanceFromVerify(row), 1, nil
}

func (r *AttendanceRepo) RejectAttendance(ctx context.Context, tx pgx.Tx, id string, rejectedBy *string, reason string) (att.Attendance, int64, error) {
	rsn := reason
	row, err := r.q.WithTx(tx).RejectAttendance(ctx, sqlcgen.RejectAttendanceParams{
		RejectedBy:   rejectedBy,
		RejectReason: &rsn,
		ID:           id,
	})
	if err != nil {
		if isNoRows(err) {
			return att.Attendance{}, 0, nil
		}
		return att.Attendance{}, 0, err
	}
	return mapAttendanceFromReject(row), 1, nil
}

func (r *AttendanceRepo) ApplyCorrectionToAttendance(ctx context.Context, tx pgx.Tx, p svc.ApplyCorrectionParams) (att.Attendance, error) {
	row, err := r.q.WithTx(tx).ApplyCorrectionToAttendance(ctx, sqlcgen.ApplyCorrectionToAttendanceParams{
		CheckInAt:        p.CheckInAt,
		CheckOutAt:       p.CheckOutAt,
		AttendanceCodeID: p.AttendanceCodeID,
		Status:           p.Status,
		IsLate:           p.IsLate,
		LateMinutes:      intPtrToI32Ptr(p.LateMinutes),
		LastCorrectionID: p.LastCorrectionID,
		ID:               p.ID,
	})
	if err != nil {
		return att.Attendance{}, mapErr(err)
	}
	return mapAttendanceFromApply(row), nil
}

// isNoRows reports whether the error is the :one "no rows" sentinel — used by the
// state-guarded UPDATE...RETURNING queries to detect a terminal-state no-op.
func isNoRows(err error) bool {
	return err == pgx.ErrNoRows || mapErr(err) == domain.ErrNotFound
}
