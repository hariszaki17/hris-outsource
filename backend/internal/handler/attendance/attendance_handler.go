// Package attendance (handler) — the 6 attendance endpoints:
// GET /attendance · GET /attendance/{id} · POST /attendance/{id}:verify ·
// :reject · POST /attendance:bulk-verify · :bulk-reject.
package attendance

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// ListAttendance handles GET /attendance (cursor-paged, filtered).
func (h *Handler) ListAttendance(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := svc.AttendanceFilter{
		CompanyID:          strPtrParam(q.Get("company_id")),
		EmployeeID:         strPtrParam(q.Get("employee_id")),
		ServiceLine:        strPtrParam(q.Get("service_line")),
		SiteID:             strPtrParam(q.Get("site_id")),
		PositionID:         strPtrParam(q.Get("position_id")),
		VerificationStatus: csvParam(q.Get("verification_status")),
		Status:             csvParam(q.Get("status")),
		DateFrom:           parseDateParam(q.Get("date_from")),
		DateTo:             parseDateParam(q.Get("date_to")),
		ExceptionsOnly:     q.Get("exceptions_only") == "true",
		Limit:              intParam(q.Get("limit")),
	}
	if cursor := q.Get("cursor"); cursor != "" {
		ci, id, err := svc.DecodeAttendanceCursor(cursor)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorCheckInAt = ci
		filter.CursorID = id
	}

	rows, next, hasMore, err := h.attendance.List(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]attendanceResponse, 0, len(rows))
	for _, a := range rows {
		items = append(items, toAttendanceResponse(a))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[attendanceResponse]{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// GetAttendance handles GET /attendance/{id}.
func (h *Handler) GetAttendance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := h.attendance.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[attendanceResponse]{Data: toAttendanceResponse(rec)})
}

// VerifyAttendance handles POST /attendance/{id}:verify (optional body with note + times).
func (h *Handler) VerifyAttendance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req verifyRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec, err := h.attendance.Verify(r.Context(), id, req.Note, req.CheckInAt, req.CheckOutAt)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[attendanceResponse]{Data: toAttendanceResponse(rec)})
}

// RejectAttendance handles POST /attendance/{id}:reject (reason required, minLen 5).
func (h *Handler) RejectAttendance(w http.ResponseWriter, r *http.Request) {
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
	rec, err := h.attendance.Reject(r.Context(), id, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[attendanceResponse]{Data: toAttendanceResponse(rec)})
}

// BulkVerify handles POST /attendance:bulk-verify. 200 if ≥1 succeeded, else 422.
func (h *Handler) BulkVerify(w http.ResponseWriter, r *http.Request) {
	var req bulkVerifyRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(req.IDs) == 0 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"ids": "Minimal satu catatan."}))
		return
	}
	result, err := h.attendance.BulkVerify(r.Context(), req.IDs, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	writeBulk(w, result)
}

// BulkReject handles POST /attendance:bulk-reject (shared reason, minLen 5).
func (h *Handler) BulkReject(w http.ResponseWriter, r *http.Request) {
	var req bulkRejectRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(req.IDs) == 0 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"ids": "Minimal satu catatan."}))
		return
	}
	if len([]rune(req.Reason)) < 5 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."}))
		return
	}
	result, err := h.attendance.BulkReject(r.Context(), req.IDs, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	writeBulk(w, result)
}


// ManualCreate handles POST /attendance:manual-create (F5.6, HR-only).
func (h *Handler) ManualCreate(w http.ResponseWriter, r *http.Request) {
	var req manualCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.EmployeeID == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"employee_id": "Wajib diisi."}))
		return
	}
	checkInAt, err := time.Parse(time.RFC3339, req.CheckInAt)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"check_in_at": "Format waktu tidak valid (RFC3339)."}))
		return
	}
	var checkOutAt *time.Time
	if req.CheckOutAt != nil {
		co, perr := time.Parse(time.RFC3339, *req.CheckOutAt)
		if perr != nil {
			httpx.WriteError(w, r, apperr.Invalid(map[string]string{"check_out_at": "Format waktu tidak valid (RFC3339)."}))
			return
		}
		checkOutAt = &co
	}
	createdBy := ""
	if p, ok := auth.PrincipalFrom(r.Context()); ok {
		createdBy = p.EmployeeID
	}
	rec, err := h.attendance.ManualCreate(r.Context(), svc.ManualCreateRequest{
		EmployeeID: req.EmployeeID,
		CheckInAt:  checkInAt,
		CheckOutAt: checkOutAt,
		Note:       req.Note,
		CreatedBy:  createdBy,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, dataResponse[attendanceResponse]{Data: toAttendanceResponse(rec)})
}

// ManualAutofill handles GET /attendance:manual-autofill?employee_id=xxx&date=yyyy-mm-dd.
// Returns the employee's active placement info and today's schedule (if any).
func (h *Handler) ManualAutofill(w http.ResponseWriter, r *http.Request) {
	empID := r.URL.Query().Get("employee_id")
	dateStr := r.URL.Query().Get("date")
	if empID == "" || dateStr == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{
			"employee_id": "Wajib diisi.",
			"date":        "Wajib diisi (YYYY-MM-DD).",
		}))
		return
	}
	refDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	data, found, err := h.attendance.GetManualAutofillData(r.Context(), empID, refDate)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !found {
		httpx.WriteError(w, r, apperr.Rule("NO_ACTIVE_PLACEMENT", nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[autofillResponse]{Data: autofillResponse{
		EmployeeName: data.EmployeeName,
		CompanyName:  data.CompanyName,
		SiteName:     data.SiteName,
		PositionName: data.PositionName,
		ServiceLine:  data.ServiceLine,
		ScheduleID:   data.ScheduleID,
		ShiftStartAt: rfc3339Ptr(data.ShiftStartAt),
		ShiftEndAt:   rfc3339Ptr(data.ShiftEndAt),
	}})
}

// writeBulk writes the BulkActionResponse with 200 (≥1 succeeded) or 422 (all failed).
func writeBulk(w http.ResponseWriter, result svc.BulkResult) {
	status := http.StatusOK
	if len(result.Succeeded) == 0 {
		status = http.StatusUnprocessableEntity
	}
	httpx.WriteJSON(w, status, toBulkActionResponse(result))
}
