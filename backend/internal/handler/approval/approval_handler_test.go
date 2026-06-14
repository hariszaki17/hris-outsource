// Package approval_test — E11 approvals handler contract tests. They drive the 8
// endpoints over the real ApprovalService + in-memory fakes and assert the openapi
// status codes + envelopes + every error code (200/204/400/403/404/409/422), the
// super-admin-only bypass route, reject/bypass reason-required, and body decoding
// for ApprovalTemplateUpsert / DecisionReason / ApproveApprovalInstanceBody.
package approval_test

import (
	"net/http"
	"testing"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// ===========================================================================
// F11.1 templates — GET / PUT / DELETE
// ===========================================================================

func TestGetTemplate_200(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-HR", "SWP-USR-HR")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-A"}, {"SWP-USR-B"}})
	rr := h.do(http.MethodGet, "/client-companies/SWP-CMP-1/approval-template", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
	d := bodyObject(t, rr)
	if d["version"].(float64) != 1 {
		t.Fatalf("version = %v, want 1", d["version"])
	}
}

func TestGetTemplate_404WhenNone(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-HR", "SWP-USR-HR")
	rr := h.do(http.MethodGet, "/client-companies/SWP-CMP-NOPE/approval-template", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	if c := errCode(t, rr); c != "NOT_FOUND" {
		t.Fatalf("code = %s, want NOT_FOUND", c)
	}
}

func TestUpsertTemplate_200(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-HR", "SWP-USR-HR")
	h.seedActive("SWP-USR-A", "SWP-USR-B")
	body := map[string]any{"lines": []any{
		map[string]any{"members": []string{"SWP-USR-A"}},
		map[string]any{"members": []string{"SWP-USR-B"}},
	}}
	rr := h.do(http.MethodPut, "/client-companies/SWP-CMP-1/approval-template", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
	if bodyObject(t, rr)["version"].(float64) != 1 {
		t.Fatalf("version != 1")
	}
}

func TestUpsertTemplate_400TooFewLines(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-HR", "SWP-USR-HR")
	h.seedActive("SWP-USR-A")
	body := map[string]any{"lines": []any{map[string]any{"members": []string{"SWP-USR-A"}}}}
	rr := h.do(http.MethodPut, "/client-companies/SWP-CMP-1/approval-template", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if c := errCode(t, rr); c != dom.CodeInvalidRequest {
		t.Fatalf("code = %s, want %s", c, dom.CodeInvalidRequest)
	}
}

func TestUpsertTemplate_422InactiveMember(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-HR", "SWP-USR-HR")
	h.seedActive("SWP-USR-A")
	h.repo.active["SWP-USR-DEAD"] = false
	body := map[string]any{"lines": []any{
		map[string]any{"members": []string{"SWP-USR-A"}},
		map[string]any{"members": []string{"SWP-USR-DEAD"}},
	}}
	rr := h.do(http.MethodPut, "/client-companies/SWP-CMP-1/approval-template", body)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody: %s", rr.Code, rr.Body.String())
	}
	if c := errCode(t, rr); c != dom.CodeApprovalLineInvalid {
		t.Fatalf("code = %s, want %s", c, dom.CodeApprovalLineInvalid)
	}
}

func TestUpsertTemplate_400MalformedBody(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-HR", "SWP-USR-HR")
	rr := h.doRaw(http.MethodPut, "/client-companies/SWP-CMP-1/approval-template", "{not-json")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestUpsertTemplate_403Agent(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", "SWP-EMP-9", "SWP-USR-9")
	body := map[string]any{"lines": []any{
		map[string]any{"members": []string{"SWP-USR-A"}},
		map[string]any{"members": []string{"SWP-USR-B"}},
	}}
	rr := h.do(http.MethodPut, "/client-companies/SWP-CMP-1/approval-template", body)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestDeleteTemplate_204(t *testing.T) {
	h := newHarness(t, auth.RoleSuperAdmin, "", "SWP-EMP-SUP", "SWP-USR-SUP")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-A"}, {"SWP-USR-B"}})
	rr := h.do(http.MethodDelete, "/client-companies/SWP-CMP-1/approval-template", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204\nbody: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("204 body not empty: %s", rr.Body.String())
	}
}

func TestDeleteTemplate_404WhenNone(t *testing.T) {
	h := newHarness(t, auth.RoleSuperAdmin, "", "SWP-EMP-SUP", "SWP-USR-SUP")
	rr := h.do(http.MethodDelete, "/client-companies/SWP-CMP-NOPE/approval-template", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// ===========================================================================
// F11.3 instances — list / detail
// ===========================================================================

func TestListInstances_200Mine(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-1", "SWP-EMP-L1", "SWP-USR-L1")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusPending)
	rr := h.do(http.MethodGet, "/approval-instances?mine=true", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
	if got := len(pageData(t, rr)); got != 1 {
		t.Fatalf("data len = %d, want 1", got)
	}
}

func TestGetInstance_200(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-HR", "SWP-USR-HR")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusPending)
	rr := h.do(http.MethodGet, "/approval-instances/SWP-APV-1", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
	d := bodyObject(t, rr)
	if d["id"].(string) != "SWP-APV-1" {
		t.Fatalf("id = %v", d["id"])
	}
	if _, ok := d["lines"].([]any); !ok {
		t.Fatalf("detail missing lines array: %s", rr.Body.String())
	}
}

func TestGetInstance_404(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-HR", "SWP-USR-HR")
	rr := h.do(http.MethodGet, "/approval-instances/SWP-APV-NOPE", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// ===========================================================================
// F11.2 execution — approve / reject / bypass
// ===========================================================================

func TestApprove_200Advances(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-1", "SWP-EMP-L1", "SWP-USR-L1")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusPending)
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:approve", map[string]any{"note": "ok"})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
	if bodyObject(t, rr)["current_line"].(float64) != 2 {
		t.Fatalf("current_line != 2")
	}
}

// :approve with an empty body decodes fine (note is optional).
func TestApprove_200NoBody(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-1", "SWP-EMP-L1", "SWP-USR-L1")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusPending)
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:approve", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
}

func TestApprove_403NonMember(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-1", "SWP-EMP-X", "SWP-USR-STRANGER")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusPending)
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:approve", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	if c := errCode(t, rr); c != "FORBIDDEN" {
		t.Fatalf("code = %s, want FORBIDDEN", c)
	}
}

func TestApprove_403SelfApproval(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-1", "SWP-EMP-L1", "SWP-USR-L1")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	// requester employee == caller's employee, and caller is line-1 member.
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-L1", 1, dom.InstanceStatusPending)
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:approve", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	if c := errCode(t, rr); c != dom.CodeSelfApprovalForbidden {
		t.Fatalf("code = %s, want %s", c, dom.CodeSelfApprovalForbidden)
	}
}

func TestApprove_409Terminal(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-1", "SWP-EMP-L1", "SWP-USR-L1")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusApproved)
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:approve", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	if c := errCode(t, rr); c != dom.CodeLineAlreadyCleared {
		t.Fatalf("code = %s, want %s", c, dom.CodeLineAlreadyCleared)
	}
}

func TestReject_200(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-1", "SWP-EMP-L1", "SWP-USR-L1")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusPending)
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:reject", map[string]any{"reason": "Tidak lengkap"})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
	if bodyObject(t, rr)["status"].(string) != string(dom.InstanceStatusRejected) {
		t.Fatalf("status field != REJECTED")
	}
}

func TestReject_400ReasonRequired(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-1", "SWP-EMP-L1", "SWP-USR-L1")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusPending)
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:reject", map[string]any{"reason": ""})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if c := errCode(t, rr); c != "INVALID_REQUEST" {
		t.Fatalf("code = %s, want INVALID_REQUEST", c)
	}
}

func TestBypass_403NonSuper(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-HR", "SWP-USR-HR")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusPending)
	// hr_admin is NOT in the bypass route's allowed roles → router-level 403.
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:bypass", map[string]any{"reason": "darurat"})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestBypass_200Super(t *testing.T) {
	h := newHarness(t, auth.RoleSuperAdmin, "", "SWP-EMP-SUP", "SWP-USR-SUP")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusPending)
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:bypass", map[string]any{"reason": "Eskalasi"})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
	if bodyObject(t, rr)["status"].(string) != string(dom.InstanceStatusApproved) {
		t.Fatalf("status field != APPROVED")
	}
}

func TestBypass_400ReasonRequired(t *testing.T) {
	h := newHarness(t, auth.RoleSuperAdmin, "", "SWP-EMP-SUP", "SWP-USR-SUP")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusPending)
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:bypass", map[string]any{"reason": ""})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestBypass_409Terminal(t *testing.T) {
	h := newHarness(t, auth.RoleSuperAdmin, "", "SWP-EMP-SUP", "SWP-USR-SUP")
	h.seedTemplate("SWP-APT-1", "SWP-CMP-1", 1, [][]string{{"SWP-USR-L1"}, {"SWP-USR-L2"}})
	h.seedInstance("SWP-APV-1", "SWP-CMP-1", "SWP-APT-1", "SWP-EMP-9", 1, dom.InstanceStatusRejected)
	rr := h.do(http.MethodPost, "/approval-instances/SWP-APV-1:bypass", map[string]any{"reason": "telat"})
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}
