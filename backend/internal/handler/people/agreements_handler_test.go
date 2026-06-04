// Package people_test — agreement contract tests.
// Asserts E2 agreement endpoint shapes, status codes, PKWT 422, multipart FileRef, and 413.
// Part of the drift gate replacing server-side codegen.
package people_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
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
// fakeAgreementRepo — in-memory implementation of peoplesvc.AgreementRepository.
// ---------------------------------------------------------------------------

type fakeAgreementRepo struct {
	agreements  map[string]domain.Agreement
	employees   map[string]domain.Employee
	attachments map[string]domain.Attachment
}

func newFakeAgreementRepo() *fakeAgreementRepo {
	return &fakeAgreementRepo{
		agreements:  make(map[string]domain.Agreement),
		employees:   make(map[string]domain.Employee),
		attachments: make(map[string]domain.Attachment),
	}
}

func (r *fakeAgreementRepo) addAgreement(ag domain.Agreement) {
	r.agreements[ag.ID] = ag
}

func (r *fakeAgreementRepo) addEmployee(e domain.Employee) {
	r.employees[e.ID] = e
}

func (r *fakeAgreementRepo) ListAgreements(_ context.Context, f domain.AgreementFilter) ([]domain.Agreement, error) {
	var all []domain.Agreement
	for _, ag := range r.agreements {
		if f.Status != nil && ag.Status != *f.Status {
			continue
		}
		if f.EmployeeID != nil && ag.EmployeeID != *f.EmployeeID {
			continue
		}
		if f.Type != nil && ag.Type != *f.Type {
			continue
		}
		if f.EndDateLTE != nil && ag.EndDate != nil {
			if ag.EndDate.After(*f.EndDateLTE) {
				continue
			}
		}
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if ag.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if ag.CreatedAt.Equal(*f.CursorCreatedAt) && ag.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, ag)
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

func (r *fakeAgreementRepo) GetAgreementByID(_ context.Context, id string) (domain.Agreement, error) {
	ag, ok := r.agreements[id]
	if !ok {
		return domain.Agreement{}, domain.ErrNotFound
	}
	return ag, nil
}

func (r *fakeAgreementRepo) GetActiveAgreementForEmployee(_ context.Context, employeeID string) (domain.Agreement, error) {
	for _, ag := range r.agreements {
		if ag.EmployeeID == employeeID && ag.Status == "active" {
			return ag, nil
		}
	}
	return domain.Agreement{}, domain.ErrNotFound
}

func (r *fakeAgreementRepo) GetEmployeeByID(_ context.Context, id string) (domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	return e, nil
}

var agCounter int

func (r *fakeAgreementRepo) CreateAgreement(_ context.Context, _ pgx.Tx, p peoplesvc.CreateAgreementParams) (domain.Agreement, error) {
	agCounter++
	now := time.Now().UTC()
	id := "SWP-AG-" + itoa(agCounter)
	ag := domain.Agreement{
		ID:            id,
		EmployeeID:    p.EmployeeID,
		Type:          p.Type,
		AgreementNo:   p.AgreementNo,
		StartDate:     p.StartDate,
		EndDate:       p.EndDate,
		Status:        "active",
		PredecessorID: p.PredecessorID,
		Compensation: domain.CompensationTerms{
			BaseSalaryIDR: p.BaseSalaryIDR,
			BpjsTerms:     p.BpjsTerms,
			TaxProfile:    p.TaxProfile,
			EffectiveDate: p.CompEffectiveDate,
		},
		CreatedBy: p.CreatedBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.agreements[id] = ag
	return ag, nil
}

func (r *fakeAgreementRepo) SetAgreementStatus(_ context.Context, _ pgx.Tx, p peoplesvc.SetAgreementStatusParams) (domain.Agreement, error) {
	ag, ok := r.agreements[p.ID]
	if !ok {
		return domain.Agreement{}, domain.ErrNotFound
	}
	ag.Status = p.Status
	if p.ClosedReason != nil {
		ag.ClosedReason = p.ClosedReason
	}
	if p.ClosedAt != nil {
		ag.ClosedAt = p.ClosedAt
	}
	if p.SuccessorID != nil {
		ag.SuccessorID = p.SuccessorID
	}
	ag.UpdatedAt = time.Now().UTC()
	r.agreements[p.ID] = ag
	return ag, nil
}

var attCounter int

func (r *fakeAgreementRepo) CreateAttachment(_ context.Context, _ pgx.Tx, p peoplesvc.CreateAttachmentParams) (domain.Attachment, error) {
	attCounter++
	now := time.Now().UTC()
	id := "SWP-FILE-" + itoa(attCounter)
	att := domain.Attachment{
		ID:          id,
		AgreementID: p.AgreementID,
		Category:    p.Category,
		Caption:     p.Caption,
		FileName:    p.FileName,
		MIME:        p.MIME,
		SizeBytes:   p.SizeBytes,
		Blob:        p.Blob,
		UploadedBy:  p.UploadedBy,
		CreatedAt:   now,
	}
	r.attachments[id] = att
	return att, nil
}

func (r *fakeAgreementRepo) GetAttachmentByID(_ context.Context, id string) (domain.Attachment, error) {
	att, ok := r.attachments[id]
	if !ok {
		return domain.Attachment{}, domain.ErrNotFound
	}
	return att, nil
}

// Compile-time interface check.
var _ peoplesvc.AgreementRepository = (*fakeAgreementRepo)(nil)

// ---------------------------------------------------------------------------
// Agreement test harness
// ---------------------------------------------------------------------------

type agreementHarness struct {
	router    *chi.Mux
	repo      *fakeAgreementRepo
	svc       *peoplesvc.AgreementService
	principal auth.Principal
}

func newAgreementHarness(t *testing.T) *agreementHarness {
	t.Helper()
	repo := newFakeAgreementRepo()
	svc := peoplesvc.NewAgreementService(repo, &fakeTxRunner{})
	handler := peoplehandler.NewAgreementHandler(svc)

	fh := &agreementHarness{
		repo:      repo,
		svc:       svc,
		principal: auth.Principal{UserID: "SWP-USR-0001", Role: auth.RoleHRAdmin},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), fh.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	// RBAC groups matching server.go agreements slice.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Get("/agreements", handler.ListAgreements)
		r.Get("/agreements/{agreement_id}", handler.GetAgreement)
	})
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Post("/agreements", handler.CreateAgreement)
		r.Post("/agreements/{agreement_id}:renew", handler.RenewAgreement)
		r.Post("/agreements/{agreement_id}:close", handler.CloseAgreement)
		r.Post("/agreements/{agreement_id}/attachments", handler.UploadAttachment)
	})
	// Download route: all authenticated roles.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
		r.Get("/files/{file_id}", handler.DownloadFile)
	})

	fh.router = r
	return fh
}

func (h *agreementHarness) doJSON(method, path string, body any) *httptest.ResponseRecorder {
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

func (h *agreementHarness) doMultipart(path string, fieldName, fileName, contentType string, fileContent []byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Create a part with the explicit Content-Type header.
	partHeader := make(map[string][]string)
	partHeader["Content-Disposition"] = []string{`form-data; name="` + fieldName + `"; filename="` + fileName + `"`}
	partHeader["Content-Type"] = []string{contentType}
	fw, err := mw.CreatePart(partHeader)
	if err != nil {
		panic("doMultipart: create part: " + err.Error())
	}
	_, _ = fw.Write(fileContent)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// Seed helpers
// ---------------------------------------------------------------------------

func seedAgreementEmployee(h *agreementHarness, id string) domain.Employee {
	now := time.Now().UTC()
	e := domain.Employee{
		ID:          id,
		FullName:    "Test Employee " + id,
		NIK:         "TEST-NIK-" + id,
		NIP:         "TEST-NIP-" + id,
		JoinAt:      now,
		Status:      "active",
		BankAccount: domain.BankAccount{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	h.repo.addEmployee(e)
	return e
}

func seedPKWTAgreement(h *agreementHarness, empID, agID string, status string, endDate time.Time) domain.Agreement {
	now := time.Now().UTC()
	ag := domain.Agreement{
		ID:          agID,
		EmployeeID:  empID,
		Type:        "PKWT",
		AgreementNo: "PKT/2026/" + agID,
		StartDate:   now.AddDate(0, -6, 0),
		EndDate:     &endDate,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	h.repo.addAgreement(ag)
	return ag
}

// ---------------------------------------------------------------------------
// Tests: ListAgreements
// ---------------------------------------------------------------------------

func TestListAgreements_ShapeAndEnvelope(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-AG-001")
	endDate := time.Now().UTC().AddDate(0, 6, 0)
	seedPKWTAgreement(h, emp.ID, "SWP-AG-TEST-001", "active", endDate)

	rr := h.doJSON("GET", "/agreements", nil)
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
	requiredKeys := []string{
		"id", "employee_id", "type", "agreement_no",
		"start_date", "end_date", "status",
		"predecessor_id", "successor_id", "closed_reason", "closed_at",
		"compensation", "created_by", "created_at", "updated_at",
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

	// type must be UPPERCASE.
	if first["type"] != "PKWT" {
		t.Errorf("data[0].type = %v, want PKWT", first["type"])
	}

	// compensation must be an object with bpjs_terms having 4 pcts.
	comp, ok := first["compensation"].(map[string]any)
	if !ok {
		t.Fatalf("compensation is not an object: %T", first["compensation"])
	}
	bpjs, ok := comp["bpjs_terms"].(map[string]any)
	if !ok {
		t.Fatalf("compensation.bpjs_terms is not an object: %T", comp["bpjs_terms"])
	}
	for _, pctKey := range []string{
		"kesehatan_employer_pct",
		"kesehatan_employee_pct",
		"ketenagakerjaan_employer_pct",
		"ketenagakerjaan_employee_pct",
	} {
		if _, ok := bpjs[pctKey]; !ok {
			t.Errorf("bpjs_terms missing key: %s", pctKey)
		}
	}
}

func TestListAgreements_EXPIRING_VirtualStatus(t *testing.T) {
	h := newAgreementHarness(t)
	// Fix clock to a deterministic date.
	fixedNow := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	h.svc.SetClock(func() time.Time { return fixedNow })

	emp := seedAgreementEmployee(h, "SWP-EMP-EXPIRING-001")

	// End date 20 days from fixedNow (within 30-day EXPIRING window).
	endDate := fixedNow.Add(20 * 24 * time.Hour)
	ag := domain.Agreement{
		ID:          "SWP-AG-EXPIRING-001",
		EmployeeID:  emp.ID,
		Type:        "PKWT",
		AgreementNo: "PKT/2026/EXPIRING",
		StartDate:   fixedNow.AddDate(0, -11, 0),
		EndDate:     &endDate,
		Status:      "active", // stored as active
		CreatedAt:   fixedNow.Add(-time.Hour),
		UpdatedAt:   fixedNow.Add(-time.Hour),
	}
	h.repo.addAgreement(ag)

	rr := h.doJSON("GET", "/agreements", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data := body["data"].([]any)
	if len(data) == 0 {
		t.Fatal("expected at least one agreement in data")
	}

	first := data[0].(map[string]any)
	// toAgreementResponse should emit EXPIRING when end_date < now+30d.
	if first["status"] != "EXPIRING" {
		t.Errorf("status = %v, want EXPIRING (end_date within 30d of now)", first["status"])
	}
}

// ---------------------------------------------------------------------------
// Tests: GetAgreement
// ---------------------------------------------------------------------------

func TestGetAgreement_200(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-GET-AG")
	endDate := time.Now().UTC().AddDate(1, 0, 0)
	ag := seedPKWTAgreement(h, emp.ID, "SWP-AG-GET-001", "active", endDate)

	rr := h.doJSON("GET", "/agreements/"+ag.ID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["id"] != ag.ID {
		t.Errorf("id = %v, want %s", body["id"], ag.ID)
	}
}

func TestGetAgreement_404(t *testing.T) {
	h := newAgreementHarness(t)

	rr := h.doJSON("GET", "/agreements/SWP-AG-GHOST", nil)
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
// Tests: CreateAgreement
// ---------------------------------------------------------------------------

func TestCreateAgreement_PKWT_201_WithCompensation(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-CREATE-PKWT")

	rr := h.doJSON("POST", "/agreements", map[string]any{
		"employee_id":  emp.ID,
		"type":         "PKWT",
		"agreement_no": "PKT/2026/001",
		"start_date":   "2026-01-01",
		"end_date":     "2027-06-30",
		"compensation": map[string]any{
			"base_salary_idr": 5000000.0,
			"bpjs_terms": map[string]any{
				"kesehatan_employee_pct":        1.0,
				"kesehatan_employer_pct":        4.0,
				"ketenagakerjaan_employee_pct":  3.0,
				"ketenagakerjaan_employer_pct":  5.7,
			},
			"tax_profile":    "PTKP_TK0",
			"effective_date": "2026-01-01",
		},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Status must be ACTIVE.
	if body["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", body["status"])
	}
	// end_date must be non-null for PKWT.
	if body["end_date"] == nil {
		t.Error("end_date should be non-null for PKWT")
	}

	// compensation must have all 4 bpjs pcts.
	comp, _ := body["compensation"].(map[string]any)
	bpjs, _ := comp["bpjs_terms"].(map[string]any)
	for _, pct := range []string{
		"kesehatan_employee_pct",
		"kesehatan_employer_pct",
		"ketenagakerjaan_employee_pct",
		"ketenagakerjaan_employer_pct",
	} {
		if _, ok := bpjs[pct]; !ok {
			t.Errorf("bpjs_terms missing key: %s", pct)
		}
	}

	// Location header.
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, "/api/v1/agreements/") {
		t.Errorf("Location = %q, want prefix /api/v1/agreements/", loc)
	}
}

func TestCreateAgreement_PKWTT_201_NullEndDate(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-CREATE-PKWTT")

	rr := h.doJSON("POST", "/agreements", map[string]any{
		"employee_id":  emp.ID,
		"type":         "PKWTT",
		"agreement_no": "PKT2/2026/001",
		"start_date":   "2026-01-01",
		// end_date intentionally omitted → null for PKWTT
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// end_date must be null for PKWTT.
	if body["end_date"] != nil {
		t.Errorf("end_date = %v, want null for PKWTT", body["end_date"])
	}
}

func TestCreateAgreement_PKWT_MissingEndDate_400(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-PKWT-NO-END")

	rr := h.doJSON("POST", "/agreements", map[string]any{
		"employee_id":  emp.ID,
		"type":         "PKWT",
		"agreement_no": "PKT/2026/NOEND",
		"start_date":   "2026-01-01",
		// end_date missing → 400 INVALID_REQUEST with fields.end_date
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 INVALID_REQUEST, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "INVALID_REQUEST" {
		t.Errorf("error.code = %v, want INVALID_REQUEST", errObj["code"])
	}
	fields, _ := errObj["fields"].(map[string]any)
	if _, ok := fields["end_date"]; !ok {
		t.Error("error.fields.end_date missing on PKWT-no-end-date 400")
	}
}

func TestCreateAgreement_PKWT_PeriodExceedsMax_422(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-PKWT-6YR")

	// 6-year period > 5-year limit → 422 PKWT_PERIOD_EXCEEDS_MAX.
	rr := h.doJSON("POST", "/agreements", map[string]any{
		"employee_id":  emp.ID,
		"type":         "PKWT",
		"agreement_no": "PKT/2026/6YR",
		"start_date":   "2026-01-01",
		"end_date":     "2032-01-02", // 6 years and 1 day from start
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 PKWT_PERIOD_EXCEEDS_MAX, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "PKWT_PERIOD_EXCEEDS_MAX" {
		t.Errorf("error.code = %v, want PKWT_PERIOD_EXCEEDS_MAX", errObj["code"])
	}
	fields, _ := errObj["fields"].(map[string]any)
	if _, ok := fields["end_date"]; !ok {
		t.Error("error.fields.end_date missing on PKWT_PERIOD_EXCEEDS_MAX 422")
	}
}

func TestCreateAgreement_ActiveAgreementExists_409(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-ACTIVE-AG")

	// Seed an existing active agreement for the employee.
	existingEndDate := time.Now().UTC().AddDate(1, 0, 0)
	existing := domain.Agreement{
		ID:         "SWP-AG-EXISTING-001",
		EmployeeID: emp.ID,
		Type:       "PKWT",
		Status:     "active",
		StartDate:  time.Now().UTC(),
		EndDate:    &existingEndDate,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	h.repo.addAgreement(existing)

	// Trying to create another active agreement → 409 ACTIVE_AGREEMENT_EXISTS.
	rr := h.doJSON("POST", "/agreements", map[string]any{
		"employee_id":  emp.ID,
		"type":         "PKWT",
		"agreement_no": "PKT/2026/DUP",
		"start_date":   "2026-06-01",
		"end_date":     "2027-06-30",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 ACTIVE_AGREEMENT_EXISTS, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "ACTIVE_AGREEMENT_EXISTS" {
		t.Errorf("error.code = %v, want ACTIVE_AGREEMENT_EXISTS", errObj["code"])
	}
}

// ---------------------------------------------------------------------------
// Tests: RenewAgreement
// ---------------------------------------------------------------------------

func TestRenewAgreement_201_PredecessorSuperseded(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-RENEW")
	endDate := time.Now().UTC().AddDate(0, 1, 0) // expires next month
	pred := seedPKWTAgreement(h, emp.ID, "SWP-AG-PRED", "active", endDate)

	rr := h.doJSON("POST", "/agreements/"+pred.ID+":renew", map[string]any{
		"type":         "PKWT",
		"agreement_no": "PKT/2026/RENEW",
		"start_date":   "2026-07-01",
		"end_date":     "2027-06-30",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// New agreement must have predecessor_id set.
	if body["predecessor_id"] != pred.ID {
		t.Errorf("predecessor_id = %v, want %s", body["predecessor_id"], pred.ID)
	}

	// Location header for the new agreement.
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, "/api/v1/agreements/") {
		t.Errorf("Location = %q, want prefix /api/v1/agreements/", loc)
	}

	// Predecessor must now be SUPERSEDED in the repo.
	predInRepo := h.repo.agreements[pred.ID]
	if predInRepo.Status != "superseded" {
		t.Errorf("predecessor status = %v, want superseded", predInRepo.Status)
	}
}

func TestRenewAgreement_NonActivePredecessor_409(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-RENEW-CLOSED")

	// Seed a CLOSED predecessor — cannot renew.
	closed := domain.Agreement{
		ID:         "SWP-AG-CLOSED-PRED",
		EmployeeID: emp.ID,
		Type:       "PKWT",
		Status:     "closed",
		StartDate:  time.Now().UTC().AddDate(-1, 0, 0),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	h.repo.addAgreement(closed)

	rr := h.doJSON("POST", "/agreements/"+closed.ID+":renew", map[string]any{
		"type":         "PKWT",
		"agreement_no": "PKT/2026/RENEW-FAIL",
		"start_date":   "2026-07-01",
		"end_date":     "2027-06-30",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 CONFLICT, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "CONFLICT" {
		t.Errorf("error.code = %v, want CONFLICT", errObj["code"])
	}
}

// ---------------------------------------------------------------------------
// Tests: CloseAgreement
// ---------------------------------------------------------------------------

func TestCloseAgreement_200_ClosedReason(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-CLOSE")
	endDate := time.Now().UTC().AddDate(1, 0, 0)
	ag := seedPKWTAgreement(h, emp.ID, "SWP-AG-CLOSE-001", "active", endDate)

	rr := h.doJSON("POST", "/agreements/"+ag.ID+":close", map[string]any{
		"reason":         "RESIGNED",
		"effective_date": "2026-06-01",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "CLOSED" {
		t.Errorf("status = %v, want CLOSED", body["status"])
	}
	if body["closed_reason"] != "RESIGNED" {
		t.Errorf("closed_reason = %v, want RESIGNED", body["closed_reason"])
	}
}

func TestCloseAgreement_AlreadyClosed_409(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-CLOSE-DUP")

	// Seed an already-closed agreement.
	closedAg := domain.Agreement{
		ID:         "SWP-AG-ALREADY-CLOSED",
		EmployeeID: emp.ID,
		Type:       "PKWT",
		Status:     "closed",
		StartDate:  time.Now().UTC().AddDate(-1, 0, 0),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	h.repo.addAgreement(closedAg)

	rr := h.doJSON("POST", "/agreements/"+closedAg.ID+":close", map[string]any{
		"reason":         "OTHER",
		"effective_date": "2026-06-01",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 CONFLICT on re-close, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Tests: UploadAttachment (multipart, §15 FileRef shape, 413, bad mime)
// ---------------------------------------------------------------------------

func TestUploadAttachment_201_FileRefShape(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-ATT")
	endDate := time.Now().UTC().AddDate(1, 0, 0)
	ag := seedPKWTAgreement(h, emp.ID, "SWP-AG-ATT-001", "active", endDate)

	// Small PDF-like content.
	pdfContent := []byte("%PDF-1.4 fake pdf content for contract testing")

	rr := h.doMultipart("/agreements/"+ag.ID+"/attachments", "file", "contract.pdf", "application/pdf", pdfContent)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// §15 FileRef shape: all 6 keys must be present.
	for _, k := range []string{"id", "url", "name", "size_bytes", "mime", "uploaded_at"} {
		if _, ok := body[k]; !ok {
			t.Errorf("FileRef missing key: %s", k)
		}
	}

	// url must start with /api/v1/files/.
	url, _ := body["url"].(string)
	if !strings.HasPrefix(url, "/api/v1/files/") {
		t.Errorf("url = %q, want prefix /api/v1/files/", url)
	}

	// size_bytes must be a number.
	if _, ok := body["size_bytes"].(float64); !ok {
		t.Errorf("size_bytes is not a number: %T", body["size_bytes"])
	}

	// name must match the uploaded filename.
	if body["name"] != "contract.pdf" {
		t.Errorf("name = %v, want contract.pdf", body["name"])
	}

	// mime must be set.
	if body["mime"] == nil || body["mime"] == "" {
		t.Error("mime should be non-empty")
	}
}

func TestUploadAttachment_413_FILE_TOO_LARGE(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-ATT-BIG")
	endDate := time.Now().UTC().AddDate(1, 0, 0)
	ag := seedPKWTAgreement(h, emp.ID, "SWP-AG-ATT-BIG", "active", endDate)

	// Build a >10MB payload (10MB + 1 byte).
	const oversizeBytes = 10*1024*1024 + 1
	bigContent := bytes.Repeat([]byte("X"), oversizeBytes)

	rr := h.doMultipart("/agreements/"+ag.ID+"/attachments", "file", "big.pdf", "application/pdf", bigContent)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 FILE_TOO_LARGE, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "FILE_TOO_LARGE" {
		t.Errorf("error.code = %v, want FILE_TOO_LARGE", errObj["code"])
	}
}

func TestUploadAttachment_400_DisallowedMIME(t *testing.T) {
	h := newAgreementHarness(t)
	emp := seedAgreementEmployee(h, "SWP-EMP-ATT-BAD-MIME")
	endDate := time.Now().UTC().AddDate(1, 0, 0)
	ag := seedPKWTAgreement(h, emp.ID, "SWP-AG-ATT-BAD-MIME", "active", endDate)

	// text/plain is not in the allowed set.
	rr := h.doMultipart("/agreements/"+ag.ID+"/attachments", "file", "notes.txt", "text/plain", []byte("plain text"))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on disallowed MIME, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "INVALID_REQUEST" {
		t.Errorf("error.code = %v, want INVALID_REQUEST", errObj["code"])
	}
}

// ---------------------------------------------------------------------------
// Tests: DownloadFile — requires authentication
// ---------------------------------------------------------------------------

func TestDownloadFile_RequiresAuth_401(t *testing.T) {
	// Build a separate router where no principal is injected (simulates missing auth).
	repo := newFakeAgreementRepo()
	svc := peoplesvc.NewAgreementService(repo, &fakeTxRunner{})
	h := peoplehandler.NewAgreementHandler(svc)

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	// RequireRole without any principal in context → 401 UNAUTHENTICATED.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
		r.Get("/files/{file_id}", h.DownloadFile)
	})

	req := httptest.NewRequest("GET", "/files/SWP-FILE-001", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated download, got %d: %s", rr.Code, rr.Body.String())
	}
}
