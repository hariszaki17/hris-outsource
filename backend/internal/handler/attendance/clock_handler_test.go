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
	openID    string
	records   map[string]att.Attendance
	newID     string
	autoClose *svc.AutoCloseRow
	inserted  bool
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
	return "", nil
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

func postClockIn(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"lat":-6.2,"lng":106.8,"gps_available":true}`
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
}
