// Package org_test contains contract tests for the E2 org handler endpoints.
// These tests assert the EXACT JSON field names, types, and status codes
// required by the OpenAPI spec — the drift gate replacing server-side codegen.
//
// Pattern: httptest + real Service wired to an in-memory fakeCompanyRepo (no DB).
// Principal injection via auth.WithPrincipal on the request context.
// Mirrors internal/handler/foundations/handler_test.go exactly.
package org_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	orghandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/org"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	orgsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/org"
)

// ---------------------------------------------------------------------------
// Fake pgx.Tx — only Exec is needed (for audit.Record); all other methods panic.
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

// ---------------------------------------------------------------------------
// Fake TxRunner — passes a real fakeTx so audit.Record can call Exec.
// ---------------------------------------------------------------------------

type fakeTxRunner struct{}

func (f *fakeTxRunner) InTx(_ context.Context, fn func(pgx.Tx) error) error {
	return fn(&fakeTx{})
}

// ---------------------------------------------------------------------------
// errUnique is a fake unique-violation error (mimics pgconn code 23505).
// ---------------------------------------------------------------------------

type errUnique struct{}

func (e errUnique) Error() string { return "duplicate key value violates unique constraint (23505)" }

var errNotFound = domain.ErrNotFound

// ---------------------------------------------------------------------------
// fakeCompanyRepo — in-memory implementation of orgsvc.CompanyRepository.
// ---------------------------------------------------------------------------

type fakeCompanyRepo struct {
	companies map[string]domain.ClientCompany
	sites     map[string]domain.Site

	// error overrides (set per-test to trigger error paths)
	createCompanyErr error
}

func newFakeCompanyRepo() *fakeCompanyRepo {
	return &fakeCompanyRepo{
		companies: make(map[string]domain.ClientCompany),
		sites:     make(map[string]domain.Site),
	}
}

func (r *fakeCompanyRepo) addCompany(c domain.ClientCompany) {
	r.companies[c.ID] = c
}

func (r *fakeCompanyRepo) addSite(s domain.Site) {
	r.sites[s.ID] = s
}

func (r *fakeCompanyRepo) ListClientCompanies(_ context.Context, f domain.CompanyFilter) ([]domain.ClientCompany, error) {
	var all []domain.ClientCompany
	for _, c := range r.companies {
		if f.Status != nil && c.Status != *f.Status {
			continue
		}
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if c.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if c.CreatedAt.Equal(*f.CursorCreatedAt) && c.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, c)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID < all[j].ID
		}
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	if f.Limit > 0 && len(all) > f.Limit {
		return all[:f.Limit], nil
	}
	return all, nil
}

func (r *fakeCompanyRepo) GetCompanyByID(_ context.Context, id string) (domain.ClientCompany, error) {
	c, ok := r.companies[id]
	if !ok {
		return domain.ClientCompany{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *fakeCompanyRepo) CountActiveSitesForCompany(_ context.Context, companyID string) (int64, error) {
	var count int64
	for _, s := range r.sites {
		if s.ClientCompanyID == companyID && s.Status == "active" {
			count++
		}
	}
	return count, nil
}

func (r *fakeCompanyRepo) CreateCompany(_ context.Context, _ pgx.Tx, p orgsvc.CreateCompanyParams) (domain.ClientCompany, error) {
	if r.createCompanyErr != nil {
		return domain.ClientCompany{}, r.createCompanyErr
	}
	now := time.Now().UTC()
	id := "SWP-CMP-" + now.Format("150405000")
	c := domain.ClientCompany{
		ID:          id,
		Name:        p.Name,
		Address:     p.Address,
		LeaderScope: p.LeaderScope,
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if p.NPWP != "" {
		s := p.NPWP
		c.NPWP = &s
	}
	if p.PICName != "" {
		s := p.PICName
		c.PICName = &s
	}
	if p.Phone != "" {
		s := p.Phone
		c.Phone = &s
	}
	if p.Email != "" {
		s := p.Email
		c.Email = &s
	}
	r.companies[id] = c
	return c, nil
}

func (r *fakeCompanyRepo) UpdateCompany(_ context.Context, _ pgx.Tx, p orgsvc.UpdateCompanyParams) (domain.ClientCompany, error) {
	c, ok := r.companies[p.ID]
	if !ok {
		return domain.ClientCompany{}, domain.ErrNotFound
	}
	c.Name = p.Name
	c.Address = p.Address
	c.LeaderScope = p.LeaderScope
	c.UpdatedAt = time.Now().UTC()
	r.companies[p.ID] = c
	return c, nil
}

func (r *fakeCompanyRepo) SetCompanyStatus(_ context.Context, _ pgx.Tx, id, status string) (domain.ClientCompany, error) {
	c, ok := r.companies[id]
	if !ok {
		return domain.ClientCompany{}, domain.ErrNotFound
	}
	c.Status = status
	c.UpdatedAt = time.Now().UTC()
	r.companies[id] = c
	return c, nil
}

func (r *fakeCompanyRepo) ListSitesForCompany(_ context.Context, companyID string, f domain.SiteFilter) ([]domain.Site, error) {
	var all []domain.Site
	for _, s := range r.sites {
		if s.ClientCompanyID != companyID {
			continue
		}
		if f.Status != nil && s.Status != *f.Status {
			continue
		}
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if s.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if s.CreatedAt.Equal(*f.CursorCreatedAt) && s.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, s)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID < all[j].ID
		}
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	if f.Limit > 0 && len(all) > f.Limit {
		return all[:f.Limit], nil
	}
	return all, nil
}

func (r *fakeCompanyRepo) GetSiteByID(_ context.Context, id string) (domain.Site, error) {
	s, ok := r.sites[id]
	if !ok {
		return domain.Site{}, domain.ErrNotFound
	}
	return s, nil
}

var siteCounter int

func (r *fakeCompanyRepo) CreateSite(_ context.Context, _ pgx.Tx, p orgsvc.CreateSiteParams) (domain.Site, error) {
	siteCounter++
	now := time.Now().UTC()
	id := "SWP-SITE-" + now.Format("150405") + itoa(siteCounter)
	s := domain.Site{
		ID:              id,
		ClientCompanyID: p.ClientCompanyID,
		Name:            p.Name,
		Address:         p.Address,
		GeofenceRadiusM: p.GeofenceRadiusM,
		IsPrimary:       p.IsPrimary,
		GeoLat:          p.GeoLat,
		GeoLng:          p.GeoLng,
		Status:          "active",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if p.Code != "" {
		s2 := p.Code
		s.Code = &s2
	}
	if p.PICName != "" {
		s2 := p.PICName
		s.PICName = &s2
	}
	if p.Phone != "" {
		s2 := p.Phone
		s.Phone = &s2
	}
	r.sites[id] = s
	return s, nil
}

func (r *fakeCompanyRepo) UpdateSite(_ context.Context, _ pgx.Tx, p orgsvc.UpdateSiteParams) (domain.Site, error) {
	s, ok := r.sites[p.ID]
	if !ok {
		return domain.Site{}, domain.ErrNotFound
	}
	s.Name = p.Name
	s.Address = p.Address
	s.GeoLat = p.GeoLat
	s.GeoLng = p.GeoLng
	s.GeofenceRadiusM = p.GeofenceRadiusM
	s.IsPrimary = p.IsPrimary
	s.UpdatedAt = time.Now().UTC()
	r.sites[p.ID] = s
	return s, nil
}

func (r *fakeCompanyRepo) DemoteOtherPrimaries(_ context.Context, _ pgx.Tx, companyID, exceptSiteID string) error {
	for id, s := range r.sites {
		if s.ClientCompanyID == companyID && s.IsPrimary && id != exceptSiteID {
			s.IsPrimary = false
			r.sites[id] = s
		}
	}
	return nil
}

func (r *fakeCompanyRepo) SetSitePrimary(_ context.Context, _ pgx.Tx, id string) (domain.Site, error) {
	s, ok := r.sites[id]
	if !ok {
		return domain.Site{}, domain.ErrNotFound
	}
	s.IsPrimary = true
	r.sites[id] = s
	return s, nil
}

func (r *fakeCompanyRepo) SetSiteStatus(_ context.Context, _ pgx.Tx, id, status string) (domain.Site, error) {
	s, ok := r.sites[id]
	if !ok {
		return domain.Site{}, domain.ErrNotFound
	}
	s.Status = status
	s.UpdatedAt = time.Now().UTC()
	r.sites[id] = s
	return s, nil
}

// Compile-time interface check.
var _ orgsvc.CompanyRepository = (*fakeCompanyRepo)(nil)

// ---------------------------------------------------------------------------
// Test harness for companies/sites
// ---------------------------------------------------------------------------

type companyHarness struct {
	router    *chi.Mux
	repo      *fakeCompanyRepo
	principal auth.Principal
}

func newCompanyHarness(t *testing.T) *companyHarness {
	t.Helper()
	repo := newFakeCompanyRepo()
	svc := orgsvc.NewService(repo, &fakeTxRunner{})
	h := orghandler.NewHandler(svc)

	fh := &companyHarness{
		repo:      repo,
		principal: auth.Principal{UserID: "SWP-USR-0001", Role: auth.RoleHRAdmin},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	// Dynamic principal injection — reads fh.principal per request.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), fh.principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	// RBAC guard — all four roles can read companies; admin roles can write.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
		r.Get("/client-companies", h.ListClientCompanies)
		r.Get("/client-companies/{client_company_id}", h.GetClientCompany)
		r.Get("/client-companies/{client_company_id}/sites", h.ListSites)
		r.Get("/sites/{site_id}", h.GetSite)
	})
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Post("/client-companies", h.CreateClientCompany)
		r.Patch("/client-companies/{client_company_id}", h.UpdateClientCompany)
		r.Post("/client-companies/{client_company_id}:deactivate", h.DeactivateClientCompany)
		r.Post("/client-companies/{client_company_id}:reactivate", h.ReactivateClientCompany)
		r.Post("/client-companies/{client_company_id}/sites", h.CreateSite)
		r.Patch("/sites/{site_id}", h.UpdateSite)
		r.Post("/sites/{site_id}:deactivate", h.DeactivateSite)
	})

	fh.router = r
	return fh
}

func (h *companyHarness) do(method, path string, body any) *httptest.ResponseRecorder {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, rr.Body.String())
	}
	return m
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

func (h *companyHarness) seedCompany(n int) []domain.ClientCompany {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	companies := make([]domain.ClientCompany, n)
	for i := 0; i < n; i++ {
		id := "SWP-CMP-" + itoa(i+1)
		c := domain.ClientCompany{
			ID:          id,
			Name:        "Company " + itoa(i+1),
			Address:     "Jl. Test " + itoa(i+1),
			LeaderScope: "company",
			Status:      "active",
			SiteCount:   1,
			CreatedAt:   base.Add(time.Duration(i) * time.Minute),
			UpdatedAt:   base.Add(time.Duration(i) * time.Minute),
		}
		h.repo.addCompany(c)
		companies[i] = c
	}
	return companies
}

func (h *companyHarness) seedSite(companyID string) domain.Site {
	lat := -6.2088
	lng := 106.8456
	s := domain.Site{
		ID:              "SWP-SITE-001",
		ClientCompanyID: companyID,
		Name:            "Main Site",
		Address:         "Jl. Test 1",
		GeoLat:          &lat,
		GeoLng:          &lng,
		GeofenceRadiusM: 100,
		IsPrimary:       true,
		Status:          "active",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	h.repo.addSite(s)
	return s
}

// ---------------------------------------------------------------------------
// Task 1: Client Companies
// ---------------------------------------------------------------------------

func TestListClientCompanies_ShapeAndEnvelope(t *testing.T) {
	h := newCompanyHarness(t)
	h.seedCompany(3)

	rr := h.do("GET", "/client-companies", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Envelope keys.
	for _, k := range []string{"data", "next_cursor", "has_more"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing envelope key: %s", k)
		}
	}

	data, ok := body["data"].([]any)
	if !ok || len(data) == 0 {
		t.Fatalf("data is not a non-empty array: %T %v", body["data"], body["data"])
	}

	// Assert first item has all required ClientCompany keys.
	first := data[0].(map[string]any)
	requiredKeys := []string{
		"id", "name", "address", "leader_scope",
		"status", "has_leader", "site_count", "active_placement_count",
		"created_at", "updated_at",
	}
	for _, k := range requiredKeys {
		if _, ok := first[k]; !ok {
			t.Errorf("data[0] missing key: %s", k)
		}
	}

	// status must be UPPERCASE.
	if first["status"] != "ACTIVE" {
		t.Errorf("data[0].status = %v, want ACTIVE", first["status"])
	}
	// has_leader must be bool.
	if _, ok := first["has_leader"].(bool); !ok {
		t.Errorf("data[0].has_leader is not bool: %T", first["has_leader"])
	}
	// site_count must be a number.
	if _, ok := first["site_count"].(float64); !ok {
		t.Errorf("data[0].site_count is not a number: %T", first["site_count"])
	}
}

func TestGetClientCompany_200(t *testing.T) {
	h := newCompanyHarness(t)
	companies := h.seedCompany(1)
	id := companies[0].ID

	rr := h.do("GET", "/client-companies/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["id"] != id {
		t.Errorf("id = %v, want %s", body["id"], id)
	}
}

func TestGetClientCompany_404(t *testing.T) {
	h := newCompanyHarness(t)

	rr := h.do("GET", "/client-companies/SWP-CMP-NONEXIST", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("error.code = %v, want NOT_FOUND", errObj["code"])
	}
}

func TestCreateClientCompany_201(t *testing.T) {
	h := newCompanyHarness(t)

	rr := h.do("POST", "/client-companies", map[string]any{
		"name":    "PT Test Mandiri",
		"address": "Jl. Sudirman No. 1, Jakarta",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Location header must be set.
	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Error("missing Location header on 201")
	}

	body := decodeBody(t, rr)
	requiredKeys := []string{
		"id", "name", "address", "leader_scope",
		"status", "has_leader", "site_count", "active_placement_count",
		"created_at", "updated_at",
	}
	for _, k := range requiredKeys {
		if _, ok := body[k]; !ok {
			t.Errorf("create response missing key: %s", k)
		}
	}
	if body["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", body["status"])
	}
	// CC-1c: Main Site auto-provisioned → site_count == 1.
	if body["site_count"] != float64(1) {
		t.Errorf("site_count = %v, want 1 (auto-provisioned Main Site)", body["site_count"])
	}
}

func TestCreateClientCompany_409_Conflict(t *testing.T) {
	h := newCompanyHarness(t)
	// Make the repo return a unique violation on CreateCompany.
	h.repo.createCompanyErr = errUnique{}

	rr := h.do("POST", "/client-companies", map[string]any{
		"name":    "Duplicate Company",
		"address": "Jl. Duplicate 1",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "CONFLICT" {
		t.Errorf("error.code = %v, want CONFLICT", errObj["code"])
	}
}

func TestUpdateClientCompany_200(t *testing.T) {
	h := newCompanyHarness(t)
	companies := h.seedCompany(1)
	id := companies[0].ID

	rr := h.do("PATCH", "/client-companies/"+id, map[string]any{
		"name": "Updated Company Name",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["name"] != "Updated Company Name" {
		t.Errorf("name = %v, want 'Updated Company Name'", body["name"])
	}
}

func TestDeactivateClientCompany_200_Then_409(t *testing.T) {
	h := newCompanyHarness(t)
	companies := h.seedCompany(1)
	id := companies[0].ID

	// First deactivate → 200 with status INACTIVE.
	rr := h.do("POST", "/client-companies/"+id+":deactivate", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("deactivate: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "INACTIVE" {
		t.Errorf("after deactivate status = %v, want INACTIVE", body["status"])
	}

	// Second deactivate → 409 already inactive.
	rr2 := h.do("POST", "/client-companies/"+id+":deactivate", nil)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("deactivate again: expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestReactivateClientCompany_200_Then_409(t *testing.T) {
	h := newCompanyHarness(t)
	// Seed an inactive company.
	now := time.Now().UTC()
	h.repo.addCompany(domain.ClientCompany{
		ID:          "SWP-CMP-INAC",
		Name:        "Inactive Co",
		Address:     "Jl. Inactive 1",
		LeaderScope: "company",
		Status:      "inactive",
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	// Reactivate → 200 with status ACTIVE.
	rr := h.do("POST", "/client-companies/SWP-CMP-INAC:reactivate", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("reactivate: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "ACTIVE" {
		t.Errorf("after reactivate status = %v, want ACTIVE", body["status"])
	}

	// Reactivate again → 409 already active.
	rr2 := h.do("POST", "/client-companies/SWP-CMP-INAC:reactivate", nil)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("reactivate again: expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Task 1: Sites
// ---------------------------------------------------------------------------

func TestListSites_ShapeAndGeofence(t *testing.T) {
	h := newCompanyHarness(t)
	companies := h.seedCompany(1)
	companyID := companies[0].ID

	// Seed a site with geo coordinates (geofence_active should be true).
	site := h.seedSite(companyID)
	_ = site

	rr := h.do("GET", "/client-companies/"+companyID+"/sites", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	for _, k := range []string{"data", "next_cursor", "has_more"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing envelope key: %s", k)
		}
	}

	data, _ := body["data"].([]any)
	if len(data) == 0 {
		t.Fatal("expected at least one site in data")
	}

	first := data[0].(map[string]any)
	requiredKeys := []string{
		"id", "client_company_id", "name", "code",
		"address", "geo", "geofence_radius_m", "geofence_active",
		"is_primary", "pic_name", "phone",
		"status", "active_placement_count",
		"created_at", "updated_at",
	}
	for _, k := range requiredKeys {
		if _, ok := first[k]; !ok {
			t.Errorf("site data[0] missing key: %s", k)
		}
	}

	// geofence_active must be bool.
	if _, ok := first["geofence_active"].(bool); !ok {
		t.Errorf("geofence_active is not bool: %T", first["geofence_active"])
	}
	// Since we have GeoLat+GeoLng, geofence_active must be true.
	if first["geofence_active"] != true {
		t.Errorf("geofence_active = %v, want true (geo coordinates present)", first["geofence_active"])
	}
	// geo must be an object with lat, lng.
	geo, ok := first["geo"].(map[string]any)
	if !ok {
		t.Errorf("geo is not an object: %T", first["geo"])
	} else {
		if _, ok := geo["lat"]; !ok {
			t.Error("geo missing key: lat")
		}
		if _, ok := geo["lng"]; !ok {
			t.Error("geo missing key: lng")
		}
	}
	// status UPPERCASE.
	if first["status"] != "ACTIVE" {
		t.Errorf("site status = %v, want ACTIVE", first["status"])
	}
	// is_primary must be bool.
	if _, ok := first["is_primary"].(bool); !ok {
		t.Errorf("is_primary is not bool: %T", first["is_primary"])
	}
	// geofence_radius_m must be a number.
	if _, ok := first["geofence_radius_m"].(float64); !ok {
		t.Errorf("geofence_radius_m is not a number: %T", first["geofence_radius_m"])
	}
}

func TestListSites_NoGeo_GeofenceActiveFalse(t *testing.T) {
	h := newCompanyHarness(t)
	companies := h.seedCompany(1)
	companyID := companies[0].ID

	// Seed a site WITHOUT geo coordinates.
	h.repo.addSite(domain.Site{
		ID:              "SWP-SITE-NOGEO",
		ClientCompanyID: companyID,
		Name:            "No Geo Site",
		Address:         "Jl. Test 1",
		GeofenceRadiusM: 100,
		IsPrimary:       true,
		Status:          "active",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	})

	rr := h.do("GET", "/client-companies/"+companyID+"/sites", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data := body["data"].([]any)
	if len(data) == 0 {
		t.Fatal("expected at least one site")
	}
	first := data[0].(map[string]any)

	// geofence_active must be false when no coordinates.
	if first["geofence_active"] != false {
		t.Errorf("geofence_active = %v, want false (no geo coordinates)", first["geofence_active"])
	}
	// geo must be null.
	if first["geo"] != nil {
		t.Errorf("geo = %v, want null (no coordinates)", first["geo"])
	}
}

func TestCreateSite_201(t *testing.T) {
	h := newCompanyHarness(t)
	companies := h.seedCompany(1)
	companyID := companies[0].ID

	rr := h.do("POST", "/client-companies/"+companyID+"/sites", map[string]any{
		"name":    "New Site",
		"address": "Jl. Baru 1",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Error("missing Location header")
	}

	body := decodeBody(t, rr)
	requiredKeys := []string{
		"id", "client_company_id", "name", "address",
		"geo", "geofence_radius_m", "geofence_active",
		"is_primary", "status", "created_at", "updated_at",
	}
	for _, k := range requiredKeys {
		if _, ok := body[k]; !ok {
			t.Errorf("create site response missing key: %s", k)
		}
	}
}

func TestCreateSite_400_GeofenceRadiusInvalid(t *testing.T) {
	h := newCompanyHarness(t)
	companies := h.seedCompany(1)
	companyID := companies[0].ID

	// 5000 meters is outside the 25–1000 valid range.
	rr := h.do("POST", "/client-companies/"+companyID+"/sites", map[string]any{
		"name":              "Bad Geofence Site",
		"address":           "Jl. Test 1",
		"geofence_radius_m": 5000,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 GEOFENCE_RADIUS_INVALID, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "GEOFENCE_RADIUS_INVALID" {
		t.Errorf("error.code = %v, want GEOFENCE_RADIUS_INVALID", errObj["code"])
	}
}

func TestUpdateSite_200_IsPrimary(t *testing.T) {
	h := newCompanyHarness(t)
	companies := h.seedCompany(1)
	companyID := companies[0].ID

	// Seed two sites: site1 is primary, site2 is not.
	now := time.Now().UTC()
	h.repo.addSite(domain.Site{
		ID:              "SWP-SITE-P1",
		ClientCompanyID: companyID,
		Name:            "Primary Site",
		Address:         "Jl. Primary 1",
		GeofenceRadiusM: 100,
		IsPrimary:       true,
		Status:          "active",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	h.repo.addSite(domain.Site{
		ID:              "SWP-SITE-P2",
		ClientCompanyID: companyID,
		Name:            "Secondary Site",
		Address:         "Jl. Secondary 1",
		GeofenceRadiusM: 100,
		IsPrimary:       false,
		Status:          "active",
		CreatedAt:       now.Add(time.Minute),
		UpdatedAt:       now.Add(time.Minute),
	})

	// Update site2 to is_primary=true.
	rr := h.do("PATCH", "/sites/SWP-SITE-P2", map[string]any{
		"is_primary": true,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["is_primary"] != true {
		t.Errorf("is_primary = %v, want true after promotion", body["is_primary"])
	}
}

func TestDeactivateSite_200_Then_409(t *testing.T) {
	h := newCompanyHarness(t)
	companies := h.seedCompany(1)
	companyID := companies[0].ID
	site := h.seedSite(companyID)

	rr := h.do("POST", "/sites/"+site.ID+":deactivate", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("deactivate site: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "INACTIVE" {
		t.Errorf("after deactivate site status = %v, want INACTIVE", body["status"])
	}

	// Second deactivate → 409.
	rr2 := h.do("POST", "/sites/"+site.ID+":deactivate", nil)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("deactivate site again: expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestGetSite_200(t *testing.T) {
	h := newCompanyHarness(t)
	companies := h.seedCompany(1)
	companyID := companies[0].ID
	site := h.seedSite(companyID)

	rr := h.do("GET", "/sites/"+site.ID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["id"] != site.ID {
		t.Errorf("site id = %v, want %s", body["id"], site.ID)
	}
}

func TestGetSite_404(t *testing.T) {
	h := newCompanyHarness(t)

	rr := h.do("GET", "/sites/SWP-SITE-NONEXIST", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("error.code = %v, want NOT_FOUND", errObj["code"])
	}
}

func TestListSites_404_CompanyNotFound(t *testing.T) {
	h := newCompanyHarness(t)

	rr := h.do("GET", "/client-companies/SWP-CMP-GHOST/sites", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCompanyRBAC_Agent_403_OnWrite(t *testing.T) {
	h := newCompanyHarness(t)
	h.seedCompany(1)

	// agent should be blocked from POST /client-companies.
	h.principal = auth.Principal{UserID: "SWP-USR-AGENT", Role: auth.RoleAgent}

	rr := h.do("POST", "/client-companies", map[string]any{
		"name":    "Agent Co",
		"address": "Jl. Agent 1",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("agent POST /client-companies: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("error.code = %v, want FORBIDDEN", errObj["code"])
	}
}

// errUnique satisfies the errors.Is interface check via string matching used by isUniqueViolation.
var _ = errors.New // ensure errors package is used
