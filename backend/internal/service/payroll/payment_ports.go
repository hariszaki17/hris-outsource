// Package payroll — F8.4 Payment Recording service ports (SWP-PPY-*).
// Consumer-defined repository interface the PaymentService depends on.
// Reads on the pool; writes via q.WithTx(tx).
package payroll

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
)

// PaymentRow is the raw sqlc row for a payroll_payment (amount_enc is ciphertext).
type PaymentRow struct {
	ID             string
	PayslipID      string
	AmountEnc      []byte
	Method         string
	ReferenceNo    *string
	EvidenceFileID *string
	PaidOn         time.Time
	PaidBy         string
	VoidedAt       *time.Time
	VoidReason     *string
	CreatedAt      time.Time
}

// PaymentRepository is the data dependency for the payment service.
type PaymentRepository interface {
	InsertPayment(ctx context.Context, tx pgx.Tx, p InsertPaymentParams) (dom.PayrollPayment, error)
	ListPayments(ctx context.Context, payslipID string) ([]dom.PayrollPayment, error)
	VoidPayment(ctx context.Context, tx pgx.Tx, id string, reason string) (dom.PayrollPayment, error)
	UpdatePayslipPaymentStatus(ctx context.Context, tx pgx.Tx, payslipID string, status string, paidOn time.Time) error

	GetPayslipForPayment(ctx context.Context, tx pgx.Tx, payslipID string) (PayslipPaymentRow, error)
}

// PayslipPaymentRow is the payslip row necessary for payment validation.
type PayslipPaymentRow struct {
	ID            string
	PaymentStatus string
	IsPosted      bool
}

// InsertPaymentParams is the payload for inserting a payroll payment.
type InsertPaymentParams struct {
	PayslipID      string
	AmountEnc      []byte
	Method         string
	ReferenceNo    *string
	EvidenceFileID *string
	PaidOn         time.Time
	PaidBy         string
}
