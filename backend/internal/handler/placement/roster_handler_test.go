// Package placement_test — E3 company-roster contract tests.
//
// Asserts the CompanyRosterResponse shape, the OUT_OF_SCOPE 403 for a shift_leader
// requesting a roster for a company they don't lead (RO-4 / C-4), and that
// include_history toggles terminal-state placements. Drift gate vs
// docs/api/E3-placement/openapi.yaml.
package placement_test

import (
	"net/http"
	"testing"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

func (h *placementHarness) seedRosterPlacement(id, companyID, status string) {
	end := jktDate(2027, 6, 30)
	h.seedPlacement(domain.Placement{
		ID: id, EmployeeID: "SWP-EMP-" + id, ClientCompanyID: companyID,
		SiteID: "SWP-SITE-0001", ServiceLineID: "SWP-SVC-001", PositionID: "SWP-POS-014",
		AgreementID: strp("SWP-AG-7002"), StartDate: jktDate(2026, 1, 1), EndDate: &end,
		LifecycleStatus: status, EmployeeName: strp("Emp " + id),
	})
}

func TestGetRoster_HRAdmin_ShapeAndSummary_200(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")
	h.leaderRepo().rosterSummary = domain.CompanyRosterSummary{
		TotalActive: 2, TotalScheduled: 0, TotalExpiring: 1,
		ByServiceLine: []domain.RosterServiceLineCount{{ServiceLineID: "SWP-SVC-001", ServiceLineName: "Parking", Count: 2}},
		ByStatus:      []domain.RosterStatusCount{{Status: "ACTIVE", Count: 2}},
	}
	h.seedRosterPlacement("R1", "SWP-CMP-0021", "ACTIVE")
	h.seedRosterPlacement("R2", "SWP-CMP-0021", "ACTIVE")

	rr := h.doJSON("GET", "/client-companies/SWP-CMP-0021/roster", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	for _, k := range []string{"company_id", "company_name", "placements", "current_shift_leader", "summary", "has_more"} {
		if _, ok := body[k]; !ok {
			t.Errorf("roster missing key: %s", k)
		}
	}
	summary, ok := body["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary is not an object: %T", body["summary"])
	}
	for _, k := range []string{"total_active", "total_scheduled", "total_expiring", "by_service_line", "by_status"} {
		if _, ok := summary[k]; !ok {
			t.Errorf("summary missing key: %s", k)
		}
	}
	if _, ok := summary["by_service_line"].([]any); !ok {
		t.Errorf("summary.by_service_line is not an array: %T", summary["by_service_line"])
	}
	if _, ok := body["placements"].([]any); !ok {
		t.Errorf("placements is not an array: %T", body["placements"])
	}
}

func TestGetRoster_ShiftLeaderOwnCompany_200(t *testing.T) {
	h := newPlacementHarness(t)
	h.principal = auth.Principal{
		UserID: "SWP-USR-SL", Role: auth.RoleShiftLeader,
		EmployeeID: "SWP-EMP-1108", CompanyID: "SWP-CMP-0021",
	}
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")
	h.seedRosterPlacement("R1", "SWP-CMP-0021", "ACTIVE")

	rr := h.doJSON("GET", "/client-companies/SWP-CMP-0021/roster", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for SL own company, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetRoster_ShiftLeaderOtherCompany_OUT_OF_SCOPE_403(t *testing.T) {
	h := newPlacementHarness(t)
	h.principal = auth.Principal{
		UserID: "SWP-USR-SL", Role: auth.RoleShiftLeader,
		EmployeeID: "SWP-EMP-1108", CompanyID: "SWP-CMP-0021",
	}
	h.seedCompany("SWP-CMP-0022", "Mall Senayan City", "active")
	h.seedRosterPlacement("R9", "SWP-CMP-0022", "ACTIVE")

	rr := h.doJSON("GET", "/client-companies/SWP-CMP-0022/roster", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 OUT_OF_SCOPE for SL cross-company roster, got %d: %s", rr.Code, rr.Body.String())
	}
	if errObject(t, decodeBody(t, rr))["code"] != "OUT_OF_SCOPE" {
		t.Errorf("error.code != OUT_OF_SCOPE")
	}
}

func TestGetRoster_IncludeHistoryToggle(t *testing.T) {
	h := newPlacementHarness(t)
	h.seedCompany("SWP-CMP-0021", "Plaza Senayan", "active")
	h.seedRosterPlacement("RACT", "SWP-CMP-0021", "ACTIVE")
	h.seedRosterPlacement("RDEAD", "SWP-CMP-0021", "ENDED") // terminal

	// Default: terminal placements excluded.
	rr := h.doJSON("GET", "/client-companies/SWP-CMP-0021/roster", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	def := decodeBody(t, rr)["placements"].([]any)
	for _, p := range def {
		if p.(map[string]any)["id"] == "RDEAD" {
			t.Error("default roster should exclude terminal-state placement RDEAD")
		}
	}

	// include_history=true: terminal placements included.
	rr2 := h.doJSON("GET", "/client-companies/SWP-CMP-0021/roster?include_history=true", nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	withHist := decodeBody(t, rr2)["placements"].([]any)
	found := false
	for _, p := range withHist {
		if p.(map[string]any)["id"] == "RDEAD" {
			found = true
		}
	}
	if !found {
		t.Error("include_history=true roster should include terminal-state placement RDEAD")
	}
}
