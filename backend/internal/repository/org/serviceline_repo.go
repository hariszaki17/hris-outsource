// Package org (repository) — ServiceLineRepository over sqlc-generated queries.
// Mirrors companies_repo.go exactly: reads on pool, writes via q.WithTx(tx),
// pgx.ErrNoRows → domain.ErrNotFound. mapErr and nullStr are declared in
// companies_repo.go (same package) — do NOT redeclare them here.
package org

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svcsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/org"
)

// ServiceLineRepository is the sqlc-backed implementation of
// svcsvc.ServiceLineRepository. Provides service-line and position reads/writes.
type ServiceLineRepository struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// compile-time check: ServiceLineRepository satisfies the service port.
var _ svcsvc.ServiceLineRepository = (*ServiceLineRepository)(nil)

// NewServiceLineRepo returns a new ServiceLineRepository backed by pool.
func NewServiceLineRepo(pool *db.Pool) *ServiceLineRepository {
	return &ServiceLineRepository{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// --- Service Lines ---

// ListServiceLines returns a page of service lines matching the filter.
// PositionCount is populated via CountActivePositionsForLine for each row.
func (r *ServiceLineRepository) ListServiceLines(ctx context.Context, f domain.ServiceLineFilter) ([]domain.ServiceLine, error) {
	rows, err := r.q.ListServiceLines(ctx, sqlcgen.ListServiceLinesParams{
		Status:          f.Status,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}

	out := make([]domain.ServiceLine, 0, len(rows))
	for _, row := range rows {
		posCount, err := r.q.CountActivePositionsForLine(ctx, row.ID)
		if err != nil {
			posCount = 0
		}
		out = append(out, domain.ServiceLine{
			ID:            row.ID,
			Name:          row.Name,
			Status:        row.Status,
			PositionCount: int(posCount),
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		})
	}
	return out, nil
}

// GetServiceLineByID fetches a single service line by SWP-SVC id.
func (r *ServiceLineRepository) GetServiceLineByID(ctx context.Context, id string) (domain.ServiceLine, error) {
	row, err := r.q.GetServiceLineByID(ctx, id)
	if err != nil {
		return domain.ServiceLine{}, mapErr(err)
	}
	posCount, err := r.q.CountActivePositionsForLine(ctx, id)
	if err != nil {
		posCount = 0
	}
	return domain.ServiceLine{
		ID:            row.ID,
		Name:          row.Name,
		Status:        row.Status,
		PositionCount: int(posCount),
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// CreateServiceLine inserts a new service line in the given transaction.
func (r *ServiceLineRepository) CreateServiceLine(ctx context.Context, tx pgx.Tx, name string) (domain.ServiceLine, error) {
	row, err := r.q.WithTx(tx).CreateServiceLine(ctx, name)
	if err != nil {
		return domain.ServiceLine{}, mapErr(err)
	}
	return domain.ServiceLine{
		ID:            row.ID,
		Name:          row.Name,
		Status:        row.Status,
		PositionCount: 0, // newly created; no positions yet
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// UpdateServiceLine renames a service line in the given transaction.
func (r *ServiceLineRepository) UpdateServiceLine(ctx context.Context, tx pgx.Tx, id, name string) (domain.ServiceLine, error) {
	row, err := r.q.WithTx(tx).UpdateServiceLine(ctx, sqlcgen.UpdateServiceLineParams{
		ID:   id,
		Name: name,
	})
	if err != nil {
		return domain.ServiceLine{}, mapErr(err)
	}
	posCount, err := r.q.CountActivePositionsForLine(ctx, id)
	if err != nil {
		posCount = 0
	}
	return domain.ServiceLine{
		ID:            row.ID,
		Name:          row.Name,
		Status:        row.Status,
		PositionCount: int(posCount),
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// SetServiceLineStatus updates the status of a service line (active/inactive).
func (r *ServiceLineRepository) SetServiceLineStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.ServiceLine, error) {
	row, err := r.q.WithTx(tx).SetServiceLineStatus(ctx, sqlcgen.SetServiceLineStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return domain.ServiceLine{}, mapErr(err)
	}
	posCount, err := r.q.CountActivePositionsForLine(ctx, id)
	if err != nil {
		posCount = 0
	}
	return domain.ServiceLine{
		ID:            row.ID,
		Name:          row.Name,
		Status:        row.Status,
		PositionCount: int(posCount),
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// CountActivePositionsForLine returns the number of active positions for a line.
func (r *ServiceLineRepository) CountActivePositionsForLine(ctx context.Context, lineID string) (int64, error) {
	return r.q.CountActivePositionsForLine(ctx, lineID)
}

// --- Positions ---

// ListPositionsForLine returns a page of positions under a service line.
func (r *ServiceLineRepository) ListPositionsForLine(ctx context.Context, lineID string, f domain.PositionFilter) ([]domain.Position, error) {
	rows, err := r.q.ListPositionsForLine(ctx, sqlcgen.ListPositionsForLineParams{
		ServiceLineID:   lineID,
		Status:          f.Status,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Position, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPosition(row.ID, row.ServiceLineID, row.Name, row.Alias, row.Status, row.CreatedAt, row.UpdatedAt))
	}
	return out, nil
}

// GetPositionByID fetches a single position by SWP-POS id.
func (r *ServiceLineRepository) GetPositionByID(ctx context.Context, id string) (domain.Position, error) {
	row, err := r.q.GetPositionByID(ctx, id)
	if err != nil {
		return domain.Position{}, mapErr(err)
	}
	return toPosition(row.ID, row.ServiceLineID, row.Name, row.Alias, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// CreatePosition inserts a new position in the given transaction.
func (r *ServiceLineRepository) CreatePosition(ctx context.Context, tx pgx.Tx, p svcsvc.CreatePositionParams) (domain.Position, error) {
	row, err := r.q.WithTx(tx).CreatePosition(ctx, sqlcgen.CreatePositionParams{
		ServiceLineID: p.ServiceLineID,
		Name:          p.Name,
		Alias:         p.Alias,
	})
	if err != nil {
		return domain.Position{}, mapErr(err)
	}
	return toPosition(row.ID, row.ServiceLineID, row.Name, row.Alias, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// UpdatePosition patches a position's mutable fields (name/alias) in the given tx.
func (r *ServiceLineRepository) UpdatePosition(ctx context.Context, tx pgx.Tx, p svcsvc.UpdatePositionParams) (domain.Position, error) {
	row, err := r.q.WithTx(tx).UpdatePosition(ctx, sqlcgen.UpdatePositionParams{
		ID:    p.ID,
		Name:  p.Name,
		Alias: p.Alias,
	})
	if err != nil {
		return domain.Position{}, mapErr(err)
	}
	return toPosition(row.ID, row.ServiceLineID, row.Name, row.Alias, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// SetPositionStatus updates the status of a position (active/inactive).
func (r *ServiceLineRepository) SetPositionStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.Position, error) {
	row, err := r.q.WithTx(tx).SetPositionStatus(ctx, sqlcgen.SetPositionStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return domain.Position{}, mapErr(err)
	}
	return toPosition(row.ID, row.ServiceLineID, row.Name, row.Alias, row.Status, row.CreatedAt, row.UpdatedAt), nil
}

// SoftDeletePosition marks a position as deleted (sets deleted_at).
func (r *ServiceLineRepository) SoftDeletePosition(ctx context.Context, tx pgx.Tx, id string) error {
	return r.q.WithTx(tx).SoftDeletePosition(ctx, id)
}

// --- mapping helpers ---

func toPosition(id, serviceLineID, name, alias, status string, createdAt, updatedAt time.Time) domain.Position {
	return domain.Position{
		ID:            id,
		ServiceLineID: serviceLineID,
		Name:          name,
		Alias:         alias,
		Status:        status,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
}
