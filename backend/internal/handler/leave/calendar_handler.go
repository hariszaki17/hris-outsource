// Package leave (handler) — the calendar endpoint: GET /leave-calendar.
package leave

import (
	"net/http"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// GetLeaveCalendar handles GET /leave-calendar (range entries).
func (h *Handler) GetLeaveCalendar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := svc.CalendarFilter{
		CompanyID:   strPtrParam(q.Get("company_id")),
		LeaveTypeID: strPtrParam(q.Get("leave_type_id")),
		Period:      intParam(q.Get("period")),
		Month:       intPtrParam(q.Get("month")),
		ShowPending: q.Get("show_pending") == "true",
	}
	result, err := h.calendar.Get(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCalendarResponse(result))
}
