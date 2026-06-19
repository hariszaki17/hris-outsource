// Package payroll (handler) — F8.4 Payment Recording handlers (§9). Hand-written
// chi handlers: decode -> service -> httpx.WriteJSON; apperr envelopes flow through
// httpx.WriteError. Single objects wrap in {data}. Mirrors the existing
// payslip_handler.go pattern.
package payroll

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// PaymentHandler holds the F8.4 payment-recording service.
type PaymentHandler struct {
	payment *svc.PaymentService
}

// NewPaymentHandler wires the F8.4 handler.
func NewPaymentHandler(p *svc.PaymentService) *PaymentHandler {
	return &PaymentHandler{payment: p}
}

// RecordPayment handles POST /payments — records a single payment against a payslip.
// Idempotency-Key required.
func (h *PaymentHandler) RecordPayment(w http.ResponseWriter, r *http.Request) {
	var req recordPaymentRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	paidOn, err := time.Parse("2006-01-02", req.PaidOn)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"paid_on": "Format tanggal harus YYYY-MM-DD."}))
		return
	}

	params := svc.RecordPaymentParams{
		PayslipID:      req.PayslipID,
		Amount:         req.Amount,
		Method:         req.Method,
		ReferenceNo:    req.ReferenceNo,
		EvidenceFileID: req.EvidenceFileID,
		PaidOn:         paidOn,
	}

	payment, err := h.payment.RecordPayment(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/payments/"+payment.ID)
	httpx.WriteJSON(w, http.StatusCreated, dataResponse[paymentResponse]{Data: toPaymentResponse(payment)})
}

// RecordBatchPayment handles POST /payments:batch — records payments for multiple
// payslips. Idempotency-Key required.
func (h *PaymentHandler) RecordBatchPayment(w http.ResponseWriter, r *http.Request) {
	var req recordBatchPaymentRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	params := make([]svc.RecordPaymentParams, 0, len(req.Payments))
	for _, p := range req.Payments {
		paidOn, err := time.Parse("2006-01-02", p.PaidOn)
		if err != nil {
			httpx.WriteError(w, r, apperr.Invalid(map[string]string{"paid_on": "Format tanggal harus YYYY-MM-DD."}))
			return
		}
		params = append(params, svc.RecordPaymentParams{
			PayslipID:      p.PayslipID,
			Amount:         p.Amount,
			Method:         p.Method,
			ReferenceNo:    p.ReferenceNo,
			EvidenceFileID: p.EvidenceFileID,
			PaidOn:         paidOn,
		})
	}

	payments, err := h.payment.RecordBatchPayment(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]paymentResponse, 0, len(payments))
	for _, p := range payments {
		items = append(items, toPaymentResponse(p))
	}
	httpx.WriteJSON(w, http.StatusCreated, dataResponse[[]paymentResponse]{Data: items})
}

// VoidPayment handles POST /payments/{id}:void — marks a payment as voided.
// Idempotency-Key required.
func (h *PaymentHandler) VoidPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req voidPaymentRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	payment, err := h.payment.VoidPayment(r.Context(), id, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, dataResponse[paymentResponse]{Data: toPaymentResponse(payment)})
}

// ListPayments handles GET /payslips/{id}/payments — returns all payments for a
// payslip, decrypting amounts.
func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	payments, err := h.payment.ListPayments(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]paymentResponse, 0, len(payments))
	for _, p := range payments {
		items = append(items, toPaymentResponse(p))
	}
	if items == nil {
		items = []paymentResponse{}
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[[]paymentResponse]{Data: items})
}
