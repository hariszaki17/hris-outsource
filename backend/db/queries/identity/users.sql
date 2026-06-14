-- name: GetUserByIdentifier :one
-- Login lookup (D2): active, non-deleted user by phone OR case-insensitive
-- email. Phone is matched exactly (stored normalized E.164); email lowercased.
SELECT id, email, phone, password_hash, role, employee_id, company_id, status,
       full_name, last_login_at, must_change_password,
       created_at, updated_at, deleted_at
FROM users
WHERE (phone = sqlc.arg(identifier)::text OR lower(email) = lower(sqlc.arg(identifier)::text))
  AND deleted_at IS NULL;

-- name: GetUserByEmail :one
-- Email-only lookup: used by forgot-password (reset link is email-delivered).
SELECT id, email, phone, password_hash, role, employee_id, company_id, status,
       full_name, last_login_at, must_change_password,
       created_at, updated_at, deleted_at
FROM users
WHERE lower(email) = lower(sqlc.arg(email))
  AND deleted_at IS NULL;

-- name: GetUserByID :one
SELECT id, email, phone, password_hash, role, employee_id, company_id, status,
       full_name, last_login_at, must_change_password, tokens_valid_after,
       created_at, updated_at, deleted_at
FROM users
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: BumpTokensValidAfter :exec
-- F2.7 session epoch: invalidate every access token issued before now() for this
-- user (called on offboard/deactivate alongside refresh-token revocation).
UPDATE users
SET tokens_valid_after = now(),
    updated_at         = now()
WHERE id = sqlc.arg(id);

-- name: CreateUser :one
-- Allocates the SWP-USR id inline from the per-prefix sequence. Phone is the
-- required login identifier (D2); email is optional.
INSERT INTO users (id, email, phone, password_hash, role, employee_id, company_id, status, full_name, must_change_password)
VALUES (
    'SWP-USR-' || swp_next_id('USR'),
    sqlc.narg(email),
    sqlc.arg(phone),
    sqlc.arg(password_hash),
    sqlc.arg(role),
    sqlc.narg(employee_id),
    sqlc.narg(company_id),
    'active',
    sqlc.arg(full_name),
    sqlc.arg(must_change_password)
)
RETURNING id, email, phone, password_hash, role, employee_id, company_id, status,
          full_name, last_login_at, must_change_password,
          created_at, updated_at, deleted_at;

-- name: SetLastLogin :exec
-- Records the time of a successful login (AU-3). Called inside issuePair's tx.
UPDATE users
SET last_login_at = now(),
    updated_at    = now()
WHERE id = sqlc.arg(id);

-- name: UpdatePassword :exec
-- Sets a new password hash after a reset/change (AU-4). The user has chosen their
-- own password, so any temp-password rotation requirement is cleared.
UPDATE users
SET password_hash        = sqlc.arg(password_hash),
    must_change_password = false,
    updated_at           = now()
WHERE id = sqlc.arg(id);

-- name: RegenerateTempPassword :exec
-- EP-3: HR re-issues a temporary password (show-once). Sets the new hash and
-- forces a rotation on next login. Used by :regenerate-password.
UPDATE users
SET password_hash        = sqlc.arg(password_hash),
    must_change_password = true,
    updated_at           = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListUsers :many
-- Cursor page ordered by (created_at desc, id desc). Fetch limit+1 to compute has_more.
-- Filters are optional: a NULL sqlc.narg means "no filter" via the `(arg IS NULL OR col = arg)` idiom.
-- Free-text q matches email or full_name (ILIKE '%' || q || '%').
SELECT id, email, phone, role, employee_id, company_id, status, full_name,
       last_login_at, created_at, updated_at
FROM users
WHERE deleted_at IS NULL
  AND (sqlc.narg(role)::text       IS NULL OR role = sqlc.narg(role))
  AND (sqlc.narg(status)::text     IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(company_id)::text IS NULL OR company_id = sqlc.narg(company_id))
  AND (sqlc.narg(q)::text          IS NULL OR email ILIKE '%' || sqlc.narg(q) || '%' OR full_name ILIKE '%' || sqlc.narg(q) || '%')
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: UpdateUserEmail :one
-- PATCH /users/{id} non-role update (email only per UpdateUserRequest). Returns the full row.
UPDATE users
SET email = sqlc.arg(email), updated_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING id, email, phone, role, employee_id, company_id, status, full_name, last_login_at, created_at, updated_at;

-- name: ChangeUserRole :one
UPDATE users
SET role = sqlc.arg(role), updated_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING id, email, phone, role, employee_id, company_id, status, full_name, last_login_at, created_at, updated_at;

-- name: SetUserStatus :one
-- Used by :deactivate (status='disabled') and :reactivate (status='active').
UPDATE users
SET status = sqlc.arg(status), updated_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING id, email, phone, role, employee_id, company_id, status, full_name, last_login_at, created_at, updated_at;
