-- name: ListEmployees :many
-- Cursor page ordered by (created_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: q (ILIKE over full_name/nik/nip/email_personal/phone), status.
SELECT id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
       phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
       bank_name, bank_account_number, bank_account_holder_name,
       status, created_by, created_at, updated_at
FROM employees
WHERE deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (
        sqlc.narg(q)::text IS NULL
        OR full_name ILIKE '%' || sqlc.narg(q)::text || '%'
        OR nik       ILIKE '%' || sqlc.narg(q)::text || '%'
        OR nip       ILIKE '%' || sqlc.narg(q)::text || '%'
        OR email_personal ILIKE '%' || sqlc.narg(q)::text || '%'
        OR phone     ILIKE '%' || sqlc.narg(q)::text || '%'
      )
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY created_at DESC, id DESC
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
