// Package placement (repository) — PlacementRepo implements the placement
// service's PlacementRepository interface over the 05-01 sqlc queries.
// Reads on the pool; writes via q.WithTx(tx). pgx.ErrNoRows → domain.ErrNotFound.
// Mirrors internal/repository/people/agreements_repo.go.
package placement

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/placement"
)

// PlacementRepo is the sqlc-backed implementation of svc.PlacementRepository.
type PlacementRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// compile-time check.
var _ svc.PlacementRepository = (*PlacementRepo)(nil)

// NewPlacementRepo returns a PlacementRepo backed by pool.
func NewPlacementRepo(pool *db.Pool) *PlacementRepo {
	return &PlacementRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// --- reads (pool) ---

// ListPlacements returns a cursor-paginated page of placements.
func (r *PlacementRepo) ListPlacements(ctx context.Context, f domain.PlacementFilter) ([]domain.Placement, error) {
	rows, err := r.q.ListPlacements(ctx, sqlcgen.ListPlacementsParams{
		CompanyID:             f.CompanyID,
		Position:              f.Position,
		EmployeeID:            f.EmployeeID,
		AgreementID:           f.AgreementID,
		Status:                f.Status,
		StatusIn:              f.StatusIn,
		EndDateLte:            ptrTimeToPgDate(f.EndDateLTE),
		AwaitingAgreement:     f.AwaitingAgreement,
		IncludeHistory:        f.IncludeHistory,
		Q:                     f.Q,
		CursorStatusChangedAt: f.CursorStatusChangedAt,
		CursorID:              f.CursorID,
		RowLimit:              int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Placement, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPlacementFromList(row))
	}
	return out, nil
}

// ListExpiringPlacements backs GET /placements/expiring.
func (r *PlacementRepo) ListExpiringPlacements(ctx context.Context, f domain.ExpiringFilter) ([]domain.Placement, error) {
	rows, err := r.q.ListExpiringPlacements(ctx, sqlcgen.ListExpiringPlacementsParams{
		Cutoff:        pgtype.Date{Time: f.Cutoff, Valid: true},
		CompanyID:     f.CompanyID,
		CursorEndDate: ptrTimeToPgDate(f.CursorEndDate),
		CursorID:      f.CursorID,
		RowLimit:      int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Placement, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPlacementFromExpiring(row))
	}
	return out, nil
}

// PlacementStats returns the global placement aggregates for the dashboard stat
// cards (F3.1 / C2SSLA). companyID scopes the counts (nil = global).
func (r *PlacementRepo) PlacementStats(ctx context.Context, companyID *string) (domain.PlacementStats, error) {
	row, err := r.q.PlacementGlobalStats(ctx, companyID)
	if err != nil {
		return domain.PlacementStats{}, err
	}
	return domain.PlacementStats{
		ClientCompanyCount: row.ClientCompanyCount,
		ActiveCount:        row.ActiveCount,
		ExpiringCount:      row.ExpiringCount,
		PendingCount:       row.PendingCount,
	}, nil
}

// SearchPositions returns DISTINCT free-text position labels matching pattern
// (the handler passes a '%q%' ILIKE pattern). Backs GET /positions:search.
func (r *PlacementRepo) SearchPositions(ctx context.Context, pattern string) ([]string, error) {
	return r.q.SearchPositions(ctx, pattern)
}

// GetPlacementByID fetches a single placement by SWP-PL id.
func (r *PlacementRepo) GetPlacementByID(ctx context.Context, id string) (domain.Placement, error) {
	row, err := r.q.GetPlacementByID(ctx, id)
	if err != nil {
		return domain.Placement{}, mapErr(err)
	}
	return mapPlacementFromGetByID(row), nil
}

// GetPlacementChain returns the full predecessor/successor chain (oldest→newest).
func (r *PlacementRepo) GetPlacementChain(ctx context.Context, id string) ([]domain.Placement, error) {
	rows, err := r.q.GetPlacementChain(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Placement, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPlacementFromChain(row))
	}
	return out, nil
}

// GetActivePlacementForEmployee returns the agent's active placement (INV-1 pre-check).
func (r *PlacementRepo) GetActivePlacementForEmployee(ctx context.Context, employeeID string) (domain.Placement, error) {
	row, err := r.q.GetActivePlacementForEmployee(ctx, employeeID)
	if err != nil {
		return domain.Placement{}, mapErr(err)
	}
	return mapPlacementFromActive(row), nil
}

// GetActivePlacementForEmployeeAtCompany returns the agent's active placement at a
// company (INV-4 check), row-locked. Caller must be inside a tx.
func (r *PlacementRepo) GetActivePlacementForEmployeeAtCompany(ctx context.Context, tx pgx.Tx, employeeID, companyID string) (domain.Placement, error) {
	row, err := r.q.WithTx(tx).GetActivePlacementForEmployeeAtCompanyForUpdate(ctx, sqlcgen.GetActivePlacementForEmployeeAtCompanyForUpdateParams{
		EmployeeID:      employeeID,
		ClientCompanyID: companyID,
	})
	if err != nil {
		return domain.Placement{}, mapErr(err)
	}
	return mapPlacementFromAtCompany(row), nil
}

// LockEmployeePlacements row-locks all of the agent's placements (INV-1 re-check in tx).
func (r *PlacementRepo) LockEmployeePlacements(ctx context.Context, tx pgx.Tx, employeeID string) ([]domain.Placement, error) {
	rows, err := r.q.WithTx(tx).LockEmployeePlacements(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Placement, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPlacementFromLock(row))
	}
	return out, nil
}

// --- cross-entity reads (pool) ---

func (r *PlacementRepo) GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error) {
	row, err := r.q.GetEmployeeByID(ctx, id)
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return domain.Employee{ID: row.ID, FullName: row.FullName, Status: row.Status}, nil
}

func (r *PlacementRepo) GetClientCompany(ctx context.Context, id string) (svc.CompanyRef, error) {
	row, err := r.q.GetClientCompanyByID(ctx, id)
	if err != nil {
		return svc.CompanyRef{}, mapErr(err)
	}
	return svc.CompanyRef{ID: row.ID, Name: row.Name, Status: row.Status, LeaderScope: row.LeaderScope}, nil
}

func (r *PlacementRepo) GetSite(ctx context.Context, id string) (svc.SiteRef, error) {
	row, err := r.q.GetSiteByID(ctx, id)
	if err != nil {
		return svc.SiteRef{}, mapErr(err)
	}
	return svc.SiteRef{ID: row.ID, ClientCompanyID: row.ClientCompanyID, Status: row.Status}, nil
}

func (r *PlacementRepo) GetAgreement(ctx context.Context, id string) (svc.AgreementRef, error) {
	row, err := r.q.GetAgreementByID(ctx, id)
	if err != nil {
		return svc.AgreementRef{}, mapErr(err)
	}
	ref := svc.AgreementRef{
		ID:         row.ID,
		EmployeeID: row.EmployeeID,
		Type:       row.Type,
		Status:     row.Status,
		StartDate:  pgtypeToTime(row.StartDate),
	}
	if row.EndDate.Valid {
		t := row.EndDate.Time
		ref.EndDate = &t
	}
	return ref, nil
}

// --- writes (tx) ---

func (r *PlacementRepo) CreatePlacement(ctx context.Context, tx pgx.Tx, p svc.CreatePlacementParams) (domain.Placement, error) {
	row, err := r.q.WithTx(tx).CreatePlacement(ctx, sqlcgen.CreatePlacementParams{
		EmployeeID:      p.EmployeeID,
		AgreementID:     p.AgreementID,
		ClientCompanyID: p.ClientCompanyID,
		SiteID:          p.SiteID,
		Position:        p.Position,
		StartDate:       pgtype.Date{Time: p.StartDate, Valid: true},
		EndDate:         ptrTimeToPgDate(p.EndDate),
		Notes:           p.Notes,
		LifecycleStatus: p.LifecycleStatus,
		PredecessorID:   p.PredecessorID,
		BackdateReason:  p.BackdateReason,
		CreatedBy:       p.CreatedBy,
	})
	if err != nil {
		return domain.Placement{}, mapErr(err)
	}
	return mapPlacementFromCreate(row), nil
}

func (r *PlacementRepo) UpdatePlacementFields(ctx context.Context, tx pgx.Tx, p svc.UpdatePlacementParams) (domain.Placement, error) {
	row, err := r.q.WithTx(tx).UpdatePlacementFields(ctx, sqlcgen.UpdatePlacementFieldsParams{
		Position: p.Position,
		EndDate:  ptrTimeToPgDate(p.EndDate),
		Notes:    p.Notes,
		ID:       p.ID,
	})
	if err != nil {
		return domain.Placement{}, mapErr(err)
	}
	return mapPlacementFromUpdate(row), nil
}

// SetPlacementAgreement backfills agreement_id (and the period-capped end_date) on a
// previously pending placement. Returns the updated placement (awaiting now false).
func (r *PlacementRepo) SetPlacementAgreement(ctx context.Context, tx pgx.Tx, p svc.SetAgreementParams) (domain.Placement, error) {
	row, err := r.q.WithTx(tx).SetPlacementAgreement(ctx, sqlcgen.SetPlacementAgreementParams{
		AgreementID: p.AgreementID,
		EndDate:     ptrTimeToPgDate(p.EndDate),
		ID:          p.ID,
	})
	if err != nil {
		return domain.Placement{}, mapErr(err)
	}
	return mapPlacementFromSetAgreement(row), nil
}

func (r *PlacementRepo) SetPlacementLifecycle(ctx context.Context, tx pgx.Tx, p svc.SetLifecycleParams) (domain.Placement, error) {
	row, err := r.q.WithTx(tx).SetPlacementLifecycle(ctx, sqlcgen.SetPlacementLifecycleParams{
		LifecycleStatus:   p.LifecycleStatus,
		EndedReason:       p.EndedReason,
		EndedAt:           ptrTimeToPgDate(p.EndedAt),
		TerminationReason: p.TerminationReason,
		ResignAt:          ptrTimeToPgDate(p.ResignAt),
		SuccessorID:       p.SuccessorID,
		ID:                p.ID,
	})
	if err != nil {
		return domain.Placement{}, mapErr(err)
	}
	return mapPlacementFromSetLifecycle(row), nil
}

func (r *PlacementRepo) SetPlacementSuccessor(ctx context.Context, tx pgx.Tx, id string, successorID *string) error {
	return r.q.WithTx(tx).SetPlacementSuccessor(ctx, sqlcgen.SetPlacementSuccessorParams{SuccessorID: successorID, ID: id})
}

func (r *PlacementRepo) InsertPlacementHistory(ctx context.Context, tx pgx.Tx, p svc.PlacementHistoryParams) error {
	_, err := r.q.WithTx(tx).InsertPlacementHistory(ctx, sqlcgen.InsertPlacementHistoryParams{
		PlacementID:   p.PlacementID,
		Action:        p.Action,
		ActorUserID:   p.ActorUserID,
		Reason:        p.Reason,
		EffectiveDate: ptrTimeToPgDate(p.EffectiveDate),
		StatusBefore:  p.StatusBefore,
		StatusAfter:   p.StatusAfter,
		Notes:         p.Notes,
	})
	return err
}

// --- mapping helpers ---

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func ptrTimeToPgDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

func pgtypeToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

func pgDateToPtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}
