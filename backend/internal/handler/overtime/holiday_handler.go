// Package overtime (handler) — the 4 holiday endpoints (HR-maintained calendar):
// GET /holidays · POST /holidays · PATCH /holidays/{id} · DELETE /holidays/{id}.
package overtime

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/overtime"
)

// ListHolidays handles GET /holidays (cursor-paged ASC by date). Writes PageResponse
// directly ({data, next_cursor, has_more}).
func (h *Handler) ListHolidays(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := svc.HolidayFilter{
		Category:      strPtrParam(q.Get("category")),
		ServiceLineID: strPtrParam(q.Get("service_line_id")),
		Year:          intPtrParam(q.Get("year")),
		Limit:         intParam(q.Get("limit")),
	}
	if cursor := q.Get("cursor"); cursor != "" {
		d, id, err := svc.DecodeHolidayCursor(cursor)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		f.CursorDate = d
		f.CursorID = id
	}
	rows, next, hasMore, err := h.holiday.List(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]holidayResponse, 0, len(rows))
	for _, rec := range rows {
		items = append(items, toHolidayResponse(rec))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[holidayResponse]{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// GetHoliday handles GET /holidays/{id} — wraps the single object in {data}.
func (h *Handler) GetHoliday(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := h.holiday.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[holidayResponse]{Data: toHolidayResponse(rec)})
}

// CreateHoliday handles POST /holidays (201 + Location). HOLIDAY_DATE_CLASH on dup.
func (h *Handler) CreateHoliday(w http.ResponseWriter, r *http.Request) {
	var req holidayWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	svcReq, perr := toHolidayServiceRequest(req)
	if perr != nil {
		httpx.WriteError(w, r, perr)
		return
	}
	rec, err := h.holiday.Create(r.Context(), svcReq)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/holidays/"+rec.ID)
	httpx.WriteJSON(w, http.StatusCreated, toHolidayResponse(rec))
}

// UpdateHoliday handles PATCH /holidays/{id} (200). HOLIDAY_DATE_CLASH on conflict.
func (h *Handler) UpdateHoliday(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req holidayWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	svcReq, perr := toHolidayServiceRequest(req)
	if perr != nil {
		httpx.WriteError(w, r, perr)
		return
	}
	rec, err := h.holiday.Update(r.Context(), id, svcReq)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toHolidayResponse(rec))
}

// DeleteHoliday handles DELETE /holidays/{id} (204). HOLIDAY_IN_USE 409 when
// referenced by APPROVED OT.
func (h *Handler) DeleteHoliday(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.holiday.Delete(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// toHolidayServiceRequest decodes + validates the holiday write body into the
// service request (date parsed YYYY-MM-DD; category passed through).
func toHolidayServiceRequest(req holidayWriteRequest) (svc.HolidayWriteRequest, error) {
	out := svc.HolidayWriteRequest{
		Name:                   req.Name,
		Recurring:              req.Recurring,
		ApplicableServiceLines: req.ApplicableServiceLines,
	}
	if req.Date != "" {
		d := parseDateParam(req.Date)
		if d == nil {
			return svc.HolidayWriteRequest{}, apperr.Invalid(map[string]string{"date": "Format tanggal harus YYYY-MM-DD."})
		}
		out.Date = d
	}
	if req.Category != "" {
		c := req.Category
		out.Category = &c
	}
	return out, nil
}
