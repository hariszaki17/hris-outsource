// Package payroll (repository) — ExportRepo implements svc.ExportRepository over the
// 10-01 sqlc export_jobs queries. The QUEUED insert runs via q.WithTx(tx) (same tx
// as the River EnqueueTx — transactional outbox); the worker reuses GetExportJob /
// UpdateExportJobStatus (the latter via NewQueries on its own pool, not this repo).
package payroll

import (
	"context"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// ExportRepo is the sqlc-backed implementation of svc.ExportRepository.
type ExportRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.ExportRepository = (*ExportRepo)(nil)

// NewExportRepo returns an ExportRepo backed by pool.
func NewExportRepo(pool *db.Pool) *ExportRepo {
	return &ExportRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// CountPayslipsInScope counts the rows an export would emit (EXPORT_TOO_LARGE guard).
func (r *ExportRepo) CountPayslipsInScope(ctx context.Context, year, month *int, employeeIDs []string) (int, error) {
	n, err := r.q.CountPayslipsInScope(ctx, sqlcgen.CountPayslipsInScopeParams{
		Year:        i32ptr(year),
		Month:       i32ptr(month),
		EmployeeIds: employeeIDs,
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// InsertExportJob writes the QUEUED row inside the export tx (transactional outbox).
func (r *ExportRepo) InsertExportJob(ctx context.Context, tx pgx.Tx, p svc.ExportJobParams) (dom.ExportJob, error) {
	row, err := r.q.WithTx(tx).InsertExportJob(ctx, sqlcgen.InsertExportJobParams{
		RequestedByID:    p.RequestedByID,
		RequestedByName:  p.RequestedByName,
		ScopePeriod:      p.ScopePeriod,
		ScopeYear:        i32ptr(p.ScopeYear),
		ScopeEmployeeIds: p.ScopeEmployeeIDs,
	})
	if err != nil {
		return dom.ExportJob{}, mapErr(err)
	}
	return mapExportJob(row), nil
}

func (r *ExportRepo) GetExportJob(ctx context.Context, id string) (dom.ExportJob, error) {
	row, err := r.q.GetExportJob(ctx, id)
	if err != nil {
		return dom.ExportJob{}, mapErr(err)
	}
	return mapExportJob(row), nil
}
