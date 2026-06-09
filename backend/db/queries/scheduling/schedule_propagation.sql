-- E4 shift-time propagation (F4.1 SM-2 ripple → F4.2 entries, with E5 attendance
-- freezing). When a shift master's start_time/end_time/cross_midnight change, the
-- snapshot times on its FUTURE schedule_entries are re-synced — UNLESS an
-- attendance row has already frozen the entry (checked-in freezes the start;
-- checked-out freezes everything). The per-row decision + cross_midnight + the
-- Asia/Jakarta shift_end_at timestamptz math live in Go (schedule_propagation.go)
-- for clarity/testability; these queries are the row-fetch + the two focused
-- writes. Times are text columns (HH:MM, Asia/Jakarta wall-clock).

-- name: ListPropagationCandidates :many
-- The FUTURE, live, non-day-off, non-cancelled entries linked to a master, with
-- their attendance check-in/out instants LEFT-JOINed (≤1 attendance per entry via
-- the attendance_schedule_uq partial unique index). "future" = work_date >= the
-- current Asia/Jakarta calendar date (passed as `now` and reduced to a date here,
-- mirroring clock.sql). The Go layer inspects check_in_at/check_out_at to pick the
-- freeze branch per row.
SELECT se.id,
       se.work_date,
       se.start_time,
       se.end_time,
       se.cross_midnight,
       a.check_in_at  AS att_check_in_at,
       a.check_out_at AS att_check_out_at
FROM schedule_entries se
LEFT JOIN attendance a
       ON a.schedule_id = se.id
      AND a.deleted_at IS NULL
WHERE se.shift_master_id = sqlc.arg(shift_master_id)
  AND se.deleted_at IS NULL
  AND se.is_day_off = false
  AND se.status <> 'CANCELLED_BY_LEAVE'
  AND se.work_date >= (sqlc.arg(now)::timestamptz AT TIME ZONE 'Asia/Jakarta')::date
ORDER BY se.id;

-- name: UpdateScheduleEntryTimes :execrows
-- Focused update of just the three snapshot time columns (start/end/cross), used
-- by the propagation loop. Status / is_day_off / shift_master_id are untouched —
-- this is a re-sync of the window only, not a reschedule.
UPDATE schedule_entries
SET start_time     = sqlc.narg(start_time),
    end_time       = sqlc.narg(end_time),
    cross_midnight = sqlc.arg(cross_midnight),
    updated_at     = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: SyncOpenAttendanceShiftEnd :execrows
-- DELIBERATE CROSS-EPIC WRITE (E4 → E5): when a master's end_time moves and an
-- agent is mid-shift (checked in, NOT yet out), the entry keeps its frozen start
-- but its end is re-synced; the OPEN attendance row's shift_end_at must follow so
-- the auto-close / overtime windows stay correct. Guarded by check_out_at IS NULL
-- (only open rows) + deleted_at IS NULL. shift_start_at is intentionally NOT
-- touched (the start is frozen at clock-in). Computed shift_end_at (work_date +
-- new end, +1 day when cross) is passed in from Go.
UPDATE attendance
SET shift_end_at = sqlc.arg(shift_end_at),
    updated_at   = now()
WHERE schedule_id = sqlc.arg(schedule_id)
  AND check_out_at IS NULL
  AND deleted_at IS NULL;
