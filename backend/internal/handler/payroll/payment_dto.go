// Package payroll (handler) — F8.4 Payment Recording request/response DTOs
// matching openapi shapes. Money is the 2-decimal decrypted string. Dates are
// strings. Mirrors the existing dto.go convention.
package payroll

import (
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
)

// --- request bodies ---

type recordPaymentRequest struct {
	PayslipID      string  `json:"payslip_id"`
	Amount         string  `json:"amount"`
	Method         string  `json:"method"`
	ReferenceNo    *string `json:"reference_no"`
	EvidenceFileID *string `json:"evidence_file_id"`
	PaidOn         string  `json:"paid_on"`
}

type recordBatchPaymentRequest struct {
	Payments []recordPaymentRequest `json:"payments"`
}

type voidPaymentRequest struct {
	Reason string `json:"reason"`
}

// --- response: PayrollPayment ---

type paymentResponse struct {
	ID             string  `json:"id"`
	PayslipID      string  `json:"payslip_id"`
	Amount         *string `json:"amount"`
	Method         string  `json:"method"`
	ReferenceNo    *string `json:"reference_no,omitempty"`
	EvidenceFileID *string `json:"evidence_file_id,omitempty"`
	PaidOn         string  `json:"paid_on"`
	PaidBy         string  `json:"paid_by"`
	VoidedAt       *string `json:"voided_at,omitempty"`
	VoidReason     *string `json:"void_reason,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

// --- mappers ---

func toPaymentResponse(p dom.PayrollPayment) paymentResponse {
	resp := paymentResponse{
		ID:             p.ID,
		PayslipID:      p.PayslipID,
		Amount:         p.Amount,
		Method:         string(p.Method),
		ReferenceNo:    p.ReferenceNo,
		EvidenceFileID: p.EvidenceFileID,
		PaidOn:         p.PaidOn.Format("2006-01-02"),
		PaidBy:         p.PaidBy,
		CreatedAt:      rfc3339(p.CreatedAt),
	}
	if p.VoidedAt != nil {
		s := p.VoidedAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.VoidedAt = &s
	}
	resp.VoidReason = p.VoidReason
	return resp
}
