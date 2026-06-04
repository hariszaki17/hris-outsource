-- E6 leave-approval decision-trail queries. Append-only; drives the
-- LeaveRequest.timeline[] the FE renders (ordered by occurred_at).

-- name: InsertLeaveApproval :one
-- One immutable decision row per approval action (L1/HR approve, override, reject).
INSERT INTO leave_approvals (
    leave_request_id, stage, decision, actor_id, actor_role,
    decision_note, reject_reason, is_override, override_reason
) VALUES (
    sqlc.arg(leave_request_id),
    sqlc.arg(stage),
    sqlc.arg(decision),
    sqlc.narg(actor_id),
    sqlc.narg(actor_role),
    sqlc.narg(decision_note),
    sqlc.narg(reject_reason),
    sqlc.arg(is_override),
    sqlc.narg(override_reason)
)
RETURNING id, leave_request_id, stage, decision, actor_id, actor_role,
          decision_note, reject_reason, is_override, override_reason, occurred_at;

-- name: ListLeaveApprovalsForRequest :many
-- Timeline source: all decisions for a request, chronological.
SELECT la.id, la.leave_request_id, la.stage, la.decision, la.actor_id, la.actor_role,
       la.decision_note, la.reject_reason, la.is_override, la.override_reason, la.occurred_at
FROM leave_approvals la
WHERE la.leave_request_id = sqlc.arg(leave_request_id)
ORDER BY la.occurred_at ASC, la.id ASC;
