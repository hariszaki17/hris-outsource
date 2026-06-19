// Package payroll (repository) — PaymentRepo implements svc.PaymentRepository using
// raw pgx queries. amount_enc is CIPHERTEXT — the service encrypts at write and
// decrypts at read. Writes via tx; reads on the pool.
package payroll

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// PaymentRepo implements svc.PaymentRepository backed by raw pgx.
type PaymentRepo struct {
	pool *db.Pool
}

var _ svc.PaymentRepository = (*PaymentRepo)(nil)

// NewPaymentRepo returns a PaymentRepo backed by pool.
func NewPaymentRepo(pool *db.Pool) *PaymentRepo {
	return &PaymentRepo{pool: pool}
}

func (r *PaymentRepo) InsertPayment(ctx context.Context, tx pgx.Tx, p svc.InsertPaymentParams) (dom.PayrollPayment, error) {
	var out dom.PayrollPayment
	err := tx.QueryRow(ctx,
		`INSERT INTO payroll_payments (payslip_id, amount_enc, method, reference_no, evidence_file_id, paid_on, paid_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, payslip_id, amount_enc, method, reference_no, evidence_file_id, paid_on, paid_by, voided_at, void_reason, created_at`,
		p.PayslipID, p.AmountEnc, p.Method, p.ReferenceNo, p.EvidenceFileID, p.PaidOn, p.PaidBy,
	).Scan(&out.ID, &out.PayslipID, &out.AmountEnc, &out.Method, &out.ReferenceNo,
		&out.EvidenceFileID, &out.PaidOn, &out.PaidBy, &out.VoidedAt, &out.VoidReason, &out.CreatedAt)
	if err != nil {
		return dom.PayrollPayment{}, mapErr(err)
	}
	return out, nil
}

func (r *PaymentRepo) ListPayments(ctx context.Context, payslipID string) ([]dom.PayrollPayment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, payslip_id, amount_enc, method, reference_no, evidence_file_id, paid_on, paid_by, voided_at, void_reason, created_at
		 FROM payroll_payments WHERE payslip_id = $1 AND deleted_at IS NULL
		 ORDER BY paid_on DESC, created_at DESC`, payslipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dom.PayrollPayment
	for rows.Next() {
		var p dom.PayrollPayment
		if err := rows.Scan(&p.ID, &p.PayslipID, &p.AmountEnc, &p.Method, &p.ReferenceNo,
			&p.EvidenceFileID, &p.PaidOn, &p.PaidBy, &p.VoidedAt, &p.VoidReason, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []dom.PayrollPayment{}
	}
	return out, rows.Err()
}

func (r *PaymentRepo) VoidPayment(ctx context.Context, tx pgx.Tx, id string, reason string) (dom.PayrollPayment, error) {
	var out dom.PayrollPayment
	err := tx.QueryRow(ctx,
		`UPDATE payroll_payments SET voided_at = now(), void_reason = $2
		 WHERE id = $1 AND voided_at IS NULL AND deleted_at IS NULL
		 RETURNING id, payslip_id, amount_enc, method, reference_no, evidence_file_id, paid_on, paid_by, voided_at, void_reason, created_at`,
		id, nullStrPg(reason),
	).Scan(&out.ID, &out.PayslipID, &out.AmountEnc, &out.Method, &out.ReferenceNo,
		&out.EvidenceFileID, &out.PaidOn, &out.PaidBy, &out.VoidedAt, &out.VoidReason, &out.CreatedAt)
	if err != nil {
		return dom.PayrollPayment{}, mapErr(err)
	}
	return out, nil
}

func (r *PaymentRepo) UpdatePayslipPaymentStatus(ctx context.Context, tx pgx.Tx, payslipID string, status string, paidOn time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE payslips SET payment_status = $2, paid_on = $3 WHERE id = $1`,
		payslipID, status, paidOn)
	return err
}

func (r *PaymentRepo) GetPayslipForPayment(ctx context.Context, tx pgx.Tx, payslipID string) (svc.PayslipPaymentRow, error) {
	var out svc.PayslipPaymentRow
	err := tx.QueryRow(ctx,
		`SELECT id, payment_status, is_posted FROM payslips WHERE id = $1 AND deleted_at IS NULL`, payslipID,
	).Scan(&out.ID, &out.PaymentStatus, &out.IsPosted)
	if err != nil {
		return svc.PayslipPaymentRow{}, mapErr(err)
	}
	return out, nil
}
