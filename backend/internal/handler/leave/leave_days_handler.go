// Package leave (handler) — per-day payability of an approved leave request:
// GET /leave-requests/{id}/days · PATCH /leave-requests/{id}/days/{date}.
//
// A leave day that cancelled a scheduled shift is auto-payable; an unpaid leave type
// (cuti di luar tanggungan perusahaan) is never payable; a paid type's NO-SHIFT day is
// pending (is_payable=null) until SL/HR/super flags it (migr. 00064).
package leave

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// leaveDayResponse is one row of the payability breakdown.
type leaveDayResponse struct {
	Date      string `json:"date"`       // YYYY-MM-DD
	LeaveType string `json:"leave_type"` // denormalized leave-type code
	HadShift  bool   `json:"had_shift"`  // cancelled a live scheduled shift
	IsPayable *bool  `json:"is_payable"` // null = pending an SL/HR/super flag
	Flaggable bool   `json:"flaggable"`  // SL/HR/super may set payability (no-shift only)
}

func toLeaveDayResponse(d dom.LeaveDayPayability) leaveDayResponse {
	return leaveDayResponse{
		Date:      dateStr(d.Date),
		LeaveType: d.LeaveType,
		HadShift:  d.HadShift,
		IsPayable: d.IsPayable,
		Flaggable: d.Flaggable(),
	}
}

// setLeaveDayPayableRequest is the PATCH body.
type setLeaveDayPayableRequest struct {
	IsPayable bool `json:"is_payable"`
}

// ListLeaveDays handles GET /leave-requests/{id}/days — the per-day payability breakdown.
func (h *Handler) ListLeaveDays(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	days, err := h.leave.ListLeaveDays(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]leaveDayResponse, 0, len(days))
	for _, d := range days {
		items = append(items, toLeaveDayResponse(d))
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[[]leaveDayResponse]{Data: items})
}

// SetLeaveDayPayable handles PATCH /leave-requests/{id}/days/{date} — SL/HR/super flags a
// NO-SHIFT leave day payable/non-payable.
func (h *Handler) SetLeaveDayPayable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	date, perr := time.Parse("2006-01-02", chi.URLParam(r, "date"))
	if perr != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	var req setLeaveDayPayableRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	day, err := h.leave.SetLeaveDayPayable(r.Context(), id, date, req.IsPayable)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveDayResponse]{Data: toLeaveDayResponse(day)})
}
