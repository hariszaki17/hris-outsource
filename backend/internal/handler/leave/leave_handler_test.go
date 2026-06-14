// Package leave_test — E6 leave-request read + lifecycle contract tests.
//
// The drift gate for the leave-request read endpoints, asserted byte-for-shape
// against docs/api/E6-leave/openapi.yaml. Approval (approve-l1/final/override/reject)
// moved to the E11 engine and is tested there; the leave surface now exposes only
// reads, the agent file/submit/cancel actions, cancel-approved, shorten, and the
// per-type quota read/adjust. Status is the collapsed enum
// (DRAFT|PENDING|APPROVED|REJECTED|CANCELLED).
//
//	GET  /leave-requests       → 200 {data,next_cursor,has_more}; leader-scope; OUT_OF_SCOPE 403
//	GET  /leave-requests/{id}  → 200 {data} full LeaveRequest (approval_instance_id); cross-scope → 404
package leave_test

import (
	"net/http"
	"testing"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// Persona / fixture constants mirror the 08-02 seed.
const (
	cmpLed    = "SWP-CMP-0021" // the leader's led company
	cmpOther  = "SWP-CMP-0022" // a company the leader does NOT lead
	empLeader = "SWP-EMP-2003" // Rudi — the shift-leader persona
	empAgent  = "SWP-EMP-3001" // an agent at the led company
	empHR     = "SWP-EMP-9001" // the HR/super persona's employee id
	leaveAnn  = "SWP-LT-001"   // annual (quota-tracked) leave type
)

var (
	leaveStart = ymd(2026, time.June, 15)
	leaveEnd   = ymd(2026, time.June, 17)
)

// ---------------------------------------------------------------------------
// GET /leave-requests — list envelope + leader scope
// ---------------------------------------------------------------------------

func TestListLeaveRequests_Envelope(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedRequest("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPending, leaveStart, leaveEnd, 3)
	h.seedRequest("SWP-LR-8005", cmpLed, empAgent, dom.LeaveStatusApproved, leaveStart, leaveEnd, 3)

	rr := h.do("GET", "/leave-requests?company_id="+cmpLed, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data missing/not an array: %T", body["data"])
	}
	if len(data) != 2 {
		t.Errorf("data length = %d, want 2", len(data))
	}
	if _, present := body["next_cursor"]; !present {
		t.Errorf("next_cursor key missing from envelope")
	}
	if _, ok := body["has_more"].(bool); !ok {
		t.Errorf("has_more missing/not a bool: %T", body["has_more"])
	}
	// Spot-check the LeaveRequest shape on the first row (openapi LeaveRequest).
	row := data[0].(map[string]any)
	for _, k := range []string{"id", "employee_id", "leave_type_id", "start_date", "end_date", "duration_days", "status", "approval_instance_id", "balance_check"} {
		if _, ok := row[k]; !ok {
			t.Errorf("leave request row missing key: %s", k)
		}
	}
}

func TestListLeaveRequests_HasMoreCursor(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	// distinct created_at so the keyset ordering is deterministic.
	a := h.seedRequest("SWP-LR-8001", cmpLed, empAgent, dom.LeaveStatusPending, leaveStart, leaveEnd, 1)
	a.CreatedAt = ymd(2026, time.June, 3)
	h.leave.requests[a.ID] = a
	b := h.seedRequest("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPending, leaveStart, leaveEnd, 1)
	b.CreatedAt = ymd(2026, time.June, 2)
	h.leave.requests[b.ID] = b
	c := h.seedRequest("SWP-LR-8003", cmpLed, empAgent, dom.LeaveStatusApproved, leaveStart, leaveEnd, 1)
	c.CreatedAt = ymd(2026, time.June, 1)
	h.leave.requests[c.ID] = c

	rr := h.do("GET", "/leave-requests?company_id="+cmpLed+"&limit=2", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if hm, _ := body["has_more"].(bool); !hm {
		t.Errorf("has_more = false, want true (3 rows, limit 2)")
	}
	cur, ok := body["next_cursor"].(string)
	if !ok || cur == "" {
		t.Fatalf("next_cursor missing/empty when has_more: %v", body["next_cursor"])
	}
	rr2 := h.do("GET", "/leave-requests?company_id="+cmpLed+"&limit=2&cursor="+cur, nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("page 2: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	data2 := decodeBody(t, rr2)["data"].([]any)
	if len(data2) != 1 {
		t.Errorf("page 2 data length = %d, want 1 (the remaining row)", len(data2))
	}
}

func TestListLeaveRequests_LeaderScopeForced(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedRequest("SWP-LR-8001", cmpLed, empAgent, dom.LeaveStatusPending, leaveStart, leaveEnd, 1)
	h.seedRequest("SWP-LR-8004", cmpOther, "SWP-EMP-2891", dom.LeaveStatusPending, leaveStart, leaveEnd, 1)

	rr := h.do("GET", "/leave-requests", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("leader saw %d rows, want 1 (own company only)", len(data))
	}
}

func TestListLeaveRequests_LeaderCrossCompany403(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	rr := h.do("GET", "/leave-requests?company_id="+cmpOther, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "OUT_OF_SCOPE" {
		t.Errorf("code = %s, want OUT_OF_SCOPE", got)
	}
}

// ---------------------------------------------------------------------------
// GET /leave-requests/{id} — full detail (approval_instance_id); cross-scope 404
// ---------------------------------------------------------------------------

func TestGetLeaveRequest_FullShape(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	req := h.seedRequest("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPending, leaveStart, leaveEnd, 3)
	inst := "SWP-API-9001"
	req.ApprovalInstanceID = &inst
	h.leave.requests[req.ID] = req

	rr := h.do("GET", "/leave-requests/SWP-LR-8002", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "PENDING" {
		t.Errorf("status = %v, want PENDING", d["status"])
	}
	// the E11 instance link is surfaced (clients read the chain from E11, not here).
	if d["approval_instance_id"] != inst {
		t.Errorf("approval_instance_id = %v, want %s", d["approval_instance_id"], inst)
	}
	// the legacy routing/timeline objects are gone (E11 owns the chain).
	if _, ok := d["timeline"]; ok {
		t.Errorf("timeline must not be serialized (E11 owns the chain): %v", d["timeline"])
	}
	if _, ok := d["routing"]; ok {
		t.Errorf("routing must not be serialized (E11 owns routing): %v", d["routing"])
	}
}

func TestGetLeaveRequest_CrossScope404(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedRequest("SWP-LR-8004", cmpOther, "SWP-EMP-2891", dom.LeaveStatusPending, leaveStart, leaveEnd, 1)
	rr := h.do("GET", "/leave-requests/SWP-LR-8004", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "NOT_FOUND" {
		t.Errorf("code = %s, want NOT_FOUND", got)
	}
}

// ---------------------------------------------------------------------------
// POST :cancel-approved — APPROVED → CANCELLED; short reason 422
// ---------------------------------------------------------------------------

func TestCancelApproved_HRRestores(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedLeaveType(leaveAnn, "ANNUAL", true)
	// future approved leave (start after fixedNow 2026-06-04) with a window to reverse.
	h.seedWindow("SWP-LQ-8001", empAgent, 12, 3, 0)
	h.seedRequest("SWP-LR-8050", cmpLed, empAgent, dom.LeaveStatusApproved, leaveStart, leaveEnd, 3)

	rr := h.do("POST", "/leave-requests/SWP-LR-8050:cancel-approved", map[string]any{"reason": "Pembatalan disetujui HR."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if dataObject(t, rr)["status"] != "CANCELLED" {
		t.Errorf("status = %v, want CANCELLED", dataObject(t, rr)["status"])
	}
}

func TestCancelApproved_ShortReason422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedRequest("SWP-LR-8051", cmpLed, empAgent, dom.LeaveStatusApproved, leaveStart, leaveEnd, 3)
	rr := h.do("POST", "/leave-requests/SWP-LR-8051:cancel-approved", map[string]any{"reason": "no"})
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 400/422, got %d: %s", rr.Code, rr.Body.String())
	}
	if f := errFields(t, rr); f["reason"] == nil {
		t.Errorf("fields.reason missing on a short-reason cancel-approved: %v", f)
	}
}

// ---------------------------------------------------------------------------
// POST :cancel — withdraw a PENDING request (releases the reservation)
// ---------------------------------------------------------------------------

func TestCancel_PendingWithdrawn(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, cmpLed, empAgent)
	h.seedLeaveType(leaveAnn, "ANNUAL", true)
	h.seedWindow("SWP-LQ-8060", empAgent, 12, 0, 3)
	h.seedRequest("SWP-LR-8060", cmpLed, empAgent, dom.LeaveStatusPending, leaveStart, leaveEnd, 3)

	rr := h.do("POST", "/leave-requests/SWP-LR-8060:cancel", map[string]any{"reason": "Berubah rencana."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if dataObject(t, rr)["status"] != "CANCELLED" {
		t.Errorf("status = %v, want CANCELLED", dataObject(t, rr)["status"])
	}
	// the pending reservation was released.
	if w := h.meterStore.windowFor(empAgent, leaveAnn, "2026"); w == nil || w.PendingDays != 0 {
		t.Errorf("window pending = %v, want 0 (released)", w)
	}
}
