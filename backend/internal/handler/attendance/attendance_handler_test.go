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
	"time"

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

// withSitePosition pins site_id/position (free-text) on a just-seeded record and re-stores it.
func (h *harness) withSitePosition(rec att.Attendance, siteID, position string) {
	rec.SiteID = siteID
	rec.Position = position
	h.attendance.records[rec.ID] = rec
}

func TestListAttendance_SitePositionFilterNarrows(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	a := h.seedAttendance("SWP-ATT-9002", cmpLed, empOther, att.VerificationPending, checkInA, att.FlagLate)
	h.withSitePosition(a, "SWP-SITE-031", "Petugas Kebersihan")
	b := h.seedAttendance("SWP-ATT-9003", cmpLed, empOther, att.VerificationPending, checkInB, att.FlagLate)
	h.withSitePosition(b, "SWP-SITE-099", "Teknisi Gedung")

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
	if _, ok := row["position"]; !ok {
		t.Errorf("row missing position")
	}

	// position (free-text) narrows independently.
	rr2 := h.do("GET", "/attendance?company_id="+cmpLed+"&position=Teknisi+Gedung", nil)
	data2 := decodeBody(t, rr2)["data"].([]any)
	if len(data2) != 1 || data2[0].(map[string]any)["id"] != "SWP-ATT-9003" {
		t.Fatalf("position filter wrong result: %v", data2)
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
// GET /attendance — agent riwayat self-scope + date-range + status filters
// (AR-1 / AR-10 / AR-11). The agent's employee_id is forced to the caller; the
// date basis is COALESCE(shift_start_at, check_in_at) in Asia/Jakarta, inclusive;
// status is single/multi-value IN.
// ---------------------------------------------------------------------------

const empAgentSelf = "SWP-EMP-7001" // the calling agent's own employee id

// newAgentHarness builds a harness whose principal is an agent (self-scope).
func newAgentHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, auth.RoleAgent, "", "")
	h.principal.EmployeeID = empAgentSelf
	return h
}

func TestListAttendance_AgentSelfScope_ForcesOwnRecords(t *testing.T) {
	h := newAgentHarness(t)
	// The agent's own records + a foreign agent's records at the same company.
	h.seedAttendance("SWP-ATT-7001", cmpLed, empAgentSelf, att.VerificationVerified, checkInA, att.FlagLate)
	h.seedAttendance("SWP-ATT-7002", cmpLed, empAgentSelf, att.VerificationVerified, checkInB)
	h.seedAttendance("SWP-ATT-7099", cmpLed, empOther, att.VerificationVerified, checkInC)

	// No employee_id supplied — service forces it to the caller.
	rr := h.do("GET", "/attendance", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("data length = %d, want 2 (own records only)", len(data))
	}
	for _, d := range data {
		if got := d.(map[string]any)["employee_id"]; got != empAgentSelf {
			t.Errorf("returned record employee_id = %v, want own %s", got, empAgentSelf)
		}
	}
}

func TestListAttendance_AgentForeignEmployeeID_OutOfScope(t *testing.T) {
	h := newAgentHarness(t)
	h.seedAttendance("SWP-ATT-7099", cmpLed, empOther, att.VerificationVerified, checkInA)

	// An agent explicitly asking for another employee's records is rejected.
	rr := h.do("GET", "/attendance?employee_id="+empOther, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "OUT_OF_SCOPE" {
		t.Errorf("error code = %q, want OUT_OF_SCOPE", code)
	}
}

func TestListAttendance_AgentOwnEmployeeID_Allowed(t *testing.T) {
	h := newAgentHarness(t)
	h.seedAttendance("SWP-ATT-7001", cmpLed, empAgentSelf, att.VerificationVerified, checkInA)

	// Supplying one's OWN employee_id is fine (matches the forced value).
	rr := h.do("GET", "/attendance?employee_id="+empAgentSelf, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(decodeBody(t, rr)["data"].([]any)) != 1 {
		t.Errorf("want 1 own record")
	}
}

func TestListAttendance_AgentDateRange_InclusiveShiftBasis(t *testing.T) {
	h := newAgentHarness(t)
	// All for the calling agent. Basis = shift_start_at when present, else check_in_at.
	// Shift starts are WIB-local midnights; build via the Jakarta zone so the basis is
	// unambiguous. 5,10,18 Mei in range; 4 Mei and 19 Mei out.
	mk := func(d int) time.Time { return time.Date(2026, time.May, d, 8, 0, 0, 0, jakarta) }
	h.seedAttendanceFull("SWP-ATT-M04", cmpLed, empAgentSelf, att.StatusPresent, att.VerificationVerified, mk(4), mk(4))
	h.seedAttendanceFull("SWP-ATT-M05", cmpLed, empAgentSelf, att.StatusPresent, att.VerificationVerified, mk(5), mk(5))
	h.seedAttendanceFull("SWP-ATT-M10", cmpLed, empAgentSelf, att.StatusLate, att.VerificationVerified, mk(10), mk(10))
	h.seedAttendanceFull("SWP-ATT-M18", cmpLed, empAgentSelf, att.StatusPresent, att.VerificationVerified, mk(18), mk(18))
	h.seedAttendanceFull("SWP-ATT-M19", cmpLed, empAgentSelf, att.StatusPresent, att.VerificationVerified, mk(19), mk(19))

	rr := h.do("GET", "/attendance?date_from=2026-05-05&date_to=2026-05-18", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	got := idSet(decodeBody(t, rr)["data"].([]any))
	want := map[string]bool{"SWP-ATT-M05": true, "SWP-ATT-M10": true, "SWP-ATT-M18": true}
	if !sameSet(got, want) {
		t.Errorf("date-range ids = %v, want %v (5–18 Mei inclusive)", got, want)
	}
}

func TestListAttendance_AgentDateRange_AbsentRowKeptByShiftDate(t *testing.T) {
	h := newAgentHarness(t)
	// An ABSENT row has NULL check_in_at; its shift_start_at must still place it in range
	// (the bug was check_in_at::date only → ABSENT rows silently dropped from any range).
	mk := func(d int) time.Time { return time.Date(2026, time.May, d, 7, 0, 0, 0, jakarta) }
	absent := h.seedAttendanceFull("SWP-ATT-MAB", cmpLed, empAgentSelf, att.StatusAbsent, att.VerificationPending, mk(12), time.Time{})
	absent.CheckInAt = nil
	h.attendance.records[absent.ID] = absent

	rr := h.do("GET", "/attendance?date_from=2026-05-10&date_to=2026-05-15", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	got := idSet(decodeBody(t, rr)["data"].([]any))
	if !got["SWP-ATT-MAB"] {
		t.Errorf("ABSENT row missing from range; ids = %v (shift_start basis must keep it)", got)
	}
}

func TestListAttendance_AgentStatusFilter_SingleAndMulti(t *testing.T) {
	h := newAgentHarness(t)
	mk := func(d int) time.Time { return time.Date(2026, time.May, d, 8, 0, 0, 0, jakarta) }
	h.seedAttendanceFull("SWP-ATT-P", cmpLed, empAgentSelf, att.StatusPresent, att.VerificationVerified, mk(2), mk(2))
	h.seedAttendanceFull("SWP-ATT-L", cmpLed, empAgentSelf, att.StatusLate, att.VerificationVerified, mk(3), mk(3))
	h.seedAttendanceFull("SWP-ATT-A", cmpLed, empAgentSelf, att.StatusAbsent, att.VerificationPending, mk(4), time.Time{})

	// Single-select (mobile sends one value).
	rr := h.do("GET", "/attendance?status=LATE", nil)
	got := idSet(decodeBody(t, rr)["data"].([]any))
	if !sameSet(got, map[string]bool{"SWP-ATT-L": true}) {
		t.Errorf("status=LATE ids = %v, want {SWP-ATT-L}", got)
	}

	// Multi-value (?status=LATE,ABSENT → IN clause).
	rr = h.do("GET", "/attendance?status=LATE,ABSENT", nil)
	got = idSet(decodeBody(t, rr)["data"].([]any))
	if !sameSet(got, map[string]bool{"SWP-ATT-L": true, "SWP-ATT-A": true}) {
		t.Errorf("status=LATE,ABSENT ids = %v, want {SWP-ATT-L,SWP-ATT-A}", got)
	}
}

func TestListAttendance_AgentDateAndStatus_Combined(t *testing.T) {
	h := newAgentHarness(t)
	mk := func(d int) time.Time { return time.Date(2026, time.May, d, 8, 0, 0, 0, jakarta) }
	h.seedAttendanceFull("SWP-ATT-C1", cmpLed, empAgentSelf, att.StatusLate, att.VerificationVerified, mk(6), mk(6))   // in range, LATE
	h.seedAttendanceFull("SWP-ATT-C2", cmpLed, empAgentSelf, att.StatusLate, att.VerificationVerified, mk(20), mk(20)) // LATE but out of range
	h.seedAttendanceFull("SWP-ATT-C3", cmpLed, empAgentSelf, att.StatusPresent, att.VerificationVerified, mk(7), mk(7)) // in range but PRESENT

	rr := h.do("GET", "/attendance?date_from=2026-05-05&date_to=2026-05-18&status=LATE", nil)
	got := idSet(decodeBody(t, rr)["data"].([]any))
	if !sameSet(got, map[string]bool{"SWP-ATT-C1": true}) {
		t.Errorf("combined filter ids = %v, want {SWP-ATT-C1}", got)
	}
}

// idSet collects the ids from a JSON data array.
func idSet(data []any) map[string]bool {
	out := map[string]bool{}
	for _, d := range data {
		out[d.(map[string]any)["id"].(string)] = true
	}
	return out
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
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

// ---------------------------------------------------------------------------
// GET /attendance:manual-autofill (F5.6) — resolve placement + schedule
// ---------------------------------------------------------------------------

func TestManualAutofill_Success(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.attendance.seedSchedule("SWP-EMP-1002", fixedNow, fixedNow.Add(8*time.Hour))
	rr := h.do("GET", "/attendance:manual-autofill?employee_id=SWP-EMP-1002&date=2026-06-04", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing: %T", body["data"])
	}
	for _, k := range []string{"employee_name", "company_name", "site_name", "position"} {
		if _, ok := data[k]; !ok {
			t.Errorf("response missing key: %s", k)
		}
	}
	if data["schedule_id"] == nil {
		t.Errorf("expected schedule_id to be non-nil, got nil")
	}
}

func TestManualAutofill_NoPlacement_404(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	rr := h.do("GET", "/attendance:manual-autofill?employee_id="+empNoPlacement+"&date=2026-06-04", nil)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "NO_ACTIVE_PLACEMENT" {
		t.Errorf("error.code = %q, want NO_ACTIVE_PLACEMENT", code)
	}
}

func TestManualAutofill_MissingParams_400(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	rr := h.do("GET", "/attendance:manual-autofill?employee_id=SWP-EMP-1002", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "INVALID_REQUEST" {
		t.Errorf("error.code = %q, want INVALID_REQUEST", code)
	}
}

// ---------------------------------------------------------------------------
// POST /attendance:manual-create (F5.6) — HR/SL creates attendance manually
// ---------------------------------------------------------------------------

func TestManualCreate_Success(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-0001")
	ci := fixedNow.Format(time.RFC3339)

	rr := h.do("POST", "/attendance:manual-create", map[string]any{
		"employee_id": empOther,
		"check_in_at": ci,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing/not an object: %T", body["data"])
	}
	for _, k := range []string{"id", "employee_id", "placement_id", "company_id", "check_in_at", "verification_status", "flags"} {
		if _, ok := data[k]; !ok {
			t.Errorf("response missing key: %s", k)
		}
	}
	if data["employee_id"] != empOther {
		t.Errorf("employee_id = %v, want %s", data["employee_id"], empOther)
	}
	if data["verification_status"] != "PENDING" {
		t.Errorf("verification_status = %v, want PENDING", data["verification_status"])
	}
	flags, ok := data["flags"].([]any)
	if !ok {
		t.Fatalf("flags missing/not array: %T", data["flags"])
	}
	hasManual := false
	for _, f := range flags {
		if f == "MANUAL_ENTRY" {
			hasManual = true
			break
		}
	}
	if !hasManual {
		t.Errorf("flags missing MANUAL_ENTRY: %v", flags)
	}
	// Verify created_by is set from the principal.
	if data["created_by"] != "SWP-EMP-0001" {
		t.Errorf("created_by = %v, want SWP-EMP-0001", data["created_by"])
	}
}

func TestManualCreate_NoActivePlacement_422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	ci := fixedNow.Format(time.RFC3339)

	rr := h.do("POST", "/attendance:manual-create", map[string]any{
		"employee_id": empNoPlacement,
		"check_in_at": ci,
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "NO_ACTIVE_PLACEMENT" {
		t.Errorf("error.code = %q, want NO_ACTIVE_PLACEMENT", code)
	}
}

func TestManualCreate_InvalidCheckIn_400(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")

	rr := h.do("POST", "/attendance:manual-create", map[string]any{
		"employee_id": empOther,
		"check_in_at": "invaliddate",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "INVALID_REQUEST" {
		t.Errorf("error.code = %q, want INVALID_REQUEST", code)
	}
}

func TestManualCreate_CheckOutBeforeCheckIn_422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	ci := fixedNow.Format(time.RFC3339)
	co := fixedNow.Add(-30 * time.Minute).Format(time.RFC3339)

	rr := h.do("POST", "/attendance:manual-create", map[string]any{
		"employee_id":  empOther,
		"check_in_at":  ci,
		"check_out_at": co,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if code := strOf(errObject(t, body)["code"]); code != "INVALID_REQUEST" {
		t.Errorf("error.code = %q, want INVALID_REQUEST", code)
	}
	e := errObject(t, body)
	fields, ok := e["fields"].(map[string]any)
	if !ok || fields["check_out_at"] == nil {
		t.Errorf("error.fields.check_out_at missing: %v", e["fields"])
	}
}

func TestManualCreate_MissingEmployeeID_400(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")

	rr := h.do("POST", "/attendance:manual-create", map[string]any{
		"check_in_at": fixedNow.Format(time.RFC3339),
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "INVALID_REQUEST" {
		t.Errorf("error.code = %q, want INVALID_REQUEST", code)
	}
}

func TestManualCreate_ShiftLeaderOutOfScope_422(t *testing.T) {
	// SL for cmpOther tries to create attendance for an empOther whose placement
	// is at cmpLed → OUT_OF_SCOPE.
	h := newHarness(t, auth.RoleShiftLeader, cmpOther, empLeader)
	ci := fixedNow.Format(time.RFC3339)

	rr := h.do("POST", "/attendance:manual-create", map[string]any{
		"employee_id": empOther,
		"check_in_at": ci,
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "OUT_OF_SCOPE" {
		t.Errorf("error.code = %q, want OUT_OF_SCOPE", code)
	}
}

func TestManualCreate_ShiftLeaderInScope_Success(t *testing.T) {
	// SL for cmpLed creates attendance for empOther who is at cmpLed → success.
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	ci := fixedNow.Format(time.RFC3339)

	rr := h.do("POST", "/attendance:manual-create", map[string]any{
		"employee_id": empOther,
		"check_in_at": ci,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing/not an object: %T", body["data"])
	}
	if data["verification_status"] != "PENDING" {
		t.Errorf("verification_status = %v, want PENDING", data["verification_status"])
	}
}
