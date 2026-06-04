-- name: ListLeaveTypes :many
-- Cursor page ordered by (created_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: status, is_annual.
SELECT id, name, code, description, default_annual_quota, is_annual,
       requires_document, color, status, created_at, updated_at
FROM leave_types
WHERE deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(is_annual)::boolean IS NULL OR is_annual = sqlc.narg(is_annual)::boolean)
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetLeaveTypeByID :one
SELECT id, name, code, description, default_annual_quota, is_annual,
       requires_document, color, status, created_at, updated_at
FROM leave_types
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: CreateLeaveType :one
-- Allocates the SWP-LT id inline from the per-prefix sequence.
INSERT INTO leave_types (id, name, code, description, default_annual_quota, is_annual,
                         requires_document, color)
VALUES (
    'SWP-LT-' || swp_next_id('LT'),
    sqlc.arg(name),
    sqlc.arg(code),
    sqlc.arg(description),
    sqlc.arg(default_annual_quota),
    sqlc.arg(is_annual),
    sqlc.arg(requires_document),
    sqlc.arg(color)
)
RETURNING id, name, code, description, default_annual_quota, is_annual,
          requires_document, color, status, created_at, updated_at;

-- name: UpdateLeaveType :one
UPDATE leave_types
SET name                 = sqlc.arg(name),
    code                 = sqlc.arg(code),
    description          = sqlc.arg(description),
    default_annual_quota = sqlc.arg(default_annual_quota),
    is_annual            = sqlc.arg(is_annual),
    requires_document    = sqlc.arg(requires_document),
    color                = sqlc.arg(color),
    updated_at           = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, code, description, default_annual_quota, is_annual,
          requires_document, color, status, created_at, updated_at;

-- name: SetLeaveTypeStatus :one
-- Drives :deactivate (status='inactive') and :reactivate (status='active').
UPDATE leave_types
SET status     = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, code, description, default_annual_quota, is_annual,
          requires_document, color, status, created_at, updated_at;

-- name: SoftDeleteLeaveType :exec
UPDATE leave_types
SET deleted_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
