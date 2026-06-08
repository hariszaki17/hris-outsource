// Package placement (repository) — ShiftLeaderRepo implements the shift-leader
// service's ShiftLeaderRepository interface over the 05-01 sqlc queries.
package placement

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/placement"
)

// ShiftLeaderRepo is the sqlc-backed implementation of svc.ShiftLeaderRepository.
type ShiftLeaderRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.ShiftLeaderRepository = (*ShiftLeaderRepo)(nil)

// NewShiftLeaderRepo returns a ShiftLeaderRepo backed by pool.
func NewShiftLeaderRepo(pool *db.Pool) *ShiftLeaderRepo {
	return &ShiftLeaderRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// --- cross-entity reads (pool) ---

func (r *ShiftLeaderRepo) GetClientCompany(ctx context.Context, id string) (svc.CompanyRef, error) {
	row, err := r.q.GetClientCompanyByID(ctx, id)
	if err != nil {
		return svc.CompanyRef{}, mapErr(err)
	}
	return svc.CompanyRef{ID: row.ID, Name: row.Name, Status: row.Status, LeaderScope: row.LeaderScope}, nil
}

func (r *ShiftLeaderRepo) GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error) {
	row, err := r.q.GetEmployeeByID(ctx, id)
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return domain.Employee{ID: row.ID, FullName: row.FullName, Status: row.Status}, nil
}

// --- current-leader reads (pool) ---

func (r *ShiftLeaderRepo) GetCurrentLeaderForCompany(ctx context.Context, companyID string) (domain.ShiftLeaderAssignment, error) {
	rows, err := r.q.ListShiftLeaderAssignments(ctx, sqlcgen.ListShiftLeaderAssignmentsParams{
		CompanyID:  &companyID,
		ActiveOnly: true,
	})
	if err != nil {
		return domain.ShiftLeaderAssignment{}, err
	}
	if len(rows) == 0 {
		return domain.ShiftLeaderAssignment{}, domain.ErrNotFound
	}
	return mapAssignmentFromList(rows[0]), nil
}

// GetActiveLeaderCompanyForEmployee returns the company the employee currently
// leads (active assignment), or domain.ErrNotFound if they lead none. Non-locking
// pool read used by the auth middleware to DERIVE a shift_leader's company scope at
// request time (GAP 3) — so reassigning a leader takes effect on their next request
// rather than at next login. The sla_active_employee_uq partial unique index
// guarantees at most one active row, so rows[0] is authoritative.
func (r *ShiftLeaderRepo) GetActiveLeaderCompanyForEmployee(ctx context.Context, employeeID string) (string, error) {
	rows, err := r.q.ListShiftLeaderAssignments(ctx, sqlcgen.ListShiftLeaderAssignmentsParams{
		EmployeeID: &employeeID,
		ActiveOnly: true,
	})
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", domain.ErrNotFound
	}
	return rows[0].ClientCompanyID, nil
}

func (r *ShiftLeaderRepo) GetAssignmentByID(ctx context.Context, id string) (domain.ShiftLeaderAssignment, error) {
	row, err := r.q.GetShiftLeaderAssignmentByID(ctx, id)
	if err != nil {
		return domain.ShiftLeaderAssignment{}, mapErr(err)
	}
	return domain.ShiftLeaderAssignment{
		ID: row.ID, ClientCompanyID: row.ClientCompanyID, SiteID: row.SiteID, EmployeeID: row.EmployeeID,
		AssignedAt: row.AssignedAt, UnassignedAt: row.UnassignedAt, AssignedBy: row.AssignedBy,
		VacatedReason: row.VacatedReason, Notes: row.Notes, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		ClientCompanyName: row.ClientCompanyName, EmployeeName: row.EmployeeName,
	}, nil
}

// --- locked reads (tx) ---

func (r *ShiftLeaderRepo) GetActiveLeaderForCompanyForUpdate(ctx context.Context, tx pgx.Tx, companyID string) (domain.ShiftLeaderAssignment, error) {
	row, err := r.q.WithTx(tx).GetActiveLeaderForCompanyForUpdate(ctx, companyID)
	if err != nil {
		return domain.ShiftLeaderAssignment{}, mapErr(err)
	}
	return mapAssignment(row), nil
}

func (r *ShiftLeaderRepo) GetActiveLeaderForSiteForUpdate(ctx context.Context, tx pgx.Tx, siteID string) (domain.ShiftLeaderAssignment, error) {
	row, err := r.q.WithTx(tx).GetActiveLeaderForSiteForUpdate(ctx, &siteID)
	if err != nil {
		return domain.ShiftLeaderAssignment{}, mapErr(err)
	}
	return mapAssignment(row), nil
}

func (r *ShiftLeaderRepo) GetActiveAssignmentForEmployeeForUpdate(ctx context.Context, tx pgx.Tx, employeeID string) (domain.ShiftLeaderAssignment, error) {
	row, err := r.q.WithTx(tx).GetActiveAssignmentForEmployeeForUpdate(ctx, employeeID)
	if err != nil {
		return domain.ShiftLeaderAssignment{}, mapErr(err)
	}
	return mapAssignment(row), nil
}

func (r *ShiftLeaderRepo) GetActivePlacementForEmployeeAtCompany(ctx context.Context, tx pgx.Tx, employeeID, companyID string) (domain.Placement, error) {
	row, err := r.q.WithTx(tx).GetActivePlacementForEmployeeAtCompanyForUpdate(ctx, sqlcgen.GetActivePlacementForEmployeeAtCompanyForUpdateParams{
		EmployeeID:      employeeID,
		ClientCompanyID: companyID,
	})
	if err != nil {
		return domain.Placement{}, mapErr(err)
	}
	return mapPlacementFromAtCompany(row), nil
}

// --- writes (tx) ---

func (r *ShiftLeaderRepo) CreateAssignment(ctx context.Context, tx pgx.Tx, p svc.CreateAssignmentParams) (domain.ShiftLeaderAssignment, error) {
	row, err := r.q.WithTx(tx).CreateShiftLeaderAssignment(ctx, sqlcgen.CreateShiftLeaderAssignmentParams{
		ClientCompanyID: p.ClientCompanyID,
		SiteID:          p.SiteID,
		EmployeeID:      p.EmployeeID,
		AssignedBy:      p.AssignedBy,
		Notes:           p.Notes,
	})
	if err != nil {
		return domain.ShiftLeaderAssignment{}, mapErr(err)
	}
	return mapAssignment(row), nil
}

func (r *ShiftLeaderRepo) EndAssignment(ctx context.Context, tx pgx.Tx, id string, vacatedReason *string) (domain.ShiftLeaderAssignment, error) {
	row, err := r.q.WithTx(tx).EndShiftLeaderAssignment(ctx, sqlcgen.EndShiftLeaderAssignmentParams{
		VacatedReason: vacatedReason,
		ID:            id,
	})
	if err != nil {
		return domain.ShiftLeaderAssignment{}, mapErr(err)
	}
	return mapAssignment(row), nil
}

// --- roster reads (pool) ---

func (r *ShiftLeaderRepo) RosterForCompany(ctx context.Context, f domain.PlacementFilter) ([]domain.Placement, error) {
	rows, err := r.q.RosterForCompany(ctx, sqlcgen.RosterForCompanyParams{
		ClientCompanyID:       deref(f.CompanyID),
		ServiceLineID:         f.ServiceLineID,
		Status:                f.Status,
		StatusIn:              f.StatusIn,
		IncludeHistory:        f.IncludeHistory,
		CursorStatusChangedAt: f.CursorStatusChangedAt,
		CursorID:              f.CursorID,
		RowLimit:              int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Placement, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPlacementFromRoster(row))
	}
	return out, nil
}

func (r *ShiftLeaderRepo) RosterSummary(ctx context.Context, companyID string) (domain.CompanyRosterSummary, error) {
	byStatus, err := r.q.RosterSummaryByStatus(ctx, companyID)
	if err != nil {
		return domain.CompanyRosterSummary{}, err
	}
	byLine, err := r.q.RosterSummaryByServiceLine(ctx, companyID)
	if err != nil {
		return domain.CompanyRosterSummary{}, err
	}

	summary := domain.CompanyRosterSummary{}
	for _, s := range byStatus {
		summary.ByStatus = append(summary.ByStatus, domain.RosterStatusCount{Status: s.Status, Count: int(s.Count)})
		switch s.Status {
		case "ACTIVE", "EXTENDED":
			summary.TotalActive += int(s.Count)
		case "PENDING_START", "SCHEDULED":
			summary.TotalScheduled += int(s.Count)
		case "EXPIRING":
			summary.TotalExpiring += int(s.Count)
		}
	}
	for _, l := range byLine {
		name := ""
		if l.ServiceLineName != nil {
			name = *l.ServiceLineName
		}
		summary.ByServiceLine = append(summary.ByServiceLine, domain.RosterServiceLineCount{
			ServiceLineID:   l.ServiceLineID,
			ServiceLineName: name,
			Count:           int(l.Count),
		})
	}
	return summary, nil
}

// --- mapping helpers ---

func mapAssignment(row sqlcgen.ShiftLeaderAssignment) domain.ShiftLeaderAssignment {
	return domain.ShiftLeaderAssignment{
		ID: row.ID, ClientCompanyID: row.ClientCompanyID, SiteID: row.SiteID, EmployeeID: row.EmployeeID,
		AssignedAt: row.AssignedAt, UnassignedAt: row.UnassignedAt, AssignedBy: row.AssignedBy,
		VacatedReason: row.VacatedReason, Notes: row.Notes, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func mapAssignmentFromList(row sqlcgen.ListShiftLeaderAssignmentsRow) domain.ShiftLeaderAssignment {
	return domain.ShiftLeaderAssignment{
		ID: row.ID, ClientCompanyID: row.ClientCompanyID, SiteID: row.SiteID, EmployeeID: row.EmployeeID,
		AssignedAt: row.AssignedAt, UnassignedAt: row.UnassignedAt, AssignedBy: row.AssignedBy,
		VacatedReason: row.VacatedReason, Notes: row.Notes, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		ClientCompanyName: row.ClientCompanyName, EmployeeName: row.EmployeeName,
	}
}

func mapPlacementFromRoster(row sqlcgen.RosterForCompanyRow) domain.Placement {
	p := placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		ClientCompanyID: row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
	p.EmployeeName = row.EmployeeName
	p.ClientCompanyName = row.ClientCompanyName
	p.SiteName = row.SiteName
	p.ServiceLineName = row.ServiceLineName
	p.PositionName = row.PositionName
	p.AgreementType = row.AgreementType
	return p
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
