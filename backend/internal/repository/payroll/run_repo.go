// Package payroll (repository) — RunRepo implements svc.RunRepository using raw
// pgx queries. Reads on the pool; writes via tx. pgx.ErrNoRows → domain.ErrNotFound.
package payroll

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// RunRepo implements svc.RunRepository backed by raw pgx.
type RunRepo struct {
	pool *db.Pool
}

var _ svc.RunRepository = (*RunRepo)(nil)

// NewRunRepo returns a RunRepo backed by pool.
func NewRunRepo(pool *db.Pool) *RunRepo {
	return &RunRepo{pool: pool}
}

// --- run FSM ---

func (r *RunRepo) InsertRun(ctx context.Context, tx pgx.Tx, year, month int, cutoffDate time.Time, createdBy string) (dom.PayrollRun, error) {
	var out dom.PayrollRun
	err := tx.QueryRow(ctx,
		`INSERT INTO payroll_runs (year, month, cutoff_date, created_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, year, month, status, cutoff_date, created_by, posted_by, posted_at, created_at, updated_at`,
		year, month, cutoffDate, createdBy,
	).Scan(&out.ID, &out.Year, &out.Month, &out.Status, &out.CutoffDate,
		&out.CreatedBy, &out.PostedBy, &out.PostedAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return dom.PayrollRun{}, mapErr(err)
	}
	return out, nil
}

func (r *RunRepo) GetRun(ctx context.Context, id string) (dom.PayrollRun, error) {
	var out dom.PayrollRun
	err := r.pool.QueryRow(ctx,
		`SELECT id, year, month, status, cutoff_date, created_by, posted_by, posted_at, created_at, updated_at
		 FROM payroll_runs WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&out.ID, &out.Year, &out.Month, &out.Status, &out.CutoffDate,
		&out.CreatedBy, &out.PostedBy, &out.PostedAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return dom.PayrollRun{}, mapErr(err)
	}
	return out, nil
}

func (r *RunRepo) GetRunByPeriod(ctx context.Context, year, month int) (dom.PayrollRun, error) {
	var out dom.PayrollRun
	err := r.pool.QueryRow(ctx,
		`SELECT id, year, month, status, cutoff_date, created_by, posted_by, posted_at, created_at, updated_at
		 FROM payroll_runs WHERE year = $1 AND month = $2 AND deleted_at IS NULL`, year, month,
	).Scan(&out.ID, &out.Year, &out.Month, &out.Status, &out.CutoffDate,
		&out.CreatedBy, &out.PostedBy, &out.PostedAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return dom.PayrollRun{}, mapErr(err)
	}
	return out, nil
}

func (r *RunRepo) ListRuns(ctx context.Context, limit, offset int) ([]dom.PayrollRun, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, year, month, status, cutoff_date, created_by, posted_by, posted_at, created_at, updated_at
		 FROM payroll_runs WHERE deleted_at IS NULL
		 ORDER BY year DESC, month DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dom.PayrollRun
	for rows.Next() {
		var r dom.PayrollRun
		if err := rows.Scan(&r.ID, &r.Year, &r.Month, &r.Status, &r.CutoffDate,
			&r.CreatedBy, &r.PostedBy, &r.PostedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []dom.PayrollRun{}
	}
	return out, rows.Err()
}

func (r *RunRepo) PostRun(ctx context.Context, tx pgx.Tx, id string, postedBy string) (dom.PayrollRun, error) {
	var out dom.PayrollRun
	err := tx.QueryRow(ctx,
		`UPDATE payroll_runs SET status = 'POSTED', posted_by = $2, posted_at = now(), updated_at = now()
		 WHERE id = $1 AND status = 'DRAFT' AND deleted_at IS NULL
		 RETURNING id, year, month, status, cutoff_date, created_by, posted_by, posted_at, created_at, updated_at`,
		id, nullStrPg(postedBy),
	).Scan(&out.ID, &out.Year, &out.Month, &out.Status, &out.CutoffDate,
		&out.CreatedBy, &out.PostedBy, &out.PostedAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return dom.PayrollRun{}, mapErr(err)
	}
	return out, nil
}

// --- eligibility ---

func (r *RunRepo) ListEligibleEmployees(ctx context.Context, periodStart, periodEnd time.Time) ([]svc.EligibleEmployeeRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT e.id, e.full_name, e.employee_type
		 FROM employees e
		 JOIN employment_agreements ea ON ea.employee_id = e.id
		 WHERE ea.status = 'active' AND ea.deleted_at IS NULL AND e.deleted_at IS NULL
		   AND ea.start_date <= $2::date
		   AND (ea.end_date IS NULL OR ea.end_date >= $1::date)
		 ORDER BY e.id`, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []svc.EligibleEmployeeRow
	for rows.Next() {
		var r svc.EligibleEmployeeRow
		if err := rows.Scan(&r.ID, &r.FullName, &r.EmployeeType); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []svc.EligibleEmployeeRow{}
	}
	return out, rows.Err()
}

func (r *RunRepo) GetActiveAgreementForEmployee(ctx context.Context, employeeID string) (svc.ActiveAgreementRow, error) {
	var out svc.ActiveAgreementRow
	err := r.pool.QueryRow(ctx,
		`SELECT id, employee_id, base_salary_idr, start_date, end_date, status
		 FROM employment_agreements
		 WHERE employee_id = $1 AND status = 'active' AND deleted_at IS NULL
		 LIMIT 1`, employeeID,
	).Scan(&out.ID, &out.EmployeeID, &out.BaseSalaryIDR, &out.StartDate, &out.EndDate, &out.Status)
	if err != nil {
		return svc.ActiveAgreementRow{}, mapErr(err)
	}
	return out, nil
}

func (r *RunRepo) ListPendingAdjustments(ctx context.Context, employeeID string) ([]dom.PayrollAdjustment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, employee_id, source_type, source_id, origin_year, origin_month, note, amount, status, applied_run_id, created_at, updated_at
		 FROM payroll_adjustments
		 WHERE employee_id = $1 AND status = 'PENDING' AND deleted_at IS NULL
		 ORDER BY origin_year, origin_month, id`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dom.PayrollAdjustment
	for rows.Next() {
		var a dom.PayrollAdjustment
		if err := rows.Scan(&a.ID, &a.EmployeeID, &a.SourceType, &a.SourceID,
			&a.OriginYear, &a.OriginMonth, &a.Note, &a.Amount, &a.Status, &a.AppliedRunID,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if out == nil {
		out = []dom.PayrollAdjustment{}
	}
	return out, rows.Err()
}

func (r *RunRepo) MarkAdjustmentsApplied(ctx context.Context, tx pgx.Tx, adjustmentIDs []string, runID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE payroll_adjustments SET status = 'APPLIED', applied_run_id = $2, updated_at = now()
		 WHERE id = ANY($1::text[]) AND status = 'PENDING'`, adjustmentIDs, runID)
	return err
}

// --- generated payslips ---

func (r *RunRepo) InsertGeneratedPayslip(ctx context.Context, tx pgx.Tx, p svc.InsertPayslipParams) (svc.GenPayslipRow, error) {
	var out svc.GenPayslipRow
	err := tx.QueryRow(ctx,
		`INSERT INTO payslips (employee_id, employee_name, placement_id, year, month,
		     working_days, gross_earnings_enc, gross_deductions_enc, take_home_pay_enc,
		     status, source_system, source_id, payroll_run_id, is_posted, source_type, payment_status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		 RETURNING id, employee_id, employee_name, placement_id, year, month, paid_on,
		     working_days, gross_earnings_enc, gross_deductions_enc, take_home_pay_enc,
		     status, source_system, source_id, payroll_run_id, is_posted, source_type, payment_status, created_at`,
		p.EmployeeID, nullStrPg(p.EmployeeName), p.PlacementID, p.Year, p.Month,
		p.WorkingDays, p.GrossEarningsEnc, p.GrossDeductionsEnc, p.TakeHomePayEnc,
		p.Status, p.SourceSystem, p.SourceID, nullStrPg(p.PayrollRunID),
		p.IsPosted, p.SourceType, p.PaymentStatus,
	).Scan(&out.ID, &out.EmployeeID, &out.EmployeeName, &out.PlacementID,
		&out.Year, &out.Month, &out.PaidOn,
		&out.WorkingDays, &out.GrossEarningsEnc, &out.GrossDeductionsEnc, &out.TakeHomePayEnc,
		&out.Status, &out.SourceSystem, &out.SourceID, &out.PayrollRunID,
		&out.IsPosted, &out.SourceType, &out.PaymentStatus, &out.CreatedAt)
	if err != nil {
		return svc.GenPayslipRow{}, err
	}
	return out, nil
}

func (r *RunRepo) ListRunPayslips(ctx context.Context, runID string, limit int) ([]svc.GenPayslipRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, employee_id, employee_name, placement_id, year, month, paid_on,
		     working_days, gross_earnings_enc, gross_deductions_enc, take_home_pay_enc,
		     status, source_system, source_id, payroll_run_id, is_posted, source_type, payment_status, created_at
		 FROM payslips WHERE payroll_run_id = $1 AND deleted_at IS NULL
		 ORDER BY employee_id ASC LIMIT $2`, runID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []svc.GenPayslipRow
	for rows.Next() {
		var r svc.GenPayslipRow
		if err := rows.Scan(&r.ID, &r.EmployeeID, &r.EmployeeName, &r.PlacementID,
			&r.Year, &r.Month, &r.PaidOn,
			&r.WorkingDays, &r.GrossEarningsEnc, &r.GrossDeductionsEnc, &r.TakeHomePayEnc,
			&r.Status, &r.SourceSystem, &r.SourceID, &r.PayrollRunID,
			&r.IsPosted, &r.SourceType, &r.PaymentStatus, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []svc.GenPayslipRow{}
	}
	return out, rows.Err()
}

func (r *RunRepo) RunExists(ctx context.Context, id string) (bool, error) {
	_, err := r.GetRun(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *RunRepo) CountRunPayslips(ctx context.Context, runID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payslips WHERE payroll_run_id = $1 AND deleted_at IS NULL`, runID).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// nullStrPg returns nil for an empty string (pgx sends NULL).
func nullStrPg(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
