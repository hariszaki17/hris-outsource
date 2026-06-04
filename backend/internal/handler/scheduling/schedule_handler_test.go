// Package scheduling_test — schedule (F4.2/F4.3/F4.4 / SA-*) contract tests.
//
// The drift gate for every E4 conflict code + the bulk-apply partial-success
// envelope + the leader-scope 403, asserted against docs/api/E4-shift-scheduling/
// openapi.yaml EXACTLY:
//
//	OUT_OF_SCOPE 403 · OUTSIDE_PLACEMENT_PERIOD 422 · SHIFT_DEACTIVATED 422
//	SHIFT_NOT_FOR_SERVICE_LINE 422 · SHIFT_OVER_LEAVE 409 · DOUBLE_SHIFT 409
//
// plus force_replace (201 MODIFIED + replaced_entry_id), :check no-side-effect,
// bulk-apply 200/422 + weekdays_mask, list envelope {data,warnings}, DELETE 204 /
// leader-past-date 403.
package scheduling_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// Fixture dates anchored on the fixed clock (2026-06-04 = a Thursday).
// All "in-window" dates sit inside [placementStart, placementEnd].
var (
	placementStart = ymd(2026, 1, 1)
	placementEndV  = ymd(2026, 12, 31)
	placementEnd   = &placementEndV

	dateInWindow = ymd(2026, 6, 8)  // Monday — clearly inside the window
	dateLeaveDay = ymd(2026, 6, 9)  // Tuesday — used for SHIFT_OVER_LEAVE
	dateOccupied = ymd(2026, 6, 10) // Wednesday — used for DOUBLE_SHIFT
	dateOutside  = ymd(2030, 1, 1)  // far outside any placement window
)

// seedHappyAgent wires an agent (EMP) with an active placement at CMP on the
// generic service line, plus the SWP-SHF-001 "Pagi" master (untagged, active).
func seedHappyAgent(h *harness, empID, companyID string) {
	h.seedPlacement(empID, "SWP-PL-5001", companyID, "SWP-SVC-001", placementStart, placementEnd)
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)
}

func createSingleBody(empID, shiftID, date string, forceReplace bool) map[string]any {
	return map[string]any{
		"kind":            "single",
		"employee_id":     empID,
		"shift_master_id": shiftID,
		"date":            date,
		"force_replace":   forceReplace,
	}
}

// ---------------------------------------------------------------------------
// Happy path + list envelope
// ---------------------------------------------------------------------------

func TestCreateScheduleEntry_Success(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-1108", "SWP-CMP-0021")

	rr := h.do("POST", "/schedule", createSingleBody("SWP-EMP-1108", "SWP-SHF-001", dateStr(dateInWindow), false))
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "SCHEDULED" {
		t.Errorf("status = %v, want SCHEDULED", body["status"])
	}
	if body["work_date"] != dateStr(dateInWindow) {
		t.Errorf("work_date = %v, want %s", body["work_date"], dateStr(dateInWindow))
	}
	if body["placement_id"] != "SWP-PL-5001" {
		t.Errorf("placement_id = %v, want SWP-PL-5001", body["placement_id"])
	}
	if body["company_id"] != "SWP-CMP-0021" {
		t.Errorf("company_id = %v, want SWP-CMP-0021 (derived from placement)", body["company_id"])
	}
	if _, ok := body["warnings"].([]any); !ok {
		t.Errorf("warnings missing/not an array: %T", body["warnings"])
	}
}

func TestListSchedule_Envelope(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-1108", "SWP-CMP-0021")
	h.seedLiveEntry("SWP-SCH-6001", "SWP-EMP-1108", "SWP-CMP-0021", dateInWindow, "Pagi")

	rr := h.do("GET", "/schedule?company_id=SWP-CMP-0021&start_date=2026-06-01&end_date=2026-06-30", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if _, ok := body["data"].([]any); !ok {
		t.Errorf("data missing/not an array: %T", body["data"])
	}
	if _, ok := body["warnings"].([]any); !ok {
		t.Errorf("warnings missing/not an array (must be present even if empty): %T", body["warnings"])
	}
}

// ---------------------------------------------------------------------------
// Conflict codes — one sub-test per code (status + error.code + details)
// ---------------------------------------------------------------------------

func TestConflict_OutsidePlacementPeriod(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-1108", "SWP-CMP-0021")

	// Date far outside the placement window → no active placement covers it.
	rr := h.do("POST", "/schedule", createSingleBody("SWP-EMP-1108", "SWP-SHF-001", dateStr(dateOutside), false))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "OUTSIDE_PLACEMENT_PERIOD" {
		t.Errorf("error.code = %q, want OUTSIDE_PLACEMENT_PERIOD", code)
	}
}

func TestConflict_ShiftDeactivated(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedPlacement("SWP-EMP-1108", "SWP-PL-5001", "SWP-CMP-0021", "SWP-SVC-001", placementStart, placementEnd)
	// Master is inactive.
	h.seedMaster("SWP-SHF-OFF", "Mati", "07:00", "15:00", nil, false)

	rr := h.do("POST", "/schedule", createSingleBody("SWP-EMP-1108", "SWP-SHF-OFF", dateStr(dateInWindow), false))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "SHIFT_DEACTIVATED" {
		t.Errorf("error.code = %q, want SHIFT_DEACTIVATED", code)
	}
}

func TestConflict_ShiftNotForServiceLine(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	// Placement is on SVC-001; master is tagged to SVC-003 (Parking).
	h.seedPlacement("SWP-EMP-1108", "SWP-PL-5001", "SWP-CMP-0021", "SWP-SVC-001", placementStart, placementEnd)
	h.seedMaster("SWP-SHF-002", "Malam", "23:00", "07:00", strp("SWP-SVC-003"), true)

	rr := h.do("POST", "/schedule", createSingleBody("SWP-EMP-1108", "SWP-SHF-002", dateStr(dateInWindow), false))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "SHIFT_NOT_FOR_SERVICE_LINE" {
		t.Fatalf("error.code = %v, want SHIFT_NOT_FOR_SERVICE_LINE", e["code"])
	}
	details, ok := e["details"].(map[string]any)
	if !ok {
		t.Fatalf("error.details missing: %T", e["details"])
	}
	if details["placement_service_line_id"] != "SWP-SVC-001" {
		t.Errorf("details.placement_service_line_id = %v, want SWP-SVC-001", details["placement_service_line_id"])
	}
	if details["shift_service_line_id"] != "SWP-SVC-003" {
		t.Errorf("details.shift_service_line_id = %v, want SWP-SVC-003", details["shift_service_line_id"])
	}
}

func TestConflict_ShiftOverLeave(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-3001", "SWP-CMP-0021")
	// Approved leave covers the target date (honest read via approvedLeave map).
	h.seedApprovedLeave("SWP-EMP-3001", dateLeaveDay, "SWP-LR-44210", "ANNUAL")

	rr := h.do("POST", "/schedule", createSingleBody("SWP-EMP-3001", "SWP-SHF-001", dateStr(dateLeaveDay), false))
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "SHIFT_OVER_LEAVE" {
		t.Fatalf("error.code = %v, want SHIFT_OVER_LEAVE", e["code"])
	}
	details, ok := e["details"].(map[string]any)
	if !ok {
		t.Fatalf("error.details missing: %T", e["details"])
	}
	if details["leave_request_id"] != "SWP-LR-44210" {
		t.Errorf("details.leave_request_id = %v, want SWP-LR-44210", details["leave_request_id"])
	}
	if details["leave_type"] != "ANNUAL" {
		t.Errorf("details.leave_type = %v, want ANNUAL", details["leave_type"])
	}
}

func TestConflict_DoubleShift(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-1108", "SWP-CMP-0021")
	// Pre-existing live entry on the target date.
	h.seedLiveEntry("SWP-SCH-6001", "SWP-EMP-1108", "SWP-CMP-0021", dateOccupied, "Pagi")

	rr := h.do("POST", "/schedule", createSingleBody("SWP-EMP-1108", "SWP-SHF-001", dateStr(dateOccupied), false))
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if e["code"] != "DOUBLE_SHIFT" {
		t.Fatalf("error.code = %v, want DOUBLE_SHIFT", e["code"])
	}
	details, ok := e["details"].(map[string]any)
	if !ok {
		t.Fatalf("error.details missing: %T", e["details"])
	}
	if details["existing_entry_id"] != "SWP-SCH-6001" {
		t.Errorf("details.existing_entry_id = %v, want SWP-SCH-6001", details["existing_entry_id"])
	}
	if details["existing_shift_name"] != "Pagi" {
		t.Errorf("details.existing_shift_name = %v, want Pagi", details["existing_shift_name"])
	}
}

func TestForceReplace_Succeeds(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-1108", "SWP-CMP-0021")
	h.seedLiveEntry("SWP-SCH-6001", "SWP-EMP-1108", "SWP-CMP-0021", dateOccupied, "Pagi")

	rr := h.do("POST", "/schedule", createSingleBody("SWP-EMP-1108", "SWP-SHF-001", dateStr(dateOccupied), true))
	if rr.Code != http.StatusCreated {
		t.Fatalf("force_replace: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "MODIFIED" {
		t.Errorf("status = %v, want MODIFIED (replace path)", body["status"])
	}
	if body["replaced_entry_id"] != "SWP-SCH-6001" {
		t.Errorf("replaced_entry_id = %v, want SWP-SCH-6001", body["replaced_entry_id"])
	}
}

func TestScope_OutOfScope(t *testing.T) {
	// Leader scoped to CMP-A; the target agent's placement is at CMP-B.
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-0021")
	h.seedPlacement("SWP-EMP-2891", "SWP-PL-5002", "SWP-CMP-0022", "SWP-SVC-001", placementStart, placementEnd)
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)

	rr := h.do("POST", "/schedule", createSingleBody("SWP-EMP-2891", "SWP-SHF-001", dateStr(dateInWindow), false))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "OUT_OF_SCOPE" {
		t.Errorf("error.code = %q, want OUT_OF_SCOPE", code)
	}
}

// ---------------------------------------------------------------------------
// :check (dry-run) — no persistence
// ---------------------------------------------------------------------------

func TestCheck_SingleNoWrite(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-3001", "SWP-CMP-0021")
	h.seedApprovedLeave("SWP-EMP-3001", dateLeaveDay, "SWP-LR-44210", "ANNUAL")

	before := len(h.schedule.entries)

	rr := h.do("POST", "/schedule:check", map[string]any{
		"kind":            "single",
		"employee_id":     "SWP-EMP-3001",
		"shift_master_id": "SWP-SHF-001",
		"date":            dateStr(dateLeaveDay),
	})
	if rr.Code != http.StatusOK {
		t.Fatalf(":check: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	failed, ok := body["failed"].([]any)
	if !ok || len(failed) == 0 {
		t.Fatalf("failed not a non-empty array: %T %v", body["failed"], body["failed"])
	}
	f0 := failed[0].(map[string]any)
	ferr, ok := f0["error"].(map[string]any)
	if !ok {
		t.Fatalf("failed[0].error missing: %T", f0["error"])
	}
	if ferr["code"] != "SHIFT_OVER_LEAVE" {
		t.Errorf("failed[0].error.code = %v, want SHIFT_OVER_LEAVE", ferr["code"])
	}
	if after := len(h.schedule.entries); after != before {
		t.Errorf(":check persisted an entry: before=%d after=%d (must be side-effect-free)", before, after)
	}
}

// ---------------------------------------------------------------------------
// :bulk-apply — partial success / all-failed / weekdays_mask
// ---------------------------------------------------------------------------

func TestBulkApply_PartialSuccess(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-3001", "SWP-CMP-0021")
	// One date in the range hits SHIFT_OVER_LEAVE; the rest succeed.
	h.seedApprovedLeave("SWP-EMP-3001", dateLeaveDay, "SWP-LR-44210", "ANNUAL")

	rr := h.do("POST", "/schedule:bulk-apply", map[string]any{
		"kind":            "bulk",
		"shift_master_id": "SWP-SHF-001",
		"start_date":      "2026-06-08", // Mon
		"end_date":        "2026-06-10", // Wed (includes the leave day 06-09)
		"employee_ids":    []string{"SWP-EMP-3001"},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("partial: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	succeeded, _ := body["succeeded"].([]any)
	failed, _ := body["failed"].([]any)
	if len(succeeded) == 0 {
		t.Errorf("succeeded is empty, want >=1")
	}
	if len(failed) == 0 {
		t.Fatalf("failed is empty, want >=1 (the leave day)")
	}
	// Shape assertions on both arrays.
	s0 := succeeded[0].(map[string]any)
	for _, k := range []string{"id", "employee_id", "date", "status"} {
		if _, ok := s0[k]; !ok {
			t.Errorf("succeeded[0] missing key: %s", k)
		}
	}
	f0 := failed[0].(map[string]any)
	for _, k := range []string{"employee_id", "date", "error"} {
		if _, ok := f0[k]; !ok {
			t.Errorf("failed[0] missing key: %s", k)
		}
	}
	ferr := f0["error"].(map[string]any)
	if ferr["code"] != "SHIFT_OVER_LEAVE" {
		t.Errorf("failed[0].error.code = %v, want SHIFT_OVER_LEAVE", ferr["code"])
	}
}

func TestBulkApply_AllFailed_422(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-3001", "SWP-CMP-0021")

	// Date range entirely OUTSIDE the placement window → every cell fails.
	rr := h.do("POST", "/schedule:bulk-apply", map[string]any{
		"kind":            "bulk",
		"shift_master_id": "SWP-SHF-001",
		"start_date":      "2030-01-01",
		"end_date":        "2030-01-03",
		"employee_ids":    []string{"SWP-EMP-3001"},
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("all-failed: expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	succeeded, _ := body["succeeded"].([]any)
	failed, _ := body["failed"].([]any)
	if len(succeeded) != 0 {
		t.Errorf("succeeded = %d, want 0 (all failed)", len(succeeded))
	}
	if len(failed) == 0 {
		t.Errorf("failed is empty, want populated on all-failed 422")
	}
}

func TestBulkApply_WeekdaysMask(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-1108", "SWP-CMP-0021")

	// Range 2026-06-08 (Mon) .. 2026-06-14 (Sun) = 7 days. Mask Mon-Fri = 5 days.
	rr := h.do("POST", "/schedule:bulk-apply", map[string]any{
		"kind":            "bulk",
		"shift_master_id": "SWP-SHF-001",
		"start_date":      "2026-06-08",
		"end_date":        "2026-06-14",
		"employee_ids":    []string{"SWP-EMP-1108"},
		"weekdays_mask":   []int{1, 2, 3, 4, 5},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("weekdays_mask: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	succeeded, _ := body["succeeded"].([]any)
	failed, _ := body["failed"].([]any)
	total := len(succeeded) + len(failed)
	// 5 in-mask dates × 1 employee = 5 cells attempted (Sat/Sun excluded).
	if total != 5 {
		t.Errorf("attempted cells = %d, want 5 (Mon-Fri × 1 agent)", total)
	}
}

// ---------------------------------------------------------------------------
// DELETE — 204 + leader past-date 403
// ---------------------------------------------------------------------------

func TestDelete_204(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	// A future-dated live entry the hr_admin may clear.
	h.seedLiveEntry("SWP-SCH-6001", "SWP-EMP-1108", "SWP-CMP-0021", dateInWindow, "Pagi")

	rr := h.do("DELETE", "/schedule/SWP-SCH-6001", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, ok := h.schedule.entries["SWP-SCH-6001"]; ok {
		t.Errorf("entry still present after DELETE (must be soft-deleted)")
	}
}

func TestDelete_LeaderPastDate_403(t *testing.T) {
	// Leader scoped to the entry's company; the entry is on a PAST date.
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-0021")
	pastDate := ymd(2026, 5, 1) // before the fixed clock (2026-06-04)
	h.seedLiveEntry("SWP-SCH-PAST", "SWP-EMP-1108", "SWP-CMP-0021", pastDate, "Pagi")

	rr := h.do("DELETE", "/schedule/SWP-SCH-PAST", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("leader past-date delete: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// silence unused in case a fixture is dropped during edits.
var _ = time.Monday
