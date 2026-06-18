// Package attendance (repository) — ActivityRepo implements the activity service port
// (F5.8 / SWP-ACT-*) over the activities.sql queries. Reads run on the pool; the
// create INSERT / soft-delete UPDATE run via q.WithTx(tx) so they share the audit
// row's transaction. CountActivities backs the clock-out gate (AA-7) and is exposed on
// the clock repo too — see clock_repo.go.
package attendance

import (
	"context"

	"github.com/jackc/pgx/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// ActivityRepo is the sqlc-backed implementation of svc.ActivityRepository.
type ActivityRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.ActivityRepository = (*ActivityRepo)(nil)

// NewActivityRepo returns an ActivityRepo backed by pool.
func NewActivityRepo(pool *db.Pool) *ActivityRepo {
	return &ActivityRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func (r *ActivityRepo) CreateActivity(ctx context.Context, tx pgx.Tx, p svc.CreateActivityParams) (att.AttendanceActivity, error) {
	row, err := r.q.WithTx(tx).CreateActivity(ctx, sqlcgen.CreateActivityParams{
		AttendanceID: p.AttendanceID,
		EmployeeID:   p.EmployeeID,
		Note:         p.Note,
	})
	if err != nil {
		return att.AttendanceActivity{}, mapErr(err)
	}
	return att.AttendanceActivity{
		ID:           row.ID,
		AttendanceID: row.AttendanceID,
		EmployeeID:   row.EmployeeID,
		Note:         row.Note,
		RecordedAt:   row.RecordedAt,
		CreatedAt:    row.CreatedAt,
	}, nil
}

func (r *ActivityRepo) ListActivities(ctx context.Context, f svc.ActivityFilter) ([]att.AttendanceActivity, error) {
	rows, err := r.q.ListActivitiesByAttendance(ctx, sqlcgen.ListActivitiesByAttendanceParams{
		AttendanceID:     f.AttendanceID,
		CursorRecordedAt: f.CursorRecordedAt,
		CursorID:         f.CursorID,
		PageLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]att.AttendanceActivity, 0, len(rows))
	for _, row := range rows {
		out = append(out, att.AttendanceActivity{
			ID:           row.ID,
			AttendanceID: row.AttendanceID,
			EmployeeID:   row.EmployeeID,
			Note:         row.Note,
			RecordedAt:   row.RecordedAt,
			CreatedAt:    row.CreatedAt,
		})
	}
	return out, nil
}

func (r *ActivityRepo) SoftDeleteActivity(ctx context.Context, tx pgx.Tx, id, attendanceID, employeeID string) (int64, error) {
	return r.q.WithTx(tx).SoftDeleteActivity(ctx, sqlcgen.SoftDeleteActivityParams{
		ID:           id,
		AttendanceID: attendanceID,
		EmployeeID:   employeeID,
	})
}
