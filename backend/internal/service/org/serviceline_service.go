// Service layer for E2 service lines + positions (F2.4 / ORG-03).
// Mirrors companies_service.go: separate ServiceLineService struct in the same
// org service package — parallel-merge clean, no struct reuse with Service.
// TxRunner and Clock are declared in companies_service.go (same package).
package org

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// ServiceLineRepository is the data dependency for the ServiceLineService,
// defined by this consumer (Go dependency-inversion idiom).
// The repository layer in internal/repository/org implements it over sqlc.
type ServiceLineRepository interface {
	// Service-line reads run on the pool.
	ListServiceLines(ctx context.Context, f domain.ServiceLineFilter) ([]domain.ServiceLine, error)
	GetServiceLineByID(ctx context.Context, id string) (domain.ServiceLine, error)
	CountActivePositionsForLine(ctx context.Context, lineID string) (int64, error)
	// Service-line writes take the active transaction.
	CreateServiceLine(ctx context.Context, tx pgx.Tx, name string) (domain.ServiceLine, error)
	UpdateServiceLine(ctx context.Context, tx pgx.Tx, id, name string) (domain.ServiceLine, error)
	SetServiceLineStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.ServiceLine, error)
	// Position reads run on the pool.
	ListPositionsForLine(ctx context.Context, lineID string, f domain.PositionFilter) ([]domain.Position, error)
	GetPositionByID(ctx context.Context, id string) (domain.Position, error)
	// Position writes take the active transaction.
	CreatePosition(ctx context.Context, tx pgx.Tx, p CreatePositionParams) (domain.Position, error)
	UpdatePosition(ctx context.Context, tx pgx.Tx, p UpdatePositionParams) (domain.Position, error)
	SetPositionStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.Position, error)
	SoftDeletePosition(ctx context.Context, tx pgx.Tx, id string) error
}

// CreatePositionParams carries the fields for inserting a new position.
type CreatePositionParams struct {
	ServiceLineID string
	Name          string
	Alias         string
}

// UpdatePositionParams carries the fields for updating a position.
type UpdatePositionParams struct {
	ID    string
	Name  string
	Alias string
}

// ServiceLineService implements E2 service-line + position business logic.
type ServiceLineService struct {
	repo ServiceLineRepository
	txm  TxRunner
}

// NewServiceLineService wires the service with its dependencies.
func NewServiceLineService(repo ServiceLineRepository, txm TxRunner) *ServiceLineService {
	return &ServiceLineService{repo: repo, txm: txm}
}

// --- Service Lines ---

// ListServiceLines returns a cursor-paginated page of service lines.
func (s *ServiceLineService) ListServiceLines(ctx context.Context, f domain.ServiceLineFilter) ([]domain.ServiceLine, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1 // fetch one extra to detect has_more

	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		f.Status = &lower
	}

	rows, err := s.repo.ListServiceLines(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// GetServiceLine returns a single service line by id.
func (s *ServiceLineService) GetServiceLine(ctx context.Context, id string) (domain.ServiceLine, error) {
	line, err := s.repo.GetServiceLineByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ServiceLine{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ServiceLine{}, apperr.Internal(err)
	}
	return line, nil
}

// CreateServiceLine creates a new service line (super_admin only — route-enforced).
// Unique name violation → 409 CONFLICT.
func (s *ServiceLineService) CreateServiceLine(ctx context.Context, name string) (domain.ServiceLine, error) {
	if strings.TrimSpace(name) == "" {
		return domain.ServiceLine{}, apperr.Invalid(map[string]string{"name": "Wajib diisi."})
	}

	var created domain.ServiceLine
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateServiceLine(ctx, tx, name)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "service_line",
			EntityID:   created.ID,
			Before:     nil,
			After:      map[string]any{"name": created.Name, "status": created.Status},
		})
	}); err != nil {
		return domain.ServiceLine{}, mapSLConflict(err)
	}

	return created, nil
}

// UpdateServiceLine renames a service line.
// 404 if not found; 409 CONFLICT on duplicate name.
func (s *ServiceLineService) UpdateServiceLine(ctx context.Context, id, name string) (domain.ServiceLine, error) {
	current, err := s.repo.GetServiceLineByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ServiceLine{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ServiceLine{}, apperr.Internal(err)
	}

	if strings.TrimSpace(name) == "" {
		return domain.ServiceLine{}, apperr.Invalid(map[string]string{"name": "Wajib diisi."})
	}

	var updated domain.ServiceLine
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdateServiceLine(ctx, tx, id, name)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "service_line",
			EntityID:   id,
			Before:     map[string]any{"name": current.Name},
			After:      map[string]any{"name": updated.Name},
		})
	}); err != nil {
		return domain.ServiceLine{}, mapSLConflict(err)
	}

	return updated, nil
}

// DiscontinueServiceLine soft-deactivates a service line (SP-1).
// 404 if not found; 409 SERVICE_LINE_IN_USE if any active position references it.
// TODO(Phase-5): also block when active placements reference the service line.
func (s *ServiceLineService) DiscontinueServiceLine(ctx context.Context, id string) (domain.ServiceLine, error) {
	current, err := s.repo.GetServiceLineByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ServiceLine{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ServiceLine{}, apperr.Internal(err)
	}

	// SP-1: block if active positions still reference this line.
	posCount, err := s.repo.CountActivePositionsForLine(ctx, id)
	if err != nil {
		return domain.ServiceLine{}, apperr.Internal(err)
	}
	if posCount > 0 {
		return domain.ServiceLine{}, apperr.Conflict("SERVICE_LINE_IN_USE")
	}

	// TODO(Phase-5): check active placements referencing this service line → SERVICE_LINE_IN_USE.

	var updated domain.ServiceLine
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetServiceLineStatus(ctx, tx, id, "inactive")
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("service_line.discontinue"),
			EntityType: "service_line",
			EntityID:   id,
			Before:     map[string]any{"status": current.Status},
			After:      map[string]any{"status": "inactive"},
		})
	}); err != nil {
		return domain.ServiceLine{}, apperr.Internal(err)
	}

	return updated, nil
}

// --- Positions ---

// ListPositions returns a cursor-paginated page of positions under a service line.
// 404 if the service line does not exist.
func (s *ServiceLineService) ListPositions(ctx context.Context, lineID string, f domain.PositionFilter) ([]domain.Position, *string, error) {
	if _, err := s.repo.GetServiceLineByID(ctx, lineID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil, apperr.NotFound()
		}
		return nil, nil, apperr.Internal(err)
	}

	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1

	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		f.Status = &lower
	}

	rows, err := s.repo.ListPositionsForLine(ctx, lineID, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// CreatePosition creates a position under a service line (SP-2/SP-3).
// 404 if the service line does not exist.
// Unique (line, name) violation → 409 POSITION_IN_USE.
func (s *ServiceLineService) CreatePosition(ctx context.Context, lineID string, p CreatePositionParams) (domain.Position, error) {
	if _, err := s.repo.GetServiceLineByID(ctx, lineID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Position{}, apperr.NotFound()
		}
		return domain.Position{}, apperr.Internal(err)
	}

	if strings.TrimSpace(p.Name) == "" {
		return domain.Position{}, apperr.Invalid(map[string]string{"name": "Wajib diisi."})
	}

	p.ServiceLineID = lineID

	var created domain.Position
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreatePosition(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "position",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"name":            created.Name,
				"alias":           created.Alias,
				"service_line_id": created.ServiceLineID,
			},
		})
	}); err != nil {
		return domain.Position{}, mapPosConflict(err)
	}

	return created, nil
}

// UpdatePosition patches a position's name/alias.
// 404 if not found; 409 POSITION_IN_USE on duplicate name within the same line.
func (s *ServiceLineService) UpdatePosition(ctx context.Context, id string, p UpdatePositionParams) (domain.Position, error) {
	current, err := s.repo.GetPositionByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Position{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Position{}, apperr.Internal(err)
	}

	// Carry forward unchanged fields (partial update).
	if p.Name == "" {
		p.Name = current.Name
	}
	if p.Alias == "" {
		p.Alias = current.Alias
	}
	p.ID = id

	var updated domain.Position
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdatePosition(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "position",
			EntityID:   id,
			Before:     map[string]any{"name": current.Name, "alias": current.Alias},
			After:      map[string]any{"name": updated.Name, "alias": updated.Alias},
		})
	}); err != nil {
		return domain.Position{}, mapPosConflict(err)
	}

	return updated, nil
}

// SoftDeletePosition soft-deletes a position (SP-4): sets deleted_at.
// 404 if not found.
// TODO(Phase-5): return 409 POSITION_IN_USE when active placements reference it.
func (s *ServiceLineService) SoftDeletePosition(ctx context.Context, id string) error {
	current, err := s.repo.GetPositionByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if err != nil {
		return apperr.Internal(err)
	}

	// TODO(Phase-5): check active placements referencing this position → POSITION_IN_USE.

	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		if inErr := s.repo.SoftDeletePosition(ctx, tx, id); inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("position.soft_delete"),
			EntityType: "position",
			EntityID:   id,
			Before:     map[string]any{"status": current.Status, "name": current.Name},
			After:      map[string]any{"deleted": true},
		})
	}); err != nil {
		return apperr.Internal(err)
	}

	return nil
}

// mapSLConflict translates unique-index violations for service lines.
// Unique violation on name → CONFLICT (generic; spec shows duplicate name → 409).
func mapSLConflict(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	if isUniqueViolation(err) {
		return apperr.Conflict("CONFLICT")
	}
	return apperr.Internal(err)
}

// mapPosConflict translates unique-index violations for positions.
// Unique violation on (line, name) → POSITION_IN_USE (SP-3).
func mapPosConflict(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	if isUniqueViolation(err) {
		return apperr.Conflict("POSITION_IN_USE")
	}
	return apperr.Internal(err)
}
