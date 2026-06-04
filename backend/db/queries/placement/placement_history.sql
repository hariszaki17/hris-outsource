-- E3 placement_history queries — one row per lifecycle transition.

-- name: InsertPlacementHistory :one
INSERT INTO placement_history (
    placement_id, action, actor_user_id, reason, effective_date,
    status_before, status_after, notes
) VALUES (
    sqlc.arg(placement_id),
    sqlc.arg(action),
    sqlc.narg(actor_user_id),
    sqlc.narg(reason),
    sqlc.narg(effective_date),
    sqlc.narg(status_before),
    sqlc.narg(status_after),
    sqlc.narg(notes)
)
RETURNING id, placement_id, action, actor_user_id, reason, effective_date,
          status_before, status_after, notes, created_at;

-- name: ListPlacementHistory :many
SELECT id, placement_id, action, actor_user_id, reason, effective_date,
       status_before, status_after, notes, created_at
FROM placement_history
WHERE placement_id = sqlc.arg(placement_id)
ORDER BY created_at ASC, id ASC;
