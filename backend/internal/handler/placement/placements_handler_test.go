// Package placement_test — E3 placement CRUD + lifecycle contract tests.
//
// Asserts the placement endpoint shapes, status codes, error envelopes, and
// invariant 409 details match docs/api/E3-placement/openapi.yaml EXACTLY. This is
// the drift gate that replaces server-side codegen (the FE Orval client is
// generated from the same spec).
//
// Pattern mirrors internal/handler/people/agreements_handler_test.go: an
// in-memory fakePlacementRepo implementing svc.PlacementRepository + a fakeTx
// (Exec no-op so audit.Record works inside InTx), mounted on a chi.Router with a
// mutable principal middleware to swap roles per case.
package placement_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	placementhandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/placement"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/placement"
)

// ---------------------------------------------------------------------------
// fakeTx — only Exec is needed (audit.Record); all other methods panic.
// ---------------------------------------------------------------------------

type fakeTx struct{}

func (f *fakeTx) Begin(_ context.Context) (pgx.Tx, error) { return f, nil }
func (f *fakeTx) Commit(_ context.Context) error          { return nil }
func (f *fakeTx) Rollback(_ context.Context) error        { return nil }
func (f *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	panic("fakeTx: Query not implemented")
}
func (f *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("fakeTx: QueryRow not implemented")
}
func (f *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	panic("fakeTx: CopyFrom not implemented")
}
func (f *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults {
	panic("fakeTx: SendBatch not implemented")
}
func (f *fakeTx) LargeObjects() pgx.LargeObjects {
	panic("fakeTx: LargeObjects not implemented")
}
func (f *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	panic("fakeTx: Prepare not implemented")
}
func (f *fakeTx) Conn() *pgx.Conn { return nil }

var _ pgx.Tx = (*fakeTx)(nil)

type fakeTxRunner struct{}

func (f *fakeTxRunner) InTx(_ context.Context, fn func(pgx.Tx) error) error {
	return fn(&fakeTx{})
}

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, rr.Body.String())
	}
	return m
}

func itoa(n int) string { return strconv.Itoa(n) }

func errObject(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	e, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("response has no error object: %v", body)
	}
	return e
}

func ymd(t time.Time) string { return t.Format("2006-01-02") }

// fixedNow is the deterministic clock for all placement tests (Asia/Jakarta-safe).
var fixedNow = time.Date(2026, 6, 4, 5, 0, 0, 0, time.UTC) // 12:00 WIB on 2026-06-04

func jktDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// fakePlacementRepo — in-memory svc.PlacementRepository.
// ---------------------------------------------------------------------------

type fakePlacementRepo struct {
	placements map[string]domain.Placement
	employees  map[string]domain.Employee
	companies  map[string]svc.CompanyRef
	sites      map[string]svc.SiteRef
	agreements map[string]svc.AgreementRef
	seq        int

	// captured filters for assertion (search + status passthrough).
	lastListFilter domain.PlacementFilter
}

func newFakePlacementRepo() *fakePlacementRepo {
	return &fakePlacementRepo{
		placements: map[string]domain.Placement{},
		employees:  map[string]domain.Employee{},
		companies:  map[string]svc.CompanyRef{},
		sites:      map[string]svc.SiteRef{},
		agreements: map[string]svc.AgreementRef{},
	}
}

func (r *fakePlacementRepo) ListPlacements(_ context.Context, f domain.PlacementFilter) ([]domain.Placement, error) {
	r.lastListFilter = f
	var out []domain.Placement
	for _, p := range r.placements {
		if f.Status != nil && p.LifecycleStatus != *f.Status {
			continue
		}
		if len(f.StatusIn) > 0 && !contains(f.StatusIn, p.LifecycleStatus) {
			continue
		}
		if f.CompanyID != nil && p.ClientCompanyID != *f.CompanyID {
			continue
		}
		if f.EmployeeID != nil && p.EmployeeID != *f.EmployeeID {
			continue
		}
		if f.Q != nil && p.EmployeeName != nil && !containsFold(*p.EmployeeName, *f.Q) {
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

func (r *fakePlacementRepo) ListExpiringPlacements(_ context.Context, f domain.ExpiringFilter) ([]domain.Placement, error) {
	var out []domain.Placement
	for _, p := range r.placements {
		// Only ACTIVE/EXPIRING with an end_date within the cutoff window.
		if p.LifecycleStatus != "ACTIVE" && p.LifecycleStatus != "EXPIRING" {
			continue
		}
		if p.EndDate == nil || p.EndDate.After(f.Cutoff) {
			continue
		}
		if f.CompanyID != nil && p.ClientCompanyID != *f.CompanyID {
			continue
		}
		out = append(out, p)
	}
	// end_date ascending.
	sort.Slice(out, func(i, j int) bool {
		if out[i].EndDate.Equal(*out[j].EndDate) {
			return out[i].ID < out[j].ID
		}
		return out[i].EndDate.Before(*out[j].EndDate)
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *fakePlacementRepo) GetPlacementByID(_ context.Context, id string) (domain.Placement, error) {
	p, ok := r.placements[id]
	if !ok {
		return domain.Placement{}, domain.ErrNotFound
	}
	return p, nil
}

func (r *fakePlacementRepo) GetPlacementChain(_ context.Context, id string) ([]domain.Placement, error) {
	if p, ok := r.placements[id]; ok {
		return []domain.Placement{p}, nil
	}
	return nil, nil
}

func (r *fakePlacementRepo) GetActivePlacementForEmployee(_ context.Context, employeeID string) (domain.Placement, error) {
	for _, p := range r.placements {
		if p.EmployeeID == employeeID && isActive(p.LifecycleStatus) {
			return p, nil
		}
	}
	return domain.Placement{}, domain.ErrNotFound
}

func (r *fakePlacementRepo) GetEmployeeByID(_ context.Context, id string) (domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	return e, nil
}

func (r *fakePlacementRepo) GetClientCompany(_ context.Context, id string) (svc.CompanyRef, error) {
	c, ok := r.companies[id]
	if !ok {
		return svc.CompanyRef{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *fakePlacementRepo) GetSite(_ context.Context, id string) (svc.SiteRef, error) {
	s, ok := r.sites[id]
	if !ok {
		return svc.SiteRef{}, domain.ErrNotFound
	}
	return s, nil
}

func (r *fakePlacementRepo) GetAgreement(_ context.Context, id string) (svc.AgreementRef, error) {
	a, ok := r.agreements[id]
	if !ok {
		return svc.AgreementRef{}, domain.ErrNotFound
	}
	return a, nil
}

func (r *fakePlacementRepo) GetActivePlacementForEmployeeAtCompany(_ context.Context, _ pgx.Tx, employeeID, companyID string) (domain.Placement, error) {
	for _, p := range r.placements {
		if p.EmployeeID == employeeID && p.ClientCompanyID == companyID && isActive(p.LifecycleStatus) {
			return p, nil
		}
	}
	return domain.Placement{}, domain.ErrNotFound
}

func (r *fakePlacementRepo) LockEmployeePlacements(_ context.Context, _ pgx.Tx, employeeID string) ([]domain.Placement, error) {
	var out []domain.Placement
	for _, p := range r.placements {
		if p.EmployeeID == employeeID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fakePlacementRepo) CreatePlacement(_ context.Context, _ pgx.Tx, p svc.CreatePlacementParams) (domain.Placement, error) {
	r.seq++
	id := "SWP-PL-" + itoa(9000+r.seq)
	now := fixedNow
	comp := r.companies[p.ClientCompanyID]
	out := domain.Placement{
		ID:                         id,
		EmployeeID:                 p.EmployeeID,
		AgreementID:                p.AgreementID,
		ClientCompanyID:            p.ClientCompanyID,
		SiteID:                     p.SiteID,
		ServiceLineID:              p.ServiceLineID,
		PositionID:                 p.PositionID,
		StartDate:                  p.StartDate,
		EndDate:                    p.EndDate,
		AnnualLeaveEntitlementDays: p.AnnualLeaveEntitlementDays,
		BaseSalaryRefIDR:           p.BaseSalaryRefIDR,
		Notes:                      p.Notes,
		LifecycleStatus:            p.LifecycleStatus,
		StatusChangedAt:            now,
		PredecessorID:              p.PredecessorID,
		BackdateReason:             p.BackdateReason,
		CreatedBy:                  p.CreatedBy,
		CreatedAt:                  now,
		UpdatedAt:                  now,
		ClientCompanyName:          strp(comp.Name),
	}
	r.placements[id] = out
	return out, nil
}

func (r *fakePlacementRepo) UpdatePlacementFields(_ context.Context, _ pgx.Tx, p svc.UpdatePlacementParams) (domain.Placement, error) {
	cur, ok := r.placements[p.ID]
	if !ok {
		return domain.Placement{}, domain.ErrNotFound
	}
	if p.PositionID != "" {
		cur.PositionID = p.PositionID
	}
	if p.EndDate != nil {
		cur.EndDate = p.EndDate
	}
	if p.AnnualLeaveEntitlementDays != nil {
		cur.AnnualLeaveEntitlementDays = p.AnnualLeaveEntitlementDays
	}
	if p.BaseSalaryRefIDR != nil {
		cur.BaseSalaryRefIDR = p.BaseSalaryRefIDR
	}
	if p.Notes != nil {
		cur.Notes = p.Notes
	}
	cur.UpdatedAt = fixedNow
	r.placements[p.ID] = cur
	return cur, nil
}

func (r *fakePlacementRepo) SetPlacementLifecycle(_ context.Context, _ pgx.Tx, p svc.SetLifecycleParams) (domain.Placement, error) {
	cur, ok := r.placements[p.ID]
	if !ok {
		return domain.Placement{}, domain.ErrNotFound
	}
	cur.LifecycleStatus = p.LifecycleStatus
	cur.EndedReason = p.EndedReason
	cur.EndedAt = p.EndedAt
	if p.TerminationReason != nil {
		cur.TerminationReason = p.TerminationReason
	}
	if p.ResignAt != nil {
		cur.ResignAt = p.ResignAt
	}
	if p.SuccessorID != nil {
		cur.SuccessorID = p.SuccessorID
	}
	cur.StatusChangedAt = fixedNow
	cur.UpdatedAt = fixedNow
	r.placements[p.ID] = cur
	return cur, nil
}

func (r *fakePlacementRepo) SetPlacementSuccessor(_ context.Context, _ pgx.Tx, id string, successorID *string) error {
	cur, ok := r.placements[id]
	if !ok {
		return domain.ErrNotFound
	}
	cur.SuccessorID = successorID
	r.placements[id] = cur
	return nil
}

func (r *fakePlacementRepo) InsertPlacementHistory(_ context.Context, _ pgx.Tx, _ svc.PlacementHistoryParams) error {
	return nil
}

var _ svc.PlacementRepository = (*fakePlacementRepo)(nil)

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

func isActive(status string) bool {
	switch status {
	case "ACTIVE", "EXPIRING", "PENDING_START", "SCHEDULED":
		return true
	}
	return false
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func containsFold(haystack, needle string) bool {
	return len(needle) == 0 ||
		bytes.Contains(bytes.ToLower([]byte(haystack)), bytes.ToLower([]byte(needle)))
}

func strp(s string) *string { return &s }

func i32p(n int32) *int32 { return &n }

// ---------------------------------------------------------------------------
// placement harness
// ---------------------------------------------------------------------------

type placementHarness struct {
	router      *chi.Mux
	repo        *fakePlacementRepo
	leaderRepoV *fakeShiftLeaderRepo
	psvc        *svc.PlacementService
	lsvc        *svc.ShiftLeaderService
	principal   auth.Principal
}

// leaderRepo returns the shared in-memory shift-leader repo for direct fixture
// seeding + assertions.
func (h *placementHarness) leaderRepo() *fakeShiftLeaderRepo { return h.leaderRepoV }

func newPlacementHarness(t *testing.T) *placementHarness {
	t.Helper()
	repo := newFakePlacementRepo()
	leaderRepo := newFakeShiftLeaderRepo()
	leaderRepo.placements = repo // share placement state for INV-4 / current-leader joins

	psvc := svc.NewPlacementService(repo, &fakeTxRunner{})
	lsvc := svc.NewShiftLeaderService(leaderRepo, &fakeTxRunner{})
	psvc.SetLeaderService(lsvc)
	psvc.SetClock(func() time.Time { return fixedNow })
	lsvc.SetClock(func() time.Time { return fixedNow })

	handler := placementhandler.NewHandler(psvc, lsvc)

	fh := &placementHarness{
		repo:        repo,
		leaderRepoV: leaderRepo,
		psvc:        psvc,
		lsvc:        lsvc,
		principal:   auth.Principal{UserID: "SWP-USR-0001", Role: auth.RoleHRAdmin},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), fh.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	// Reads: super_admin, hr_admin, shift_leader (mirror server.go).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
		r.Get("/placements", handler.ListPlacements)
		r.Get("/placements/expiring", handler.ListExpiringPlacements)
		r.Get("/placements/{id}", handler.GetPlacement)
		r.Get("/client-companies/{company_id}/roster", handler.GetCompanyRoster)
	})
	// Writes: super_admin, hr_admin (global).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Post("/placements", handler.CreatePlacement)
		r.Patch("/placements/{id}", handler.UpdatePlacement)
		r.Post("/placements/{id}:renew", handler.RenewPlacement)
		r.Post("/placements/{id}:transfer", handler.TransferPlacement)
		r.Post("/placements/{id}:end", handler.EndPlacement)
		r.Post("/placements/{id}:resign", handler.ResignPlacement)
		r.Post("/placements/{id}:terminate", handler.TerminatePlacement)
		r.Post("/shift-leader-assignments", handler.CreateShiftLeaderAssignment)
		r.Post("/shift-leader-assignments/{id}:replace", handler.ReplaceShiftLeaderAssignment)
		r.Post("/shift-leader-assignments/{id}:end", handler.EndShiftLeaderAssignment)
	})

	fh.router = r
	return fh
}

func (h *placementHarness) doJSON(method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// seed helpers
// ---------------------------------------------------------------------------

func (h *placementHarness) seedCompany(id, name, status string) {
	h.repo.companies[id] = svc.CompanyRef{ID: id, Name: name, Status: status, LeaderScope: "company"}
}

func (h *placementHarness) seedSite(id, companyID string) {
	h.repo.sites[id] = svc.SiteRef{ID: id, ClientCompanyID: companyID, Status: "active"}
}

func (h *placementHarness) seedEmployee(id string) {
	h.repo.employees[id] = domain.Employee{ID: id, FullName: "Emp " + id, Status: "active"}
}

func (h *placementHarness) seedAgreement(id, empID string, start, end time.Time) {
	e := end
	h.repo.agreements[id] = svc.AgreementRef{
		ID: id, EmployeeID: empID, Type: "PKWT", Status: "active",
		StartDate: start, EndDate: &e,
	}
}

func (h *placementHarness) seedPlacement(p domain.Placement) domain.Placement {
	if p.StatusChangedAt.IsZero() {
		p.StatusChangedAt = fixedNow
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = fixedNow
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = fixedNow
	}
	if c, ok := h.repo.companies[p.ClientCompanyID]; ok {
		p.ClientCompanyName = strp(c.Name)
	}
	h.repo.placements[p.ID] = p
	return p
}

// seedFullCompany wires a company + site + employee + agreement for create flows.
func (h *placementHarness) seedFullCreateContext(companyID, siteID, empID, agID string) {
	h.seedCompany(companyID, "Plaza Senayan", "active")
	h.seedSite(siteID, companyID)
	h.seedEmployee(empID)
	h.seedAgreement(agID, empID, jktDate(2026, 1, 1), jktDate(2027, 12, 31))
}

// ---------------------------------------------------------------------------
// Tests: ListPlacements
// ---------------------------------------------------------------------------

func TestListPlacements_ShapeAndEnvelope(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")
	end := jktDate(2027, 6, 30)
	p := domain.Placement{
		ID: "SWP-PL-5001", EmployeeID: "SWP-EMP-1108", AgreementID: "SWP-AG-7003",
		ClientCompanyID: "SWP-CMP-0021", SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001",
		PositionID: "SWP-POS-014", StartDate: jktDate(2026, 1, 1), EndDate: &end,
		LifecycleStatus: "ACTIVE",
		EmployeeName:    strp("Rudi Wijaya"), SiteName: strp("Main Site"),
		ServiceLineName: strp("Parking"), PositionName: strp("Parking Attendant"),
	}
	h.seedPlacement(p)

	rr := h.doJSON("GET", "/placements", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	for _, k := range []string{"data", "next_cursor", "has_more"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing envelope key: %s", k)
		}
	}
	data, ok := body["data"].([]any)
	if !ok || len(data) == 0 {
		t.Fatalf("data is not a non-empty array: %T %v", body["data"], body["data"])
	}
	first := data[0].(map[string]any)
	for _, k := range []string{
		"id", "employee_id", "employee_name", "client_company_id", "client_company_name",
		"site_id", "site_name", "service_line_id", "service_line_name",
		"position_id", "position_name", "lifecycle_status", "start_date", "end_date",
	} {
		if _, ok := first[k]; !ok {
			t.Errorf("data[0] missing key: %s", k)
		}
	}
	if first["lifecycle_status"] != "ACTIVE" {
		t.Errorf("lifecycle_status = %v, want ACTIVE", first["lifecycle_status"])
	}
}

func TestListPlacements_SearchAndStatusFilterPassthrough(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")
	endA := jktDate(2027, 6, 30)
	h.seedPlacement(domain.Placement{
		ID: "SWP-PL-5003", EmployeeID: "SWP-EMP-1042", ClientCompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-002", PositionID: "SWP-POS-015",
		AgreementID: "SWP-AG-7002", StartDate: jktDate(2026, 1, 1), EndDate: &endA,
		LifecycleStatus: "ACTIVE", EmployeeName: strp("Sari Hadi"),
	})
	h.seedPlacement(domain.Placement{
		ID: "SWP-PL-5001", EmployeeID: "SWP-EMP-1108", ClientCompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001", PositionID: "SWP-POS-014",
		AgreementID: "SWP-AG-7003", StartDate: jktDate(2026, 1, 1), EndDate: &endA,
		LifecycleStatus: "ENDED", EmployeeName: strp("Rudi Wijaya"),
	})

	rr := h.doJSON("GET", "/placements?q=Sari&status=ACTIVE", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// The fake repo must have received q="Sari" and status="ACTIVE" — proving the
	// search box + status filter are wired through, not silently dropped.
	lf := h.repo.lastListFilter
	if lf.Q == nil || *lf.Q != "Sari" {
		t.Errorf("repo filter Q = %v, want \"Sari\" (search box not wired)", lf.Q)
	}
	if lf.Status == nil || *lf.Status != "ACTIVE" {
		t.Errorf("repo filter Status = %v, want \"ACTIVE\" (status filter not wired)", lf.Status)
	}

	body := decodeBody(t, rr)
	data := body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("expected exactly 1 matching row (Sari/ACTIVE), got %d", len(data))
	}
	first := data[0].(map[string]any)
	if first["employee_name"] != "Sari Hadi" {
		t.Errorf("matched row employee_name = %v, want Sari Hadi", first["employee_name"])
	}
}

// ---------------------------------------------------------------------------
// Tests: GET /placements/expiring (DEDICATED endpoint)
// ---------------------------------------------------------------------------

func TestListExpiringPlacements_WithinWindowSortedAscAndDefaults(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")

	// Within window: today (2026-06-04) + 30d = 2026-07-04.
	end15 := jktDate(2026, 6, 19) // 15d out
	end25 := jktDate(2026, 6, 29) // 25d out
	endFar := jktDate(2026, 12, 1)
	h.seedPlacement(domain.Placement{
		ID: "SWP-PL-A25", EmployeeID: "SWP-EMP-A", ClientCompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001", PositionID: "SWP-POS-014",
		AgreementID: "SWP-AG-1", StartDate: jktDate(2026, 1, 1), EndDate: &end25,
		LifecycleStatus: "ACTIVE", EmployeeName: strp("A"),
	})
	h.seedPlacement(domain.Placement{
		ID: "SWP-PL-B15", EmployeeID: "SWP-EMP-B", ClientCompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001", PositionID: "SWP-POS-014",
		AgreementID: "SWP-AG-2", StartDate: jktDate(2026, 1, 1), EndDate: &end15,
		LifecycleStatus: "ACTIVE", EmployeeName: strp("B"),
	})
	h.seedPlacement(domain.Placement{
		ID: "SWP-PL-CFAR", EmployeeID: "SWP-EMP-C", ClientCompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001", PositionID: "SWP-POS-014",
		AgreementID: "SWP-AG-3", StartDate: jktDate(2026, 1, 1), EndDate: &endFar,
		LifecycleStatus: "ACTIVE", EmployeeName: strp("C"),
	})

	assertExpiring := func(path string) {
		rr := h.doJSON("GET", path, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", path, rr.Code, rr.Body.String())
		}
		body := decodeBody(t, rr)
		for _, k := range []string{"data", "next_cursor", "has_more"} {
			if _, ok := body[k]; !ok {
				t.Errorf("%s: missing envelope key %s", path, k)
			}
		}
		data := body["data"].([]any)
		if len(data) != 2 {
			t.Fatalf("%s: expected 2 expiring rows within 30d, got %d", path, len(data))
		}
		// end_date ascending: B (15d) before A (25d).
		if data[0].(map[string]any)["id"] != "SWP-PL-B15" {
			t.Errorf("%s: first row id = %v, want SWP-PL-B15 (sorted end_date asc)", path, data[0].(map[string]any)["id"])
		}
		if data[1].(map[string]any)["id"] != "SWP-PL-A25" {
			t.Errorf("%s: second row id = %v, want SWP-PL-A25", path, data[1].(map[string]any)["id"])
		}
	}

	assertExpiring("/placements/expiring?within_days=30")
	// within_days omitted → defaults to 30 → same result set.
	assertExpiring("/placements/expiring")
}

// ---------------------------------------------------------------------------
// Tests: GET /placements/{id}
// ---------------------------------------------------------------------------

func TestGetPlacement_DetailShape_200(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")
	end := jktDate(2027, 6, 30)
	h.seedPlacement(domain.Placement{
		ID: "SWP-PL-5001", EmployeeID: "SWP-EMP-1108", ClientCompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001", PositionID: "SWP-POS-014",
		AgreementID: "SWP-AG-7003", StartDate: jktDate(2026, 1, 1), EndDate: &end,
		LifecycleStatus: "ACTIVE",
	})

	rr := h.doJSON("GET", "/placements/SWP-PL-5001", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	for _, k := range []string{"placement", "history_chain", "current_shift_leader"} {
		if _, ok := body[k]; !ok {
			t.Errorf("detail missing key: %s", k)
		}
	}
	pl, ok := body["placement"].(map[string]any)
	if !ok {
		t.Fatalf("placement is not an object: %T", body["placement"])
	}
	if pl["id"] != "SWP-PL-5001" {
		t.Errorf("placement.id = %v, want SWP-PL-5001", pl["id"])
	}
	if _, ok := body["history_chain"].([]any); !ok {
		t.Errorf("history_chain is not an array: %T", body["history_chain"])
	}
}

func TestGetPlacement_404(t *testing.T) {
	h := newPlacementHarness(t)
	rr := h.doJSON("GET", "/placements/SWP-PL-GHOST", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	if errObject(t, decodeBody(t, rr))["code"] != "NOT_FOUND" {
		t.Errorf("error.code != NOT_FOUND")
	}
}

// ---------------------------------------------------------------------------
// Tests: POST /placements
// ---------------------------------------------------------------------------

func placementCreateBody(empID, agID, companyID, siteID, start, end string) map[string]any {
	return map[string]any{
		"employee_id":       empID,
		"agreement_id":      agID,
		"client_company_id": companyID,
		"site_id":           siteID,
		"service_line_id":   "SWP-SVC-001",
		"position_id":       "SWP-POS-014",
		"start_date":        start,
		"end_date":          end,
	}
}

func TestCreatePlacement_Happy_201_LocationAndBody(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedFullCreateContext("SWP-CMP-0021", "SWP-SITE-0001", "SWP-EMP-1042", "SWP-AG-7002")

	// Backdated start (with reason) → clearly <= today → ACTIVE.
	body0 := placementCreateBody("SWP-EMP-1042", "SWP-AG-7002", "SWP-CMP-0021", "SWP-SITE-0001", "2026-06-01", "2026-12-31")
	body0["backdate_reason"] = "Onboarding doc lost; agent worked since 1 Jun per timesheets."
	rr := h.doJSON("POST", "/placements", body0)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	for _, k := range []string{"id", "employee_id", "client_company_id", "site_id", "lifecycle_status", "start_date"} {
		if _, ok := body[k]; !ok {
			t.Errorf("created placement missing key: %s", k)
		}
	}
	// start_date <= today → ACTIVE.
	if body["lifecycle_status"] != "ACTIVE" {
		t.Errorf("lifecycle_status = %v, want ACTIVE", body["lifecycle_status"])
	}
	loc := rr.Header().Get("Location")
	if len(loc) < len("/api/v1/placements/SWP-PL-") || loc[:len("/api/v1/placements/SWP-PL-")] != "/api/v1/placements/SWP-PL-" {
		t.Errorf("Location = %q, want prefix /api/v1/placements/SWP-PL-", loc)
	}
}

func TestCreatePlacement_INV1Violation_409_Details(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedFullCreateContext("SWP-CMP-0021", "SWP-SITE-0001", "SWP-EMP-1042", "SWP-AG-7002")
	// Existing active placement for the same employee (different company).
	h.seedCompany("SWP-CMP-0009", "Mall Kelapa Gading", "active")
	endX := jktDate(2026, 8, 31)
	h.seedPlacement(domain.Placement{
		ID: "SWP-PL-988", EmployeeID: "SWP-EMP-1042", ClientCompanyID: "SWP-CMP-0009",
		SiteID: "SWP-SITE-0009", ServiceLineID: "SWP-SVC-003", PositionID: "SWP-POS-021",
		AgreementID: "SWP-AG-OLD", StartDate: jktDate(2025, 9, 1), EndDate: &endX,
		LifecycleStatus: "ACTIVE",
	})

	rr := h.doJSON("POST", "/placements",
		placementCreateBody("SWP-EMP-1042", "SWP-AG-7002", "SWP-CMP-0021", "SWP-SITE-0001", "2026-06-04", "2026-12-31"))
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 INV_1_VIOLATION, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "INV_1_VIOLATION" {
		t.Errorf("error.code = %v, want INV_1_VIOLATION", e["code"])
	}
	details, ok := e["details"].(map[string]any)
	if !ok {
		t.Fatalf("error.details missing/not an object: %T", e["details"])
	}
	if details["invariant"] != "INV_1" {
		t.Errorf("details.invariant = %v, want INV_1", details["invariant"])
	}
	cp, ok := details["current_placement"].(map[string]any)
	if !ok {
		t.Fatalf("details.current_placement missing: %T", details["current_placement"])
	}
	if cp["id"] != "SWP-PL-988" {
		t.Errorf("details.current_placement.id = %v, want SWP-PL-988", cp["id"])
	}
	sa, ok := details["suggested_actions"].([]any)
	if !ok {
		t.Fatalf("details.suggested_actions missing: %T", details["suggested_actions"])
	}
	if !containsAny(sa, "transfer") || !containsAny(sa, "end") {
		t.Errorf("suggested_actions = %v, want to contain transfer + end", sa)
	}
}

func TestCreatePlacement_CompanyInactive_409(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-ARCH", "Archived Co", "archived")
	h.seedSite("SWP-SITE-ARCH", "SWP-CMP-ARCH")
	h.seedEmployee("SWP-EMP-1042")
	h.seedAgreement("SWP-AG-7002", "SWP-EMP-1042", jktDate(2026, 1, 1), jktDate(2027, 12, 31))

	rr := h.doJSON("POST", "/placements",
		placementCreateBody("SWP-EMP-1042", "SWP-AG-7002", "SWP-CMP-ARCH", "SWP-SITE-ARCH", "2026-06-03", "2026-12-31"))
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 COMPANY_INACTIVE, got %d: %s", rr.Code, rr.Body.String())
	}
	if errObject(t, decodeBody(t, rr))["code"] != "COMPANY_INACTIVE" {
		t.Errorf("error.code != COMPANY_INACTIVE")
	}
}

func TestCreatePlacement_EndBeforeStart_400(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedFullCreateContext("SWP-CMP-0021", "SWP-SITE-0001", "SWP-EMP-1042", "SWP-AG-7002")

	rr := h.doJSON("POST", "/placements",
		placementCreateBody("SWP-EMP-1042", "SWP-AG-7002", "SWP-CMP-0021", "SWP-SITE-0001", "2026-06-03", "2026-06-01"))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "INVALID_REQUEST" {
		t.Errorf("error.code = %v, want INVALID_REQUEST", e["code"])
	}
	fields, _ := e["fields"].(map[string]any)
	if _, ok := fields["end_date"]; !ok {
		t.Error("error.fields.end_date missing on end<=start 400")
	}
}

func TestCreatePlacement_StartOutsideContract_422(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")
	h.seedSite("SWP-SITE-0001", "SWP-CMP-0021")
	h.seedEmployee("SWP-EMP-1042")
	// Agreement starts 2026-06-01; start placement 2026-05-15 (before) → 422.
	h.seedAgreement("SWP-AG-7002", "SWP-EMP-1042", jktDate(2026, 6, 1), jktDate(2027, 5, 31))

	rr := h.doJSON("POST", "/placements", map[string]any{
		"employee_id":       "SWP-EMP-1042",
		"agreement_id":      "SWP-AG-7002",
		"client_company_id": "SWP-CMP-0021",
		"site_id":           "SWP-SITE-0001",
		"service_line_id":   "SWP-SVC-001",
		"position_id":       "SWP-POS-014",
		"start_date":        "2026-05-15",
		"end_date":          "2026-12-31",
		"backdate_reason":   "Onboarding doc lost.",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 PLACEMENT_OUTSIDE_CONTRACT, got %d: %s", rr.Code, rr.Body.String())
	}
	if errObject(t, decodeBody(t, rr))["code"] != "PLACEMENT_OUTSIDE_CONTRACT" {
		t.Errorf("error.code != PLACEMENT_OUTSIDE_CONTRACT")
	}
}

func TestCreatePlacement_RBAC_AgentForbidden_403(t *testing.T) {
	h := newPlacementHarness(t)
	h.principal = auth.Principal{UserID: "SWP-USR-AGENT", Role: auth.RoleAgent, EmployeeID: "SWP-EMP-1042"}
	h.seedFullCreateContext("SWP-CMP-0021", "SWP-SITE-0001", "SWP-EMP-1042", "SWP-AG-7002")

	rr := h.doJSON("POST", "/placements",
		placementCreateBody("SWP-EMP-1042", "SWP-AG-7002", "SWP-CMP-0021", "SWP-SITE-0001", "2026-06-03", "2026-12-31"))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for agent POST /placements, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tests: PATCH /placements/{id} (terminal-state immutability)
// ---------------------------------------------------------------------------

func TestUpdatePlacement_TerminalImmutable_409(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")
	h.seedPlacement(domain.Placement{
		ID: "SWP-PL-DEAD", EmployeeID: "SWP-EMP-1042", ClientCompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001", PositionID: "SWP-POS-014",
		AgreementID: "SWP-AG-7002", StartDate: jktDate(2025, 1, 1),
		LifecycleStatus: "ENDED",
	})

	rr := h.doJSON("PATCH", "/placements/SWP-PL-DEAD", map[string]any{"notes": "edit"})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 TERMINAL_STATE_IMMUTABLE, got %d: %s", rr.Code, rr.Body.String())
	}
	if errObject(t, decodeBody(t, rr))["code"] != "TERMINAL_STATE_IMMUTABLE" {
		t.Errorf("error.code != TERMINAL_STATE_IMMUTABLE")
	}
}

// ---------------------------------------------------------------------------
// Tests: lifecycle actions :end / :resign / :terminate
// ---------------------------------------------------------------------------

func (h *placementHarness) seedActivePlacement(id, empID, companyID string) {
	h.seedCompany(companyID, "Plaza Senayan", "active")
	end := jktDate(2027, 6, 30)
	h.seedPlacement(domain.Placement{
		ID: id, EmployeeID: empID, ClientCompanyID: companyID,
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001", PositionID: "SWP-POS-014",
		AgreementID: "SWP-AG-7002", StartDate: jktDate(2026, 1, 1), EndDate: &end,
		LifecycleStatus: "ACTIVE",
	})
}

func TestEndPlacement_200_EndedReason(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedActivePlacement("SWP-PL-END", "SWP-EMP-1042", "SWP-CMP-0021")

	rr := h.doJSON("POST", "/placements/SWP-PL-END:end", map[string]any{
		"reason": "END_OF_TERM", "effective_date": "2026-12-31",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["lifecycle_status"] != "ENDED" {
		t.Errorf("lifecycle_status = %v, want ENDED", body["lifecycle_status"])
	}
	if body["ended_reason"] != "ENDED" {
		t.Errorf("ended_reason = %v, want ENDED", body["ended_reason"])
	}
}

func TestResignPlacement_200_ResignAtSet(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedActivePlacement("SWP-PL-RES", "SWP-EMP-1042", "SWP-CMP-0021")

	rr := h.doJSON("POST", "/placements/SWP-PL-RES:resign", map[string]any{
		"resign_at": "2026-06-30", "resignation_reason": "Pindah kerja.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["lifecycle_status"] != "RESIGNED" {
		t.Errorf("lifecycle_status = %v, want RESIGNED", body["lifecycle_status"])
	}
	if body["resign_at"] != "2026-06-30" {
		t.Errorf("resign_at = %v, want 2026-06-30", body["resign_at"])
	}
}

func TestTerminatePlacement_WrongCompanyNameConfirm_400(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedActivePlacement("SWP-PL-TRM", "SWP-EMP-1042", "SWP-CMP-0021")

	rr := h.doJSON("POST", "/placements/SWP-PL-TRM:terminate", map[string]any{
		"termination_reason":        "Pelanggaran SOP berulang terdokumentasi.",
		"type_company_name_confirm": "Wrong Name",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on wrong company-name confirm, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	fields, _ := e["fields"].(map[string]any)
	if _, ok := fields["type_company_name_confirm"]; !ok {
		t.Error("error.fields.type_company_name_confirm missing")
	}
}

func TestTerminatePlacement_Happy_200(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedActivePlacement("SWP-PL-TRM2", "SWP-EMP-1042", "SWP-CMP-0021")

	rr := h.doJSON("POST", "/placements/SWP-PL-TRM2:terminate", map[string]any{
		"termination_reason":        "Pelanggaran SOP berulang terdokumentasi.",
		"type_company_name_confirm": "Plaza Senayan", // matches seeded company name
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if decodeBody(t, rr)["lifecycle_status"] != "TERMINATED" {
		t.Errorf("lifecycle_status != TERMINATED")
	}
}

// ---------------------------------------------------------------------------
// Tests: :transfer
// ---------------------------------------------------------------------------

func TestTransferPlacement_Happy_201(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedActivePlacement("SWP-PL-XFER", "SWP-EMP-1042", "SWP-CMP-0021")
	h.seedSite("SWP-SITE-0001", "SWP-CMP-0021")
	h.seedCompany("SWP-CMP-0022", "Mall Senayan City", "active")

	rr := h.doJSON("POST", "/placements/SWP-PL-XFER:transfer", map[string]any{
		"new_client_company_id": "SWP-CMP-0022",
		"new_service_line_id":   "SWP-SVC-002",
		"new_position_id":       "SWP-POS-031",
		"new_start_date":        "2026-07-01",
		"new_end_date":          "2027-06-30",
		"transfer_reason":       "Rotasi rutin.",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	pred, ok := body["predecessor"].(map[string]any)
	if !ok {
		t.Fatalf("predecessor missing: %T", body["predecessor"])
	}
	if pred["lifecycle_status"] != "TRANSFERRED" {
		t.Errorf("predecessor.lifecycle_status = %v, want TRANSFERRED", pred["lifecycle_status"])
	}
	succ, ok := body["successor"].(map[string]any)
	if !ok {
		t.Fatalf("successor missing: %T", body["successor"])
	}
	if succ["predecessor_id"] != "SWP-PL-XFER" {
		t.Errorf("successor.predecessor_id = %v, want SWP-PL-XFER", succ["predecessor_id"])
	}
}

func TestTransferPlacement_SameCompanySameLine_422(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedActivePlacement("SWP-PL-XFER2", "SWP-EMP-1042", "SWP-CMP-0021")

	rr := h.doJSON("POST", "/placements/SWP-PL-XFER2:transfer", map[string]any{
		"new_client_company_id": "SWP-CMP-0021",
		"new_service_line_id":   "SWP-SVC-001", // same company + same line
		"new_position_id":       "SWP-POS-014",
		"new_start_date":        "2026-07-01",
		"transfer_reason":       "noop",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 RULE_VIOLATION, got %d: %s", rr.Code, rr.Body.String())
	}
	if errObject(t, decodeBody(t, rr))["code"] != "RULE_VIOLATION" {
		t.Errorf("error.code != RULE_VIOLATION")
	}
}

// ---------------------------------------------------------------------------
// Tests: :renew
// ---------------------------------------------------------------------------

func TestRenewPlacement_Happy_201_PredecessorSuperseded(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")
	h.seedAgreement("SWP-AG-7002", "SWP-EMP-1042", jktDate(2026, 1, 1), jktDate(2028, 12, 31))
	end := jktDate(2026, 12, 31)
	h.seedPlacement(domain.Placement{
		ID: "SWP-PL-RNW", EmployeeID: "SWP-EMP-1042", ClientCompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001", PositionID: "SWP-POS-014",
		AgreementID: "SWP-AG-7002", StartDate: jktDate(2026, 1, 1), EndDate: &end,
		LifecycleStatus: "ACTIVE",
	})

	rr := h.doJSON("POST", "/placements/SWP-PL-RNW:renew", map[string]any{
		"new_start_date": "2027-01-01",
		"new_end_date":   "2027-12-31",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	pred, _ := body["predecessor"].(map[string]any)
	if pred["lifecycle_status"] != "SUPERSEDED" {
		t.Errorf("predecessor.lifecycle_status = %v, want SUPERSEDED", pred["lifecycle_status"])
	}
	succ, _ := body["successor"].(map[string]any)
	if succ["predecessor_id"] != "SWP-PL-RNW" {
		t.Errorf("successor.predecessor_id = %v, want SWP-PL-RNW", succ["predecessor_id"])
	}
}

func TestRenewPlacement_BufferOverlap_422(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")
	h.seedAgreement("SWP-AG-7002", "SWP-EMP-1042", jktDate(2026, 1, 1), jktDate(2028, 12, 31))
	end := jktDate(2026, 12, 31)
	h.seedPlacement(domain.Placement{
		ID: "SWP-PL-OVL", EmployeeID: "SWP-EMP-1042", ClientCompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001", PositionID: "SWP-POS-014",
		AgreementID: "SWP-AG-7002", StartDate: jktDate(2026, 1, 1), EndDate: &end,
		LifecycleStatus: "ACTIVE",
	})

	// new_start_date == predecessor.end_date violates the 1-day buffer.
	rr := h.doJSON("POST", "/placements/SWP-PL-OVL:renew", map[string]any{
		"new_start_date": "2026-12-31",
		"new_end_date":   "2027-12-31",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 PLACEMENT_PERIOD_OVERLAP, got %d: %s", rr.Code, rr.Body.String())
	}
	if errObject(t, decodeBody(t, rr))["code"] != "PLACEMENT_PERIOD_OVERLAP" {
		t.Errorf("error.code != PLACEMENT_PERIOD_OVERLAP")
	}
}

func containsAny(arr []any, v string) bool {
	for _, x := range arr {
		if s, ok := x.(string); ok && s == v {
			return true
		}
	}
	return false
}

// silence unused import in case ymd is dropped during edits.
var _ = ymd
