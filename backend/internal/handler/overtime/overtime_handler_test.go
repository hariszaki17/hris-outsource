// Package overtime_test — E7 overtime workflow (F7.1 / OVT-01) contract tests.
//
// The drift gate for the 9 overtime endpoints, asserted byte-for-shape against
// docs/api/E7-overtime/openapi.yaml:
//
//	GET  /overtime              → 200 {data,next_cursor,has_more}; leader-scope filter
//	GET  /overtime/{id}         → 200 {data} full Overtime incl calculation + approvals;
//	                              required-nullable attendance_id serializes JSON null
//	POST :confirm               → PENDING_AGENT_CONFIRM → PENDING_L1; wrong-state 409
//	POST :approve-l1            → PENDING_L1 → PENDING_HR (leader); 409; OUT_OF_SCOPE 403;
//	                              SELF_APPROVAL_FORBIDDEN 403
//	POST :approve-final         → PENDING_HR → APPROVED (HR); 409; OVERRIDE_REASON_REQUIRED 422
//	POST :reject                → PENDING_* → REJECTED; short reason 400
//	POST :withdraw              → PENDING_L1 → 204; APPROVED → 409
//	POST :bulk-approve/:reject  → {succeeded, failed} partial success (see the OT_BELOW_MIN file)
package overtime_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	schedulingsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// Persona / fixture constants mirror the 09-02 seed.
const (
	cmpLed    = "SWP-CMP-0021" // the leader's led company
	cmpOther  = "SWP-CMP-0022" // a company the leader does NOT lead
	empLeader = "SWP-EMP-1108" // Rudi — the shift-leader persona (PL-5001)
	empAgent  = "SWP-EMP-3001" // an agent at the led company
	empHR     = "SWP-EMP-9001" // the HR/super persona's employee id
)

// ---------------------------------------------------------------------------
// GET /overtime — list envelope + leader scope filtering
// ---------------------------------------------------------------------------

func TestListOvertime_Envelope(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	h.seedOvertime("SWP-OT-30007", cmpLed, empAgent, dom.OvertimeStatusApproved, dom.OvertimeTierWorkday)

	rr := h.do("GET", "/overtime?company_id="+cmpLed, nil)
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
	// Spot-check the Overtime shape on the first row (openapi Overtime).
	row := data[0].(map[string]any)
	for _, k := range []string{"id", "employee", "company", "placement_id", "work_date", "source", "status", "tier_indicator", "flagged_no_preapproval", "calculation", "created_at", "updated_at"} {
		if _, ok := row[k]; !ok {
			t.Errorf("overtime row missing key: %s", k)
		}
	}
	// list omits approvals[] per openapi.
	if _, present := row["approvals"]; present {
		t.Errorf("list row should omit approvals[]; got %v", row["approvals"])
	}
	// tier_indicator equals the day_type.
	if row["tier_indicator"] != "WORKDAY" {
		t.Errorf("tier_indicator = %v, want WORKDAY", row["tier_indicator"])
	}
}

func TestListOvertime_HasMoreCursor(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	a := h.seedOvertime("SWP-OT-30001", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	a.CreatedAt = ymd(2026, 6, 3)
	h.overtime.records[a.ID] = a
	b := h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingHR, dom.OvertimeTierWorkday)
	b.CreatedAt = ymd(2026, 6, 2)
	h.overtime.records[b.ID] = b
	c := h.seedOvertime("SWP-OT-30003", cmpLed, empAgent, dom.OvertimeStatusApproved, dom.OvertimeTierWorkday)
	c.CreatedAt = ymd(2026, 6, 1)
	h.overtime.records[c.ID] = c

	rr := h.do("GET", "/overtime?company_id="+cmpLed+"&limit=2", nil)
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
	rr2 := h.do("GET", "/overtime?company_id="+cmpLed+"&limit=2&cursor="+cur, nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("page 2: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	data2 := decodeBody(t, rr2)["data"].([]any)
	if len(data2) != 1 {
		t.Errorf("page 2 data length = %d, want 1 (the remaining row)", len(data2))
	}
}

func TestListOvertime_LeaderScopeForced(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	h.seedOvertime("SWP-OT-30005", cmpOther, "SWP-EMP-2891", dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)

	rr := h.do("GET", "/overtime", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("leader saw %d rows, want 1 (own company only; cross-company absent)", len(data))
	}
	if data[0].(map[string]any)["id"] != "SWP-OT-30002" {
		t.Errorf("leader saw wrong row: %v", data[0])
	}
}

func TestListOvertime_LeaderCrossCompany403(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	rr := h.do("GET", "/overtime?company_id="+cmpOther, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "OUT_OF_SCOPE" {
		t.Errorf("code = %s, want OUT_OF_SCOPE", got)
	}
}

// ---------------------------------------------------------------------------
// GET /overtime/{id} — full detail + calculation + approvals; nullable JSON null
// ---------------------------------------------------------------------------

func TestGetOvertime_FullShapeWithCalculation(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedRule("", 30)
	rec := h.seedOvertime("SWP-OT-30003", cmpLed, empAgent, dom.OvertimeStatusPendingHR, dom.OvertimeTierWorkday)
	// attendance_id stays nil for a REQUESTED row → must serialize as JSON null.
	rec.AttendanceID = nil
	h.overtime.records[rec.ID] = rec
	// an L1-approved decision row → approvals[] timeline present on detail.
	h.overtime.approvals["SWP-OT-30003"] = []dom.OvertimeApproval{{
		Level: 1, Decision: "APPROVED", ApproverID: strp(empLeader), DecidedAt: ymd(2026, 6, 3),
	}}

	rr := h.do("GET", "/overtime/SWP-OT-30003", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "PENDING_HR" {
		t.Errorf("status = %v, want PENDING_HR", d["status"])
	}
	// required-nullable attendance_id serializes as explicit JSON null (present key, nil value).
	if v, present := d["attendance_id"]; !present || v != nil {
		t.Errorf("attendance_id = %v (present=%v), want present-and-null", v, present)
	}
	// calculation block with tier_breakdown.
	calc, ok := d["calculation"].(map[string]any)
	if !ok {
		t.Fatalf("calculation missing/not an object: %v", d["calculation"])
	}
	for _, k := range []string{"worked_minutes", "counted_minutes", "min_minutes_threshold", "skipped_too_short", "tier_breakdown"} {
		if _, ok := calc[k]; !ok {
			t.Errorf("calculation missing key: %s", k)
		}
	}
	tb, ok := calc["tier_breakdown"].([]any)
	if !ok || len(tb) != 1 {
		t.Fatalf("tier_breakdown = %v, want 1 entry", calc["tier_breakdown"])
	}
	entry := tb[0].(map[string]any)
	if entry["tier"] != "WORKDAY" {
		t.Errorf("tier_breakdown[0].tier = %v, want WORKDAY", entry["tier"])
	}
	// supersedes is required-nullable → present-and-null on the single effective tier.
	if v, present := entry["supersedes"]; !present || v != nil {
		t.Errorf("tier_breakdown[0].supersedes = %v (present=%v), want present-and-null", v, present)
	}
	// reference multiplier surfaced from the rule (WORKDAY → weekday rate 1.5) — INV-2 reference only.
	if mult, _ := entry["multiplier"].(float64); mult != 1.5 {
		t.Errorf("tier_breakdown[0].multiplier = %v, want 1.5 (weekday reference rate)", entry["multiplier"])
	}
	// approvals[] timeline present on detail.
	ap, ok := d["approvals"].([]any)
	if !ok || len(ap) != 1 {
		t.Fatalf("approvals = %v, want 1 entry on detail", d["approvals"])
	}
}

func TestGetOvertime_CrossScope404(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30005", cmpOther, "SWP-EMP-2891", dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	rr := h.do("GET", "/overtime/SWP-OT-30005", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "NOT_FOUND" {
		t.Errorf("code = %s, want NOT_FOUND", got)
	}
}

// ---------------------------------------------------------------------------
// :confirm → PENDING_AGENT_CONFIRM → PENDING_L1; wrong-state 409
// ---------------------------------------------------------------------------

func TestConfirm_AgentConfirmForwardsToL1(t *testing.T) {
	// Web confirm is staff-triggered on a seeded auto-detected candidate (CONTEXT).
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30001", cmpLed, empAgent, dom.OvertimeStatusPendingAgentConfirm, dom.OvertimeTierWorkday)

	rr := h.do("POST", "/overtime/SWP-OT-30001:confirm", map[string]any{"note": "Konfirmasi lembur."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if dataObject(t, rr)["status"] != "PENDING_L1" {
		t.Errorf("status = %v, want PENDING_L1", dataObject(t, rr)["status"])
	}
}

func TestConfirm_WrongState409(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30002:confirm", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "CONFLICT" {
		t.Errorf("code = %s, want CONFLICT", got)
	}
	if f := errFields(t, rr); f["status"] != "PENDING_L1" {
		t.Errorf("fields.status = %v, want PENDING_L1", f["status"])
	}
}

// ---------------------------------------------------------------------------
// :approve-l1 → PENDING_L1 → PENDING_HR; 409 wrong-state; 403 scope/self
// ---------------------------------------------------------------------------

func TestApproveL1_LeaderForwardsToHR(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)

	rr := h.do("POST", "/overtime/SWP-OT-30002:approve-l1", map[string]any{"note": "Coverage aman."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "PENDING_HR" {
		t.Errorf("status = %v, want PENDING_HR", d["status"])
	}
	// a level-1 approval row landed in the trail.
	ap := d["approvals"].([]any)
	var sawL1 bool
	for _, e := range ap {
		m := e.(map[string]any)
		if int(m["level"].(float64)) == 1 && m["decision"] == "APPROVED" {
			sawL1 = true
		}
	}
	if !sawL1 {
		t.Errorf("approvals has no level-1 APPROVED row: %v", ap)
	}
}

func TestApproveL1_WrongState409(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30003", cmpLed, empAgent, dom.OvertimeStatusPendingHR, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30003:approve-l1", nil)
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

func TestApproveL1_CrossCompany403OutOfScope(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30005", cmpOther, "SWP-EMP-2891", dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30005:approve-l1", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "OUT_OF_SCOPE" {
		t.Errorf("code = %s, want OUT_OF_SCOPE", got)
	}
}

func TestApproveL1_SelfApprove403Forbidden(t *testing.T) {
	// The acting leader IS the OT's employee → OA-5 self-approve guard.
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30004", cmpLed, empLeader, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30004:approve-l1", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "SELF_APPROVAL_FORBIDDEN" {
		t.Errorf("code = %s, want SELF_APPROVAL_FORBIDDEN", got)
	}
}

// ---------------------------------------------------------------------------
// :approve-final → PENDING_HR → APPROVED (HR); 409; OVERRIDE_REASON_REQUIRED 422
// ---------------------------------------------------------------------------

func TestApproveFinal_HRApproves(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30003", cmpLed, empAgent, dom.OvertimeStatusPendingHR, dom.OvertimeTierWorkday)

	rr := h.do("POST", "/overtime/SWP-OT-30003:approve-final", map[string]any{"note": "OK."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "APPROVED" {
		t.Errorf("status = %v, want APPROVED", d["status"])
	}
	// a level-2 (HR final) approval row landed.
	ap := d["approvals"].([]any)
	var sawL2 bool
	for _, e := range ap {
		m := e.(map[string]any)
		if int(m["level"].(float64)) == 2 && m["decision"] == "APPROVED" {
			sawL2 = true
		}
	}
	if !sawL2 {
		t.Errorf("approvals has no level-2 APPROVED row: %v", ap)
	}
}

func TestApproveFinal_WrongStateNonOverride409(t *testing.T) {
	// :approve-final on a PENDING_L1 record without override → 409.
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30002:approve-final", map[string]any{"note": "OK."})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "CONFLICT" {
		t.Errorf("code = %s, want CONFLICT", got)
	}
	if f := errFields(t, rr); f["status"] != "PENDING_L1" {
		t.Errorf("fields.status = %v, want PENDING_L1", f["status"])
	}
}

func TestApproveFinal_OverrideRequiresReason422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30002:approve-final", map[string]any{"is_override": true})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "OVERRIDE_REASON_REQUIRED" {
		t.Errorf("code = %s, want OVERRIDE_REASON_REQUIRED", got)
	}
	if f := errFields(t, rr); f["note"] == nil {
		t.Errorf("fields.note missing on an override-without-reason: %v", f)
	}
}

func TestApproveFinal_OverrideBypassesL1(t *testing.T) {
	// is_override allows HR to approve a PENDING_L1 directly (OA-8) with a note.
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30002:approve-final", map[string]any{
		"is_override": true, "note": "Disetujui langsung oleh HR — leader berhalangan.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "APPROVED" {
		t.Errorf("status = %v, want APPROVED", d["status"])
	}
	var sawOverride bool
	for _, e := range d["approvals"].([]any) {
		m := e.(map[string]any)
		if int(m["level"].(float64)) == 2 && m["decision"] == "OVERRIDE_APPROVED" {
			sawOverride = true
		}
	}
	if !sawOverride {
		t.Errorf("approvals has no level-2 OVERRIDE_APPROVED row: %v", d["approvals"])
	}
}

// ---------------------------------------------------------------------------
// :reject → PENDING_* → REJECTED; short reason 400
// ---------------------------------------------------------------------------

func TestReject_Happy(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30003", cmpLed, empAgent, dom.OvertimeStatusPendingHR, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30003:reject", map[string]any{"reason": "Tidak ada pre-approval valid."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if dataObject(t, rr)["status"] != "REJECTED" {
		t.Errorf("status = %v, want REJECTED", dataObject(t, rr)["status"])
	}
}

func TestReject_ShortReason400(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30003", cmpLed, empAgent, dom.OvertimeStatusPendingHR, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30003:reject", map[string]any{"reason": "no"})
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 400/422, got %d: %s", rr.Code, rr.Body.String())
	}
	if f := errFields(t, rr); f["reason"] == nil {
		t.Errorf("fields.reason missing on a short-reason reject: %v", f)
	}
}

// ---------------------------------------------------------------------------
// :withdraw → PENDING_L1 → 204; APPROVED → 409
// ---------------------------------------------------------------------------

func TestWithdraw_FromPendingL1_204(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30002:withdraw", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() != 0 {
		t.Errorf("204 body should be empty, got %q", rr.Body.String())
	}
	// state actually advanced to WITHDRAWN.
	if got := h.overtime.records["SWP-OT-30002"].Status; got != dom.OvertimeStatusWithdrawn {
		t.Errorf("status = %s, want WITHDRAWN", got)
	}
}

func TestWithdraw_FromApproved_409(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30007", cmpLed, empAgent, dom.OvertimeStatusApproved, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30007:withdraw", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "CONFLICT" {
		t.Errorf("code = %s, want CONFLICT", got)
	}
}

// ---------------------------------------------------------------------------
// OT_BELOW_MIN — counted_minutes < rule.min_minutes → 422 + field errors.
//
// The openapi returns OT_BELOW_MIN on the create path (mobile/system, OUT of web
// scope); E7 exposes it as the exported EnforceMinMinutes seam. We drive the REAL
// service method directly + assert the apperr wire shape (422, code, fields).
// ---------------------------------------------------------------------------

func TestEnforceMinMinutes_BelowMin422WithFields(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedRule("", 30) // global default min_minutes = 30
	line := "SWP-SVC-001"

	err := h.otSvc.EnforceMinMinutes(context.Background(), 20, &line)
	if err == nil {
		t.Fatalf("expected OT_BELOW_MIN error for counted 20 < min 30, got nil")
	}
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected *apperr.Error, got %T: %v", err, err)
	}
	if ae.Status() != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", ae.Status())
	}
	if ae.Code != "OT_BELOW_MIN" {
		t.Errorf("code = %s, want OT_BELOW_MIN", ae.Code)
	}
	// error.fields carries counted_minutes + min_minutes (INV-5).
	if ae.Fields["counted_minutes"] != "20" {
		t.Errorf("fields.counted_minutes = %q, want \"20\"", ae.Fields["counted_minutes"])
	}
	if ae.Fields["min_minutes"] != "30" {
		t.Errorf("fields.min_minutes = %q, want \"30\"", ae.Fields["min_minutes"])
	}
}

func TestEnforceMinMinutes_AtOrAboveMin_OK(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedRule("", 30)
	line := "SWP-SVC-001"
	if err := h.otSvc.EnforceMinMinutes(context.Background(), 30, &line); err != nil {
		t.Errorf("counted 30 >= min 30 should pass, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ClassifyDayType — schedule + holiday calendar with HOLIDAY>RESTDAY>WORKDAY.
// ---------------------------------------------------------------------------

func TestClassifyDayType_HolidayOverridesSchedule(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	workDate := ymd(2026, time.August, 17)
	// even with a live shift (WORKDAY), a holiday on the date wins (precedence).
	h.schedule.live[liveKey(empAgent, workDate)] = schedulingsvc.LiveEntry{ID: "SWP-SCH-6001", Status: "PUBLISHED", IsDayOff: false}
	h.seedHoliday("SWP-HOL-9001", "Hari Kemerdekaan", workDate, dom.HolidayCategoryNational, 0)

	tier, holidayID := h.otSvc.ClassifyDayType(context.Background(), empAgent, workDate, nil)
	if tier != dom.OvertimeTierHoliday {
		t.Errorf("tier = %s, want HOLIDAY (precedence over WORKDAY)", tier)
	}
	if holidayID == nil || *holidayID != "SWP-HOL-9001" {
		t.Errorf("holidayID = %v, want SWP-HOL-9001", holidayID)
	}
}

func TestClassifyDayType_NoScheduleNoHoliday_Restday(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	workDate := ymd(2026, time.August, 18)
	tier, holidayID := h.otSvc.ClassifyDayType(context.Background(), empAgent, workDate, nil)
	if tier != dom.OvertimeTierRestday {
		t.Errorf("tier = %s, want RESTDAY (no live shift, no holiday)", tier)
	}
	if holidayID != nil {
		t.Errorf("holidayID = %v, want nil on a non-holiday", holidayID)
	}
}

// ---------------------------------------------------------------------------
// :bulk-approve — leader → L1 dispatch; partial success {succeeded, failed}.
//
// ids = [in-scope PENDING_L1, the leader's OWN OT, a CMP-0022 OT, a terminal
// APPROVED] → succeeded carries the in-scope id; failed[] carries the self
// (SELF_APPROVAL_FORBIDDEN), cross-company (OUT_OF_SCOPE), terminal (CONFLICT).
// ---------------------------------------------------------------------------

func TestBulkApprove_LeaderPartialSuccess(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)         // in-scope OK
	h.seedOvertime("SWP-OT-30004", cmpLed, empLeader, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday)        // leader's own → SELF
	h.seedOvertime("SWP-OT-30005", cmpOther, "SWP-EMP-2891", dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday) // cross-company → OUT_OF_SCOPE
	h.seedOvertime("SWP-OT-30007", cmpLed, empAgent, dom.OvertimeStatusApproved, dom.OvertimeTierWorkday)          // terminal → CONFLICT

	rr := h.do("POST", "/overtime:bulk-approve", map[string]any{
		"ids": []string{"SWP-OT-30002", "SWP-OT-30004", "SWP-OT-30005", "SWP-OT-30007"},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (>=1 succeeded), got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	succeeded, _ := body["succeeded"].([]any)
	failed, _ := body["failed"].([]any)
	if len(succeeded) != 1 || succeeded[0] != "SWP-OT-30002" {
		t.Errorf("succeeded = %v, want [SWP-OT-30002]", succeeded)
	}
	if len(failed) != 3 {
		t.Fatalf("failed = %d, want 3 (self + cross-company + terminal)", len(failed))
	}
	// each failed[] row carries {id, error.code}.
	codeByID := map[string]string{}
	for _, raw := range failed {
		row := raw.(map[string]any)
		errObj := row["error"].(map[string]any)
		codeByID[row["id"].(string)] = errObj["code"].(string)
	}
	if codeByID["SWP-OT-30004"] != "SELF_APPROVAL_FORBIDDEN" {
		t.Errorf("own OT code = %s, want SELF_APPROVAL_FORBIDDEN", codeByID["SWP-OT-30004"])
	}
	if codeByID["SWP-OT-30005"] != "OUT_OF_SCOPE" {
		t.Errorf("cross-company code = %s, want OUT_OF_SCOPE", codeByID["SWP-OT-30005"])
	}
	if codeByID["SWP-OT-30007"] != "CONFLICT" {
		t.Errorf("terminal code = %s, want CONFLICT", codeByID["SWP-OT-30007"])
	}
}

func TestBulkApprove_AllFailed422(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30007", cmpLed, empAgent, dom.OvertimeStatusApproved, dom.OvertimeTierWorkday) // terminal
	h.seedOvertime("SWP-OT-30008", cmpLed, empAgent, dom.OvertimeStatusRejected, dom.OvertimeTierWorkday) // terminal

	rr := h.do("POST", "/overtime:bulk-approve", map[string]any{
		"ids": []string{"SWP-OT-30007", "SWP-OT-30008"},
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 (all failed), got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if s, _ := body["succeeded"].([]any); len(s) != 0 {
		t.Errorf("succeeded = %v, want empty", s)
	}
	if f, _ := body["failed"].([]any); len(f) != 2 {
		t.Errorf("failed = %d, want 2", len(f))
	}
}

// ---------------------------------------------------------------------------
// :bulk-reject — mix of succeeded + a terminal failure; envelope shape.
// ---------------------------------------------------------------------------

func TestBulkReject_LeaderPartialSuccess(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPendingL1, dom.OvertimeTierWorkday) // OK to reject
	h.seedOvertime("SWP-OT-30007", cmpLed, empAgent, dom.OvertimeStatusApproved, dom.OvertimeTierWorkday)  // terminal → CONFLICT

	rr := h.do("POST", "/overtime:bulk-reject", map[string]any{
		"ids":    []string{"SWP-OT-30002", "SWP-OT-30007"},
		"reason": "Tidak ada pre-approval untuk lembur ini.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	succeeded, _ := body["succeeded"].([]any)
	failed, _ := body["failed"].([]any)
	if len(succeeded) != 1 || succeeded[0] != "SWP-OT-30002" {
		t.Errorf("succeeded = %v, want [SWP-OT-30002]", succeeded)
	}
	if len(failed) != 1 {
		t.Fatalf("failed = %d, want 1 (terminal)", len(failed))
	}
	fr := failed[0].(map[string]any)
	if fr["id"] != "SWP-OT-30007" {
		t.Errorf("failed[0].id = %v, want SWP-OT-30007", fr["id"])
	}
	if fr["error"].(map[string]any)["code"] != "CONFLICT" {
		t.Errorf("failed[0].error.code = %v, want CONFLICT", fr["error"].(map[string]any)["code"])
	}
}

func TestBulkReject_AllFailed422(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30007", cmpLed, empAgent, dom.OvertimeStatusApproved, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime:bulk-reject", map[string]any{
		"ids":    []string{"SWP-OT-30007"},
		"reason": "Penolakan terlambat untuk lembur ini.",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 (all failed), got %d: %s", rr.Code, rr.Body.String())
	}
}
