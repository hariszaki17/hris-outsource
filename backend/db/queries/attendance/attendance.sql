-- E5 attendance queries (F5.1/F5.2 / SWP-ATT-*). Reads LEFT JOIN employees for
-- employee_name, client_companies for company_name, client_sites for site_name,
-- and positions for position_name. Cursor lists keyset on (check_in_at DESC, id).
-- `make gen` writes internal/repository/sqlc (NEVER hand-edit). Geofence/lateness/
-- auto-close are STORED columns (07-01 decision); no runtime compute.

-- name: ListAttendance :many
-- Verification queue / history for a company over filters, newest first.
-- Keyset cursor: pass cursor_check_in_at + cursor_id from the previous page tail
-- (both NULL on the first page). Filters are nullable nargs (IS NULL OR ...).
--   verification_status_in / status_in: text[] = ANY membership.
--   site_id / position_id: narrow within the (leader-pinned) company scope.
--   date_from/date_to: bound on the shift-date basis (check_in_at::date).
--   exceptions: when true, only rows with verification_status IN ('PENDING','ESCALATED').
SELECT a.id, a.employee_id, a.placement_id, a.schedule_id, a.company_id,
       a.service_line, a.site_id, a.position_id, a.attendance_code_id,
       a.shift_start_at, a.shift_end_at,
       a.check_in_at, a.check_out_at, a.lat_in, a.lng_in, a.lat_out, a.lng_out,
       a.photo_in_id, a.photo_out_id, a.wfo, a.is_late, a.late_minutes,
       a.worked_minutes, a.auto_closed, a.in_geofence, a.in_distance_m,
       a.out_geofence, a.out_distance_m, a.geofence_radius_m, a.status,
       a.verification_status, a.flags, a.verified_by, a.verified_at,
       a.rejected_by, a.rejected_at, a.reject_reason, a.last_correction_id,
       a.created_at, a.updated_at,
       e.full_name AS employee_name,
       c.name      AS company_name,
       s.name      AS site_name,
       pos.name    AS position_name
FROM attendance a
LEFT JOIN employees e        ON e.id = a.employee_id
LEFT JOIN client_companies c ON c.id = a.company_id
LEFT JOIN client_sites s     ON s.id = a.site_id
LEFT JOIN positions pos      ON pos.id = a.position_id
WHERE a.deleted_at IS NULL
  AND (sqlc.narg(company_id)::text IS NULL OR a.company_id = sqlc.narg(company_id)::text)
  AND (sqlc.narg(employee_id)::text IS NULL OR a.employee_id = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(service_line)::text IS NULL OR a.service_line = sqlc.narg(service_line)::text)
  AND (sqlc.narg(site_id)::text IS NULL OR a.site_id = sqlc.narg(site_id)::text)
  AND (sqlc.narg(position_id)::text IS NULL OR a.position_id = sqlc.narg(position_id)::text)
  AND (sqlc.narg(verification_status_in)::text[] IS NULL OR a.verification_status = ANY(sqlc.narg(verification_status_in)::text[]))
  AND (sqlc.narg(status_in)::text[] IS NULL OR a.status = ANY(sqlc.narg(status_in)::text[]))
  AND (sqlc.narg(date_from)::date IS NULL OR a.check_in_at::date >= sqlc.narg(date_from)::date)
  AND (sqlc.narg(date_to)::date IS NULL OR a.check_in_at::date <= sqlc.narg(date_to)::date)
  AND (sqlc.narg(exceptions)::boolean IS NOT TRUE OR a.verification_status IN ('PENDING','ESCALATED'))
  AND (
        sqlc.narg(cursor_check_in_at)::timestamptz IS NULL
        OR a.check_in_at < sqlc.narg(cursor_check_in_at)::timestamptz
        OR (a.check_in_at = sqlc.narg(cursor_check_in_at)::timestamptz AND a.id < sqlc.narg(cursor_id)::text)
      )
ORDER BY a.check_in_at DESC, a.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetAttendance :one
-- Single record with denormalized names.
SELECT a.id, a.employee_id, a.placement_id, a.schedule_id, a.company_id,
       a.service_line, a.site_id, a.position_id, a.attendance_code_id,
       a.shift_start_at, a.shift_end_at,
       a.check_in_at, a.check_out_at, a.lat_in, a.lng_in, a.lat_out, a.lng_out,
       a.photo_in_id, a.photo_out_id, a.wfo, a.is_late, a.late_minutes,
       a.worked_minutes, a.auto_closed, a.in_geofence, a.in_distance_m,
       a.out_geofence, a.out_distance_m, a.geofence_radius_m, a.status,
       a.verification_status, a.flags, a.verified_by, a.verified_at,
       a.rejected_by, a.rejected_at, a.reject_reason, a.last_correction_id,
       a.created_at, a.updated_at,
       e.full_name AS employee_name,
       c.name      AS company_name,
       s.name      AS site_name,
       pos.name    AS position_name
FROM attendance a
LEFT JOIN employees e        ON e.id = a.employee_id
LEFT JOIN client_companies c ON c.id = a.company_id
LEFT JOIN client_sites s     ON s.id = a.site_id
LEFT JOIN positions pos      ON pos.id = a.position_id
WHERE a.id = sqlc.arg(id)
  AND a.deleted_at IS NULL;

-- name: GetAttendanceForUpdate :one
-- Row-lock for verify/reject/bulk + correction-apply: reads company_id/employee_id/
-- verification_status for scope + state guards (omits joins; service re-reads for DTO).
SELECT a.id, a.employee_id, a.placement_id, a.schedule_id, a.company_id,
       a.service_line, a.site_id, a.position_id, a.attendance_code_id,
       a.shift_start_at, a.shift_end_at,
       a.check_in_at, a.check_out_at, a.lat_in, a.lng_in, a.lat_out, a.lng_out,
       a.photo_in_id, a.photo_out_id, a.wfo, a.is_late, a.late_minutes,
       a.worked_minutes, a.auto_closed, a.in_geofence, a.in_distance_m,
       a.out_geofence, a.out_distance_m, a.geofence_radius_m, a.status,
       a.verification_status, a.flags, a.verified_by, a.verified_at,
       a.rejected_by, a.rejected_at, a.reject_reason, a.last_correction_id,
       a.created_at, a.updated_at
FROM attendance a
WHERE a.id = sqlc.arg(id)
  AND a.deleted_at IS NULL
FOR UPDATE;

-- name: VerifyAttendance :one
-- Approve an exception record. Only PENDING/ESCALATED are verifiable; zero rows
-- returned ⇒ terminal state (service emits 409 ALREADY_VERIFIED/REJECTED).
UPDATE attendance
SET verification_status = 'VERIFIED',
    verified_by         = sqlc.arg(verified_by),
    verified_at         = now(),
    updated_at          = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND verification_status IN ('PENDING','ESCALATED')
RETURNING id, employee_id, placement_id, schedule_id, company_id, service_line,
          site_id, position_id, attendance_code_id, shift_start_at, shift_end_at,
          check_in_at, check_out_at, lat_in, lng_in, lat_out, lng_out, photo_in_id,
          photo_out_id, wfo, is_late, late_minutes, worked_minutes, auto_closed,
          in_geofence, in_distance_m, out_geofence, out_distance_m,
          geofence_radius_m, status, verification_status, flags, verified_by,
          verified_at, rejected_by, rejected_at, reject_reason, last_correction_id,
          created_at, updated_at;

-- name: RejectAttendance :one
-- Reject an exception record (reason required). Same PENDING/ESCALATED guard.
UPDATE attendance
SET verification_status = 'REJECTED',
    rejected_by         = sqlc.arg(rejected_by),
    rejected_at         = now(),
    reject_reason       = sqlc.arg(reject_reason),
    updated_at          = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND verification_status IN ('PENDING','ESCALATED')
RETURNING id, employee_id, placement_id, schedule_id, company_id, service_line,
          site_id, position_id, attendance_code_id, shift_start_at, shift_end_at,
          check_in_at, check_out_at, lat_in, lng_in, lat_out, lng_out, photo_in_id,
          photo_out_id, wfo, is_late, late_minutes, worked_minutes, auto_closed,
          in_geofence, in_distance_m, out_geofence, out_distance_m,
          geofence_radius_m, status, verification_status, flags, verified_by,
          verified_at, rejected_by, rejected_at, reject_reason, last_correction_id,
          created_at, updated_at;

-- name: ApplyCorrectionToAttendance :one
-- Apply an approved correction's whitelisted proposed_* fields to the target row:
-- COALESCE(narg, existing) preserves untouched fields; appends 'CORRECTED' to flags
-- (de-duped via array_remove first); sets last_correction_id. status/is_late/
-- late_minutes are RE-EVALUATED in Go (BR CR-9: a CHECK_IN correction that resolves
-- an absence flips ABSENT→PRESENT/LATE) and passed in as nargs (NULL = leave as-is).
UPDATE attendance
SET check_in_at        = COALESCE(sqlc.narg(check_in_at)::timestamptz, check_in_at),
    check_out_at       = COALESCE(sqlc.narg(check_out_at)::timestamptz, check_out_at),
    attendance_code_id = COALESCE(sqlc.narg(attendance_code_id)::text, attendance_code_id),
    status             = COALESCE(sqlc.narg(status)::text, status),
    is_late            = COALESCE(sqlc.narg(is_late)::boolean, is_late),
    late_minutes       = COALESCE(sqlc.narg(late_minutes)::integer, late_minutes),
    flags              = array_remove(flags, 'CORRECTED') || ARRAY['CORRECTED'],
    last_correction_id = sqlc.arg(last_correction_id),
    updated_at         = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, employee_id, placement_id, schedule_id, company_id, service_line,
          site_id, position_id, attendance_code_id, shift_start_at, shift_end_at,
          check_in_at, check_out_at, lat_in, lng_in, lat_out, lng_out, photo_in_id,
          photo_out_id, wfo, is_late, late_minutes, worked_minutes, auto_closed,
          in_geofence, in_distance_m, out_geofence, out_distance_m,
          geofence_radius_m, status, verification_status, flags, verified_by,
          verified_at, rejected_by, rejected_at, reject_reason, last_correction_id,
          created_at, updated_at;
