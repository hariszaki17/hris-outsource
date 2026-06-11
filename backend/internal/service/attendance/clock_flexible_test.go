// Package attendance — unit tests for the F5.1 flexible check-in decision in
// ClockService.ClockIn: a stale forgotten clock-out is auto-closed (at its computed
// shift_end) so a new check-in proceeds, while an open record still within its checkout
// window blocks as ALREADY_CLOCKED_IN. Uses a fake ClockRepository + the shared
// sweepFakeRunner/sweepFakeTx (audit.Record Exec is a no-op). No Postgres.
package attendance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// fakeClockRepo is a minimal ClockRepository for the flexible-check-in decision.
type fakeClockRepo struct {
	openID    string                    // id returned by GetOpenAttendance (empty ⇒ none open)
	records   map[string]att.Attendance // id → record served by GetAttendance
	autoClose *AutoCloseRow             // captured AutoCloseAttendance call
	inserted  *ClockInRow               // captured ClockIn insert
	newID     string                    // id the insert returns
}

func (f *fakeClockRepo) GetActivePlacement(_ context.Context, _ string) (PlacementInfo, bool, error) {
	return PlacementInfo{
		PlacementID: "SWP-PL-0001", CompanyID: "SWP-CMP-0021",
		SiteID: "SWP-SITE-0001", PositionID: "SWP-POS-014", ServiceLine: "parking",
	}, true, nil
}

// GetSite returns no coordinates → geofence skipped (keeps the test focused).
func (f *fakeClockRepo) GetSite(_ context.Context, _ string) (*float64, *float64, int, bool, error) {
	return nil, nil, 0, true, nil
}

// GetTodaySchedule: unscheduled (the auto-close test forgot to clock out a prior day).
func (f *fakeClockRepo) GetTodaySchedule(_ context.Context, _ string, _ time.Time) (string, time.Time, time.Time, bool, error) {
	return "", time.Time{}, time.Time{}, false, nil
}

func (f *fakeClockRepo) GetOpenAttendance(_ context.Context, _ string) (string, bool, error) {
	if f.openID == "" {
		return "", false, nil
	}
	return f.openID, true, nil
}

func (f *fakeClockRepo) ClockIn(_ context.Context, _ pgx.Tx, p ClockInRow) (string, bool, error) {
	f.inserted = &p
	return f.newID, true, nil
}

func (f *fakeClockRepo) ClockOut(_ context.Context, _ pgx.Tx, _ ClockOutRow) (string, error) {
	return "", errors.New("ClockOut unused")
}

func (f *fakeClockRepo) AutoCloseAttendance(_ context.Context, _ pgx.Tx, p AutoCloseRow) (string, bool, error) {
	f.autoClose = &p
	return p.ID, true, nil
}

func (f *fakeClockRepo) GetAttendance(_ context.Context, id string) (att.Attendance, error) {
	rec, ok := f.records[id]
	if !ok {
		return att.Attendance{}, errors.New("record not found: " + id)
	}
	return rec, nil
}

// agentCtx returns a context carrying an agent principal with an employee id.
func agentCtx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "SWP-USR-1", EmployeeID: "SWP-EMP-0001", Role: auth.RoleAgent,
	})
}

func clockInReq() ClockInParams {
	return ClockInParams{Lat: -6.2, Lng: 106.8, GPSAvailable: true, WFO: true}
}

func TestClockIn_Flexible(t *testing.T) {
	t.Run("forgotten check-out, no schedule, past window → check-in succeeds, stale row auto-closed at check_in+18h", func(t *testing.T) {
		staleCheckIn := time.Date(2026, 6, 9, 8, 0, 0, 0, time.UTC) // yesterday 08:00, no shift snapshot
		now := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)         // >18h+4h later → window elapsed
		repo := &fakeClockRepo{
			openID: "SWP-ATT-STALE",
			newID:  "SWP-ATT-NEW",
			records: map[string]att.Attendance{
				"SWP-ATT-STALE": {ID: "SWP-ATT-STALE", CheckInAt: &staleCheckIn}, // open, unscheduled
				"SWP-ATT-NEW":   {ID: "SWP-ATT-NEW", CheckInAt: &now},            // reread after insert
			},
		}
		svc := NewClockService(repo, sweepFakeRunner{})
		svc.SetClock(fixedClock(now))

		rec, autoClosed, err := svc.ClockIn(agentCtx(), clockInReq())
		if err != nil {
			t.Fatalf("ClockIn: %v", err)
		}
		if !autoClosed {
			t.Error("autoClosedPrevious = false, want true")
		}
		if rec.ID != "SWP-ATT-NEW" {
			t.Errorf("returned id = %q, want SWP-ATT-NEW", rec.ID)
		}
		if repo.inserted == nil {
			t.Fatal("no clock-in insert happened")
		}
		if repo.autoClose == nil {
			t.Fatal("stale row was not auto-closed")
		}
		wantClose := staleCheckIn.Add(att.FallbackShiftHours) // check_in + 18h
		if !repo.autoClose.CheckOutAt.Equal(wantClose) {
			t.Errorf("auto-close check_out = %v, want %v (check_in+18h)", repo.autoClose.CheckOutAt, wantClose)
		}
		if repo.autoClose.Status != string(att.StatusIncomplete) {
			t.Errorf("auto-close status = %q, want INCOMPLETE", repo.autoClose.Status)
		}
		if !hasFlag(repo.autoClose.Flags, string(att.FlagAutoClosed)) {
			t.Errorf("auto-close flags = %v, want AUTO_CLOSED present", repo.autoClose.Flags)
		}
	})

	t.Run("open row within window → ALREADY_CLOCKED_IN, no auto-close, no insert", func(t *testing.T) {
		checkIn := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
		shiftEnd := time.Date(2026, 6, 10, 16, 0, 0, 0, time.UTC) // 08:00→16:00
		now := time.Date(2026, 6, 10, 17, 0, 0, 0, time.UTC)      // 16:00 + 4h grace = 20:00 → within
		repo := &fakeClockRepo{
			openID: "SWP-ATT-OPEN",
			records: map[string]att.Attendance{
				"SWP-ATT-OPEN": {ID: "SWP-ATT-OPEN", CheckInAt: &checkIn, ShiftStartAt: &checkIn, ShiftEndAt: &shiftEnd},
			},
		}
		svc := NewClockService(repo, sweepFakeRunner{})
		svc.SetClock(fixedClock(now))

		_, _, err := svc.ClockIn(agentCtx(), clockInReq())
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Code != "ALREADY_CLOCKED_IN" {
			t.Fatalf("err = %v, want ALREADY_CLOCKED_IN", err)
		}
		if repo.autoClose != nil {
			t.Error("auto-close happened, want none (still within window)")
		}
		if repo.inserted != nil {
			t.Error("insert happened, want none")
		}
	})

	t.Run("no open row → normal check-in, autoClosedPrevious=false", func(t *testing.T) {
		now := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
		repo := &fakeClockRepo{
			newID:   "SWP-ATT-NEW",
			records: map[string]att.Attendance{"SWP-ATT-NEW": {ID: "SWP-ATT-NEW", CheckInAt: &now}},
		}
		svc := NewClockService(repo, sweepFakeRunner{})
		svc.SetClock(fixedClock(now))

		_, autoClosed, err := svc.ClockIn(agentCtx(), clockInReq())
		if err != nil {
			t.Fatalf("ClockIn: %v", err)
		}
		if autoClosed {
			t.Error("autoClosedPrevious = true, want false")
		}
		if repo.autoClose != nil {
			t.Error("auto-close happened, want none")
		}
	})
}

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}
