// Package reporting_test — E10 generic export-framework contract tests (F10.4),
// asserted against docs/api/E10-reporting/openapi.yaml:
//
//	POST /exports {ATTENDANCE_BILLABLE, EXCEL, filters} → 202 + ExportJob
//	    (status QUEUED) under {data}; ASSERT exactly one ReportExportArgs whose
//	    JobID == the returned id was enqueued in the export tx (transactional
//	    outbox).
//	POST /exports format=PDF → 422 EXPORT_FORMAT_UNSUPPORTED (fields.format).
//	POST /exports oversized scope → 422 EXPORT_TOO_LARGE (fields.period_end).
//	POST /exports tripping the throttle → 429 RATE_LIMITED_EXPORTS.
//	GET /exports/{id}: DB RUNNING → wire PROCESSING; DB DONE → wire COMPLETED
//	    (+ file_url/filename/size_bytes); non-owner → 404.
//	POST /exports/{id}:cancel QUEUED → 200 CANCELLED; terminal → 200 no-op.
//	RBAC: agent → 403 on POST /exports (x-rbac excludes agent).
package reporting_test

import (
	"net/http"
	"regexp"
	"testing"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
)

var exportJobIDRe = regexp.MustCompile(`^SWP-EXP-\d+$`)

// billableExportBody is the canonical POST /exports request used across cases.
func billableExportBody(format string) map[string]any {
	return map[string]any{
		"report_type": "ATTENDANCE_BILLABLE",
		"format":      format,
		"filters": map[string]any{
			"company_id":   "SWP-CMP-0021",
			"period_start": "2026-06-01",
			"period_end":   "2026-06-30",
			"group_by":     "employee",
		},
	}
}

func i64p(n int64) *int64 { return &n }
func ip(n int) *int       { return &n }

// ---------------------------------------------------------------------------
// 202 + ExportJob body + transactional-outbox enqueue.
// ---------------------------------------------------------------------------

func TestCreateExport_202QueuedAndEnqueued(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	h.billable.countInScope = 1200 // well under the row cap
	h.exports.countRecent = 0      // under the throttle

	rr := h.doWithHeaders("POST", "/exports", billableExportBody("EXCEL"),
		map[string]string{"Idempotency-Key": "33333333-3333-3333-3333-333333333333"})
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)

	id, _ := d["id"].(string)
	if !exportJobIDRe.MatchString(id) {
		t.Errorf("id = %q, want ^SWP-EXP-\\d+$", id)
	}
	if d["status"] != "QUEUED" {
		t.Errorf("status = %v, want QUEUED", d["status"])
	}
	if d["report_type"] != "ATTENDANCE_BILLABLE" {
		t.Errorf("report_type = %v, want ATTENDANCE_BILLABLE", d["report_type"])
	}
	if d["format"] != "EXCEL" {
		t.Errorf("format = %v, want EXCEL", d["format"])
	}
	if d["requester_id"] != "SWP-USR-9001" {
		t.Errorf("requester_id = %v, want SWP-USR-9001", d["requester_id"])
	}
	// audit_log_entry_id present (openapi required, non-null).
	if al, _ := d["audit_log_entry_id"].(string); al == "" {
		t.Errorf("audit_log_entry_id = %v, want a non-empty SWP-AL id", d["audit_log_entry_id"])
	}
	// filters echoed.
	filters, ok := d["filters"].(map[string]any)
	if !ok {
		t.Fatalf("filters missing/not object: %v", d["filters"])
	}
	if filters["company_id"] != "SWP-CMP-0021" {
		t.Errorf("filters.company_id = %v, want SWP-CMP-0021", filters["company_id"])
	}

	// Transactional-outbox: exactly one ReportExportArgs with the matching JobID.
	if len(h.jobs.enqueued) != 1 {
		t.Fatalf("enqueued = %d jobs, want exactly 1", len(h.jobs.enqueued))
	}
	args, ok := h.jobs.enqueued[0].(jobs.ReportExportArgs)
	if !ok {
		t.Fatalf("enqueued[0] = %T, want jobs.ReportExportArgs", h.jobs.enqueued[0])
	}
	if args.JobID != id {
		t.Errorf("enqueued JobID = %q, want %q (matches the 202 body id)", args.JobID, id)
	}
	if args.Kind() != "report.export" {
		t.Errorf("enqueued Kind() = %q, want report.export", args.Kind())
	}
	if args.ReportType != "ATTENDANCE_BILLABLE" {
		t.Errorf("enqueued ReportType = %q, want ATTENDANCE_BILLABLE", args.ReportType)
	}
}

// ---------------------------------------------------------------------------
// EXPORT_FORMAT_UNSUPPORTED 422 (PDF) — no enqueue.
// ---------------------------------------------------------------------------

func TestCreateExport_PdfUnsupported422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")

	rr := h.do("POST", "/exports", billableExportBody("PDF"))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "EXPORT_FORMAT_UNSUPPORTED" {
		t.Errorf("code = %s, want EXPORT_FORMAT_UNSUPPORTED", got)
	}
	if f := errFields(t, rr); f["format"] == nil {
		t.Errorf("fields.format missing on EXPORT_FORMAT_UNSUPPORTED: %v", f)
	}
	if len(h.jobs.enqueued) != 0 {
		t.Errorf("enqueued = %d, want 0 (rejected before the tx)", len(h.jobs.enqueued))
	}
}

// ---------------------------------------------------------------------------
// EXPORT_TOO_LARGE 422 — oversized scope; no enqueue.
// ---------------------------------------------------------------------------

func TestCreateExport_TooLarge422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	h.billable.countInScope = 300000 // > 250k row cap

	rr := h.do("POST", "/exports", billableExportBody("EXCEL"))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "EXPORT_TOO_LARGE" {
		t.Errorf("code = %s, want EXPORT_TOO_LARGE", got)
	}
	if f := errFields(t, rr); f["period_end"] == nil {
		t.Errorf("fields.period_end missing on EXPORT_TOO_LARGE: %v", f)
	}
	if len(h.jobs.enqueued) != 0 {
		t.Errorf("enqueued = %d, want 0 (size guard rejected)", len(h.jobs.enqueued))
	}
}

// ---------------------------------------------------------------------------
// RATE_LIMITED_EXPORTS 429 — throttle tripped.
// ---------------------------------------------------------------------------

func TestCreateExport_RateLimited429(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	h.billable.countInScope = 100
	h.exports.countRecent = 30 // == throttleMax

	rr := h.do("POST", "/exports", billableExportBody("EXCEL"))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "RATE_LIMITED_EXPORTS" {
		t.Errorf("code = %s, want RATE_LIMITED_EXPORTS", got)
	}
	if len(h.jobs.enqueued) != 0 {
		t.Errorf("enqueued = %d, want 0 (throttled)", len(h.jobs.enqueued))
	}
}

// ---------------------------------------------------------------------------
// GET /exports/{id} — DB→wire status mapping + scope.
// ---------------------------------------------------------------------------

func TestGetExport_StatusMapping(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	completed := fixedNow.Add(time.Minute)

	// DB RUNNING → wire PROCESSING.
	h.exports.seedJob(dom.ExportJob{
		ID:              "SWP-EXP-2001",
		ReportType:      dom.ReportAttendanceBillable,
		Status:          dom.StatusRunning,
		Format:          dom.FormatExcel,
		ProgressPercent: ip(40),
		AuditLogEntryID: strp("SWP-AL-1204520"),
		RequesterID:     "SWP-USR-9001",
		RequestedAt:     fixedNow,
	})
	// DB DONE → wire COMPLETED (+ file_url/filename/size_bytes).
	h.exports.seedJob(dom.ExportJob{
		ID:              "SWP-EXP-2002",
		ReportType:      dom.ReportAttendanceBillable,
		Status:          dom.StatusDone,
		Format:          dom.FormatExcel,
		ProgressPercent: ip(100),
		Filename:        strp("billable-plaza-senayan-2026-06.xlsx"),
		SizeBytes:       i64p(184320),
		AuditLogEntryID: strp("SWP-AL-1204520"),
		RequesterID:     "SWP-USR-9001",
		RequestedAt:     fixedNow,
		CompletedAt:     &completed,
	})

	rr := h.do("GET", "/exports/SWP-EXP-2001", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("PROCESSING get expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "PROCESSING" {
		t.Errorf("DB RUNNING wire status = %v, want PROCESSING", d["status"])
	}
	if d["file_url"] != nil {
		t.Errorf("file_url = %v, want null while PROCESSING", d["file_url"])
	}

	rr2 := h.do("GET", "/exports/SWP-EXP-2002", nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("COMPLETED get expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	d2 := dataObject(t, rr2)
	if d2["status"] != "COMPLETED" {
		t.Errorf("DB DONE wire status = %v, want COMPLETED", d2["status"])
	}
	if d2["filename"] != "billable-plaza-senayan-2026-06.xlsx" {
		t.Errorf("filename = %v, want the seeded artifact name", d2["filename"])
	}
	if d2["size_bytes"] != float64(184320) {
		t.Errorf("size_bytes = %v, want 184320", d2["size_bytes"])
	}
	if url, _ := d2["file_url"].(string); url == "" {
		t.Errorf("file_url = %v, want a download URL once COMPLETED", d2["file_url"])
	}
}

func TestGetExport_NonOwner404(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	// Job owned by ANOTHER requester.
	h.exports.seedJob(dom.ExportJob{
		ID:          "SWP-EXP-3001",
		ReportType:  dom.ReportAttendanceBillable,
		Status:      dom.StatusQueued,
		Format:      dom.FormatExcel,
		RequesterID: "SWP-USR-5555",
		RequestedAt: fixedNow,
	})

	rr := h.do("GET", "/exports/SWP-EXP-3001", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-owned job, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "NOT_FOUND" {
		t.Errorf("code = %s, want NOT_FOUND", got)
	}
}

// ---------------------------------------------------------------------------
// POST /exports/{id}:cancel — QUEUED → CANCELLED; terminal → no-op.
// ---------------------------------------------------------------------------

func TestCancelExport_QueuedToCancelled(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	h.exports.seedJob(dom.ExportJob{
		ID:          "SWP-EXP-4001",
		ReportType:  dom.ReportAttendanceBillable,
		Status:      dom.StatusQueued,
		Format:      dom.FormatExcel,
		RequesterID: "SWP-USR-9001",
		RequestedAt: fixedNow,
	})

	rr := h.doWithHeaders("POST", "/exports/SWP-EXP-4001:cancel", nil,
		map[string]string{"Idempotency-Key": "44444444-4444-4444-4444-444444444444"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if dataObject(t, rr)["status"] != "CANCELLED" {
		t.Errorf("status = %v, want CANCELLED", dataObject(t, rr)["status"])
	}
}

func TestCancelExport_TerminalNoOp(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	completed := fixedNow
	// Already DONE → cancel is a no-op (status stays COMPLETED on the wire).
	h.exports.seedJob(dom.ExportJob{
		ID:          "SWP-EXP-4002",
		ReportType:  dom.ReportAttendanceBillable,
		Status:      dom.StatusDone,
		Format:      dom.FormatExcel,
		Filename:    strp("done.xlsx"),
		RequesterID: "SWP-USR-9001",
		RequestedAt: fixedNow,
		CompletedAt: &completed,
	})

	rr := h.do("POST", "/exports/SWP-EXP-4002:cancel", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	// DB DONE maps to wire COMPLETED — the no-op left it unchanged.
	if dataObject(t, rr)["status"] != "COMPLETED" {
		t.Errorf("terminal cancel status = %v, want COMPLETED (no-op)", dataObject(t, rr)["status"])
	}
}

// ---------------------------------------------------------------------------
// RBAC — agent → 403 on POST /exports (x-rbac excludes agent).
// ---------------------------------------------------------------------------

func TestCreateExport_AgentForbidden(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", "SWP-EMP-3104")

	rr := h.do("POST", "/exports", billableExportBody("EXCEL"))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for agent, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.jobs.enqueued) != 0 {
		t.Errorf("enqueued = %d, want 0 (blocked at RBAC)", len(h.jobs.enqueued))
	}
}
