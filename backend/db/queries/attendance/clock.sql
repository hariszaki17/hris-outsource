-- E5 agent clock-in/out queries (F5.1 / SWP-ATT-*). The mobile agent flow:
-- resolve the active placement + site geofence (existing generated queries), find
-- today's schedule entry for lateness eval, detect an already-open record, then
-- INSERT a clock-in (ON CONFLICT on the partial schedule_id unique index → no-op for
-- a concurrent absence-sweep / double-tap) or UPDATE a clock-out. schedule_entries
-- carries NO stored shift timestamptz — the window is computed from work_date +
-- start_time/end_time (HH:MM, Asia/Jakarta wall-clock) via `AT TIME ZONE` (CLAUDE.md
-- TZ layer); cross_midnight adds a day to the end. `make gen` writes
-- internal/repository/sqlc (NEVER hand-edit).

-- name: GetOpenAttendanceForEmployee :one
-- The caller's currently-open record (clocked in, not yet clocked out). Drives
-- ALREADY_CLOCKED_IN (clock-in) / NOT_CLOCKED_IN (clock-out). Most-recent first so a
-- stale half-open row never masks the latest.
SELECT a.id
FROM attendance a
WHERE a.employee_id = sqlc.arg(employee_id)
  AND a.check_out_at IS NULL
  AND a.deleted_at IS NULL
ORDER BY a.check_in_at DESC
LIMIT 1;

-- name: GetTodayScheduleForEmployee :one
-- Today's (Asia/Jakarta) live schedule entry for the employee — the basis for the
-- lateness eval and the schedule_id stamped on the clock-in record. Mirrors
-- absence.sql's shift-window computation. is_day_off / CANCELLED_BY_LEAVE entries are
-- not work days; both times must be present. Earliest shift wins when more than one.
SELECT
    se.id AS schedule_id,
    ((se.work_date + se.start_time::time) AT TIME ZONE 'Asia/Jakarta')::timestamptz AS shift_start_at,
    (((se.work_date + se.end_time::time)
        + (CASE WHEN se.cross_midnight THEN interval '1 day' ELSE interval '0' END))
        AT TIME ZONE 'Asia/Jakarta')::timestamptz AS shift_end_at
FROM schedule_entries se
WHERE se.employee_id = sqlc.arg(employee_id)
  AND se.work_date = (sqlc.arg(now)::timestamptz AT TIME ZONE 'Asia/Jakarta')::date
  AND se.deleted_at IS NULL
  AND se.is_day_off = false
  AND se.status <> 'CANCELLED_BY_LEAVE'
  AND se.start_time IS NOT NULL
  AND se.end_time   IS NOT NULL
ORDER BY se.start_time
LIMIT 1;

-- name: ClockInAttendance :one
-- Insert ONE clock-in row. id fires via the column DEFAULT (omitted). schedule_id is
-- nullable (NULL ⇒ unscheduled). ON CONFLICT (the partial schedule_id unique index)
-- DO NOTHING makes a concurrent absence-sweep / double-tap a no-op — RETURNING then
-- yields NO row (pgx.ErrNoRows), which the repo maps to created=false so the service
-- emits ALREADY_CLOCKED_IN. flags is text[].
INSERT INTO attendance (
    employee_id, placement_id, schedule_id, company_id,
    site_id, position,
    shift_start_at, shift_end_at,
    check_in_at, lat_in, lng_in, photo_in_id,
    wfo, is_late, late_minutes,
    in_geofence, in_distance_m, geofence_radius_m,
    status, verification_status, flags
)
VALUES (
    sqlc.arg(employee_id), sqlc.arg(placement_id), sqlc.arg(schedule_id),
    sqlc.arg(company_id),
    sqlc.arg(site_id), sqlc.arg(position),
    sqlc.arg(shift_start_at), sqlc.arg(shift_end_at),
    sqlc.arg(check_in_at), sqlc.arg(lat_in), sqlc.arg(lng_in), sqlc.arg(photo_in_id),
    sqlc.arg(wfo), sqlc.arg(is_late), sqlc.arg(late_minutes),
    sqlc.arg(in_geofence), sqlc.arg(in_distance_m), sqlc.arg(geofence_radius_m),
    sqlc.arg(status), sqlc.arg(verification_status), sqlc.arg(flags)
)
ON CONFLICT (schedule_id) WHERE schedule_id IS NOT NULL AND deleted_at IS NULL
DO NOTHING
RETURNING id;

-- name: AutoCloseAttendance :one
-- Auto-close a STALE open record at clock-in time (F5.1 flexible check-in): the agent
-- clocked in, never clocked out, and the checkout window has elapsed. check_out_at is
-- stamped at the computed shift_end (NOT now — they did not actually work past it),
-- auto_closed=true, the AUTO_CLOSED flag added, status INCOMPLETE, verification PENDING
-- (enters the leader queue as an anomaly). Synchronous complement to the absence sweep,
-- for users/companies the cron skips. Guarded by check_out_at IS NULL so a concurrent
-- clock-out wins the race (yields no row → repo treats as already-closed, no-op).
UPDATE attendance
SET check_out_at        = sqlc.arg(check_out_at),
    worked_minutes      = sqlc.arg(worked_minutes),
    auto_closed         = true,
    flags               = sqlc.arg(flags),
    status              = sqlc.arg(status),
    verification_status = sqlc.arg(verification_status),
    updated_at          = now()
WHERE id = sqlc.arg(id)
  AND check_out_at IS NULL
  AND deleted_at IS NULL
RETURNING id;

-- name: ClockOutAttendance :one
-- Close one open record: stamp the clock-out columns + recomputed worked_minutes /
-- flags / status / verification_status. Guarded by deleted_at — yields NO row
-- (pgx.ErrNoRows) if the record vanished, which the service treats as a 500/NotFound.
UPDATE attendance
SET check_out_at        = sqlc.arg(check_out_at),
    lat_out             = sqlc.arg(lat_out),
    lng_out             = sqlc.arg(lng_out),
    photo_out_id        = sqlc.arg(photo_out_id),
    out_geofence        = sqlc.arg(out_geofence),
    out_distance_m      = sqlc.arg(out_distance_m),
    worked_minutes      = sqlc.arg(worked_minutes),
    flags               = sqlc.arg(flags),
    status              = sqlc.arg(status),
    verification_status = sqlc.arg(verification_status),
    updated_at          = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id;
