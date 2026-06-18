// Package attendance_test — HTTP contract tests for the agent clock-in endpoint
// (F5.1 flexible check-in). Mounts the REAL ClockHandler + ClockService over a fake
// ClockRepository on a chi router with an agent principal, and asserts the additive
// JSON contract: auto_closed_previous + message + data.can_check_out.
//
// Uses the REAL wall clock (no SetClock): fixtures are expressed as offsets from
// time.Now() so the service decision clock and the DTO's time.Now() agree. Offsets are
// large (hours) → no flakiness.
package attendance_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	attendancehandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// fakeClockRepo implements svc.ClockRepository for the clock-in handler test.
type fakeClockRepo struct {
	openID        string
	records       map[string]att.Attendance
	newID         string
	autoClose     *svc.AutoCloseRow
	inserted      bool
	activityCount int64  // CountActivities result — the clock-out gate (AA-7)
	clockedOut    bool   // set when ClockOut is invoked
	empType       string // "" ⇒ INTERNAL (unscheduled clock-in allowed)
}

// EmployeeType defaults to INTERNAL so the no-shift tests below clock in unscheduled; a
// FIELD case sets empType to assert the NO_SCHEDULED_SHIFT block.
func (f *fakeClockRepo) EmployeeType(_ context.Context, _ string) (string, error) {
	if f.empType == "" {
		return "INTERNAL", nil
	}
	return f.empType, nil
}

func (f *fakeClockRepo) GetActivePlacement(_ context.Context, _ string) (svc.PlacementInfo, bool, error) {
	return svc.PlacementInfo{
		PlacementID: "SWP-PL-0001", CompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", Position: "Petugas Parkir",
	}, true, nil
}
func (f *fakeClockRepo) IsOnApprovedLeave(_ context.Context, _ string, _ time.Time) (bool, error) {
	return false, nil
}
func (f *fakeClockRepo) GetSite(_ context.Context, _ string) (*float64, *float64, int, bool, error) {
	return nil, nil, 0, true, nil // no coords → geofence skipped
}
func (f *fakeClockRepo) GetTodaySchedule(_ context.Context, _ string, _ time.Time) (string, time.Time, time.Time, bool, error) {
	return "", time.Time{}, time.Time{}, false, nil // unscheduled
}
func (f *fakeClockRepo) GetOpenAttendance(_ context.Context, _ string) (string, bool, error) {
	if f.openID == "" {
		return "", false, nil
	}
	return f.openID, true, nil
}
func (f *fakeClockRepo) ClockIn(_ context.Context, _ pgx.Tx, _ svc.ClockInRow) (string, bool, error) {
	f.inserted = true
	return f.newID, true, nil
}
func (f *fakeClockRepo) ClockOut(_ context.Context, _ pgx.Tx, _ svc.ClockOutRow) (string, error) {
	f.clockedOut = true
	return f.openID, nil
}
func (f *fakeClockRepo) CountActivities(_ context.Context, _ string) (int64, error) {
	return f.activityCount, nil
}
func (f *fakeClockRepo) AutoCloseAttendance(_ context.Context, _ pgx.Tx, p svc.AutoCloseRow) (string, bool, error) {
	f.autoClose = &p
	return p.ID, true, nil
}
func (f *fakeClockRepo) GetAttendance(_ context.Context, id string) (att.Attendance, error) {
	return f.records[id], nil
}

// clockHarness mounts ClockHandler.ClockIn over the fake repo with an agent principal.
func clockHarness(repo *fakeClockRepo) http.Handler {
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
	r.Post("/attendance:clock-in", h.ClockIn)
	return r
}

// postClockIn posts a default MOBILE-with-photo clock-in (the normal happy path now
// that mobile requires a selfie). Use postClockInBody for platform/photo variants.
func postClockIn(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	return postClockInBody(t, h, `{"lat":-6.2,"lng":106.8,"gps_available":true,"photo_id":"SWP-FILE-1"}`)
}

func postClockInBody(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/attendance:clock-in", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestClockInHandler_Contract(t *testing.T) {
	t.Run("normal check-in: auto_closed_previous=false, normal message, can_check_out=true", func(t *testing.T) {
		now := time.Now()
		repo := &fakeClockRepo{
			newID:   "SWP-ATT-NEW",
			records: map[string]att.Attendance{"SWP-ATT-NEW": {ID: "SWP-ATT-NEW", CheckInAt: &now}},
		}
		rr := postClockIn(t, clockHarness(repo))

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
		}
		body := decodeBody(t, rr)
		if body["auto_closed_previous"] != false {
			t.Errorf("auto_closed_previous = %v, want false", body["auto_closed_previous"])
		}
		if msg := strOf(body["message"]); msg != "Berhasil Check In." {
			t.Errorf("message = %q, want \"Berhasil Check In.\"", msg)
		}
		data, _ := body["data"].(map[string]any)
		if data["can_check_out"] != true {
			t.Errorf("data.can_check_out = %v, want true (fresh open row)", data["can_check_out"])
		}
		if repo.autoClose != nil {
			t.Error("auto-close happened, want none")
		}
	})

	t.Run("forgotten clock-out (stale, past window): auto_closed_previous=true, system message, stale row closed", func(t *testing.T) {
		now := time.Now()
		stale := now.Add(-48 * time.Hour) // open, unscheduled → window = stale+18h+4h, long elapsed
		repo := &fakeClockRepo{
			openID: "SWP-ATT-STALE",
			newID:  "SWP-ATT-NEW",
			records: map[string]att.Attendance{
				"SWP-ATT-STALE": {ID: "SWP-ATT-STALE", CheckInAt: &stale},
				"SWP-ATT-NEW":   {ID: "SWP-ATT-NEW", CheckInAt: &now},
			},
		}
		rr := postClockIn(t, clockHarness(repo))

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
		}
		body := decodeBody(t, rr)
		if body["auto_closed_previous"] != true {
			t.Errorf("auto_closed_previous = %v, want true", body["auto_closed_previous"])
		}
		if msg := strOf(body["message"]); !strings.Contains(msg, "ditutup otomatis") {
			t.Errorf("message = %q, want the auto-close notice", msg)
		}
		if repo.autoClose == nil {
			t.Fatal("stale row was not auto-closed")
		}
		if repo.autoClose.Status != string(att.StatusIncomplete) {
			t.Errorf("auto-close status = %q, want INCOMPLETE", repo.autoClose.Status)
		}
	})

	t.Run("FIELD agent with no scheduled shift is blocked (NO_SCHEDULED_SHIFT, 422)", func(t *testing.T) {
		now := time.Now()
		repo := &fakeClockRepo{
			empType: "FIELD", // FIELD + unscheduled ⇒ blocked (EPICS §8 2026-06-18)
			newID:   "SWP-ATT-NEW",
			records: map[string]att.Attendance{"SWP-ATT-NEW": {ID: "SWP-ATT-NEW", CheckInAt: &now}},
		}
		rr := postClockIn(t, clockHarness(repo))

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422 (body: %s)", rr.Code, rr.Body.String())
		}
		if code := strOf(decodeBody(t, rr)["error"].(map[string]any)["code"]); code != "NO_SCHEDULED_SHIFT" {
			t.Errorf("error.code = %q, want NO_SCHEDULED_SHIFT", code)
		}
		if repo.inserted {
			t.Error("attendance row was inserted, want none (blocked before insert)")
		}
	})
}

// TestClockInHandler_PhotoGate asserts the 2026-06-17 platform/photo rule:
// MOBILE (default) requires photo_id → 422 PHOTO_REQUIRED when missing; WEB is exempt.
func TestClockInHandler_PhotoGate(t *testing.T) {
	now := time.Now()
	newRepo := func() *fakeClockRepo {
		return &fakeClockRepo{
			newID:   "SWP-ATT-NEW",
			records: map[string]att.Attendance{"SWP-ATT-NEW": {ID: "SWP-ATT-NEW", CheckInAt: &now}},
		}
	}

	t.Run("mobile, no photo: 422 PHOTO_REQUIRED, no record created", func(t *testing.T) {
		repo := newRepo()
		// platform omitted → defaults MOBILE; photo_id omitted.
		rr := postClockInBody(t, clockHarness(repo), `{"lat":-6.2,"lng":106.8,"gps_available":true}`)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422 (body: %s)", rr.Code, rr.Body.String())
		}
		if code := strOf(errObject(t, decodeBody(t, rr))["code"]); code != "PHOTO_REQUIRED" {
			t.Errorf("error code = %q, want PHOTO_REQUIRED", code)
		}
		if repo.inserted {
			t.Error("a record was created, want none on PHOTO_REQUIRED")
		}
	})

	t.Run("mobile, with photo: 201", func(t *testing.T) {
		repo := newRepo()
		rr := postClockInBody(t, clockHarness(repo),
			`{"lat":-6.2,"lng":106.8,"gps_available":true,"platform":"MOBILE","photo_id":"SWP-FILE-1"}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
		}
		if !repo.inserted {
			t.Error("no record created on a valid mobile clock-in")
		}
	})

	t.Run("web, no photo: 201 (exempt)", func(t *testing.T) {
		repo := newRepo()
		rr := postClockInBody(t, clockHarness(repo),
			`{"lat":-6.2,"lng":106.8,"gps_available":true,"platform":"WEB"}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
		}
		if !repo.inserted {
			t.Error("no record created on a valid web clock-in")
		}
	})
}
