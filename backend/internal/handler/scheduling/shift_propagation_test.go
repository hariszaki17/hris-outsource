// Package scheduling_test — shift-time propagation (F4.1 SM-2 ripple → F4.2
// entries, with E5 attendance freezing). When a master's window changes via
// PATCH /shift-masters/{id}, its FUTURE schedule_entries are re-synced unless an
// attendance row has frozen them. Tests drive the real ShiftMasterService over
// the in-memory fakes (newHarness) and assert the per-row outcome.
//
// Branches covered:
//
//	(a) no attendance              → start + end both re-synced.
//	(b) checked in, not out        → start FROZEN, end re-synced, open
//	    attendance.shift_end_at pushed forward.
//	(c) checked out                → entry untouched (fully frozen).
//	(d) past-dated (work_date<today)→ untouched.
//	(e) is_day_off                 → untouched.
//
// Plus: name-only PATCH does NOT ripple; cross_midnight re-derivation.
package scheduling_test

import (
	"testing"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// patchMasterTimes edits a master's window via the real PATCH path (hr_admin).
func patchMasterTimes(t *testing.T, h *harness, id, start, end string) {
	t.Helper()
	rr := h.do("PATCH", "/shift-masters/"+id, map[string]any{
		"start_time": start,
		"end_time":   end,
	})
	if rr.Code != 200 {
		t.Fatalf("PATCH shift-master: want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func tp(v time.Time) *time.Time { return &v }

// fixedNow is 2026-06-04 (12:00 WIB). future = on/after that date; past = before.
var (
	futureDate = ymd(2026, 6, 10)
	pastDate   = ymd(2026, 6, 1)
)

// (a) no attendance → both start and end re-synced to the new master window.
func TestPropagation_NoAttendance_ResyncsBoth(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)
	h.seedEntry("SWP-SCH-001", "SWP-EMP-1", "SWP-SHF-001", futureDate, "07:00", "15:00", false, "SCHEDULED")

	patchMasterTimes(t, h, "SWP-SHF-001", "08:00", "16:00")

	e := h.schedule.entries["SWP-SCH-001"]
	if *e.StartTime != "08:00" || *e.EndTime != "16:00" {
		t.Fatalf("want start/end 08:00/16:00, got %s/%s", *e.StartTime, *e.EndTime)
	}
	if e.CrossMidnight {
		t.Fatalf("cross_midnight should stay false for 08:00–16:00")
	}
}

// (b) checked in, not out → start frozen, end re-synced, open attendance
// shift_end_at pushed to the new end.
func TestPropagation_CheckedInNotOut_FreezesStart_SyncsAttendance(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)
	h.seedEntry("SWP-SCH-001", "SWP-EMP-1", "SWP-SHF-001", futureDate, "07:00", "15:00", false, "SCHEDULED")
	checkIn := time.Date(2026, 6, 10, 0, 5, 0, 0, time.UTC) // arbitrary clock-in instant
	oldEnd := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	h.seedAttendance("SWP-SCH-001", tp(checkIn), nil, tp(oldEnd))

	patchMasterTimes(t, h, "SWP-SHF-001", "08:00", "16:00")

	e := h.schedule.entries["SWP-SCH-001"]
	if *e.StartTime != "07:00" {
		t.Fatalf("start should be FROZEN at 07:00, got %s", *e.StartTime)
	}
	if *e.EndTime != "16:00" {
		t.Fatalf("end should re-sync to 16:00, got %s", *e.EndTime)
	}
	att := h.schedule.attendance["SWP-SCH-001"]
	if att.shiftEndAt == nil {
		t.Fatalf("open attendance shift_end_at should be pushed, got nil")
	}
	// 16:00 WIB on 2026-06-10 == 09:00 UTC.
	wantEnd := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	if !att.shiftEndAt.Equal(wantEnd) {
		t.Fatalf("shift_end_at: want %s, got %s", wantEnd, att.shiftEndAt.UTC())
	}
}

// (b') checked in, not out, new end wraps past the frozen start → cross_midnight
// recomputed true and shift_end_at gets the +1 day.
func TestPropagation_CheckedInNotOut_CrossMidnightRecompute(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)
	h.seedEntry("SWP-SCH-001", "SWP-EMP-1", "SWP-SHF-001", futureDate, "07:00", "15:00", false, "SCHEDULED")
	checkIn := time.Date(2026, 6, 10, 0, 5, 0, 0, time.UTC)
	h.seedAttendance("SWP-SCH-001", tp(checkIn), nil, nil)

	// End moves to 06:00 — earlier than the frozen 07:00 start → crosses midnight.
	patchMasterTimes(t, h, "SWP-SHF-001", "08:00", "06:00")

	e := h.schedule.entries["SWP-SCH-001"]
	if *e.StartTime != "07:00" {
		t.Fatalf("start frozen at 07:00, got %s", *e.StartTime)
	}
	if !e.CrossMidnight {
		t.Fatalf("cross_midnight should recompute true (06:00 <= 07:00)")
	}
	att := h.schedule.attendance["SWP-SCH-001"]
	// 06:00 WIB on 2026-06-11 (next day) == 23:00 UTC on 2026-06-10.
	wantEnd := time.Date(2026, 6, 10, 23, 0, 0, 0, time.UTC)
	if att.shiftEndAt == nil || !att.shiftEndAt.Equal(wantEnd) {
		t.Fatalf("shift_end_at: want %s, got %v", wantEnd, att.shiftEndAt)
	}
}

// (c) checked out → entry fully frozen (untouched), no attendance write.
func TestPropagation_CheckedOut_Untouched(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)
	h.seedEntry("SWP-SCH-001", "SWP-EMP-1", "SWP-SHF-001", futureDate, "07:00", "15:00", false, "SCHEDULED")
	checkIn := time.Date(2026, 6, 10, 0, 5, 0, 0, time.UTC)
	checkOut := time.Date(2026, 6, 10, 8, 5, 0, 0, time.UTC)
	oldEnd := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	h.seedAttendance("SWP-SCH-001", tp(checkIn), tp(checkOut), tp(oldEnd))

	patchMasterTimes(t, h, "SWP-SHF-001", "08:00", "16:00")

	e := h.schedule.entries["SWP-SCH-001"]
	if *e.StartTime != "07:00" || *e.EndTime != "15:00" {
		t.Fatalf("checked-out entry must be untouched, got %s/%s", *e.StartTime, *e.EndTime)
	}
	att := h.schedule.attendance["SWP-SCH-001"]
	if att.shiftEndAt == nil || !att.shiftEndAt.Equal(oldEnd) {
		t.Fatalf("checked-out attendance shift_end_at must be untouched, got %v", att.shiftEndAt)
	}
}

// (d) past-dated entry → never a candidate.
func TestPropagation_PastEntry_Untouched(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)
	h.seedEntry("SWP-SCH-001", "SWP-EMP-1", "SWP-SHF-001", pastDate, "07:00", "15:00", false, "SCHEDULED")

	patchMasterTimes(t, h, "SWP-SHF-001", "08:00", "16:00")

	e := h.schedule.entries["SWP-SCH-001"]
	if *e.StartTime != "07:00" || *e.EndTime != "15:00" {
		t.Fatalf("past entry must be untouched, got %s/%s", *e.StartTime, *e.EndTime)
	}
}

// (e) is_day_off entry → never a candidate.
func TestPropagation_DayOff_Untouched(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)
	h.seedEntry("SWP-SCH-001", "SWP-EMP-1", "SWP-SHF-001", futureDate, "07:00", "15:00", true, "SCHEDULED")

	patchMasterTimes(t, h, "SWP-SHF-001", "08:00", "16:00")

	e := h.schedule.entries["SWP-SCH-001"]
	if *e.StartTime != "07:00" || *e.EndTime != "15:00" {
		t.Fatalf("day-off entry must be untouched, got %s/%s", *e.StartTime, *e.EndTime)
	}
}

// CANCELLED_BY_LEAVE entry → never a candidate.
func TestPropagation_Cancelled_Untouched(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)
	h.seedEntry("SWP-SCH-001", "SWP-EMP-1", "SWP-SHF-001", futureDate, "07:00", "15:00", false, "CANCELLED_BY_LEAVE")

	patchMasterTimes(t, h, "SWP-SHF-001", "08:00", "16:00")

	e := h.schedule.entries["SWP-SCH-001"]
	if *e.StartTime != "07:00" || *e.EndTime != "15:00" {
		t.Fatalf("cancelled entry must be untouched, got %s/%s", *e.StartTime, *e.EndTime)
	}
}

// A name-only PATCH (window unchanged) must NOT ripple.
func TestPropagation_NameOnlyPatch_NoRipple(t *testing.T) {
	h := newHarness(t, auth.RoleHRAdmin, "")
	h.seedMaster("SWP-SHF-001", "Pagi", "07:00", "15:00", nil, true)
	h.seedEntry("SWP-SCH-001", "SWP-EMP-1", "SWP-SHF-001", futureDate, "07:00", "15:00", false, "SCHEDULED")

	rr := h.do("PATCH", "/shift-masters/SWP-SHF-001", map[string]any{"name": "Pagi Baru"})
	if rr.Code != 200 {
		t.Fatalf("PATCH name: want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	e := h.schedule.entries["SWP-SCH-001"]
	if *e.StartTime != "07:00" || *e.EndTime != "15:00" {
		t.Fatalf("name-only edit must not move entry times, got %s/%s", *e.StartTime, *e.EndTime)
	}
}
