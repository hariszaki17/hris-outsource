// Package leave (handler) — the 6 leave-request endpoints:
// GET /leave-requests · GET /leave-requests/{id} ·
// POST /leave-requests/{id}:approve-l1 · :approve-final · :approve-override · :reject.
package leave

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// ListLeaveRequests handles GET /leave-requests (cursor-paged, filtered, scoped).
func (h *Handler) ListLeaveRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := svc.RequestFilter{
		CompanyID:   strPtrParam(q.Get("company_id")),
		EmployeeID:  strPtrParam(q.Get("employee_id")),
		LeaveTypeID: strPtrParam(q.Get("leave_type_id")),
		ServiceLine: strPtrParam(q.Get("service_line")),
		Status:      strPtrParam(q.Get("status")),
		StatusIn:    csvParam(q.Get("status__in")),
		StartFrom:   parseDateParam(q.Get("start_date__gte")),
		StartTo:     parseDateParam(q.Get("start_date__lte")),
		Q:           strPtrParam(q.Get("q")),
		Limit:       intParam(q.Get("limit")),
	}
	if cursor := q.Get("cursor"); cursor != "" {
		ca, id, err := svc.DecodeRequestCursor(cursor)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		f.CursorCreated = ca
		f.CursorID = id
	}
	rows, next, hasMore, err := h.leave.List(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]leaveRequestResponse, 0, len(rows))
	for _, rec := range rows {
		items = append(items, toLeaveRequestResponse(rec))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[leaveRequestResponse]{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// GetLeaveRequest handles GET /leave-requests/{id} (full timeline).
func (h *Handler) GetLeaveRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := h.leave.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveRequestResponse]{Data: toLeaveRequestResponse(rec)})
}

// ApproveLeaveRequestL1 handles POST /leave-requests/{id}:approve-l1 (optional note).
func (h *Handler) ApproveLeaveRequestL1(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req noteRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec, err := h.leave.ApproveL1(r.Context(), id, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveRequestResponse]{Data: toLeaveRequestResponse(rec)})
}

// ApproveLeaveRequestFinal handles POST /leave-requests/{id}:approve-final.
func (h *Handler) ApproveLeaveRequestFinal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req noteRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec, err := h.leave.ApproveFinal(r.Context(), id, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveRequestResponse]{Data: toLeaveRequestResponse(rec)})
}

// ApproveLeaveRequestOverride handles POST /leave-requests/{id}:approve-override
// (override_reason required, min 10).
func (h *Handler) ApproveLeaveRequestOverride(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req overrideRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len([]rune(req.OverrideReason)) < 10 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"override_reason": "Wajib diisi (minimum 10 karakter)."}))
		return
	}
	rec, err := h.leave.ApproveOverride(r.Context(), id, req.OverrideReason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveRequestResponse]{Data: toLeaveRequestResponse(rec)})
}

// RejectLeaveRequest handles POST /leave-requests/{id}:reject (reason required, min 5).
func (h *Handler) RejectLeaveRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req rejectRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len([]rune(req.Reason)) < 5 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."}))
		return
	}
	rec, err := h.leave.Reject(r.Context(), id, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveRequestResponse]{Data: toLeaveRequestResponse(rec)})
}

// CancelLeaveRequest handles POST /leave-requests/{id}:cancel (withdraw a not-yet-
// approved request; releases any pending reservation). reason optional.
func (h *Handler) CancelLeaveRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req rejectRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec, err := h.leave.Cancel(r.Context(), id, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveRequestResponse]{Data: toLeaveRequestResponse(rec)})
}

// CancelApprovedLeaveRequest handles POST /leave-requests/{id}:cancel-approved
// (reverses the exact consumption rows; reason required, min 5).
func (h *Handler) CancelApprovedLeaveRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req rejectRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len([]rune(req.Reason)) < 5 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."}))
		return
	}
	rec, err := h.leave.CancelApproved(r.Context(), id, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveRequestResponse]{Data: toLeaveRequestResponse(rec)})
}

// ShortenLeaveRequest handles POST /leave-requests/{id}:shorten (HR sets an earlier
// end_date; partial grant restore). new_end_date + reason required.
func (h *Handler) ShortenLeaveRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req shortenRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len([]rune(req.Reason)) < 5 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."}))
		return
	}
	newEnd := parseDateParam(req.NewEndDate)
	if newEnd == nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"new_end_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	rec, err := h.leave.Shorten(r.Context(), id, *newEnd, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveRequestResponse]{Data: toLeaveRequestResponse(rec)})
}
