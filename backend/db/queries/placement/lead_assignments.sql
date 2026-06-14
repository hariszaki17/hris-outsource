-- E3 lead_assignments queries. A lead (service-line operational approver) covers
-- MANY client companies; the auth middleware derives Principal.CompanyIDs from the
-- active rows here at request time (mirror of shift_leader_assignments derivation).

-- name: ListLeadAssignments :many
-- Filters: employee_id, company_id, active (unassigned_at IS NULL).
SELECT la.id, la.employee_id, la.client_company_id, la.site_id,
       la.assigned_at, la.unassigned_at, la.assigned_by,
       la.created_at, la.updated_at,
       c.name AS client_company_name,
       e.full_name AS employee_name
FROM lead_assignments la
LEFT JOIN client_companies c ON c.id = la.client_company_id
LEFT JOIN employees e        ON e.id = la.employee_id
WHERE (sqlc.narg(company_id)::text  IS NULL OR la.client_company_id = sqlc.narg(company_id)::text)
  AND (sqlc.narg(employee_id)::text IS NULL OR la.employee_id       = sqlc.narg(employee_id)::text)
  AND (NOT sqlc.arg(active_only)::boolean OR la.unassigned_at IS NULL)
ORDER BY la.assigned_at DESC, la.id DESC;

-- name: ListActiveLeadCompaniesForEmployee :many
-- The set of companies the lead currently covers — drives Principal.CompanyIDs.
SELECT client_company_id
FROM lead_assignments
WHERE employee_id = $1
  AND unassigned_at IS NULL
ORDER BY client_company_id;

-- name: CreateLeadAssignment :one
-- id allocated by the column DEFAULT ('SWP-LA-' || swp_next_id('LA')).
INSERT INTO lead_assignments (
    employee_id, client_company_id, site_id, assigned_by
) VALUES (
    sqlc.arg(employee_id),
    sqlc.arg(client_company_id),
    sqlc.narg(site_id),
    sqlc.narg(assigned_by)
)
RETURNING id, employee_id, client_company_id, site_id, assigned_at,
          unassigned_at, assigned_by, created_at, updated_at;

-- name: EndLeadAssignment :exec
-- Sets unassigned_at=now() on the active row (releases lead_assignment_active_uq).
UPDATE lead_assignments
SET unassigned_at = now(),
    updated_at    = now()
WHERE id = sqlc.arg(id)
  AND unassigned_at IS NULL;
