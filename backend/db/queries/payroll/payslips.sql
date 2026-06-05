-- E8 payroll queries (F8.1/F8.2 / SWP-PS-*). Historical, read-only archive.
-- Monetary columns come back as ENCRYPTED ciphertext (gross_*_enc /
-- take_home_pay_enc / value_enc bytea) — the 10-02 repo/service decrypts at the
-- boundary via internal/platform/crypto, NOT in SQL. paid_on comes back as
-- pgtype.Date (10-02 repo converts <-> time.Time like Phase-5/6/8/9). Keyset
-- cursor on (paid_on DESC NULLS LAST, id DESC) per CONVENTIONS §11 (default sort
-- paid_on:desc).

-- name: ListPayslips :many
-- Archive / list load. Keyset cursor on (paid_on, id) DESC, NULL paid_on sorts
-- last. Filters (all optional via narg): employee_id, year, month (from the
-- period YYYY-MM), status. Summary only — NO components/benefits join. Returns
-- the *_enc ciphertext (repo decrypts in 10-02).
SELECT p.id, p.employee_id, p.employee_name, p.placement_id, p.year, p.month,
       p.paid_on, p.working_days, p.gross_earnings_enc, p.gross_deductions_enc,
       p.take_home_pay_enc, p.status, p.source_system, p.source_id,
       p.created_at, p.updated_at
FROM payslips p
WHERE p.deleted_at IS NULL
  AND (sqlc.narg(employee_id)::text IS NULL OR p.employee_id = sqlc.narg(employee_id)::text)
  AND (sqlc.narg(year)::int        IS NULL OR p.year         = sqlc.narg(year)::int)
  AND (sqlc.narg(month)::int       IS NULL OR p.month        = sqlc.narg(month)::int)
  AND (sqlc.narg(status)::text     IS NULL OR p.status       = sqlc.narg(status)::text)
  -- Keyset cursor under the ORDER BY below. We sort by a COALESCE sentinel
  -- (NULL paid_on -> '0001-01-01', so it sorts last under DESC) tupled with id,
  -- and page strictly past the cursor tuple. 10-02 encodes (paid_on, id) into
  -- the opaque cursor; a NULL paid_on row encodes the sentinel date.
  AND (
    sqlc.narg(cursor_id)::text IS NULL
    OR (COALESCE(p.paid_on, DATE '0001-01-01'), p.id)
       < (COALESCE(sqlc.narg(cursor_paid_on)::date, DATE '0001-01-01'), sqlc.narg(cursor_id)::text)
  )
ORDER BY COALESCE(p.paid_on, DATE '0001-01-01') DESC, p.id DESC
LIMIT sqlc.arg(lim);

-- name: GetPayslip :one
-- Single payslip with all columns incl. ENCRYPTED money (for GET /payslips/{id}).
SELECT p.id, p.employee_id, p.employee_name, p.placement_id, p.year, p.month,
       p.paid_on, p.working_days, p.gross_earnings_enc, p.gross_deductions_enc,
       p.take_home_pay_enc, p.status, p.source_system, p.source_id,
       p.created_at, p.updated_at
FROM payslips p
WHERE p.id = sqlc.arg(id)
  AND p.deleted_at IS NULL;

-- name: ListPayslipComponents :many
-- Earnings + deductions breakdown for the detail view. value_enc is ENCRYPTED
-- (decrypted at the boundary). Ordered kind then sort_order.
SELECT id, payslip_id, kind, name, value_enc, for_bpjs, sort_order
FROM payslip_components
WHERE payslip_id = sqlc.arg(payslip_id)
ORDER BY kind, sort_order, id;

-- name: ListPayslipBenefits :many
-- Employer-borne benefits (HR-only). value_enc ENCRYPTED.
SELECT id, payslip_id, name, value_enc, sort_order
FROM payslip_benefits
WHERE payslip_id = sqlc.arg(payslip_id)
ORDER BY sort_order, id;

-- name: InsertPayslip :one
-- Seed path (10-02 inserts ciphertext produced by the crypto helper). id
-- allocated by the column DEFAULT ('SWP-PS-' || swp_next_id('PS')) when omitted,
-- OR supplied explicitly (deterministic E2E targets) via ON CONFLICT (id) DO NOTHING.
INSERT INTO payslips (
    id, employee_id, employee_name, placement_id, year, month, paid_on,
    working_days, gross_earnings_enc, gross_deductions_enc, take_home_pay_enc,
    status, source_system, source_id
) VALUES (
    COALESCE(sqlc.narg(id)::text, 'SWP-PS-' || swp_next_id('PS')),
    sqlc.arg(employee_id),
    sqlc.narg(employee_name),
    sqlc.narg(placement_id),
    sqlc.arg(year),
    sqlc.arg(month),
    sqlc.narg(paid_on),
    sqlc.narg(working_days),
    sqlc.narg(gross_earnings_enc),
    sqlc.narg(gross_deductions_enc),
    sqlc.narg(take_home_pay_enc),
    sqlc.arg(status),
    sqlc.arg(source_system),
    sqlc.arg(source_id)
)
ON CONFLICT (id) DO NOTHING
RETURNING id, employee_id, employee_name, placement_id, year, month, paid_on,
          working_days, gross_earnings_enc, gross_deductions_enc, take_home_pay_enc,
          status, source_system, source_id, created_at, updated_at;

-- name: InsertPayslipComponent :one
-- Seed: one earning/deduction line (value_enc = crypto.Encrypt of the Money string).
INSERT INTO payslip_components (payslip_id, kind, name, value_enc, for_bpjs, sort_order)
VALUES (sqlc.arg(payslip_id), sqlc.arg(kind), sqlc.arg(name),
        sqlc.narg(value_enc), sqlc.arg(for_bpjs), sqlc.arg(sort_order))
RETURNING id, payslip_id, kind, name, value_enc, for_bpjs, sort_order;

-- name: InsertPayslipBenefit :one
-- Seed: one employer-borne benefit line.
INSERT INTO payslip_benefits (payslip_id, name, value_enc, sort_order)
VALUES (sqlc.arg(payslip_id), sqlc.arg(name), sqlc.narg(value_enc), sqlc.arg(sort_order))
RETURNING id, payslip_id, name, value_enc, sort_order;
