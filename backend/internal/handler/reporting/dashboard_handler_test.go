// Package reporting_test — E10 dashboard contract tests (F10.2), asserted against
// docs/api/E10-reporting/openapi.yaml Dashboard oneOf examples:
//
//	GET /dashboards/me as hr_admin → HrDashboard (role hr_admin, role_label
//	    "HR Admin", kpis + pending_approvals_panel deep_link paths); super_admin →
//	    role super_admin + role_label "Super Admin", same shape; shift_leader →
//	    LeaderDashboard (company-scoped, today + pending_counts + company deep
//	    links); agent → AgentDashboard.
//
// The payload is under the {data} envelope (FE unwraps query.data.data + reads
// data.role).
package reporting_test

import (
	"net/http"
	"testing"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/reporting"
)

func TestDashboard_HrAdminShape(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "", "SWP-EMP-9001")
	h.dash.hrCounts = svc.DashboardCounts{
		PendingAttendanceVerify: 8,
		PendingLeaveApprove:     3,
		PendingLeaveApproveHR:   6,
		PendingOTApprove:        5,
		ExpiringPlacements30d:   12,
		ExpiringAgreements30d:   4,
		ActivePlacements:        482,
		ActiveCompanies:         27,
	}

	rr := h.do("GET", "/dashboards/me", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "private, max-age=30" {
		t.Errorf("Cache-Control = %q, want private, max-age=30", cc)
	}
	d := dataObject(t, rr)
	if d["role"] != "hr_admin" {
		t.Errorf("role = %v, want hr_admin", d["role"])
	}
	if d["role_label"] != "HR Admin" {
		t.Errorf("role_label = %v, want HR Admin", d["role_label"])
	}

	kpis, ok := d["kpis"].(map[string]any)
	if !ok {
		t.Fatalf("kpis missing/not object: %v", d["kpis"])
	}
	if kpis["active_placements"] != float64(482) {
		t.Errorf("kpis.active_placements = %v, want 482", kpis["active_placements"])
	}
	if kpis["active_companies"] != float64(27) {
		t.Errorf("kpis.active_companies = %v, want 27", kpis["active_companies"])
	}
	if kpis["leave_pending"] != float64(6) {
		t.Errorf("kpis.leave_pending = %v, want 6 (PENDING_HR only)", kpis["leave_pending"])
	}
	// Required float KPI fields present (honest 0 where no rollup query exists).
	for _, k := range []string{"attendance_rate_pct", "billable_hours_mtd", "ot_hours_mtd"} {
		if _, ok := kpis[k]; !ok {
			t.Errorf("kpis.%s missing (required field)", k)
		}
	}

	if d["expiring_placements_30d"] != float64(12) {
		t.Errorf("expiring_placements_30d = %v, want 12", d["expiring_placements_30d"])
	}
	if d["attendance_anomalies_today"] != float64(8) {
		t.Errorf("attendance_anomalies_today = %v, want 8", d["attendance_anomalies_today"])
	}

	// pending_approvals_panel — 4 rows with the EXACT openapi deep-link paths.
	panel, ok := d["pending_approvals_panel"].([]any)
	if !ok {
		t.Fatalf("pending_approvals_panel missing/not array: %v", d["pending_approvals_panel"])
	}
	if len(panel) != 4 {
		t.Fatalf("panel rows = %d, want 4 (all counts > 0)", len(panel))
	}
	wantPaths := map[string]string{
		"ATTENDANCE_VERIFY":  "/attendance?status=PENDING_VERIFY",
		"LEAVE_APPROVE":      "/leave-requests?status=PENDING_L1,PENDING_L2",
		"OT_APPROVE":         "/overtime?status=PENDING",
		"PLACEMENT_EXPIRING": "/placements?expiring_within=30d",
	}
	for _, row := range panel {
		r := row.(map[string]any)
		kind := r["kind"].(string)
		dl, ok := r["deep_link"].(map[string]any)
		if !ok {
			t.Fatalf("panel row %s deep_link missing/not object: %v", kind, r["deep_link"])
		}
		if want, found := wantPaths[kind]; found {
			if dl["path"] != want {
				t.Errorf("panel[%s].deep_link.path = %v, want %s", kind, dl["path"], want)
			}
		} else {
			t.Errorf("unexpected panel kind %s", kind)
		}
	}
}

func TestDashboard_SuperAdminSameShapeDifferentLabel(t *testing.T) {
	h := newHarness(t, auth.RoleSuperAdmin, "", "SWP-EMP-9001")
	h.dash.hrCounts = svc.DashboardCounts{ActivePlacements: 482, ActiveCompanies: 27}

	rr := h.do("GET", "/dashboards/me", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["role"] != "super_admin" {
		t.Errorf("role = %v, want super_admin", d["role"])
	}
	if d["role_label"] != "Super Admin" {
		t.Errorf("role_label = %v, want Super Admin", d["role_label"])
	}
	// Same shape: kpis present.
	if _, ok := d["kpis"].(map[string]any); !ok {
		t.Errorf("super_admin missing kpis (must share HrDashboard shape)")
	}
	// With all counts zero, the panel is an empty array (present, not null).
	panel, ok := d["pending_approvals_panel"].([]any)
	if !ok {
		t.Fatalf("pending_approvals_panel not an array: %v", d["pending_approvals_panel"])
	}
	if len(panel) != 0 {
		t.Errorf("panel = %d rows, want 0 (no pending counts)", len(panel))
	}
}

func TestDashboard_ShiftLeaderShape(t *testing.T) {
	h := newHarness(t, auth.RoleShiftLeader, "SWP-CMP-0021", "SWP-EMP-7001")
	h.dash.companyName = "Plaza Senayan"
	h.dash.leaderToday = svc.LeaderTodayRow{
		ShiftsTotal: 24, ClockedIn: 19, LateCount: 2, AbsentCount: 3, PendingVerifications: 4,
	}
	h.dash.leaderAV = 4
	h.dash.leaderLA = 1
	h.dash.leaderOT = 2

	rr := h.do("GET", "/dashboards/me", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["role"] != "shift_leader" {
		t.Errorf("role = %v, want shift_leader", d["role"])
	}
	if d["role_label"] != "Shift Leader" {
		t.Errorf("role_label = %v, want Shift Leader", d["role_label"])
	}

	company, ok := d["company"].(map[string]any)
	if !ok {
		t.Fatalf("company missing/not object: %v", d["company"])
	}
	if company["id"] != "SWP-CMP-0021" || company["name"] != "Plaza Senayan" {
		t.Errorf("company = %v, want SWP-CMP-0021 / Plaza Senayan", company)
	}

	today, ok := d["today"].(map[string]any)
	if !ok {
		t.Fatalf("today missing/not object: %v", d["today"])
	}
	if today["shifts_total"] != float64(24) || today["clocked_in"] != float64(19) {
		t.Errorf("today = %v, want shifts_total 24 / clocked_in 19", today)
	}

	pc, ok := d["pending_counts"].(map[string]any)
	if !ok {
		t.Fatalf("pending_counts missing/not object: %v", d["pending_counts"])
	}
	if pc["attendance_verify"] != float64(4) || pc["leave_approve"] != float64(1) || pc["ot_approve"] != float64(2) {
		t.Errorf("pending_counts = %v, want 4/1/2", pc)
	}

	// schedule_alerts is a present (empty) array (no rollup query in 11-01).
	if _, ok := d["schedule_alerts"].([]any); !ok {
		t.Errorf("schedule_alerts missing/not array: %v", d["schedule_alerts"])
	}

	// Company-scoped deep links on the panel.
	panel := d["pending_approvals_panel"].([]any)
	if len(panel) != 3 {
		t.Fatalf("leader panel rows = %d, want 3", len(panel))
	}
	wantPaths := map[string]string{
		"ATTENDANCE_VERIFY": "/attendance?company_id=SWP-CMP-0021&status=PENDING_VERIFY",
		"LEAVE_APPROVE":     "/leave-requests?company_id=SWP-CMP-0021&status=PENDING_L1",
		"OT_APPROVE":        "/overtime?company_id=SWP-CMP-0021&status=PENDING",
	}
	for _, row := range panel {
		r := row.(map[string]any)
		kind := r["kind"].(string)
		dl := r["deep_link"].(map[string]any)
		if want, found := wantPaths[kind]; found {
			if dl["path"] != want {
				t.Errorf("leader panel[%s].path = %v, want %s", kind, dl["path"], want)
			}
		} else {
			t.Errorf("unexpected leader panel kind %s", kind)
		}
	}
}

func TestDashboard_AgentShape(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", "SWP-EMP-3104")
	h.dash.agentRecent = svc.AgentRecentRow{Present: 5, Late: 1, Absent: 0}
	h.dash.agentLeave = 1
	h.dash.agentOT = 0
	h.dash.unread = 3

	rr := h.do("GET", "/dashboards/me", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["role"] != "agent" {
		t.Errorf("role = %v, want agent", d["role"])
	}
	// today_shift nullable (off-duty in 11-01 — no query).
	if _, ok := d["today_shift"]; !ok {
		t.Errorf("today_shift key missing (required, nullable)")
	}

	recent, ok := d["recent_attendance"].(map[string]any)
	if !ok {
		t.Fatalf("recent_attendance missing/not object: %v", d["recent_attendance"])
	}
	if recent["last_7d_present"] != float64(5) || recent["last_7d_late"] != float64(1) {
		t.Errorf("recent_attendance = %v, want present 5 / late 1", recent)
	}

	pending, ok := d["pending_requests"].(map[string]any)
	if !ok {
		t.Fatalf("pending_requests missing/not object: %v", d["pending_requests"])
	}
	if pending["leave"] != float64(1) || pending["ot"] != float64(0) {
		t.Errorf("pending_requests = %v, want leave 1 / ot 0", pending)
	}

	if d["recent_notifications_unread"] != float64(3) {
		t.Errorf("recent_notifications_unread = %v, want 3", d["recent_notifications_unread"])
	}
	// leave_balance object present (required).
	if _, ok := d["leave_balance"].(map[string]any); !ok {
		t.Errorf("leave_balance missing/not object: %v", d["leave_balance"])
	}
}
