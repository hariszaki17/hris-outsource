// Package attendance_test — HTTP contract tests for the F5.8 attendance activity log
// endpoints (createAttendanceActivity / listAttendanceActivities /
// deleteAttendanceActivity) AND the agent clock-out activity gate (AA-7 / INV-7).
//
// Activity endpoints mount on the shared newHarness (real ActivityService +
// ActivityHandler over the in-memory fakeActivityRepo / fakeAttendanceRepo). The
// clock-out gate is exercised on a dedicated clock harness over fakeClockRepo, whose
// CountActivities result drives the 422 ACTIVITY_REQUIRED vs 200 decision.
//
// Test names carry the TC ids from
// docs/epics/E5-attendance/prds/attendance-activity.test-cases.md.
package attendance_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	attendancehandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// seedOpenOwnRecord plants an OPEN attendance record owned by the calling agent
// (check_in set, check_out nil). Returns the record id.
func seedOpenOwnRecord(h *harness, id, employee string) string {
	ci := fixedNow
	h.attendance.records[id] = att.Attendance{
		ID:                 id,
		EmployeeID:         employee,
		PlacementID:        "SWP-PL-5001",
		CompanyID:          cmpLed,
		SiteID:             "SWP-SITE-001",
		Position:           "Petugas Kebersihan",
		CheckInAt:          &ci,
		Status:             att.StatusPresent,
		VerificationStatus: att.VerificationAutoApproved,
		CreatedAt:          ci,
		UpdatedAt:          ci,
	}
	return id
}

// agentHarness builds an agent-principal harness with an open own record seeded.
func agentHarness(t *testing.T, attID string) *harness {
	t.Helper()
	h := newHarness(t, auth.RoleAgent, "", empAgentSelf)
	seedOpenOwnRecord(h, attID, empAgentSelf)
	return h
}

// TC-1 — Log activity right after clock-in (happy, 201).
func TestCreateActivity_TC1_Happy(t *testing.T) {
	h := agentHarness(t, "SWP-ATT-A1")
	rr := h.do(http.MethodPost, "/attendance/SWP-ATT-A1/activities", map[string]any{"note": "Patroli lantai 1"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	data, _ := decodeBody(t, rr)["data"].(map[string]any)
	if got := strOf(data["note"]); got != "Patroli lantai 1" {
		t.Errorf("note = %q, want \"Patroli lantai 1\"", got)
	}
	if strOf(data["recorded_at"]) == "" {
		t.Error("recorded_at is empty, want a server-set timestamp (AA-5)")
	}
	if got := strOf(data["employee_id"]); got != empAgentSelf {
		t.Errorf("employee_id = %q, want own %s (AA-2)", got, empAgentSelf)
	}
	if n := h.activity.countLive("SWP-ATT-A1"); n != 1 {
		t.Errorf("live activity count = %d, want 1", n)
	}
}

// TC-10 — Empty/whitespace note rejected (400 INVALID_REQUEST, fields.note).
func TestCreateActivity_TC10_BlankNote(t *testing.T) {
	h := agentHarness(t, "SWP-ATT-A2")
	rr := h.do(http.MethodPost, "/attendance/SWP-ATT-A2/activities", map[string]any{"note": "   "})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if strOf(e["code"]) != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", e["code"])
	}
	if fields, ok := e["fields"].(map[string]any); !ok || fields["note"] == nil {
		t.Errorf("expected fields.note in error, got %v", e["fields"])
	}
}

// TC-6 — Write is scope-self: posting to another agent's record is hidden as 404,
// nothing created (AA-2).
func TestCreateActivity_TC6_NotOwner(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", empAgentSelf)
	// Record owned by a DIFFERENT employee.
	ci := fixedNow
	h.attendance.records["SWP-ATT-OTHER"] = att.Attendance{
		ID: "SWP-ATT-OTHER", EmployeeID: "SWP-EMP-9999", CompanyID: cmpLed,
		CheckInAt: &ci, VerificationStatus: att.VerificationAutoApproved,
	}
	rr := h.do(http.MethodPost, "/attendance/SWP-ATT-OTHER/activities", map[string]any{"note": "Patroli"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
	if n := h.activity.countLive("SWP-ATT-OTHER"); n != 0 {
		t.Errorf("live activity count = %d, want 0 (nothing created)", n)
	}
}

// TC-7 — No add after clock-out: a closed own record → 422 ATTENDANCE_CLOSED (AA-3).
func TestCreateActivity_TC7_Closed(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", empAgentSelf)
	ci := fixedNow
	co := fixedNow.Add(8 * time.Hour)
	h.attendance.records["SWP-ATT-CLOSED"] = att.Attendance{
		ID: "SWP-ATT-CLOSED", EmployeeID: empAgentSelf, CompanyID: cmpLed,
		CheckInAt: &ci, CheckOutAt: &co, VerificationStatus: att.VerificationVerified,
	}
	rr := h.do(http.MethodPost, "/attendance/SWP-ATT-CLOSED/activities", map[string]any{"note": "Patroli"})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body: %s)", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "ATTENDANCE_CLOSED" {
		t.Errorf("code = %q, want ATTENDANCE_CLOSED", code)
	}
}

// TC-8 — No add to an ABSENT record (no clock-in) → 422 ATTENDANCE_NOT_OPEN (AA-3).
func TestCreateActivity_TC8_Absent(t *testing.T) {
	h := newHarness(t, auth.RoleAgent, "", empAgentSelf)
	h.attendance.records["SWP-ATT-ABSENT"] = att.Attendance{
		ID: "SWP-ATT-ABSENT", EmployeeID: empAgentSelf, CompanyID: cmpLed,
		Status: att.StatusAbsent, VerificationStatus: att.VerificationPending,
		// CheckInAt nil → true ABSENT.
	}
	rr := h.do(http.MethodPost, "/attendance/SWP-ATT-ABSENT/activities", map[string]any{"note": "Patroli"})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body: %s)", rr.Code, rr.Body.String())
	}
	if code := errCode(t, rr); code != "ATTENDANCE_NOT_OPEN" {
		t.Errorf("code = %q, want ATTENDANCE_NOT_OPEN", code)
	}
}

// Activity list returns the record's activities chronologically (200, AA-13).
func TestListActivities_OK(t *testing.T) {
	h := agentHarness(t, "SWP-ATT-A3")
	for _, note := range []string{"Patroli lantai 1", "Cek APAR", "Laporan shift"} {
		rr := h.do(http.MethodPost, "/attendance/SWP-ATT-A3/activities", map[string]any{"note": note})
		if rr.Code != http.StatusCreated {
			t.Fatalf("seed create status = %d (body: %s)", rr.Code, rr.Body.String())
		}
	}
	rr := h.do(http.MethodGet, "/attendance/SWP-ATT-A3/activities", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, _ := body["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("data length = %d, want 3", len(data))
	}
	// Chronological order: first inserted note appears first.
	first, _ := data[0].(map[string]any)
	if strOf(first["note"]) != "Patroli lantai 1" {
		t.Errorf("first note = %q, want \"Patroli lantai 1\" (recorded_at asc)", first["note"])
	}
	if body["has_more"] != false {
		t.Errorf("has_more = %v, want false", body["has_more"])
	}
}

// TC-9 — Delete own activity while open (204, removed from list).
func TestDeleteActivity_TC9_Happy(t *testing.T) {
	h := agentHarness(t, "SWP-ATT-A4")
	rr := h.do(http.MethodPost, "/attendance/SWP-ATT-A4/activities", map[string]any{"note": "Cek APAR"})
	data, _ := decodeBody(t, rr)["data"].(map[string]any)
	actID := strOf(data["id"])

	del := h.do(http.MethodDelete, "/attendance/SWP-ATT-A4/activities/"+actID, nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (body: %s)", del.Code, del.Body.String())
	}
	if n := h.activity.countLive("SWP-ATT-A4"); n != 0 {
		t.Errorf("live activity count = %d, want 0 after delete (AA-9)", n)
	}
}

// Delete a non-existent activity → 404 (idempotent, AA-9).
func TestDeleteActivity_NotFound(t *testing.T) {
	h := agentHarness(t, "SWP-ATT-A5")
	rr := h.do(http.MethodDelete, "/attendance/SWP-ATT-A5/activities/SWP-ACT-NOPE", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Clock-out activity gate (AA-7 / INV-7) — dedicated clock harness.
// ---------------------------------------------------------------------------

// clockOutHarness mounts ClockHandler.ClockOut over the fake clock repo with an agent
// principal (mirrors clockHarness but for the clock-out path).
func clockOutHarness(repo *fakeClockRepo) http.Handler {
	csvc := svc.NewClockService(repo, &fakeTxRunner{})
	h := attendancehandler.NewClockHandler(csvc)
	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), auth.Principal{
				UserID: "SWP-USR-1", EmployeeID: "SWP-EMP-0001", Role: auth.RoleAgent,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/attendance:clock-out", h.ClockOut)
	return r
}

func postClockOut(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/attendance:clock-out",
		strings.NewReader(`{"lat":-6.2,"lng":106.8,"gps_available":true,"photo_id":"SWP-FILE-1"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// TC-3 — Clock-out blocked with zero activities → 422 ACTIVITY_REQUIRED,
// fields.activity_count="0"; the record is NOT closed (still clocked in).
func TestClockOut_TC3_GateBlocksZeroActivities(t *testing.T) {
	now := time.Now()
	repo := &fakeClockRepo{
		openID:        "SWP-ATT-OPEN",
		activityCount: 0,
		records:       map[string]att.Attendance{"SWP-ATT-OPEN": {ID: "SWP-ATT-OPEN", CheckInAt: &now}},
	}
	rr := postClockOut(t, clockOutHarness(repo))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body: %s)", rr.Code, rr.Body.String())
	}
	e := errObject(t, decodeBody(t, rr))
	if strOf(e["code"]) != "ACTIVITY_REQUIRED" {
		t.Errorf("code = %q, want ACTIVITY_REQUIRED", e["code"])
	}
	if fields, _ := e["fields"].(map[string]any); strOf(fields["activity_count"]) != "0" {
		t.Errorf("fields.activity_count = %v, want \"0\"", e["fields"])
	}
	if repo.clockedOut {
		t.Error("record was clocked out despite the gate; want still clocked in")
	}
}

// TC-4 — Clock-out succeeds with >=1 activity (200, record closed).
func TestClockOut_TC4_GatePassesWithActivity(t *testing.T) {
	now := time.Now()
	repo := &fakeClockRepo{
		openID:        "SWP-ATT-OPEN",
		activityCount: 1,
		records:       map[string]att.Attendance{"SWP-ATT-OPEN": {ID: "SWP-ATT-OPEN", CheckInAt: &now}},
	}
	rr := postClockOut(t, clockOutHarness(repo))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !repo.clockedOut {
		t.Error("record was not clocked out despite >=1 activity")
	}
}

// TC-5 — Clock-out blocked after deleting the last activity (count back to 0) →
// 422 ACTIVITY_REQUIRED (AA-7 / AA-9). Same gate path as TC-3 with count=0.
func TestClockOut_TC5_GateBlocksAfterDeleteToZero(t *testing.T) {
	now := time.Now()
	repo := &fakeClockRepo{
		openID:        "SWP-ATT-OPEN",
		activityCount: 0, // last activity deleted → count back to 0
		records:       map[string]att.Attendance{"SWP-ATT-OPEN": {ID: "SWP-ATT-OPEN", CheckInAt: &now}},
	}
	rr := postClockOut(t, clockOutHarness(repo))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body: %s)", rr.Code, rr.Body.String())
	}
	if code := strOf(errObject(t, decodeBody(t, rr))["code"]); code != "ACTIVITY_REQUIRED" {
		t.Errorf("code = %q, want ACTIVITY_REQUIRED", code)
	}
	if repo.clockedOut {
		t.Error("record was clocked out despite the gate")
	}
}
