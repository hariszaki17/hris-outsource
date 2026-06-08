// Package attendance_test — attendance verification (F5.3 / ATT-01) contract tests.
//
// The drift gate for the 6 attendance endpoints, asserted byte-for-shape against
// docs/api/E5-attendance/openapi.yaml:
//
//	GET /attendance            → 200 {data,next_cursor,has_more}; leader-scope; OUT_OF_SCOPE 403
//	GET /attendance/{id}       → 200 {data}; cross-scope → 404 (hide existence)
//	POST :verify               → 200 {data}; VERIFY_OWN_RECORD 403; OUT_OF_SCOPE 403; terminal CONFLICT 409
//	POST :reject               → 200 {data}; missing/short reason → 400 INVALID_REQUEST; terminal 409
//	POST :bulk-verify          → 200 {succeeded,failed} (partial) / 422 (all failed)
//	POST :bulk-reject          → 200 partial / 422 all-failed
//	idempotency replay         → same key+body replays; same key+different body → 409 IDEMPOTENCY_KEY_REUSED
package attendance_test

import (
	"net/http"
	"testing"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// Persona constants mirror the 07-02 seed fixtures.
const (
	cmpLed    = "SWP-CMP-0021" // the leader's led company
	cmpOther  = "SWP-CMP-0022" // a company the leader does NOT lead
	empLeader = "SWP-EMP-1108" // Rudi — the shift-leader persona (own-record target)
	empOther  = "SWP-EMP-2891" // an agent at the led company
)

var (
	checkInA = ymd(2026, 6, 3) // newest
	checkInB = ymd(2026, 6, 2)
	checkInC = ymd(2026, 6, 1) // oldest
)

// ---------------------------------------------------------------------------
// GET /attendance — list envelope + leader scope + OUT_OF_SCOPE
// ---------------------------------------------------------------------------

func TestListAttendance_Envelope(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)
	h.seedAttendance("SWP-ATT-9003", cmpLed, empOther, att.VerificationPending, checkInB, att.FlagOutsideGeofence)

	rr := h.do("GET", "/attendance?company_id="+cmpLed, nil)
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
	// next_cursor key must be present (null when no more), has_more must be a bool.
	if _, present := body["next_cursor"]; !present {
		t.Errorf("next_cursor key missing from envelope")
	}
	if _, ok := body["has_more"].(bool); !ok {
		t.Errorf("has_more missing/not a bool: %T", body["has_more"])
	}
	// Spot-check the Attendance shape on the first row.
	row := data[0].(map[string]any)
	for _, k := range []string{"id", "employee_id", "placement_id", "company_id", "check_in_at", "verification_status", "flags"} {
		if _, ok := row[k]; !ok {
			t.Errorf("attendance row missing key: %s", k)
		}
	}
}

func TestListAttendance_HasMore_Cursor(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9001", cmpLed, empOther, att.VerificationPending, checkInA)
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInB)
	h.seedAttendance("SWP-ATT-9003", cmpLed, empOther, att.VerificationPending, checkInC)

	rr := h.do("GET", "/attendance?company_id="+cmpLed+"&limit=2", nil)
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
	// Page 2 via the opaque cursor returns the remaining row.
	rr2 := h.do("GET", "/attendance?company_id="+cmpLed+"&limit=2&cursor="+cur, nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("page 2: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	body2 := decodeBody(t, rr2)
	data2 := body2["data"].([]any)
	if len(data2) != 1 {
		t.Errorf("page 2 data length = %d, want 1 (the remaining row)", len(data2))
	}
	if hm, _ := body2["has_more"].(bool); hm {
		t.Errorf("page 2 has_more = true, want false")
	}
}

func TestListAttendance_LeaderScopeForced(t *testing.T) {
	// Leader sees only their led company even with rows seeded elsewhere.
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA)
	h.seedAttendance("SWP-ATT-9005", cmpOther, empOther, att.VerificationPending, checkInB)

	rr := h.do("GET", "/attendance", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("leader saw %d rows, want 1 (own company only)", len(data))
	}
	if got := data[0].(map[string]any)["company_id"]; got != cmpLed {
		t.Errorf("row company_id = %v, want %s", got, cmpLed)
	}
}

func TestListAttendance_LeaderCrossCompany_OutOfScope(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)

	rr := h.do("GET", "/attendance?company_id="+cmpOther, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "OUT_OF_SCOPE" {
		t.Errorf("error.code = %q, want OUT_OF_SCOPE", code)
	}
}

// withSitePosition pins site_id/position_id on a just-seeded record and re-stores it.
func (h *harness) withSitePosition(rec att.Attendance, siteID, positionID string) {
	rec.SiteID = siteID
	rec.PositionID = positionID
	h.attendance.records[rec.ID] = rec
}

func TestListAttendance_SitePositionFilterNarrows(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	a := h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)
	h.withSitePosition(a, "SWP-SITE-031", "SWP-POS-009")
	b := h.seedAttendance("SWP-ATT-9003", cmpLed, empOther, att.VerificationPending, checkInB, att.FlagLate)
	h.withSitePosition(b, "SWP-SITE-099", "SWP-POS-077")

	// site_id narrows to the matching record only.
	rr := h.do("GET", "/attendance?company_id="+cmpLed+"&site_id=SWP-SITE-031", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("site_id filter returned %d rows, want 1", len(data))
	}
	row := data[0].(map[string]any)
	if row["id"] != "SWP-ATT-9002" {
		t.Errorf("row id = %v, want SWP-ATT-9002", row["id"])
	}
	if row["site_id"] != "SWP-SITE-031" {
		t.Errorf("row site_id = %v, want SWP-SITE-031", row["site_id"])
	}
	if _, ok := row["position_id"]; !ok {
		t.Errorf("row missing position_id")
	}

	// position_id narrows independently.
	rr2 := h.do("GET", "/attendance?company_id="+cmpLed+"&position_id=SWP-POS-077", nil)
	data2 := decodeBody(t, rr2)["data"].([]any)
	if len(data2) != 1 || data2[0].(map[string]any)["id"] != "SWP-ATT-9003" {
		t.Fatalf("position_id filter wrong result: %v", data2)
	}
}

func TestListAttendance_LeaderScope_SiteCannotWiden(t *testing.T) {
	// A leader is pinned to their led company; site_id only narrows WITHIN it and
	// can never surface another company's record.
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	led := h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)
	h.withSitePosition(led, "SWP-SITE-031", "SWP-POS-009")
	// A record at ANOTHER company that happens to share the same site_id value.
	other := h.seedAttendance("SWP-ATT-9005", cmpOther, empOther, att.VerificationPending, checkInB, att.FlagLate)
	h.withSitePosition(other, "SWP-SITE-031", "SWP-POS-009")

	rr := h.do("GET", "/attendance?site_id=SWP-SITE-031", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("leader saw %d rows, want 1 (site must not widen beyond led company)", len(data))
	}
	if got := data[0].(map[string]any)["company_id"]; got != cmpLed {
		t.Errorf("row company_id = %v, want %s (led company)", got, cmpLed)
	}

	// Supplying another company explicitly is still OUT_OF_SCOPE even with site_id.
	rr2 := h.do("GET", "/attendance?company_id="+cmpOther+"&site_id=SWP-SITE-031", nil)
	if rr2.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr2.Code, rr2.Body.String())
	}
	if code := errCode(t, rr2); code != "OUT_OF_SCOPE" {
		t.Errorf("error.code = %q, want OUT_OF_SCOPE", code)
	}
}

func TestListAttendance_AbsentRow_NullCheckInAt(t *testing.T) {
	// A true ABSENT record (no clock-in) serializes check_in_at as JSON null
	// (present key, null value) — not absent, not a zero timestamp.
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	rec := h.seedAttendance("SWP-ATT-9009", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagAbsent)
	rec.CheckInAt = nil
	rec.LatIn = nil
	rec.LngIn = nil
	rec.Status = att.StatusAbsent
	h.attendance.records[rec.ID] = rec

	rr := h.do("GET", "/attendance?company_id="+cmpLed, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data length = %d, want 1", len(data))
	}
	row := data[0].(map[string]any)
	ci, present := row["check_in_at"]
	if !present {
		t.Fatalf("check_in_at key absent; must be present and null for an ABSENT row")
	}
	if ci != nil {
		t.Errorf("check_in_at = %v, want null", ci)
	}
	if row["status"] != "ABSENT" {
		t.Errorf("status = %v, want ABSENT", row["status"])
	}
}

// ---------------------------------------------------------------------------
// GET /attendance/{id} — 200 {data} + cross-scope 404
// ---------------------------------------------------------------------------

func TestGetAttendance_Success(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)

	rr := h.do("GET", "/attendance/SWP-ATT-9002", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data, ok := decodeBody(t, rr)["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing/not an object")
	}
	if data["id"] != "SWP-ATT-9002" {
		t.Errorf("data.id = %v, want SWP-ATT-9002", data["id"])
	}
	if data["verification_status"] != "PENDING" {
		t.Errorf("data.verification_status = %v, want PENDING", data["verification_status"])
	}
}

func TestGetAttendance_CrossScope_404(t *testing.T) {
	// Leader requests a record at a company they don't lead → 404 (hide existence).
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedAttendance("SWP-ATT-9005", cmpOther, empOther, att.VerificationPending, checkInA)

	rr := h.do("GET", "/attendance/SWP-ATT-9005", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "NOT_FOUND" {
		t.Errorf("error.code = %q, want NOT_FOUND", code)
	}
}

// ---------------------------------------------------------------------------
// POST /attendance/{id}:verify
// ---------------------------------------------------------------------------

func TestVerifyAttendance_Success(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)

	rr := h.do("POST", "/attendance/SWP-ATT-9002:verify", map[string]any{"note": "ok"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].(map[string]any)
	if data["verification_status"] != "VERIFIED" {
		t.Errorf("verification_status = %v, want VERIFIED", data["verification_status"])
	}
}

func TestVerifyAttendance_VerifyOwnRecord_403(t *testing.T) {
	// Leader principal whose employee id matches the (escalated) record → 403.
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedAttendance("SWP-ATT-9006", cmpLed, empLeader, att.VerificationEscalated, checkInA, att.FlagEscalated)

	rr := h.do("POST", "/attendance/SWP-ATT-9006:verify", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "VERIFY_OWN_RECORD" {
		t.Errorf("error.code = %q, want VERIFY_OWN_RECORD", code)
	}
}

func TestVerifyAttendance_OutOfScope_403(t *testing.T) {
	// Leader verifying a record at a company they don't lead → 403 OUT_OF_SCOPE.
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedAttendance("SWP-ATT-9005", cmpOther, empOther, att.VerificationPending, checkInA)

	rr := h.do("POST", "/attendance/SWP-ATT-9005:verify", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "OUT_OF_SCOPE" {
		t.Errorf("error.code = %q, want OUT_OF_SCOPE", code)
	}
}

func TestVerifyAttendance_Terminal_Conflict_409(t *testing.T) {
	// Already-VERIFIED record → 409 CONFLICT with fields.verification_status.
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9001", cmpLed, empOther, att.VerificationVerified, checkInA)

	rr := h.do("POST", "/attendance/SWP-ATT-9001:verify", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "CONFLICT" {
		t.Fatalf("error.code = %v, want CONFLICT", e["code"])
	}
	fields, ok := e["fields"].(map[string]any)
	if !ok {
		t.Fatalf("error.fields missing: %T", e["fields"])
	}
	if fields["verification_status"] != "VERIFIED" {
		t.Errorf("fields.verification_status = %v, want VERIFIED", fields["verification_status"])
	}
}

// ---------------------------------------------------------------------------
// POST /attendance/{id}:reject
// ---------------------------------------------------------------------------

func TestRejectAttendance_Success(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)

	rr := h.do("POST", "/attendance/SWP-ATT-9002:reject", map[string]any{"reason": "Jam clock-in tidak sesuai."})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].(map[string]any)
	if data["verification_status"] != "REJECTED" {
		t.Errorf("verification_status = %v, want REJECTED", data["verification_status"])
	}
	if data["reject_reason"] != "Jam clock-in tidak sesuai." {
		t.Errorf("reject_reason = %v, want the supplied reason", data["reject_reason"])
	}
}

func TestRejectAttendance_MissingReason_400(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA)

	rr := h.do("POST", "/attendance/SWP-ATT-9002:reject", map[string]any{"reason": "x"})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "INVALID_REQUEST" {
		t.Fatalf("error.code = %v, want INVALID_REQUEST", e["code"])
	}
	fields, ok := e["fields"].(map[string]any)
	if !ok || fields["reason"] == nil {
		t.Errorf("error.fields.reason missing: %v", e["fields"])
	}
}

func TestRejectAttendance_Terminal_Conflict_409(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9001", cmpLed, empOther, att.VerificationRejected, checkInA)

	rr := h.do("POST", "/attendance/SWP-ATT-9001:reject", map[string]any{"reason": "Sudah diputuskan."})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "CONFLICT" {
		t.Errorf("error.code = %q, want CONFLICT", code)
	}
}

// ---------------------------------------------------------------------------
// POST /attendance:bulk-verify / :bulk-reject — partial success + all-failed
// ---------------------------------------------------------------------------

func TestBulkVerify_PartialSuccess_200(t *testing.T) {
	// Leader: one valid PENDING record + their own ESCALATED record.
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)
	h.seedAttendance("SWP-ATT-9006", cmpLed, empLeader, att.VerificationEscalated, checkInB, att.FlagEscalated)

	rr := h.do("POST", "/attendance:bulk-verify", map[string]any{
		"ids":  []string{"SWP-ATT-9002", "SWP-ATT-9006"},
		"note": "Konfirmasi massal.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (>=1 succeeded), got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	succeeded, _ := body["succeeded"].([]any)
	failed, _ := body["failed"].([]any)
	if len(succeeded) != 1 || succeeded[0] != "SWP-ATT-9002" {
		t.Errorf("succeeded = %v, want [SWP-ATT-9002]", succeeded)
	}
	if len(failed) != 1 {
		t.Fatalf("failed length = %d, want 1", len(failed))
	}
	f0 := failed[0].(map[string]any)
	if f0["id"] != "SWP-ATT-9006" {
		t.Errorf("failed[0].id = %v, want SWP-ATT-9006", f0["id"])
	}
	ferr, ok := f0["error"].(map[string]any)
	if !ok {
		t.Fatalf("failed[0].error missing: %T", f0["error"])
	}
	if ferr["code"] != "VERIFY_OWN_RECORD" {
		t.Errorf("failed[0].error.code = %v, want VERIFY_OWN_RECORD", ferr["code"])
	}
	if _, ok := ferr["message"].(string); !ok {
		t.Errorf("failed[0].error.message missing/not a string: %T", ferr["message"])
	}
}

func TestBulkVerify_AllFailed_422(t *testing.T) {
	// Leader: only their own ESCALATED record → every row fails → 422.
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedAttendance("SWP-ATT-9006", cmpLed, empLeader, att.VerificationEscalated, checkInA, att.FlagEscalated)

	rr := h.do("POST", "/attendance:bulk-verify", map[string]any{
		"ids": []string{"SWP-ATT-9006"},
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 (all failed), got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	succeeded, _ := body["succeeded"].([]any)
	failed, _ := body["failed"].([]any)
	if len(succeeded) != 0 {
		t.Errorf("succeeded = %d, want 0", len(succeeded))
	}
	if len(failed) != 1 {
		t.Errorf("failed = %d, want 1", len(failed))
	}
}

func TestBulkReject_PartialSuccess_200(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)
	h.seedAttendance("SWP-ATT-9006", cmpLed, empLeader, att.VerificationEscalated, checkInB, att.FlagEscalated)

	rr := h.do("POST", "/attendance:bulk-reject", map[string]any{
		"ids":    []string{"SWP-ATT-9002", "SWP-ATT-9006"},
		"reason": "Penolakan massal — bukti kurang.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	succeeded, _ := body["succeeded"].([]any)
	failed, _ := body["failed"].([]any)
	if len(succeeded) != 1 || succeeded[0] != "SWP-ATT-9002" {
		t.Errorf("succeeded = %v, want [SWP-ATT-9002]", succeeded)
	}
	if len(failed) != 1 {
		t.Fatalf("failed length = %d, want 1", len(failed))
	}
	ferr := failed[0].(map[string]any)["error"].(map[string]any)
	if ferr["code"] != "VERIFY_OWN_RECORD" {
		t.Errorf("failed[0].error.code = %v, want VERIFY_OWN_RECORD", ferr["code"])
	}
}

func TestBulkReject_AllFailed_422(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedAttendance("SWP-ATT-9006", cmpLed, empLeader, att.VerificationEscalated, checkInA, att.FlagEscalated)

	rr := h.do("POST", "/attendance:bulk-reject", map[string]any{
		"ids":    []string{"SWP-ATT-9006"},
		"reason": "Penolakan — tidak valid.",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Idempotency — replay seam (router-level middleware)
// ---------------------------------------------------------------------------

func TestVerify_Idempotency_ReplaySameKey(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)

	hdr := map[string]string{"Idempotency-Key": "00000000-0000-0000-0000-000000000001"}
	body := map[string]any{"note": "ok"}

	rr1 := h.doWithHeaders("POST", "/attendance/SWP-ATT-9002:verify", body, hdr)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first call: expected 200, got %d: %s", rr1.Code, rr1.Body.String())
	}
	first := rr1.Body.String()

	// Second identical call replays the SAME body/status (even though the record
	// is now VERIFIED — without replay it would 409).
	rr2 := h.doWithHeaders("POST", "/attendance/SWP-ATT-9002:verify", body, hdr)
	if rr2.Code != http.StatusOK {
		t.Fatalf("replay: expected 200 (replayed), got %d: %s", rr2.Code, rr2.Body.String())
	}
	if rr2.Body.String() != first {
		t.Errorf("replay body differs:\n first=%s\nsecond=%s", first, rr2.Body.String())
	}
	if rr2.Header().Get("Idempotent-Replayed") != "true" {
		t.Errorf("replay missing Idempotent-Replayed header")
	}
}

func TestVerify_Idempotency_KeyReuse_409(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)
	h.seedAttendance("SWP-ATT-9003", cmpLed, empOther, att.VerificationPending, checkInB, att.FlagLate)

	hdr := map[string]string{"Idempotency-Key": "00000000-0000-0000-0000-000000000002"}

	rr1 := h.doWithHeaders("POST", "/attendance/SWP-ATT-9002:verify", map[string]any{"note": "a"}, hdr)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first call: expected 200, got %d: %s", rr1.Code, rr1.Body.String())
	}
	// Same key, DIFFERENT body → 409 IDEMPOTENCY_KEY_REUSED.
	rr2 := h.doWithHeaders("POST", "/attendance/SWP-ATT-9002:verify", map[string]any{"note": "different"}, hdr)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("key-reuse: expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}
	if code := errCode(t, rr2); code != "IDEMPOTENCY_KEY_REUSED" {
		t.Errorf("error.code = %q, want IDEMPOTENCY_KEY_REUSED", code)
	}
}
