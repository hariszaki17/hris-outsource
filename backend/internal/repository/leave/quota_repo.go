// Package leave (repository) — QuotaRepo implements svc.QuotaRepository over the
// 08-01 sqlc leave_quotas queries. remaining = total-used-pending is derived in the
// domain; pending is recomputed on read via CountPendingLeaveDaysForQuota.
package leave

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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

// Per-type ledger (2026-06-12): QuotaRepo also drives the QuotaMeter.
var (
	_ svc.QuotaMeterStore  = (*QuotaRepo)(nil)
	_ svc.QuotaMeterReader = (*QuotaRepo)(nil)
)

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

// --- per-type ledger (2026-06-12, EPICS §8) — reserve/commit/release lifecycle ---
// These are the new live-path primitives (wired by the QuotaMeter service in
// Phase 3/4). Window-mutating methods take a tx so the service can row-lock the
// window (ResolveQuotaWindow ... FOR UPDATE) and mutate atomically.

func i32ptrToIntPtr(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

// GetLeaveTypeCap reads a leave type's cap mechanics (QuotaMeterReader).
func (r *QuotaRepo) GetLeaveTypeCap(ctx context.Context, leaveTypeID string) (dom.LeaveTypeCap, error) {
	row, err := r.q.GetLeaveTypeCap(ctx, leaveTypeID)
	if err != nil {
		return dom.LeaveTypeCap{}, mapErr(err)
	}
	return dom.LeaveTypeCap{
		ID: row.ID, Code: row.Code, Name: row.Name,
		CapBasis:         dom.LeaveTypeCapBasis(row.CapBasis),
		CapValue:         i32ptrToIntPtr(row.CapValue),
		CapUnit:          row.CapUnit,
		Paid:             row.Paid,
		Gender:           row.Gender,
		RequiresDocument: row.RequiresDocument,
		NoticeDays:       int(row.NoticeDays),
		MinServiceYears:  int(row.MinServiceYears),
		LeadDays:         int(row.LeadDays),
		TrailDays:        int(row.TrailDays),
	}, nil
}

// GetEmployeeGateInfo reads gender + join date for the gates (QuotaMeterReader).
func (r *QuotaRepo) GetEmployeeGateInfo(ctx context.Context, employeeID string) (dom.EmployeeGateInfo, error) {
	row, err := r.q.GetEmployeeGateInfo(ctx, employeeID)
	if err != nil {
		return dom.EmployeeGateInfo{}, mapErr(err)
	}
	return dom.EmployeeGateInfo{Gender: row.Gender, JoinAt: pgDateToTime(row.JoinAt)}, nil
}

// GetAnnualEntitlement reads the active agreement's annual entitlement (QuotaMeterReader).
func (r *QuotaRepo) GetAnnualEntitlement(ctx context.Context, employeeID string) (*int, error) {
	v, err := r.q.GetAnnualEntitlementForEmployee(ctx, employeeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapErr(err)
	}
	return i32ptrToIntPtr(v), nil
}

func pgDateFromPtr(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// ResolveQuotaWindow row-locks the (employee, leave_type, period_key) window.
func (r *QuotaRepo) ResolveQuotaWindow(ctx context.Context, tx pgx.Tx, employeeID, leaveTypeID, periodKey string) (dom.LeaveQuota, error) {
	row, err := r.q.WithTx(tx).ResolveQuotaWindow(ctx, sqlcgen.ResolveQuotaWindowParams{
		EmployeeID: employeeID, LeaveTypeID: leaveTypeID, PeriodKey: &periodKey,
	})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

// OpenQuotaWindow auto-opens (or upserts entitlement of) a per-type window.
func (r *QuotaRepo) OpenQuotaWindow(ctx context.Context, tx pgx.Tx, s dom.QuotaWindowSpec) (dom.LeaveQuota, error) {
	pk := s.PeriodKey
	row, err := r.q.WithTx(tx).OpenQuotaWindow(ctx, sqlcgen.OpenQuotaWindowParams{
		EmployeeID:   s.EmployeeID,
		LeaveTypeID:  s.LeaveTypeID,
		PeriodKey:    &pk,
		Period:       i32(s.Period),
		PeriodStart:  timeToPgDate(s.PeriodStart),
		PeriodEnd:    timeToPgDate(s.PeriodEnd),
		EntitledDays: i32(s.EntitledDays),
		Source:       string(s.Source),
		Remark:       s.Remark,
		ExpiresAt:    pgDateFromPtr(s.ExpiresAt),
		CreatedBy:    s.CreatedBy,
	})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

// ReserveQuotaDays holds pending_days on the window (submit).
func (r *QuotaRepo) ReserveQuotaDays(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error) {
	row, err := r.q.WithTx(tx).ReserveQuotaDays(ctx, sqlcgen.ReserveQuotaDaysParams{Delta: i32(delta), ID: id})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

// CommitQuotaDays moves pending_days -> used_days (final approval).
func (r *QuotaRepo) CommitQuotaDays(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error) {
	row, err := r.q.WithTx(tx).CommitQuotaDays(ctx, sqlcgen.CommitQuotaDaysParams{Delta: i32(delta), ID: id})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

// ReleaseQuotaDays releases held pending_days (reject/withdraw).
func (r *QuotaRepo) ReleaseQuotaDays(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error) {
	row, err := r.q.WithTx(tx).ReleaseQuotaDays(ctx, sqlcgen.ReleaseQuotaDaysParams{Delta: i32(delta), ID: id})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

// ReverseCommittedQuotaDays returns committed used_days to the balance (cancel/shorten).
func (r *QuotaRepo) ReverseCommittedQuotaDays(ctx context.Context, tx pgx.Tx, id string, delta int) (dom.LeaveQuota, error) {
	row, err := r.q.WithTx(tx).ReverseCommittedQuotaDays(ctx, sqlcgen.ReverseCommittedQuotaDaysParams{Delta: i32(delta), ID: id})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

// AdjustQuotaEntitled applies an audited signed delta on entitled_days (HR LQ-6).
func (r *QuotaRepo) AdjustQuotaEntitled(ctx context.Context, tx pgx.Tx, id string, delta int, remark string, adj dom.LeaveQuotaAdjustment) (dom.LeaveQuota, error) {
	raw, err := marshalAdjustment(adj)
	if err != nil {
		return dom.LeaveQuota{}, err
	}
	row, err := r.q.WithTx(tx).AdjustQuotaEntitled(ctx, sqlcgen.AdjustQuotaEntitledParams{
		Delta: i32(delta), Remark: remark, LastAdjustment: raw, ID: id,
	})
	if err != nil {
		return dom.LeaveQuota{}, mapErr(err)
	}
	return mapQuotaFromModel(row), nil
}

// CountApprovedRequestsForType counts the employee's non-rejected requests of a
// type whose start_date falls in [from,to] — the PER_YEAR_COUNT / LIFETIME_ONCE gate.
func (r *QuotaRepo) CountApprovedRequestsForType(ctx context.Context, employeeID, leaveTypeID string, from, to time.Time) (int, error) {
	n, err := r.q.CountApprovedRequestsForType(ctx, sqlcgen.CountApprovedRequestsForTypeParams{
		EmployeeID:  employeeID,
		LeaveTypeID: leaveTypeID,
		WindowStart: timeToPgDate(from),
		WindowEnd:   timeToPgDate(to),
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
