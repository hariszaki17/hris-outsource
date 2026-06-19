// Package payroll_test — F8.3 Payroll Run contract tests (PR-1/PR-2/PR-3).
//
// The drift gate for the 6 run endpoints, asserted byte-for-shape against
// docs/api/E8-payroll/openapi.yaml:
//
//	POST /payroll-runs                  → 201 {data:<PayrollRun>} status=DRAFT; 409
//	                                      on duplicate month.
//	GET  /payroll-runs                  → 200 {data:[<PayrollRun>...]}
//	GET  /payroll-runs/{id}             → 200 {data:<PayrollRun>}
//	POST /payroll-runs/{id}:assemble     → 200 {data:{run,lines[],eligible_count,...}}
//	POST /payroll-runs/{id}:post         → 200 {data:{run,payslip_count,...}}
//	GET  /payroll-runs/{id}/payslips     → 200 {data:[<RunPayslip>...]}
//
// The fakeRunRepo implements RunRepository fully in-memory so the REAL
// RunService runs against it with a REAL crypto.Cipher.
package payroll_test

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
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	payrollhandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/crypto"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// ---------------------------------------------------------------------------
// fakeRunRepo — in-memory svc.RunRepository over shared maps.
// ---------------------------------------------------------------------------

type fakeRunRepo struct {
	runs               map[string]dom.PayrollRun
	eligibleEmployees  []svc.EligibleEmployeeRow
	activeAgreements   map[string]svc.ActiveAgreementRow
	pendingAdjustments map[string][]dom.PayrollAdjustment
	payslips           map[string]svc.GenPayslipRow
	seq                int
}

func newFakeRunRepo() *fakeRunRepo {
	return &fakeRunRepo{
		runs:               map[string]dom.PayrollRun{},
		activeAgreements:   map[string]svc.ActiveAgreementRow{},
		pendingAdjustments: map[string][]dom.PayrollAdjustment{},
		payslips:           map[string]svc.GenPayslipRow{},
	}
}

func (r *fakeRunRepo) InsertRun(_ context.Context, _ pgx.Tx, year, month int, cutoffDate time.Time, createdBy string) (dom.PayrollRun, error) {
	r.seq++
	run := dom.PayrollRun{
		ID:         "SWP-PRR-" + itoa(1000+r.seq),
		Year:       year,
		Month:      month,
		Status:     dom.RunStatusDraft,
		CutoffDate: cutoffDate,
		CreatedBy:  createdBy,
		CreatedAt:  fixedNow,
		UpdatedAt:  fixedNow,
	}
	r.runs[run.ID] = run
	return run, nil
}

func (r *fakeRunRepo) GetRun(_ context.Context, id string) (dom.PayrollRun, error) {
	run, ok := r.runs[id]
	if !ok {
		return dom.PayrollRun{}, domain.ErrNotFound
	}
	return run, nil
}

func (r *fakeRunRepo) GetRunByPeriod(_ context.Context, year, month int) (dom.PayrollRun, error) {
	for _, run := range r.runs {
		if run.Year == year && run.Month == month {
			return run, nil
		}
	}
	return dom.PayrollRun{}, domain.ErrNotFound
}

func (r *fakeRunRepo) ListRuns(_ context.Context, limit, offset int) ([]dom.PayrollRun, error) {
	all := make([]dom.PayrollRun, 0, len(r.runs))
	for _, run := range r.runs {
		all = append(all, run)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	if offset > len(all) {
		return nil, nil
	}
	all = all[offset:]
	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}
	return all, nil
}

func (r *fakeRunRepo) PostRun(_ context.Context, _ pgx.Tx, id string, postedBy string) (dom.PayrollRun, error) {
	run, ok := r.runs[id]
	if !ok {
		return dom.PayrollRun{}, domain.ErrNotFound
	}
	postedAt := fixedNow
	run.Status = dom.RunStatusPosted
	run.PostedBy = &postedBy
	run.PostedAt = &postedAt
	run.UpdatedAt = fixedNow
	r.runs[id] = run
	return run, nil
}

func (r *fakeRunRepo) ListEligibleEmployees(_ context.Context, _, _ time.Time) ([]svc.EligibleEmployeeRow, error) {
	return r.eligibleEmployees, nil
}

func (r *fakeRunRepo) GetActiveAgreementForEmployee(_ context.Context, employeeID string) (svc.ActiveAgreementRow, error) {
	agr, ok := r.activeAgreements[employeeID]
	if !ok {
		return svc.ActiveAgreementRow{}, domain.ErrNotFound
	}
	return agr, nil
}

func (r *fakeRunRepo) ListPendingAdjustments(_ context.Context, employeeID string) ([]dom.PayrollAdjustment, error) {
	return r.pendingAdjustments[employeeID], nil
}

func (r *fakeRunRepo) MarkAdjustmentsApplied(_ context.Context, _ pgx.Tx, _ []string, _ string) error {
	return nil
}

func (r *fakeRunRepo) InsertGeneratedPayslip(_ context.Context, _ pgx.Tx, p svc.InsertPayslipParams) (svc.GenPayslipRow, error) {
	row := svc.GenPayslipRow{
		ID:                 "SWP-PS-" + itoa(90000+len(r.payslips)+1),
		EmployeeID:         p.EmployeeID,
		EmployeeName:       strp(p.EmployeeName),
		PlacementID:        p.PlacementID,
		Year:               p.Year,
		Month:              p.Month,
		WorkingDays:        p.WorkingDays,
		GrossEarningsEnc:   p.GrossEarningsEnc,
		GrossDeductionsEnc: p.GrossDeductionsEnc,
		TakeHomePayEnc:     p.TakeHomePayEnc,
		Status:             p.Status,
		SourceSystem:       p.SourceSystem,
		SourceID:           p.SourceID,
		PayrollRunID:       &p.PayrollRunID,
		IsPosted:           p.IsPosted,
		SourceType:         p.SourceType,
		PaymentStatus:      p.PaymentStatus,
		CreatedAt:          fixedNow,
	}
	r.payslips[row.ID] = row
	return row, nil
}

func (r *fakeRunRepo) ListRunPayslips(_ context.Context, runID string, limit int) ([]svc.GenPayslipRow, error) {
	var out []svc.GenPayslipRow
	for _, ps := range r.payslips {
		if ps.PayrollRunID != nil && *ps.PayrollRunID == runID {
			out = append(out, ps)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeRunRepo) RunExists(_ context.Context, id string) (bool, error) {
	_, ok := r.runs[id]
	return ok, nil
}

func (r *fakeRunRepo) CountRunPayslips(_ context.Context, runID string) (int, error) {
	count := 0
	for _, ps := range r.payslips {
		if ps.PayrollRunID != nil && *ps.PayrollRunID == runID {
			count++
		}
	}
	return count, nil
}

var _ svc.RunRepository = (*fakeRunRepo)(nil)

// ---------------------------------------------------------------------------
// runHarness — mounts the REAL RunService + RunHandler over the fakes + cipher.
// ---------------------------------------------------------------------------

type runHarness struct {
	router    *chi.Mux
	runs      *fakeRunRepo
	cipher    *crypto.Cipher
	idem      *stubIdempotency
	principal auth.Principal
}

func newRunHarness(t *testing.T, role auth.Role) *runHarness {
	t.Helper()
	repo := newFakeRunRepo()
	cipher := newTestCipher(t)
	svc := svc.NewRunService(repo, &fakeTxRunner{}, cipher)
	handler := payrollhandler.NewRunHandler(svc)
	idem := newStubIdempotency()

	h := &runHarness{
		runs:   repo,
		cipher: cipher,
		idem:   idem,
		principal: auth.Principal{
			UserID:     "SWP-USR-9001",
			Role:       role,
			CompanyID:  "",
			EmployeeID: "SWP-EMP-9001",
		},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), h.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.With(idem.Handler).Post("/payroll-runs", handler.OpenRun)
		r.Get("/payroll-runs", handler.ListRuns)
		r.Get("/payroll-runs/{id}", handler.GetRun)
		r.Post("/payroll-runs/{id}:assemble", handler.AssembleRun)
		r.With(idem.Handler).Post("/payroll-runs/{id}:post", handler.PostRun)
		r.Get("/payroll-runs/{id}/payslips", handler.ListRunPayslips)
	})

	h.router = r
	return h
}

func (h *runHarness) do(method, path string, body any) *httptest.ResponseRecorder {
	return h.doWithHeaders(method, path, body, nil)
}

func (h *runHarness) doWithHeaders(method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	buf := bytes.Buffer{}
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// stubs: eligible employees + active agreements for the assemble test.
// ---------------------------------------------------------------------------

func seedEligibleEmployees(h *runHarness) {
	h.runs.eligibleEmployees = []svc.EligibleEmployeeRow{
		{ID: "SWP-EMP-1042", FullName: "Budi Santoso", EmployeeType: "FIELD"},
		{ID: "SWP-EMP-1118", FullName: "Rudi Hartono", EmployeeType: "FIELD"},
	}
	for _, emp := range h.runs.eligibleEmployees {
		h.runs.activeAgreements[emp.ID] = svc.ActiveAgreementRow{
			ID:            "SWP-EA-" + emp.ID[8:],
			EmployeeID:    emp.ID,
			BaseSalaryIDR: f64ptr(6500000),
			StartDate:     ymd(2025, time.January, 1),
			Status:        "ACTIVE",
		}
	}
}

func f64ptr(v float64) *float64 { return &v }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestOpenRun_Happy_201(t *testing.T) {
	h := newRunHarness(t, auth.RoleHRAdmin)
	rr := h.do("POST", "/payroll-runs", map[string]any{
		"year":        2026,
		"month":       6,
		"cutoff_date": "2026-06-30",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["id"] == nil || strOf(d["id"]) == "" {
		t.Errorf("id missing/empty in response: %v", d)
	}
	if d["status"] != "DRAFT" {
		t.Errorf("status = %v, want DRAFT", d["status"])
	}
	if d["year"] != float64(2026) {
		t.Errorf("year = %v, want 2026", d["year"])
	}
	if d["month"] != float64(6) {
		t.Errorf("month = %v, want 6", d["month"])
	}
	if d["cutoff_date"] != "2026-06-30" {
		t.Errorf("cutoff_date = %v, want 2026-06-30", d["cutoff_date"])
	}
	loc := rr.Header().Get("Location")
	if loc == "" || loc != "/api/v1/payroll-runs/"+strOf(d["id"]) {
		t.Errorf("Location = %q, want /api/v1/payroll-runs/<id>", loc)
	}
}

func TestOpenRun_DuplicateMonth_409(t *testing.T) {
	h := newRunHarness(t, auth.RoleHRAdmin)
	h.do("POST", "/payroll-runs", map[string]any{
		"year": 2026, "month": 6, "cutoff_date": "2026-06-30",
	})
	rr := h.do("POST", "/payroll-runs", map[string]any{
		"year": 2026, "month": 6, "cutoff_date": "2026-06-30",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 on duplicate month, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "PAYROLL_RUN_ALREADY_EXISTS" && got != "CONFLICT" {
		t.Errorf("error code = %q", got)
	}
}

func TestListRuns_200(t *testing.T) {
	h := newRunHarness(t, auth.RoleHRAdmin)
	h.do("POST", "/payroll-runs", map[string]any{"year": 2026, "month": 6, "cutoff_date": "2026-06-30"})
	h.do("POST", "/payroll-runs", map[string]any{"year": 2026, "month": 5, "cutoff_date": "2026-05-31"})
	h.do("POST", "/payroll-runs", map[string]any{"year": 2026, "month": 4, "cutoff_date": "2026-04-30"})

	rr := h.do("GET", "/payroll-runs", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("data length = %d, want 3", len(data))
	}
}

func TestGetRun_200(t *testing.T) {
	h := newRunHarness(t, auth.RoleHRAdmin)
	rr1 := h.do("POST", "/payroll-runs", map[string]any{"year": 2026, "month": 6, "cutoff_date": "2026-06-30"})
	runID := strOf(dataObject(t, rr1)["id"])

	rr := h.do("GET", "/payroll-runs/"+runID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["id"] != runID {
		t.Errorf("run id = %v, want %s", d["id"], runID)
	}
	if d["status"] != "DRAFT" {
		t.Errorf("status = %v, want DRAFT", d["status"])
	}
}

func TestAssembleRun_200(t *testing.T) {
	h := newRunHarness(t, auth.RoleHRAdmin)
	rr1 := h.do("POST", "/payroll-runs", map[string]any{"year": 2026, "month": 6, "cutoff_date": "2026-06-30"})
	runID := strOf(dataObject(t, rr1)["id"])
	seedEligibleEmployees(h)

	rr := h.do("POST", "/payroll-runs/"+runID+":assemble", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	d, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing/not an object: %v", body)
	}
	if d["eligible_count"] != float64(2) {
		t.Errorf("eligible_count = %v, want 2", d["eligible_count"])
	}
	lines, ok := d["lines"].([]any)
	if !ok || len(lines) != 2 {
		t.Fatalf("lines = %v, want 2 entries", d["lines"])
	}
	l0 := lines[0].(map[string]any)
	if l0["employee_name"] != "Budi Santoso" {
		t.Errorf("lines[0].employee_name = %v, want Budi Santoso", l0["employee_name"])
	}
	if l0["gross_earnings"] != "6500000.00" {
		t.Errorf("lines[0].gross_earnings = %v, want 6500000.00", l0["gross_earnings"])
	}
	run := d["run"].(map[string]any)
	if run["status"] != "DRAFT" {
		t.Errorf("run.status = %v, want DRAFT", run["status"])
	}
}

func TestPostRun_200(t *testing.T) {
	h := newRunHarness(t, auth.RoleHRAdmin)
	rr1 := h.do("POST", "/payroll-runs", map[string]any{"year": 2026, "month": 6, "cutoff_date": "2026-06-30"})
	runID := strOf(dataObject(t, rr1)["id"])
	seedEligibleEmployees(h)

	assembleRR := h.do("POST", "/payroll-runs/"+runID+":assemble", nil)
	if assembleRR.Code != http.StatusOK {
		t.Fatalf("assemble: expected 200, got %d: %s", assembleRR.Code, assembleRR.Body.String())
	}
	assembleBody := decodeBody(t, assembleRR)
	assembleData, _ := assembleBody["data"].(map[string]any)
	lines := assembleData["lines"].([]any)

	postBody := map[string]any{"lines": make([]map[string]any, 0, len(lines))}
	for _, raw := range lines {
		l := raw.(map[string]any)
		postBody["lines"] = append(postBody["lines"].([]map[string]any), map[string]any{
			"employee_id":      l["employee_id"],
			"employee_name":    l["employee_name"],
			"employee_type":    l["employee_type"],
			"base_salary_idr":  l["base_salary_idr"],
			"working_days":     l["working_days"],
			"gross_earnings":   l["gross_earnings"],
			"gross_deductions": l["gross_deductions"],
			"take_home_pay":    l["take_home_pay"],
			"adjustment_ids":   l["adjustment_ids"],
		})
	}

	rr := h.do("POST", "/payroll-runs/"+runID+":post", postBody)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	run := d["run"].(map[string]any)
	if run["status"] != "POSTED" {
		t.Errorf("run.status = %v, want POSTED", run["status"])
	}
	if strOf(run["posted_by"]) != "SWP-EMP-9001" {
		t.Errorf("run.posted_by = %v, want SWP-EMP-9001", run["posted_by"])
	}
	if d["payslip_count"] != float64(2) {
		t.Errorf("payslip_count = %v, want 2", d["payslip_count"])
	}
}

func TestRunRoutes_AgentForbidden_403(t *testing.T) {
	h := newRunHarness(t, auth.RoleAgent)
	rr := h.do("POST", "/payroll-runs", map[string]any{
		"year": 2026, "month": 6, "cutoff_date": "2026-06-30",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for agent, got %d: %s", rr.Code, rr.Body.String())
	}
}
