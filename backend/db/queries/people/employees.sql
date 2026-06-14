-- name: ListEmployees :many
-- Cursor page ordered by (created_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: q (ILIKE over full_name/nik/nip ONLY — not email/phone), status.
-- current_* come from the employee's single non-terminal placement (INV-1 → at most one);
-- LEFT JOINs so unplaced employees still list (current_* null). current_position is
-- the free-text placement label (no master / FK / ID; service_line dropped 2026-06-12).
SELECT e.id, e.user_id, e.full_name, e.nik, e.nip, e.join_at, e.gender, e.birth_date, e.birth_place,
       e.phone, e.email_personal, e.address, e.npwp, e.bpjs_kesehatan, e.bpjs_ketenagakerjaan,
       e.bank_name, e.bank_account_number, e.bank_account_holder_name,
       e.emergency_contact_name, e.emergency_contact_phone, e.app_language, e.photo_object_key,
       e.status, e.created_by, e.created_at, e.updated_at,
       COALESCE(cp.position, '') AS current_position,
       cc.id       AS current_client_company_id,
       cc.name     AS current_client_company_name
FROM employees e
LEFT JOIN LATERAL (
    SELECT p.position, p.client_company_id
    FROM placements p
    WHERE p.employee_id = e.id
      AND p.deleted_at IS NULL
      AND p.lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START','EXTENDED')
    ORDER BY p.status_changed_at DESC
    LIMIT 1
) cp ON true
LEFT JOIN client_companies cc  ON cc.id  = cp.client_company_id
WHERE e.deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR e.status = sqlc.narg(status)::text)
  -- role: filter by the linked User's E1 role (CONVENTIONS §18). Requires the
  -- bidirectional link e.user_id (set on provisioning / seed back-fill).
  AND (
        sqlc.narg(role)::text IS NULL
        OR EXISTS (
            SELECT 1 FROM users u
            WHERE u.id = e.user_id
              AND u.deleted_at IS NULL
              AND u.role = sqlc.narg(role)::text
        )
      )
  -- assigned: true/false against an active shift-leader assignment (unassigned_at
  -- IS NULL). Drives the unassigned-leader picker (role=shift_leader&assigned=false).
  AND (
        sqlc.narg(assigned)::boolean IS NULL
        OR sqlc.narg(assigned)::boolean = EXISTS (
            SELECT 1 FROM shift_leader_assignments sla
            WHERE sla.employee_id = e.id
              AND sla.unassigned_at IS NULL
        )
      )
  AND (
        sqlc.narg(q)::text IS NULL
        OR e.full_name ILIKE '%' || sqlc.narg(q)::text || '%'
        OR e.nik       ILIKE '%' || sqlc.narg(q)::text || '%'
        OR e.nip       ILIKE '%' || sqlc.narg(q)::text || '%'
      )
  AND (
        sqlc.narg(client_company)::text IS NULL
        OR cc.id = sqlc.narg(client_company)::text
      )
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (e.created_at, e.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY e.created_at DESC, e.id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetEmployeeByID :one
-- current_* come from the employee's single non-terminal placement (INV-1 → at most
-- one), resolved with the same LATERAL as ListEmployees. LEFT JOINs so an unplaced
-- employee still resolves (current_* null). current_position is the free-text
-- placement label (no master / FK / ID; service_line dropped 2026-06-12).
SELECT e.id, e.user_id, e.full_name, e.nik, e.nip, e.join_at, e.gender, e.birth_date, e.birth_place,
       e.phone, e.email_personal, e.address, e.npwp, e.bpjs_kesehatan, e.bpjs_ketenagakerjaan,
       e.bank_name, e.bank_account_number, e.bank_account_holder_name,
       e.emergency_contact_name, e.emergency_contact_phone, e.app_language, e.photo_object_key,
       e.status, e.created_by, e.created_at, e.updated_at,
       COALESCE(cp.position, '') AS current_position,
       cc.id       AS current_client_company_id,
       cc.name     AS current_client_company_name
FROM employees e
LEFT JOIN LATERAL (
    SELECT p.position, p.client_company_id
    FROM placements p
    WHERE p.employee_id = e.id
      AND p.deleted_at IS NULL
      AND p.lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START','EXTENDED')
    ORDER BY p.status_changed_at DESC
    LIMIT 1
) cp ON true
LEFT JOIN client_companies cc  ON cc.id  = cp.client_company_id
WHERE e.id = sqlc.arg(id)
  AND e.deleted_at IS NULL;

-- name: GetEmployeeByNIK :one
-- Used for duplicate-NIK pre-check (EP-2) before insert/update.
SELECT id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
       phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
       bank_name, bank_account_number, bank_account_holder_name,
       emergency_contact_name, emergency_contact_phone, app_language, photo_object_key,
       status, created_by, created_at, updated_at
FROM employees
WHERE nik = sqlc.arg(nik)
  AND deleted_at IS NULL;

-- name: CreateEmployee :one
-- Allocates the SWP-EMP id inline from the per-prefix sequence.
INSERT INTO employees (
    id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
    phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
    bank_name, bank_account_number, bank_account_holder_name,
    emergency_contact_name, emergency_contact_phone, created_by
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
    sqlc.narg(emergency_contact_name),
    sqlc.narg(emergency_contact_phone),
    sqlc.narg(created_by)
)
RETURNING id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
          phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
          bank_name, bank_account_number, bank_account_holder_name,
          emergency_contact_name, emergency_contact_phone, app_language, photo_object_key,
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
    emergency_contact_name   = sqlc.narg(emergency_contact_name),
    emergency_contact_phone  = sqlc.narg(emergency_contact_phone),
    updated_at               = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
          phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
          bank_name, bank_account_number, bank_account_holder_name,
          emergency_contact_name, emergency_contact_phone, app_language, photo_object_key,
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
          emergency_contact_name, emergency_contact_phone, app_language, photo_object_key,
          status, created_by, created_at, updated_at;

-- name: UpdateEmployeeSelfInstant :one
-- EP-5 agent self-service instant apply (PATCH /me/profile). E11 removed the
-- change-request approval queue: phone / emergency contact / bank fields are now
-- instant self-edit too (alongside address, app_language, photo_object_key).
-- COALESCE keeps a column unchanged when the caller passes NULL, so partial patches
-- don't clobber the other fields. Phone uniqueness is enforced in the Go service
-- layer, not here.
UPDATE employees
SET address                  = COALESCE(sqlc.narg(address), address),
    app_language             = COALESCE(sqlc.narg(app_language), app_language),
    photo_object_key         = COALESCE(sqlc.narg(photo_object_key), photo_object_key),
    phone                    = COALESCE(sqlc.narg(phone), phone),
    emergency_contact_name   = COALESCE(sqlc.narg(emergency_contact_name), emergency_contact_name),
    emergency_contact_phone  = COALESCE(sqlc.narg(emergency_contact_phone), emergency_contact_phone),
    bank_name                = COALESCE(sqlc.narg(bank_name), bank_name),
    bank_account_number      = COALESCE(sqlc.narg(bank_account_number), bank_account_number),
    bank_account_holder_name = COALESCE(sqlc.narg(bank_account_holder_name), bank_account_holder_name),
    updated_at               = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, user_id, full_name, nik, nip, join_at, gender, birth_date, birth_place,
          phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan,
          bank_name, bank_account_number, bank_account_holder_name,
          emergency_contact_name, emergency_contact_phone, app_language, photo_object_key,
          status, created_by, created_at, updated_at;

-- name: SetEmployeeUserID :exec
-- EP-3: links a freshly provisioned E1 User to the employee (1:1). Flips
-- has_login (derived from user_id) to true.
UPDATE employees
SET user_id    = sqlc.arg(user_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
