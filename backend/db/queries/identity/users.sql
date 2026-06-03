-- name: GetUserByEmail :one
-- Login lookup: active, non-deleted user by case-insensitive email.
SELECT id, email, password_hash, role, employee_id, company_id, status,
       created_at, updated_at, deleted_at
FROM users
WHERE lower(email) = lower(sqlc.arg(email))
  AND deleted_at IS NULL;

-- name: GetUserByID :one
SELECT id, email, password_hash, role, employee_id, company_id, status,
       created_at, updated_at, deleted_at
FROM users
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: CreateUser :one
-- Allocates the SWP-USR id inline from the per-prefix sequence.
INSERT INTO users (id, email, password_hash, role, employee_id, company_id, status)
VALUES (
    'SWP-USR-' || swp_next_id('USR'),
    sqlc.arg(email),
    sqlc.arg(password_hash),
    sqlc.arg(role),
    sqlc.narg(employee_id),
    sqlc.narg(company_id),
    'active'
)
RETURNING id, email, password_hash, role, employee_id, company_id, status,
          created_at, updated_at, deleted_at;
