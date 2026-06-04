-- name: ListPositionsForLine :many
-- Cursor page ordered by (created_at desc, id desc), scoped to one service line.
SELECT id, service_line_id, name, alias, status, created_at, updated_at
FROM positions
WHERE service_line_id = sqlc.arg(service_line_id)
  AND deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetPositionByID :one
SELECT id, service_line_id, name, alias, status, created_at, updated_at
FROM positions
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: CreatePosition :one
-- Allocates the SWP-POS id inline from the per-prefix sequence.
INSERT INTO positions (id, service_line_id, name, alias)
VALUES (
    'SWP-POS-' || swp_next_id('POS'),
    sqlc.arg(service_line_id),
    sqlc.arg(name),
    sqlc.arg(alias)
)
RETURNING id, service_line_id, name, alias, status, created_at, updated_at;

-- name: UpdatePosition :one
UPDATE positions
SET name       = sqlc.arg(name),
    alias      = sqlc.arg(alias),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, service_line_id, name, alias, status, created_at, updated_at;

-- name: SetPositionStatus :one
-- Drives soft-delete-like deactivation (status='inactive') or reactivation.
UPDATE positions
SET status     = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, service_line_id, name, alias, status, created_at, updated_at;

-- name: SoftDeletePosition :exec
-- Hard soft-delete: sets deleted_at so the position is invisible to all queries.
UPDATE positions
SET deleted_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
