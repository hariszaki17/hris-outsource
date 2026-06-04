-- name: ListAgreements :many
-- Cursor page ordered by (created_at desc, id desc). Fetch limit+1 for has_more.
-- Filters: employee_id, status, type, end_date__lte (agreements expiring on or before).
SELECT id, employee_id, type, agreement_no, start_date, end_date, status,
       predecessor_id, successor_id, closed_reason, closed_at,
       base_salary_idr, bpjs_terms, tax_profile, comp_effective_date,
       created_by, created_at, updated_at
FROM employment_agreements
WHERE deleted_at IS NULL
  AND (sqlc.narg(employee_id)::text IS NULL OR employee_id = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(type)::text IS NULL OR type = sqlc.narg(type)::text)
  AND (sqlc.narg(end_date__lte)::date IS NULL OR end_date <= sqlc.narg(end_date__lte)::date)
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetAgreementByID :one
SELECT id, employee_id, type, agreement_no, start_date, end_date, status,
       predecessor_id, successor_id, closed_reason, closed_at,
       base_salary_idr, bpjs_terms, tax_profile, comp_effective_date,
       created_by, created_at, updated_at
FROM employment_agreements
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: GetActiveAgreementForEmployee :one
-- EA-2 pre-check + predecessor lookup for :renew/:close operations.
SELECT id, employee_id, type, agreement_no, start_date, end_date, status,
       predecessor_id, successor_id, closed_reason, closed_at,
       base_salary_idr, bpjs_terms, tax_profile, comp_effective_date,
       created_by, created_at, updated_at
FROM employment_agreements
WHERE employee_id = sqlc.arg(employee_id)
  AND status = 'active'
  AND deleted_at IS NULL;

-- name: CreateAgreement :one
-- Allocates the SWP-AG id inline from the per-prefix sequence.
INSERT INTO employment_agreements (
    id, employee_id, type, agreement_no, start_date, end_date,
    predecessor_id, base_salary_idr, bpjs_terms, tax_profile, comp_effective_date, created_by
) VALUES (
    'SWP-AG-' || swp_next_id('AG'),
    sqlc.arg(employee_id),
    sqlc.arg(type),
    sqlc.arg(agreement_no),
    sqlc.arg(start_date),
    sqlc.narg(end_date),
    sqlc.narg(predecessor_id),
    sqlc.narg(base_salary_idr),
    sqlc.narg(bpjs_terms),
    sqlc.narg(tax_profile),
    sqlc.narg(comp_effective_date),
    sqlc.narg(created_by)
)
RETURNING id, employee_id, type, agreement_no, start_date, end_date, status,
          predecessor_id, successor_id, closed_reason, closed_at,
          base_salary_idr, bpjs_terms, tax_profile, comp_effective_date,
          created_by, created_at, updated_at;

-- name: SetAgreementStatus :one
-- Drives :close (status='closed') and supersede-on-renew (status='superseded').
-- Also sets closed_reason, closed_at, successor_id as applicable.
UPDATE employment_agreements
SET status        = sqlc.arg(status),
    closed_reason = sqlc.narg(closed_reason),
    closed_at     = sqlc.narg(closed_at),
    successor_id  = sqlc.narg(successor_id),
    updated_at    = now()
WHERE id = sqlc.arg(id)
RETURNING id, employee_id, type, agreement_no, start_date, end_date, status,
          predecessor_id, successor_id, closed_reason, closed_at,
          base_salary_idr, bpjs_terms, tax_profile, comp_effective_date,
          created_by, created_at, updated_at;
