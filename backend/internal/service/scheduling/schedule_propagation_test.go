package scheduling

import (
	"testing"
	"time"
)

func sp(s string) *string       { return &s }
func tt(t time.Time) *time.Time { return &t }

// TestPropagationDecision exercises the pure per-row freeze rule (no DB/clock).
func TestPropagationDecision(t *testing.T) {
	in := time.Date(2026, 6, 10, 0, 5, 0, 0, time.UTC)
	out := time.Date(2026, 6, 10, 8, 5, 0, 0, time.UTC)

	tests := []struct {
		name string
		cand PropagationCandidate
		newS string
		newE string
		newX bool
		want propagationAction
	}{
		{
			name: "no attendance → full re-sync",
			cand: PropagationCandidate{StartTime: sp("07:00"), EndTime: sp("15:00")},
			newS: "08:00", newE: "16:00", newX: false,
			want: propagationAction{newStart: "08:00", newEnd: "16:00", newCross: false},
		},
		{
			name: "no attendance, new window crosses midnight → cross propagated",
			cand: PropagationCandidate{StartTime: sp("07:00"), EndTime: sp("15:00")},
			newS: "23:00", newE: "07:00", newX: true,
			want: propagationAction{newStart: "23:00", newEnd: "07:00", newCross: true},
		},
		{
			name: "checked in, not out → freeze start, re-sync end, sync attendance",
			cand: PropagationCandidate{StartTime: sp("07:00"), EndTime: sp("15:00"), CheckInAt: tt(in)},
			newS: "08:00", newE: "16:00", newX: false,
			want: propagationAction{newStart: "07:00", newEnd: "16:00", newCross: false, syncOpenAttendance: true},
		},
		{
			name: "checked in, end wraps before frozen start → cross recomputed true",
			cand: PropagationCandidate{StartTime: sp("07:00"), EndTime: sp("15:00"), CheckInAt: tt(in)},
			newS: "08:00", newE: "06:00", newX: false, // caller newX irrelevant for frozen-start
			want: propagationAction{newStart: "07:00", newEnd: "06:00", newCross: true, syncOpenAttendance: true},
		},
		{
			name: "checked out → skip",
			cand: PropagationCandidate{StartTime: sp("07:00"), EndTime: sp("15:00"), CheckInAt: tt(in), CheckOutAt: tt(out)},
			newS: "08:00", newE: "16:00", newX: false,
			want: propagationAction{skip: true},
		},
		{
			name: "checked in, nil snapshot start → falls back to new start",
			cand: PropagationCandidate{EndTime: sp("15:00"), CheckInAt: tt(in)},
			newS: "08:00", newE: "16:00", newX: false,
			want: propagationAction{newStart: "08:00", newEnd: "16:00", newCross: false, syncOpenAttendance: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := propagationDecision(tc.cand, tc.newS, tc.newE, tc.newX)
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestShiftEndAt checks the Asia/Jakarta (work_date + end) timestamptz math,
// including the cross-midnight +1 day. Asserted in UTC (WIB = UTC+7).
func TestShiftEndAt(t *testing.T) {
	wd := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)

	// 16:00 WIB == 09:00 UTC same day.
	got := shiftEndAt(wd, "16:00", false)
	want := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("same-day: got %s, want %s", got.UTC(), want)
	}

	// 06:00 WIB next day == 23:00 UTC on the work_date.
	gotX := shiftEndAt(wd, "06:00", true)
	wantX := time.Date(2026, 6, 10, 23, 0, 0, 0, time.UTC)
	if !gotX.Equal(wantX) {
		t.Fatalf("cross-midnight: got %s, want %s", gotX.UTC(), wantX)
	}
}
