// Package attendance — unit tests for the flexible check-out window logic (F5.1):
// ShiftEndTimestamp (schedule-aware + overnight snapshot + fallback) and
// IsWithinCheckoutWindow (the mobile CHECK OUT vs CHECK IN toggle hint). Pure functions
// of one record + now — no DB.
package attendance

import (
	"testing"
	"time"
)

func tp(t time.Time) *time.Time { return &t }

// openRow builds a minimal OPEN record (clocked in, not yet clocked out) with the given
// check-in and optional snapshotted shift window.
func openRow(checkIn time.Time, shiftStart, shiftEnd *time.Time) Attendance {
	return Attendance{
		CheckInAt:    &checkIn,
		CheckOutAt:   nil,
		ShiftStartAt: shiftStart,
		ShiftEndAt:   shiftEnd,
	}
}

// TestShiftEndTimestamp covers the snapshot, the start==end sentinel, and the fallback.
func TestShiftEndTimestamp(t *testing.T) {
	checkIn := time.Date(2026, 6, 10, 22, 0, 0, 0, time.UTC) // 22:00, night shift

	// Overnight snapshot: end 06:00 next day already baked into ShiftEndAt.
	end := time.Date(2026, 6, 11, 6, 0, 0, 0, time.UTC)
	start := time.Date(2026, 6, 10, 22, 0, 0, 0, time.UTC)
	if got := ShiftEndTimestamp(openRow(checkIn, &start, &end)); !got.Equal(end) {
		t.Errorf("overnight snapshot: got %v, want %v", got, end)
	}

	// No usable shift (unscheduled) → check_in + FallbackShiftHours.
	wantFallback := checkIn.Add(FallbackShiftHours)
	if got := ShiftEndTimestamp(openRow(checkIn, nil, nil)); !got.Equal(wantFallback) {
		t.Errorf("fallback: got %v, want %v", got, wantFallback)
	}

	// 00:00==00:00 sentinel (start==end) → treated as no shift → fallback.
	sentinel := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	if got := ShiftEndTimestamp(openRow(checkIn, &sentinel, &sentinel)); !got.Equal(wantFallback) {
		t.Errorf("sentinel: got %v, want %v (fallback)", got, wantFallback)
	}
}

// TestIsWithinCheckoutWindow covers the four mobile-toggle scenarios.
func TestIsWithinCheckoutWindow(t *testing.T) {
	checkIn := time.Date(2026, 6, 10, 22, 0, 0, 0, time.UTC)
	start := checkIn
	end := time.Date(2026, 6, 11, 6, 0, 0, 0, time.UTC) // overnight 22:00→06:00

	// (a) night shift, now within window (06:00 + 4h grace = 10:00) → CHECK OUT.
	nowWithin := time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)
	if !IsWithinCheckoutWindow(openRow(checkIn, &start, &end), nowWithin) {
		t.Error("night shift within window: got false, want true (CHECK OUT)")
	}

	// boundary: exactly shift_end + grace is still within (now <= end+grace).
	atBoundary := end.Add(CheckoutWindowGrace)
	if !IsWithinCheckoutWindow(openRow(checkIn, &start, &end), atBoundary) {
		t.Error("at window boundary: got false, want true (inclusive)")
	}

	// (b) night shift, now past window → CHECK IN (false).
	nowPast := time.Date(2026, 6, 11, 10, 0, 1, 0, time.UTC) // 1s past 10:00
	if IsWithinCheckoutWindow(openRow(checkIn, &start, &end), nowPast) {
		t.Error("night shift past window: got true, want false (CHECK IN)")
	}

	// (c) no schedule: fallback window = check_in + 18h + 4h grace = 22h after check-in.
	noSched := openRow(checkIn, nil, nil)
	if !IsWithinCheckoutWindow(noSched, checkIn.Add(21*time.Hour)) {
		t.Error("fallback within: got false, want true")
	}
	if IsWithinCheckoutWindow(noSched, checkIn.Add(23*time.Hour)) {
		t.Error("fallback past: got true, want false")
	}

	// (d) already checked out → never (toggle is CHECK IN).
	closed := openRow(checkIn, &start, &end)
	closed.CheckOutAt = tp(end)
	if IsWithinCheckoutWindow(closed, nowWithin) {
		t.Error("already checked out: got true, want false")
	}

	// true ABSENT (no clock-in) → never.
	absent := Attendance{CheckInAt: nil, ShiftEndAt: &end}
	if IsWithinCheckoutWindow(absent, nowWithin) {
		t.Error("no clock-in: got true, want false")
	}
}
