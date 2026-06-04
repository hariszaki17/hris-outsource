-- E3 shift_leader_assignments queries (F3.4 / SL-*). Reads LEFT JOIN the company
-- and employee tables to fill the denormalized *_name fields.

-- name: ListShiftLeaderAssignments :many
-- Filters: company_id, employee_id, active (unassigned_at IS NULL).
SELECT sla.id, sla.client_company_id, sla.site_id, sla.employee_id,
       sla.assigned_at, sla.unassigned_at, sla.assigned_by, sla.vacated_reason,
       sla.notes, sla.created_at, sla.updated_at,
       c.name AS client_company_name,
       e.full_name AS employee_name
FROM shift_leader_assignments sla
LEFT JOIN client_companies c ON c.id = sla.client_company_id
LEFT JOIN employees e        ON e.id = sla.employee_id
WHERE (sqlc.narg(company_id)::text  IS NULL OR sla.client_company_id = sqlc.narg(company_id)::text)
  AND (sqlc.narg(employee_id)::text IS NULL OR sla.employee_id       = sqlc.narg(employee_id)::text)
  AND (NOT sqlc.arg(active_only)::boolean OR sla.unassigned_at IS NULL)
ORDER BY sla.assigned_at DESC, sla.id DESC;

-- name: GetActiveLeaderForCompanyForUpdate :one
-- INV-2 company-scope lock: active leader of a company-scope unit, row-locked.
SELECT id, client_company_id, site_id, employee_id, assigned_at, unassigned_at,
       assigned_by, vacated_reason, notes, created_at, updated_at
FROM shift_leader_assignments
WHERE client_company_id = sqlc.arg(client_company_id)
  AND site_id IS NULL
  AND unassigned_at IS NULL
FOR UPDATE;

-- name: GetActiveLeaderForSiteForUpdate :one
-- INV-2 site-scope lock: active leader of a site-scope unit, row-locked.
SELECT id, client_company_id, site_id, employee_id, assigned_at, unassigned_at,
       assigned_by, vacated_reason, notes, created_at, updated_at
FROM shift_leader_assignments
WHERE site_id = sqlc.arg(site_id)
  AND unassigned_at IS NULL
FOR UPDATE;

-- name: GetActiveAssignmentForEmployeeForUpdate :one
-- INV-3 lock: the employee's active leadership assignment, row-locked.
SELECT id, client_company_id, site_id, employee_id, assigned_at, unassigned_at,
       assigned_by, vacated_reason, notes, created_at, updated_at
FROM shift_leader_assignments
WHERE employee_id = sqlc.arg(employee_id)
  AND unassigned_at IS NULL
FOR UPDATE;

-- name: GetShiftLeaderAssignmentByID :one
SELECT sla.id, sla.client_company_id, sla.site_id, sla.employee_id,
       sla.assigned_at, sla.unassigned_at, sla.assigned_by, sla.vacated_reason,
       sla.notes, sla.created_at, sla.updated_at,
       c.name AS client_company_name,
       e.full_name AS employee_name
FROM shift_leader_assignments sla
LEFT JOIN client_companies c ON c.id = sla.client_company_id
LEFT JOIN employees e        ON e.id = sla.employee_id
WHERE sla.id = sqlc.arg(id);

-- name: CreateShiftLeaderAssignment :one
-- id allocated by the column DEFAULT ('SWP-SLA-' || swp_next_id('SLA')).
INSERT INTO shift_leader_assignments (
    client_company_id, site_id, employee_id, assigned_by, notes
) VALUES (
    sqlc.arg(client_company_id),
    sqlc.narg(site_id),
    sqlc.arg(employee_id),
    sqlc.narg(assigned_by),
    sqlc.narg(notes)
)
RETURNING id, client_company_id, site_id, employee_id, assigned_at,
          unassigned_at, assigned_by, vacated_reason, notes, created_at, updated_at;

-- name: EndShiftLeaderAssignment :one
-- Sets unassigned_at=now() + vacated_reason (release the active partial unique index).
UPDATE shift_leader_assignments
SET unassigned_at  = now(),
    vacated_reason = sqlc.arg(vacated_reason),
    updated_at     = now()
WHERE id = sqlc.arg(id)
RETURNING id, client_company_id, site_id, employee_id, assigned_at,
          unassigned_at, assigned_by, vacated_reason, notes, created_at, updated_at;
