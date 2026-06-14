// Package placement — ShiftLeaderService implements E3 shift-leader assignment
// (INV-2/3/4 enforcement via FOR UPDATE locks + leader_scope company|site) and
// the company roster (RO-*). Mirrors the placement service's structure.
package placement

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// ShiftLeaderRepository is the data dependency for the shift-leader service.
type ShiftLeaderRepository interface {
	// Cross-entity reads (pool).
	GetClientCompany(ctx context.Context, id string) (CompanyRef, error)
	GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error)
	// Current-leader read (pool).
	GetCurrentLeaderForCompany(ctx context.Context, companyID string) (domain.ShiftLeaderAssignment, error)
	GetAssignmentByID(ctx context.Context, id string) (domain.ShiftLeaderAssignment, error)
	// Locked reads (tx).
	GetActiveLeaderForCompanyForUpdate(ctx context.Context, tx pgx.Tx, companyID string) (domain.ShiftLeaderAssignment, error)
	GetActiveLeaderForSiteForUpdate(ctx context.Context, tx pgx.Tx, siteID string) (domain.ShiftLeaderAssignment, error)
	GetActiveAssignmentForEmployeeForUpdate(ctx context.Context, tx pgx.Tx, employeeID string) (domain.ShiftLeaderAssignment, error)
	GetActivePlacementForEmployeeAtCompany(ctx context.Context, tx pgx.Tx, employeeID, companyID string) (domain.Placement, error)
	// Writes (tx).
	CreateAssignment(ctx context.Context, tx pgx.Tx, p CreateAssignmentParams) (domain.ShiftLeaderAssignment, error)
	EndAssignment(ctx context.Context, tx pgx.Tx, id string, vacatedReason *string) (domain.ShiftLeaderAssignment, error)
	// Roster reads (pool).
	RosterForCompany(ctx context.Context, f domain.PlacementFilter) ([]domain.Placement, error)
	RosterSummary(ctx context.Context, companyID string) (domain.CompanyRosterSummary, error)
}

// CreateAssignmentParams carries fields for a new SLA row.
type CreateAssignmentParams struct {
	ClientCompanyID string
	SiteID          *string
	EmployeeID      string
	AssignedBy      *string
	Notes           *string
}

// ShiftLeaderService implements the shift-leader business logic.
type ShiftLeaderService struct {
	repo ShiftLeaderRepository
	txm  TxRunner
	now  Clock
}

// NewShiftLeaderService wires the shift-leader service.
func NewShiftLeaderService(repo ShiftLeaderRepository, txm TxRunner) *ShiftLeaderService {
	return &ShiftLeaderService{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *ShiftLeaderService) SetClock(c Clock) { s.now = c }

// today returns the current calendar date (Asia/Jakarta) expressed as UTC
// midnight, matching how placement start/end dates are parsed and stored — see
// PlacementService.today for the same convention.
func (s *ShiftLeaderService) today() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	n := s.now().In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// placementNotStarted reports whether a placement fails INV-4 because it has not
// begun yet. A placement is leadable once it has reached its start day; a
// PENDING_START whose start_date is still in the future is not. There is no
// stored-status promoter — "active now" is derived by date (consistent with the
// read-time DTO boundary), so we must NOT reject a PENDING_START that has already
// started (e.g. a placement created with start_date = today).
func placementNotStarted(p domain.Placement, today time.Time) bool {
	return p.LifecycleStatus == "PENDING_START" && p.StartDate.After(today)
}

// --- assign ---

// AssignParams carries the create-assignment request fields.
type AssignParams struct {
	ClientCompanyID string
	EmployeeID      string
	StartDate       time.Time
	Replace         bool
	ReplaceReason   *string
	Notes           *string
	ActorUserID     *string
}

// AssignResult bundles the new assignment + the replaced one (if replace=true).
type AssignResult struct {
	Assignment domain.ShiftLeaderAssignment
	Replaced   *domain.ShiftLeaderAssignment
}

// CreateAssignment assigns a shift leader, enforcing INV-2/3/4 under row locks.
func (s *ShiftLeaderService) CreateAssignment(ctx context.Context, p AssignParams) (AssignResult, error) {
	company, err := s.repo.GetClientCompany(ctx, p.ClientCompanyID)
	if errors.Is(err, domain.ErrNotFound) {
		return AssignResult{}, apperr.NotFound()
	}
	if err != nil {
		return AssignResult{}, apperr.Internal(err)
	}
	if !strings.EqualFold(company.Status, "active") {
		return AssignResult{}, apperr.Conflict("COMPANY_INACTIVE")
	}

	emp, err := s.repo.GetEmployeeByID(ctx, p.EmployeeID)
	if errors.Is(err, domain.ErrNotFound) {
		return AssignResult{}, apperr.NotFound()
	}
	if err != nil {
		return AssignResult{}, apperr.Internal(err)
	}
	if !strings.EqualFold(emp.Status, "active") {
		return AssignResult{}, apperr.Rule("LEADER_NOT_ELIGIBLE", map[string]string{"employee_id": "Karyawan tidak aktif."})
	}

	var result AssignResult
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// INV-4: employee must have a placement at the company that has already
		// started (a future-dated PENDING_START is not yet leadable). Row-locked.
		placement, inErr := s.repo.GetActivePlacementForEmployeeAtCompany(ctx, tx, p.EmployeeID, p.ClientCompanyID)
		if errors.Is(inErr, domain.ErrNotFound) || (inErr == nil && placementNotStarted(placement, s.today())) {
			return inv4Conflict(p.ClientCompanyID, p.EmployeeID, placement, errors.Is(inErr, domain.ErrNotFound))
		}
		if inErr != nil {
			return inErr
		}

		// INV-3: employee must not already lead another unit. Row-locked.
		if existing, inErr := s.repo.GetActiveAssignmentForEmployeeForUpdate(ctx, tx, p.EmployeeID); inErr == nil {
			return inv3Conflict(existing)
		} else if !errors.Is(inErr, domain.ErrNotFound) {
			return inErr
		}

		// INV-2: existing active leader for the unit (company-scope here).
		var replaced *domain.ShiftLeaderAssignment
		if current, inErr := s.repo.GetActiveLeaderForCompanyForUpdate(ctx, tx, p.ClientCompanyID); inErr == nil {
			if !p.Replace {
				return inv2Conflict(current)
			}
			ended, endErr := s.repo.EndAssignment(ctx, tx, current.ID, strPtr("REASSIGNED"))
			if endErr != nil {
				return endErr
			}
			replaced = &ended
		} else if !errors.Is(inErr, domain.ErrNotFound) {
			return inErr
		}

		created, inErr := s.repo.CreateAssignment(ctx, tx, CreateAssignmentParams{
			ClientCompanyID: p.ClientCompanyID,
			SiteID:          nil, // company-scope
			EmployeeID:      p.EmployeeID,
			AssignedBy:      p.ActorUserID,
			Notes:           p.Notes,
		})
		if inErr != nil {
			if isUniqueViolation(inErr) {
				return apperr.Conflict("INV_2_VIOLATION")
			}
			return inErr
		}

		if inErr := audit.Record(ctx, tx, audit.Entry{
			Action: audit.Action("shift_leader.assign"), EntityType: "shift_leader_assignment", EntityID: created.ID,
			After: map[string]any{"client_company_id": created.ClientCompanyID, "employee_id": created.EmployeeID},
		}); inErr != nil {
			return inErr
		}
		// TODO(Phase-11 notifications): enqueue NotificationArgs (leader assigned).

		result.Assignment = created
		result.Replaced = replaced
		return nil
	}); err != nil {
		return AssignResult{}, asAppErr(err)
	}
	return result, nil
}

// --- replace ---

// ReplaceParams carries the replace request fields (addresses the assignment by ID).
type ReplaceParams struct {
	AssignmentID  string
	NewEmployeeID string
	StartDate     time.Time
	ReplaceReason string
	Notes         *string
	ActorUserID   *string
}

// ReplaceAssignment ends the existing assignment (REASSIGNED) + creates a new one.
func (s *ShiftLeaderService) ReplaceAssignment(ctx context.Context, p ReplaceParams) (AssignResult, error) {
	cur, err := s.repo.GetAssignmentByID(ctx, p.AssignmentID)
	if errors.Is(err, domain.ErrNotFound) {
		return AssignResult{}, apperr.NotFound()
	}
	if err != nil {
		return AssignResult{}, apperr.Internal(err)
	}
	if !cur.Active() {
		return AssignResult{}, apperr.Conflict("ALREADY_ENDED")
	}

	emp, err := s.repo.GetEmployeeByID(ctx, p.NewEmployeeID)
	if errors.Is(err, domain.ErrNotFound) {
		return AssignResult{}, apperr.NotFound()
	}
	if err != nil {
		return AssignResult{}, apperr.Internal(err)
	}
	if !strings.EqualFold(emp.Status, "active") {
		return AssignResult{}, apperr.Rule("LEADER_NOT_ELIGIBLE", map[string]string{"new_employee_id": "Karyawan tidak aktif."})
	}

	var result AssignResult
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// Re-validate INV-4 for the new candidate at the same company.
		placement, inErr := s.repo.GetActivePlacementForEmployeeAtCompany(ctx, tx, p.NewEmployeeID, cur.ClientCompanyID)
		if errors.Is(inErr, domain.ErrNotFound) || (inErr == nil && placementNotStarted(placement, s.today())) {
			return inv4Conflict(cur.ClientCompanyID, p.NewEmployeeID, placement, errors.Is(inErr, domain.ErrNotFound))
		}
		if inErr != nil {
			return inErr
		}

		// INV-3 for the new candidate.
		if existing, inErr := s.repo.GetActiveAssignmentForEmployeeForUpdate(ctx, tx, p.NewEmployeeID); inErr == nil {
			return inv3Conflict(existing)
		} else if !errors.Is(inErr, domain.ErrNotFound) {
			return inErr
		}

		ended, inErr := s.repo.EndAssignment(ctx, tx, cur.ID, strPtr("REASSIGNED"))
		if inErr != nil {
			return inErr
		}

		created, inErr := s.repo.CreateAssignment(ctx, tx, CreateAssignmentParams{
			ClientCompanyID: cur.ClientCompanyID,
			SiteID:          cur.SiteID,
			EmployeeID:      p.NewEmployeeID,
			AssignedBy:      p.ActorUserID,
			Notes:           p.Notes,
		})
		if inErr != nil {
			if isUniqueViolation(inErr) {
				return apperr.Conflict("INV_2_VIOLATION")
			}
			return inErr
		}

		if inErr := audit.Record(ctx, tx, audit.Entry{
			Action: audit.Action("shift_leader.replace"), EntityType: "shift_leader_assignment", EntityID: created.ID,
			Before: map[string]any{"replaced_id": cur.ID}, After: map[string]any{"employee_id": created.EmployeeID},
		}); inErr != nil {
			return inErr
		}
		// TODO(Phase-11 notifications): enqueue NotificationArgs (leader replaced).

		result.Assignment = created
		result.Replaced = &ended
		return nil
	}); err != nil {
		return AssignResult{}, asAppErr(err)
	}
	return result, nil
}

// --- end ---

// EndAssignmentParams carries the end request fields.
type EndAssignmentParams struct {
	AssignmentID string
	Reason       *string // free-text → MANUAL
	ActorUserID  *string
}

// EndAssignment vacates a leader assignment (vacated_reason=MANUAL).
func (s *ShiftLeaderService) EndAssignment(ctx context.Context, p EndAssignmentParams) (domain.ShiftLeaderAssignment, error) {
	cur, err := s.repo.GetAssignmentByID(ctx, p.AssignmentID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ShiftLeaderAssignment{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ShiftLeaderAssignment{}, apperr.Internal(err)
	}
	if !cur.Active() {
		return domain.ShiftLeaderAssignment{}, apperr.Conflict("ALREADY_ENDED")
	}

	var ended domain.ShiftLeaderAssignment
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		ended, inErr = s.repo.EndAssignment(ctx, tx, cur.ID, strPtr("MANUAL"))
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action: audit.Action("shift_leader.end"), EntityType: "shift_leader_assignment", EntityID: cur.ID,
			Before: map[string]any{"active": true}, After: map[string]any{"vacated_reason": "MANUAL"},
		})
		// TODO(Phase-11 notifications): enqueue NotificationArgs (leader vacated).
	}); err != nil {
		return domain.ShiftLeaderAssignment{}, asAppErr(err)
	}
	return ended, nil
}

// --- roster ---

// CompanyRoster bundles the per-company roster projection.
type CompanyRoster struct {
	CompanyID     string
	CompanyName   string
	Placements    []domain.Placement
	CurrentLeader *domain.ShiftLeaderAssignment
	Summary       domain.CompanyRosterSummary
	NextCursor    *string
	HasMore       bool
}

// GetCompanyRoster returns the company's roster (RO-*), scope-guarded (RO-4).
func (s *ShiftLeaderService) GetCompanyRoster(ctx context.Context, companyID string, f domain.PlacementFilter) (CompanyRoster, error) {
	// RO-4: shift_leader only their company; HR/super-admin global.
	if err := rbac.GuardCompany(ctx, companyID); err != nil {
		return CompanyRoster{}, err
	}

	company, err := s.repo.GetClientCompany(ctx, companyID)
	if errors.Is(err, domain.ErrNotFound) {
		return CompanyRoster{}, apperr.NotFound()
	}
	if err != nil {
		return CompanyRoster{}, apperr.Internal(err)
	}

	limit := httpx.ClampLimit(f.Limit)
	f.CompanyID = &companyID
	f.Limit = limit + 1

	rows, err := s.repo.RosterForCompany(ctx, f)
	if err != nil {
		return CompanyRoster{}, apperr.Internal(err)
	}
	var next *string
	hasMore := false
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, cErr := httpx.EncodeCursor(listCursor{StatusChangedAt: last.StatusChangedAt, ID: last.ID})
		if cErr != nil {
			return CompanyRoster{}, apperr.Internal(cErr)
		}
		next = &cur
		hasMore = true
	}

	summary, err := s.repo.RosterSummary(ctx, companyID)
	if err != nil {
		return CompanyRoster{}, apperr.Internal(err)
	}

	roster := CompanyRoster{
		CompanyID:   companyID,
		CompanyName: company.Name,
		Placements:  rows,
		Summary:     summary,
		NextCursor:  next,
		HasMore:     hasMore,
	}
	if lead, ok, lErr := s.currentLeaderForCompany(ctx, companyID); lErr != nil {
		return CompanyRoster{}, lErr
	} else if ok {
		roster.CurrentLeader = &lead
	}
	return roster, nil
}

// --- internal helpers (used by the placement service) ---

// currentLeaderForCompany returns the active leader for a company (ok=false on vacancy).
func (s *ShiftLeaderService) currentLeaderForCompany(ctx context.Context, companyID string) (domain.ShiftLeaderAssignment, bool, error) {
	lead, err := s.repo.GetCurrentLeaderForCompany(ctx, companyID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ShiftLeaderAssignment{}, false, nil
	}
	if err != nil {
		return domain.ShiftLeaderAssignment{}, false, apperr.Internal(err)
	}
	return lead, true, nil
}

// GetCurrentLeader is the public lookup backing /shift-leader-assignments/by-company/{id}.
func (s *ShiftLeaderService) GetCurrentLeader(ctx context.Context, companyID string) (domain.ShiftLeaderAssignment, error) {
	lead, ok, err := s.currentLeaderForCompany(ctx, companyID)
	if err != nil {
		return domain.ShiftLeaderAssignment{}, err
	}
	if !ok {
		return domain.ShiftLeaderAssignment{}, &apperr.Error{Code: "NO_ACTIVE_LEADER", HTTPStatus: 404}
	}
	return lead, nil
}

// autoVacateForEmployeeAtCompany ends the agent's leadership of a company within
// the caller's tx (SL-6: PLACEMENT_ENDED, system actor). No-op if not leading.
func (s *ShiftLeaderService) autoVacateForEmployeeAtCompany(ctx context.Context, tx pgx.Tx, employeeID, companyID string) error {
	existing, err := s.repo.GetActiveAssignmentForEmployeeForUpdate(ctx, tx, employeeID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if existing.ClientCompanyID != companyID {
		return nil
	}
	_, err = s.repo.EndAssignment(ctx, tx, existing.ID, strPtr("PLACEMENT_ENDED"))
	return err
}

// --- INV detail builders ---

func inv2Conflict(current domain.ShiftLeaderAssignment) error {
	sum := toLeaderSummary(current)
	return apperr.ConflictWithDetails("INV_2_VIOLATION",
		map[string]string{"client_company_id": "Sudah ada shift leader aktif."},
		INVViolationDetails{Invariant: "INV_2", CurrentAssignment: &sum, SuggestedActions: []string{"replace"}})
}

func inv3Conflict(existing domain.ShiftLeaderAssignment) error {
	sum := toLeaderSummary(existing)
	return apperr.ConflictWithDetails("INV_3_VIOLATION",
		map[string]string{"employee_id": "Sudah memimpin perusahaan lain."},
		INVViolationDetails{Invariant: "INV_3", ExistingAssignment: &sum, SuggestedActions: []string{"end_existing_first"}})
}

func inv4Conflict(companyID, employeeID string, placement domain.Placement, _ bool) error {
	details := INVViolationDetails{
		Invariant:        "INV_4",
		CompanyID:        companyID,
		EmployeeID:       employeeID,
		SuggestedActions: []string{"assign_after_placement"},
	}
	if placement.ID != "" {
		details.EmployeePlacementsAtCompany = []PlacementSummary{toPlacementSummary(placement)}
	}
	return apperr.ConflictWithDetails("INV_4_VIOLATION",
		map[string]string{"employee_id": "Tidak memiliki penempatan aktif di perusahaan ini."}, details)
}

func toLeaderSummary(a domain.ShiftLeaderAssignment) ShiftLeaderSummary {
	sum := ShiftLeaderSummary{
		ID:                a.ID,
		ClientCompanyID:   a.ClientCompanyID,
		ClientCompanyName: a.ClientCompanyName,
		EmployeeID:        a.EmployeeID,
		EmployeeName:      a.EmployeeName,
		AssignedAt:        a.AssignedAt.UTC().Format(time.RFC3339),
	}
	if a.UnassignedAt != nil {
		u := a.UnassignedAt.UTC().Format(time.RFC3339)
		sum.UnassignedAt = &u
	}
	return sum
}

// isUniqueViolation checks for Postgres error code 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "23505") ||
		strings.Contains(err.Error(), "unique constraint") ||
		strings.Contains(err.Error(), "duplicate key")
}
