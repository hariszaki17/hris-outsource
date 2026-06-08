// Package leave (repository) — GrantRepo implements svc.GrantRepository over the
// leave_grants / leave_consumptions sqlc queries (F6.1 grant-lot ledger). Reads on the
// pool; locked allocation + writes via q.WithTx(tx). Dates convert pgtype.Date ↔
// time.Time; remaining = amount-consumed-pending is derived in the domain.
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

// GrantRepo is the sqlc-backed implementation of svc.GrantRepository.
type GrantRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.GrantRepository = (*GrantRepo)(nil)

// NewGrantRepo returns a GrantRepo backed by pool.
func NewGrantRepo(pool *db.Pool) *GrantRepo {
	return &GrantRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// --- create / patch ---

func (r *GrantRepo) CreateLeaveGrant(ctx context.Context, tx pgx.Tx, p svc.CreateGrantParams) (dom.LeaveGrant, error) {
	row, err := r.q.WithTx(tx).CreateLeaveGrant(ctx, sqlcgen.CreateLeaveGrantParams{
		EmployeeID:    p.EmployeeID,
		AmountDays:    i32(p.Amount),
		EffectiveFrom: timeToPgDate(p.EffectiveFrom),
		ExpiresAt:     timeToPgDate(p.ExpiresAt),
		Source:        string(p.Source),
		Earmark:       p.Earmark,
		Remark:        p.Remark,
		CreatedBy:     p.CreatedBy,
	})
	if err != nil {
		return dom.LeaveGrant{}, mapErr(err)
	}
	return grantFromCreate(row), nil
}

func (r *GrantRepo) PatchLeaveGrant(ctx context.Context, tx pgx.Tx, p svc.PatchGrantParams) (dom.LeaveGrant, error) {
	remark := p.Remark
	params := sqlcgen.PatchLeaveGrantParams{
		ID:         p.ID,
		SetEarmark: p.SetEarmark,
		Earmark:    p.Earmark,
		Remark:     &remark,
	}
	if p.Amount != nil {
		params.AmountDays = i32ptr(p.Amount)
	}
	if p.ExpiresAt != nil {
		params.ExpiresAt = timeToPgDate(*p.ExpiresAt)
	}
	row, err := r.q.WithTx(tx).PatchLeaveGrant(ctx, params)
	if err != nil {
		return dom.LeaveGrant{}, mapErr(err)
	}
	return grantFromPatch(row), nil
}

// --- get / list ---

func (r *GrantRepo) GetLeaveGrant(ctx context.Context, id string) (dom.LeaveGrant, error) {
	row, err := r.q.GetLeaveGrant(ctx, id)
	if err != nil {
		return dom.LeaveGrant{}, mapErr(err)
	}
	return grantFromGet(row), nil
}

func (r *GrantRepo) GetLeaveGrantForUpdate(ctx context.Context, tx pgx.Tx, id string) (dom.LeaveGrant, error) {
	row, err := r.q.WithTx(tx).GetLeaveGrantForUpdate(ctx, id)
	if err != nil {
		return dom.LeaveGrant{}, mapErr(err)
	}
	return grantFromForUpdate(row), nil
}

func (r *GrantRepo) ListLeaveGrants(ctx context.Context, f svc.GrantFilter, now time.Time) ([]dom.LeaveGrant, error) {
	var inc *bool
	if f.IncludeExpired {
		t := true
		inc = &t
	}
	p := sqlcgen.ListLeaveGrantsParams{
		EmployeeID:     strptr(f.EmployeeID),
		Source:         strptr(f.Source),
		EarmarkFilter:  strptr(f.Earmark),
		IncludeExpired: inc,
		NowDate:        timeToPgDate(now),
		CompanyID:      strptr(f.CompanyID),
		Lim:            i32(f.Limit),
	}
	if f.CursorExpires != nil {
		p.CursorExpiresAt = timeToPgDate(*f.CursorExpires)
		p.CursorID = f.CursorID
	}
	rows, err := r.q.ListLeaveGrants(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]dom.LeaveGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, grantFromList(row))
	}
	return out, nil
}

// ListLeaveBalances returns the per-employee aggregate balance page (one row per
// employee over active lots) for GET /leave-balances. Keyset on (full_name, id).
func (r *GrantRepo) ListLeaveBalances(ctx context.Context, f svc.BalanceListFilter, now time.Time) ([]dom.EmployeeLeaveBalance, error) {
	p := sqlcgen.ListLeaveBalancesParams{
		NowDate:        timeToPgDate(now),
		Q:              strptr(f.Q),
		CursorFullName: strptr(f.CursorFullName),
		CursorID:       strptr(f.CursorID),
		Lim:            i32(f.Limit),
	}
	rows, err := r.q.ListLeaveBalances(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]dom.EmployeeLeaveBalance, 0, len(rows))
	for _, row := range rows {
		b := dom.EmployeeLeaveBalance{
			EmployeeID:         row.EmployeeID,
			FullName:           row.FullName,
			NIK:                row.Nik,
			NIP:                row.Nip,
			PoolTotal:          int(row.PoolTotal),
			PoolConsumed:       int(row.PoolConsumed),
			PoolPending:        int(row.PoolPending),
			PoolRemaining:      int(row.PoolRemaining),
			EarmarkedRemaining: int(row.EarmarkedRemaining),
			LotCount:           int(row.LotCount),
		}
		if row.NextExpiry.Valid {
			t := row.NextExpiry.Time
			b.NextExpiry = &t
		}
		out = append(out, b)
	}
	return out, nil
}

// --- consumptions ---

func (r *GrantRepo) ListConsumptionsForGrant(ctx context.Context, grantID string) ([]dom.LeaveConsumption, error) {
	rows, err := r.q.ListConsumptionsForGrant(ctx, grantID)
	if err != nil {
		return nil, err
	}
	return consumptions(rows), nil
}

func (r *GrantRepo) ListConsumptionsForRequest(ctx context.Context, requestID string) ([]dom.LeaveConsumption, error) {
	rows, err := r.q.ListConsumptionsForRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return consumptions(rows), nil
}

func (r *GrantRepo) ApplyConsumption(ctx context.Context, tx pgx.Tx, requestID, grantID string, days int) (dom.LeaveConsumption, error) {
	row, err := r.q.WithTx(tx).ApplyConsumption(ctx, sqlcgen.ApplyConsumptionParams{
		LeaveRequestID: requestID,
		GrantID:        grantID,
		Days:           i32(days),
	})
	if err != nil {
		return dom.LeaveConsumption{}, mapErr(err)
	}
	return consumption(row), nil
}

func (r *GrantRepo) DeleteConsumptionsForRequest(ctx context.Context, tx pgx.Tx, requestID string) error {
	return r.q.WithTx(tx).DeleteConsumptionsForRequest(ctx, requestID)
}

// --- allocation lifecycle ---

func (r *GrantRepo) GetActiveLotsForAllocation(ctx context.Context, tx pgx.Tx, employeeID, earmarkMatch string, now time.Time) ([]dom.LeaveGrant, error) {
	rows, err := r.q.WithTx(tx).GetActiveLotsForAllocation(ctx, sqlcgen.GetActiveLotsForAllocationParams{
		EmployeeID:   employeeID,
		NowDate:      timeToPgDate(now),
		EarmarkMatch: earmarkMatch,
	})
	if err != nil {
		return nil, err
	}
	out := make([]dom.LeaveGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, grantFromAlloc(row))
	}
	return out, nil
}

func (r *GrantRepo) ReservePending(ctx context.Context, tx pgx.Tx, grantID string, days int) error {
	_, err := r.q.WithTx(tx).ReservePending(ctx, sqlcgen.ReservePendingParams{Days: i32(days), ID: grantID})
	return mapErr(err)
}

func (r *GrantRepo) CommitReservation(ctx context.Context, tx pgx.Tx, grantID string, days int) error {
	_, err := r.q.WithTx(tx).CommitReservation(ctx, sqlcgen.CommitReservationParams{Days: i32(days), ID: grantID})
	return mapErr(err)
}

func (r *GrantRepo) ReleasePending(ctx context.Context, tx pgx.Tx, grantID string, days int) error {
	_, err := r.q.WithTx(tx).ReleasePending(ctx, sqlcgen.ReleasePendingParams{Days: i32(days), ID: grantID})
	return mapErr(err)
}

func (r *GrantRepo) ReverseConsumption(ctx context.Context, tx pgx.Tx, grantID string, days int) error {
	_, err := r.q.WithTx(tx).ReverseConsumption(ctx, sqlcgen.ReverseConsumptionParams{Days: i32(days), ID: grantID})
	return mapErr(err)
}

// --- balance read model ---

func (r *GrantRepo) SumActiveBalanceByEarmark(ctx context.Context, employeeID string, now time.Time) ([]svc.EarmarkBalanceGroup, error) {
	rows, err := r.q.SumActiveBalanceByEarmark(ctx, sqlcgen.SumActiveBalanceByEarmarkParams{
		EmployeeID: employeeID,
		NowDate:    timeToPgDate(now),
	})
	if err != nil {
		return nil, err
	}
	out := make([]svc.EarmarkBalanceGroup, 0, len(rows))
	for _, row := range rows {
		g := svc.EarmarkBalanceGroup{
			Earmark:   row.Earmark,
			Remaining: int(row.RemainingDays),
			Pending:   int(row.PendingDays),
		}
		if row.NextExpiry.Valid {
			t := row.NextExpiry.Time
			g.NextExpiry = &t
		}
		out = append(out, g)
	}
	return out, nil
}

// --- expiry sweep ---

func (r *GrantRepo) FindExpiredLotsWithPending(ctx context.Context, today time.Time, limit int) ([]svc.ExpiredLot, error) {
	rows, err := r.q.ExpireLeaveLots(ctx, sqlcgen.ExpireLeaveLotsParams{
		Today: timeToPgDate(today),
		Lim:   i32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]svc.ExpiredLot, 0, len(rows))
	for _, row := range rows {
		out = append(out, svc.ExpiredLot{
			ID:          row.ID,
			EmployeeID:  row.EmployeeID,
			ExpiresAt:   pgDateToTime(row.ExpiresAt),
			PendingDays: int(row.PendingDays),
		})
	}
	return out, nil
}

func (r *GrantRepo) ZeroLotPending(ctx context.Context, tx pgx.Tx, grantID string) error {
	_, err := r.q.WithTx(tx).ZeroLotPending(ctx, grantID)
	return mapErr(err)
}
