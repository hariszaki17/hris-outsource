// Package attendance (handler) — the 4 correction endpoints:
// GET /corrections · GET /corrections/{id} · POST /corrections/{id}:approve ·
// :reject.
package attendance

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// CreateCorrection handles POST /corrections (F5.4): an agent/leader/HR files a
// correction against a target attendance. Scope, the 7-day window, single-active-
// PENDING dedupe, and per-type validation are enforced in the service. Returns 201.
func (h *Handler) CreateCorrection(w http.ResponseWriter, r *http.Request) {
	var req correctionWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Cheap required-field gate (full per-type validation lives in the service).
	fields := map[string]string{}
	if req.AttendanceID == "" {
		fields["attendance_id"] = "Wajib diisi."
	}
	if req.Type == "" {
		fields["type"] = "Wajib diisi."
	}
	if req.Reason == "" {
		fields["reason"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	cor, err := h.corrections.Create(r.Context(), svc.CreateCorrectionInput{
		AttendanceID:             req.AttendanceID,
		Type:                     req.Type,
		ProposedCheckInAt:        req.ProposedCheckInAt,
		ProposedCheckOutAt:       req.ProposedCheckOutAt,
		ProposedAttendanceCodeID: req.ProposedAttendanceCodeID,
		Reason:                   req.Reason,
		EvidenceFileID:           req.EvidenceFileID,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, dataResponse[correctionResponse]{Data: toCorrectionResponse(cor)})
}

// ListCorrections handles GET /corrections (cursor-paged, scoped by role).
func (h *Handler) ListCorrections(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := svc.CorrectionFilter{
		CompanyID:  strPtrParam(q.Get("company_id")),
		EmployeeID: strPtrParam(q.Get("employee_id")),
		Status:     csvParam(q.Get("status")),
		Type:       csvParam(q.Get("type")),
		DateFrom:   parseDateParam(q.Get("date_from")),
		DateTo:     parseDateParam(q.Get("date_to")),
		Limit:      intParam(q.Get("limit")),
	}
	if cursor := q.Get("cursor"); cursor != "" {
		ca, id, err := svc.DecodeCorrectionCursor(cursor)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorCreatedAt = ca
		filter.CursorID = id
	}

	rows, next, hasMore, err := h.corrections.List(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]correctionResponse, 0, len(rows))
	for _, c := range rows {
		items = append(items, toCorrectionResponse(c))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[correctionResponse]{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// GetCorrection handles GET /corrections/{id} (includes server-rendered diff[]).
func (h *Handler) GetCorrection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cor, err := h.corrections.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[correctionResponse]{Data: toCorrectionResponse(cor)})
}

// ApproveCorrection handles POST /corrections/{id}:approve. Applies the proposed
// change to the target attendance + flips status to APPLIED; returns { data, attendance }.
func (h *Handler) ApproveCorrection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req approveRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	cor, attn, err := h.corrections.Approve(r.Context(), id, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, approveCorrectionResponse{
		Data:       toCorrectionResponse(cor),
		Attendance: toAttendanceResponse(attn),
	})
}

// RejectCorrection handles POST /corrections/{id}:reject (reason required, minLen 5).
func (h *Handler) RejectCorrection(w http.ResponseWriter, r *http.Request) {
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
	cor, err := h.corrections.Reject(r.Context(), id, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[correctionResponse]{Data: toCorrectionResponse(cor)})
}
