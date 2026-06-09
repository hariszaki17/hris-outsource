// Package scheduling — shift-time propagation (F4.1 SM-2 ripple). When a shift
// master's start_time / end_time / cross_midnight change, the snapshot times on
// its FUTURE schedule_entries are re-synced, with attendance-driven freezing:
//
//   - no attendance (no check-in)      → re-sync start, end, and cross_midnight.
//   - checked in, NOT checked out      → FREEZE start; re-sync end; recompute
//     cross_midnight from the KEPT start + new end; ALSO push the open
//     attendance row's shift_end_at forward (work_date + new end, +1 day when
//     the recomputed cross applies). shift_start_at is left frozen.
//   - checked out                      → skip entirely (fully frozen).
//
// The per-row decision is a pure function (propagationDecision) so it is
// unit-testable without a DB; the service loop applies it + does the
// Asia/Jakarta shift_end_at timestamptz math (reusing clock.sql's wall-clock
// model). Only FUTURE entries (work_date >= today in Asia/Jakarta) are touched —
// the DB query enforces that; past/day-off/cancelled/deleted entries never enter
// the candidate set.
package scheduling

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// PropagationCandidate is one future schedule_entry linked to the edited master,
// with its (optional) attendance instants. Mirrors ListPropagationCandidatesRow.
type PropagationCandidate struct {
	EntryID       string
	WorkDate      time.Time
	StartTime     *string // current snapshot start (HH:MM)
	EndTime       *string // current snapshot end (HH:MM)
	CrossMidnight bool
	CheckInAt     *time.Time // attendance check-in (nil = no attendance row)
	CheckOutAt    *time.Time // attendance check-out (nil = not checked out)
}

// propagationAction is the resolved per-row outcome.
type propagationAction struct {
	skip bool // checked out → leave the row entirely alone

	// New snapshot for the entry (start is the KEPT value when frozen).
	newStart string
	newEnd   string
	newCross bool

	// syncOpenAttendance is true when the row is mid-shift (checked in, not out):
	// the open attendance row's shift_end_at must be pushed to the new end.
	syncOpenAttendance bool
}

// propagationDecision is the pure per-row rule (no DB, no clock). Given a
// candidate and the master's new window, it returns the action to apply.
//
//   - checked out (CheckOutAt != nil)                → skip.
//   - checked in, not out (CheckInAt != nil)         → freeze start (keep cand
//     start when present), set end=newEnd, cross=(end<=start), and flag the
//     open-attendance shift_end_at sync.
//   - no attendance (CheckInAt == nil)               → start=newStart,
//     end=newEnd, cross=newCross.
//
// newCross is the caller-supplied cross for the no-attendance case (mirrors the
// shift-master crossMidnight derivation); the frozen-start case recomputes cross
// from the effective (kept) start vs newEnd because the start did not move.
func propagationDecision(c PropagationCandidate, newStart, newEnd string, newCross bool) propagationAction {
	// Checked out → fully frozen.
	if c.CheckOutAt != nil {
		return propagationAction{skip: true}
	}
	// Checked in, not out → freeze start, re-sync end, recompute cross from the
	// kept start. A candidate's snapshot start is normally present; fall back to
	// newStart if it is somehow nil so cross stays well-defined.
	if c.CheckInAt != nil {
		keptStart := newStart
		if c.StartTime != nil {
			keptStart = *c.StartTime
		}
		return propagationAction{
			newStart:           keptStart,
			newEnd:             newEnd,
			newCross:           crossMidnight(keptStart, newEnd),
			syncOpenAttendance: true,
		}
	}
	// No attendance → full re-sync to the master's new window.
	return propagationAction{
		newStart: newStart,
		newEnd:   newEnd,
		newCross: newCross,
	}
}

// PropagationRepo is the data surface the propagation needs (a slice of the
// shift-master repo port). Reads run on the pool; the two writes take the same
// pgx.Tx as the master update so propagation is atomic with it.
type PropagationRepo interface {
	// ListPropagationCandidates fetches the future/live/non-day-off/non-cancelled
	// entries linked to masterID, with their attendance instants. `now` resolves
	// "today" in Asia/Jakarta inside the query.
	ListPropagationCandidates(ctx context.Context, masterID string, now time.Time) ([]PropagationCandidate, error)
	// UpdateScheduleEntryTimes re-syncs the three snapshot time columns on one entry.
	UpdateScheduleEntryTimes(ctx context.Context, tx pgx.Tx, entryID, startTime, endTime string, cross bool) error
	// SyncOpenAttendanceShiftEnd pushes the open attendance row's shift_end_at
	// (E4→E5 cross-epic write). No-op when the entry has no open attendance row.
	SyncOpenAttendanceShiftEnd(ctx context.Context, tx pgx.Tx, scheduleID string, shiftEndAt time.Time) error
}

// jakarta returns the Asia/Jakarta location (falls back to a fixed +07:00 zone,
// mirroring ScheduleService.today()).
func jakarta() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

// shiftEndAt computes the absolute shift-end instant for an entry: the wall-clock
// (work_date + end_time) interpreted in Asia/Jakarta, plus one day when the shift
// crosses midnight. Mirrors clock.sql GetTodayScheduleForEmployee's shift_end_at
// (work_date + end::time AT TIME ZONE 'Asia/Jakarta', + interval '1 day' when cross).
func shiftEndAt(workDate time.Time, endHHMM string, cross bool) time.Time {
	h, m := parseHHMM(endHHMM)
	loc := jakarta()
	end := time.Date(workDate.Year(), workDate.Month(), workDate.Day(), h, m, 0, 0, loc)
	if cross {
		end = end.AddDate(0, 0, 1)
	}
	return end
}

// parseHHMM splits a zero-padded "HH:MM" into hour/minute. Inputs are
// service-validated (validHHMM) before they ever reach a master/entry, so this
// is total; a malformed value degrades to 00:00 rather than panicking.
func parseHHMM(s string) (int, int) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	return h, m
}

// propagateShiftMasterTimeChange re-syncs the future schedule_entries linked to
// masterID after its window changed, applying the freeze rule per row. Runs
// inside the SAME tx as the master update (atomic). `now` is the service clock
// (Asia/Jakarta "today" cutoff is resolved in the query).
func (s *ShiftMasterService) propagateShiftMasterTimeChange(
	ctx context.Context, tx pgx.Tx, masterID, newStart, newEnd string, newCross bool, now time.Time,
) error {
	cands, err := s.prop.ListPropagationCandidates(ctx, masterID, now)
	if err != nil {
		return err
	}
	for _, c := range cands {
		act := propagationDecision(c, newStart, newEnd, newCross)
		if act.skip {
			continue
		}
		if uerr := s.prop.UpdateScheduleEntryTimes(ctx, tx, c.EntryID, act.newStart, act.newEnd, act.newCross); uerr != nil {
			return uerr
		}
		if act.syncOpenAttendance {
			// Push the open attendance row's shift_end_at to the new end. The
			// recomputed cross (from the kept start) decides the +1 day.
			endAt := shiftEndAt(c.WorkDate, act.newEnd, act.newCross)
			if serr := s.prop.SyncOpenAttendanceShiftEnd(ctx, tx, c.EntryID, endAt); serr != nil {
				return serr
			}
		}
	}
	return nil
}
