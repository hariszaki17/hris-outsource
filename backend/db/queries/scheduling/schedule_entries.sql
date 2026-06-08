-- E4 schedule-entry queries (F4.2/F4.3 / SWP-SCH-*). Reads LEFT JOIN employees for
-- employee_name, shift_masters for shift_master_name, and JOIN placements →
-- client_companies for company_id/company_name. Times are text columns (HH:MM).

-- name: ListSchedule :many
-- Grid load for a company over a date range. Joins placement → company.
-- Filters: company_id (via placement), work_date BETWEEN start/end,
--   employee_id (optional), status__in (optional text[] → status = ANY).
-- Ordered by employee_id, work_date for a stable grid layout.
SELECT se.id, se.employee_id, se.placement_id, se.service_line_id,
       se.shift_master_id, se.start_time, se.end_time, se.cross_midnight,
       se.work_date, se.status, se.is_day_off, se.replaced_entry_id,
       se.created_by, se.created_at, se.updated_at,
       e.full_name AS employee_name,
       p.client_company_id AS company_id,
       c.name AS company_name,
       sm.name AS shift_master_name
FROM schedule_entries se
JOIN placements p             ON p.id  = se.placement_id
LEFT JOIN client_companies c  ON c.id  = p.client_company_id
LEFT JOIN employees e         ON e.id  = se.employee_id
LEFT JOIN shift_masters sm    ON sm.id = se.shift_master_id
WHERE se.deleted_at IS NULL
  AND p.client_company_id = sqlc.arg(company_id)
  AND se.work_date BETWEEN sqlc.arg(start_date)::date AND sqlc.arg(end_date)::date
  AND (sqlc.narg(employee_id)::text IS NULL OR se.employee_id = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(status_in)::text[] IS NULL OR se.status = ANY(sqlc.narg(status_in)::text[]))
ORDER BY se.employee_id ASC, se.work_date ASC, se.id ASC;

-- name: ListScheduleByAgent :many
-- F4.3 "Jadwal Saya": ONE agent's schedule across ALL their placements (no
-- company_id filter — by-agent spans companies). Same projected columns as
-- ListSchedule so the row reuses the list mapper. Ordered by work_date,
-- start_time for the agent's day/week timeline.
-- TODO(SV-3): include_company geo/address enrichment (company_geo/address) is
--   deferred — this query returns the base ScheduleEntry projection only.
SELECT se.id, se.employee_id, se.placement_id, se.service_line_id,
       se.shift_master_id, se.start_time, se.end_time, se.cross_midnight,
       se.work_date, se.status, se.is_day_off, se.replaced_entry_id,
       se.created_by, se.created_at, se.updated_at,
       e.full_name AS employee_name,
       p.client_company_id AS company_id,
       c.name AS company_name,
       sm.name AS shift_master_name
FROM schedule_entries se
JOIN placements p             ON p.id  = se.placement_id
LEFT JOIN client_companies c  ON c.id  = p.client_company_id
LEFT JOIN employees e         ON e.id  = se.employee_id
LEFT JOIN shift_masters sm    ON sm.id = se.shift_master_id
WHERE se.deleted_at IS NULL
  AND se.employee_id = sqlc.arg(employee_id)
  AND se.work_date BETWEEN sqlc.arg(start_date)::date AND sqlc.arg(end_date)::date
ORDER BY se.work_date ASC, se.start_time ASC, se.id ASC;

-- name: GetScheduleEntry :one
-- Single entry with denormalized names + company_id (from placement).
SELECT se.id, se.employee_id, se.placement_id, se.service_line_id,
       se.shift_master_id, se.start_time, se.end_time, se.cross_midnight,
       se.work_date, se.status, se.is_day_off, se.replaced_entry_id,
       se.created_by, se.created_at, se.updated_at,
       e.full_name AS employee_name,
       p.client_company_id AS company_id,
       c.name AS company_name,
       sm.name AS shift_master_name
FROM schedule_entries se
JOIN placements p             ON p.id  = se.placement_id
LEFT JOIN client_companies c  ON c.id  = p.client_company_id
LEFT JOIN employees e         ON e.id  = se.employee_id
LEFT JOIN shift_masters sm    ON sm.id = se.shift_master_id
WHERE se.id = sqlc.arg(id)
  AND se.deleted_at IS NULL;

-- name: GetScheduleEntryForUpdate :one
-- Row-lock for PATCH / soft-delete (omits joins; service re-reads for DTO).
SELECT se.id, se.employee_id, se.placement_id, se.service_line_id,
       se.shift_master_id, se.start_time, se.end_time, se.cross_midnight,
       se.work_date, se.status, se.is_day_off, se.replaced_entry_id,
       se.created_by, se.created_at, se.updated_at
FROM schedule_entries se
WHERE se.id = sqlc.arg(id)
  AND se.deleted_at IS NULL
FOR UPDATE;

-- name: FindLiveEntryForAgentDate :one
-- DOUBLE_SHIFT pre-check / replace lookup: the live entry (if any) for an
-- agent on a date. Mirrors the INV-1 partial unique index predicate.
SELECT se.id, se.shift_master_id, se.status, se.is_day_off
FROM schedule_entries se
WHERE se.employee_id = sqlc.arg(employee_id)
  AND se.work_date = sqlc.arg(work_date)
  AND se.deleted_at IS NULL;

-- name: CreateScheduleEntry :one
-- id allocated by the column DEFAULT ('SWP-SCH-' || swp_next_id('SCH')).
INSERT INTO schedule_entries (
    employee_id, placement_id, service_line_id, shift_master_id,
    start_time, end_time, cross_midnight, work_date, status,
    is_day_off, replaced_entry_id, created_by
) VALUES (
    sqlc.arg(employee_id),
    sqlc.arg(placement_id),
    sqlc.narg(service_line_id),
    sqlc.narg(shift_master_id),
    sqlc.narg(start_time),
    sqlc.narg(end_time),
    sqlc.arg(cross_midnight),
    sqlc.arg(work_date),
    sqlc.arg(status),
    sqlc.arg(is_day_off),
    sqlc.narg(replaced_entry_id),
    sqlc.narg(created_by)
)
RETURNING id, employee_id, placement_id, service_line_id, shift_master_id,
          start_time, end_time, cross_midnight, work_date, status,
          is_day_off, replaced_entry_id, created_by, created_at, updated_at;

-- name: UpdateScheduleEntry :one
UPDATE schedule_entries
SET shift_master_id   = sqlc.narg(shift_master_id),
    service_line_id   = sqlc.narg(service_line_id),
    start_time        = sqlc.narg(start_time),
    end_time          = sqlc.narg(end_time),
    cross_midnight    = sqlc.arg(cross_midnight),
    status            = sqlc.arg(status),
    is_day_off        = sqlc.arg(is_day_off),
    replaced_entry_id = sqlc.narg(replaced_entry_id),
    updated_at        = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, employee_id, placement_id, service_line_id, shift_master_id,
          start_time, end_time, cross_midnight, work_date, status,
          is_day_off, replaced_entry_id, created_by, created_at, updated_at;

-- name: SoftDeleteScheduleEntry :execrows
UPDATE schedule_entries
SET deleted_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: CancelScheduleEntriesForLeave :many
-- INV-3 loop-closer (E6 / Phase 8): cancel overlapping live schedule entries for
-- the leave dates on final/override approval. status='CANCELLED_BY_LEAVE' is the
-- ONLY value the schedule_entries CHECK (migration 00024) permits for this
-- transition — NEVER write 'LEAVE' to the column ('LEAVE' is the DTO boundary
-- value the service maps to). The `status <> 'CANCELLED_BY_LEAVE'` guard makes the
-- cancel idempotent across re-runs. RETURNING drives schedule_impact[] (id + date +
-- the new DB status; the service maps DB 'CANCELLED_BY_LEAVE' → DTO new_status='LEAVE').
UPDATE schedule_entries
SET status = 'CANCELLED_BY_LEAVE', updated_at = now()
WHERE employee_id = sqlc.arg(employee_id)
  AND work_date BETWEEN sqlc.arg(start_date)::date AND sqlc.arg(end_date)::date
  AND status <> 'CANCELLED_BY_LEAVE'
  AND deleted_at IS NULL
RETURNING id, work_date, status;

-- name: CountLeaveDurationDays :one
-- E6 F6.2 server-authoritative leave duration: the count of days in
-- [start_date, end_date] the agent would otherwise be ROSTERED for a shift
-- (a live schedule_entries row: SCHEDULED/MODIFIED, not a day off, not
-- CANCELLED_BY_LEAVE, not deleted) MINUS the days that fall on a public holiday
-- (E7 holidays). Mirrors the openapi rule: "days the agent would be rostered
-- (per E4 Schedule) minus E7 public holidays." DISTINCT work_date guards against
-- duplicate live rows; the NOT EXISTS holiday subquery excludes holiday dates.
SELECT count(DISTINCT se.work_date)::bigint AS duration_days
FROM schedule_entries se
WHERE se.employee_id = sqlc.arg(employee_id)
  AND se.work_date BETWEEN sqlc.arg(start_date)::date AND sqlc.arg(end_date)::date
  AND se.deleted_at IS NULL
  AND se.is_day_off = false
  AND se.status <> 'CANCELLED_BY_LEAVE'
  AND NOT EXISTS (
      SELECT 1 FROM holidays h
      WHERE h.holiday_date = se.work_date
        AND h.deleted_at IS NULL
  );

-- name: FindActivePlacementForAgentDate :one
-- INV-2 / OUTSIDE_PLACEMENT_PERIOD source: the agent's ACTIVE/EXPIRING placement
-- whose period covers work_date (open-ended end_date treated as +inf).
SELECT p.id, p.client_company_id, p.service_line_id, p.site_id,
       p.start_date, p.end_date, p.lifecycle_status
FROM placements p
WHERE p.employee_id = sqlc.arg(employee_id)
  AND p.lifecycle_status IN ('ACTIVE','EXPIRING')
  AND p.start_date <= sqlc.arg(work_date)::date
  AND (p.end_date IS NULL OR p.end_date >= sqlc.arg(work_date)::date)
  AND p.deleted_at IS NULL
LIMIT 1;
