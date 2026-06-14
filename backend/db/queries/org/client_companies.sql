-- name: ListClientCompanies :many
-- Cursor page ordered by (created_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: q (ILIKE name), status, has_leader. (service_line removed 2026-06-12.)
SELECT id, name, address, leader_scope, npwp, pic_name, phone, email,
       status, created_at, updated_at,
       EXISTS (
         SELECT 1 FROM shift_leader_assignments sla
         WHERE sla.client_company_id = client_companies.id
           AND sla.unassigned_at IS NULL
       ) AS has_leader
FROM client_companies
WHERE deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(q)::text IS NULL OR name ILIKE '%' || sqlc.narg(q)::text || '%')
  AND (sqlc.narg(has_leader)::boolean IS NULL OR
       EXISTS (
         SELECT 1 FROM shift_leader_assignments sla
         WHERE sla.client_company_id = client_companies.id
           AND sla.unassigned_at IS NULL
       ) = sqlc.narg(has_leader)::boolean)
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (client_companies.created_at, client_companies.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY client_companies.created_at DESC, client_companies.id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetClientCompanyByID :one
SELECT id, name, address, leader_scope, npwp, pic_name, phone, email,
       status, created_at, updated_at,
       EXISTS (
         SELECT 1 FROM shift_leader_assignments sla
         WHERE sla.client_company_id = client_companies.id
           AND sla.unassigned_at IS NULL
       ) AS has_leader
FROM client_companies
WHERE client_companies.id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: CreateClientCompany :one
-- Allocates the SWP-CMP id inline from the per-prefix sequence.
INSERT INTO client_companies (id, name, address, leader_scope, npwp, pic_name, phone, email)
VALUES (
    'SWP-CMP-' || swp_next_id('CMP'),
    sqlc.arg(name),
    sqlc.arg(address),
    sqlc.arg(leader_scope),
    sqlc.narg(npwp),
    sqlc.narg(pic_name),
    sqlc.narg(phone),
    sqlc.narg(email)
)
RETURNING id, name, address, leader_scope, npwp, pic_name, phone, email,
          status, created_at, updated_at;

-- name: UpdateClientCompany :one
UPDATE client_companies
SET name         = sqlc.arg(name),
    address      = sqlc.arg(address),
    leader_scope = sqlc.arg(leader_scope),
    npwp         = sqlc.narg(npwp),
    pic_name     = sqlc.narg(pic_name),
    phone        = sqlc.narg(phone),
    email        = sqlc.narg(email),
    updated_at   = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, address, leader_scope, npwp, pic_name, phone, email,
          status, created_at, updated_at;

-- name: SetClientCompanyStatus :one
-- Drives :deactivate (status='inactive') and :reactivate (status='active').
UPDATE client_companies
SET status     = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, address, leader_scope, npwp, pic_name, phone, email,
          status, created_at, updated_at;

-- name: CountActiveSitesForCompany :one
-- Used to populate site_count in the ClientCompany DTO.
SELECT count(*)
FROM client_sites
WHERE client_company_id = sqlc.arg(client_company_id)
  AND status = 'active'
  AND deleted_at IS NULL;
