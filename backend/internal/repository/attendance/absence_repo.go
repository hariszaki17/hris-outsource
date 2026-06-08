// Package attendance (repository) — AbsenceSweepRepo implements the absence-sweep
// service port over the 00043 sqlc queries. Reads run on the pool; the insert runs
// via q.WithTx(tx) so it shares the audit row's transaction. The ON CONFLICT DO
// NOTHING insert yields pgx.ErrNoRows on a no-op, which maps to created=false.
package attendance

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// AbsenceSweepRepo is the sqlc-backed implementation of svc.AbsenceSweepRepository.
type AbsenceSweepRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.AbsenceSweepRepository = (*AbsenceSweepRepo)(nil)

// NewAbsenceSweepRepo returns an AbsenceSweepRepo backed by pool.
func NewAbsenceSweepRepo(pool *db.Pool) *AbsenceSweepRepo {
	return &AbsenceSweepRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func (r *AbsenceSweepRepo) FindUnreportedAbsences(ctx context.Context, cutoff time.Time, limit int) ([]svc.AbsenceCandidate, error) {
	rows, err := r.q.FindUnreportedAbsences(ctx, sqlcgen.FindUnreportedAbsencesParams{
		Cutoff:    cutoff,
		PageLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]svc.AbsenceCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, svc.AbsenceCandidate{
			ScheduleID:   row.ScheduleID,
			EmployeeID:   row.EmployeeID,
			PlacementID:  row.PlacementID,
			CompanyID:    row.CompanyID,
			SiteID:       row.SiteID,
			PositionID:   row.PositionID,
			ServiceLine:  row.ServiceLine,
			ShiftStartAt: row.ShiftStartAt,
			ShiftEndAt:   row.ShiftEndAt,
		})
	}
	return out, nil
}

func (r *AbsenceSweepRepo) CreateAbsentAttendance(ctx context.Context, tx pgx.Tx, p svc.CreateAbsentParams) (string, bool, error) {
	scheduleID := p.ScheduleID
	shiftStart := p.ShiftStartAt
	shiftEnd := p.ShiftEndAt
	id, err := r.q.WithTx(tx).CreateAbsentAttendance(ctx, sqlcgen.CreateAbsentAttendanceParams{
		EmployeeID:   p.EmployeeID,
		PlacementID:  p.PlacementID,
		ScheduleID:   &scheduleID,
		CompanyID:    p.CompanyID,
		ServiceLine:  p.ServiceLine,
		SiteID:       p.SiteID,
		PositionID:   p.PositionID,
		ShiftStartAt: &shiftStart,
		ShiftEndAt:   &shiftEnd,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil // ON CONFLICT DO NOTHING — row already existed
		}
		return "", false, err
	}
	return id, true, nil
}
