-- F8.3/F8.4 Payroll run + payment queries (SWP-PRR-* / SWP-PPY-*).
-- Monetary amount_enc comes back as CIPHERTEXT bytea — the service decrypts at the
-- boundary via internal/platform/crypto, NOT in SQL. Run list is keyset-paginated
-- on (year DESC, month DESC); payment list is keyset on (paid_on DESC, created_at DESC).

-- ============================================================================
-- payroll_runs — FSM (DRAFT -> POSTED)
-- ============================================================================

-- name: InsertPayrollRun :one
INSERT INTO payroll_runs (year, month, cutoff_date, created_by)
VALUES ($1, $2, $3, $4)
RETURNING id, year, month, status, cutoff_date, created_by, posted_by, posted_at, created_at, updated_at;

-- name: GetPayrollRun :one
SELECT id, year, month, status, cutoff_date, created_by, posted_by, posted_at, created_at, updated_at
FROM payroll_runs WHERE id = $1 AND deleted_at IS NULL;

-- name: GetPayrollRunByPeriod :one
SELECT id, year, month, status, cutoff_date, created_by, posted_by, posted_at, created_at, updated_at
FROM payroll_runs WHERE year = $1 AND month = $2 AND deleted_at IS NULL;

-- name: ListPayrollRuns :many
SELECT id, year, month, status, cutoff_date, created_by, posted_by, posted_at, created_at, updated_at
FROM payroll_runs WHERE deleted_at IS NULL
ORDER BY year DESC, month DESC
LIMIT $1 OFFSET $2;

-- name: PostPayrollRun :one
UPDATE payroll_runs SET status = 'POSTED', posted_by = $2, posted_at = now(), updated_at = now()
WHERE id = $1 AND status = 'DRAFT' AND deleted_at IS NULL
RETURNING id, year, month, status, cutoff_date, created_by, posted_by, posted_at, created_at, updated_at;

-- ============================================================================
-- payroll_payments
-- ============================================================================

-- name: InsertPayrollPayment :one
INSERT INTO payroll_payments (payslip_id, amount_enc, method, reference_no, evidence_file_id, paid_on, paid_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, payslip_id, amount_enc, method, reference_no, evidence_file_id, paid_on, paid_by, voided_at, void_reason, created_at;

-- name: ListPayments :many
SELECT id, payslip_id, amount_enc, method, reference_no, evidence_file_id, paid_on, paid_by, voided_at, void_reason, created_at
FROM payroll_payments WHERE payslip_id = $1 AND deleted_at IS NULL
ORDER BY paid_on DESC, created_at DESC;

-- name: VoidPayment :one
UPDATE payroll_payments SET voided_at = now(), void_reason = $2
WHERE id = $1 AND voided_at IS NULL AND deleted_at IS NULL
RETURNING id, payslip_id, amount_enc, method, reference_no, evidence_file_id, paid_on, paid_by, voided_at, void_reason, created_at;

-- name: UpdatePayslipPaymentStatus :exec
UPDATE payslips SET payment_status = $2, paid_on = $3
WHERE id = $1;

-- ============================================================================
-- generated payslips (F8.3 run)
-- ============================================================================

-- name: InsertGeneratedPayslip :one
INSERT INTO payslips (employee_id, employee_name, placement_id, year, month,
    working_days, gross_earnings_enc, gross_deductions_enc, take_home_pay_enc,
    status, source_system, source_id, payroll_run_id, is_posted, source_type, payment_status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING id, employee_id, employee_name, placement_id, year, month, paid_on,
    working_days, gross_earnings_enc, gross_deductions_enc, take_home_pay_enc,
    status, source_system, source_id, payroll_run_id, is_posted, source_type, payment_status, created_at;

-- name: ListRunPayslips :many
SELECT id, employee_id, employee_name, placement_id, year, month, paid_on,
    working_days, gross_earnings_enc, gross_deductions_enc, take_home_pay_enc,
    status, source_system, source_id, payroll_run_id, is_posted, source_type, payment_status, created_at
FROM payslips WHERE payroll_run_id = $1 AND deleted_at IS NULL
ORDER BY employee_id ASC LIMIT $2;

-- ============================================================================
-- payroll adjustments (carry-forward from F8.5)
-- ============================================================================

-- name: ListPendingAdjustments :many
SELECT id, employee_id, source_type, source_id, origin_year, origin_month, note, amount, status, applied_run_id, created_at, updated_at
FROM payroll_adjustments
WHERE employee_id = $1 AND status = 'PENDING' AND deleted_at IS NULL
ORDER BY origin_year, origin_month, id;

-- name: MarkAdjustmentsApplied :exec
UPDATE payroll_adjustments SET status = 'APPLIED', applied_run_id = $2, updated_at = now()
WHERE id = ANY($1::text[]) AND status = 'PENDING';

-- ============================================================================
-- eligibility lookup (F8.3 assembly)
-- ============================================================================

-- name: GetPayrollAgreementForEmployee :one
SELECT id, employee_id, base_salary_idr, start_date, end_date, status
FROM employment_agreements
WHERE employee_id = $1 AND status = 'active' AND deleted_at IS NULL
LIMIT 1;

-- name: ListEligibleEmployees :many
SELECT DISTINCT e.id, e.full_name, e.employee_type
FROM employees e
JOIN employment_agreements ea ON ea.employee_id = e.id
WHERE ea.status = 'active' AND ea.deleted_at IS NULL AND e.deleted_at IS NULL
  AND ea.start_date <= $2::date
  AND (ea.end_date IS NULL OR ea.end_date >= $1::date)
ORDER BY e.id;

-- name: GetPayslipPaymentInfo :one
SELECT id, payment_status, is_posted
FROM payslips WHERE id = $1 AND deleted_at IS NULL;
