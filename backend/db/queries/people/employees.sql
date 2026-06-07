-- name: ListEmployees :many
-- Cursor page ordered by (created_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: q (ILIKE over full_name/nik/nip ONLY — not email/phone), status.
-- current_* come from the employee's single non-terminal placement (INV-1 → at most one);
-- LEFT JOINs so unplaced employees still list (current_* null).
SELECT e.id, e.user_id, e.full_name, e.nik, e.nip, e.join_at, e.gender, e.birth_date, e.birth_place,
       e.phone, e.email_personal, e.address, e.npwp, e.bpjs_kesehatan, e.bpjs_ketenagakerjaan,
       e.bank_name, e.bank_account_number, e.bank_account_holder_name,
       e.status, e.created_by, e.created_at, e.updated_at,
       pos.id   AS current_position_id,
       pos.name AS current_position_name,
       sl.id    AS current_service_line_id,
       sl.name  AS current_service_line_name,
       cc.id    AS current_client_company_id,
       cc.name  AS current_client_company_name
FROM employees e
LEFT JOIN LATERAL (
    SELECT p.position_id, p.service_line_id, p.client_company_id
    FROM placements p
    WHERE p.employee_id = e.id
      AND p.deleted_at IS NULL
      AND p.lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START','EXTENDED')
    ORDER BY p.status_changed_at DESC
    LIMIT 1
) cp ON true
LEFT JOIN positions        pos ON pos.id = cp.position_id
LEFT JOIN service_lines    sl  ON sl.id  = cp.service_line_id
LEFT JOIN client_companies cc  ON cc.id  = cp.client_company_id
WHERE e.deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR e.status = sqlc.narg(status)::text)
  AND (
        sqlc.narg(q)::text IS NULL
        OR e.full_name ILIKE '%' || sqlc.narg(q)::text || '%'
        OR e.nik       ILIKE '%' || sqlc.narg(q)::text || '%'
        OR e.nip       ILIKE '%' || sqlc.narg(q)::text || '%'
      )
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (e.created_at, e.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY e.created_at DESC, e.id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetEmployeeByID :one
SELECT id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
       phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
       bank_name, bank_account_number, bank_account_holder_name,
       status, created_by, created_at, updated_at
FROM employees
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: GetEmployeeByNIK :one
-- Used for duplicate-NIK pre-check (EP-2) before insert/update.
SELECT id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
       phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
       bank_name, bank_account_number, bank_account_holder_name,
       status, created_by, created_at, updated_at
FROM employees
WHERE nik = sqlc.arg(nik)
  AND deleted_at IS NULL;

-- name: CreateEmployee :one
-- Allocates the SWP-EMP id inline from the per-prefix sequence.
INSERT INTO employees (
    id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
    phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
    bank_name, bank_account_number, bank_account_holder_name, created_by
) VALUES (
    'SWP-EMP-' || swp_next_id('EMP'),
    sqlc.narg(user_id),
    sqlc.arg(full_name),
    sqlc.arg(nik),
    sqlc.arg(nip),
    sqlc.arg(join_at),
    sqlc.narg(gender),
    sqlc.narg(birth_date),
    sqlc.narg(birth_place),
    sqlc.narg(phone),
    sqlc.narg(email_personal),
    sqlc.narg(address),
    sqlc.narg(npwp),
    sqlc.narg(bpjs_kesehatan),
    sqlc.narg(bpjs_ketenagakerjaan),
    sqlc.narg(bank_name),
    sqlc.narg(bank_account_number),
    sqlc.narg(bank_account_holder_name),
    sqlc.narg(created_by)
)
RETURNING id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
          phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
          bank_name, bank_account_number, bank_account_holder_name,
          status, created_by, created_at, updated_at;

-- name: UpdateEmployee :one
UPDATE employees
SET full_name                = sqlc.arg(full_name),
    nik                      = sqlc.arg(nik),
    nip                      = sqlc.arg(nip),
    join_at                  = sqlc.arg(join_at),
    gender                   = sqlc.narg(gender),
    birth_date               = sqlc.narg(birth_date),
    birth_place              = sqlc.narg(birth_place),
    phone                    = sqlc.narg(phone),
    email_personal           = sqlc.narg(email_personal),
    address                  = sqlc.narg(address),
    npwp                     = sqlc.narg(npwp),
    bpjs_kesehatan           = sqlc.narg(bpjs_kesehatan),
    bpjs_ketenagakerjaan     = sqlc.narg(bpjs_ketenagakerjaan),
    bank_name                = sqlc.narg(bank_name),
    bank_account_number      = sqlc.narg(bank_account_number),
    bank_account_holder_name = sqlc.narg(bank_account_holder_name),
    updated_at               = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
          phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
          bank_name, bank_account_number, bank_account_holder_name,
          status, created_by, created_at, updated_at;

-- name: SetEmployeeStatus :one
-- Drives :deactivate (status='inactive') and :reactivate (status='active').
UPDATE employees
SET status     = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
          phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
          bank_name, bank_account_number, bank_account_holder_name,
          status, created_by, created_at, updated_at;

-- name: SetEmployeeUserID :exec
-- EP-3: links a freshly provisioned E1 User to the employee (1:1). Flips
-- has_login (derived from user_id) to true.
UPDATE employees
SET user_id    = sqlc.arg(user_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
