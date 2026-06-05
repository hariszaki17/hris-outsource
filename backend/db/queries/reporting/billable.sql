-- E10 F10.3 billable-hours report (GET /reports/attendance-billable). Computed
-- LIVE from VERIFIED attendance (INV-4 / BR-1) on billable attendance codes
-- (E2 attendance_codes.is_billable). Hours only — no rates/amounts (EPICS §8).
--
-- SCHEMA-ALIGNMENT NOTE [11-01 DECISION]:
--   * "verified" = attendance.verification_status = 'VERIFIED' (00026; the plan
--     prose mentioned is_verified — there is no such column, verification_status
--     is the source of truth).
--   * worked/billable/payable hours are DERIVED from attendance.worked_minutes
--     (a real STORED column set on clock-out/auto-close), /60.0 → hours. v1 has no
--     separate payable column, so payable_hours = worked_hours for billable codes
--     (faithful stand-in, derived from real rows — NOT a constant). When
--     worked_minutes is NULL (open shift) it contributes 0.
--   * billable_hours counts only billable-code minutes; worked_hours counts ALL
--     verified worked minutes in the group; the GROUP filter is applied per-variant.
--   * company/service-line names come from a JOIN to placements -> client_companies
--     / service_lines (attendance.company_id is the denormalized scope key;
--     placements.service_line_id is the line).
--   * shift date = check_in_at::date (no attendance_shift_date column exists).
--
-- Three GROUP BY variants (employee / day / shift_master) are provided as distinct
-- typed queries rather than a single @group_by CASE — keeps sqlc row types precise
-- and the service maps each to BillableReportRow. The shared filter is identical.

-- name: BillableAggregateByEmployee :many
-- group_by=employee: group_key = SWP-EMP-*, group_label = employee full_name.
SELECT
    a.employee_id                                                            AS group_key,
    COALESCE(e.full_name, a.employee_id)                                     AS group_label,
    min(p.client_company_id)::text                                                 AS company_id,
    min(cc.name)::text                                                             AS company_name,
    min(p.service_line_id)::text                                                   AS service_line_id,
    min(sl.name)::text                                                             AS service_line_name,
    COALESCE(sum(a.worked_minutes), 0)::bigint                               AS worked_minutes,
    COALESCE(sum(a.worked_minutes) FILTER (WHERE ac.is_billable), 0)::bigint AS billable_minutes,
    count(*)::bigint                                                         AS verified_record_count
FROM attendance a
JOIN placements p          ON p.id = a.placement_id
LEFT JOIN attendance_codes ac ON ac.id = a.attendance_code_id
LEFT JOIN employees e      ON e.id = a.employee_id
LEFT JOIN client_companies cc ON cc.id = p.client_company_id
LEFT JOIN service_lines sl ON sl.id = p.service_line_id
WHERE a.deleted_at IS NULL
  AND a.verification_status = 'VERIFIED'
  AND a.check_in_at::date BETWEEN sqlc.arg(period_start)::date AND sqlc.arg(period_end)::date
  AND (sqlc.narg(company_id)::text IS NULL OR a.company_id = sqlc.narg(company_id)::text)
  AND (sqlc.narg(service_line_id)::text IS NULL OR p.service_line_id = sqlc.narg(service_line_id)::text)
GROUP BY a.employee_id, e.full_name
ORDER BY group_label;

-- name: BillableAggregateByDay :many
-- group_by=day: group_key = ISO date, group_label = same ISO date.
SELECT
    (a.check_in_at::date)::text                                              AS group_key,
    (a.check_in_at::date)::text                                              AS group_label,
    min(p.client_company_id)::text                                                 AS company_id,
    min(cc.name)::text                                                             AS company_name,
    min(p.service_line_id)::text                                                   AS service_line_id,
    min(sl.name)::text                                                             AS service_line_name,
    COALESCE(sum(a.worked_minutes), 0)::bigint                               AS worked_minutes,
    COALESCE(sum(a.worked_minutes) FILTER (WHERE ac.is_billable), 0)::bigint AS billable_minutes,
    count(*)::bigint                                                         AS verified_record_count
FROM attendance a
JOIN placements p          ON p.id = a.placement_id
LEFT JOIN attendance_codes ac ON ac.id = a.attendance_code_id
LEFT JOIN client_companies cc ON cc.id = p.client_company_id
LEFT JOIN service_lines sl ON sl.id = p.service_line_id
WHERE a.deleted_at IS NULL
  AND a.verification_status = 'VERIFIED'
  AND a.check_in_at::date BETWEEN sqlc.arg(period_start)::date AND sqlc.arg(period_end)::date
  AND (sqlc.narg(company_id)::text IS NULL OR a.company_id = sqlc.narg(company_id)::text)
  AND (sqlc.narg(service_line_id)::text IS NULL OR p.service_line_id = sqlc.narg(service_line_id)::text)
GROUP BY (a.check_in_at::date)
ORDER BY group_key;

-- name: BillableAggregateByShiftMaster :many
-- group_by=shift_master: group_key = SWP-SHF-*, group_label = shift master name.
-- shift_master resolved via the schedule_entries row linked by attendance.schedule_id.
SELECT
    COALESCE(se.shift_master_id, 'UNSCHEDULED')                              AS group_key,
    COALESCE(sm.name, 'Tanpa Jadwal')                                        AS group_label,
    min(p.client_company_id)::text                                                 AS company_id,
    min(cc.name)::text                                                             AS company_name,
    min(p.service_line_id)::text                                                   AS service_line_id,
    min(sl.name)::text                                                             AS service_line_name,
    COALESCE(sum(a.worked_minutes), 0)::bigint                               AS worked_minutes,
    COALESCE(sum(a.worked_minutes) FILTER (WHERE ac.is_billable), 0)::bigint AS billable_minutes,
    count(*)::bigint                                                         AS verified_record_count
FROM attendance a
JOIN placements p          ON p.id = a.placement_id
LEFT JOIN attendance_codes ac ON ac.id = a.attendance_code_id
LEFT JOIN schedule_entries se ON se.id = a.schedule_id
LEFT JOIN shift_masters sm ON sm.id = se.shift_master_id
LEFT JOIN client_companies cc ON cc.id = p.client_company_id
LEFT JOIN service_lines sl ON sl.id = p.service_line_id
WHERE a.deleted_at IS NULL
  AND a.verification_status = 'VERIFIED'
  AND a.check_in_at::date BETWEEN sqlc.arg(period_start)::date AND sqlc.arg(period_end)::date
  AND (sqlc.narg(company_id)::text IS NULL OR a.company_id = sqlc.narg(company_id)::text)
  AND (sqlc.narg(service_line_id)::text IS NULL OR p.service_line_id = sqlc.narg(service_line_id)::text)
GROUP BY COALESCE(se.shift_master_id, 'UNSCHEDULED'), sm.name
ORDER BY group_label;

-- name: BillableSummary :one
-- summary totals across the whole filtered set (verified-only). verification_rate
-- numerator/denominator returned so the service computes the pct (null when 0).
SELECT
    COALESCE(sum(a.worked_minutes) FILTER (WHERE ac.is_billable), 0)::bigint AS total_billable_minutes,
    COALESCE(sum(a.worked_minutes), 0)::bigint                               AS total_worked_minutes,
    count(*)::bigint                                                         AS total_verified_records
FROM attendance a
JOIN placements p          ON p.id = a.placement_id
LEFT JOIN attendance_codes ac ON ac.id = a.attendance_code_id
WHERE a.deleted_at IS NULL
  AND a.verification_status = 'VERIFIED'
  AND a.check_in_at::date BETWEEN sqlc.arg(period_start)::date AND sqlc.arg(period_end)::date
  AND (sqlc.narg(company_id)::text IS NULL OR a.company_id = sqlc.narg(company_id)::text)
  AND (sqlc.narg(service_line_id)::text IS NULL OR p.service_line_id = sqlc.narg(service_line_id)::text);

-- name: BillablePendingSummary :one
-- pending_summary: records in the period NOT yet billable (not VERIFIED). Hours
-- estimate from worked_minutes (pre-verification, worked-hours stand-in).
SELECT
    count(*)::bigint                          AS pending_records,
    COALESCE(sum(a.worked_minutes), 0)::bigint AS pending_minutes_estimate
FROM attendance a
JOIN placements p          ON p.id = a.placement_id
WHERE a.deleted_at IS NULL
  AND a.verification_status <> 'VERIFIED'
  AND a.check_in_at::date BETWEEN sqlc.arg(period_start)::date AND sqlc.arg(period_end)::date
  AND (sqlc.narg(company_id)::text IS NULL OR a.company_id = sqlc.narg(company_id)::text)
  AND (sqlc.narg(service_line_id)::text IS NULL OR p.service_line_id = sqlc.narg(service_line_id)::text);
