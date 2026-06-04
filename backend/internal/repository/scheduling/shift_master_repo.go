// Package scheduling (repository) — ShiftMasterRepo implements the scheduling
// shift-master service port over the 06-01 sqlc queries. Reads on the pool;
// writes via q.WithTx(tx). pgx.ErrNoRows → domain.ErrNotFound. Name-uniqueness
// 23505 propagates raw so the service maps it to DUPLICATE_NAME.
package scheduling

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// ShiftMasterRepo is the sqlc-backed implementation of svc.ShiftMasterRepository.
type ShiftMasterRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.ShiftMasterRepository = (*ShiftMasterRepo)(nil)

// NewShiftMasterRepo returns a ShiftMasterRepo backed by pool.
func NewShiftMasterRepo(pool *db.Pool) *ShiftMasterRepo {
	return &ShiftMasterRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

func (r *ShiftMasterRepo) ListShiftMasters(ctx context.Context, f domain.ShiftMasterFilter) ([]domain.ShiftMaster, error) {
	var isActive *bool
	if f.Status != nil {
		switch *f.Status {
		case "ACTIVE":
			t := true
			isActive = &t
		case "INACTIVE":
			t := false
			isActive = &t
		}
	}
	rows, err := r.q.ListShiftMasters(ctx, sqlcgen.ListShiftMastersParams{
		ServiceLineID: f.ServiceLineID,
		IsActive:      isActive,
		Q:             f.Q,
		CursorID:      f.Cursor,
		RowLimit:      f.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ShiftMaster, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapShiftMasterFromList(row))
	}
	return out, nil
}

func (r *ShiftMasterRepo) GetShiftMaster(ctx context.Context, id string) (domain.ShiftMaster, error) {
	row, err := r.q.GetShiftMaster(ctx, id)
	if err != nil {
		return domain.ShiftMaster{}, mapErr(err)
	}
	return mapShiftMasterFromGet(row), nil
}

func (r *ShiftMasterRepo) GetShiftMasterForUpdate(ctx context.Context, tx pgx.Tx, id string) (domain.ShiftMaster, error) {
	row, err := r.q.WithTx(tx).GetShiftMasterForUpdate(ctx, id)
	if err != nil {
		return domain.ShiftMaster{}, mapErr(err)
	}
	return mapShiftMasterFromForUpdate(row), nil
}

func (r *ShiftMasterRepo) CreateShiftMaster(ctx context.Context, tx pgx.Tx, p svc.CreateShiftMasterParams) (domain.ShiftMaster, error) {
	row, err := r.q.WithTx(tx).CreateShiftMaster(ctx, sqlcgen.CreateShiftMasterParams{
		Name:          p.Name,
		StartTime:     p.StartTime,
		EndTime:       p.EndTime,
		BreakStart:    p.BreakStart,
		BreakEnd:      p.BreakEnd,
		ServiceLineID: p.ServiceLineID,
		CrossMidnight: p.CrossMidnight,
		IsActive:      p.IsActive,
		CreatedBy:     p.CreatedBy,
	})
	if err != nil {
		return domain.ShiftMaster{}, err
	}
	return mapShiftMasterFromCreate(row), nil
}

func (r *ShiftMasterRepo) UpdateShiftMaster(ctx context.Context, tx pgx.Tx, p svc.UpdateShiftMasterParams) (domain.ShiftMaster, error) {
	row, err := r.q.WithTx(tx).UpdateShiftMaster(ctx, sqlcgen.UpdateShiftMasterParams{
		Name:          p.Name,
		StartTime:     p.StartTime,
		EndTime:       p.EndTime,
		BreakStart:    p.BreakStart,
		BreakEnd:      p.BreakEnd,
		ServiceLineID: p.ServiceLineID,
		CrossMidnight: p.CrossMidnight,
		IsActive:      p.IsActive,
		ID:            p.ID,
	})
	if err != nil {
		return domain.ShiftMaster{}, err
	}
	return mapShiftMasterFromUpdate(row), nil
}

func (r *ShiftMasterRepo) SetShiftMasterActive(ctx context.Context, tx pgx.Tx, id string, active bool) (domain.ShiftMaster, error) {
	row, err := r.q.WithTx(tx).SetShiftMasterActive(ctx, sqlcgen.SetShiftMasterActiveParams{
		IsActive: active,
		ID:       id,
	})
	if err != nil {
		return domain.ShiftMaster{}, mapErr(err)
	}
	return mapShiftMasterFromSetActive(row), nil
}
