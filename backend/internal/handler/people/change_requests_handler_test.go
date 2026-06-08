// Package people_test — change-request contract tests.
// Asserts E2 change-request endpoint shapes, approve/reject transitions,
// 409 on already-resolved, and RBAC. Part of the drift gate.
package people_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	peoplehandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/people"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	peoplesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
)

// ---------------------------------------------------------------------------
// fakeChangeRequestRepo — in-memory implementation of peoplesvc.ChangeRequestRepository.
// ---------------------------------------------------------------------------

type fakeChangeRequestRepo struct {
	changeRequests map[string]domain.ChangeRequest
	employees      map[string]domain.Employee
}

func newFakeChangeRequestRepo() *fakeChangeRequestRepo {
	return &fakeChangeRequestRepo{
		changeRequests: make(map[string]domain.ChangeRequest),
		employees:      make(map[string]domain.Employee),
	}
}

func (r *fakeChangeRequestRepo) addChangeRequest(cr domain.ChangeRequest) {
	r.changeRequests[cr.ID] = cr
}

func (r *fakeChangeRequestRepo) addEmployee(e domain.Employee) {
	r.employees[e.ID] = e
}

func (r *fakeChangeRequestRepo) ListChangeRequests(_ context.Context, f domain.ChangeRequestFilter) ([]domain.ChangeRequest, error) {
	var all []domain.ChangeRequest
	for _, cr := range r.changeRequests {
		if f.Status != nil && cr.Status != *f.Status {
			continue
		}
		if f.EmployeeID != nil && cr.EmployeeID != *f.EmployeeID {
			continue
		}
		if f.RequestType != nil && cr.RequestType != *f.RequestType {
			continue
		}
		if f.CursorSubmittedAt != nil && f.CursorID != nil {
			if cr.SubmittedAt.Before(*f.CursorSubmittedAt) {
				continue
			}
			if cr.SubmittedAt.Equal(*f.CursorSubmittedAt) && cr.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, cr)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].SubmittedAt.Equal(all[j].SubmittedAt) {
			return all[i].ID < all[j].ID
		}
		return all[i].SubmittedAt.Before(all[j].SubmittedAt)
	})
	if f.Limit > 0 && len(all) > f.Limit {
		return all[:f.Limit], nil
	}
	return all, nil
}

func (r *fakeChangeRequestRepo) GetChangeRequestByID(_ context.Context, id string) (domain.ChangeRequest, error) {
	cr, ok := r.changeRequests[id]
	if !ok {
		return domain.ChangeRequest{}, domain.ErrNotFound
	}
	return cr, nil
}

func (r *fakeChangeRequestRepo) GetEmployeeByID(_ context.Context, id string) (domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	return e, nil
}

func (r *fakeChangeRequestRepo) UpdateEmployee(_ context.Context, _ pgx.Tx, p peoplesvc.UpdateEmployeeParams) (domain.Employee, error) {
	e, ok := r.employees[p.ID]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	// Apply whitelisted fields (phone, address, bank_account).
	if p.Phone != "" {
		e.Phone = &p.Phone
	} else {
		e.Phone = nil
	}
	if p.Address != "" {
		e.Address = &p.Address
	} else {
		e.Address = nil
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

func (r *fakeChangeRequestRepo) ResolveChangeRequest(_ context.Context, _ pgx.Tx, p peoplesvc.ResolveChangeRequestParams) (domain.ChangeRequest, error) {
	cr, ok := r.changeRequests[p.ID]
	if !ok {
		return domain.ChangeRequest{}, domain.ErrNotFound
	}
	cr.Status = p.Status
	cr.ResolvedAt = p.ResolvedAt
	cr.ResolvedBy = p.ResolvedBy
	cr.RejectionReason = p.RejectionReason
	r.changeRequests[p.ID] = cr
	return cr, nil
}

func (r *fakeChangeRequestRepo) CreateChangeRequest(_ context.Context, _ pgx.Tx, p peoplesvc.CreateChangeRequestParams) (domain.ChangeRequest, error) {
	cr := domain.ChangeRequest{
		ID:          "SWP-CHG-" + p.EmployeeID,
		EmployeeID:  p.EmployeeID,
		Status:      "pending",
		RequestType: p.RequestType,
		Changes:     p.Changes,
		Note:        p.Note,
		SubmittedAt: time.Now().UTC(),
	}
	r.changeRequests[cr.ID] = cr
	return cr, nil
}

// Compile-time interface check.
var _ peoplesvc.ChangeRequestRepository = (*fakeChangeRequestRepo)(nil)

// ---------------------------------------------------------------------------
// Change-request test harness
// ---------------------------------------------------------------------------

type changeRequestHarness struct {
	router    *chi.Mux
	repo      *fakeChangeRequestRepo
	principal auth.Principal
}

func newChangeRequestHarness(t *testing.T) *changeRequestHarness {
	t.Helper()
	repo := newFakeChangeRequestRepo()
	svc := peoplesvc.NewChangeRequestService(repo, &fakeTxRunner{})
	handler := peoplehandler.NewChangeRequestHandler(svc)

	fh := &changeRequestHarness{
		repo:      repo,
		principal: auth.Principal{UserID: "SWP-USR-HR-001", Role: auth.RoleHRAdmin},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), fh.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	// RBAC group matching server.go change-requests slice: super_admin + hr_admin only.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Get("/change-requests", handler.ListPendingChangeRequests)
		r.Get("/change-requests/{change_request_id}", handler.GetChangeRequest)
		r.Post("/change-requests/{change_request_id}:approve", handler.ApproveChangeRequest)
		r.Post("/change-requests/{change_request_id}:reject", handler.RejectChangeRequest)
	})

	// Agent-inclusive create group matching server.go (agent/hr/super).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleHRAdmin, auth.RoleSuperAdmin))
		r.Post("/employees/{employee_id}/change-requests", handler.CreateChangeRequest)
	})

	fh.router = r
	return fh
}

func (h *changeRequestHarness) doJSON(method, path string, body any) *httptest.ResponseRecorder {
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
// Seed helpers
// ---------------------------------------------------------------------------

func seedCREmployee(h *changeRequestHarness, id, phone, address string) domain.Employee {
	now := time.Now().UTC()
	ph := phone
	addr := address
	e := domain.Employee{
		ID:          id,
		FullName:    "CR Employee " + id,
		NIK:         "NIK-CR-" + id,
		NIP:         "NIP-CR-" + id,
		JoinAt:      now,
		Status:      "active",
		Phone:       &ph,
		Address:     &addr,
		BankAccount: domain.BankAccount{BankName: "BNI", AccountNumber: "111222333", AccountHolderName: "CR Employee " + id},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	h.repo.addEmployee(e)
	return e
}

func seedPendingCR(h *changeRequestHarness, id, empID string, changes domain.ChangeRequestChanges, reqType string) domain.ChangeRequest {
	now := time.Now().UTC()
	cr := domain.ChangeRequest{
		ID:          id,
		EmployeeID:  empID,
		Status:      "pending",
		RequestType: reqType,
		Changes:     changes,
		SubmittedAt: now,
	}
	h.repo.addChangeRequest(cr)
	return cr
}

// ---------------------------------------------------------------------------
// Tests: ListPendingChangeRequests
// ---------------------------------------------------------------------------

func TestListPendingChangeRequests_ShapeAndEnvelope(t *testing.T) {
	h := newChangeRequestHarness(t)
	emp := seedCREmployee(h, "SWP-EMP-CR-LIST", "+628111", "Jl. Test 1")

	newPhone := "+628999888777"
	seedPendingCR(h, "SWP-CR-LIST-001", emp.ID, domain.ChangeRequestChanges{
		Phone: &newPhone,
	}, "PHONE")

	rr := h.doJSON("GET", "/change-requests", nil)
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

	// Required ChangeRequest keys per E2 OpenAPI spec.
	requiredKeys := []string{
		"id", "employee_id", "status", "request_type",
		"note", "changes", "submitted_at",
		"resolved_at", "resolved_by", "rejection_reason",
	}
	for _, k := range requiredKeys {
		if _, ok := first[k]; !ok {
			t.Errorf("data[0] missing key: %s", k)
		}
	}

	// status must be UPPERCASE.
	if first["status"] != "PENDING" {
		t.Errorf("data[0].status = %v, want PENDING", first["status"])
	}

	// request_type must be set.
	if first["request_type"] != "PHONE" {
		t.Errorf("data[0].request_type = %v, want PHONE", first["request_type"])
	}

	// changes must be an object.
	changes, ok := first["changes"].(map[string]any)
	if !ok {
		t.Fatalf("changes is not an object: %T", first["changes"])
	}
	if _, ok := changes["phone"]; !ok {
		t.Error("changes missing phone key")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetChangeRequest (detail with diff)
// ---------------------------------------------------------------------------

func TestGetChangeRequest_200_WithDiff(t *testing.T) {
	h := newChangeRequestHarness(t)
	emp := seedCREmployee(h, "SWP-EMP-CR-DETAIL", "+628111111", "Jl. Lama 1")

	newPhone := "+628999999"
	cr := seedPendingCR(h, "SWP-CR-DETAIL-001", emp.ID, domain.ChangeRequestChanges{
		Phone: &newPhone,
	}, "PHONE")

	rr := h.doJSON("GET", "/change-requests/"+cr.ID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Detail must include employee{id, full_name, nip}.
	employee, ok := body["employee"].(map[string]any)
	if !ok {
		t.Fatalf("employee is not an object: %T", body["employee"])
	}
	for _, k := range []string{"id", "full_name", "nip"} {
		if _, ok := employee[k]; !ok {
			t.Errorf("employee missing key: %s", k)
		}
	}
	if employee["id"] != emp.ID {
		t.Errorf("employee.id = %v, want %s", employee["id"], emp.ID)
	}

	// diff must be an object with phone field having {old, new}.
	diff, ok := body["diff"].(map[string]any)
	if !ok {
		t.Fatalf("diff is not an object: %T", body["diff"])
	}
	phoneDiff, ok := diff["phone"].(map[string]any)
	if !ok {
		t.Fatalf("diff.phone is not an object: %T", diff["phone"])
	}
	for _, k := range []string{"old", "new"} {
		if _, ok := phoneDiff[k]; !ok {
			t.Errorf("diff.phone missing key: %s", k)
		}
	}
	// new must be the requested phone.
	if phoneDiff["new"] != newPhone {
		t.Errorf("diff.phone.new = %v, want %s", phoneDiff["new"], newPhone)
	}
}

func TestGetChangeRequest_404(t *testing.T) {
	h := newChangeRequestHarness(t)

	rr := h.doJSON("GET", "/change-requests/SWP-CR-GHOST", nil)
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
// Tests: ApproveChangeRequest
// ---------------------------------------------------------------------------

func TestApproveChangeRequest_200_AppliesToEmployee(t *testing.T) {
	h := newChangeRequestHarness(t)
	emp := seedCREmployee(h, "SWP-EMP-APPROVE", "+628111", "Jl. Old 1")

	newPhone := "+628222222"
	cr := seedPendingCR(h, "SWP-CR-APPROVE-001", emp.ID, domain.ChangeRequestChanges{
		Phone: &newPhone,
	}, "PHONE")

	rr := h.doJSON("POST", "/change-requests/"+cr.ID+":approve", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Response status must be APPROVED.
	if body["status"] != "APPROVED" {
		t.Errorf("status = %v, want APPROVED", body["status"])
	}
	// resolved_by must be the actor (principal UserID).
	if body["resolved_by"] != "SWP-USR-HR-001" {
		t.Errorf("resolved_by = %v, want SWP-USR-HR-001", body["resolved_by"])
	}
	// resolved_at must be non-null.
	if body["resolved_at"] == nil {
		t.Error("resolved_at should be non-null after approval")
	}

	// The employee's phone must have been updated to the requested value.
	updatedEmp := h.repo.employees[emp.ID]
	if updatedEmp.Phone == nil || *updatedEmp.Phone != newPhone {
		t.Errorf("employee phone after approve = %v, want %s", updatedEmp.Phone, newPhone)
	}
}

func TestApproveChangeRequest_AlreadyResolved_409(t *testing.T) {
	h := newChangeRequestHarness(t)
	emp := seedCREmployee(h, "SWP-EMP-APPROVE-DUP", "+628333", "Jl. Already 1")

	// Seed an already-approved CR.
	resolvedAt := time.Now().UTC()
	actor := "SWP-USR-HR-001"
	resolved := domain.ChangeRequest{
		ID:          "SWP-CR-APPROVED-001",
		EmployeeID:  emp.ID,
		Status:      "approved",
		RequestType: "PHONE",
		ResolvedAt:  &resolvedAt,
		ResolvedBy:  &actor,
		SubmittedAt: time.Now().UTC().Add(-time.Hour),
	}
	h.repo.addChangeRequest(resolved)

	rr := h.doJSON("POST", "/change-requests/"+resolved.ID+":approve", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 CONFLICT on re-approve, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "CONFLICT" {
		t.Errorf("error.code = %v, want CONFLICT", errObj["code"])
	}
}

// ---------------------------------------------------------------------------
// Tests: RejectChangeRequest
// ---------------------------------------------------------------------------

func TestRejectChangeRequest_MissingReason_400(t *testing.T) {
	h := newChangeRequestHarness(t)
	emp := seedCREmployee(h, "SWP-EMP-REJECT-NOREASON", "+628444", "Jl. No Reason 1")
	cr := seedPendingCR(h, "SWP-CR-REJECT-NOREASON", emp.ID, domain.ChangeRequestChanges{}, "PHONE")

	// Missing reason → 400.
	rr := h.doJSON("POST", "/change-requests/"+cr.ID+":reject", map[string]any{
		"reason": "",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 INVALID_REQUEST on empty reason, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "INVALID_REQUEST" {
		t.Errorf("error.code = %v, want INVALID_REQUEST", errObj["code"])
	}
	// reason field error.
	fields, _ := errObj["fields"].(map[string]any)
	if _, ok := fields["reason"]; !ok {
		t.Error("error.fields.reason missing on empty-reason 400")
	}
}

func TestRejectChangeRequest_ShortReason_400(t *testing.T) {
	h := newChangeRequestHarness(t)
	emp := seedCREmployee(h, "SWP-EMP-REJECT-SHORT", "+628555", "Jl. Short 1")
	cr := seedPendingCR(h, "SWP-CR-REJECT-SHORT", emp.ID, domain.ChangeRequestChanges{}, "PHONE")

	// 2-char reason → below minLength 3 → 400.
	rr := h.doJSON("POST", "/change-requests/"+cr.ID+":reject", map[string]any{
		"reason": "ab",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on short reason (len<3), got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRejectChangeRequest_200_StatusRejectedAndReason(t *testing.T) {
	h := newChangeRequestHarness(t)
	emp := seedCREmployee(h, "SWP-EMP-REJECT-OK", "+628666", "Jl. Reject 1")

	newPhone := "+628777"
	cr := seedPendingCR(h, "SWP-CR-REJECT-OK", emp.ID, domain.ChangeRequestChanges{
		Phone: &newPhone,
	}, "PHONE")

	rr := h.doJSON("POST", "/change-requests/"+cr.ID+":reject", map[string]any{
		"reason": "Data tidak sesuai dengan dokumen pendukung.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Status must be REJECTED.
	if body["status"] != "REJECTED" {
		t.Errorf("status = %v, want REJECTED", body["status"])
	}
	// rejection_reason must be set.
	if body["rejection_reason"] != "Data tidak sesuai dengan dokumen pendukung." {
		t.Errorf("rejection_reason = %v, want the submitted reason", body["rejection_reason"])
	}

	// Employee phone must NOT have changed (reject doesn't apply).
	updatedEmp := h.repo.employees[emp.ID]
	if updatedEmp.Phone == nil || *updatedEmp.Phone != "+628666" {
		t.Errorf("employee phone after reject = %v, want unchanged +628666", updatedEmp.Phone)
	}
}

func TestRejectChangeRequest_AlreadyResolved_409(t *testing.T) {
	h := newChangeRequestHarness(t)
	emp := seedCREmployee(h, "SWP-EMP-REJECT-DUP", "+628888", "Jl. Reject Dup 1")

	// Seed an already-rejected CR.
	resolvedAt := time.Now().UTC()
	actor := "SWP-USR-HR-001"
	reason := "Already rejected."
	resolved := domain.ChangeRequest{
		ID:              "SWP-CR-REJECTED-001",
		EmployeeID:      emp.ID,
		Status:          "rejected",
		RequestType:     "PHONE",
		ResolvedAt:      &resolvedAt,
		ResolvedBy:      &actor,
		RejectionReason: &reason,
		SubmittedAt:     time.Now().UTC().Add(-time.Hour),
	}
	h.repo.addChangeRequest(resolved)

	rr := h.doJSON("POST", "/change-requests/"+resolved.ID+":reject", map[string]any{
		"reason": "Trying to re-reject this request.",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 CONFLICT on re-reject, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "CONFLICT" {
		t.Errorf("error.code = %v, want CONFLICT", errObj["code"])
	}
}

// ---------------------------------------------------------------------------
// Tests: RBAC — shift_leader and agent cannot access change-requests
// ---------------------------------------------------------------------------

func TestChangeRequestRBAC_ShiftLeader_403(t *testing.T) {
	h := newChangeRequestHarness(t)

	// shift_leader is not in the allowed group for change-requests.
	h.principal = auth.Principal{UserID: "SWP-USR-SL", Role: auth.RoleShiftLeader, CompanyID: "SWP-CMP-0021"}

	rr := h.doJSON("GET", "/change-requests", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("shift_leader GET /change-requests: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("error.code = %v, want FORBIDDEN", errObj["code"])
	}
}

func TestChangeRequestRBAC_Agent_403(t *testing.T) {
	h := newChangeRequestHarness(t)

	h.principal = auth.Principal{UserID: "SWP-USR-AGENT", Role: auth.RoleAgent}

	rr := h.doJSON("GET", "/change-requests", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("agent GET /change-requests: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tests: CreateChangeRequest (agent self-service, EP-5)
// ---------------------------------------------------------------------------

func TestCreateChangeRequest_AgentOwn_201(t *testing.T) {
	h := newChangeRequestHarness(t)
	seedCREmployee(h, "SWP-EMP-CR-OWN", "+628111", "Jl. Lama 1")
	h.principal = auth.Principal{UserID: "SWP-USR-A", EmployeeID: "SWP-EMP-CR-OWN", Role: auth.RoleAgent}

	rr := h.doJSON("POST", "/employees/SWP-EMP-CR-OWN/change-requests", map[string]any{
		"changes": map[string]any{
			"phone": "+628999888777",
			"bank_account": map[string]any{
				"bank_name":           "BCA",
				"account_number":      "9999000011",
				"account_holder_name": "CR Employee SWP-EMP-CR-OWN",
			},
		},
		"note": "Ganti nomor & rekening",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "PENDING" {
		t.Errorf("status = %v, want PENDING", body["status"])
	}
	if body["request_type"] != "MULTIPLE" {
		t.Errorf("request_type = %v, want MULTIPLE (phone+bank)", body["request_type"])
	}
	if body["employee_id"] != "SWP-EMP-CR-OWN" {
		t.Errorf("employee_id = %v, want SWP-EMP-CR-OWN", body["employee_id"])
	}
	if loc := rr.Header().Get("Location"); loc == "" {
		t.Errorf("missing Location header")
	}
}

func TestCreateChangeRequest_AgentAnother_404(t *testing.T) {
	h := newChangeRequestHarness(t)
	seedCREmployee(h, "SWP-EMP-CR-OTHER", "+628111", "Jl. Lama 2")
	// Agent caller is a different employee.
	h.principal = auth.Principal{UserID: "SWP-USR-B", EmployeeID: "SWP-EMP-CR-SELF", Role: auth.RoleAgent}

	newPhone := "+628000111222"
	rr := h.doJSON("POST", "/employees/SWP-EMP-CR-OTHER/change-requests", map[string]any{
		"changes": map[string]any{"phone": newPhone},
	})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("agent filing for another employee: expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateChangeRequest_SinglePhone_DerivesType(t *testing.T) {
	h := newChangeRequestHarness(t)
	seedCREmployee(h, "SWP-EMP-CR-PH", "+628111", "Jl. Lama 3")
	h.principal = auth.Principal{UserID: "SWP-USR-C", EmployeeID: "SWP-EMP-CR-PH", Role: auth.RoleAgent}

	rr := h.doJSON("POST", "/employees/SWP-EMP-CR-PH/change-requests", map[string]any{
		"changes": map[string]any{"phone": "+628777666555"},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["request_type"] != "PHONE" {
		t.Errorf("request_type = %v, want PHONE", body["request_type"])
	}
}

func TestCreateChangeRequest_EmptyChanges_400(t *testing.T) {
	h := newChangeRequestHarness(t)
	seedCREmployee(h, "SWP-EMP-CR-EMPTY", "+628111", "Jl. Lama 4")
	h.principal = auth.Principal{UserID: "SWP-USR-D", EmployeeID: "SWP-EMP-CR-EMPTY", Role: auth.RoleAgent}

	rr := h.doJSON("POST", "/employees/SWP-EMP-CR-EMPTY/change-requests", map[string]any{
		"changes": map[string]any{},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty changes: expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
