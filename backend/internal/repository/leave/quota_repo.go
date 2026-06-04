// Package leave (repository) — QuotaRepo implements svc.QuotaRepository over the
// 08-01 sqlc leave_quotas queries. remaining = total-used-pending is derived in the
// domain; pending is recomputed on read via CountPendingLeaveDaysForQuota.
package leave

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// QuotaRepo is the sqlc-backed implementation of svc.QuotaRepository.
type QuotaRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.QuotaRepository = (*QuotaRepo)(nil)

// NewQuotaRepo returns a QuotaRepo backed by pool.
func NewQuotaRepo(pool *db.Pool) *QuotaRepo {
	return &QuotaRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func intPtr32(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

// --- list / get ---

func (r *QuotaRepo) ListLeaveQuotas(ctx context.Context, f svc.QuotaFilter) ([]dom.LeaveQuota, error) {
	var inc *bool
	if f.IncludeClosed {
		t := true
		inc = &t
	}
	rows, err := r.q.ListLeaveQuotas(ctx, sqlcgen.ListLeaveQuotasParams{
		EmployeeID:      strptr(f.EmployeeID),
		LeaveTypeID:     strptr(f.LeaveTypeID),
		Period:          intPtr32(f.Period),
		IncludeClosed:   inc,
		CompanyID:       strptr(f.CompanyID),
		CursorCreatedAt: f.CursorCreated,
		CursorID:        f.CursorID,
		Lim:             i32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]dom.LeaveQuota, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapQuotaFromList(row))
	}
	return out, nil
}

func (r *QuotaRepo) GetLeaveQuota(ctx context.Context, id string) (dom.LeaveQuota, error) {
	row, err := r.q.GetLeaveQuota(ctx, id)
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromGet(row), nil
}

func (r *QuotaRepo) GetLeaveQuotaForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.LeaveQuota, error) {
	row, err := r.q.WithTx(tx).GetLeaveQuotaForUpdate(ctx, id)
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

func (r *QuotaRepo) FindQuotaForEmployeeTypePeriod(ctx context.Context, employeeID, leaveTypeID string, period int) (dom.LeaveQuota, error) {
	row, err := r.q.FindQuotaForEmployeeTypePeriod(ctx, sqlcgen.FindQuotaForEmployeeTypePeriodParams{
		EmployeeID:  employeeID,
		LeaveTypeID: leaveTypeID,
		Period:      i32(period),
	})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

// --- mutations ---

func (r *QuotaRepo) UpsertLeaveQuota(ctx context.Context, tx pgx.Tx, p svc.UpsertQuotaParams) (dom.LeaveQuota, error) {
	row, err := r.q.WithTx(tx).UpsertLeaveQuota(ctx, sqlcgen.UpsertLeaveQuotaParams{
		EmployeeID:    p.EmployeeID,
		LeaveTypeID:   p.LeaveTypeID,
		Period:        i32(p.Period),
		PeriodStart:   timeToPgDate(p.PeriodStart),
		PeriodEnd:     timeToPgDate(p.PeriodEnd),
		Total:         i32(p.Total),
		IsProrated:    p.IsProrated,
		ProrateMonths: i32(p.ProrateMonths),
	})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

func (r *QuotaRepo) AdjustLeaveQuotaTotal(ctx context.Context, tx pgx.Tx, id string, delta int, adj dom.LeaveQuotaAdjustment) (dom.LeaveQuota, error) {
	raw, err := marshalAdjustment(adj)
	if err != nil {
		return dom.LeaveQuota{}, err
	}
	row, err := r.q.WithTx(tx).AdjustLeaveQuotaTotal(ctx, sqlcgen.AdjustLeaveQuotaTotalParams{
		Delta:          i32(delta),
		LastAdjustment: raw,
		ID:             id,
	})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

func (r *QuotaRepo) DeductLeaveQuota(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error) {
	row, err := r.q.WithTx(tx).DeductLeaveQuota(ctx, sqlcgen.DeductLeaveQuotaParams{Delta: i32(delta), ID: id})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

func (r *QuotaRepo) RestoreLeaveQuota(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error) {
	row, err := r.q.WithTx(tx).RestoreLeaveQuota(ctx, sqlcgen.RestoreLeaveQuotaParams{Delta: i32(delta), ID: id})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

func (r *QuotaRepo) SetLeaveQuotaOverride(ctx context.Context, tx pgx.Tx, id string, ov dom.LeaveQuotaOverride) (dom.LeaveQuota, error) {
	raw, err := marshalOverride(ov)
	if err != nil {
		return dom.LeaveQuota{}, err
	}
	row, err := r.q.WithTx(tx).SetLeaveQuotaOverride(ctx, sqlcgen.SetLeaveQuotaOverrideParams{LastOverride: raw, ID: id})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

// --- recompute / bulk-grant sources ---

func (r *QuotaRepo) CountPendingLeaveDaysForQuota(ctx context.Context, employeeID, leaveTypeID string, periodStart, periodEnd time.Time) (int, error) {
	n, err := r.q.CountPendingLeaveDaysForQuota(ctx, sqlcgen.CountPendingLeaveDaysForQuotaParams{
		EmployeeID:  employeeID,
		LeaveTypeID: leaveTypeID,
		PeriodStart: timeToPgDate(periodStart),
		PeriodEnd:   timeToPgDate(periodEnd),
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *QuotaRepo) ListActivePlacedEmployeesForGrant(ctx context.Context, periodStart, periodEnd time.Time) ([]svc.GrantCandidate, error) {
	rows, err := r.q.ListActivePlacedEmployeesForGrant(ctx, sqlcgen.ListActivePlacedEmployeesForGrantParams{
		PeriodStart: timeToPgDate(periodStart),
		PeriodEnd:   timeToPgDate(periodEnd),
	})
	if err != nil {
		return nil, err
	}
	out := make([]svc.GrantCandidate, 0, len(rows))
	for _, row := range rows {
		name := row.EmployeeName
		out = append(out, svc.GrantCandidate{
			EmployeeID:     row.EmployeeID,
			EmployeeName:   &name,
			PlacementStart: pgDateToTime(row.PlacementStartDate),
		})
	}
	return out, nil
}
