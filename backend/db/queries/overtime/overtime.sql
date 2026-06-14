-- E7 overtime queries (F7.1/F7.4 / SWP-OT-*). Reads LEFT JOIN employees for
-- employee_name, client_companies for company_name (the EmployeeRef/CompanyRef
-- DTOs). work_date comes back as pgtype.Date (09-02 repo converts <-> time.Time
-- like Phase-5/6/8). Keyset cursor on (created_at DESC, id) per CONVENTIONS §11.
-- overtime_rules CRUD is NOT here — reused from E2/Phase-3 (db/queries/org).

-- name: ListOvertime :many
-- Queue / list load. Keyset cursor (created_at,id) DESC. Filters (all optional via
-- narg): employee_id, company_id, status (single), status__in (text[] → status =
-- ANY), work_date >= / <=, day_type (tier), source, flagged_no_preapproval.
SELECT ot.id, ot.employee_id, ot.company_id, ot.placement_id, ot.attendance_id,
       ot.work_date, ot.planned_start_time, ot.planned_end_time,
       ot.actual_start_time, ot.actual_end_time, ot.cross_midnight, ot.source,
       ot.status, ot.day_type, ot.worked_minutes, ot.counted_minutes,
       ot.min_minutes_threshold, ot.skipped_too_short, ot.reference_multiplier,
       ot.overtime_rule_id, ot.holiday_id, ot.flagged_no_preapproval, ot.reason,
       ot.approval_instance_id,
       ot.created_by, ot.created_at, ot.updated_at,
       e.full_name AS employee_name,
       c.name      AS company_name
FROM overtime ot
LEFT JOIN employees e        ON e.id = ot.employee_id
LEFT JOIN client_companies c ON c.id = ot.company_id
WHERE ot.deleted_at IS NULL
  AND (sqlc.narg(employee_id)::text  IS NULL OR ot.employee_id = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(company_id)::text   IS NULL OR ot.company_id  = sqlc.narg(company_id)::text)
  AND (sqlc.narg(status)::text       IS NULL OR ot.status      = sqlc.narg(status)::text)
  AND (sqlc.narg(status_in)::text[]  IS NULL OR ot.status      = ANY(sqlc.narg(status_in)::text[]))
  AND (sqlc.narg(work_from)::date    IS NULL OR ot.work_date  >= sqlc.narg(work_from)::date)
  AND (sqlc.narg(work_to)::date      IS NULL OR ot.work_date  <= sqlc.narg(work_to)::date)
  AND (sqlc.narg(day_type)::text     IS NULL OR ot.day_type    = sqlc.narg(day_type)::text)
  AND (sqlc.narg(source)::text       IS NULL OR ot.source      = sqlc.narg(source)::text)
  AND (sqlc.narg(flagged_no_preapproval)::boolean IS NULL
       OR ot.flagged_no_preapproval = sqlc.narg(flagged_no_preapproval)::boolean)
  -- keyset: rows strictly before the cursor (created_at,id) when provided.
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL OR
       (ot.created_at, ot.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY ot.created_at DESC, ot.id DESC
LIMIT sqlc.arg(lim);

-- name: GetOvertime :one
-- Single OT record with denormalized employee + company names (for GET /overtime/{id}).
SELECT ot.id, ot.employee_id, ot.company_id, ot.placement_id, ot.attendance_id,
       ot.work_date, ot.planned_start_time, ot.planned_end_time,
       ot.actual_start_time, ot.actual_end_time, ot.cross_midnight, ot.source,
       ot.status, ot.day_type, ot.worked_minutes, ot.counted_minutes,
       ot.min_minutes_threshold, ot.skipped_too_short, ot.reference_multiplier,
       ot.overtime_rule_id, ot.holiday_id, ot.flagged_no_preapproval, ot.reason,
       ot.approval_instance_id,
       ot.created_by, ot.created_at, ot.updated_at,
       e.full_name AS employee_name,
       c.name      AS company_name
FROM overtime ot
LEFT JOIN employees e        ON e.id = ot.employee_id
LEFT JOIN client_companies c ON c.id = ot.company_id
WHERE ot.id = sqlc.arg(id)
  AND ot.deleted_at IS NULL;

-- name: GetOvertimeForUpdate :one
-- Row-lock for the state-machine transitions (confirm/approve-l1/approve-final/
-- reject/withdraw). Omits joins; the service re-reads via GetOvertime for the DTO.
SELECT ot.id, ot.employee_id, ot.company_id, ot.placement_id, ot.attendance_id,
       ot.work_date, ot.planned_start_time, ot.planned_end_time,
       ot.actual_start_time, ot.actual_end_time, ot.cross_midnight, ot.source,
       ot.status, ot.day_type, ot.worked_minutes, ot.counted_minutes,
       ot.min_minutes_threshold, ot.skipped_too_short, ot.reference_multiplier,
       ot.overtime_rule_id, ot.holiday_id, ot.flagged_no_preapproval, ot.reason,
       ot.created_by, ot.created_at, ot.updated_at
FROM overtime ot
WHERE ot.id = sqlc.arg(id)
  AND ot.deleted_at IS NULL
FOR UPDATE;

-- name: UpdateOvertimeStatus :one
-- The transition writer (RETURNING-or-409 pattern). 09-02 guards the legal
-- from→to transition before calling this.
UPDATE overtime
SET status     = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, employee_id, company_id, placement_id, attendance_id,
          work_date, planned_start_time, planned_end_time, actual_start_time,
          actual_end_time, cross_midnight, source, status, day_type, worked_minutes,
          counted_minutes, min_minutes_threshold, skipped_too_short,
          reference_multiplier, overtime_rule_id, holiday_id, flagged_no_preapproval,
          reason, created_by, created_at, updated_at;

-- name: InsertOvertime :one
-- Seed / HR-on-behalf path (FE/web does not create OT — mobile/agent does). id
-- allocated by the column DEFAULT ('SWP-OT-' || swp_next_id('OT')) when omitted,
-- OR supplied explicitly (deterministic E2E targets) via ON CONFLICT (id) DO NOTHING.
INSERT INTO overtime (
    id, employee_id, company_id, placement_id, attendance_id,
    work_date, planned_start_time, planned_end_time, actual_start_time,
    actual_end_time, cross_midnight, source, status, day_type, worked_minutes,
    counted_minutes, min_minutes_threshold, skipped_too_short, reference_multiplier,
    overtime_rule_id, holiday_id, flagged_no_preapproval, reason, created_by
) VALUES (
    COALESCE(sqlc.narg(id)::text, 'SWP-OT-' || swp_next_id('OT')),
    sqlc.arg(employee_id),
    sqlc.narg(company_id),
    sqlc.arg(placement_id),
    sqlc.narg(attendance_id),
    sqlc.arg(work_date),
    sqlc.narg(planned_start_time),
    sqlc.narg(planned_end_time),
    sqlc.narg(actual_start_time),
    sqlc.narg(actual_end_time),
    sqlc.arg(cross_midnight),
    sqlc.arg(source),
    sqlc.arg(status),
    sqlc.arg(day_type),
    sqlc.arg(worked_minutes),
    sqlc.arg(counted_minutes),
    sqlc.arg(min_minutes_threshold),
    sqlc.arg(skipped_too_short),
    sqlc.narg(reference_multiplier),
    sqlc.narg(overtime_rule_id),
    sqlc.narg(holiday_id),
    sqlc.arg(flagged_no_preapproval),
    sqlc.narg(reason),
    sqlc.narg(created_by)
)
ON CONFLICT (id) DO NOTHING
RETURNING id, employee_id, company_id, placement_id, attendance_id,
          work_date, planned_start_time, planned_end_time, actual_start_time,
          actual_end_time, cross_midnight, source, status, day_type, worked_minutes,
          counted_minutes, min_minutes_threshold, skipped_too_short,
          reference_multiplier, overtime_rule_id, holiday_id, flagged_no_preapproval,
          reason, created_by, created_at, updated_at;

-- name: SetOvertimeApprovalInstanceID :exec
-- E11 linkage: bind an OT record to its governing approval instance.
UPDATE overtime
SET approval_instance_id = sqlc.narg(approval_instance_id),
    updated_at           = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
