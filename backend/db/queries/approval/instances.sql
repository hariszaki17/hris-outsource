-- E11 approval-instance queries (F11.2 / SWP-APV-*). One live run of the engine per
-- domain request (unique request_type+request_id). Keyset cursor on (created_at,id)
-- DESC, mirroring ListLeaveRequests (CONVENTIONS §11). id from the column DEFAULT
-- ('SWP-APV-' || swp_next_id('APV')).

-- name: InsertApprovalInstance :one
-- Build an instance from the company's template (template_id null = super-admin
-- fallback, INV-7). line_count = configured lines (1 for fallback). id from DEFAULT.
INSERT INTO approval_instances (
    request_type, request_id, company_id, template_id, template_version,
    current_line, line_count, status, requester_id
) VALUES (
    sqlc.arg(request_type),
    sqlc.arg(request_id),
    sqlc.narg(company_id),
    sqlc.narg(template_id),
    sqlc.narg(template_version),
    sqlc.arg(current_line),
    sqlc.arg(line_count),
    sqlc.arg(status),
    sqlc.narg(requester_id)
)
RETURNING id, request_type, request_id, company_id, template_id, template_version,
          current_line, line_count, status, requester_id, created_at, updated_at;

-- name: GetApprovalInstance :one
SELECT id, request_type, request_id, company_id, template_id, template_version,
       current_line, line_count, status, requester_id, created_at, updated_at
FROM approval_instances
WHERE id = sqlc.arg(id);

-- name: GetApprovalInstanceForUpdate :one
-- Row-lock for the act/bypass transition (advance line / terminate). The service
-- guards the legal transition before writing.
SELECT id, request_type, request_id, company_id, template_id, template_version,
       current_line, line_count, status, requester_id, created_at, updated_at
FROM approval_instances
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: GetApprovalInstanceByRequest :one
-- Resolve the instance governing a given domain request (leave/OT read linkage).
SELECT id, request_type, request_id, company_id, template_id, template_version,
       current_line, line_count, status, requester_id, created_at, updated_at
FROM approval_instances
WHERE request_type = sqlc.arg(request_type)
  AND request_id   = sqlc.arg(request_id);

-- name: ListApprovalInstances :many
-- Queue / list load. Keyset cursor (created_at,id) DESC. Filters (all optional via
-- narg): company_id, request_type, status.
SELECT id, request_type, request_id, company_id, template_id, template_version,
       current_line, line_count, status, requester_id, created_at, updated_at
FROM approval_instances
WHERE (sqlc.narg(company_id)::text   IS NULL OR company_id   = sqlc.narg(company_id)::text)
  AND (sqlc.narg(request_type)::text IS NULL OR request_type = sqlc.narg(request_type)::text)
  AND (sqlc.narg(status)::text       IS NULL OR status       = sqlc.narg(status)::text)
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL OR
       (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(lim);

-- name: ListApprovalInstancesForMember :many
-- The `mine` inbox (F11.3): PENDING instances whose CURRENT line includes the given
-- member (member_user_id), excluding the member's own requests (INV-3 self-approval
-- block — requester_id != member). Same optional filters (company_id, request_type)
-- + keyset cursor (created_at,id) DESC. The current line is resolved by joining the
-- live template's line at line_no = current_line.
SELECT ai.id, ai.request_type, ai.request_id, ai.company_id, ai.template_id,
       ai.template_version, ai.current_line, ai.line_count, ai.status,
       ai.requester_id, ai.created_at, ai.updated_at
FROM approval_instances ai
JOIN approval_lines al
     ON al.template_id = ai.template_id
    AND al.line_no     = ai.current_line
JOIN approval_line_members alm
     ON alm.line_id = al.id
    AND alm.user_id = sqlc.arg(member_user_id)
WHERE ai.status = 'PENDING'
  AND (ai.requester_id IS NULL OR ai.requester_id <> sqlc.arg(member_user_id))
  AND (sqlc.narg(company_id)::text   IS NULL OR ai.company_id   = sqlc.narg(company_id)::text)
  AND (sqlc.narg(request_type)::text IS NULL OR ai.request_type = sqlc.narg(request_type)::text)
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL OR
       (ai.created_at, ai.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY ai.created_at DESC, ai.id DESC
LIMIT sqlc.arg(lim);

-- name: UpdateApprovalInstanceProgress :exec
-- Advance the chain: set current_line + status after a line clears / terminal
-- reject / bypass.
UPDATE approval_instances
SET current_line = sqlc.arg(current_line),
    status       = sqlc.arg(status),
    updated_at   = now()
WHERE id = sqlc.arg(id);

-- name: ResetPendingInstancesForCompany :exec
-- INV-6 live-template reset: on a template edit, every non-terminal instance for the
-- company restarts at line 1 on the new version (prior actions retained for audit
-- but no longer count).
UPDATE approval_instances
SET current_line     = 1,
    template_version = sqlc.arg(template_version),
    updated_at       = now()
WHERE company_id = sqlc.arg(company_id)
  AND status     = 'PENDING';

-- name: GetCurrentLineMembers :many
-- Members of an instance's CURRENT line — for the membership / self-approval check
-- (INV-2 OR-set, INV-3). Resolves the live template's line at line_no = current_line.
SELECT alm.user_id
FROM approval_instances ai
JOIN approval_lines al
     ON al.template_id = ai.template_id
    AND al.line_no     = ai.current_line
JOIN approval_line_members alm
     ON alm.line_id = al.id
WHERE ai.id = sqlc.arg(instance_id);
