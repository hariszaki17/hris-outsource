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

	// admin block is OMITTED for hr_admin (C-6).
	if _, present := d["admin"]; present {
		t.Errorf("hr_admin payload must OMIT admin (C-6), got: %v", d["admin"])
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
	// admin block PRESENT for super_admin (DB-7); empty admin data → empty arrays +
	// zero counts (the empty-state variant, C-5), but the block + all four
	// sub-widgets are present.
	admin, ok := d["admin"].(map[string]any)
	if !ok {
		t.Fatalf("super_admin payload must include admin block (DB-7), got: %v", d["admin"])
	}
	for _, k := range []string{"user_access", "recent_audit", "org_rollups", "pending_grants"} {
		if _, present := admin[k]; !present {
			t.Errorf("admin.%s missing (required sub-widget)", k)
		}
	}
	if _, ok := admin["recent_audit"].([]any); !ok {
		t.Errorf("admin.recent_audit must be an array (present, not null): %v", admin["recent_audit"])
	}
	if _, ok := admin["org_rollups"].([]any); !ok {
		t.Errorf("admin.org_rollups must be an array (present, not null): %v", admin["org_rollups"])
	}
}

// TestDashboard_SuperAdminWidgets exercises the populated admin block (DB-7): the
// four sub-widgets, the free-text position org-rollup (decision 2026-06-12:
// service_line removed; rollups GROUP BY the free-text placement position), and the
// audit actor/target label composition.
func TestDashboard_SuperAdminWidgets(t *testing.T) {
	h := newHarness(t, auth.RoleSuperAdmin, "", "SWP-EMP-9001")
	h.dash.adminData = svc.SuperAdminWidgetsData{
		ActiveUsers:          514,
		OffboardedUsers30d:   7,
		BankApprovalsPending: 3,
		OrgRollups: []svc.OrgRollupRow{
			{Position: "Cleaning Service", Headcount: 312, ActivePlacements: 298},
			{Position: "Security", Headcount: 90, ActivePlacements: 88},
			{Position: "Parking Attendant", Headcount: 80, ActivePlacements: 80},
		},
		RecentAudit: []svc.AuditRow{
			{ID: "SWP-AL-90412", ActorUserID: strp("SWP-USR-7"), ActorRole: strp("hr_admin"), Action: "ROLE_GRANTED", EntityType: "user", EntityID: "SWP-USR-42", CreatedAt: fixedNow},
			{ID: "SWP-AL-90411", ActorUserID: nil, ActorRole: nil, Action: "PLACEMENT_EXPIRED", EntityType: "placement", EntityID: "SWP-PL-882", CreatedAt: fixedNow},
		},
	}

	rr := h.do("GET", "/dashboards/me", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	admin, ok := dataObject(t, rr)["admin"].(map[string]any)
	if !ok {
		t.Fatalf("admin block missing")
	}

	ua := admin["user_access"].(map[string]any)
	if ua["active_users"] != float64(514) || ua["offboarded_30d"] != float64(7) {
		t.Errorf("user_access = %v, want active_users 514 / offboarded_30d 7", ua)
	}
	// pending_provisioning honest 0 (E2 §8 D1 auto-provision).
	if ua["pending_provisioning"] != float64(0) {
		t.Errorf("user_access.pending_provisioning = %v, want 0", ua["pending_provisioning"])
	}

	pg := admin["pending_grants"].(map[string]any)
	if pg["bank_approvals"] != float64(3) {
		t.Errorf("pending_grants.bank_approvals = %v, want 3", pg["bank_approvals"])
	}
	// role_requests honest 0 (no role-change-request table).
	if pg["role_requests"] != float64(0) {
		t.Errorf("pending_grants.role_requests = %v, want 0", pg["role_requests"])
	}

	// org_rollups: 3 free-text positions (no master/enum; all rows pass through).
	rollups := admin["org_rollups"].([]any)
	if len(rollups) != 3 {
		t.Fatalf("org_rollups = %d rows, want 3", len(rollups))
	}
	wantPos := map[string][2]float64{ // position → {headcount, active_placements}
		"Cleaning Service":  {312, 298},
		"Security":          {90, 88},
		"Parking Attendant": {80, 80},
	}
	for _, row := range rollups {
		r := row.(map[string]any)
		pos := r["position"].(string)
		want, found := wantPos[pos]
		if !found {
			t.Errorf("unexpected org_rollups position %s", pos)
			continue
		}
		if r["headcount"] != want[0] || r["active_placements"] != want[1] {
			t.Errorf("org_rollups[%s] = %v, want headcount %v / active_placements %v", pos, r, want[0], want[1])
		}
	}

	// recent_audit: newest-first, labels composed from the raw audit columns.
	audit := admin["recent_audit"].([]any)
	if len(audit) != 2 {
		t.Fatalf("recent_audit = %d rows, want 2", len(audit))
	}
	first := audit[0].(map[string]any)
	if first["id"] != "SWP-AL-90412" || first["action"] != "ROLE_GRANTED" {
		t.Errorf("recent_audit[0] = %v, want id SWP-AL-90412 / action ROLE_GRANTED", first)
	}
	if first["actor_label"] != "SWP-USR-7 (hr_admin)" {
		t.Errorf("recent_audit[0].actor_label = %v, want \"SWP-USR-7 (hr_admin)\"", first["actor_label"])
	}
	if first["target_label"] != "user SWP-USR-42" {
		t.Errorf("recent_audit[0].target_label = %v, want \"user SWP-USR-42\"", first["target_label"])
	}
	if _, ok := first["at"].(string); !ok {
		t.Errorf("recent_audit[0].at missing/not string: %v", first["at"])
	}
	// null actor → "Sistem".
	second := audit[1].(map[string]any)
	if second["actor_label"] != "Sistem" {
		t.Errorf("recent_audit[1].actor_label = %v, want Sistem (null actor)", second["actor_label"])
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
