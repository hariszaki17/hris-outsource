// Package leave_test — E6 F6.2 "agent file a leave request" contract tests.
//
// Covers POST /leave-requests (create-and-submit), POST /leave-requests/{id}:submit,
// the create-time validation order
// (INVALID_DATE_RANGE → MISSING_REQUIRED_DOCUMENT → OVERLAPPING_LEAVE → BACKDATED_LEAVE
// → QUOTA_EXCEEDED), agent SELF scope on List/Get, and the balance SELF guard. Asserted
// against docs/api/E6-leave/openapi.yaml.
package leave_test

import (
	"net/http"
	"testing"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// future dates relative to the handler-test fixedNow (2026-06-04).
var (
	createStart = ymd(2026, time.June, 20)
	createEnd   = ymd(2026, time.June, 22)
)

// seedAgentHarness builds an agent-scoped harness with an annual (pool) lot so the
// FIFO reservation succeeds on submit.
func seedAgentHarness(t *testing.T, lotAmount int) *harness {
	t.Helper()
	h := newHarness(t, auth.RoleAgent, cmpLed, empAgent)
	h.seedLeaveType(leaveAnn, "ANNUAL", true)
	h.seedGrant("SWP-LG-7001", empAgent, lotAmount, 0, 0, "", ymd(2027, time.January, 1))
	return h
}

// ---------------------------------------------------------------------------
// POST /leave-requests — agent create-and-submit (201)
// ---------------------------------------------------------------------------

func TestCreateLeaveRequest_AgentCreateAndSubmit201(t *testing.T) {
	h := seedAgentHarness(t, 12)

	rr := h.do("POST", "/leave-requests", map[string]any{
		"leave_type_id": leaveAnn,
		"start_date":    "2026-06-20",
		"end_date":      "2026-06-22",
		"reason":        "Acara keluarga di kampung halaman.",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc == "" {
		t.Errorf("missing Location header")
	}
	d := dataObject(t, rr)
	// submit defaults true → reserved → PENDING (E11 owns the chain progress).
	if d["status"] != "PENDING" {
		t.Errorf("status = %v, want PENDING (create-and-submit)", d["status"])
	}
	// the E11 approval instance was created + linked at submit.
	if d["approval_instance_id"] == nil {
		t.Errorf("approval_instance_id missing after create-and-submit: %v", d)
	}
	// duration_days is server-computed (3 inclusive days, fake stand-in).
	if dd, _ := d["duration_days"].(float64); int(dd) != 3 {
		t.Errorf("duration_days = %v, want 3 (server-computed)", d["duration_days"])
	}
	if d["employee_id"] != empAgent {
		t.Errorf("employee_id = %v, want %s (filled from token)", d["employee_id"], empAgent)
	}
}

func TestCreateLeaveRequest_DraftWhenSubmitFalse(t *testing.T) {
	h := seedAgentHarness(t, 12)
	submit := false
	rr := h.do("POST", "/leave-requests", map[string]any{
		"leave_type_id": leaveAnn,
		"start_date":    "2026-06-20",
		"end_date":      "2026-06-22",
		"reason":        "Disimpan dulu sebagai draf.",
		"submit":        submit,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if d := dataObject(t, rr); d["status"] != "DRAFT" {
		t.Errorf("status = %v, want DRAFT (submit=false)", d["status"])
	}
}

func TestCreateLeaveRequest_AgentOtherEmployee403(t *testing.T) {
	h := seedAgentHarness(t, 12)
	rr := h.do("POST", "/leave-requests", map[string]any{
		"leave_type_id": leaveAnn,
		"employee_id":   "SWP-EMP-9999", // someone else
		"start_date":    "2026-06-20",
		"end_date":      "2026-06-22",
		"reason":        "Mengajukan untuk orang lain (tidak boleh).",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateLeaveRequest_InvalidDateRange422(t *testing.T) {
	h := seedAgentHarness(t, 12)
	rr := h.do("POST", "/leave-requests", map[string]any{
		"leave_type_id": leaveAnn,
		"start_date":    "2026-06-22",
		"end_date":      "2026-06-20", // before start
		"reason":        "Tanggal terbalik.",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if c := errCode(t, rr); c != "INVALID_DATE_RANGE" {
		t.Errorf("code = %s, want INVALID_DATE_RANGE", c)
	}
}

func TestCreateLeaveRequest_MissingRequiredDocument422(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, cmpLed, empAgent)
	// document-required type, no document_file_id supplied.
	h.seedLeaveTypeFull("SWP-LT-DOC", "MATERNITY", true, true, false)
	h.seedGrant("SWP-LG-7002", empAgent, 90, 0, 0, "MATERNITY", ymd(2027, time.January, 1))
	rr := h.do("POST", "/leave-requests", map[string]any{
		"leave_type_id": "SWP-LT-DOC",
		"start_date":    "2026-06-20",
		"end_date":      "2026-06-22",
		"reason":        "Cuti yang butuh dokumen.",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if c := errCode(t, rr); c != "MISSING_REQUIRED_DOCUMENT" {
		t.Errorf("code = %s, want MISSING_REQUIRED_DOCUMENT", c)
	}
}

func TestCreateLeaveRequest_Overlapping409(t *testing.T) {
	h := seedAgentHarness(t, 12)
	// an existing live request overlapping [2026-06-20, 2026-06-22].
	h.seedRequest("SWP-LR-8010", cmpLed, empAgent, dom.LeaveStatusPending, createStart, createEnd, 3)
	rr := h.do("POST", "/leave-requests", map[string]any{
		"leave_type_id": leaveAnn,
		"start_date":    "2026-06-21",
		"end_date":      "2026-06-23",
		"reason":        "Tumpang tindih dengan cuti lain.",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if c := errCode(t, rr); c != "OVERLAPPING_LEAVE" {
		t.Errorf("code = %s, want OVERLAPPING_LEAVE", c)
	}
}

func TestCreateLeaveRequest_Backdated422(t *testing.T) {
	h := seedAgentHarness(t, 12)
	// start before fixedNow (2026-06-04); the annual type does not allow backdated.
	rr := h.do("POST", "/leave-requests", map[string]any{
		"leave_type_id": leaveAnn,
		"start_date":    "2026-06-01",
		"end_date":      "2026-06-02",
		"reason":        "Lupa mengajukan sebelumnya.",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if c := errCode(t, rr); c != "BACKDATED_LEAVE" {
		t.Errorf("code = %s, want BACKDATED_LEAVE", c)
	}
}

func TestCreateLeaveRequest_QuotaExceeded422(t *testing.T) {
	h := seedAgentHarness(t, 1) // only 1 day available, request needs 3
	rr := h.do("POST", "/leave-requests", map[string]any{
		"leave_type_id": leaveAnn,
		"start_date":    "2026-06-20",
		"end_date":      "2026-06-22",
		"reason":        "Saldo tidak mencukupi.",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if c := errCode(t, rr); c != "QUOTA_EXCEEDED" {
		t.Errorf("code = %s, want QUOTA_EXCEEDED", c)
	}
	// Per-type meter reports the over-cap as a localized leave_type_id field message.
	if f := errFields(t, rr); f["leave_type_id"] == nil {
		t.Errorf("QUOTA_EXCEEDED missing leave_type_id detail: %v", f)
	}
}

func TestCreateLeaveRequest_ShortReason400(t *testing.T) {
	h := seedAgentHarness(t, 12)
	rr := h.do("POST", "/leave-requests", map[string]any{
		"leave_type_id": leaveAnn,
		"start_date":    "2026-06-20",
		"end_date":      "2026-06-22",
		"reason":        "x", // < 5 chars
	})
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 400/422, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /leave-requests/{id}:submit
// ---------------------------------------------------------------------------

func TestSubmitLeaveRequest_DraftToPending(t *testing.T) {
	h := seedAgentHarness(t, 12)
	h.seedRequest("SWP-LR-8020", cmpLed, empAgent, dom.LeaveStatusDraft, createStart, createEnd, 3)
	rr := h.do("POST", "/leave-requests/SWP-LR-8020:submit", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if d := dataObject(t, rr); d["status"] != "PENDING" {
		t.Errorf("status = %v, want PENDING", d["status"])
	}
}

func TestSubmitLeaveRequest_NotDraft409(t *testing.T) {
	h := seedAgentHarness(t, 12)
	h.seedRequest("SWP-LR-8021", cmpLed, empAgent, dom.LeaveStatusPending, createStart, createEnd, 3)
	rr := h.do("POST", "/leave-requests/SWP-LR-8021:submit", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// agent SELF scope on List / Get
// ---------------------------------------------------------------------------

func TestListLeaveRequests_AgentSelfScoped(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, cmpLed, empAgent)
	h.seedRequest("SWP-LR-8030", cmpLed, empAgent, dom.LeaveStatusPending, createStart, createEnd, 3)
	h.seedRequest("SWP-LR-8031", cmpLed, "SWP-EMP-9999", dom.LeaveStatusPending, createStart, createEnd, 3)

	rr := h.do("GET", "/leave-requests", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("agent saw %d rows, want 1 (self only)", len(data))
	}
	if data[0].(map[string]any)["employee_id"] != empAgent {
		t.Errorf("agent saw another employee's row: %v", data[0])
	}
}

func TestGetLeaveRequest_AgentOther404(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, cmpLed, empAgent)
	h.seedRequest("SWP-LR-8040", cmpLed, "SWP-EMP-9999", dom.LeaveStatusPending, createStart, createEnd, 3)
	rr := h.do("GET", "/leave-requests/SWP-LR-8040", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetLeaveRequest_AgentSelf200(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, cmpLed, empAgent)
	h.seedRequest("SWP-LR-8041", cmpLed, empAgent, dom.LeaveStatusPending, createStart, createEnd, 3)
	rr := h.do("GET", "/leave-requests/SWP-LR-8041", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// (The aggregate grant-pool GET /leave-balances/by-employee/{id} was retired with the
// grant-lot ledger 2026-06-12; the per-type GET .../{id}/types replaces it and is
// covered against the live meter elsewhere.)
