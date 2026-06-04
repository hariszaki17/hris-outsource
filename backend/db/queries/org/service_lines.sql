-- name: ListServiceLines :many
-- Cursor page ordered by (created_at desc, id desc). Fetch limit+1 for has_more.
SELECT id, name, status, created_at, updated_at
FROM service_lines
WHERE deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetServiceLineByID :one
SELECT id, name, status, created_at, updated_at
FROM service_lines
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: CreateServiceLine :one
-- Allocates the SWP-SVC id inline from the per-prefix sequence.
INSERT INTO service_lines (id, name)
VALUES (
    'SWP-SVC-' || swp_next_id('SVC'),
    sqlc.arg(name)
)
RETURNING id, name, status, created_at, updated_at;

-- name: UpdateServiceLine :one
UPDATE service_lines
SET name       = sqlc.arg(name),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, status, created_at, updated_at;

-- name: SetServiceLineStatus :one
-- Drives :discontinue (status='inactive') and :reactivate (status='active').
UPDATE service_lines
SET status     = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, name, status, created_at, updated_at;

-- name: CountActivePositionsForLine :one
-- Used to populate position_count in the ServiceLine DTO and to guard :discontinue.
SELECT count(*)
FROM positions
WHERE service_line_id = sqlc.arg(service_line_id)
  AND status = 'active'
  AND deleted_at IS NULL;
