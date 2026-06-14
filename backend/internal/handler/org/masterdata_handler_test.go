// Package org_test — contract tests for E2 master-data endpoints:
// leave types, attendance codes, overtime rules.
// Pattern: httptest + real MasterDataService wired to an in-memory
// fakeMasterDataRepo (no DB). Mirrors companies/serviceline handler tests.
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
// fakeMasterDataRepo — in-memory implementation of orgsvc.MasterDataRepository
// ---------------------------------------------------------------------------

type fakeMasterDataRepo struct {
	leaveTypes      map[string]domain.LeaveType
	attendanceCodes map[string]domain.AttendanceCode
	overtimeRules   map[string]domain.OvertimeRule

	// error overrides (set per-test to trigger error paths)
	createLTErr  error
	createACErr  error
	createOTRErr error
}

func newFakeMasterDataRepo() *fakeMasterDataRepo {
	return &fakeMasterDataRepo{
		leaveTypes:      make(map[string]domain.LeaveType),
		attendanceCodes: make(map[string]domain.AttendanceCode),
		overtimeRules:   make(map[string]domain.OvertimeRule),
	}
}

var ltCounter, acCounter, otrCounter int

// --- Leave Types ---

func (r *fakeMasterDataRepo) ListLeaveTypes(_ context.Context, f domain.LeaveTypeFilter) ([]domain.LeaveType, error) {
	var all []domain.LeaveType
	for _, lt := range r.leaveTypes {
		if f.Status != nil && lt.Status != *f.Status {
			continue
		}
		if f.IsAnnual != nil && lt.IsAnnual != *f.IsAnnual {
			continue
		}
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if lt.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if lt.CreatedAt.Equal(*f.CursorCreatedAt) && lt.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, lt)
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

func (r *fakeMasterDataRepo) GetLeaveTypeByID(_ context.Context, id string) (domain.LeaveType, error) {
	lt, ok := r.leaveTypes[id]
	if !ok {
		return domain.LeaveType{}, domain.ErrNotFound
	}
	return lt, nil
}

func (r *fakeMasterDataRepo) CreateLeaveType(_ context.Context, _ pgx.Tx, p orgsvc.CreateLeaveTypeParams) (domain.LeaveType, error) {
	if r.createLTErr != nil {
		return domain.LeaveType{}, r.createLTErr
	}
	ltCounter++
	now := time.Now().UTC()
	id := "SWP-LT-T" + itoa(ltCounter)
	lt := domain.LeaveType{
		ID:                 id,
		Name:               p.Name,
		Code:               p.Code,
		Description:        p.Description,
		DefaultAnnualQuota: p.DefaultAnnualQuota,
		IsAnnual:           p.IsAnnual,
		RequiresDocument:   p.RequiresDocument,
		Color:              p.Color,
		Status:             "active",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	r.leaveTypes[id] = lt
	return lt, nil
}

func (r *fakeMasterDataRepo) UpdateLeaveType(_ context.Context, _ pgx.Tx, p orgsvc.UpdateLeaveTypeParams) (domain.LeaveType, error) {
	lt, ok := r.leaveTypes[p.ID]
	if !ok {
		return domain.LeaveType{}, domain.ErrNotFound
	}
	lt.Name = p.Name
	lt.Code = p.Code
	lt.Description = p.Description
	lt.DefaultAnnualQuota = p.DefaultAnnualQuota
	lt.IsAnnual = p.IsAnnual
	lt.RequiresDocument = p.RequiresDocument
	lt.Color = p.Color
	lt.UpdatedAt = time.Now().UTC()
	r.leaveTypes[p.ID] = lt
	return lt, nil
}

func (r *fakeMasterDataRepo) SoftDeleteLeaveType(_ context.Context, _ pgx.Tx, id string) error {
	_, ok := r.leaveTypes[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(r.leaveTypes, id)
	return nil
}

// --- Attendance Codes ---

func (r *fakeMasterDataRepo) ListAttendanceCodes(_ context.Context, f domain.AttendanceCodeFilter) ([]domain.AttendanceCode, error) {
	var all []domain.AttendanceCode
	for _, ac := range r.attendanceCodes {
		if f.Status != nil && ac.Status != *f.Status {
			continue
		}
		if f.IsBillable != nil && ac.IsBillable != *f.IsBillable {
			continue
		}
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if ac.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if ac.CreatedAt.Equal(*f.CursorCreatedAt) && ac.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, ac)
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

func (r *fakeMasterDataRepo) GetAttendanceCodeByID(_ context.Context, id string) (domain.AttendanceCode, error) {
	ac, ok := r.attendanceCodes[id]
	if !ok {
		return domain.AttendanceCode{}, domain.ErrNotFound
	}
	return ac, nil
}

func (r *fakeMasterDataRepo) CreateAttendanceCode(_ context.Context, _ pgx.Tx, p orgsvc.CreateAttendanceCodeParams) (domain.AttendanceCode, error) {
	if r.createACErr != nil {
		return domain.AttendanceCode{}, r.createACErr
	}
	acCounter++
	now := time.Now().UTC()
	id := "SWP-AC-T" + itoa(acCounter)
	ac := domain.AttendanceCode{
		ID:                id,
		Code:              p.Code,
		Label:             p.Label,
		Description:       p.Description,
		Color:             p.Color,
		IsWorkday:         p.IsWorkday,
		IsPaid:            p.IsPaid,
		IsBillable:        p.IsBillable,
		NeedsVerification: p.NeedsVerification,
		Status:            "active",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	r.attendanceCodes[id] = ac
	return ac, nil
}

func (r *fakeMasterDataRepo) UpdateAttendanceCode(_ context.Context, _ pgx.Tx, p orgsvc.UpdateAttendanceCodeParams) (domain.AttendanceCode, error) {
	ac, ok := r.attendanceCodes[p.ID]
	if !ok {
		return domain.AttendanceCode{}, domain.ErrNotFound
	}
	ac.Code = p.Code
	ac.Label = p.Label
	ac.Description = p.Description
	ac.Color = p.Color
	ac.IsWorkday = p.IsWorkday
	ac.IsPaid = p.IsPaid
	ac.IsBillable = p.IsBillable
	ac.NeedsVerification = p.NeedsVerification
	ac.UpdatedAt = time.Now().UTC()
	r.attendanceCodes[p.ID] = ac
	return ac, nil
}

func (r *fakeMasterDataRepo) SoftDeleteAttendanceCode(_ context.Context, _ pgx.Tx, id string) error {
	_, ok := r.attendanceCodes[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(r.attendanceCodes, id)
	return nil
}

// --- Overtime Rules ---

func (r *fakeMasterDataRepo) ListOvertimeRules(_ context.Context, f domain.OvertimeRuleFilter) ([]domain.OvertimeRule, error) {
	var all []domain.OvertimeRule
	for _, otr := range r.overtimeRules {
		if f.Status != nil && otr.Status != *f.Status {
			continue
		}
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if otr.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if otr.CreatedAt.Equal(*f.CursorCreatedAt) && otr.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, otr)
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

func (r *fakeMasterDataRepo) GetOvertimeRuleByID(_ context.Context, id string) (domain.OvertimeRule, error) {
	otr, ok := r.overtimeRules[id]
	if !ok {
		return domain.OvertimeRule{}, domain.ErrNotFound
	}
	return otr, nil
}

func (r *fakeMasterDataRepo) CreateOvertimeRule(_ context.Context, _ pgx.Tx, p orgsvc.CreateOvertimeRuleParams) (domain.OvertimeRule, error) {
	if r.createOTRErr != nil {
		return domain.OvertimeRule{}, r.createOTRErr
	}
	otrCounter++
	now := time.Now().UTC()
	id := "SWP-OTR-T" + itoa(otrCounter)
	otr := domain.OvertimeRule{
		ID:                  id,
		Name:                p.Name,
		WeekdayRate:         p.WeekdayRate,
		RestdayRate:         p.RestdayRate,
		HolidayRate:         p.HolidayRate,
		MinMinutes:          p.MinMinutes,
		MaxMinutesPerDay:    p.MaxMinutesPerDay,
		PreApprovalRequired: p.PreApprovalRequired,
		Status:              "active",
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	r.overtimeRules[id] = otr
	return otr, nil
}

func (r *fakeMasterDataRepo) UpdateOvertimeRule(_ context.Context, _ pgx.Tx, p orgsvc.UpdateOvertimeRuleParams) (domain.OvertimeRule, error) {
	otr, ok := r.overtimeRules[p.ID]
	if !ok {
		return domain.OvertimeRule{}, domain.ErrNotFound
	}
	otr.Name = p.Name
	otr.WeekdayRate = p.WeekdayRate
	otr.RestdayRate = p.RestdayRate
	otr.HolidayRate = p.HolidayRate
	otr.MinMinutes = p.MinMinutes
	otr.MaxMinutesPerDay = p.MaxMinutesPerDay
	otr.PreApprovalRequired = p.PreApprovalRequired
	otr.UpdatedAt = time.Now().UTC()
	r.overtimeRules[p.ID] = otr
	return otr, nil
}

func (r *fakeMasterDataRepo) SoftDeleteOvertimeRule(_ context.Context, _ pgx.Tx, id string) error {
	_, ok := r.overtimeRules[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(r.overtimeRules, id)
	return nil
}

// Compile-time interface check.
var _ orgsvc.MasterDataRepository = (*fakeMasterDataRepo)(nil)

// ---------------------------------------------------------------------------
// Test harness for master data
// ---------------------------------------------------------------------------

type masterDataHarness struct {
	router    *chi.Mux
	repo      *fakeMasterDataRepo
	principal auth.Principal
}

func newMasterDataHarness(t *testing.T) *masterDataHarness {
	t.Helper()
	repo := newFakeMasterDataRepo()
	svc := orgsvc.NewMasterDataService(repo, &fakeTxRunner{})
	h := orghandler.NewMasterDataHandler(svc)

	fh := &masterDataHarness{
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

	// LT + AC reads: all 4 roles.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
		r.Get("/leave-types", h.ListLeaveTypes)
		r.Get("/attendance-codes", h.ListAttendanceCodes)
	})
	// OTR reads: all roles except agent (spec x-rbac).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
		r.Get("/overtime-rules", h.ListOvertimeRules)
	})
	// All writes: super_admin + hr_admin.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Post("/leave-types", h.CreateLeaveType)
		r.Patch("/leave-types/{leave_type_id}", h.UpdateLeaveType)
		r.Delete("/leave-types/{leave_type_id}", h.SoftDeleteLeaveType)
		r.Post("/attendance-codes", h.CreateAttendanceCode)
		r.Patch("/attendance-codes/{attendance_code_id}", h.UpdateAttendanceCode)
		r.Delete("/attendance-codes/{attendance_code_id}", h.SoftDeleteAttendanceCode)
		r.Post("/overtime-rules", h.CreateOvertimeRule)
		r.Patch("/overtime-rules/{overtime_rule_id}", h.UpdateOvertimeRule)
		r.Delete("/overtime-rules/{overtime_rule_id}", h.SoftDeleteOvertimeRule)
	})

	fh.router = r
	return fh
}

func (h *masterDataHarness) do(method, path string, body any) *httptest.ResponseRecorder {
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

func (h *masterDataHarness) seedLeaveType(name, code string, isAnnual bool) domain.LeaveType {
	ltCounter++
	now := time.Now().UTC()
	id := "SWP-LT-S" + itoa(ltCounter)
	lt := domain.LeaveType{
		ID:                 id,
		Name:               name,
		Code:               code,
		IsAnnual:           isAnnual,
		DefaultAnnualQuota: 12,
		Status:             "active",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	h.repo.leaveTypes[id] = lt
	return lt
}

func (h *masterDataHarness) seedAttendanceCode(code, label string) domain.AttendanceCode {
	acCounter++
	now := time.Now().UTC()
	id := "SWP-AC-S" + itoa(acCounter)
	ac := domain.AttendanceCode{
		ID:                id,
		Code:              code,
		Label:             label,
		IsWorkday:         true,
		IsPaid:            true,
		IsBillable:        true,
		NeedsVerification: false,
		Status:            "active",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	h.repo.attendanceCodes[id] = ac
	return ac
}

func (h *masterDataHarness) seedOvertimeRule(name string, weekdayRate, restdayRate, holidayRate float64, minMinutes int) domain.OvertimeRule {
	otrCounter++
	now := time.Now().UTC()
	id := "SWP-OTR-S" + itoa(otrCounter)
	otr := domain.OvertimeRule{
		ID:               id,
		Name:             name,
		WeekdayRate:      weekdayRate,
		RestdayRate:      restdayRate,
		HolidayRate:      holidayRate,
		MinMinutes:       minMinutes,
		MaxMinutesPerDay: 240,
		Status:           "active",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	h.repo.overtimeRules[id] = otr
	return otr
}

// ---------------------------------------------------------------------------
// Task 3: Leave Types
// ---------------------------------------------------------------------------

func TestListLeaveTypes_Shape(t *testing.T) {
	h := newMasterDataHarness(t)
	h.seedLeaveType("Cuti Tahunan", "ANNUAL", true)

	rr := h.do("GET", "/leave-types", nil)
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
		t.Fatal("expected at least one leave type in data")
	}
	first := data[0].(map[string]any)
	requiredKeys := []string{
		"id", "name", "code", "description",
		"default_annual_quota", "is_annual", "requires_document",
		"color", "status", "created_at", "updated_at",
	}
	for _, k := range requiredKeys {
		if _, ok := first[k]; !ok {
			t.Errorf("leave_type data[0] missing key: %s", k)
		}
	}
	if first["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", first["status"])
	}
	if _, ok := first["is_annual"].(bool); !ok {
		t.Errorf("is_annual is not bool: %T", first["is_annual"])
	}
	if _, ok := first["requires_document"].(bool); !ok {
		t.Errorf("requires_document is not bool: %T", first["requires_document"])
	}
	if _, ok := first["default_annual_quota"].(float64); !ok {
		t.Errorf("default_annual_quota is not a number: %T", first["default_annual_quota"])
	}
}

func TestCreateLeaveType_201(t *testing.T) {
	h := newMasterDataHarness(t)

	rr := h.do("POST", "/leave-types", map[string]any{
		"name":      "Cuti Tahunan",
		"code":      "ANNUAL",
		"is_annual": true,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Error("missing Location header on 201")
	}
	body := decodeBody(t, rr)
	if body["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", body["status"])
	}
}

func TestCreateLeaveType_409_Conflict(t *testing.T) {
	h := newMasterDataHarness(t)
	h.repo.createLTErr = errUnique{}

	rr := h.do("POST", "/leave-types", map[string]any{
		"name": "Duplicate LT",
		"code": "DUP",
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

func TestUpdateLeaveType_200(t *testing.T) {
	h := newMasterDataHarness(t)
	lt := h.seedLeaveType("Old Name", "OLD", false)

	rr := h.do("PATCH", "/leave-types/"+lt.ID, map[string]any{"name": "New Name"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["name"] != "New Name" {
		t.Errorf("name = %v, want 'New Name'", body["name"])
	}
}

func TestSoftDeleteLeaveType_204(t *testing.T) {
	h := newMasterDataHarness(t)
	lt := h.seedLeaveType("Delete Me", "DEL", false)

	rr := h.do("DELETE", "/leave-types/"+lt.ID, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if strings.TrimSpace(rr.Body.String()) != "" {
		t.Errorf("expected empty body on 204, got: %s", rr.Body.String())
	}
}

func TestSoftDeleteLeaveType_404(t *testing.T) {
	h := newMasterDataHarness(t)

	rr := h.do("DELETE", "/leave-types/SWP-LT-GHOST", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Task 3: Attendance Codes
// ---------------------------------------------------------------------------

func TestListAttendanceCodes_Shape(t *testing.T) {
	h := newMasterDataHarness(t)
	h.seedAttendanceCode("PRESENT", "Hadir")

	rr := h.do("GET", "/attendance-codes", nil)
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
		t.Fatal("expected at least one attendance code in data")
	}
	first := data[0].(map[string]any)
	requiredKeys := []string{
		"id", "code", "label", "description",
		"color", "is_workday", "is_paid", "is_billable", "needs_verification",
		"status", "created_at", "updated_at",
	}
	for _, k := range requiredKeys {
		if _, ok := first[k]; !ok {
			t.Errorf("attendance_code data[0] missing key: %s", k)
		}
	}
	if first["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", first["status"])
	}
	for _, boolKey := range []string{"is_workday", "is_paid", "is_billable", "needs_verification"} {
		if _, ok := first[boolKey].(bool); !ok {
			t.Errorf("%s is not bool: %T", boolKey, first[boolKey])
		}
	}
}

func TestCreateAttendanceCode_201(t *testing.T) {
	h := newMasterDataHarness(t)

	rr := h.do("POST", "/attendance-codes", map[string]any{
		"code":       "LATE",
		"label":      "Terlambat",
		"is_workday": true,
		"is_paid":    true,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Error("missing Location header on 201")
	}
	body := decodeBody(t, rr)
	if body["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", body["status"])
	}
}

func TestCreateAttendanceCode_409_Conflict(t *testing.T) {
	h := newMasterDataHarness(t)
	h.repo.createACErr = errUnique{}

	rr := h.do("POST", "/attendance-codes", map[string]any{"code": "DUP", "label": "Duplicate"})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "CONFLICT" {
		t.Errorf("error.code = %v, want CONFLICT", errObj["code"])
	}
}

func TestUpdateAttendanceCode_200(t *testing.T) {
	h := newMasterDataHarness(t)
	ac := h.seedAttendanceCode("OLD", "Old Label")

	rr := h.do("PATCH", "/attendance-codes/"+ac.ID, map[string]any{"label": "New Label"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["label"] != "New Label" {
		t.Errorf("label = %v, want 'New Label'", body["label"])
	}
}

func TestSoftDeleteAttendanceCode_204(t *testing.T) {
	h := newMasterDataHarness(t)
	ac := h.seedAttendanceCode("DEL", "Delete Me")

	rr := h.do("DELETE", "/attendance-codes/"+ac.ID, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if strings.TrimSpace(rr.Body.String()) != "" {
		t.Errorf("expected empty body on 204, got: %s", rr.Body.String())
	}
}

func TestSoftDeleteAttendanceCode_404(t *testing.T) {
	h := newMasterDataHarness(t)

	rr := h.do("DELETE", "/attendance-codes/SWP-AC-GHOST", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Task 3: Overtime Rules
// ---------------------------------------------------------------------------

func TestListOvertimeRules_Shape(t *testing.T) {
	h := newMasterDataHarness(t)
	h.seedOvertimeRule("Default OT", 1.5, 2.0, 3.0, 30)

	rr := h.do("GET", "/overtime-rules", nil)
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
		t.Fatal("expected at least one overtime rule in data")
	}
	first := data[0].(map[string]any)
	requiredKeys := []string{
		"id", "name",
		"weekday_rate", "restday_rate", "holiday_rate",
		"min_minutes", "max_minutes_per_day", "pre_approval_required",
		"status", "created_at", "updated_at",
	}
	for _, k := range requiredKeys {
		if _, ok := first[k]; !ok {
			t.Errorf("overtime_rule data[0] missing key: %s", k)
		}
	}
	if first["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", first["status"])
	}
	// Rates must be numbers.
	for _, rateKey := range []string{"weekday_rate", "restday_rate", "holiday_rate"} {
		if _, ok := first[rateKey].(float64); !ok {
			t.Errorf("%s is not a number: %T", rateKey, first[rateKey])
		}
	}
	// Rates must round-trip cleanly (1.5 not 1.5000001).
	if first["weekday_rate"] != 1.5 {
		t.Errorf("weekday_rate = %v, want 1.5", first["weekday_rate"])
	}
	if first["restday_rate"] != 2.0 {
		t.Errorf("restday_rate = %v, want 2.0", first["restday_rate"])
	}
	if first["holiday_rate"] != 3.0 {
		t.Errorf("holiday_rate = %v, want 3.0", first["holiday_rate"])
	}
	// Overtime rules are GLOBAL ONLY — service_line_id must NOT appear.
	if _, exists := first["service_line_id"]; exists {
		t.Error("service_line_id key must be absent (overtime is global only)")
	}
	// pre_approval_required must be bool.
	if _, ok := first["pre_approval_required"].(bool); !ok {
		t.Errorf("pre_approval_required is not bool: %T", first["pre_approval_required"])
	}
	// min_minutes/max_minutes_per_day must be numbers.
	if _, ok := first["min_minutes"].(float64); !ok {
		t.Errorf("min_minutes is not a number: %T", first["min_minutes"])
	}
}

func TestCreateOvertimeRule_201(t *testing.T) {
	h := newMasterDataHarness(t)

	rr := h.do("POST", "/overtime-rules", map[string]any{
		"name":         "Parking OT",
		"weekday_rate": 1.5,
		"restday_rate": 2.0,
		"holiday_rate": 3.0,
		"min_minutes":  30,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Error("missing Location header on 201")
	}
	body := decodeBody(t, rr)
	if body["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", body["status"])
	}
	// Overtime rules are GLOBAL ONLY — service_line_id must NOT appear.
	if _, exists := body["service_line_id"]; exists {
		t.Error("service_line_id key must be absent (overtime is global only)")
	}
}

func TestCreateOvertimeRule_422_RuleViolation_MinMinutes(t *testing.T) {
	h := newMasterDataHarness(t)

	// OR-1: min_minutes < 30 must return 422 RULE_VIOLATION with field error.
	rr := h.do("POST", "/overtime-rules", map[string]any{
		"name":        "Invalid OT",
		"min_minutes": 20,
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 RULE_VIOLATION, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "RULE_VIOLATION" {
		t.Errorf("error.code = %v, want RULE_VIOLATION", errObj["code"])
	}
	// fields must contain min_minutes.
	fields, _ := errObj["fields"].(map[string]any)
	if _, ok := fields["min_minutes"]; !ok {
		t.Errorf("error.fields missing min_minutes key; fields = %v", fields)
	}
}

func TestCreateOvertimeRule_409_Conflict(t *testing.T) {
	h := newMasterDataHarness(t)
	h.repo.createOTRErr = errUnique{}

	rr := h.do("POST", "/overtime-rules", map[string]any{
		"name":        "Duplicate OT",
		"min_minutes": 30,
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

func TestUpdateOvertimeRule_200(t *testing.T) {
	h := newMasterDataHarness(t)
	otr := h.seedOvertimeRule("Old OT Rule", 1.5, 2.0, 3.0, 30)

	rr := h.do("PATCH", "/overtime-rules/"+otr.ID, map[string]any{"name": "Updated OT Rule"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["name"] != "Updated OT Rule" {
		t.Errorf("name = %v, want 'Updated OT Rule'", body["name"])
	}
}

func TestSoftDeleteOvertimeRule_204(t *testing.T) {
	h := newMasterDataHarness(t)
	otr := h.seedOvertimeRule("Delete Me OT", 1.5, 2.0, 3.0, 30)

	rr := h.do("DELETE", "/overtime-rules/"+otr.ID, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if strings.TrimSpace(rr.Body.String()) != "" {
		t.Errorf("expected empty body on 204, got: %s", rr.Body.String())
	}
}

func TestSoftDeleteOvertimeRule_404(t *testing.T) {
	h := newMasterDataHarness(t)

	rr := h.do("DELETE", "/overtime-rules/SWP-OTR-GHOST", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestOvertimeRuleAgent_403_OnList verifies agents cannot list overtime rules.
func TestOvertimeRuleAgent_403_OnList(t *testing.T) {
	h := newMasterDataHarness(t)
	h.principal = auth.Principal{UserID: "SWP-USR-AGENT", Role: auth.RoleAgent}

	rr := h.do("GET", "/overtime-rules", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("agent GET /overtime-rules: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}
