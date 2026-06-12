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

-- name: GetAnnualEntitlementForEmployee :one
-- ANNUAL_POOL entitlement source: the employee's active employment agreement
-- (annual_leave_entitlement_days, migr. 00040). NULL when unset.
SELECT annual_leave_entitlement_days
FROM employment_agreements
WHERE employee_id = sqlc.arg(employee_id)
  AND status = 'active'
  AND deleted_at IS NULL
LIMIT 1;
