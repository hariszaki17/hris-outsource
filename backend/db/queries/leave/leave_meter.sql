-- E6 per-type meter read queries (Phase 3, EPICS §8 2026-06-12). Source data the
-- QuotaMeter needs to pick the window, gate eligibility, and size the annual pool.

-- name: GetLeaveTypeCap :one
-- Cap mechanics for a leave type (leave_types, migr. 00050).
SELECT id, code, name, cap_basis, cap_value, cap_unit, paid, gender,
       requires_document, notice_days, min_service_years, lead_days, trail_days
FROM leave_types
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetEmployeeGateInfo :one
-- Gender (nullable) + tenure source (join_at) for the request-time gates.
SELECT gender, join_at
FROM employees
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListEmployeeLeaveBalances :many
-- Per-type balance for an employee (F6.5 / mobile "Saldo per jenis"): every active
-- leave type, LEFT JOINed to the employee's quota row for the CURRENT window of that
-- type's cap_basis (year | year-month | EMP). PER_EVENT/UNCAPPED types have no row.
SELECT lt.id, lt.code, lt.name, lt.cap_basis, lt.cap_value, lt.cap_unit, lt.paid,
       lt.gender, lt.requires_document, lt.color,
       lq.id           AS quota_id,
       lq.entitled_days,
       lq.used_days,
       lq.pending_days,
       lq.expires_at,
       lq.period_key
FROM leave_types lt
LEFT JOIN leave_quotas lq
  ON lq.leave_type_id = lt.id
 AND lq.employee_id   = sqlc.arg(employee_id)
 AND lq.period_key    = CASE lt.cap_basis
        WHEN 'PER_MONTH'      THEN sqlc.arg(cur_month)::text
        WHEN 'LIFETIME_ONCE'  THEN 'EMP'
        WHEN 'SERVICE_UNPAID' THEN 'EMP'
        ELSE sqlc.arg(cur_year)::text
      END
WHERE lt.deleted_at IS NULL AND lt.status = 'active'
ORDER BY lt.code;

-- name: GetAnnualEntitlementForEmployee :one
-- ANNUAL_POOL entitlement source: the employee's active employment agreement
-- (annual_leave_entitlement_days, migr. 00040). NULL when unset.
SELECT annual_leave_entitlement_days
FROM employment_agreements
WHERE employee_id = sqlc.arg(employee_id)
  AND status = 'active'
  AND deleted_at IS NULL
LIMIT 1;
