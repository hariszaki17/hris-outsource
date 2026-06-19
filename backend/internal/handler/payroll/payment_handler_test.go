// Package payroll_test — F8.4 Payment Recording contract tests (PPY-1/PPY-2/PPY-3).
//
// The drift gate for the 4 payment endpoints, asserted byte-for-shape against
// docs/api/E8-payroll/openapi.yaml:
//
//	POST /payments                  → 201 {data:<PayrollPayment>}
//	POST /payments:batch             → 201 {data:[<PayrollPayment>...]}
//	POST /payments/{id}:void         → 200 {data:<PayrollPayment>}; 409 on
//	                                  already-voided.
//	GET  /payslips/{id}/payments     → 200 {data:[<PayrollPayment>...]}
//
// The fakePaymentRepo implements PaymentRepository fully in-memory so the REAL
// PaymentService runs against it with a REAL crypto.Cipher.
package payroll_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	payrollhandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/crypto"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
)

// ---------------------------------------------------------------------------
// fakePaymentRepo — in-memory svc.PaymentRepository over shared maps.
// ---------------------------------------------------------------------------

type fakePaymentRepo struct {
	payments      map[string]dom.PayrollPayment
	payslipStates map[string]svc.PayslipPaymentRow
	seq           int
}

func newFakePaymentRepo() *fakePaymentRepo {
	return &fakePaymentRepo{
		payments:      map[string]dom.PayrollPayment{},
		payslipStates: map[string]svc.PayslipPaymentRow{},
	}
}

func (r *fakePaymentRepo) InsertPayment(_ context.Context, _ pgx.Tx, p svc.InsertPaymentParams) (dom.PayrollPayment, error) {
	r.seq++
	payment := dom.PayrollPayment{
		ID:             "SWP-PPY-" + itoa(5000+r.seq),
		PayslipID:      p.PayslipID,
		AmountEnc:      p.AmountEnc,
		Method:         dom.PaymentMethod(p.Method),
		ReferenceNo:    p.ReferenceNo,
		EvidenceFileID: p.EvidenceFileID,
		PaidOn:         p.PaidOn,
		PaidBy:         p.PaidBy,
		CreatedAt:      fixedNow,
	}
	r.payments[payment.ID] = payment
	return payment, nil
}

func (r *fakePaymentRepo) ListPayments(_ context.Context, payslipID string) ([]dom.PayrollPayment, error) {
	var out []dom.PayrollPayment
	for _, p := range r.payments {
		if p.PayslipID == payslipID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fakePaymentRepo) VoidPayment(_ context.Context, _ pgx.Tx, id string, reason string) (dom.PayrollPayment, error) {
	p, ok := r.payments[id]
	if !ok {
		return dom.PayrollPayment{}, domain.ErrNotFound
	}
	if p.VoidedAt != nil {
		return dom.PayrollPayment{}, apperr.Conflict("PAYMENT_ALREADY_VOIDED")
	}
	now := fixedNow
	p.VoidedAt = &now
	p.VoidReason = &reason
	r.payments[id] = p
	return p, nil
}

func (r *fakePaymentRepo) UpdatePayslipPaymentStatus(_ context.Context, _ pgx.Tx, _ string, _ string, _ time.Time) error {
	return nil
}

func (r *fakePaymentRepo) GetPayslipForPayment(_ context.Context, _ pgx.Tx, payslipID string) (svc.PayslipPaymentRow, error) {
	row, ok := r.payslipStates[payslipID]
	if !ok {
		return svc.PayslipPaymentRow{}, domain.ErrNotFound
	}
	return row, nil
}

func (r *fakePaymentRepo) seedPayslipState(id string, isPosted bool, paymentStatus string) {
	r.payslipStates[id] = svc.PayslipPaymentRow{
		ID:            id,
		IsPosted:      isPosted,
		PaymentStatus: paymentStatus,
	}
}

var _ svc.PaymentRepository = (*fakePaymentRepo)(nil)

// ---------------------------------------------------------------------------
// paymentHarness — mounts the REAL PaymentService + PaymentHandler.
// ---------------------------------------------------------------------------

type paymentHarness struct {
	router    *chi.Mux
	payments  *fakePaymentRepo
	cipher    *crypto.Cipher
	idem      *stubIdempotency
	principal auth.Principal
}

func newPaymentHarness(t *testing.T, role auth.Role) *paymentHarness {
	t.Helper()
	repo := newFakePaymentRepo()
	cipher := newTestCipher(t)
	svc := svc.NewPaymentService(repo, &fakeTxRunner{}, cipher)
	handler := payrollhandler.NewPaymentHandler(svc)
	idem := newStubIdempotency()

	h := &paymentHarness{
		payments: repo,
		cipher:   cipher,
		idem:     idem,
		principal: auth.Principal{
			UserID:     "SWP-USR-9001",
			Role:       role,
			CompanyID:  "",
			EmployeeID: "SWP-EMP-9001",
		},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), h.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.With(idem.Handler).Post("/payments", handler.RecordPayment)
		r.With(idem.Handler).Post("/payments:batch", handler.RecordBatchPayment)
		r.With(idem.Handler).Post("/payments/{id}:void", handler.VoidPayment)
	})

	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleAgent, auth.RoleHRAdmin, auth.RoleSuperAdmin))
		r.Get("/payslips/{id}/payments", handler.ListPayments)
	})

	h.router = r
	return h
}

func (h *paymentHarness) do(method, path string, body any) *httptest.ResponseRecorder {
	return h.doWithHeaders(method, path, body, nil)
}

func (h *paymentHarness) doWithHeaders(method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	buf := bytes.Buffer{}
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

const psPosted = "SWP-PS-90200"
const psPosted2 = "SWP-PS-90201"
const psPosted3 = "SWP-PS-90202"

func TestRecordPayment_Happy_201(t *testing.T) {
	h := newPaymentHarness(t, auth.RoleHRAdmin)
	h.payments.seedPayslipState(psPosted, true, "Unpaid")

	rr := h.do("POST", "/payments", map[string]any{
		"payslip_id":   psPosted,
		"amount":       "8500000.00",
		"method":       "BankTransfer",
		"reference_no": "TRF-20260605-001",
		"paid_on":      "2026-06-05",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["id"] == nil || strOf(d["id"]) == "" {
		t.Errorf("id missing/empty: %v", d)
	}
	if d["payslip_id"] != psPosted {
		t.Errorf("payslip_id = %v, want %s", d["payslip_id"], psPosted)
	}
	if d["amount"] != "8500000.00" {
		t.Errorf("amount = %v, want 8500000.00", d["amount"])
	}
	if d["method"] != "BankTransfer" {
		t.Errorf("method = %v, want BankTransfer", d["method"])
	}
	if d["paid_on"] != "2026-06-05" {
		t.Errorf("paid_on = %v, want 2026-06-05", d["paid_on"])
	}
	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Errorf("Location header missing on 201")
	}
}

func TestRecordBatchPayment_201(t *testing.T) {
	h := newPaymentHarness(t, auth.RoleHRAdmin)
	h.payments.seedPayslipState(psPosted, true, "Unpaid")
	h.payments.seedPayslipState(psPosted2, true, "Unpaid")
	h.payments.seedPayslipState(psPosted3, true, "Unpaid")

	rr := h.do("POST", "/payments:batch", map[string]any{
		"payments": []map[string]any{
			{"payslip_id": psPosted, "amount": "7500000.00", "method": "BankTransfer", "paid_on": "2026-06-05"},
			{"payslip_id": psPosted2, "amount": "8200000.00", "method": "Cash", "paid_on": "2026-06-05"},
			{"payslip_id": psPosted3, "amount": "6800000.00", "method": "BankTransfer", "paid_on": "2026-06-05"},
		},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("data length = %d, want 3", len(data))
	}
	for i, raw := range data {
		p := raw.(map[string]any)
		if p["id"] == nil || strOf(p["id"]) == "" {
			t.Errorf("payment[%d] id missing", i)
		}
		if p["amount"] == nil {
			t.Errorf("payment[%d] amount nil", i)
		}
	}
}

func TestVoidPayment_200(t *testing.T) {
	h := newPaymentHarness(t, auth.RoleHRAdmin)
	h.payments.seedPayslipState(psPosted, true, "Unpaid")

	rrPost := h.do("POST", "/payments", map[string]any{
		"payslip_id": psPosted,
		"amount":     "8500000.00",
		"method":     "BankTransfer",
		"paid_on":    "2026-06-05",
	})
	payID := strOf(dataObject(t, rrPost)["id"])

	rr := h.do("POST", "/payments/"+payID+":void", map[string]any{
		"reason": "Salah nominal transfer.",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	d := dataObject(t, rr)
	if d["void_reason"] != "Salah nominal transfer." {
		t.Errorf("void_reason = %v, want Salah nominal transfer.", d["void_reason"])
	}
	if d["voided_at"] == nil {
		t.Errorf("voided_at nil after void")
	}
}

func TestVoidPayment_AlreadyVoided_409(t *testing.T) {
	h := newPaymentHarness(t, auth.RoleHRAdmin)
	h.payments.seedPayslipState(psPosted, true, "Unpaid")

	rrPost := h.do("POST", "/payments", map[string]any{
		"payslip_id": psPosted,
		"amount":     "8500000.00",
		"method":     "BankTransfer",
		"paid_on":    "2026-06-05",
	})
	payID := strOf(dataObject(t, rrPost)["id"])

	h.do("POST", "/payments/"+payID+":void", map[string]any{"reason": "Salah."})
	rr := h.do("POST", "/payments/"+payID+":void", map[string]any{"reason": "Sekali lagi."})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 on double void, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListPayments_200(t *testing.T) {
	h := newPaymentHarness(t, auth.RoleHRAdmin)
	h.payments.seedPayslipState(psPosted, true, "Unpaid")

	h.do("POST", "/payments", map[string]any{
		"payslip_id": psPosted,
		"amount":     "8500000.00",
		"method":     "BankTransfer",
		"paid_on":    "2026-06-05",
	})

	rr := h.do("GET", "/payslips/"+psPosted+"/payments", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	data := decodeBody(t, rr)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data length = %d, want 1", len(data))
	}
	p := data[0].(map[string]any)
	if p["amount"] != "8500000.00" {
		t.Errorf("amount = %v, want 8500000.00 (decrypted)", p["amount"])
	}
}

func TestRecordPayment_AgentForbidden_403(t *testing.T) {
	h := newPaymentHarness(t, auth.RoleAgent)
	rr := h.do("POST", "/payments", map[string]any{
		"payslip_id": psPosted,
		"amount":     "8500000.00",
		"method":     "BankTransfer",
		"paid_on":    "2026-06-05",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for agent, got %d: %s", rr.Code, rr.Body.String())
	}
}
