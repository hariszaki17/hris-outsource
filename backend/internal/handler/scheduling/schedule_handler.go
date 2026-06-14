// Package scheduling (handler) — schedule-entry + :check + :bulk-apply handlers.
// Decode → validate → service → httpx.WriteJSON; conflict apperr envelopes flow
// through httpx.WriteError (carrying code/fields/details/status). :bulk-apply is
// 200 when ≥1 cell succeeded else 422; :check is always 200 (no side effects).
package scheduling

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// --- grid read ---

// ListSchedule handles GET /schedule (required company_id + start_date + end_date).
func (h *Handler) ListSchedule(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	companyID := q.Get("company_id")
	if companyID == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"company_id": "Wajib diisi."}))
		return
	}
	start, err := parseDate(q.Get("start_date"))
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"start_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	end, err := parseDate(q.Get("end_date"))
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"end_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	filter := domain.ScheduleFilter{
		CompanyID:  companyID,
		StartDate:  start,
		EndDate:    end,
		EmployeeID: strPtrParam(q.Get("employee_id")),
		StatusIn:   csvParam(q.Get("status__in")),
	}
	rows, serr := h.schedule.ListSchedule(r.Context(), filter)
	if serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toScheduleListResponse(rows))
}

// --- agent self-schedule (F4.3 "Jadwal Saya") ---

// GetScheduleByAgent handles GET /schedule/by-agent/{employee_id} (required
// start_date + end_date). RBAC scope (agent self-only / leader-company / staff
// any) is enforced in the service. include_company is accepted but ignored for
// the MVP (SV-3 geo/address enrichment deferred).
func (h *Handler) GetScheduleByAgent(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employee_id")
	q := r.URL.Query()
	start, err := parseDate(q.Get("start_date"))
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"start_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	end, err := parseDate(q.Get("end_date"))
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"end_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	// include_company is parsed for forward-compat but ignored (SV-3 deferred).

	rows, serr := h.schedule.GetScheduleByAgent(r.Context(), employeeID, start, end)
	if serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	items := make([]scheduleEntryResponse, 0, len(rows))
	for _, e := range rows {
		items = append(items, toScheduleEntryResponse(e))
	}
	httpx.WriteJSON(w, http.StatusOK, scheduleByAgentResponse{
		Data:     items,
		Warnings: []warningResponse{},
	})
}

// --- single-cell create ---

// CreateScheduleEntry handles POST /schedule (201 + Location + warnings:[]).
func (h *Handler) CreateScheduleEntry(w http.ResponseWriter, r *http.Request) {
	var req scheduleEntryWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	date, err := parseDate(req.Date)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	created, serr := h.schedule.CreateEntry(r.Context(), svc.CreateEntryRequest{
		EmployeeID:    req.EmployeeID,
		ShiftMasterID: req.ShiftMasterID,
		Date:          date,
		IsDayOff:      boolVal(req.IsDayOff),
		ForceReplace:  boolVal(req.ForceReplace),
		CreatedBy:     actorPtr(r),
	})
	if serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	w.Header().Set("Location", "/api/v1/schedule/"+created.ID)
	resp := scheduleEntryCreateResponse{
		scheduleEntryResponse: toScheduleEntryResponse(created),
		Warnings:              []warningResponse{},
	}
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// --- single-cell update ---

// UpdateScheduleEntry handles PATCH /schedule/{id} (200).
func (h *Handler) UpdateScheduleEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httpx.WriteError(w, r, apperr.Invalid(nil).WithCause(err))
		return
	}
	req := svc.UpdateEntryRequest{}
	if v, ok := raw["shift_master_id"]; ok {
		req.ShiftMasterIDSet = true
		var s *string
		_ = json.Unmarshal(v, &s)
		req.ShiftMasterID = s
	}
	if v, ok := raw["date"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil && s != "" {
			d, derr := parseDate(s)
			if derr != nil {
				httpx.WriteError(w, r, apperr.Invalid(map[string]string{"date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
				return
			}
			req.Date = &d
		}
	}
	if v, ok := raw["is_day_off"]; ok {
		var b bool
		if json.Unmarshal(v, &b) == nil {
			req.IsDayOff = &b
		}
	}

	updated, serr := h.schedule.UpdateEntry(r.Context(), id, req)
	if serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toScheduleEntryResponse(updated))
}

// --- single-cell delete ---

// DeleteScheduleEntry handles DELETE /schedule/{id} (204, no body).
func (h *Handler) DeleteScheduleEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.schedule.DeleteEntry(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- :check (dry-run) ---

// CheckScheduleConflicts handles POST /schedule:check. The kind discriminator
// ("single"|"bulk") selects the request shape; no writes are performed.
func (h *Handler) CheckScheduleConflicts(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httpx.WriteError(w, r, apperr.Invalid(nil).WithCause(err))
		return
	}
	var kindProbe struct {
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(raw, &kindProbe)

	var result svc.BulkResult
	var serr error
	if kindProbe.Kind == "bulk" {
		req, perr := parseBulkRequest(raw, r)
		if perr != nil {
			httpx.WriteError(w, r, perr)
			return
		}
		result, serr = h.schedule.CheckBulk(r.Context(), req)
	} else {
		var sreq scheduleEntryWriteRequest
		if err := json.Unmarshal(raw, &sreq); err != nil {
			httpx.WriteError(w, r, apperr.Invalid(nil).WithCause(err))
			return
		}
		date, derr := parseDate(sreq.Date)
		if derr != nil {
			httpx.WriteError(w, r, apperr.Invalid(map[string]string{"date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
			return
		}
		result, serr = h.schedule.CheckSingle(r.Context(), svc.CreateEntryRequest{
			EmployeeID:    sreq.EmployeeID,
			ShiftMasterID: sreq.ShiftMasterID,
			Date:          date,
			IsDayOff:      boolVal(sreq.IsDayOff),
			ForceReplace:  boolVal(sreq.ForceReplace),
		})
	}
	if serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toBulkApplyResult(result))
}

// --- :bulk-apply (per-cell atomic) ---

// BulkApplySchedule handles POST /schedule:bulk-apply. 200 when ≥1 cell
// succeeded; 422 when ALL cells failed (same BulkApplyResult body either way).
func (h *Handler) BulkApplySchedule(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httpx.WriteError(w, r, apperr.Invalid(nil).WithCause(err))
		return
	}
	req, perr := parseBulkRequest(raw, r)
	if perr != nil {
		httpx.WriteError(w, r, perr)
		return
	}
	result, serr := h.schedule.BulkApply(r.Context(), req)
	if serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	status := http.StatusOK
	if len(result.Succeeded) == 0 {
		status = http.StatusUnprocessableEntity
	}
	httpx.WriteJSON(w, status, toBulkApplyResult(result))
}

// --- shared helpers ---

// parseBulkRequest decodes + validates a bulk body into the service request.
func parseBulkRequest(raw json.RawMessage, r *http.Request) (svc.BulkRequest, error) {
	var req bulkApplyRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return svc.BulkRequest{}, apperr.Invalid(nil).WithCause(err)
	}
	start, err := parseDate(req.StartDate)
	if err != nil {
		return svc.BulkRequest{}, apperr.Invalid(map[string]string{"start_date": "Format tanggal tidak valid (YYYY-MM-DD)."})
	}
	end, err := parseDate(req.EndDate)
	if err != nil {
		return svc.BulkRequest{}, apperr.Invalid(map[string]string{"end_date": "Format tanggal tidak valid (YYYY-MM-DD)."})
	}
	return svc.BulkRequest{
		ShiftMasterID:    req.ShiftMasterID,
		StartDate:        start,
		EndDate:          end,
		EmployeeIDs:      req.EmployeeIDs,
		WeekdaysMask:     req.WeekdaysMask,
		OverrideExisting: boolVal(req.OverrideExisting),
		CreatedBy:        actorPtr(r),
	}, nil
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func boolVal(p *bool) bool {
	return p != nil && *p
}

func csvParam(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	cur := ""
	for _, c := range s {
		if c == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
