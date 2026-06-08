// Package payroll_test — agent self-scope contract tests for the payslip READS
// (E8 F8.1 / PAY-01). The agent role may read ONLY its own payslips:
//
//	GET /payslips        → employee_id forced to the caller (own rows only); an
//	                       explicit foreign employee_id → 403 OUT_OF_SCOPE.
//	GET /payslips/{id}   → own payslip 200; another employee's payslip 404 (no
//	                       existence leak).
//
// Audit-notes + export are NOT admitted to the agent (route RBAC, asserted via a
// 403 here as defense-in-depth on the contract).
package payroll_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// newAgentHarness builds the payroll slice with an agent caller whose employee id
// is empAgent (the self-scope subject).
func newAgentHarness(t *testing.T, employeeID string) *harness {
	t.Helper()
	h := newHarness(t, auth.RoleAgent)
	h.principal.EmployeeID = employeeID
	return h
}

// TestListPayslips_AgentSeesOnlyOwn: the agent's list is forced to their own
// employee id even when another employee's payslips exist.
func TestListPayslips_AgentSeesOnlyOwn(t *testing.T) {
	h := newAgentHarness(t, empBudi)
	// own row
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)
	// another employee's row — must NOT appear
	h.seedFinal("SWP-PS-90130", empRudi, "Rudi Hartono", 2025, 12, ymd(2025, time.December, 27), finalMoney)

	rr := h.do("GET", "/payslips", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data missing/not an array: %T", body["data"])
	}
	if len(data) != 1 {
		t.Fatalf("agent list length = %d, want 1 (own rows only)", len(data))
	}
	row, _ := data[0].(map[string]any)
	if got := strOf(row["employee_id"]); got != empBudi {
		t.Errorf("row employee_id = %q, want own %q", got, empBudi)
	}
}

// TestListPayslips_AgentForeignEmployeeID_OutOfScope: an explicit foreign
// employee_id query → 403 OUT_OF_SCOPE.
func TestListPayslips_AgentForeignEmployeeID_OutOfScope(t *testing.T) {
	h := newAgentHarness(t, empBudi)
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)

	rr := h.do("GET", "/payslips?employee_id="+empRudi, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for foreign employee_id, got %d: %s", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "OUT_OF_SCOPE" {
		t.Errorf("error code = %q, want OUT_OF_SCOPE", code)
	}
}

// TestListPayslips_AgentOwnEmployeeID_OK: an explicit OWN employee_id query is
// allowed (it equals the forced filter).
func TestListPayslips_AgentOwnEmployeeID_OK(t *testing.T) {
	h := newAgentHarness(t, empBudi)
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)

	rr := h.do("GET", "/payslips?employee_id="+empBudi, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for own employee_id, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestGetPayslip_AgentOwn_OK: the agent reads their own payslip detail at 200.
func TestGetPayslip_AgentOwn_OK(t *testing.T) {
	h := newAgentHarness(t, empBudi)
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)
	h.seedFinalBreakdown(psFinal)

	rr := h.do("GET", "/payslips/"+psFinal, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for own payslip, got %d: %s", rr.Code, rr.Body.String())
	}
	data := dataObject(t, rr)
	if got := strOf(data["employee_id"]); got != empBudi {
		t.Errorf("employee_id = %q, want own %q", got, empBudi)
	}
}

// TestGetPayslip_AgentForeign_NotFound: another employee's payslip is hidden as
// 404 (no existence leak) — never a 403.
func TestGetPayslip_AgentForeign_NotFound(t *testing.T) {
	h := newAgentHarness(t, empBudi)
	// payslip belongs to a DIFFERENT employee
	h.seedFinal(psFinal, empRudi, "Rudi Hartono", 2025, 12, ymd(2025, time.December, 28), finalMoney)

	rr := h.do("GET", "/payslips/"+psFinal, nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for another employee's payslip, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestAuditNotes_AgentForbidden: audit-notes + export are NOT admitted to the
// agent (route RBAC) — 403, defense-in-depth on the contract.
func TestAuditNotes_AgentForbidden(t *testing.T) {
	h := newAgentHarness(t, empBudi)
	h.seedFinal(psFinal, empBudi, "Budi Santoso", 2025, 12, ymd(2025, time.December, 28), finalMoney)

	if rr := h.do("GET", "/payslips/"+psFinal+"/audit-notes", nil); rr.Code != http.StatusForbidden {
		t.Errorf("GET audit-notes: expected 403 for agent, got %d", rr.Code)
	}
	if rr := h.do("POST", "/payslips/"+psFinal+"/audit-notes", map[string]any{"text": "x"}); rr.Code != http.StatusForbidden {
		t.Errorf("POST audit-notes: expected 403 for agent, got %d", rr.Code)
	}
	if rr := h.do("POST", "/payslips:export", map[string]any{}); rr.Code != http.StatusForbidden {
		t.Errorf("POST export: expected 403 for agent, got %d", rr.Code)
	}
}
