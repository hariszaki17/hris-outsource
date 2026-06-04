// Package placement_test — E3 shift-leader assignment contract tests.
//
// Asserts INV-2/3/4 409 envelopes (with error.details.invariant + the right
// detail object), ALREADY_ENDED, LEADER_NOT_ELIGIBLE, the PENDING_START → INV-4
// case (C-2), and the SITE-SCOPE leadership path that the FE E2E does not
// exercise (per-site leadership unit). Drift gate vs docs/api/E3-placement/openapi.yaml.
package placement_test

import (
	"context"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/placement"
)

// ---------------------------------------------------------------------------
// fakeShiftLeaderRepo — in-memory svc.ShiftLeaderRepository.
//
// It shares placement state with the placement repo via the `placements` pointer
// so INV-4 (employee actively placed at company) and current-leader joins resolve
// off the same fixtures.
// ---------------------------------------------------------------------------

type fakeShiftLeaderRepo struct {
	assignments map[string]domain.ShiftLeaderAssignment
	companies   map[string]svc.CompanyRef
	employees   map[string]domain.Employee
	placements  *fakePlacementRepo
	seq         int

	// roster fixtures (used by roster tests).
	rosterSummary domain.CompanyRosterSummary
}

func newFakeShiftLeaderRepo() *fakeShiftLeaderRepo {
	return &fakeShiftLeaderRepo{
		assignments: map[string]domain.ShiftLeaderAssignment{},
		companies:   map[string]svc.CompanyRef{},
		employees:   map[string]domain.Employee{},
	}
}

func (r *fakeShiftLeaderRepo) GetClientCompany(_ context.Context, id string) (svc.CompanyRef, error) {
	if c, ok := r.companies[id]; ok {
		return c, nil
	}
	if r.placements != nil {
		if c, ok := r.placements.companies[id]; ok {
			return c, nil
		}
	}
	return svc.CompanyRef{}, domain.ErrNotFound
}

func (r *fakeShiftLeaderRepo) GetEmployeeByID(_ context.Context, id string) (domain.Employee, error) {
	if e, ok := r.employees[id]; ok {
		return e, nil
	}
	if r.placements != nil {
		if e, ok := r.placements.employees[id]; ok {
			return e, nil
		}
	}
	return domain.Employee{}, domain.ErrNotFound
}

func (r *fakeShiftLeaderRepo) activeLeaderForCompany(companyID string, siteID *string) (domain.ShiftLeaderAssignment, bool) {
	for _, a := range r.assignments {
		if !a.Active() || a.ClientCompanyID != companyID {
			continue
		}
		if siteID == nil {
			if a.SiteID == nil {
				return a, true
			}
			continue
		}
		if a.SiteID != nil && *a.SiteID == *siteID {
			return a, true
		}
	}
	return domain.ShiftLeaderAssignment{}, false
}

func (r *fakeShiftLeaderRepo) GetCurrentLeaderForCompany(_ context.Context, companyID string) (domain.ShiftLeaderAssignment, error) {
	if a, ok := r.activeLeaderForCompany(companyID, nil); ok {
		return a, nil
	}
	// also match site-scoped active leaders for the company (any site).
	for _, a := range r.assignments {
		if a.Active() && a.ClientCompanyID == companyID {
			return a, nil
		}
	}
	return domain.ShiftLeaderAssignment{}, domain.ErrNotFound
}

func (r *fakeShiftLeaderRepo) GetAssignmentByID(_ context.Context, id string) (domain.ShiftLeaderAssignment, error) {
	if a, ok := r.assignments[id]; ok {
		return a, nil
	}
	return domain.ShiftLeaderAssignment{}, domain.ErrNotFound
}

func (r *fakeShiftLeaderRepo) GetActiveLeaderForCompanyForUpdate(_ context.Context, _ pgx.Tx, companyID string) (domain.ShiftLeaderAssignment, error) {
	if a, ok := r.activeLeaderForCompany(companyID, nil); ok {
		return a, nil
	}
	return domain.ShiftLeaderAssignment{}, domain.ErrNotFound
}

func (r *fakeShiftLeaderRepo) GetActiveLeaderForSiteForUpdate(_ context.Context, _ pgx.Tx, siteID string) (domain.ShiftLeaderAssignment, error) {
	for _, a := range r.assignments {
		if a.Active() && a.SiteID != nil && *a.SiteID == siteID {
			return a, nil
		}
	}
	return domain.ShiftLeaderAssignment{}, domain.ErrNotFound
}

func (r *fakeShiftLeaderRepo) GetActiveAssignmentForEmployeeForUpdate(_ context.Context, _ pgx.Tx, employeeID string) (domain.ShiftLeaderAssignment, error) {
	for _, a := range r.assignments {
		if a.Active() && a.EmployeeID == employeeID {
			return a, nil
		}
	}
	return domain.ShiftLeaderAssignment{}, domain.ErrNotFound
}

func (r *fakeShiftLeaderRepo) GetActivePlacementForEmployeeAtCompany(_ context.Context, _ pgx.Tx, employeeID, companyID string) (domain.Placement, error) {
	if r.placements == nil {
		return domain.Placement{}, domain.ErrNotFound
	}
	// Return PENDING_START too so the service can detect C-2 (PENDING_START fails INV-4).
	var pendingMatch domain.Placement
	havePending := false
	for _, p := range r.placements.placements {
		if p.EmployeeID != employeeID || p.ClientCompanyID != companyID {
			continue
		}
		if p.LifecycleStatus == "ACTIVE" || p.LifecycleStatus == "EXPIRING" {
			return p, nil
		}
		if p.LifecycleStatus == "PENDING_START" {
			pendingMatch = p
			havePending = true
		}
	}
	if havePending {
		return pendingMatch, nil
	}
	return domain.Placement{}, domain.ErrNotFound
}

func (r *fakeShiftLeaderRepo) CreateAssignment(_ context.Context, _ pgx.Tx, p svc.CreateAssignmentParams) (domain.ShiftLeaderAssignment, error) {
	r.seq++
	id := "SWP-SLA-" + itoa(3000+r.seq)
	now := fixedNow
	comp, _ := r.GetClientCompany(context.Background(), p.ClientCompanyID)
	a := domain.ShiftLeaderAssignment{
		ID:                id,
		ClientCompanyID:   p.ClientCompanyID,
		SiteID:            p.SiteID,
		EmployeeID:        p.EmployeeID,
		AssignedAt:        now,
		AssignedBy:        p.AssignedBy,
		Notes:             p.Notes,
		CreatedAt:         now,
		UpdatedAt:         now,
		ClientCompanyName: strp(comp.Name),
	}
	r.assignments[id] = a
	return a, nil
}

func (r *fakeShiftLeaderRepo) EndAssignment(_ context.Context, _ pgx.Tx, id string, vacatedReason *string) (domain.ShiftLeaderAssignment, error) {
	a, ok := r.assignments[id]
	if !ok {
		return domain.ShiftLeaderAssignment{}, domain.ErrNotFound
	}
	t := fixedNow
	a.UnassignedAt = &t
	a.VacatedReason = vacatedReason
	a.UpdatedAt = t
	r.assignments[id] = a
	return a, nil
}

func (r *fakeShiftLeaderRepo) RosterForCompany(_ context.Context, f domain.PlacementFilter) ([]domain.Placement, error) {
	if r.placements == nil {
		return nil, nil
	}
	var out []domain.Placement
	terminal := map[string]bool{"ENDED": true, "TERMINATED": true, "RESIGNED": true, "TRANSFERRED": true, "SUPERSEDED": true}
	for _, p := range r.placements.placements {
		if f.CompanyID != nil && p.ClientCompanyID != *f.CompanyID {
			continue
		}
		if !f.IncludeHistory && terminal[p.LifecycleStatus] {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *fakeShiftLeaderRepo) RosterSummary(_ context.Context, _ string) (domain.CompanyRosterSummary, error) {
	return r.rosterSummary, nil
}

var _ svc.ShiftLeaderRepository = (*fakeShiftLeaderRepo)(nil)

// ---------------------------------------------------------------------------
// seed helpers for SLA tests (operate on the shared leader repo)
// ---------------------------------------------------------------------------

func (h *placementHarness) seedActivePlacementWithLine(id, empID, companyID, siteID, line, status string) {
	end := jktDate(2027, 6, 30)
	h.seedPlacement(domain.Placement{
		ID: id, EmployeeID: empID, ClientCompanyID: companyID,
		SiteID: siteID, ServiceLineID: line, PositionID: "SWP-POS-014",
		AgreementID: "SWP-AG-7002", StartDate: jktDate(2026, 1, 1), EndDate: &end,
		LifecycleStatus: status, EmployeeName: strp("Emp " + empID),
	})
}

func (h *placementHarness) seedLeaderCompany(id, name, scope string) {
	h.repo.companies[id] = svc.CompanyRef{ID: id, Name: name, Status: "active", LeaderScope: scope}
}

func (h *placementHarness) seedAssignment(a domain.ShiftLeaderAssignment) {
	leaderRepo := h.leaderRepo()
	if a.AssignedAt.IsZero() {
		a.AssignedAt = fixedNow
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = fixedNow
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = fixedNow
	}
	leaderRepo.assignments[a.ID] = a
}

// ---------------------------------------------------------------------------
// Tests: POST /shift-leader-assignments (company-scope)
// ---------------------------------------------------------------------------

func TestCreateSLA_FirstLeader_201(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedLeaderCompany("SWP-CMP-0021", "Plaza Senayan", "company")
	h.repo.employees["SWP-EMP-1108"] = domain.Employee{ID: "SWP-EMP-1108", FullName: "Rudi", Status: "active"}
	h.seedActivePlacementWithLine("SWP-PL-5001", "SWP-EMP-1108", "SWP-CMP-0021", "SWP-SITE-0001", "SWP-SVC-001", "ACTIVE")

	rr := h.doJSON("POST", "/shift-leader-assignments", map[string]any{
		"client_company_id": "SWP-CMP-0021",
		"employee_id":       "SWP-EMP-1108",
		"start_date":        "2026-06-03",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	a, ok := body["assignment"].(map[string]any)
	if !ok {
		t.Fatalf("assignment missing: %T", body["assignment"])
	}
	for _, k := range []string{"id", "client_company_id", "employee_id", "active", "assigned_at"} {
		if _, ok := a[k]; !ok {
			t.Errorf("assignment missing key: %s", k)
		}
	}
	if a["active"] != true {
		t.Errorf("assignment.active = %v, want true", a["active"])
	}
	if a["client_company_id"] != "SWP-CMP-0021" {
		t.Errorf("assignment.client_company_id = %v", a["client_company_id"])
	}
}

func TestCreateSLA_SecondLeaderNoReplace_INV2_409(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedLeaderCompany("SWP-CMP-0021", "Plaza Senayan", "company")
	// Existing active leader (Rudi).
	h.repo.employees["SWP-EMP-1108"] = domain.Employee{ID: "SWP-EMP-1108", FullName: "Rudi", Status: "active"}
	h.seedActivePlacementWithLine("SWP-PL-5001", "SWP-EMP-1108", "SWP-CMP-0021", "SWP-SITE-0001", "SWP-SVC-001", "ACTIVE")
	h.seedAssignment(domain.ShiftLeaderAssignment{
		ID: "SWP-SLA-3001", ClientCompanyID: "SWP-CMP-0021", EmployeeID: "SWP-EMP-1108",
		EmployeeName: strp("Rudi"),
	})
	// New candidate Budi (also actively placed there).
	h.repo.employees["SWP-EMP-2891"] = domain.Employee{ID: "SWP-EMP-2891", FullName: "Budi", Status: "active"}
	h.seedActivePlacementWithLine("SWP-PL-5002", "SWP-EMP-2891", "SWP-CMP-0021", "SWP-SITE-0001", "SWP-SVC-001", "ACTIVE")

	rr := h.doJSON("POST", "/shift-leader-assignments", map[string]any{
		"client_company_id": "SWP-CMP-0021",
		"employee_id":       "SWP-EMP-2891",
		"start_date":        "2026-06-03",
		"replace":           false,
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 INV_2_VIOLATION, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "INV_2_VIOLATION" {
		t.Errorf("error.code = %v, want INV_2_VIOLATION", e["code"])
	}
	details, _ := e["details"].(map[string]any)
	if details["invariant"] != "INV_2" {
		t.Errorf("details.invariant = %v, want INV_2", details["invariant"])
	}
	if _, ok := details["current_assignment"].(map[string]any); !ok {
		t.Error("details.current_assignment missing")
	}
	if !containsAny(details["suggested_actions"].([]any), "replace") {
		t.Error("suggested_actions should contain replace")
	}
}

func TestCreateSLA_Replace_201_ReplacedAssignment(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedLeaderCompany("SWP-CMP-0021", "Plaza Senayan", "company")
	h.repo.employees["SWP-EMP-1108"] = domain.Employee{ID: "SWP-EMP-1108", FullName: "Rudi", Status: "active"}
	h.seedActivePlacementWithLine("SWP-PL-5001", "SWP-EMP-1108", "SWP-CMP-0021", "SWP-SITE-0001", "SWP-SVC-001", "ACTIVE")
	h.seedAssignment(domain.ShiftLeaderAssignment{
		ID: "SWP-SLA-3001", ClientCompanyID: "SWP-CMP-0021", EmployeeID: "SWP-EMP-1108",
	})
	h.repo.employees["SWP-EMP-2891"] = domain.Employee{ID: "SWP-EMP-2891", FullName: "Budi", Status: "active"}
	h.seedActivePlacementWithLine("SWP-PL-5002", "SWP-EMP-2891", "SWP-CMP-0021", "SWP-SITE-0001", "SWP-SVC-001", "ACTIVE")

	rr := h.doJSON("POST", "/shift-leader-assignments", map[string]any{
		"client_company_id": "SWP-CMP-0021",
		"employee_id":       "SWP-EMP-2891",
		"start_date":        "2026-06-03",
		"replace":           true,
		"replace_reason":    "Rotasi.",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	rep, ok := body["replaced_assignment"].(map[string]any)
	if !ok {
		t.Fatalf("replaced_assignment missing: %T", body["replaced_assignment"])
	}
	if rep["id"] != "SWP-SLA-3001" {
		t.Errorf("replaced_assignment.id = %v, want SWP-SLA-3001", rep["id"])
	}
	if rep["vacated_reason"] != "REASSIGNED" {
		t.Errorf("replaced_assignment.vacated_reason = %v, want REASSIGNED", rep["vacated_reason"])
	}
	if rep["active"] != false {
		t.Errorf("replaced_assignment.active = %v, want false", rep["active"])
	}
}

func TestCreateSLA_EmployeeLeadsAnother_INV3_409(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedLeaderCompany("SWP-CMP-0021", "Plaza Senayan", "company")
	h.seedLeaderCompany("SWP-CMP-0009", "Mall Kelapa Gading", "company")
	h.repo.employees["SWP-EMP-1042"] = domain.Employee{ID: "SWP-EMP-1042", FullName: "Sari", Status: "active"}
	// Sari is actively placed at 0021 (so INV-4 holds) ...
	h.seedActivePlacementWithLine("SWP-PL-5003", "SWP-EMP-1042", "SWP-CMP-0021", "SWP-SITE-0001", "SWP-SVC-002", "ACTIVE")
	// ... but already leads 0009.
	h.seedAssignment(domain.ShiftLeaderAssignment{
		ID: "SWP-SLA-OTHER", ClientCompanyID: "SWP-CMP-0009", EmployeeID: "SWP-EMP-1042",
		ClientCompanyName: strp("Mall Kelapa Gading"),
	})

	rr := h.doJSON("POST", "/shift-leader-assignments", map[string]any{
		"client_company_id": "SWP-CMP-0021",
		"employee_id":       "SWP-EMP-1042",
		"start_date":        "2026-06-03",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 INV_3_VIOLATION, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "INV_3_VIOLATION" {
		t.Errorf("error.code = %v, want INV_3_VIOLATION", e["code"])
	}
	details, _ := e["details"].(map[string]any)
	if _, ok := details["existing_assignment"].(map[string]any); !ok {
		t.Error("details.existing_assignment missing for INV-3")
	}
}

func TestCreateSLA_NotPlacedAtCompany_INV4_409(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedLeaderCompany("SWP-CMP-0021", "Plaza Senayan", "company")
	h.repo.employees["SWP-EMP-9999"] = domain.Employee{ID: "SWP-EMP-9999", FullName: "Nomad", Status: "active"}
	// No placement at SWP-CMP-0021.

	rr := h.doJSON("POST", "/shift-leader-assignments", map[string]any{
		"client_company_id": "SWP-CMP-0021",
		"employee_id":       "SWP-EMP-9999",
		"start_date":        "2026-06-03",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 INV_4_VIOLATION, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "INV_4_VIOLATION" {
		t.Errorf("error.code = %v, want INV_4_VIOLATION", e["code"])
	}
	details, _ := e["details"].(map[string]any)
	if details["company_id"] != "SWP-CMP-0021" {
		t.Errorf("details.company_id = %v, want SWP-CMP-0021", details["company_id"])
	}
	if details["employee_id"] != "SWP-EMP-9999" {
		t.Errorf("details.employee_id = %v, want SWP-EMP-9999", details["employee_id"])
	}
	// With no placement at all, employee_placements_at_company is empty (omitempty) —
	// the PENDING_START case (C-2) below proves the populated array. Here we assert
	// suggested_actions guides HR to place the agent first.
	if !containsAny(details["suggested_actions"].([]any), "assign_after_placement") {
		t.Errorf("suggested_actions = %v, want assign_after_placement for INV-4", details["suggested_actions"])
	}
}

func TestCreateSLA_PendingStartFailsINV4_409_C2(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedLeaderCompany("SWP-CMP-0021", "Plaza Senayan", "company")
	h.repo.employees["SWP-EMP-7000"] = domain.Employee{ID: "SWP-EMP-7000", FullName: "Future", Status: "active"}
	// Only a PENDING_START placement at the company — does NOT satisfy INV-4 (C-2).
	h.seedActivePlacementWithLine("SWP-PL-PEND", "SWP-EMP-7000", "SWP-CMP-0021", "SWP-SITE-0001", "SWP-SVC-001", "PENDING_START")

	rr := h.doJSON("POST", "/shift-leader-assignments", map[string]any{
		"client_company_id": "SWP-CMP-0021",
		"employee_id":       "SWP-EMP-7000",
		"start_date":        "2026-06-03",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 INV_4_VIOLATION (PENDING_START fails C-2), got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "INV_4_VIOLATION" {
		t.Errorf("error.code = %v, want INV_4_VIOLATION", e["code"])
	}
	details, _ := e["details"].(map[string]any)
	// The PENDING_START placement should be surfaced as the (insufficient) evidence.
	pls, ok := details["employee_placements_at_company"].([]any)
	if !ok || len(pls) == 0 {
		t.Fatalf("expected employee_placements_at_company to include the PENDING_START placement, got %v", details["employee_placements_at_company"])
	}
	if pls[0].(map[string]any)["lifecycle_status"] != "PENDING_START" {
		t.Errorf("surfaced placement status = %v, want PENDING_START", pls[0].(map[string]any)["lifecycle_status"])
	}
}

func TestCreateSLA_InactiveEmployee_LeaderNotEligible_422(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedLeaderCompany("SWP-CMP-0021", "Plaza Senayan", "company")
	h.repo.employees["SWP-EMP-INACT"] = domain.Employee{ID: "SWP-EMP-INACT", FullName: "Inactive", Status: "inactive"}

	rr := h.doJSON("POST", "/shift-leader-assignments", map[string]any{
		"client_company_id": "SWP-CMP-0021",
		"employee_id":       "SWP-EMP-INACT",
		"start_date":        "2026-06-03",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 LEADER_NOT_ELIGIBLE, got %d: %s", rr.Code, rr.Body.String())
	}
	if errObject(t, decodeBody(t, rr))["code"] != "LEADER_NOT_ELIGIBLE" {
		t.Errorf("error.code != LEADER_NOT_ELIGIBLE")
	}
}

// ---------------------------------------------------------------------------
// Tests: SITE-SCOPE leadership path (contract-only; FE E2E skips it)
// ---------------------------------------------------------------------------

func TestCreateSLA_SiteScope_DifferentSiteSucceeds(t *testing.T) {
	h := newPlacementHarness(t)
	// Company is site-scoped.
	h.seedLeaderCompany("SWP-CMP-SITE", "Multi-Site Co", "site")

	// Site A already has an active leader (site-scoped assignment).
	siteA := "SWP-SITE-A"
	h.seedAssignment(domain.ShiftLeaderAssignment{
		ID: "SWP-SLA-SITEA", ClientCompanyID: "SWP-CMP-SITE", SiteID: &siteA,
		EmployeeID: "SWP-EMP-LEADA",
	})

	// New leader for a DIFFERENT site B of the same company. The candidate is
	// actively placed at the company → INV-4 holds; per-site unit B is vacant.
	h.repo.employees["SWP-EMP-LEADB"] = domain.Employee{ID: "SWP-EMP-LEADB", FullName: "Leader B", Status: "active"}
	h.seedActivePlacementWithLine("SWP-PL-SITEB", "SWP-EMP-LEADB", "SWP-CMP-SITE", "SWP-SITE-B", "SWP-SVC-001", "ACTIVE")

	// Note: the FE/service company-scope assign path uses company-level locks; the
	// site-scope unit is proven distinct because assigning a leader to the company
	// while site A is led (site-scoped, SiteID != nil) does NOT collide on the
	// company-level active-leader check (which only matches SiteID == nil).
	rr := h.doJSON("POST", "/shift-leader-assignments", map[string]any{
		"client_company_id": "SWP-CMP-SITE",
		"employee_id":       "SWP-EMP-LEADB",
		"start_date":        "2026-06-03",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 (site B leader, distinct per-site unit from site A), got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	a, _ := body["assignment"].(map[string]any)
	if a["client_company_id"] != "SWP-CMP-SITE" {
		t.Errorf("assignment.client_company_id = %v", a["client_company_id"])
	}
	// Both leaders are now active (site A + the new one) — per-site leadership unit
	// allows a leader per site of the same company.
	leaderRepo := h.leaderRepo()
	active := 0
	for _, asg := range leaderRepo.assignments {
		if asg.Active() && asg.ClientCompanyID == "SWP-CMP-SITE" {
			active++
		}
	}
	if active < 2 {
		t.Errorf("expected >=2 active leaders across distinct sites of the same company, got %d", active)
	}
}

// ---------------------------------------------------------------------------
// Tests: :replace and :end
// ---------------------------------------------------------------------------

func TestReplaceSLA_AlreadyEnded_409(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedLeaderCompany("SWP-CMP-0021", "Plaza Senayan", "company")
	ended := fixedNow
	h.seedAssignment(domain.ShiftLeaderAssignment{
		ID: "SWP-SLA-ENDED", ClientCompanyID: "SWP-CMP-0021", EmployeeID: "SWP-EMP-1108",
		UnassignedAt: &ended, VacatedReason: strp("MANUAL"),
	})
	h.repo.employees["SWP-EMP-2891"] = domain.Employee{ID: "SWP-EMP-2891", FullName: "Budi", Status: "active"}

	rr := h.doJSON("POST", "/shift-leader-assignments/SWP-SLA-ENDED:replace", map[string]any{
		"new_employee_id": "SWP-EMP-2891",
		"start_date":      "2026-06-03",
		"replace_reason":  "swap",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 ALREADY_ENDED, got %d: %s", rr.Code, rr.Body.String())
	}
	if errObject(t, decodeBody(t, rr))["code"] != "ALREADY_ENDED" {
		t.Errorf("error.code != ALREADY_ENDED")
	}
}

func TestEndSLA_200_ThenAlreadyEnded_409(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedLeaderCompany("SWP-CMP-0021", "Plaza Senayan", "company")
	h.seedAssignment(domain.ShiftLeaderAssignment{
		ID: "SWP-SLA-3001", ClientCompanyID: "SWP-CMP-0021", EmployeeID: "SWP-EMP-1108",
	})

	rr := h.doJSON("POST", "/shift-leader-assignments/SWP-SLA-3001:end", map[string]any{
		"reason": "Rotasi struktur.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["active"] != false {
		t.Errorf("active = %v, want false", body["active"])
	}
	if body["unassigned_at"] == nil {
		t.Error("unassigned_at should be set after :end")
	}

	// Ending again → 409 ALREADY_ENDED.
	rr2 := h.doJSON("POST", "/shift-leader-assignments/SWP-SLA-3001:end", map[string]any{})
	if rr2.Code != http.StatusConflict {
		t.Fatalf("expected 409 ALREADY_ENDED on re-end, got %d: %s", rr2.Code, rr2.Body.String())
	}
	if errObject(t, decodeBody(t, rr2))["code"] != "ALREADY_ENDED" {
		t.Errorf("re-end error.code != ALREADY_ENDED")
	}
}

// silence unused.
var _ = time.Now
