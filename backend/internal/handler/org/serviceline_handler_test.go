// Package org_test — contract tests for E2 service-line + position endpoints.
// Pattern: httptest + real ServiceLineService wired to an in-memory
// fakeServiceLineRepo (no DB). Principal injection via the closure middleware
// defined here. Mirrors the companies_handler_test.go pattern.
package org_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	orghandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/org"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	orgsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/org"
)

// ---------------------------------------------------------------------------
// fakeServiceLineRepo — in-memory implementation of orgsvc.ServiceLineRepository
// ---------------------------------------------------------------------------

type fakeServiceLineRepo struct {
	lines     map[string]domain.ServiceLine
	positions map[string]domain.Position

	// error overrides (set per-test to trigger error paths)
	createLineErr     error
	createPositionErr error
}

func newFakeServiceLineRepo() *fakeServiceLineRepo {
	return &fakeServiceLineRepo{
		lines:     make(map[string]domain.ServiceLine),
		positions: make(map[string]domain.Position),
	}
}

var slCounter int

func (r *fakeServiceLineRepo) addLine(sl domain.ServiceLine) {
	r.lines[sl.ID] = sl
}

func (r *fakeServiceLineRepo) addPosition(p domain.Position) {
	r.positions[p.ID] = p
}

func (r *fakeServiceLineRepo) ListServiceLines(_ context.Context, f domain.ServiceLineFilter) ([]domain.ServiceLine, error) {
	var all []domain.ServiceLine
	for _, sl := range r.lines {
		if f.Status != nil && sl.Status != *f.Status {
			continue
		}
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if sl.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if sl.CreatedAt.Equal(*f.CursorCreatedAt) && sl.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, sl)
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

func (r *fakeServiceLineRepo) GetServiceLineByID(_ context.Context, id string) (domain.ServiceLine, error) {
	sl, ok := r.lines[id]
	if !ok {
		return domain.ServiceLine{}, domain.ErrNotFound
	}
	return sl, nil
}

func (r *fakeServiceLineRepo) CountActivePositionsForLine(_ context.Context, lineID string) (int64, error) {
	var count int64
	for _, p := range r.positions {
		if p.ServiceLineID == lineID && p.Status == "active" {
			count++
		}
	}
	return count, nil
}

func (r *fakeServiceLineRepo) CreateServiceLine(_ context.Context, _ pgx.Tx, name string) (domain.ServiceLine, error) {
	if r.createLineErr != nil {
		return domain.ServiceLine{}, r.createLineErr
	}
	slCounter++
	now := time.Now().UTC()
	id := "SWP-SVC-T" + itoa(slCounter)
	sl := domain.ServiceLine{
		ID:        id,
		Name:      name,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.lines[id] = sl
	return sl, nil
}

func (r *fakeServiceLineRepo) UpdateServiceLine(_ context.Context, _ pgx.Tx, id, name string) (domain.ServiceLine, error) {
	sl, ok := r.lines[id]
	if !ok {
		return domain.ServiceLine{}, domain.ErrNotFound
	}
	sl.Name = name
	sl.UpdatedAt = time.Now().UTC()
	r.lines[id] = sl
	return sl, nil
}

func (r *fakeServiceLineRepo) SetServiceLineStatus(_ context.Context, _ pgx.Tx, id, status string) (domain.ServiceLine, error) {
	sl, ok := r.lines[id]
	if !ok {
		return domain.ServiceLine{}, domain.ErrNotFound
	}
	sl.Status = status
	sl.UpdatedAt = time.Now().UTC()
	r.lines[id] = sl
	return sl, nil
}

func (r *fakeServiceLineRepo) ListPositionsForLine(_ context.Context, lineID string, f domain.PositionFilter) ([]domain.Position, error) {
	var all []domain.Position
	for _, p := range r.positions {
		if p.ServiceLineID != lineID {
			continue
		}
		if f.Status != nil && p.Status != *f.Status {
			continue
		}
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if p.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if p.CreatedAt.Equal(*f.CursorCreatedAt) && p.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, p)
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

func (r *fakeServiceLineRepo) GetPositionByID(_ context.Context, id string) (domain.Position, error) {
	p, ok := r.positions[id]
	if !ok {
		return domain.Position{}, domain.ErrNotFound
	}
	return p, nil
}

var posCounter int

func (r *fakeServiceLineRepo) CreatePosition(_ context.Context, _ pgx.Tx, p orgsvc.CreatePositionParams) (domain.Position, error) {
	if r.createPositionErr != nil {
		return domain.Position{}, r.createPositionErr
	}
	posCounter++
	now := time.Now().UTC()
	id := "SWP-POS-T" + itoa(posCounter)
	pos := domain.Position{
		ID:            id,
		ServiceLineID: p.ServiceLineID,
		Name:          p.Name,
		Alias:         p.Alias,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	r.positions[id] = pos
	return pos, nil
}

func (r *fakeServiceLineRepo) UpdatePosition(_ context.Context, _ pgx.Tx, p orgsvc.UpdatePositionParams) (domain.Position, error) {
	pos, ok := r.positions[p.ID]
	if !ok {
		return domain.Position{}, domain.ErrNotFound
	}
	pos.Name = p.Name
	pos.Alias = p.Alias
	pos.UpdatedAt = time.Now().UTC()
	r.positions[p.ID] = pos
	return pos, nil
}

func (r *fakeServiceLineRepo) SetPositionStatus(_ context.Context, _ pgx.Tx, id, status string) (domain.Position, error) {
	p, ok := r.positions[id]
	if !ok {
		return domain.Position{}, domain.ErrNotFound
	}
	p.Status = status
	p.UpdatedAt = time.Now().UTC()
	r.positions[id] = p
	return p, nil
}

func (r *fakeServiceLineRepo) SoftDeletePosition(_ context.Context, _ pgx.Tx, id string) error {
	_, ok := r.positions[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(r.positions, id)
	return nil
}

// Compile-time interface check.
var _ orgsvc.ServiceLineRepository = (*fakeServiceLineRepo)(nil)

// ---------------------------------------------------------------------------
// Test harness for service lines + positions
// ---------------------------------------------------------------------------

type serviceLineHarness struct {
	router    *chi.Mux
	repo      *fakeServiceLineRepo
	principal auth.Principal
}

func newServiceLineHarness(t *testing.T) *serviceLineHarness {
	t.Helper()
	repo := newFakeServiceLineRepo()
	svc := orgsvc.NewServiceLineService(repo, &fakeTxRunner{})
	h := orghandler.NewServiceLineHandler(svc)

	fh := &serviceLineHarness{
		repo:      repo,
		principal: auth.Principal{UserID: "SWP-USR-ADMIN", Role: auth.RoleSuperAdmin},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	// Dynamic principal injection — reads fh.principal per request.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), fh.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	// Reads: all roles can list/get.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
		r.Get("/service-lines", h.ListServiceLines)
		r.Get("/service-lines/{service_line_id}", h.GetServiceLine)
		r.Get("/service-lines/{service_line_id}/positions", h.ListPositionsInServiceLine)
	})
	// Service-line writes: super_admin only.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin))
		r.Post("/service-lines", h.CreateServiceLine)
		r.Patch("/service-lines/{service_line_id}", h.UpdateServiceLine)
		r.Post("/service-lines/{service_line_id}:discontinue", h.DiscontinueServiceLine)
	})
	// Position writes: super_admin + hr_admin.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Post("/service-lines/{service_line_id}/positions", h.CreatePosition)
		r.Patch("/positions/{position_id}", h.UpdatePosition)
		r.Delete("/positions/{position_id}", h.SoftDeletePosition)
	})

	fh.router = r
	return fh
}

func (h *serviceLineHarness) do(method, path string, body any) *httptest.ResponseRecorder {
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

func (h *serviceLineHarness) seedLine(name string) domain.ServiceLine {
	slCounter++
	now := time.Now().UTC()
	id := "SWP-SVC-S" + itoa(slCounter)
	sl := domain.ServiceLine{
		ID:        id,
		Name:      name,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	h.repo.addLine(sl)
	return sl
}

func (h *serviceLineHarness) seedPosition(lineID, name string) domain.Position {
	posCounter++
	now := time.Now().UTC()
	id := "SWP-POS-S" + itoa(posCounter)
	p := domain.Position{
		ID:            id,
		ServiceLineID: lineID,
		Name:          name,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	h.repo.addPosition(p)
	return p
}

// ---------------------------------------------------------------------------
// Task 2: Service Lines
// ---------------------------------------------------------------------------

func TestListServiceLines_ShapeAndEnvelope(t *testing.T) {
	h := newServiceLineHarness(t)
	sl := h.seedLine("Facility Services")
	// Add a position so position_count can be verified from response (we seed it in repo).
	h.seedPosition(sl.ID, "Cleaning Staff")

	rr := h.do("GET", "/service-lines", nil)
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
		t.Fatal("expected at least one service line in data")
	}

	first := data[0].(map[string]any)
	requiredKeys := []string{"id", "name", "status", "position_count", "created_at", "updated_at"}
	for _, k := range requiredKeys {
		if _, ok := first[k]; !ok {
			t.Errorf("service line data[0] missing key: %s", k)
		}
	}

	// status must be UPPERCASE.
	if first["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", first["status"])
	}
	// position_count must be a number.
	if _, ok := first["position_count"].(float64); !ok {
		t.Errorf("position_count is not a number: %T", first["position_count"])
	}
}

func TestCreateServiceLine_201_SuperAdmin(t *testing.T) {
	h := newServiceLineHarness(t)
	// principal defaults to super_admin.

	rr := h.do("POST", "/service-lines", map[string]any{"name": "Parking Services"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Error("missing Location header on 201")
	}

	body := decodeBody(t, rr)
	for _, k := range []string{"id", "name", "status", "position_count", "created_at", "updated_at"} {
		if _, ok := body[k]; !ok {
			t.Errorf("create service line response missing key: %s", k)
		}
	}
	if body["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", body["status"])
	}
}

func TestCreateServiceLine_403_HRAdmin(t *testing.T) {
	h := newServiceLineHarness(t)
	// Service-line writes are super_admin only — hr_admin must get 403.
	h.principal = auth.Principal{UserID: "SWP-USR-HR", Role: auth.RoleHRAdmin}

	rr := h.do("POST", "/service-lines", map[string]any{"name": "New Line"})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("hr_admin POST /service-lines: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("error.code = %v, want FORBIDDEN", errObj["code"])
	}
}

func TestCreateServiceLine_409_Conflict(t *testing.T) {
	h := newServiceLineHarness(t)
	h.repo.createLineErr = errUnique{}

	rr := h.do("POST", "/service-lines", map[string]any{"name": "Duplicate Line"})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "CONFLICT" {
		t.Errorf("error.code = %v, want CONFLICT", errObj["code"])
	}
}

func TestUpdateServiceLine_200(t *testing.T) {
	h := newServiceLineHarness(t)
	sl := h.seedLine("Old Name")

	rr := h.do("PATCH", "/service-lines/"+sl.ID, map[string]any{"name": "New Name"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["name"] != "New Name" {
		t.Errorf("name = %v, want 'New Name'", body["name"])
	}
}

func TestDiscontinueServiceLine_200(t *testing.T) {
	h := newServiceLineHarness(t)
	// Seed line without active positions — discontinue should succeed.
	sl := h.seedLine("Empty Line")

	rr := h.do("POST", "/service-lines/"+sl.ID+":discontinue", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "INACTIVE" {
		t.Errorf("status = %v, want INACTIVE", body["status"])
	}
}

func TestDiscontinueServiceLine_409_ServiceLineInUse(t *testing.T) {
	h := newServiceLineHarness(t)
	sl := h.seedLine("Active Line")
	// Add an active position to block discontinuation.
	h.seedPosition(sl.ID, "Active Position")

	rr := h.do("POST", "/service-lines/"+sl.ID+":discontinue", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 SERVICE_LINE_IN_USE, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "SERVICE_LINE_IN_USE" {
		t.Errorf("error.code = %v, want SERVICE_LINE_IN_USE", errObj["code"])
	}
}

// ---------------------------------------------------------------------------
// Task 2: Positions
// ---------------------------------------------------------------------------

func TestListPositionsInServiceLine_Shape(t *testing.T) {
	h := newServiceLineHarness(t)
	sl := h.seedLine("Facility Services")
	h.seedPosition(sl.ID, "Cleaning Staff")

	rr := h.do("GET", "/service-lines/"+sl.ID+"/positions", nil)
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
		t.Fatal("expected at least one position in data")
	}
	first := data[0].(map[string]any)
	for _, k := range []string{"id", "service_line_id", "name", "alias", "status", "created_at", "updated_at"} {
		if _, ok := first[k]; !ok {
			t.Errorf("position data[0] missing key: %s", k)
		}
	}
	if first["status"] != "ACTIVE" {
		t.Errorf("position status = %v, want ACTIVE", first["status"])
	}
}

func TestListPositionsInServiceLine_404_LineNotFound(t *testing.T) {
	h := newServiceLineHarness(t)

	rr := h.do("GET", "/service-lines/SWP-SVC-GHOST/positions", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreatePosition_201(t *testing.T) {
	h := newServiceLineHarness(t)
	sl := h.seedLine("Building Management")

	rr := h.do("POST", "/service-lines/"+sl.ID+"/positions", map[string]any{
		"name":  "Security Guard",
		"alias": "Security",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Error("missing Location header")
	}
	body := decodeBody(t, rr)
	for _, k := range []string{"id", "service_line_id", "name", "alias", "status", "created_at", "updated_at"} {
		if _, ok := body[k]; !ok {
			t.Errorf("create position response missing key: %s", k)
		}
	}
	if body["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", body["status"])
	}
}

func TestCreatePosition_409_PositionInUse(t *testing.T) {
	h := newServiceLineHarness(t)
	sl := h.seedLine("Parking")
	h.repo.createPositionErr = errUnique{}

	rr := h.do("POST", "/service-lines/"+sl.ID+"/positions", map[string]any{"name": "Duplicate Position"})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 POSITION_IN_USE, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "POSITION_IN_USE" {
		t.Errorf("error.code = %v, want POSITION_IN_USE", errObj["code"])
	}
}

func TestUpdatePosition_200(t *testing.T) {
	h := newServiceLineHarness(t)
	sl := h.seedLine("Facility Services")
	pos := h.seedPosition(sl.ID, "Old Position Name")

	rr := h.do("PATCH", "/positions/"+pos.ID, map[string]any{"name": "New Position Name"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["name"] != "New Position Name" {
		t.Errorf("name = %v, want 'New Position Name'", body["name"])
	}
}

func TestSoftDeletePosition_204(t *testing.T) {
	h := newServiceLineHarness(t)
	sl := h.seedLine("Parking")
	pos := h.seedPosition(sl.ID, "Attendant")

	rr := h.do("DELETE", "/positions/"+pos.ID, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	// Body must be empty.
	if strings.TrimSpace(rr.Body.String()) != "" {
		t.Errorf("expected empty body on 204, got: %s", rr.Body.String())
	}
}

func TestSoftDeletePosition_404_NotFound(t *testing.T) {
	h := newServiceLineHarness(t)

	rr := h.do("DELETE", "/positions/SWP-POS-GHOST", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("error.code = %v, want NOT_FOUND", errObj["code"])
	}
}
