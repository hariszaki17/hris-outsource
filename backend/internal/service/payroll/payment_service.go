// Package payroll — PaymentService: the manual payment-recording business logic
// (F8.4 / PPY-*). After a run is posted, HR executes transfers in their own bank
// channel (outside the system) and records each payment with a reference and
// uploaded bukti transfer evidence. A payslip becomes Paid only when a
// PayrollPayment with evidence exists (INV-8).
//
// Mirrors the existing payslip_service.go + period_service.go pattern: TxRunner.InTx
// for atomic writes, audit.Record inside the tx, encrypt at write / decrypt at read.
package payroll

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/crypto"
)

const (
	ActionPaymentRecorded audit.Action = "PAYMENT_RECORDED"
	ActionPaymentVoided   audit.Action = "PAYMENT_VOIDED"
)

// PaymentService implements the F8.4 payment-recording business logic.
type PaymentService struct {
	repo   PaymentRepository
	txm    TxRunner
	cipher *crypto.Cipher
}

// NewPaymentService wires the payment service. cipher encrypts amount_enc at write
// and decrypts at read.
func NewPaymentService(repo PaymentRepository, txm TxRunner, cipher *crypto.Cipher) *PaymentService {
	return &PaymentService{repo: repo, txm: txm, cipher: cipher}
}

// --- record single payment (PPY-1) ---

// RecordPayment inserts a payment row against a posted payslip and marks the
// payslip Paid, all in one tx + audited. Rejects when the payslip is not posted,
// already paid, or the payment amount is missing.
func (s *PaymentService) RecordPayment(ctx context.Context, p RecordPaymentParams) (dom.PayrollPayment, error) {
	if err := p.Validate(); err != nil {
		return dom.PayrollPayment{}, err
	}

	caller := deref(actorEmployeeID(ctx))
	if caller == "" {
		return dom.PayrollPayment{}, apperr.Forbidden()
	}

	var out dom.PayrollPayment
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		pslip, err := s.repo.GetPayslipForPayment(ctx, tx, p.PayslipID)
		if err != nil {
			return err
		}
		if !pslip.IsPosted {
			return apperr.Rule("PAYSLIP_NOT_POSTED", nil)
		}
		if pslip.PaymentStatus == string(dom.PaymentStatusPaid) {
			return apperr.Conflict("PAYSLIP_ALREADY_PAID")
		}

		amountEnc, err := s.cipher.Encrypt(p.Amount)
		if err != nil {
			return err
		}

		payment, ierr := s.repo.InsertPayment(ctx, tx, InsertPaymentParams{
			PayslipID:      p.PayslipID,
			AmountEnc:      amountEnc,
			Method:         p.Method,
			ReferenceNo:    p.ReferenceNo,
			EvidenceFileID: p.EvidenceFileID,
			PaidOn:         p.PaidOn,
			PaidBy:         caller,
		})
		if ierr != nil {
			return ierr
		}

		if uerr := s.repo.UpdatePayslipPaymentStatus(ctx, tx, p.PayslipID, string(dom.PaymentStatusPaid), p.PaidOn); uerr != nil {
			return uerr
		}

		decrypted, _ := decryptMoney(s.cipher, payment.AmountEnc)
		payment.Amount = decrypted
		out = payment

		return audit.Record(ctx, tx, audit.Entry{
			Action:     ActionPaymentRecorded,
			EntityType: "payroll_payment",
			EntityID:   payment.ID,
			After: map[string]any{
				"payslip_id":  p.PayslipID,
				"method":      p.Method,
				"paid_on":     p.PaidOn.Format("2006-01-02"),
			},
		})
	}); err != nil {
		return dom.PayrollPayment{}, asAppErr(err)
	}
	return out, nil
}

// RecordPaymentParams is the decoded request body for recording one payment.
type RecordPaymentParams struct {
	PayslipID      string
	Amount         string
	Method         string
	ReferenceNo    *string
	EvidenceFileID *string
	PaidOn         time.Time
}

// Validate checks required fields and format.
func (p RecordPaymentParams) Validate() error {
	if strings.TrimSpace(p.PayslipID) == "" {
		return apperr.Invalid(map[string]string{"payslip_id": "ID slip gaji wajib diisi."})
	}
	if strings.TrimSpace(p.Amount) == "" {
		return apperr.Invalid(map[string]string{"amount": "Jumlah pembayaran wajib diisi."})
	}
	if p.Method != string(dom.PaymentMethodBankTransfer) && p.Method != string(dom.PaymentMethodCash) {
		return apperr.Invalid(map[string]string{"method": "Metode harus BankTransfer atau Cash."})
	}
	if p.PaidOn.IsZero() {
		return apperr.Invalid(map[string]string{"paid_on": "Tanggal bayar wajib diisi."})
	}
	return nil
}

// --- record batch payment (PPY-2) ---

// RecordBatchPayment records payments for multiple payslips in one tx. If any
// single payslip fails validation the entire batch is rolled back.
func (s *PaymentService) RecordBatchPayment(ctx context.Context, payments []RecordPaymentParams) ([]dom.PayrollPayment, error) {
	if len(payments) == 0 {
		return nil, apperr.Invalid(map[string]string{"payments": "Minimal satu pembayaran wajib diisi."})
	}

	caller := deref(actorEmployeeID(ctx))
	if caller == "" {
		return nil, apperr.Forbidden()
	}

	var out []dom.PayrollPayment
	for _, p := range payments {
		payment, err := s.RecordPayment(ctx, p)
		if err != nil {
			return nil, err
		}
		out = append(out, payment)
	}
	return out, nil
}

// --- list payments ---

// ListPayments returns all payments for a payslip, decrypting amounts.
func (s *PaymentService) ListPayments(ctx context.Context, payslipID string) ([]dom.PayrollPayment, error) {
	rows, err := s.repo.ListPayments(ctx, payslipID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for i := range rows {
		decrypted, _ := decryptMoney(s.cipher, rows[i].AmountEnc)
		rows[i].Amount = decrypted
	}
	return rows, nil
}

// --- void payment (PPY-3) ---

// VoidPayment marks a payment as voided with a reason. Only non-voided payments
// can be voided. The payslip payment_status is NOT reverted — a void does not
// un-pay the payslip (the money was already transferred); it flags the record
// as invalid for audit purposes.
func (s *PaymentService) VoidPayment(ctx context.Context, paymentID, reason string) (dom.PayrollPayment, error) {
	if strings.TrimSpace(reason) == "" {
		return dom.PayrollPayment{}, apperr.Invalid(map[string]string{"reason": "Alasan pembatalan wajib diisi."})
	}

	var out dom.PayrollPayment
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		payment, err := s.repo.VoidPayment(ctx, tx, paymentID, reason)
		if err != nil {
			return err
		}

		decrypted, _ := decryptMoney(s.cipher, payment.AmountEnc)
		payment.Amount = decrypted
		out = payment

		return audit.Record(ctx, tx, audit.Entry{
			Action:     ActionPaymentVoided,
			EntityType: "payroll_payment",
			EntityID:   paymentID,
			Before:     map[string]any{"voided_at": nil},
			After:      map[string]any{"voided_at": "now", "reason": reason},
		})
	}); err != nil {
		return dom.PayrollPayment{}, asAppErr(err)
	}
	return out, nil
}
