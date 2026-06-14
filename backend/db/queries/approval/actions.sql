-- E11 approval-action queries (F11.2 / SWP-APA-*). Append-only decision trail
-- (INV-9): one row per decision (APPROVE/REJECT/BYPASS), stamped with the
-- template_version in force. id from the column DEFAULT ('SWP-APA-' ||
-- swp_next_id('APA')).

-- name: InsertApprovalAction :one
-- One immutable decision row; written in-tx with the instance progress update.
INSERT INTO approval_actions (
    instance_id, line_no, template_version, actor_user_id, action, reason
) VALUES (
    sqlc.arg(instance_id),
    sqlc.arg(line_no),
    sqlc.narg(template_version),
    sqlc.narg(actor_user_id),
    sqlc.arg(action),
    sqlc.narg(reason)
)
RETURNING id, instance_id, line_no, template_version, actor_user_id, action, reason, created_at;

-- name: ListApprovalActionsByInstance :many
-- The decision timeline for an instance (chronological), joined to users + the
-- linked employee to expose actor_name (employee full_name, falling back to email).
SELECT aa.id, aa.instance_id, aa.line_no, aa.template_version, aa.actor_user_id,
       aa.action, aa.reason, aa.created_at,
       COALESCE(e.full_name, u.email) AS actor_name
FROM approval_actions aa
LEFT JOIN users u     ON u.id = aa.actor_user_id
LEFT JOIN employees e ON e.id = u.employee_id
WHERE aa.instance_id = sqlc.arg(instance_id)
ORDER BY aa.created_at ASC, aa.id ASC;
