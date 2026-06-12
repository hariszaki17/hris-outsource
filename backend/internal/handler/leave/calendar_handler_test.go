// Package leave_test — E6 leave-calendar (F6.3 / LVE-03) contract test.
//
// The drift gate for GET /leave-calendar, asserted byte-for-shape against
// docs/api/E6-leave/openapi.yaml (CalendarHRJuneExample):
//
//	GET /leave-calendar → 200 {period, month, show_pending, entries[]}
//	  - show_pending=false → only APPROVED entries
//	  - show_pending=true  → entries include PENDING_L1 / PENDING_HR
//	  - shift_leader with a cross-company company_id → 403 OUT_OF_SCOPE
package leave_test

import (
	"net/http"
	"testing"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// ---------------------------------------------------------------------------
// GET /leave-calendar — response shape + show_pending toggle
// ---------------------------------------------------------------------------

func TestCalendar_ShapeAndApprovedOnly(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedCalendarEntry("SWP-LR-8005", cmpLed, empAgent, dom.LeaveStatusApproved, ymd(2026, time.June, 15), ymd(2026, time.June, 17))
	h.seedCalendarEntry("SWP-LR-8002", cmpLed, empAgent, dom.LeaveStatusPendingHR, ymd(2026, time.June, 20), ymd(2026, time.June, 22))

	rr := h.do("GET", "/leave-calendar?period=2026&month=6", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	// LeaveCalendarResponse shape.
	for _, k := range []string{"period", "month", "show_pending", "entries"} {
		if _, ok := body[k]; !ok {
			t.Errorf("calendar response missing key: %s", k)
		}
	}
	if int(body["period"].(float64)) != 2026 {
		t.Errorf("period = %v, want 2026", body["period"])
	}
	entries, ok := body["entries"].([]any)
	if !ok {
		t.Fatalf("entries missing/not an array: %v", body["entries"])
	}
	// show_pending defaults false → only the APPROVED entry is returned.
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1 (APPROVED only; show_pending=false)", len(entries))
	}
	e := entries[0].(map[string]any)
	if e["status"] != "APPROVED" {
		t.Errorf("entry status = %v, want APPROVED", e["status"])
	}
	for _, k := range []string{"leave_request_id", "employee_id", "company_id", "leave_type_id", "start_date", "end_date", "status"} {
		if _, ok := e[k]; !ok {
			t.Errorf("calendar entry missing key: %s", k)
		}
	}
}

func TestCalendar_ShowPendingToggle(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "")
	h.seedCalendarEntry("SWP-LR-8005", cmpLed, empAgent, dom.LeaveStatusApproved, ymd(2026, time.June, 15), ymd(2026, time.June, 17))
	h.seedCalendarEntry("SWP-LR-8002", cmpLed, "SWP-EMP-3002", dom.LeaveStatusPendingHR, ymd(2026, time.June, 20), ymd(2026, time.June, 22))
	h.seedCalendarEntry("SWP-LR-8001", cmpLed, "SWP-EMP-3003", dom.LeaveStatusPendingL1, ymd(2026, time.June, 25), ymd(2026, time.June, 26))

	rr := h.do("GET", "/leave-calendar?period=2026&month=6&show_pending=true", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if sp, _ := body["show_pending"].(bool); !sp {
		t.Errorf("show_pending = false, want true")
	}
	entries := body["entries"].([]any)
	// APPROVED + PENDING_HR + PENDING_L1 all included.
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3 (APPROVED + 2 pending)", len(entries))
	}
	seen := map[string]bool{}
	for _, raw := range entries {
		seen[raw.(map[string]any)["status"].(string)] = true
	}
	for _, want := range []string{"APPROVED", "PENDING_HR", "PENDING_L1"} {
		if !seen[want] {
			t.Errorf("show_pending=true missing a %s entry", want)
		}
	}
}

func TestCalendar_LeaderCrossCompany403(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	rr := h.do("GET", "/leave-calendar?period=2026&month=6&company_id="+cmpOther, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := errCode(t, rr); got != "OUT_OF_SCOPE" {
		t.Errorf("code = %s, want OUT_OF_SCOPE", got)
	}
}

func TestCalendar_LeaderScopeForced(t *testing.T) {
	// A leader sees only their led company's entries even if others are seeded.
	h := newHarness(t, auth.RoleShiftLeader, cmpLed, empLeader)
	h.seedCalendarEntry("SWP-LR-8005", cmpLed, empAgent, dom.LeaveStatusApproved, ymd(2026, time.June, 15), ymd(2026, time.June, 17))
	h.seedCalendarEntry("SWP-LR-8009", cmpOther, "SWP-EMP-2891", dom.LeaveStatusApproved, ymd(2026, time.June, 15), ymd(2026, time.June, 17))

	rr := h.do("GET", "/leave-calendar?period=2026&month=6", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	entries := decodeBody(t, rr)["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("leader saw %d entries, want 1 (own company only)", len(entries))
	}
}
