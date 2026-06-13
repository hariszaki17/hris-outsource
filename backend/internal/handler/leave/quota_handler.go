// Package leave (handler) — the per-type quota endpoints:
// GET /leave-balances/by-employee/{employee_id}/types · POST /leave-quotas:adjust-entitled.
package leave

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// GetEmployeeTypeBalances handles GET /leave-balances/by-employee/{employee_id}/types
// — the per-type balance (F6.5 / mobile "Saldo per jenis"): one line per active leave
// type with its current-window quota (resolved by cap_basis).
func (h *Handler) GetEmployeeTypeBalances(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employee_id")
	rows, err := h.quota.EmployeeTypeBalances(r.Context(), employeeID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]typeBalanceResponse, 0, len(rows))
	for _, b := range rows {
		items = append(items, toTypeBalanceResponse(b))
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[[]typeBalanceResponse]{Data: items})
}

// AdjustTypeQuota handles POST /leave-quotas:adjust-entitled — HR per-type quota
// adjust (LQ-6 / "Sesuaikan Kuota"): signed delta on (employee, type, window)
// entitlement; start_date selects the window by cap_basis. Reason required, audited.
func (h *Handler) AdjustTypeQuota(w http.ResponseWriter, r *http.Request) {
	var req adjustEntitledRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len([]rune(req.Reason)) < 5 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."}))
		return
	}
	start, perr := time.Parse("2006-01-02", req.StartDate)
	if perr != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"start_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	q, err := h.leave.AdjustTypeQuota(r.Context(), req.EmployeeID, req.LeaveTypeID, start, req.Delta, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveQuotaResponse]{Data: toLeaveQuotaResponse(q)})
}
