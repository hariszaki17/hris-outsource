// Package payroll (repository) — PeriodRepo + ClarificationRepo implement the F8.5
// service ports over the sqlc period queries. Reads on the pool; guarded
// transitions + the lock snapshot via q.WithTx(tx). pgx.ErrNoRows on a guarded
// UPDATE means the status guard rejected -> found=false (the service maps to a
// 409/422), NOT an error. Mirrors the attendance repo's verify/reject found-count
// convention.
package payroll

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// PeriodRepo is the sqlc-backed implementation of svc.PeriodRepository.
type PeriodRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.PeriodRepository = (*PeriodRepo)(nil)

// NewPeriodRepo returns a PeriodRepo backed by pool.
func NewPeriodRepo(pool *db.Pool) *PeriodRepo {
	return &PeriodRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// --- period FSM ---

func (r *PeriodRepo) GetPeriod(ctx context.Context, year, month int) (dom.PayrollPeriod, bool, error) {
	row, err := r.q.GetPayrollPeriod(ctx, sqlcgen.GetPayrollPeriodParams{
		Year: int32(year), Month: int32(month),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dom.PayrollPeriod{}, false, nil
	}
	if err != nil {
		return dom.PayrollPeriod{}, false, err
	}
	return mapGetPeriod(row), true, nil
}

func (r *PeriodRepo) GetOrCreatePeriod(ctx context.Context, year, month int) (dom.PayrollPeriod, error) {
	row, err := r.q.CreatePayrollPeriod(ctx, sqlcgen.CreatePayrollPeriodParams{
		Year: int32(year), Month: int32(month),
	})
	if err != nil {
		return dom.PayrollPeriod{}, err
	}
	return mapCreatePeriod(row), nil
}

func (r *PeriodRepo) GetPeriodForUpdate(ctx context.Context, tx pgx.Tx, year, month int) (dom.PayrollPeriod, bool, error) {
	row, err := r.q.WithTx(tx).GetPayrollPeriodForUpdate(ctx, sqlcgen.GetPayrollPeriodForUpdateParams{
		Year: int32(year), Month: int32(month),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dom.PayrollPeriod{}, false, nil
	}
	if err != nil {
		return dom.PayrollPeriod{}, false, err
	}
	return mapGetPeriodForUpdate(row), true, nil
}

func (r *PeriodRepo) Lock(ctx context.Context, tx pgx.Tx, p svc.LockPeriodParams) (dom.PayrollPeriod, bool, error) {
	row, err := r.q.WithTx(tx).LockPayrollPeriod(ctx, sqlcgen.LockPayrollPeriodParams{
		ID:              p.ID,
		LockedBy:        p.LockedBy,
		ForceLocked:     p.ForceLocked,
		ForceLockReason: p.ForceLockReason,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dom.PayrollPeriod{}, false, nil
	}
	if err != nil {
		return dom.PayrollPeriod{}, false, err
	}
	return mapLockPeriod(row), true, nil
}

func (r *PeriodRepo) Reopen(ctx context.Context, tx pgx.Tx, id string, reopenedBy *string) (dom.PayrollPeriod, bool, error) {
	row, err := r.q.WithTx(tx).ReopenPayrollPeriod(ctx, sqlcgen.ReopenPayrollPeriodParams{
		ID:         id,
		ReopenedBy: reopenedBy,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dom.PayrollPeriod{}, false, nil
	}
	if err != nil {
		return dom.PayrollPeriod{}, false, err
	}
	return mapReopenPeriod(row), true, nil
}

// --- snapshot (PC-8) ---

func (r *PeriodRepo) SnapshotSummaries(ctx context.Context, tx pgx.Tx, p svc.SnapshotParams) (int64, error) {
	return r.q.WithTx(tx).SnapshotPeriodSummaries(ctx, sqlcgen.SnapshotPeriodSummariesParams{
		Year:     int32(p.Year),
		Month:    int32(p.Month),
		PeriodID: p.PeriodID,
	})
}

func (r *PeriodRepo) VoidSummaries(ctx context.Context, tx pgx.Tx, periodID string) (int64, error) {
	return r.q.WithTx(tx).VoidPeriodSummaries(ctx, periodID)
}

// IsMonthLocked reports whether the PayrollPeriod for (year, month) is LOCKED
// (PC-11 guard, consumed by the attendance reconcile service). A never-created month
// is not locked (false).
func (r *PeriodRepo) IsMonthLocked(ctx context.Context, year, month int) (bool, error) {
	period, found, err := r.GetPeriod(ctx, year, month)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	return period.IsLocked(), nil
}

// --- completeness / blockers / summaries ---

func (r *PeriodRepo) ListCompleteness(ctx context.Context, year, month int, cursorID *string, limit int) ([]dom.EmployeeCompleteness, error) {
	rows, err := r.q.ListPeriodCompleteness(ctx, sqlcgen.ListPeriodCompletenessParams{
		Year:      int32(year),
		Month:     int32(month),
		CursorID:  cursorID,
		PageLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]dom.EmployeeCompleteness, 0, len(rows))
	for _, row := range rows {
		name := row.EmployeeName
		out = append(out, dom.EmployeeCompleteness{
			EmployeeID:         row.EmployeeID,
			EmployeeName:       &name,
			EmployeeType:       row.EmployeeType,
			Expected:           int(row.Expected),
			Recorded:           int(row.Recorded),
			Clean:              int(row.Clean),
			OnLeave:            int(row.OnLeave),
			AttendanceBlockers: int(row.AttendanceBlockers),
			PendingOT:          int(row.PendingOt),
			PendingLeave:       int(row.PendingLeave),
		})
	}
	return out, nil
}

func (r *PeriodRepo) CountFieldBlockers(ctx context.Context, year, month int) (dom.PeriodBlockerCounts, error) {
	row, err := r.q.CountFieldBlockers(ctx, sqlcgen.CountFieldBlockersParams{
		Year: int32(year), Month: int32(month),
	})
	if err != nil {
		return dom.PeriodBlockerCounts{}, err
	}
	return dom.PeriodBlockerCounts{
		Attendance: int(row.AttendanceBlockers),
		Overtime:   int(row.OvertimeBlockers),
		Leave:      int(row.LeaveBlockers),
	}, nil
}

func (r *PeriodRepo) ListSummaries(ctx context.Context, periodID string, cursorID *string, limit int) ([]dom.PeriodEmployeeSummary, error) {
	rows, err := r.q.ListPeriodSummaries(ctx, sqlcgen.ListPeriodSummariesParams{
		PeriodID:  periodID,
		CursorID:  cursorID,
		PageLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]dom.PeriodEmployeeSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, dom.PeriodEmployeeSummary{
			ID:                 row.ID,
			PeriodID:           row.PeriodID,
			EmployeeID:         row.EmployeeID,
			EmployeeName:       row.EmployeeName,
			PayableDays:        int(row.PayableDays),
			PresentDays:        int(row.PresentDays),
			LateDays:           int(row.LateDays),
			AbsentDays:         int(row.AbsentDays),
			NoShiftPayableDays: int(row.NoShiftPayableDays),
			WorkedMinutes:      int(row.WorkedMinutes),
			ApprovedOTMinutes:  int(row.ApprovedOtMinutes),
			PaidLeaveDays:      int(row.PaidLeaveDays),
			UnpaidLeaveDays:    int(row.UnpaidLeaveDays),
			CreatedAt:          row.CreatedAt,
		})
	}
	return out, nil
}
