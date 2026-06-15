-- E6 HR-assigned leave entitlements (PRD leave-entitlement-assignment, 2026-06-15).
-- The base policy: which leave types an employee is entitled to + the per-period
-- quota. The QuotaMeter reads GetEmployeeEntitlement to auto-open windows (ELE-2);
-- HR CRUDs assignments via the list/upsert/update/deactivate queries.

-- name: GetEmployeeEntitlement :one
-- The active entitlement for one (employee, leave_type) — the entitlementFor source.
-- Returns no rows when the type is not assigned (caller falls back to cap_value).
SELECT id, employee_id, leave_type_id, entitled_days, active, note, assigned_by,
       created_at, updated_at
FROM employee_leave_entitlements
WHERE employee_id   = sqlc.arg(employee_id)
  AND leave_type_id = sqlc.arg(leave_type_id)
  AND active
  AND deleted_at IS NULL;

-- name: ListEmployeeEntitlements :many
-- An employee's active assignments + the leave-type display/cap metadata (HR "Hak
-- Cuti" grid + the agent picker scope). Only active, non-deleted rows whose type is
-- still in the catalog. Ordered by type code for a stable grid.
SELECT ele.id, ele.employee_id, ele.leave_type_id, ele.entitled_days, ele.active,
       ele.note, ele.assigned_by, ele.created_at, ele.updated_at,
       lt.code, lt.name, lt.cap_basis, lt.cap_value, lt.cap_unit, lt.paid,
       lt.requires_document, lt.color, lt.applies_to, lt.common
FROM employee_leave_entitlements ele
JOIN leave_types lt ON lt.id = ele.leave_type_id AND lt.deleted_at IS NULL
WHERE ele.employee_id = sqlc.arg(employee_id)
  AND ele.active
  AND ele.deleted_at IS NULL
ORDER BY lt.code;

-- name: UpsertEmployeeEntitlement :one
-- Assign a type to an employee (POST). Inserts a fresh SWP-ELE row, or reactivates
-- + re-quotas an existing (employee, type) assignment (ELE-6 add/reactivate).
INSERT INTO employee_leave_entitlements
    (id, employee_id, leave_type_id, entitled_days, active, note, assigned_by)
VALUES (
    'SWP-ELE-' || swp_next_id('ELE'),
    sqlc.arg(employee_id),
    sqlc.arg(leave_type_id),
    sqlc.narg(entitled_days),
    true,
    sqlc.arg(note),
    sqlc.narg(assigned_by)
)
ON CONFLICT (employee_id, leave_type_id) WHERE deleted_at IS NULL
DO UPDATE SET
    entitled_days = EXCLUDED.entitled_days,
    active        = true,
    note          = EXCLUDED.note,
    assigned_by   = EXCLUDED.assigned_by,
    updated_at    = now()
RETURNING id, employee_id, leave_type_id, entitled_days, active, note, assigned_by,
          created_at, updated_at;

-- name: UpdateEmployeeEntitlement :one
-- Edit the base entitled_days / reactivate (PATCH). Targets the active-or-inactive
-- live row for (employee, type); reactivates if it was removed (ELE-4/ELE-6).
UPDATE employee_leave_entitlements
SET entitled_days = sqlc.narg(entitled_days),
    active        = true,
    note          = COALESCE(sqlc.narg(note), note),
    assigned_by   = sqlc.narg(assigned_by),
    updated_at    = now()
WHERE employee_id   = sqlc.arg(employee_id)
  AND leave_type_id = sqlc.arg(leave_type_id)
  AND deleted_at IS NULL
RETURNING id, employee_id, leave_type_id, entitled_days, active, note, assigned_by,
          created_at, updated_at;

-- name: DeactivateEmployeeEntitlement :one
-- Remove a type from an employee (DELETE) — soft on/off, keeps the row + history.
UPDATE employee_leave_entitlements
SET active      = false,
    assigned_by = sqlc.narg(assigned_by),
    updated_at  = now()
WHERE employee_id   = sqlc.arg(employee_id)
  AND leave_type_id = sqlc.arg(leave_type_id)
  AND active
  AND deleted_at IS NULL
RETURNING id, employee_id, leave_type_id, entitled_days, active, note, assigned_by,
          created_at, updated_at;
