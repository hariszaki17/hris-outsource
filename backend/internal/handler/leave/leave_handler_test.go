// Package leave_test — E6 leave-request approval (F6.1 / LVE-01) contract tests.
//
// The drift gate for the 6 leave-request endpoints, asserted byte-for-shape
// against docs/api/E6-leave/openapi.yaml:
//
//	GET  /leave-requests                 → 200 {data,next_cursor,has_more}; leader-scope; OUT_OF_SCOPE 403
//	GET  /leave-requests/{id}            → 200 {data} full LeaveRequest incl timeline; cross-scope → 404
//	POST :approve-l1                     → 200 PENDING_HR (ApprovedL1Example); wrong-state 409; cross-company 403;
//	                                       self-approve 403 FORBIDDEN
//	POST :approve-final                  → 200 APPROVED (ApprovedFinalExample); INV-3 fired (schedule_impact[]);
//	                                       over-balance → 422 BALANCE_RECHECK_FAILED(requires_override)
//	POST :approve-override               → 200 APPROVED OVERRIDE_APPROVED (OverrideApprovedExample); short reason → 422
//	POST :reject                         → 200 REJECTED; terminal 409; no/short reason → 400 INVALID_REQUEST
//
// Plus the LA-2 no-leader routing serialization (CreatedPendingHRNoLeaderExample).
package leave_test

import (
	"net/http"
	"testing"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
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
	h.seedRequest("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 3)
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
	for _, k := range []string{"id", "employee_id", "leave_type_id", "start_date", "end_date", "duration_days", "status", "routing", "balance_check", "timeline"} {
		if _, ok := row[k]; !ok {
			t.Errorf("leave request row missing key: %s", k)
		}
	}
}

func TestListLeaveRequests_HasMoreCursor(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	// distinct created_at so the keyset ordering is deterministic.
	a := h.seedRequest("SWP-LR-8001", cmpLed, empAgent, dom.LeaveStatusPendingL1, leaveStart, leaveEnd, 1)
	a.CreatedAt = ymd(2026, time.June, 3)
	h.leave.requests[a.ID] = a
	b := h.seedRequest("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 1)
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
	h.seedRequest("SWP-LR-8001", cmpLed, empAgent, dom.LeaveStatusPendingL1, leaveStart, leaveEnd, 1)
	h.seedRequest("SWP-LR-8004", cmpOther, "SWP-EMP-2891", dom.LeaveStatusPendingL1, leaveStart, leaveEnd, 1)

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
// GET /leave-requests/{id} — full detail + timeline; cross-scope hidden as 404
// ---------------------------------------------------------------------------

func TestGetLeaveRequest_FullShapeWithTimeline(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedRequest("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 3)
	// an L1-approved decision row → timeline must carry it + the implicit HR-pending marker.
	h.leave.approvals["SWP-LR-8002"] = []dom.LeaveApproval{{
		LeaveRequestID: "SWP-LR-8002", Stage: dom.StageL1, Decision: dom.DecisionApproved,
		ActorID: strp(empLeader), ActorRole: strp("shift_leader"), OccurredAt: ymd(2026, time.June, 3),
	}}

	rr := h.do("GET", "/leave-requests/SWP-LR-8002", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "PENDING_HR" {
		t.Errorf("status = %v, want PENDING_HR", d["status"])
	}
	tl, ok := d["timeline"].([]any)
	if !ok || len(tl) < 2 {
		t.Fatalf("timeline = %v, want >=2 entries (L1 APPROVED + HR PENDING)", d["timeline"])
	}
	first := tl[0].(map[string]any)
	if first["stage"] != "L1" || first["status"] != "APPROVED" {
		t.Errorf("timeline[0] = %v, want {L1, APPROVED}", first)
	}
	last := tl[len(tl)-1].(map[string]any)
	if last["stage"] != "HR" || last["status"] != "PENDING" {
		t.Errorf("timeline[last] = %v, want {HR, PENDING}", last)
	}
}

func TestGetLeaveRequest_CrossScope404(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedRequest("SWP-LR-8004", cmpOther, "SWP-EMP-2891", dom.LeaveStatusPendingL1, leaveStart, leaveEnd, 1)
	rr := h.do("GET", "/leave-requests/SWP-LR-8004", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "NOT_FOUND" {
		t.Errorf("code = %s, want NOT_FOUND", got)
	}
}

// ---------------------------------------------------------------------------
// POST :approve-l1 — PENDING_L1 → PENDING_HR; 409 wrong-state; 403 scope/self
// ---------------------------------------------------------------------------

func TestApproveL1_LeaderForwardsToHR(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedRequest("SWP-LR-8001", cmpLed, empAgent, dom.LeaveStatusPendingL1, leaveStart, leaveEnd, 1)

	rr := h.do("POST", "/leave-requests/SWP-LR-8001:approve-l1", map[string]any{"note": "Coverage aman."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "PENDING_HR" {
		t.Errorf("status = %v, want PENDING_HR (ApprovedL1Example)", d["status"])
	}
	tl := d["timeline"].([]any)
	var sawL1Approved bool
	for _, e := range tl {
		m := e.(map[string]any)
		if m["stage"] == "L1" && m["status"] == "APPROVED" {
			sawL1Approved = true
		}
	}
	if !sawL1Approved {
		t.Errorf("timeline has no L1/APPROVED entry: %v", tl)
	}
}

func TestApproveL1_WrongState409(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedRequest("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 1)
	rr := h.do("POST", "/leave-requests/SWP-LR-8002:approve-l1", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "CONFLICT" {
		t.Errorf("code = %s, want CONFLICT", got)
	}
	if f := errFields(t, rr); f["status"] != "PENDING_HR" {
		t.Errorf("fields.status = %v, want PENDING_HR", f["status"])
	}
}

func TestApproveL1_CrossCompany403(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedRequest("SWP-LR-8004", cmpOther, "SWP-EMP-2891", dom.LeaveStatusPendingL1, leaveStart, leaveEnd, 1)
	rr := h.do("POST", "/leave-requests/SWP-LR-8004:approve-l1", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "OUT_OF_SCOPE" {
		t.Errorf("code = %s, want OUT_OF_SCOPE (ErrorOutOfScopeExample)", got)
	}
}

func TestApproveL1_SelfApprove403(t *testing.T) {
	// The acting leader IS the request's employee → LA-6 self-approve guard.
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedRequest("SWP-LR-8001", cmpLed, empLeader, dom.LeaveStatusPendingL1, leaveStart, leaveEnd, 1)
	rr := h.do("POST", "/leave-requests/SWP-LR-8001:approve-l1", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "FORBIDDEN" {
		t.Errorf("code = %s, want FORBIDDEN (ErrorSelfApproveExample)", got)
	}
}

// ---------------------------------------------------------------------------
// POST :approve-final — PENDING_HR → APPROVED; INV-3 fires; over-balance 422
// ---------------------------------------------------------------------------

func TestApproveFinal_DeductsAndFiresINV3(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedLeaveType(leaveAnn, "ANNUAL", true)
	// per-type window: entitled 12, used 4 → remaining 8 >= 3 (commit target).
	h.seedWindow("SWP-LQ-8001", empAgent, 12, 4, 0)
	h.seedRequest("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 3)
	// the loop-closer returns one cancelled schedule entry → schedule_impact[].
	h.schedule.cancelReturns[empAgent] = []svc.ScheduleImpact{
		{ScheduleID: "SWP-SCH-6002", Date: leaveStart, NewStatus: "CANCELLED_BY_LEAVE"},
	}

	rr := h.do("POST", "/leave-requests/SWP-LR-8002:approve-final", map[string]any{"note": "OK."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "APPROVED" {
		t.Errorf("status = %v, want APPROVED (ApprovedFinalExample)", d["status"])
	}
	// per-type window used grows by the duration (4 + 3).
	if w := h.meterStore.windowFor(empAgent, leaveAnn, "2026"); w == nil || w.UsedDays != 7 {
		t.Errorf("window used = %v, want 7 (4+3)", w)
	}
	// INV-3 fired: Cancel called once + 3 approved_leave_days inserted (one per day).
	if len(h.schedule.cancelCalls) != 1 {
		t.Errorf("CancelScheduleEntriesForLeave calls = %d, want 1", len(h.schedule.cancelCalls))
	}
	if len(h.schedule.insertedDays) != 3 {
		t.Errorf("approved_leave_days inserts = %d, want 3 (Jun 15-17)", len(h.schedule.insertedDays))
	}
	// schedule_impact[] surfaced with the DB→DTO new_status mapping (CANCELLED_BY_LEAVE → LEAVE).
	si, ok := d["schedule_impact"].([]any)
	if !ok || len(si) != 1 {
		t.Fatalf("schedule_impact = %v, want 1 entry", d["schedule_impact"])
	}
	imp := si[0].(map[string]any)
	if imp["new_status"] != "LEAVE" {
		t.Errorf("schedule_impact[0].new_status = %v, want LEAVE", imp["new_status"])
	}
	if imp["schedule_id"] != "SWP-SCH-6002" {
		t.Errorf("schedule_impact[0].schedule_id = %v, want SWP-SCH-6002", imp["schedule_id"])
	}
}

func TestApproveFinal_OverBalance422QuotaExceeded(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedLeaveType(leaveAnn, "ANNUAL", true)
	// per-type window: entitled 12, used 11 → remaining 1 < 3 (block target).
	h.seedWindow("SWP-LQ-8002", empAgent, 12, 11, 0)
	h.seedRequest("SWP-LR-8003", cmpLed, empAgent, dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 3)

	rr := h.do("POST", "/leave-requests/SWP-LR-8003:approve-final", nil)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	// Per-type meter blocks an over-cap commit with QUOTA_EXCEEDED (LA-5; override CTA).
	if got := errCode(t, rr); got != "QUOTA_EXCEEDED" {
		t.Fatalf("code = %s, want QUOTA_EXCEEDED", got)
	}
	// no state change, no commit on a blocked approval (never negative).
	if w := h.meterStore.windowFor(empAgent, leaveAnn, "2026"); w == nil || w.UsedDays != 11 {
		t.Errorf("window used = %v, want 11 (no commit on block)", w)
	}
	if req := h.leave.requests["SWP-LR-8003"]; req.Status != dom.LeaveStatusPendingHR {
		t.Errorf("status = %s, want PENDING_HR (unchanged)", req.Status)
	}
	if len(h.schedule.cancelCalls) != 0 {
		t.Errorf("INV-3 fired on a blocked approval")
	}
}

// ---------------------------------------------------------------------------
// POST :approve-override — over-balance force-approve; OVERRIDE_APPROVED timeline
// ---------------------------------------------------------------------------

func TestApproveOverride_ForceApprovesOverBalance(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedLeaveType(leaveAnn, "ANNUAL", true)
	// per-type window: entitled 12, used 11 → remaining 1 < 3 (override target).
	h.seedWindow("SWP-LQ-8002", empAgent, 12, 11, 0)
	h.seedRequest("SWP-LR-8003", cmpLed, empAgent, dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 3)
	h.schedule.cancelReturns[empAgent] = []svc.ScheduleImpact{
		{ScheduleID: "SWP-SCH-6002", Date: leaveStart, NewStatus: "CANCELLED_BY_LEAVE"},
	}

	rr := h.do("POST", "/leave-requests/SWP-LR-8003:approve-override", map[string]any{
		"override_reason": "Direstui Direktur Operasional retroaktif via grant Q3.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "APPROVED" {
		t.Errorf("status = %v, want APPROVED (OverrideApprovedExample)", d["status"])
	}
	// the HR-stage timeline entry is OVERRIDE_APPROVED with override=true.
	tl := d["timeline"].([]any)
	var sawOverride bool
	for _, e := range tl {
		m := e.(map[string]any)
		if m["stage"] == "HR" && m["status"] == "OVERRIDE_APPROVED" {
			sawOverride = true
			if ov, _ := m["override"].(bool); !ov {
				t.Errorf("HR timeline override flag = %v, want true", m["override"])
			}
		}
	}
	if !sawOverride {
		t.Errorf("timeline has no HR/OVERRIDE_APPROVED entry: %v", tl)
	}
	// Over-balance override force-commits the full duration onto the window
	// (entitled 12, used 11 + 3 = 14; the override is the audited authorization).
	if w := h.meterStore.windowFor(empAgent, leaveAnn, "2026"); w == nil || w.UsedDays != 14 {
		t.Errorf("window used = %v, want 14 (11+3 forced)", w)
	}
	if len(h.schedule.insertedDays) != 3 {
		t.Errorf("INV-3 approved_leave_days inserts = %d, want 3", len(h.schedule.insertedDays))
	}
}

func TestApproveOverride_ShortReason422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedRequest("SWP-LR-8003", cmpLed, empAgent, dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 3)
	rr := h.do("POST", "/leave-requests/SWP-LR-8003:approve-override", map[string]any{"override_reason": "short"})
	if rr.Code != http.StatusUnprocessableEntity && rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400/422, got %d: %s", rr.Code, rr.Body.String())
	}
	f := errFields(t, rr)
	if _, ok := f["override_reason"]; !ok {
		t.Errorf("fields.override_reason missing on a short-reason reject: %v", f)
	}
}

// ---------------------------------------------------------------------------
// POST :reject — PENDING_* → REJECTED; terminal 409; no/short reason 400
// ---------------------------------------------------------------------------

func TestReject_Happy(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedRequest("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 3)
	rr := h.do("POST", "/leave-requests/SWP-LR-8002:reject", map[string]any{"reason": "Coverage tidak mencukupi periode ini."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if dataObject(t, rr)["status"] != "REJECTED" {
		t.Errorf("status = %v, want REJECTED", dataObject(t, rr)["status"])
	}
}

func TestReject_Terminal409(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedRequest("SWP-LR-8005", cmpLed, empAgent, dom.LeaveStatusApproved, leaveStart, leaveEnd, 3)
	rr := h.do("POST", "/leave-requests/SWP-LR-8005:reject", map[string]any{"reason": "terlambat ditolak."})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "CONFLICT" {
		t.Errorf("code = %s, want CONFLICT", got)
	}
}

func TestReject_NoReason400(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedRequest("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 3)
	rr := h.do("POST", "/leave-requests/SWP-LR-8002:reject", map[string]any{"reason": "no"})
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 400/422, got %d: %s", rr.Code, rr.Body.String())
	}
	if f := errFields(t, rr); f["reason"] == nil {
		t.Errorf("fields.reason missing on a short-reason reject: %v", f)
	}
}

// ---------------------------------------------------------------------------
// LA-2 no-leader routing serialization (CreatedPendingHRNoLeaderExample shape)
// ---------------------------------------------------------------------------

func TestGetLeaveRequest_NoLeaderRoutingSerialized(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	req := h.seedRequest("SWP-LR-8003", cmpOther, "SWP-EMP-2891", dom.LeaveStatusPendingHR, leaveStart, leaveEnd, 3)
	req.Routing.NoLeader = true
	h.leave.requests[req.ID] = req

	rr := h.do("GET", "/leave-requests/SWP-LR-8003", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	routing, ok := d["routing"].(map[string]any)
	if !ok {
		t.Fatalf("routing missing/not an object: %v", d["routing"])
	}
	if nl, _ := routing["no_leader"].(bool); !nl {
		t.Errorf("routing.no_leader = %v, want true (LA-2 collapsed PENDING_HR)", routing["no_leader"])
	}
	// no-leader → the timeline first marker is the HR stage (no L1 entry).
	tl := d["timeline"].([]any)
	if len(tl) == 0 {
		t.Fatalf("timeline empty for a PENDING_HR no-leader request")
	}
	if first := tl[0].(map[string]any); first["stage"] != "HR" {
		t.Errorf("no-leader timeline[0].stage = %v, want HR (collapsed; L1 absent)", first["stage"])
	}
}
