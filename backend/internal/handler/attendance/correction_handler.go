// Package attendance (handler) — the 4 correction endpoints:
// GET /corrections · GET /corrections/{id} · POST /corrections/{id}:approve ·
// :reject.
package attendance

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
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
	// NEW_ENTRY carries work_date instead of attendance_id; other types need attendance_id.
	isNewEntry := att.CorrectionType(req.Type) == att.CorrectionTypeNewEntry
	fields := map[string]string{}
	if req.Type == "" {
		fields["type"] = "Wajib diisi."
	}
	if !isNewEntry && req.AttendanceID == "" {
		fields["attendance_id"] = "Wajib diisi."
	}
	if isNewEntry && (req.WorkDate == nil || *req.WorkDate == "") {
		fields["work_date"] = "Wajib diisi untuk koreksi tanpa catatan (NEW_ENTRY)."
	}
	if req.Reason == "" {
		fields["reason"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	var workDate *time.Time
	if req.WorkDate != nil && *req.WorkDate != "" {
		wd, perr := time.Parse("2006-01-02", *req.WorkDate)
		if perr != nil {
			httpx.WriteError(w, r, apperr.Invalid(map[string]string{"work_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
			return
		}
		workDate = &wd
	}

	cor, err := h.corrections.Create(r.Context(), svc.CreateCorrectionInput{
		AttendanceID:             req.AttendanceID,
		WorkDate:                 workDate,
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

// NOTE: correction approve/reject are no longer correction-native endpoints. A correction
// opens an E11 approval_instance on submit (request_type=CORRECTION); decisions go through
// POST /approval-instances/{id}:approve|:reject, which fire CorrectionService.OnApproved /
// OnRejected on the engine's terminal transition.

// CancelCorrection handles POST /corrections/{id}:cancel (agent self-cancel).
func (h *Handler) CancelCorrection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cor, err := h.corrections.Cancel(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[correctionResponse]{Data: toCorrectionResponse(cor)})
}
