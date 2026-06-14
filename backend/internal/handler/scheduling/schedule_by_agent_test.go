// Package scheduling_test — GET /schedule/by-agent/{employee_id} (F4.3 "Jadwal
// Saya") contract tests. Covers the SV-1 self-scope (agent self 200 / agent
// other 403), staff cross-agent reads, leader company scope, and the required
// date-param 400s. Asserted against docs/api/E4-shift-scheduling/openapi.yaml
// (operationId getScheduleByAgent).
package scheduling_test

import (
	"net/http"
	"testing"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// byAgentPath builds the by-agent URL for the given agent + window.
func byAgentPath(empID, start, end string) string {
	return "/schedule/by-agent/" + empID + "?start_date=" + start + "&end_date=" + end
}

func TestByAgent_AgentSelf_200(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "")
	h.principal.EmployeeID = "SWP-EMP-1108"
	seedHappyAgent(h, "SWP-EMP-1108", "SWP-CMP-0021")
	h.seedLiveEntry("SWP-SCH-6001", "SWP-EMP-1108", "SWP-CMP-0021", dateInWindow, "Pagi")

	rr := h.do("GET", byAgentPath("SWP-EMP-1108", "2026-06-01", "2026-06-30"), nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data missing/not an array: %T", body["data"])
	}
	if len(data) != 1 {
		t.Errorf("data len = %d, want 1", len(data))
	}
	if _, ok := body["warnings"].([]any); !ok {
		t.Errorf("warnings missing/not an array (must be present even if empty): %T", body["warnings"])
	}
}

func TestByAgent_AgentOther_403(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "")
	h.principal.EmployeeID = "SWP-EMP-1108"
	seedHappyAgent(h, "SWP-EMP-2222", "SWP-CMP-0021")

	// SV-1: an agent may not read another agent's schedule.
	rr := h.do("GET", byAgentPath("SWP-EMP-2222", "2026-06-01", "2026-06-30"), nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestByAgent_MissingStartDate_400(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")

	rr := h.do("GET", "/schedule/by-agent/SWP-EMP-1108?end_date=2026-06-30", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestByAgent_StaffReadsAnyAgent_200(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	seedHappyAgent(h, "SWP-EMP-1108", "SWP-CMP-0021")
	h.seedLiveEntry("SWP-SCH-6001", "SWP-EMP-1108", "SWP-CMP-0021", dateInWindow, "Pagi")

	rr := h.do("GET", byAgentPath("SWP-EMP-1108", "2026-06-01", "2026-06-30"), nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestByAgent_LeaderOutOfCompany_403(t *testing.T) {
	// Leader scoped to CMP-0099; the agent is placed at CMP-0021 → out of scope.
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-0099")
	seedHappyAgent(h, "SWP-EMP-1108", "SWP-CMP-0021")

	rr := h.do("GET", byAgentPath("SWP-EMP-1108", "2026-06-01", "2026-06-30"), nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}
