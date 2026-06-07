// Package people_test contains contract tests for the E2 people handler endpoints.
// These tests assert the EXACT JSON field names, types, and status codes required
// by the OpenAPI spec — the drift gate replacing server-side codegen.
//
// Pattern: httptest + real Service wired to an in-memory fakeEmployeeRepo (no DB).
// Principal injection via auth.WithPrincipal on the request context.
// Mirrors internal/handler/org/companies_handler_test.go exactly.
package people_test

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
	peoplehandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/people"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	peoplesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
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

// ---------------------------------------------------------------------------
// fakeEmployeeRepo — in-memory implementation of peoplesvc.EmployeeRepository.
// ---------------------------------------------------------------------------

type fakeEmployeeRepo struct {
	employees   map[string]domain.Employee
	nikIndex    map[string]string // nik → id
	loginPhones map[string]bool   // phones already taken by a seeded login (D2)

	// offboard cascade (OB-1) fixtures + capture
	activeAgreements map[string]string // employee_id → active agreement id
	placements       map[string][]string // employee_id → non-terminal placement ids

	closedAgreementID    string // last agreement closed via CloseAgreement
	closedAgreementReason string
	endedPlacementIDs    []string
	endLifecycleStatus   string
	endReason            string

	// error overrides (set per-test to trigger error paths)
	createErr error
}

func newFakeEmployeeRepo() *fakeEmployeeRepo {
	return &fakeEmployeeRepo{
		employees:        make(map[string]domain.Employee),
		nikIndex:         make(map[string]string),
		loginPhones:      make(map[string]bool),
		activeAgreements: make(map[string]string),
		placements:       make(map[string][]string),
	}
}

func (r *fakeEmployeeRepo) addEmployee(e domain.Employee) {
	r.employees[e.ID] = e
	r.nikIndex[e.NIK] = e.ID
}

func (r *fakeEmployeeRepo) ListEmployees(_ context.Context, f domain.EmployeeFilter) ([]domain.Employee, error) {
	var all []domain.Employee
	for _, e := range r.employees {
		if f.Status != nil && e.Status != *f.Status {
			continue
		}
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if e.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if e.CreatedAt.Equal(*f.CursorCreatedAt) && e.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, e)
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

func (r *fakeEmployeeRepo) GetEmployeeByID(_ context.Context, id string) (domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	return e, nil
}

func (r *fakeEmployeeRepo) GetEmployeeByNIK(_ context.Context, nik string) (domain.Employee, error) {
	id, ok := r.nikIndex[nik]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	return r.employees[id], nil
}

var empCounter int

func (r *fakeEmployeeRepo) CreateEmployee(_ context.Context, _ pgx.Tx, p peoplesvc.CreateEmployeeParams) (domain.Employee, error) {
	if r.createErr != nil {
		return domain.Employee{}, r.createErr
	}
	empCounter++
	now := time.Now().UTC()
	id := "SWP-EMP-" + itoa(empCounter)
	e := domain.Employee{
		ID:        id,
		FullName:  p.FullName,
		NIK:       p.NIK,
		NIP:       p.NIP,
		JoinAt:    p.JoinAt,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
		BankAccount: domain.BankAccount{
			BankName:          p.BankName,
			AccountNumber:     p.BankAccountNumber,
			AccountHolderName: p.BankAccountHolderName,
		},
	}
	if p.Phone != "" {
		s := p.Phone
		e.Phone = &s
	}
	if p.Address != "" {
		s := p.Address
		e.Address = &s
	}
	r.employees[id] = e
	r.nikIndex[e.NIK] = id
	return e, nil
}

func (r *fakeEmployeeRepo) UpdateEmployee(_ context.Context, _ pgx.Tx, p peoplesvc.UpdateEmployeeParams) (domain.Employee, error) {
	e, ok := r.employees[p.ID]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	e.FullName = p.FullName
	e.NIK = p.NIK
	e.NIP = p.NIP
	e.JoinAt = p.JoinAt
	if p.Phone != "" {
		e.Phone = &p.Phone
	}
	if p.Address != "" {
		e.Address = &p.Address
	}
	e.BankAccount = domain.BankAccount{
		BankName:          p.BankName,
		AccountNumber:     p.BankAccountNumber,
		AccountHolderName: p.BankAccountHolderName,
	}
	e.UpdatedAt = time.Now().UTC()
	r.employees[p.ID] = e
	return e, nil
}

func (r *fakeEmployeeRepo) SetEmployeeStatus(_ context.Context, _ pgx.Tx, id, status string) (domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	e.Status = status
	e.UpdatedAt = time.Now().UTC()
	r.employees[id] = e
	return e, nil
}

func (r *fakeEmployeeRepo) UserEmailTaken(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// UserPhoneTaken mirrors UserEmailTaken: phone is the primary login identifier (D2).
// Returns true if a seeded login already owns the (normalized) phone; default false.
func (r *fakeEmployeeRepo) UserPhoneTaken(_ context.Context, phone string) (bool, error) {
	return r.loginPhones[phone], nil
}

func (r *fakeEmployeeRepo) ProvisionLogin(_ context.Context, _ pgx.Tx, p peoplesvc.ProvisionLoginRepoParams) (string, error) {
	userID := "SWP-USR-test-" + p.EmployeeID
	if e, ok := r.employees[p.EmployeeID]; ok {
		uid := userID
		e.UserID = &uid
		r.employees[p.EmployeeID] = e
	}
	return userID, nil
}

func (r *fakeEmployeeRepo) RegenerateTempPassword(_ context.Context, _ pgx.Tx, _, _ string) error {
	return nil
}

func (r *fakeEmployeeRepo) DisableUserAndRevoke(_ context.Context, _ pgx.Tx, _ string) error {
	return nil
}

func (r *fakeEmployeeRepo) EnableUser(_ context.Context, _ pgx.Tx, _ string) error {
	return nil
}

// Offboard-cascade ports (F2.7 OB-1). The fake reports no active agreement and no
// placements, so the cascade is a no-op in handler tests (the cascade logic itself
// is exercised at the service layer).
func (r *fakeEmployeeRepo) GetActiveAgreementForEmployee(_ context.Context, _ string) (string, bool, error) {
	return "", false, nil
}

func (r *fakeEmployeeRepo) CloseAgreement(_ context.Context, _ pgx.Tx, _, _ string, _ time.Time) error {
	return nil
}

func (r *fakeEmployeeRepo) EndPlacementsForEmployee(_ context.Context, _ pgx.Tx, _, _, _ string, _ time.Time) ([]string, error) {
	return nil, nil
}

// Compile-time interface check.
var _ peoplesvc.EmployeeRepository = (*fakeEmployeeRepo)(nil)

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

type employeeHarness struct {
	router    *chi.Mux
	repo      *fakeEmployeeRepo
	principal auth.Principal
}

func newEmployeeHarness(t *testing.T) *employeeHarness {
	t.Helper()
	repo := newFakeEmployeeRepo()
	svc := peoplesvc.NewService(repo, &fakeTxRunner{})
	h := peoplehandler.NewHandler(svc)

	fh := &employeeHarness{
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

	// RBAC groups matching server.go people slice.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
		r.Get("/employees", h.ListEmployees)
		r.Get("/employees/{employee_id}", h.GetEmployee)
	})
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Post("/employees", h.CreateEmployee)
		r.Patch("/employees/{employee_id}", h.UpdateEmployee)
		r.Post("/employees/{employee_id}:deactivate", h.DeactivateEmployee)
		r.Post("/employees/{employee_id}:reactivate", h.ReactivateEmployee)
	})

	fh.router = r
	return fh
}

func (h *employeeHarness) do(method, path string, body any) *httptest.ResponseRecorder {
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

func (h *employeeHarness) seedEmployee(n int) []domain.Employee {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	employees := make([]domain.Employee, n)
	for i := 0; i < n; i++ {
		id := "SWP-EMP-SEED-" + itoa(i+1)
		nik := "3201234567890" + itoa(i+1)
		joinAt := base.Add(time.Duration(i) * time.Hour * 24)
		phone := "+628123456789" + itoa(i)
		e := domain.Employee{
			ID:       id,
			FullName: "Employee " + itoa(i+1),
			NIK:      nik,
			NIP:      "EMP-" + itoa(i+1),
			JoinAt:   joinAt,
			Status:   "active",
			Phone:    &phone,
			BankAccount: domain.BankAccount{
				BankName:          "BCA",
				AccountNumber:     "123456789",
				AccountHolderName: "Employee " + itoa(i+1),
			},
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
			UpdatedAt: base.Add(time.Duration(i) * time.Minute),
		}
		h.repo.addEmployee(e)
		employees[i] = e
	}
	return employees
}

// ---------------------------------------------------------------------------
// Tests: ListEmployees
// ---------------------------------------------------------------------------

func TestListEmployees_ShapeAndEnvelope(t *testing.T) {
	h := newEmployeeHarness(t)
	h.seedEmployee(3)

	rr := h.do("GET", "/employees", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Envelope keys must be present.
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

	// Required Employee keys per E2 OpenAPI spec.
	requiredKeys := []string{
		"id", "user_id", "full_name", "nik", "nip", "join_at",
		"gender", "birth_date", "birth_place", "phone", "email_personal",
		"address", "npwp", "bpjs_kesehatan", "bpjs_ketenagakerjaan",
		"bank_account", "status", "has_login",
		"current_position", "current_service_line", "current_client_company",
		"created_at", "updated_at", "created_by",
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

	// has_login must be bool.
	if _, ok := first["has_login"].(bool); !ok {
		t.Errorf("data[0].has_login is not bool: %T", first["has_login"])
	}

	// bank_account must be an object with all three sub-keys.
	ba, ok := first["bank_account"].(map[string]any)
	if !ok {
		t.Fatalf("bank_account is not an object: %T", first["bank_account"])
	}
	for _, bk := range []string{"bank_name", "account_number", "account_holder_name"} {
		if _, ok := ba[bk]; !ok {
			t.Errorf("bank_account missing key: %s", bk)
		}
	}
}

func TestListEmployees_Cursor_Pagination(t *testing.T) {
	h := newEmployeeHarness(t)
	// Seed exactly one employee beyond the default page size of 20.
	for i := 0; i < 21; i++ {
		nik := "NIK-CURSOR-" + itoa(i)
		id := "SWP-EMP-CURSOR-" + itoa(i)
		h.repo.addEmployee(domain.Employee{
			ID:          id,
			FullName:    "Cursor Emp " + itoa(i),
			NIK:         nik,
			NIP:         "C" + itoa(i),
			JoinAt:      time.Now().UTC(),
			Status:      "active",
			BankAccount: domain.BankAccount{},
			CreatedAt:   time.Now().UTC().Add(time.Duration(i) * time.Second),
			UpdatedAt:   time.Now().UTC(),
		})
	}

	rr := h.do("GET", "/employees?limit=20", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	if body["has_more"] != true {
		t.Errorf("has_more = %v, want true when 21 records with limit=20", body["has_more"])
	}
	if body["next_cursor"] == nil {
		t.Error("next_cursor should be non-null when has_more=true")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetEmployee
// ---------------------------------------------------------------------------

func TestGetEmployee_200(t *testing.T) {
	h := newEmployeeHarness(t)
	emps := h.seedEmployee(1)
	id := emps[0].ID

	rr := h.do("GET", "/employees/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["id"] != id {
		t.Errorf("id = %v, want %s", body["id"], id)
	}
	// Bank account nesting must be present.
	if _, ok := body["bank_account"]; !ok {
		t.Error("missing bank_account key")
	}
}

func TestGetEmployee_404(t *testing.T) {
	h := newEmployeeHarness(t)

	rr := h.do("GET", "/employees/SWP-EMP-NONEXIST", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("error.code = %v, want NOT_FOUND", errObj["code"])
	}
}

// ---------------------------------------------------------------------------
// Tests: CreateEmployee
// ---------------------------------------------------------------------------

func TestCreateEmployee_201_WithBankAccount(t *testing.T) {
	h := newEmployeeHarness(t)

	rr := h.do("POST", "/employees", map[string]any{
		"full_name": "Budi Santoso",
		"nik":       "3201234567890001",
		"join_at":   "2026-01-15",
		"phone":     "081200000001", // required: login identifier (D2)
		"bank_account": map[string]any{
			"bank_name":           "BCA",
			"account_number":      "987654321",
			"account_holder_name": "Budi Santoso",
		},
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

	// Status must be ACTIVE on creation.
	if body["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", body["status"])
	}

	// bank_account nesting.
	ba, ok := body["bank_account"].(map[string]any)
	if !ok {
		t.Fatalf("bank_account is not an object: %T", body["bank_account"])
	}
	if ba["bank_name"] != "BCA" {
		t.Errorf("bank_account.bank_name = %v, want BCA", ba["bank_name"])
	}
	if ba["account_number"] != "987654321" {
		t.Errorf("bank_account.account_number = %v, want 987654321", ba["account_number"])
	}
	if ba["account_holder_name"] != "Budi Santoso" {
		t.Errorf("bank_account.account_holder_name = %v, want Budi Santoso", ba["account_holder_name"])
	}

	// D1: every employee auto-provisions a login at create — has_login must be true.
	if body["has_login"] != true {
		t.Errorf("has_login = %v, want true", body["has_login"])
	}

	// D1: a login is always provisioned, so temp_password is returned show-once.
	if tp, ok := body["temp_password"].(string); !ok || tp == "" {
		t.Errorf("temp_password = %v, want non-empty string (show-once)", body["temp_password"])
	}
}

func TestCreateEmployee_400_MissingRequiredFields(t *testing.T) {
	h := newEmployeeHarness(t)

	// Missing nik, full_name, join_at.
	rr := h.do("POST", "/employees", map[string]any{})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 INVALID_REQUEST, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "INVALID_REQUEST" {
		t.Errorf("error.code = %v, want INVALID_REQUEST", errObj["code"])
	}
	fields, _ := errObj["fields"].(map[string]any)
	if fields == nil {
		t.Fatal("error.fields is nil, expected field-level errors")
	}
	// full_name, nik, and join_at must each be present.
	for _, f := range []string{"full_name", "nik", "join_at"} {
		if _, ok := fields[f]; !ok {
			t.Errorf("error.fields missing key: %s", f)
		}
	}
}

func TestCreateEmployee_409_DuplicateNIK(t *testing.T) {
	h := newEmployeeHarness(t)
	h.seedEmployee(1)
	// Use the seeded employee's NIK.
	existingNIK := "32012345678901" // matches seedEmployee pattern for i=0

	rr := h.do("POST", "/employees", map[string]any{
		"full_name": "Duplikat Orang",
		"nik":       existingNIK,
		"join_at":   "2026-01-15",
		"phone":     "081200000099", // required; NIK check fires before phone uniqueness
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 DUPLICATE_NIK, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "DUPLICATE_NIK" {
		t.Errorf("error.code = %v, want DUPLICATE_NIK", errObj["code"])
	}
	fields, _ := errObj["fields"].(map[string]any)
	if _, ok := fields["nik"]; !ok {
		t.Error("error.fields.nik missing on DUPLICATE_NIK")
	}
}

func TestCreateEmployee_409_DuplicatePhone(t *testing.T) {
	h := newEmployeeHarness(t)
	// Seed a login that already owns the normalized phone (E.164).
	h.repo.loginPhones["+6281200000001"] = true

	rr := h.do("POST", "/employees", map[string]any{
		"full_name": "Nomor Bentrok",
		"nik":       "3209999999999999",
		"join_at":   "2026-01-15",
		"phone":     "081200000001", // normalizes to +6281200000001 → conflict
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 CONFLICT (phone), got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "CONFLICT" {
		t.Errorf("error.code = %v, want CONFLICT", errObj["code"])
	}
	fields, _ := errObj["fields"].(map[string]any)
	if _, ok := fields["phone"]; !ok {
		t.Error("error.fields.phone missing on phone CONFLICT")
	}
}

func TestCreateEmployee_400_MissingPhone(t *testing.T) {
	h := newEmployeeHarness(t)

	// Phone is now required (D2 login identifier) — omitting it is a 400 on `phone`.
	rr := h.do("POST", "/employees", map[string]any{
		"full_name": "Tanpa Telepon",
		"nik":       "3208888888888888",
		"join_at":   "2026-01-15",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 INVALID_REQUEST, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	fields, _ := errObj["fields"].(map[string]any)
	if _, ok := fields["phone"]; !ok {
		t.Error("error.fields.phone missing when phone omitted")
	}
}

// ---------------------------------------------------------------------------
// Tests: UpdateEmployee
// ---------------------------------------------------------------------------

func TestUpdateEmployee_200(t *testing.T) {
	h := newEmployeeHarness(t)
	emps := h.seedEmployee(1)
	id := emps[0].ID

	rr := h.do("PATCH", "/employees/"+id, map[string]any{
		"full_name": "Updated Name",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["full_name"] != "Updated Name" {
		t.Errorf("full_name = %v, want 'Updated Name'", body["full_name"])
	}
}

func TestUpdateEmployee_404(t *testing.T) {
	h := newEmployeeHarness(t)

	rr := h.do("PATCH", "/employees/SWP-EMP-GHOST", map[string]any{
		"full_name": "Ghost",
	})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tests: DeactivateEmployee / ReactivateEmployee
// ---------------------------------------------------------------------------

func TestDeactivateEmployee_200_Then_409(t *testing.T) {
	h := newEmployeeHarness(t)
	emps := h.seedEmployee(1)
	id := emps[0].ID

	// First deactivate (offboard) → 200 with status INACTIVE. reason is required (OB-1).
	rr := h.do("POST", "/employees/"+id+":deactivate", map[string]any{"reason": "TERMINATED"})
	if rr.Code != http.StatusOK {
		t.Fatalf("deactivate: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "INACTIVE" {
		t.Errorf("after deactivate status = %v, want INACTIVE", body["status"])
	}

	// Second deactivate → 409 already inactive.
	rr2 := h.do("POST", "/employees/"+id+":deactivate", map[string]any{"reason": "TERMINATED"})
	if rr2.Code != http.StatusConflict {
		t.Fatalf("deactivate again: expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}
	body2 := decodeBody(t, rr2)
	errObj, _ := body2["error"].(map[string]any)
	if errObj["code"] != "CONFLICT" {
		t.Errorf("error.code = %v, want CONFLICT", errObj["code"])
	}
}

func TestReactivateEmployee_200_Then_409(t *testing.T) {
	h := newEmployeeHarness(t)

	// Seed an inactive employee.
	now := time.Now().UTC()
	h.repo.addEmployee(domain.Employee{
		ID:          "SWP-EMP-INAC",
		FullName:    "Inactive Emp",
		NIK:         "INACTIVE-NIK-001",
		NIP:         "INE-001",
		JoinAt:      now,
		Status:      "inactive",
		BankAccount: domain.BankAccount{},
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	// Reactivate → 200 with status ACTIVE.
	rr := h.do("POST", "/employees/SWP-EMP-INAC:reactivate", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("reactivate: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "ACTIVE" {
		t.Errorf("after reactivate status = %v, want ACTIVE", body["status"])
	}

	// Reactivate again → 409 already active.
	rr2 := h.do("POST", "/employees/SWP-EMP-INAC:reactivate", nil)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("reactivate again: expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tests: RBAC
// ---------------------------------------------------------------------------

func TestEmployeeRBAC_ShiftLeader_403_OnWrite(t *testing.T) {
	h := newEmployeeHarness(t)

	// shift_leader should be blocked from POST /employees (write group excludes them).
	h.principal = auth.Principal{UserID: "SWP-USR-SL", Role: auth.RoleShiftLeader}

	rr := h.do("POST", "/employees", map[string]any{
		"full_name": "Shift Leader Emp",
		"nik":       "SL-NIK-001",
		"join_at":   "2026-01-15",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("shift_leader POST /employees: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("error.code = %v, want FORBIDDEN", errObj["code"])
	}
}

func TestEmployeeRBAC_ShiftLeader_200_OnRead(t *testing.T) {
	h := newEmployeeHarness(t)
	h.seedEmployee(1)

	// shift_leader CAN read employees.
	h.principal = auth.Principal{UserID: "SWP-USR-SL", Role: auth.RoleShiftLeader, CompanyID: "SWP-CMP-0021"}

	rr := h.do("GET", "/employees", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("shift_leader GET /employees: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ensure errors package is used
var _ = errors.New
