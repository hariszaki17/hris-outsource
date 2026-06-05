// Package payroll_test — E8 async-export contract tests (PAY-02).
//
// The drift gate for POST /payslips:export, asserted byte-for-shape against
// docs/api/E8-payroll/openapi.yaml:
//
//	POST /payslips:export → 202 PayslipExportJob {id ^SWP-EXP-\d+$, status QUEUED,
//	                        format XLSX, confidential TRUE (server-forced despite a
//	                        false input), requested_by.{id}, scope.{period|year,
//	                        employee_ids}, poll_url}; ASSERT exactly one
//	                        PayslipExportArgs with the matching JobID was enqueued
//	                        in the export tx (transactional outbox).
//	                        EXPORT_TOO_LARGE → 422 + error.fields + NO enqueue;
//	                        no period AND no year → 422; agent/shift_leader → 403.
package payroll_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
)

var expJobIDRe = regexp.MustCompile(`^SWP-EXP-\d+$`)

// ---------------------------------------------------------------------------
// 202 + exact PayslipExportJob body + transactional-outbox enqueue.
// ---------------------------------------------------------------------------

func TestExportPayslips_202QueuedAndEnqueued(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.exports.countInScope = 12 // well under the threshold

	// confidential:false in the request — the server MUST coerce it to true.
	rr := h.doWithHeaders("POST", "/payslips:export",
		map[string]any{"period": "2025-12", "format": "XLSX", "confidential": false},
		map[string]string{"Idempotency-Key": "22222222-2222-2222-2222-222222222222"})
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	id, _ := body["id"].(string)
	if !expJobIDRe.MatchString(id) {
		t.Errorf("id = %q, want ^SWP-EXP-\\d+$", id)
	}
	if body["status"] != "QUEUED" {
		t.Errorf("status = %v, want QUEUED", body["status"])
	}
	if body["format"] != "XLSX" {
		t.Errorf("format = %v, want XLSX", body["format"])
	}
	// confidential server-forced true despite the false input (Wave 2.8 lock).
	if body["confidential"] != true {
		t.Errorf("confidential = %v, want true (server-forced)", body["confidential"])
	}
	// requested_by.id = the caller's employee id.
	rb, ok := body["requested_by"].(map[string]any)
	if !ok {
		t.Fatalf("requested_by missing/not an object: %v", body["requested_by"])
	}
	if rb["id"] != "SWP-EMP-9001" {
		t.Errorf("requested_by.id = %v, want SWP-EMP-9001", rb["id"])
	}
	// scope echoes the period + an (empty) employee_ids array.
	scope, ok := body["scope"].(map[string]any)
	if !ok {
		t.Fatalf("scope missing/not an object: %v", body["scope"])
	}
	if scope["period"] != "2025-12" {
		t.Errorf("scope.period = %v, want 2025-12", scope["period"])
	}
	eids, ok := scope["employee_ids"].([]any)
	if !ok || len(eids) != 0 {
		t.Errorf("scope.employee_ids = %v, want empty array", scope["employee_ids"])
	}
	if body["poll_url"] != "/api/v1/exports/"+id {
		t.Errorf("poll_url = %v, want /api/v1/exports/%s", body["poll_url"], id)
	}
	if loc := rr.Header().Get("Location"); loc != "/api/v1/exports/"+id {
		t.Errorf("Location = %q, want /api/v1/exports/%s", loc, id)
	}

	// Transactional-outbox: exactly one PayslipExportArgs with the matching JobID.
	if len(h.jobs.enqueued) != 1 {
		t.Fatalf("enqueued = %d jobs, want exactly 1", len(h.jobs.enqueued))
	}
	args, ok := h.jobs.enqueued[0].(jobs.PayslipExportArgs)
	if !ok {
		t.Fatalf("enqueued[0] = %T, want jobs.PayslipExportArgs", h.jobs.enqueued[0])
	}
	if args.JobID != id {
		t.Errorf("enqueued JobID = %q, want %q (matches the 202 body id)", args.JobID, id)
	}
	if args.Kind() != "payslip.export" {
		t.Errorf("enqueued Kind() = %q, want payslip.export", args.Kind())
	}
}

func TestExportPayslips_ByYearEchoesScope(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.exports.countInScope = 40

	rr := h.do("POST", "/payslips:export", map[string]any{
		"year":         2025,
		"employee_ids": []string{"SWP-EMP-1042"},
		"format":       "XLSX",
	})
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	scope := body["scope"].(map[string]any)
	if scope["year"] != float64(2025) {
		t.Errorf("scope.year = %v, want 2025", scope["year"])
	}
	eids, ok := scope["employee_ids"].([]any)
	if !ok || len(eids) != 1 || eids[0] != "SWP-EMP-1042" {
		t.Errorf("scope.employee_ids = %v, want [SWP-EMP-1042]", scope["employee_ids"])
	}
	// the by-year request still enqueued the worker.
	if len(h.jobs.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(h.jobs.enqueued))
	}
}

// ---------------------------------------------------------------------------
// EXPORT_TOO_LARGE → 422 + error.fields + NO enqueue.
// ---------------------------------------------------------------------------

func TestExportPayslips_TooLarge422NoEnqueue(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	h.exports.countInScope = 124000 // > 50,000 threshold

	rr := h.do("POST", "/payslips:export", map[string]any{"period": "2025-12", "format": "XLSX"})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "EXPORT_TOO_LARGE" {
		t.Errorf("code = %s, want EXPORT_TOO_LARGE", got)
	}
	if f := errFields(t, rr); f["period"] == nil {
		t.Errorf("fields.period missing on EXPORT_TOO_LARGE: %v", f)
	}
	// no job enqueued when the size guard rejects (tx never ran the enqueue).
	if len(h.jobs.enqueued) != 0 {
		t.Errorf("enqueued = %d jobs, want 0 (rejected before the tx)", len(h.jobs.enqueued))
	}
}

// ---------------------------------------------------------------------------
// no period AND no year → 422.
// ---------------------------------------------------------------------------

func TestExportPayslips_NoScope422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin)
	rr := h.do("POST", "/payslips:export", map[string]any{"format": "XLSX"})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "RULE_VIOLATION" {
		t.Errorf("code = %s, want RULE_VIOLATION", got)
	}
	if len(h.jobs.enqueued) != 0 {
		t.Errorf("enqueued = %d, want 0", len(h.jobs.enqueued))
	}
}

// ---------------------------------------------------------------------------
// RBAC — agent + shift_leader → 403.
// ---------------------------------------------------------------------------

func TestExportPayslips_RBACForbidden(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleAgent, auth.RoleShiftLeader} {
		t.Run(string(role), func(t *testing.T) {
			h := newHarness(t, role)
			rr := h.do("POST", "/payslips:export", map[string]any{"period": "2025-12", "format": "XLSX"})
			if rr.Code != http.StatusForbidden {
				t.Fatalf("expected 403 for %s, got %d: %s", role, rr.Code, rr.Body.String())
			}
			if len(h.jobs.enqueued) != 0 {
				t.Errorf("enqueued = %d, want 0 (blocked at RBAC)", len(h.jobs.enqueued))
			}
		})
	}
}
