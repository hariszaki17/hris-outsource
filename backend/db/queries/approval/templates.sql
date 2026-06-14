-- E11 approval-template queries (F11.1 / SWP-APT-*, SWP-APL-*). One template per
-- company (INV-1). Lines are ordered 2..3 (INV-2); members are an OR-set per line.
-- IDs allocated by the column DEFAULT ('SWP-APT-' / 'SWP-APL-' || swp_next_id(...)).

-- name: GetApprovalTemplateByCompany :one
-- The company's single template (INV-1). The service then loads lines + members.
SELECT id, company_id, version, created_by, created_at, updated_at
FROM approval_templates
WHERE company_id = sqlc.arg(company_id);

-- name: GetApprovalTemplateByID :one
SELECT id, company_id, version, created_by, created_at, updated_at
FROM approval_templates
WHERE id = sqlc.arg(id);

-- name: ListApprovalLinesByTemplate :many
-- Ordered chain (line_no asc) for a template.
SELECT id, template_id, line_no
FROM approval_lines
WHERE template_id = sqlc.arg(template_id)
ORDER BY line_no ASC;

-- name: ListApprovalLineMembersByTemplate :many
-- All members across a template's lines, joined to users + the linked employee for
-- the LineMember readOnly fields: display_name (employee full_name) and active
-- (an active employment_agreement exists, i.e. employment not ended — TM-3).
-- Ordered by line_no then user_id for deterministic chain rendering.
SELECT alm.line_id,
       al.line_no,
       alm.user_id,
       COALESCE(e.full_name, u.email) AS display_name,
       EXISTS (
           SELECT 1 FROM employment_agreements ea
           WHERE ea.employee_id = u.employee_id
             AND ea.status = 'active'
             AND ea.deleted_at IS NULL
       ) AS active
FROM approval_line_members alm
JOIN approval_lines al ON al.id = alm.line_id
LEFT JOIN users u      ON u.id  = alm.user_id
LEFT JOIN employees e  ON e.id  = u.employee_id
WHERE al.template_id = sqlc.arg(template_id)
ORDER BY al.line_no ASC, alm.user_id ASC;

-- name: InsertApprovalTemplate :one
-- Create a company's template (INV-1 unique company_id). id from column DEFAULT.
INSERT INTO approval_templates (company_id, created_by)
VALUES (sqlc.arg(company_id), sqlc.narg(created_by))
RETURNING id, company_id, version, created_by, created_at, updated_at;

-- name: UpdateApprovalTemplateVersion :one
-- Bump version on every edit (INV-6); the service then re-inserts lines/members and
-- resets pending instances.
UPDATE approval_templates
SET version    = version + 1,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, company_id, version, created_by, created_at, updated_at;

-- name: DeleteApprovalTemplate :exec
-- Drops the template; lines + members cascade.
DELETE FROM approval_templates
WHERE id = sqlc.arg(id);

-- name: InsertApprovalLine :one
-- One line of a template (line_no 1..3, unique within template). id from DEFAULT.
INSERT INTO approval_lines (template_id, line_no)
VALUES (sqlc.arg(template_id), sqlc.arg(line_no))
RETURNING id, template_id, line_no;

-- name: DeleteApprovalLinesByTemplate :exec
-- Clears a template's lines (members cascade) before re-inserting on edit.
DELETE FROM approval_lines
WHERE template_id = sqlc.arg(template_id);

-- name: InsertApprovalLineMember :exec
-- Adds a user to a line's OR-set (composite PK line_id+user_id).
INSERT INTO approval_line_members (line_id, user_id)
VALUES (sqlc.arg(line_id), sqlc.arg(user_id))
ON CONFLICT (line_id, user_id) DO NOTHING;
