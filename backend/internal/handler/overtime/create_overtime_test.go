// Package overtime_test — F7.2 createOvertimeRequest (POST /overtime) contract
// tests. Covers the agent self-request 201 happy path, the agent-impersonation
// 403, the approved-leave overlap 409 (OT_OVERLAPS_LEAVE), the no-active-placement
// 422 (OT_NO_SCHEDULED_SHIFT / OC-6), and the agent cross-employee GET → 404.
package overtime_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	schedulingsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// seedActivePlacement plants an active placement covering (employee, date) so the
// OC-6 resolution succeeds; company is denormalized onto the OT (service_line removed
// 2026-06-12).
func seedActivePlacement(h *harness, employee string, date time.Time, company string) {
	h.schedule.placements[liveKey(employee, date)] = schedulingsvc.PlacementCover{
		PlacementID: "SWP-PL-5001",
		CompanyID:   company,
		StartDate:   date.AddDate(0, -1, 0),
	}
}

func createBody(employeeID string) map[string]any {
	return map[string]any{
		"employee_id":        employeeID,
		"work_date":          "2026-06-10",
		"planned_start_time": "15:00",
		"planned_end_time":   "17:00",
		"reason":             "Cover for absent colleague at front desk.",
	}
}

// ---------------------------------------------------------------------------
// POST /overtime — agent self-request 201
// ---------------------------------------------------------------------------

func TestCreateOvertime_AgentSelf_201(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", empAgent)
	workDate := ymd(2026, time.June, 10)
	seedActivePlacement(h, empAgent, workDate, cmpLed)
	h.schedule.live[liveKey(empAgent, workDate)] = schedulingsvc.LiveEntry{ID: "SWP-SCH-1", Status: "PUBLISHED"}

	// employee_id omitted: server fills from the token.
	body := createBody("")
	rr := h.do("POST", "/overtime", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	out := decodeBody(t, rr)
	data, ok := out["data"].(map[string]any)
	if !ok {
		t.Fatalf("data missing/not an object: %T", out["data"])
	}
	if data["status"] != "PENDING_L1" {
		t.Errorf("status = %v, want PENDING_L1", data["status"])
	}
	if data["source"] != "REQUESTED" {
		t.Errorf("source = %v, want REQUESTED", data["source"])
	}
	if data["tier_indicator"] != "WORKDAY" {
		t.Errorf("tier_indicator = %v, want WORKDAY", data["tier_indicator"])
	}
	emp, _ := data["employee"].(map[string]any)
	if emp["id"] != empAgent {
		t.Errorf("employee.id = %v, want %s", emp["id"], empAgent)
	}
	if data["placement_id"] != "SWP-PL-5001" {
		t.Errorf("placement_id = %v, want SWP-PL-5001", data["placement_id"])
	}
	if loc := rr.Header().Get("Location"); loc == "" {
		t.Errorf("Location header missing")
	}
}

// ---------------------------------------------------------------------------
// POST /overtime — agent requests for a DIFFERENT employee → 403
// ---------------------------------------------------------------------------

func TestCreateOvertime_AgentOtherEmployee_403(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", empAgent)
	workDate := ymd(2026, time.June, 10)
	seedActivePlacement(h, "SWP-EMP-9999", workDate, cmpLed)

	rr := h.do("POST", "/overtime", createBody("SWP-EMP-9999"))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /overtime — no active placement on work_date → 422 OT_NO_SCHEDULED_SHIFT
// ---------------------------------------------------------------------------

func TestCreateOvertime_NoActivePlacement_422(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", empAgent)
	// no placement seeded
	rr := h.do("POST", "/overtime", createBody(""))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "OT_NO_SCHEDULED_SHIFT" {
		t.Errorf("error code = %s, want OT_NO_SCHEDULED_SHIFT", code)
	}
}

// ---------------------------------------------------------------------------
// POST /overtime — work_date overlaps an APPROVED leave → 409 OT_OVERLAPS_LEAVE
// ---------------------------------------------------------------------------

func TestCreateOvertime_OverlapsLeave_409(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", empAgent)
	workDate := ymd(2026, time.June, 10)
	seedActivePlacement(h, empAgent, workDate, cmpLed)
	lr := "SWP-LR-7001"
	lt := "ANNUAL"
	h.schedule.leaves[liveKey(empAgent, workDate)] = schedulingsvc.ApprovedLeave{LeaveRequestID: &lr, LeaveType: &lt}

	rr := h.do("POST", "/overtime", createBody(""))
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "OT_OVERLAPS_LEAVE" {
		t.Errorf("error code = %s, want OT_OVERLAPS_LEAVE", code)
	}
}

// ---------------------------------------------------------------------------
// GET /overtime/{id} — agent reading another agent's OT → 404 (no existence leak)
// ---------------------------------------------------------------------------

func TestGetOvertime_AgentCrossEmployee_404(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", empAgent)
	h.seedOvertime("SWP-OT-40001", cmpLed, "SWP-EMP-OTHER", "PENDING_L1", "WORKDAY")

	rr := h.do("GET", "/overtime/SWP-OT-40001", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// GET own OT as the agent → 200 (self-scope allows the read).
func TestGetOvertime_AgentOwn_200(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", empAgent)
	h.seedOvertime("SWP-OT-40002", cmpLed, empAgent, "PENDING_L1", "WORKDAY")

	rr := h.do("GET", "/overtime/SWP-OT-40002", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
