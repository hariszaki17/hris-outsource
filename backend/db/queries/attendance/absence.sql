-- E5 absence-sweep queries (F5.2 true-ABSENT / SWP-ATT-*). The in-process cron
-- (cmd/api, single binary) periodically marks scheduled shifts that ended past a
-- grace with NO clock-in as ABSENT. schedule_entries carries NO stored shift
-- timestamptz — the window is computed from work_date + start_time/end_time (HH:MM,
-- Asia/Jakarta wall-clock) via `... AT TIME ZONE 'Asia/Jakarta'` (CLAUDE.md TZ layer:
-- 07:00 WIB → 00:00 UTC). cross_midnight adds a day to the end. service_line is the
-- attendance text enum (facility_services|building_management|parking) derived from the
-- placement's service_lines row as lower(replace(name,' ','_')) — there is no slug
-- column, the three seeded names map 1:1 ("Facility Services"→facility_services, etc).
-- `make gen` writes internal/repository/sqlc (NEVER hand-edit).

-- name: FindUnreportedAbsences :many
-- Candidate scheduled shifts that ended before :cutoff (now - grace) and have no
-- attendance row yet. Joins placements for the denormalized company/site/position/
-- service_line the ABSENT row must carry. Deterministic order (work_date, id) so
-- batching is stable across ticks. is_day_off and CANCELLED_BY_LEAVE entries are
-- never absences; only live (deleted_at IS NULL) entries with both times present.
SELECT
    se.id                                   AS schedule_id,
    se.employee_id                          AS employee_id,
    se.placement_id                         AS placement_id,
    p.client_company_id                     AS company_id,
    p.site_id                               AS site_id,
    p.position_id                           AS position_id,
    lower(replace(sl.name, ' ', '_'))::text AS service_line,
    ((se.work_date + se.start_time::time) AT TIME ZONE 'Asia/Jakarta')::timestamptz AS shift_start_at,
    (((se.work_date + se.end_time::time)
        + (CASE WHEN se.cross_midnight THEN interval '1 day' ELSE interval '0' END))
        AT TIME ZONE 'Asia/Jakarta')::timestamptz AS shift_end_at
FROM schedule_entries se
JOIN placements p     ON p.id = se.placement_id
JOIN service_lines sl ON sl.id = p.service_line_id
WHERE se.deleted_at IS NULL
  AND se.is_day_off = false
  AND se.status NOT IN ('CANCELLED_BY_LEAVE')
  AND se.start_time IS NOT NULL
  AND se.end_time   IS NOT NULL
  AND (((se.work_date + se.end_time::time)
        + (CASE WHEN se.cross_midnight THEN interval '1 day' ELSE interval '0' END))
        AT TIME ZONE 'Asia/Jakarta') < sqlc.arg(cutoff)::timestamptz
  AND NOT EXISTS (
        SELECT 1 FROM attendance a
        WHERE a.schedule_id = se.id
          AND a.deleted_at IS NULL
      )
ORDER BY se.work_date, se.id
LIMIT sqlc.arg(page_limit);

-- name: CreateAbsentAttendance :one
-- Insert ONE true-ABSENT row: no clock-in (check_in_at/lat_in/lng_in NULL), status
-- ABSENT, verification PENDING (enters the leader verification queue naturally). id
-- fires via the column DEFAULT (omitted). geofence_radius_m / flags / wfo / is_late
-- fall to their column defaults. ON CONFLICT (the partial schedule_id unique index)
-- DO NOTHING makes a concurrent/duplicate insert a no-op — RETURNING then yields NO
-- row (pgx.ErrNoRows), which the repo maps to created=false so the sweep counts only
-- real inserts. Annotation is :one RETURNING id (not :execrows) so the created row's
-- SWP-ATT id is available for the audit EntityID.
INSERT INTO attendance (
    employee_id, placement_id, schedule_id, company_id, service_line,
    site_id, position_id,
    shift_start_at, shift_end_at,
    check_in_at, lat_in, lng_in,
    status, verification_status
)
VALUES (
    sqlc.arg(employee_id), sqlc.arg(placement_id), sqlc.arg(schedule_id),
    sqlc.arg(company_id), sqlc.arg(service_line),
    sqlc.arg(site_id), sqlc.arg(position_id),
    sqlc.arg(shift_start_at), sqlc.arg(shift_end_at),
    NULL, NULL, NULL,
    'ABSENT', 'PENDING'
)
ON CONFLICT (schedule_id) WHERE schedule_id IS NOT NULL AND deleted_at IS NULL
DO NOTHING
RETURNING id;
