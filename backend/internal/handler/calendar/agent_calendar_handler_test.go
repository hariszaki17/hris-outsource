// Package calendar_test — Agent calendar (E10 F10.5) contract tests.
//
// The drift gate for GET /me/calendar, asserted byte-for-shape against the
// handler's agentCalendarDayResponse DTOs:
//
//	GET /me/calendar?year=2026&month=6 → 200 {data: [{date, schedule?, leave?, holiday?, attendance?}]}
//	  - Each day carries sub-objects for schedule, leave, holiday, attendance
//	  - Leave day has leave block populated
//	  - Holiday day has holiday block populated
//	  - Unauthenticated → 401
//	  - Default month → uses current time
package calendar_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	calendarhandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/calendar"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/calendar"
)

// --- fake repositories implementing the calendar ports ---

type fakeScheduleCalendar struct {
	byKey map[string]*svc.ScheduleCalendarEntry // "employeeID|date"
}

func newFakeScheduleCalendar() *fakeScheduleCalendar {
	return &fakeScheduleCalendar{byKey: map[string]*svc.ScheduleCalendarEntry{}}
}

func (r *fakeScheduleCalendar) GetScheduleForAgentDate(_ context.Context, employeeID string, date time.Time) (*svc.ScheduleCalendarEntry, error) {
	key := calendarKey(employeeID, date)
	if e, ok := r.byKey[key]; ok {
		return e, nil
	}
	return nil, nil
}

type fakeLeaveCalendar struct {
	byKey map[string]*svc.LeaveCalendarEntry // "employeeID|date"
}

func newFakeLeaveCalendar() *fakeLeaveCalendar {
	return &fakeLeaveCalendar{byKey: map[string]*svc.LeaveCalendarEntry{}}
}

func (r *fakeLeaveCalendar) GetLeaveForAgentDate(_ context.Context, employeeID string, date time.Time) (*svc.LeaveCalendarEntry, error) {
	key := calendarKey(employeeID, date)
	if e, ok := r.byKey[key]; ok {
		return e, nil
	}
	return nil, nil
}

type fakeHolidayCalendar struct {
	byDate map[string]*svc.HolidayCalendarEntry // "2006-01-02"
}

func newFakeHolidayCalendar() *fakeHolidayCalendar {
	return &fakeHolidayCalendar{byDate: map[string]*svc.HolidayCalendarEntry{}}
}

func (r *fakeHolidayCalendar) GetHolidayForDate(_ context.Context, date time.Time) (*svc.HolidayCalendarEntry, error) {
	key := date.Format("2006-01-02")
	if e, ok := r.byDate[key]; ok {
		return e, nil
	}
	return nil, nil
}

type fakeAttendanceCalendar struct {
	byKey map[string]*svc.AttendanceCalendarEntry // "employeeID|date"
}

func newFakeAttendanceCalendar() *fakeAttendanceCalendar {
	return &fakeAttendanceCalendar{byKey: map[string]*svc.AttendanceCalendarEntry{}}
}

func (r *fakeAttendanceCalendar) GetAttendanceForAgentDate(_ context.Context, employeeID string, date time.Time) (*svc.AttendanceCalendarEntry, error) {
	key := calendarKey(employeeID, date)
	if e, ok := r.byKey[key]; ok {
		return e, nil
	}
	return nil, nil
}

// --- helpers ---

func calendarKey(employeeID string, date time.Time) string {
	return employeeID + "|" + date.Format("2006-01-02")
}

func strp(s string) *string { return &s }

func ymd(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, rr.Body.String())
	}
	return m
}

// --- harness ---

type calHarness struct {
	router     *chi.Mux
	schedule   *fakeScheduleCalendar
	leave      *fakeLeaveCalendar
	holiday    *fakeHolidayCalendar
	attendance *fakeAttendanceCalendar
	principal  auth.Principal
}

func newCalHarness(t *testing.T, role auth.Role, employeeID string) *calHarness {
	t.Helper()
	srepo := newFakeScheduleCalendar()
	lrepo := newFakeLeaveCalendar()
	hrepo := newFakeHolidayCalendar()
	arepo := newFakeAttendanceCalendar()

	calSvc := svc.NewAgentCalendarService(srepo, lrepo, hrepo, arepo)
	handler := calendarhandler.NewAgentCalendarHandler(calSvc)

	h := &calHarness{
		schedule:   srepo,
		leave:      lrepo,
		holiday:    hrepo,
		attendance: arepo,
		principal: auth.Principal{
			UserID:     "SWP-USR-0001",
			Role:       role,
			EmployeeID: employeeID,
		},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), h.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleShiftLeader, auth.RoleHRAdmin, auth.RoleSuperAdmin))
		r.Get("/me/calendar", handler.GetAgentCalendar)
	})

	h.router = r
	return h
}

func (h *calHarness) do(method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// GET /me/calendar — response shape
// ---------------------------------------------------------------------------

func TestAgentCalendar_Shape_200(t *testing.T) {
	h := newCalHarness(t, auth.RoleAgent, "SWP-EMP-1042")
	h.schedule.byKey[calendarKey("SWP-EMP-1042", ymd(2026, 6, 1))] = &svc.ScheduleCalendarEntry{
		ID:       "SWP-SCH-5001",
		Status:   "PUBLISHED",
		IsDayOff: false,
	}
	h.attendance.byKey[calendarKey("SWP-EMP-1042", ymd(2026, 6, 2))] = &svc.AttendanceCalendarEntry{
		ID:     "SWP-ATT-9001",
		Status: "PRESENT",
	}

	rr := h.do("GET", "/me/calendar?year=2026&month=6")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data missing/not an array: %T", body["data"])
	}
	if len(data) != 30 {
		t.Errorf("data length = %d, want 30 (days in June 2026)", len(data))
	}
	day1 := data[0].(map[string]any)
	if day1["date"] != "2026-06-01" {
		t.Errorf("day[0].date = %v, want 2026-06-01", day1["date"])
	}
	if _, ok := day1["schedule"]; !ok {
		t.Errorf("day[0] missing schedule key")
	}
	if day1["schedule"] == nil {
		t.Errorf("day[0].schedule should not be nil (seeded)")
	}
	day2 := data[1].(map[string]any)
	if _, ok := day2["attendance"]; !ok {
		t.Errorf("day[1] missing attendance key")
	}
	if day2["attendance"] == nil {
		t.Errorf("day[1].attendance should not be nil (seeded)")
	}
}

func TestAgentCalendar_LeaveDay_200(t *testing.T) {
	h := newCalHarness(t, auth.RoleAgent, "SWP-EMP-1042")
	start := ymd(2026, 6, 10)
	end := ymd(2026, 6, 12)
	for d := 10; d <= 12; d++ {
		h.leave.byKey[calendarKey("SWP-EMP-1042", ymd(2026, 6, d))] = &svc.LeaveCalendarEntry{
			LeaveRequestID: "SWP-LR-8001",
			LeaveTypeName:  "Cuti Tahunan",
			LeaveTypeCode:  "ANNUAL",
			StartDate:      start,
			EndDate:        end,
			Status:         "APPROVED",
		}
	}

	rr := h.do("GET", "/me/calendar?year=2026&month=6")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data := body["data"].([]any)
	day10 := data[9].(map[string]any) // 0-indexed: June 10
	if day10["leave"] == nil {
		t.Fatalf("day 2026-06-10 should have leave block")
	}
	lev := day10["leave"].(map[string]any)
	if lev["leave_request_id"] != "SWP-LR-8001" {
		t.Errorf("leave_request_id = %v, want SWP-LR-8001", lev["leave_request_id"])
	}
	if lev["leave_type_code"] != "ANNUAL" {
		t.Errorf("leave_type_code = %v, want ANNUAL", lev["leave_type_code"])
	}
}

func TestAgentCalendar_HolidayDay_200(t *testing.T) {
	h := newCalHarness(t, auth.RoleAgent, "SWP-EMP-1042")
	h.holiday.byDate["2026-06-17"] = &svc.HolidayCalendarEntry{
		ID:   "SWP-HOL-9001",
		Name: "Hari Raya Idul Adha",
	}

	rr := h.do("GET", "/me/calendar?year=2026&month=6")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data := body["data"].([]any)
	day17 := data[16].(map[string]any) // 0-indexed: June 17
	if day17["holiday"] == nil {
		t.Fatalf("day 2026-06-17 should have holiday block")
	}
	hol := day17["holiday"].(map[string]any)
	if hol["id"] != "SWP-HOL-9001" {
		t.Errorf("holiday id = %v, want SWP-HOL-9001", hol["id"])
	}
	if hol["name"] != "Hari Raya Idul Adha" {
		t.Errorf("holiday name = %v, want Hari Raya Idul Adha", hol["name"])
	}
}

func TestAgentCalendar_Unauthenticated_401(t *testing.T) {
	h := newCalHarness(t, auth.RoleAgent, "SWP-EMP-1042")

	r := chi.NewRouter()
	r.Get("/me/calendar", calendarhandler.NewAgentCalendarHandler(
		svc.NewAgentCalendarService(newFakeScheduleCalendar(), newFakeLeaveCalendar(), newFakeHolidayCalendar(), newFakeAttendanceCalendar()),
	).GetAgentCalendar)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/me/calendar?year=2026&month=6", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
	_ = h // suppress unused warning
}

func TestAgentCalendar_DefaultMonth_200(t *testing.T) {
	h := newCalHarness(t, auth.RoleAgent, "SWP-EMP-1042")

	rr := h.do("GET", "/me/calendar")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data missing/not an array: %T", body["data"])
	}
	// Should default to the current year/month (at least 28 days).
	if len(data) < 28 {
		t.Errorf("data length = %d, want at least 28 (a full month)", len(data))
	}
	firstDay := data[0].(map[string]any)
	if _, ok := firstDay["date"]; !ok {
		t.Errorf("first day missing date key")
	}
}
