// Package overtime_test — E7 overtime workflow contract tests (post-E11 approval
// migration). The drift gate for the live E7 endpoints, asserted byte-for-shape
// against docs/api/E7-overtime/openapi.yaml:
//
//	GET  /overtime              → 200 {data,next_cursor,has_more}; leader-scope filter
//	GET  /overtime/{id}         → 200 {data} full Overtime incl calculation +
//	                              approval_instance_id; required-nullable
//	                              attendance_id serializes JSON null
//	POST :confirm               → PENDING_AGENT_CONFIRM → PENDING + engine instance;
//	                              wrong-state 409
//	POST :withdraw              → PENDING → 204 (CANCELLED); APPROVED → 409
//
// Approval ROUTING (approve/reject/bulk) moved to the E11 engine and is no longer
// part of the E7 surface; the engine drives the chain and calls OnApproved/
// OnRejected on terminal transition (covered in overtime_service_test.go).
package overtime_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	approval "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	schedulingsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// Persona / fixture constants mirror the seed.
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
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPending, dom.OvertimeTierWorkday)
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
	// tier_indicator equals the day_type.
	if row["tier_indicator"] != "WORKDAY" {
		t.Errorf("tier_indicator = %v, want WORKDAY", row["tier_indicator"])
	}
}

func TestListOvertime_HasMoreCursor(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	a := h.seedOvertime("SWP-OT-30001", cmpLed, empAgent, dom.OvertimeStatusPending, dom.OvertimeTierWorkday)
	a.CreatedAt = ymd(2026, 6, 3)
	h.overtime.records[a.ID] = a
	b := h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPending, dom.OvertimeTierWorkday)
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
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPending, dom.OvertimeTierWorkday)
	h.seedOvertime("SWP-OT-30005", cmpOther, "SWP-EMP-2891", dom.OvertimeStatusPending, dom.OvertimeTierWorkday)

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
// GET /overtime/{id} — full detail + calculation + approval_instance_id; nullable JSON null
// ---------------------------------------------------------------------------

func TestGetOvertime_FullShapeWithCalculation(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedRule("", 30)
	rec := h.seedOvertime("SWP-OT-30003", cmpLed, empAgent, dom.OvertimeStatusPending, dom.OvertimeTierWorkday)
	// attendance_id stays nil for a REQUESTED row → must serialize as JSON null.
	rec.AttendanceID = nil
	// the E11 instance link is present once the OT entered the chain.
	inst := "SWP-API-0001"
	rec.ApprovalInstanceID = &inst
	h.overtime.records[rec.ID] = rec

	rr := h.do("GET", "/overtime/SWP-OT-30003", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "PENDING" {
		t.Errorf("status = %v, want PENDING", d["status"])
	}
	// required-nullable attendance_id serializes as explicit JSON null (present key, nil value).
	if v, present := d["attendance_id"]; !present || v != nil {
		t.Errorf("attendance_id = %v (present=%v), want present-and-null", v, present)
	}
	// the approval chain is OWNED BY E11 — surfaced via approval_instance_id.
	if d["approval_instance_id"] != inst {
		t.Errorf("approval_instance_id = %v, want %s", d["approval_instance_id"], inst)
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
}

func TestGetOvertime_CrossScope404(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedOvertime("SWP-OT-30005", cmpOther, "SWP-EMP-2891", dom.OvertimeStatusPending, dom.OvertimeTierWorkday)
	rr := h.do("GET", "/overtime/SWP-OT-30005", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "NOT_FOUND" {
		t.Errorf("code = %s, want NOT_FOUND", got)
	}
}

// ---------------------------------------------------------------------------
// :confirm → PENDING_AGENT_CONFIRM → PENDING + engine CreateInstance; wrong-state 409
// ---------------------------------------------------------------------------

func TestConfirm_AgentConfirmEntersApprovalChain(t *testing.T) {
	// Web confirm is staff-triggered on a seeded auto-detected candidate (CONTEXT).
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30001", cmpLed, empAgent, dom.OvertimeStatusPendingAgentConfirm, dom.OvertimeTierWorkday)

	rr := h.do("POST", "/overtime/SWP-OT-30001:confirm", map[string]any{"note": "Konfirmasi lembur."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["status"] != "PENDING" {
		t.Errorf("status = %v, want PENDING", d["status"])
	}
	// confirming creates the E11 ApprovalInstance for this OT and links it.
	if len(h.engine.calls) != 1 {
		t.Fatalf("engine.CreateInstance calls = %d, want 1", len(h.engine.calls))
	}
	call := h.engine.calls[0]
	if call.RequestType != approval.RequestTypeOvertime || call.RequestID != "SWP-OT-30001" {
		t.Errorf("CreateInstance input = %+v, want {OVERTIME, SWP-OT-30001}", call)
	}
	if call.RequesterID != empAgent {
		t.Errorf("CreateInstance requester = %s, want %s", call.RequesterID, empAgent)
	}
	if d["approval_instance_id"] == nil || d["approval_instance_id"] == "" {
		t.Errorf("approval_instance_id not linked after confirm: %v", d["approval_instance_id"])
	}
	if got := deref(h.overtime.records["SWP-OT-30001"].ApprovalInstanceID); got == "" {
		t.Errorf("instance id not persisted on the record")
	}
}

func TestConfirm_WrongState409(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPending, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30002:confirm", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "CONFLICT" {
		t.Errorf("code = %s, want CONFLICT", got)
	}
	if f := errFields(t, rr); f["status"] != "PENDING" {
		t.Errorf("fields.status = %v, want PENDING", f["status"])
	}
	// no instance is created on a rejected transition.
	if len(h.engine.calls) != 0 {
		t.Errorf("engine.CreateInstance should not run on a wrong-state confirm: %d calls", len(h.engine.calls))
	}
}

// ---------------------------------------------------------------------------
// :withdraw → PENDING → 204 (CANCELLED); APPROVED → 409
// ---------------------------------------------------------------------------

func TestWithdraw_FromPending_204(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30002", cmpLed, empAgent, dom.OvertimeStatusPending, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30002:withdraw", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() != 0 {
		t.Errorf("204 body should be empty, got %q", rr.Body.String())
	}
	// state advanced to CANCELLED (collapsed from WITHDRAWN).
	if got := h.overtime.records["SWP-OT-30002"].Status; got != dom.OvertimeStatusCancelled {
		t.Errorf("status = %s, want CANCELLED", got)
	}
}

func TestWithdraw_FromPendingAgentConfirm_204(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", empHR)
	h.seedOvertime("SWP-OT-30009", cmpLed, empAgent, dom.OvertimeStatusPendingAgentConfirm, dom.OvertimeTierWorkday)
	rr := h.do("POST", "/overtime/SWP-OT-30009:withdraw", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := h.overtime.records["SWP-OT-30009"].Status; got != dom.OvertimeStatusCancelled {
		t.Errorf("status = %s, want CANCELLED", got)
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
	h.seedRule("", 30) // single GLOBAL rule, min_minutes = 30

	err := h.otSvc.EnforceMinMinutes(context.Background(), 20)
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
	if err := h.otSvc.EnforceMinMinutes(context.Background(), 30); err != nil {
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

	tier, holidayID := h.otSvc.ClassifyDayType(context.Background(), empAgent, workDate)
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
	tier, holidayID := h.otSvc.ClassifyDayType(context.Background(), empAgent, workDate)
	if tier != dom.OvertimeTierRestday {
		t.Errorf("tier = %s, want RESTDAY (no live shift, no holiday)", tier)
	}
	if holidayID != nil {
		t.Errorf("holidayID = %v, want nil on a non-holiday", holidayID)
	}
}
