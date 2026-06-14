// Package placement implements the E3 placement service: CRUD + lifecycle state
// machine (renew/transfer/end/resign/terminate) with INV-1..4 enforcement,
// transfer/renew atomicity, placement_history + audit on every action, and the
// company roster. Mirrors the Phase-4 people slice (consumer-defined repo
// interface, TxRunner, Clock, supersede-before-insert, audit-in-tx).
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
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// --- dependency interfaces (consumer-defined) ---

// TxRunner runs fn inside a database transaction (mirrors people.TxRunner).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock is the time source (overridable in tests).
type Clock func() time.Time

// CompanyRef / SiteRef / AgreementRef are the slim cross-entity projections the
// service needs for FK + status validation.
type CompanyRef struct {
	ID          string
	Name        string
	Status      string // "active" | "archived"
	LeaderScope string // "company" | "site"
}

type SiteRef struct {
	ID              string
	ClientCompanyID string
	Status          string
}

type AgreementRef struct {
	ID         string
	EmployeeID string
	Type       string // PKWT | PKWTT
	Status     string // "active" | ...
	StartDate  time.Time
	EndDate    *time.Time
}

// CreatePlacementParams carries the fields for inserting a placement.
// AgreementID is optional: nil = pending agreement (awaiting_agreement), in which
// case the BR-1b period validation is skipped and end_date may be open-ended.
type CreatePlacementParams struct {
	EmployeeID      string
	AgreementID     *string
	ClientCompanyID string
	SiteID          string
	Position        string // free-text position label
	StartDate       time.Time
	EndDate         *time.Time
	Notes           *string
	LifecycleStatus string
	PredecessorID   *string
	BackdateReason  *string
	CreatedBy       *string
}

// UpdatePlacementParams carries the limited-field PATCH columns.
type UpdatePlacementParams struct {
	ID       string
	Position string // free-text position label
	EndDate  *time.Time
	Notes    *string
}

// SetAgreementParams carries the backfill of an agreement onto a pending placement.
// EndDate is the (possibly BR-1b auto-capped) end_date persisted alongside it.
type SetAgreementParams struct {
	ID          string
	AgreementID *string
	EndDate     *time.Time
}

// SetLifecycleParams drives end/terminate/resign/transfer/supersede.
type SetLifecycleParams struct {
	ID                string
	LifecycleStatus   string
	EndedReason       *string
	EndedAt           *time.Time
	TerminationReason *string
	ResignAt          *time.Time
	SuccessorID       *string
}

// PlacementHistoryParams is one placement_history row.
type PlacementHistoryParams struct {
	PlacementID   string
	Action        string
	ActorUserID   *string
	Reason        *string
	EffectiveDate *time.Time
	StatusBefore  *string
	StatusAfter   *string
	Notes         *string
}

// PlacementRepository is the data dependency for the placement service.
type PlacementRepository interface {
	// Reads (pool).
	ListPlacements(ctx context.Context, f domain.PlacementFilter) ([]domain.Placement, error)
	ListExpiringPlacements(ctx context.Context, f domain.ExpiringFilter) ([]domain.Placement, error)
	PlacementStats(ctx context.Context, companyID *string) (domain.PlacementStats, error)
	SearchPositions(ctx context.Context, pattern string) ([]string, error)
	GetPlacementByID(ctx context.Context, id string) (domain.Placement, error)
	GetPlacementChain(ctx context.Context, id string) ([]domain.Placement, error)
	GetActivePlacementForEmployee(ctx context.Context, employeeID string) (domain.Placement, error)
	GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error)
	GetClientCompany(ctx context.Context, id string) (CompanyRef, error)
	GetSite(ctx context.Context, id string) (SiteRef, error)
	GetAgreement(ctx context.Context, id string) (AgreementRef, error)
	// Locked reads (tx).
	GetActivePlacementForEmployeeAtCompany(ctx context.Context, tx pgx.Tx, employeeID, companyID string) (domain.Placement, error)
	LockEmployeePlacements(ctx context.Context, tx pgx.Tx, employeeID string) ([]domain.Placement, error)
	// Writes (tx).
	CreatePlacement(ctx context.Context, tx pgx.Tx, p CreatePlacementParams) (domain.Placement, error)
	UpdatePlacementFields(ctx context.Context, tx pgx.Tx, p UpdatePlacementParams) (domain.Placement, error)
	SetPlacementAgreement(ctx context.Context, tx pgx.Tx, p SetAgreementParams) (domain.Placement, error)
	SetPlacementLifecycle(ctx context.Context, tx pgx.Tx, p SetLifecycleParams) (domain.Placement, error)
	SetPlacementSuccessor(ctx context.Context, tx pgx.Tx, id string, successorID *string) error
	InsertPlacementHistory(ctx context.Context, tx pgx.Tx, p PlacementHistoryParams) error
}

// PlacementService implements the placement business logic.
type PlacementService struct {
	repo   PlacementRepository
	leader *ShiftLeaderService // for current-leader joins + auto-vacate on resolution
	txm    TxRunner
	now    Clock
}

// NewPlacementService wires the placement service. leader may be set after
// construction via SetLeaderService (the two services are mutually referential).
func NewPlacementService(repo PlacementRepository, txm TxRunner) *PlacementService {
	return &PlacementService{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *PlacementService) SetClock(c Clock) { s.now = c }

// SetLeaderService wires the shift-leader service for current-leader resolution
// and auto-vacate on placement resolution (SL-6).
func (s *PlacementService) SetLeaderService(l *ShiftLeaderService) { s.leader = l }

// --- terminal-state helpers ---

var terminalStates = map[string]bool{
	"ENDED": true, "TRANSFERRED": true, "TERMINATED": true,
	"RESIGNED": true, "SUPERSEDED": true,
}

func isTerminal(status string) bool { return terminalStates[status] }

// today returns the current calendar date in Asia/Jakarta (date-only, UTC clock).
func (s *PlacementService) today() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	n := s.now().In(loc)
	// Express the Jakarta calendar day as UTC midnight so it compares correctly
	// against start/end dates, which are parsed and stored as UTC-midnight `date`
	// columns. Using loc here put `today` 7h behind a same-day start_date, so a
	// placement starting today was wrongly left PENDING_START instead of ACTIVE.
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// expiringWindowDays is the default expiring-soon threshold (FEATURE.md §7).
const expiringWindowDays = 30

// --- INV / structured details ---

// PlacementSummary is the compact placement projection used inside error
// payloads + the history_chain. JSON tags match the openapi PlacementSummary.
type PlacementSummary struct {
	ID                string  `json:"id"`
	EmployeeID        string  `json:"employee_id"`
	ClientCompanyID   string  `json:"client_company_id"`
	ClientCompanyName *string `json:"client_company_name,omitempty"`
	Position          string  `json:"position"`
	LifecycleStatus   string  `json:"lifecycle_status"`
	StartDate         string  `json:"start_date"`
	EndDate           *string `json:"end_date"`
}

// ShiftLeaderSummary mirrors the openapi ShiftLeaderAssignmentSummary.
type ShiftLeaderSummary struct {
	ID                string  `json:"id"`
	ClientCompanyID   string  `json:"client_company_id"`
	ClientCompanyName *string `json:"client_company_name,omitempty"`
	EmployeeID        string  `json:"employee_id"`
	EmployeeName      *string `json:"employee_name,omitempty"`
	AssignedAt        string  `json:"assigned_at"`
	UnassignedAt      *string `json:"unassigned_at,omitempty"`
}

// INVViolationDetails is the structured error.details payload for INV-1..4
// (openapi INVViolationDetails). The frontend reads current_placement /
// current_assignment / existing_assignment / employee_placements_at_company /
// suggested_actions to render the right warning state + CTA.
type INVViolationDetails struct {
	Invariant                   string              `json:"invariant"`
	CurrentPlacement            *PlacementSummary   `json:"current_placement,omitempty"`
	CurrentAssignment           *ShiftLeaderSummary `json:"current_assignment,omitempty"`
	ExistingAssignment          *ShiftLeaderSummary `json:"existing_assignment,omitempty"`
	CompanyID                   string              `json:"company_id,omitempty"`
	EmployeeID                  string              `json:"employee_id,omitempty"`
	EmployeePlacementsAtCompany []PlacementSummary  `json:"employee_placements_at_company,omitempty"`
	SuggestedActions            []string            `json:"suggested_actions,omitempty"`
}

func toPlacementSummary(p domain.Placement) PlacementSummary {
	sum := PlacementSummary{
		ID:                p.ID,
		EmployeeID:        p.EmployeeID,
		ClientCompanyID:   p.ClientCompanyID,
		ClientCompanyName: p.ClientCompanyName,
		Position:          p.Position,
		LifecycleStatus:   p.LifecycleStatus,
		StartDate:         p.StartDate.Format("2006-01-02"),
	}
	if p.EndDate != nil {
		e := p.EndDate.Format("2006-01-02")
		sum.EndDate = &e
	}
	return sum
}

// --- list / get ---

type listCursor struct {
	StatusChangedAt time.Time `json:"c"`
	ID              string    `json:"i"`
}

type expiringCursor struct {
	EndDate time.Time `json:"e"`
	ID      string    `json:"i"`
}

// ListPlacements returns a cursor-paginated page of placements.
func (s *PlacementService) ListPlacements(ctx context.Context, f domain.PlacementFilter) ([]domain.Placement, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1

	rows, err := s.repo.ListPlacements(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(listCursor{StatusChangedAt: last.StatusChangedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		next = &cur
	}
	return rows, next, nil
}

// ListExpiringPlacements backs GET /placements/expiring (sorted end_date:asc).
func (s *PlacementService) ListExpiringPlacements(ctx context.Context, withinDays int, companyID *string, limit int, cursorEndDate *time.Time, cursorID *string) ([]domain.Placement, *string, error) {
	if withinDays <= 0 {
		withinDays = expiringWindowDays
	}
	clamped := httpx.ClampLimit(limit)
	f := domain.ExpiringFilter{
		Cutoff:        s.today().AddDate(0, 0, withinDays),
		CompanyID:     companyID,
		Limit:         clamped + 1,
		CursorEndDate: cursorEndDate,
		CursorID:      cursorID,
	}

	rows, err := s.repo.ListExpiringPlacements(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var next *string
	if len(rows) > clamped {
		rows = rows[:clamped]
		last := rows[len(rows)-1]
		end := last.StartDate
		if last.EndDate != nil {
			end = *last.EndDate
		}
		cur, err := httpx.EncodeCursor(expiringCursor{EndDate: end, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		next = &cur
	}
	return rows, next, nil
}

// PlacementStats returns the global placement aggregates backing the /placements
// dashboard stat cards (F3.1 / C2SSLA). companyID scopes the counts (nil =
// global; set to a shift-leader's company for company-scoped totals).
func (s *PlacementService) PlacementStats(ctx context.Context, companyID *string) (domain.PlacementStats, error) {
	stats, err := s.repo.PlacementStats(ctx, companyID)
	if err != nil {
		return domain.PlacementStats{}, apperr.Internal(err)
	}
	return stats, nil
}

// SearchPositions backs GET /positions:search — the free-text position typeahead.
// Returns the distinct existing position labels matching q (case-insensitive
// substring); empty q returns every label. No master / FK / ID. The '%'..'%'
// wrapping is built here so the SQL stays a plain ILIKE bind.
func (s *PlacementService) SearchPositions(ctx context.Context, q string) ([]string, error) {
	pattern := "%" + strings.TrimSpace(q) + "%"
	rows, err := s.repo.SearchPositions(ctx, pattern)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if rows == nil {
		rows = []string{}
	}
	return rows, nil
}

// PlacementDetail bundles a placement with its chain + current leader.
type PlacementDetail struct {
	Placement     domain.Placement
	HistoryChain  []domain.Placement
	CurrentLeader *domain.ShiftLeaderAssignment
}

// GetPlacement returns a placement + history chain + current shift leader.
func (s *PlacementService) GetPlacement(ctx context.Context, id string) (PlacementDetail, error) {
	p, err := s.repo.GetPlacementByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return PlacementDetail{}, apperr.NotFound()
	}
	if err != nil {
		return PlacementDetail{}, apperr.Internal(err)
	}

	// Agent self-scope (E3 getPlacement x-rbac, scope:self): an agent may read ONLY
	// their own placement. Reading anyone else is hidden as 404 (no existence leak),
	// mirroring the employee/attendance/leave self-scope rule. Staff roles unaffected.
	if pr, ok := auth.PrincipalFrom(ctx); ok && pr.Role == auth.RoleAgent {
		if pr.EmployeeID == "" || pr.EmployeeID != p.EmployeeID {
			return PlacementDetail{}, apperr.NotFound()
		}
	}

	chain, err := s.repo.GetPlacementChain(ctx, id)
	if err != nil {
		return PlacementDetail{}, apperr.Internal(err)
	}

	detail := PlacementDetail{Placement: p, HistoryChain: chain}
	if s.leader != nil {
		if lead, ok, err := s.leader.currentLeaderForCompany(ctx, p.ClientCompanyID); err != nil {
			return PlacementDetail{}, err
		} else if ok {
			detail.CurrentLeader = &lead
		}
	}
	// Soft warning: company has no active leader.
	if detail.CurrentLeader == nil {
		detail.Placement.Warnings = append(detail.Placement.Warnings, "NO_SHIFT_LEADER_AT_COMPANY")
	}
	return detail, nil
}

// --- create ---

// CreatePlacement creates a placement, enforcing INV-1 + BR-1b/3/4/5/6.
func (s *PlacementService) CreatePlacement(ctx context.Context, p CreatePlacementParams) (domain.Placement, error) {
	// 1. Employee exists + active.
	emp, err := s.repo.GetEmployeeByID(ctx, p.EmployeeID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Placement{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Placement{}, apperr.Internal(err)
	}
	if !strings.EqualFold(emp.Status, "active") {
		return domain.Placement{}, apperr.Rule("RULE_VIOLATION", map[string]string{"employee_id": "Karyawan tidak aktif."})
	}

	// Company exists + ACTIVE (BR-3).
	company, err := s.repo.GetClientCompany(ctx, p.ClientCompanyID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Placement{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Placement{}, apperr.Internal(err)
	}
	if !strings.EqualFold(company.Status, "active") {
		return domain.Placement{}, apperr.Conflict("COMPANY_INACTIVE")
	}

	// Scope guard: a lead may only arrange placements at its assigned client
	// companies (no-op for super/hr = global). Done after the company FK is
	// resolved so we guard the real target company.
	if serr := rbac.GuardCompany(ctx, p.ClientCompanyID); serr != nil {
		return domain.Placement{}, serr
	}

	// Site belongs to company (BR-3b).
	site, err := s.repo.GetSite(ctx, p.SiteID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Placement{}, apperr.Invalid(map[string]string{"site_id": "Lokasi tidak ditemukan."})
	}
	if err != nil {
		return domain.Placement{}, apperr.Internal(err)
	}
	if site.ClientCompanyID != p.ClientCompanyID {
		return domain.Placement{}, apperr.Invalid(map[string]string{"site_id": "Lokasi bukan milik perusahaan ini."})
	}

	// Agreement is OPTIONAL. When present it must belong to the employee; when nil
	// the placement is created "pending agreement" (awaiting_agreement) and the
	// BR-1b period validation below is skipped.
	var ag *AgreementRef
	if p.AgreementID != nil && *p.AgreementID != "" {
		fetched, agErr := s.repo.GetAgreement(ctx, *p.AgreementID)
		if errors.Is(agErr, domain.ErrNotFound) {
			return domain.Placement{}, apperr.Invalid(map[string]string{"agreement_id": "Perjanjian tidak ditemukan."})
		}
		if agErr != nil {
			return domain.Placement{}, apperr.Internal(agErr)
		}
		if fetched.EmployeeID != p.EmployeeID {
			return domain.Placement{}, apperr.Invalid(map[string]string{"agreement_id": "Perjanjian bukan milik karyawan ini."})
		}
		ag = &fetched
	} else {
		// Normalize empty-string to nil so the column stores NULL (awaiting_agreement).
		p.AgreementID = nil
	}

	today := s.today()

	// 2. Date validation (BR-4, BR-6).
	if p.EndDate != nil && !p.EndDate.After(p.StartDate) {
		return domain.Placement{}, apperr.Invalid(map[string]string{"end_date": "Tanggal berakhir harus setelah tanggal mulai."})
	}
	if p.StartDate.Before(today) && (p.BackdateReason == nil || strings.TrimSpace(*p.BackdateReason) == "") {
		return domain.Placement{}, apperr.Invalid(map[string]string{"backdate_reason": "Alasan backdating wajib diisi."})
	}

	// 3. Agreement-period validation (BR-1b) — ONLY when an agreement is present.
	// Out-of-range START → 422; PKWT end past the agreement end → auto-cap + warning.
	// No agreement → skip the period check; end_date may be open-ended.
	var warnings []string
	if ag != nil {
		if err := validateStartWithinAgreement(*ag, p.StartDate); err != nil {
			return domain.Placement{}, err
		}
		if ag.EndDate != nil && p.EndDate != nil && p.EndDate.After(*ag.EndDate) {
			capped := *ag.EndDate
			p.EndDate = &capped
			warnings = append(warnings, "END_DATE_AUTO_CAPPED_TO_AGREEMENT")
		}
	}

	// 4. INV-1 service pre-check.
	if existing, err := s.repo.GetActivePlacementForEmployee(ctx, p.EmployeeID); err == nil {
		return domain.Placement{}, inv1Conflict(existing)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.Placement{}, apperr.Internal(err)
	}

	// 7. Lifecycle on create (BR-5).
	p.LifecycleStatus = "PENDING_START"
	if !p.StartDate.After(today) {
		p.LifecycleStatus = "ACTIVE"
	}

	var created domain.Placement
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// 5. Lock the agent's placements + re-check INV-1 under the lock.
		locked, inErr := s.repo.LockEmployeePlacements(ctx, tx, p.EmployeeID)
		if inErr != nil {
			return inErr
		}
		for _, lp := range locked {
			if isActiveLifecycle(lp.LifecycleStatus) {
				return inv1Conflict(lp)
			}
		}

		created, inErr = s.repo.CreatePlacement(ctx, tx, p)
		if inErr != nil {
			// DB partial-unique index is the final backstop.
			if isUniqueViolation(inErr) {
				return apperr.Conflict("INV_1_VIOLATION")
			}
			return inErr
		}

		statusAfter := created.LifecycleStatus
		if inErr := s.repo.InsertPlacementHistory(ctx, tx, PlacementHistoryParams{
			PlacementID:   created.ID,
			Action:        "create",
			ActorUserID:   p.CreatedBy,
			EffectiveDate: &p.StartDate,
			StatusAfter:   &statusAfter,
		}); inErr != nil {
			return inErr
		}

		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("placement.create"),
			EntityType: "placement",
			EntityID:   created.ID,
			After: map[string]any{
				"employee_id":       created.EmployeeID,
				"client_company_id": created.ClientCompanyID,
				"lifecycle_status":  created.LifecycleStatus,
			},
		})
		// TODO(Phase-11 notifications): enqueue NotificationArgs (placement created).
	}); err != nil {
		return domain.Placement{}, asAppErr(err)
	}

	created.Warnings = append(created.Warnings, warnings...)
	// 6. Soft warning: target company has no active leader.
	if s.leader != nil {
		if _, ok, lerr := s.leader.currentLeaderForCompany(ctx, created.ClientCompanyID); lerr == nil && !ok {
			created.Warnings = append(created.Warnings, "NO_SHIFT_LEADER_AT_COMPANY")
		}
	}
	return created, nil
}

// --- backfill agreement ---

// SetAgreement attaches an employment agreement to a previously pending placement
// (awaiting_agreement). Re-runs the BR-1b period validation: out-of-range start →
// 422 PLACEMENT_OUTSIDE_CONTRACT; PKWT end past the agreement end → auto-cap +
// warning. 404 if the placement is missing; 422 if the agreement is not owned by
// the placement's agent. Returns the updated placement (awaiting now false).
func (s *PlacementService) SetAgreement(ctx context.Context, placementID, agreementID string) (domain.Placement, error) {
	cur, err := s.repo.GetPlacementByID(ctx, placementID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Placement{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Placement{}, apperr.Internal(err)
	}
	if isTerminal(cur.LifecycleStatus) {
		return domain.Placement{}, apperr.Conflict("TERMINAL_STATE_IMMUTABLE")
	}

	ag, err := s.repo.GetAgreement(ctx, agreementID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Placement{}, apperr.Rule("RULE_VIOLATION", map[string]string{"agreement_id": "Perjanjian tidak ditemukan."})
	}
	if err != nil {
		return domain.Placement{}, apperr.Internal(err)
	}
	if ag.EmployeeID != cur.EmployeeID {
		return domain.Placement{}, apperr.Rule("RULE_VIOLATION", map[string]string{"agreement_id": "Perjanjian bukan milik karyawan ini."})
	}

	// BR-1b period validation (422 PLACEMENT_OUTSIDE_CONTRACT on out-of-range start).
	if err := validateStartWithinAgreement(ag, cur.StartDate); err != nil {
		return domain.Placement{}, err
	}
	// PKWT auto-cap: clamp the placement end_date to the agreement end.
	endDate := cur.EndDate
	var warnings []string
	if ag.EndDate != nil && endDate != nil && endDate.After(*ag.EndDate) {
		capped := *ag.EndDate
		endDate = &capped
		warnings = append(warnings, "END_DATE_AUTO_CAPPED_TO_AGREEMENT")
	}

	var updated domain.Placement
	agID := agreementID
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetPlacementAgreement(ctx, tx, SetAgreementParams{
			ID:          placementID,
			AgreementID: &agID,
			EndDate:     endDate,
		})
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("placement.set_agreement"),
			EntityType: "placement",
			EntityID:   placementID,
			Before:     map[string]any{"awaiting_agreement": true},
			After:      map[string]any{"agreement_id": agID, "awaiting_agreement": false},
		})
	}); err != nil {
		return domain.Placement{}, asAppErr(err)
	}

	updated.Warnings = append(updated.Warnings, warnings...)
	return updated, nil
}

// --- update ---

// UpdatePlacement edits limited fields; rejects terminal placements.
func (s *PlacementService) UpdatePlacement(ctx context.Context, p UpdatePlacementParams) (domain.Placement, error) {
	cur, err := s.repo.GetPlacementByID(ctx, p.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Placement{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Placement{}, apperr.Internal(err)
	}
	if isTerminal(cur.LifecycleStatus) {
		return domain.Placement{}, apperr.Conflict("TERMINAL_STATE_IMMUTABLE")
	}

	// Default unset fields to current values (PATCH semantics).
	if p.Position == "" {
		p.Position = cur.Position
	}

	var updated domain.Placement
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdatePlacementFields(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		action := "update"
		if inErr := s.repo.InsertPlacementHistory(ctx, tx, PlacementHistoryParams{
			PlacementID: updated.ID,
			Action:      action,
			StatusAfter: &updated.LifecycleStatus,
		}); inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("placement.update"),
			EntityType: "placement",
			EntityID:   updated.ID,
			Before:     map[string]any{"position": cur.Position},
			After:      map[string]any{"position": updated.Position},
		})
	}); err != nil {
		return domain.Placement{}, asAppErr(err)
	}
	return updated, nil
}

// --- lifecycle resolution: end / resign / terminate ---

// EndParams / ResignParams / TerminateParams carry the resolution request fields.
type EndParams struct {
	ID            string
	Reason        string // END_OF_TERM|MUTUAL_AGREEMENT|CLIENT_REQUEST|OTHER
	EffectiveDate time.Time
	Notes         *string
	ActorUserID   *string
}

type ResignParams struct {
	ID          string
	ResignAt    time.Time
	Reason      string
	Notes       *string
	ActorUserID *string
}

type TerminateParams struct {
	ID                  string
	TerminationReason   string
	EffectiveDate       *time.Time
	TypeCompanyNameConf string
	ActorUserID         *string
}

// EndPlacement closes a placement with ended_reason=ENDED.
func (s *PlacementService) EndPlacement(ctx context.Context, p EndParams) (domain.Placement, error) {
	return s.resolve(ctx, p.ID, "ENDED", "ENDED", &p.EffectiveDate, nil, nil, p.Notes, p.ActorUserID, &p.Reason, nil)
}

// ResignPlacement closes a placement with ended_reason=RESIGNED + resign_at.
func (s *PlacementService) ResignPlacement(ctx context.Context, p ResignParams) (domain.Placement, error) {
	if strings.TrimSpace(p.Reason) == "" {
		return domain.Placement{}, apperr.Invalid(map[string]string{"resignation_reason": "Wajib diisi."})
	}
	reason := "RESIGNED"
	return s.resolve(ctx, p.ID, "RESIGNED", reason, &p.ResignAt, nil, &p.ResignAt, p.Notes, p.ActorUserID, &p.Reason, nil)
}

// TerminatePlacement closes a placement with ended_reason=TERMINATED (strong confirm).
func (s *PlacementService) TerminatePlacement(ctx context.Context, p TerminateParams) (domain.Placement, error) {
	if len(strings.TrimSpace(p.TerminationReason)) < 10 {
		return domain.Placement{}, apperr.Invalid(map[string]string{"termination_reason": "Alasan minimal 10 karakter."})
	}
	cur, err := s.repo.GetPlacementByID(ctx, p.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Placement{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Placement{}, apperr.Internal(err)
	}
	if isTerminal(cur.LifecycleStatus) {
		return domain.Placement{}, apperr.Conflict("TERMINAL_STATE_IMMUTABLE")
	}
	// Company-name confirmation (case-insensitive, trimmed).
	company, err := s.repo.GetClientCompany(ctx, cur.ClientCompanyID)
	if err != nil {
		return domain.Placement{}, apperr.Internal(err)
	}
	if !strings.EqualFold(strings.TrimSpace(p.TypeCompanyNameConf), strings.TrimSpace(company.Name)) {
		return domain.Placement{}, apperr.Invalid(map[string]string{"type_company_name_confirm": "Nama perusahaan tidak cocok."})
	}
	eff := s.today()
	if p.EffectiveDate != nil {
		eff = *p.EffectiveDate
	}
	return s.resolveLoaded(ctx, cur, "TERMINATED", "TERMINATED", &eff, &eff, nil, nil, p.ActorUserID, nil, &p.TerminationReason)
}

// resolve loads the placement then delegates to resolveLoaded.
func (s *PlacementService) resolve(ctx context.Context, id, status, endedReason string, effective, endedAt, resignAt *time.Time, notes, actor, reason, terminationReason *string) (domain.Placement, error) {
	cur, err := s.repo.GetPlacementByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Placement{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Placement{}, apperr.Internal(err)
	}
	if isTerminal(cur.LifecycleStatus) {
		return domain.Placement{}, apperr.Conflict("TERMINAL_STATE_IMMUTABLE")
	}
	if status == "ENDED" && endedAt == nil {
		endedAt = effective
	}
	return s.resolveLoaded(ctx, cur, status, endedReason, effective, endedAt, resignAt, notes, actor, reason, terminationReason)
}

// resolveLoaded performs the shared end/resign/terminate write in one tx:
// SetPlacementLifecycle + auto-vacate leadership + history + audit.
func (s *PlacementService) resolveLoaded(ctx context.Context, cur domain.Placement, status, endedReason string, effective, endedAt, resignAt *time.Time, notes, actor, reason, terminationReason *string) (domain.Placement, error) {
	// Scope guard: a lead may only end/resign/terminate placements at its
	// assigned client companies (no-op for super/hr = global). Covers
	// EndPlacement / ResignPlacement / TerminatePlacement, which all funnel here.
	if serr := rbac.GuardCompany(ctx, cur.ClientCompanyID); serr != nil {
		return domain.Placement{}, serr
	}
	before := cur.LifecycleStatus
	var updated domain.Placement
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetPlacementLifecycle(ctx, tx, SetLifecycleParams{
			ID:                cur.ID,
			LifecycleStatus:   status,
			EndedReason:       &endedReason,
			EndedAt:           endedAt,
			TerminationReason: terminationReason,
			ResignAt:          resignAt,
		})
		if inErr != nil {
			return inErr
		}

		// Auto-vacate leadership if the agent led this company (SL-6).
		if s.leader != nil {
			if inErr := s.leader.autoVacateForEmployeeAtCompany(ctx, tx, cur.EmployeeID, cur.ClientCompanyID); inErr != nil {
				return inErr
			}
		}

		if inErr := s.repo.InsertPlacementHistory(ctx, tx, PlacementHistoryParams{
			PlacementID:   cur.ID,
			Action:        strings.ToLower(status),
			ActorUserID:   actor,
			Reason:        reason,
			EffectiveDate: effective,
			StatusBefore:  &before,
			StatusAfter:   &status,
			Notes:         notes,
		}); inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("placement." + strings.ToLower(status)),
			EntityType: "placement",
			EntityID:   cur.ID,
			Before:     map[string]any{"lifecycle_status": before},
			After:      map[string]any{"lifecycle_status": status, "ended_reason": endedReason},
		})
		// TODO(Phase-11 notifications): enqueue NotificationArgs (placement resolved).
	}); err != nil {
		return domain.Placement{}, asAppErr(err)
	}
	return updated, nil
}

// --- transfer ---

// TransferParams carries the transfer request fields.
type TransferParams struct {
	ID                 string
	NewClientCompanyID string
	NewPosition        string // free-text destination position label
	NewStartDate       time.Time
	NewEndDate         *time.Time
	NewAgreementID     *string
	TransferReason     string
	ActorUserID        *string
}

// TransferResult bundles the closed predecessor + new successor + warnings.
type TransferResult struct {
	Predecessor       domain.Placement
	Successor         domain.Placement
	VacatedAssignment *domain.ShiftLeaderAssignment
	Warnings          []string
}

// TransferPlacement atomically closes the source placement (TRANSFERRED) and
// creates the successor at the destination.
func (s *PlacementService) TransferPlacement(ctx context.Context, p TransferParams) (TransferResult, error) {
	cur, err := s.repo.GetPlacementByID(ctx, p.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return TransferResult{}, apperr.NotFound()
	}
	if err != nil {
		return TransferResult{}, apperr.Internal(err)
	}
	if isTerminal(cur.LifecycleStatus) {
		return TransferResult{}, apperr.Conflict("TERMINAL_STATE_IMMUTABLE")
	}
	// Scope guard (source): a lead must own the SOURCE company to move an agent
	// out of it (no-op for super/hr = global).
	if serr := rbac.GuardCompany(ctx, cur.ClientCompanyID); serr != nil {
		return TransferResult{}, serr
	}
	// TR-1: a transfer must move the agent to a different company. A same-company
	// position change is a :renew, not a :transfer (service_line dropped 2026-06-12).
	if p.NewClientCompanyID == cur.ClientCompanyID {
		return TransferResult{}, apperr.Rule("RULE_VIOLATION", map[string]string{"new_client_company_id": "Transfer harus mengubah perusahaan. Gunakan :renew untuk perubahan posisi di perusahaan yang sama."})
	}
	// Destination company ACTIVE.
	destCompany, err := s.repo.GetClientCompany(ctx, p.NewClientCompanyID)
	if errors.Is(err, domain.ErrNotFound) {
		return TransferResult{}, apperr.Invalid(map[string]string{"new_client_company_id": "Perusahaan tujuan tidak ditemukan."})
	}
	if err != nil {
		return TransferResult{}, apperr.Internal(err)
	}
	if !strings.EqualFold(destCompany.Status, "active") {
		return TransferResult{}, apperr.Conflict("COMPANY_INACTIVE")
	}
	// Scope guard (destination): a lead must ALSO own the destination company to
	// move an agent into it (no-op for super/hr = global). Lead owns both ends.
	if serr := rbac.GuardCompany(ctx, p.NewClientCompanyID); serr != nil {
		return TransferResult{}, serr
	}

	// Agreement: default to the predecessor's (may be nil = pending). An explicit
	// new_agreement_id overrides; empty string is ignored. nil propagates cleanly so
	// the successor of a pending placement is itself pending.
	agreementID := cur.AgreementID
	if p.NewAgreementID != nil && *p.NewAgreementID != "" {
		agreementID = p.NewAgreementID
	}
	// A transfer always crosses companies (TR-1), so the successor needs a site of
	// the destination company. The FE transfer modal does not collect a site; reuse
	// the source site id only when it happens to belong to the destination company,
	// otherwise keep the source site as a best-effort default (resolved properly
	// once a destination-site query exists).
	siteID := cur.SiteID
	if site, serr := s.repo.GetSite(ctx, cur.SiteID); serr == nil && site.ClientCompanyID == p.NewClientCompanyID {
		siteID = cur.SiteID
	}

	today := s.today()
	successorStatus := "PENDING_START"
	if !p.NewStartDate.After(today) {
		successorStatus = "ACTIVE"
	}
	// Predecessor ended_at = new_start_date − 1 day (BR-2 buffer).
	endedAt := p.NewStartDate.AddDate(0, 0, -1)
	reason := p.TransferReason

	var result TransferResult
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		before := cur.LifecycleStatus
		// 1. Close predecessor (TRANSFERRED).
		pred, inErr := s.repo.SetPlacementLifecycle(ctx, tx, SetLifecycleParams{
			ID:              cur.ID,
			LifecycleStatus: "TRANSFERRED",
			EndedReason:     strPtr("TRANSFERRED"),
			EndedAt:         &endedAt,
		})
		if inErr != nil {
			return inErr
		}

		// 2. Create successor.
		notesPtr := &reason
		succ, inErr := s.repo.CreatePlacement(ctx, tx, CreatePlacementParams{
			EmployeeID:      cur.EmployeeID,
			AgreementID:     agreementID,
			ClientCompanyID: p.NewClientCompanyID,
			SiteID:          siteID,
			Position:        p.NewPosition,
			StartDate:       p.NewStartDate,
			EndDate:         p.NewEndDate,
			Notes:           notesPtr,
			LifecycleStatus: successorStatus,
			PredecessorID:   &cur.ID,
			CreatedBy:       p.ActorUserID,
		})
		if inErr != nil {
			if isUniqueViolation(inErr) {
				return apperr.Conflict("INV_1_VIOLATION")
			}
			return inErr
		}

		// 3. Backfill predecessor.successor_id.
		if inErr := s.repo.SetPlacementSuccessor(ctx, tx, cur.ID, &succ.ID); inErr != nil {
			return inErr
		}
		pred.SuccessorID = &succ.ID

		// 4. Auto-end old leadership if agent led the old company.
		if s.leader != nil {
			if inErr := s.leader.autoVacateForEmployeeAtCompany(ctx, tx, cur.EmployeeID, cur.ClientCompanyID); inErr != nil {
				return inErr
			}
		}

		// 5. History rows for both + audit.
		if inErr := s.repo.InsertPlacementHistory(ctx, tx, PlacementHistoryParams{
			PlacementID: cur.ID, Action: "transfer_out", ActorUserID: p.ActorUserID,
			Reason: &reason, EffectiveDate: &p.NewStartDate, StatusBefore: &before,
			StatusAfter: strPtr("TRANSFERRED"),
		}); inErr != nil {
			return inErr
		}
		if inErr := s.repo.InsertPlacementHistory(ctx, tx, PlacementHistoryParams{
			PlacementID: succ.ID, Action: "transfer_in", ActorUserID: p.ActorUserID,
			Reason: &reason, EffectiveDate: &p.NewStartDate, StatusAfter: &successorStatus,
		}); inErr != nil {
			return inErr
		}
		if inErr := audit.Record(ctx, tx, audit.Entry{
			Action: audit.Action("placement.transfer"), EntityType: "placement", EntityID: succ.ID,
			Before: map[string]any{"predecessor_id": cur.ID, "predecessor_status": "TRANSFERRED"},
			After:  map[string]any{"successor_id": succ.ID, "client_company_id": succ.ClientCompanyID},
		}); inErr != nil {
			return inErr
		}
		// TODO(Phase-11 notifications): enqueue NotificationArgs (transfer).

		result.Predecessor = pred
		result.Successor = succ
		return nil
	}); err != nil {
		return TransferResult{}, asAppErr(err)
	}

	// Warning: destination has no leader.
	if s.leader != nil {
		if _, ok, lerr := s.leader.currentLeaderForCompany(ctx, p.NewClientCompanyID); lerr == nil && !ok {
			result.Warnings = append(result.Warnings, "NO_SHIFT_LEADER_AT_DESTINATION")
		}
	}
	return result, nil
}

// --- renew ---

// RenewParams carries the renew request fields.
type RenewParams struct {
	ID             string
	NewStartDate   time.Time
	NewEndDate     *time.Time
	NewAgreementID *string
	NewPosition    *string // free-text; nil/"" keeps the predecessor's position
	Notes          *string
	ActorUserID    *string
}

// RenewResult bundles the superseded predecessor + new successor + warnings.
type RenewResult struct {
	Predecessor domain.Placement
	Successor   domain.Placement
	Warnings    []string
}

// RenewPlacement supersedes the predecessor (releasing the partial unique index)
// then creates the successor — same company + position by default.
func (s *PlacementService) RenewPlacement(ctx context.Context, p RenewParams) (RenewResult, error) {
	cur, err := s.repo.GetPlacementByID(ctx, p.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return RenewResult{}, apperr.NotFound()
	}
	if err != nil {
		return RenewResult{}, apperr.Internal(err)
	}
	if isTerminal(cur.LifecycleStatus) {
		return RenewResult{}, apperr.Conflict("TERMINAL_STATE_IMMUTABLE")
	}
	// Scope guard: a lead may only renew placements at its assigned client
	// companies (no-op for super/hr = global). A renewal keeps the same company.
	if serr := rbac.GuardCompany(ctx, cur.ClientCompanyID); serr != nil {
		return RenewResult{}, serr
	}

	// Destination company must still be ACTIVE.
	company, err := s.repo.GetClientCompany(ctx, cur.ClientCompanyID)
	if err != nil {
		return RenewResult{}, apperr.Internal(err)
	}
	if !strings.EqualFold(company.Status, "active") {
		return RenewResult{}, apperr.Conflict("COMPANY_INACTIVE")
	}

	// 1-day buffer (BR-2): new_start_date must be > predecessor.end_date.
	if cur.EndDate != nil && !p.NewStartDate.After(*cur.EndDate) {
		return RenewResult{}, apperr.Rule("PLACEMENT_PERIOD_OVERLAP", map[string]string{"new_start_date": "Tanggal mulai harus setelah penempatan sebelumnya berakhir."})
	}

	// Agreement: default to the predecessor's (may be nil = pending). nil propagates
	// so the renewal of a pending placement stays pending.
	agreementID := cur.AgreementID
	if p.NewAgreementID != nil && *p.NewAgreementID != "" {
		agreementID = p.NewAgreementID
	}
	position := cur.Position
	if p.NewPosition != nil && *p.NewPosition != "" {
		position = *p.NewPosition
	}

	// PKWT auto-cap on new_end_date — ONLY when an agreement is present.
	var warnings []string
	endDate := p.NewEndDate
	if agreementID != nil {
		if ag, agErr := s.repo.GetAgreement(ctx, *agreementID); agErr == nil {
			if ag.EndDate != nil && endDate != nil && endDate.After(*ag.EndDate) {
				capped := *ag.EndDate
				endDate = &capped
				warnings = append(warnings, "END_DATE_AUTO_CAPPED_TO_AGREEMENT")
			}
		}
	}

	today := s.today()
	successorStatus := "PENDING_START"
	if !p.NewStartDate.After(today) {
		successorStatus = "ACTIVE"
	}

	var result RenewResult
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		before := cur.LifecycleStatus
		// Supersede predecessor FIRST (release the index), effective successor start.
		pred, inErr := s.repo.SetPlacementLifecycle(ctx, tx, SetLifecycleParams{
			ID:              cur.ID,
			LifecycleStatus: "SUPERSEDED",
			EndedReason:     strPtr("SUPERSEDED"),
			EndedAt:         &p.NewStartDate,
		})
		if inErr != nil {
			return inErr
		}

		succ, inErr := s.repo.CreatePlacement(ctx, tx, CreatePlacementParams{
			EmployeeID:      cur.EmployeeID,
			AgreementID:     agreementID,
			ClientCompanyID: cur.ClientCompanyID,
			SiteID:          cur.SiteID,
			Position:        position,
			StartDate:       p.NewStartDate,
			EndDate:         endDate,
			Notes:           p.Notes,
			LifecycleStatus: successorStatus,
			PredecessorID:   &cur.ID,
			CreatedBy:       p.ActorUserID,
		})
		if inErr != nil {
			if isUniqueViolation(inErr) {
				return apperr.Conflict("INV_1_VIOLATION")
			}
			return inErr
		}

		if inErr := s.repo.SetPlacementSuccessor(ctx, tx, cur.ID, &succ.ID); inErr != nil {
			return inErr
		}
		pred.SuccessorID = &succ.ID

		if inErr := s.repo.InsertPlacementHistory(ctx, tx, PlacementHistoryParams{
			PlacementID: cur.ID, Action: "renew_out", ActorUserID: p.ActorUserID,
			EffectiveDate: &p.NewStartDate, StatusBefore: &before, StatusAfter: strPtr("SUPERSEDED"), Notes: p.Notes,
		}); inErr != nil {
			return inErr
		}
		if inErr := s.repo.InsertPlacementHistory(ctx, tx, PlacementHistoryParams{
			PlacementID: succ.ID, Action: "renew_in", ActorUserID: p.ActorUserID,
			EffectiveDate: &p.NewStartDate, StatusAfter: &successorStatus, Notes: p.Notes,
		}); inErr != nil {
			return inErr
		}
		if inErr := audit.Record(ctx, tx, audit.Entry{
			Action: audit.Action("placement.renew"), EntityType: "placement", EntityID: succ.ID,
			Before: map[string]any{"predecessor_id": cur.ID, "predecessor_status": "SUPERSEDED"},
			After:  map[string]any{"successor_id": succ.ID},
		}); inErr != nil {
			return inErr
		}
		// TODO(Phase-11 notifications): enqueue NotificationArgs (renew).

		result.Predecessor = pred
		result.Successor = succ
		return nil
	}); err != nil {
		return RenewResult{}, asAppErr(err)
	}
	result.Warnings = warnings
	return result, nil
}

// --- helpers ---

func inv1Conflict(existing domain.Placement) error {
	sum := toPlacementSummary(existing)
	return apperr.ConflictWithDetails("INV_1_VIOLATION",
		map[string]string{"employee_id": "Sudah memiliki penempatan aktif."},
		INVViolationDetails{
			Invariant:        "INV_1",
			CurrentPlacement: &sum,
			SuggestedActions: []string{"transfer", "end"},
		})
}

func isActiveLifecycle(status string) bool {
	switch status {
	case "ACTIVE", "EXPIRING", "PENDING_START", "SCHEDULED":
		return true
	}
	return false
}

// validateStartWithinAgreement rejects a start_date before the agreement starts
// (BR-1b out-of-range start). PKWT end overflow is auto-capped by the caller.
func validateStartWithinAgreement(ag AgreementRef, start time.Time) error {
	if start.Before(ag.StartDate) {
		return apperr.Rule("PLACEMENT_OUTSIDE_CONTRACT", map[string]string{
			"start_date": "Sebelum perjanjian dimulai.",
		})
	}
	if ag.EndDate != nil && start.After(*ag.EndDate) {
		return apperr.Rule("PLACEMENT_OUTSIDE_CONTRACT", map[string]string{
			"start_date": "Setelah perjanjian berakhir.",
		})
	}
	return nil
}

func strPtr(s string) *string { return &s }

// asAppErr passes *apperr.Error through, wrapping anything else as 500.
func asAppErr(err error) error {
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.Internal(err)
}
