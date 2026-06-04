-- name: ListAuditLog :many
-- Cursor page ordered by (created_at desc, id desc), fetch limit+1. All filters optional.
SELECT id, actor_user_id, actor_role, action, entity_type, entity_id,
       before_state, after_state, request_id, created_at
FROM audit_log
WHERE (sqlc.narg(actor_user_id)::text IS NULL OR actor_user_id = sqlc.narg(actor_user_id))
  AND (sqlc.narg(action)::text        IS NULL OR action = sqlc.narg(action))
  AND (sqlc.narg(entity_type)::text   IS NULL OR entity_type = sqlc.narg(entity_type))
  AND (sqlc.narg(entity_id)::text     IS NULL OR entity_id = sqlc.narg(entity_id))
  AND (sqlc.narg(created_gte)::timestamptz IS NULL OR created_at >= sqlc.narg(created_gte))
  AND (sqlc.narg(created_lte)::timestamptz IS NULL OR created_at <= sqlc.narg(created_lte))
  AND (sqlc.narg(q)::text IS NULL OR action ILIKE '%' || sqlc.narg(q) || '%' OR entity_id ILIKE '%' || sqlc.narg(q) || '%')
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetAuditLogByID :one
SELECT id, actor_user_id, actor_role, action, entity_type, entity_id,
       before_state, after_state, request_id, created_at
FROM audit_log
WHERE id = sqlc.arg(id);
