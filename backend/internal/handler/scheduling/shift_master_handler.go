// Package scheduling (handler) — hand-written chi handlers for the E4 endpoints.
// One Handler struct aggregates the shift-master + schedule services; server.Deps
// holds a single *scheduling.Handler. Mirrors the placement handler shape.
package scheduling

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// Handler holds the shift-master + schedule services.
type Handler struct {
	masters  *svc.ShiftMasterService
	schedule *svc.ScheduleService
}

// NewHandler wires the handler to its services.
func NewHandler(m *svc.ShiftMasterService, s *svc.ScheduleService) *Handler {
	return &Handler{masters: m, schedule: s}
}

// --- shift masters ---

// ListShiftMasters handles GET /shift-masters (cursor list).
func (h *Handler) ListShiftMasters(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := domain.ShiftMasterFilter{
		Status: strPtrParam(q.Get("status")),
		Q:      strPtrParam(q.Get("q")),
		Limit:  int32(intParam(q.Get("limit"))),
	}
	if cursor := q.Get("cursor"); cursor != "" {
		var c struct {
			ID string `json:"i"`
		}
		if err := httpx.DecodeCursor(cursor, &c); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.Cursor = &c.ID
	}

	rows, next, err := h.masters.ListShiftMasters(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toShiftMasterListResponse(rows, next))
}

// GetShiftMaster handles GET /shift-masters/{id}.
func (h *Handler) GetShiftMaster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.masters.GetShiftMaster(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toShiftMasterResponse(m))
}

// CreateShiftMaster handles POST /shift-masters (201 + Location).
func (h *Handler) CreateShiftMaster(w http.ResponseWriter, r *http.Request) {
	var req shiftMasterWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	created, err := h.masters.CreateShiftMaster(r.Context(), svc.ShiftMasterWrite{
		Name:       req.Name,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		BreakStart: req.BreakStart,
		BreakEnd:   req.BreakEnd,
		IsActive:   req.IsActive,
		CreatedBy:  actorPtr(r),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/shift-masters/"+created.ID)
	httpx.WriteJSON(w, http.StatusCreated, toShiftMasterResponse(created))
}

// UpdateShiftMaster handles PATCH /shift-masters/{id}.
func (h *Handler) UpdateShiftMaster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// Probe raw JSON so explicit null vs absent is distinguishable for the
	// nullable break/service-line fields.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httpx.WriteError(w, r, apperr.Invalid(nil).WithCause(err))
		return
	}
	patch := svc.ShiftMasterPatch{}
	if v, ok := raw["name"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			patch.Name = &s
		}
	}
	if v, ok := raw["start_time"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			patch.StartTime = &s
		}
	}
	if v, ok := raw["end_time"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			patch.EndTime = &s
		}
	}
	if v, ok := raw["break_start"]; ok {
		patch.BreakStartSet = true
		var s *string
		_ = json.Unmarshal(v, &s)
		patch.BreakStart = s
	}
	if v, ok := raw["break_end"]; ok {
		patch.BreakEndSet = true
		var s *string
		_ = json.Unmarshal(v, &s)
		patch.BreakEnd = s
	}
	if v, ok := raw["is_active"]; ok {
		var b bool
		if json.Unmarshal(v, &b) == nil {
			patch.IsActive = &b
		}
	}

	updated, err := h.masters.UpdateShiftMaster(r.Context(), id, patch)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toShiftMasterResponse(updated))
}

// DeactivateShiftMaster handles POST /shift-masters/{id}:deactivate (200).
func (h *Handler) DeactivateShiftMaster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.masters.DeactivateShiftMaster(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toShiftMasterResponse(m))
}

// ReactivateShiftMaster handles POST /shift-masters/{id}:reactivate (200).
func (h *Handler) ReactivateShiftMaster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.masters.ReactivateShiftMaster(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toShiftMasterResponse(m))
}

// --- shared helpers ---

func decodeJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return apperr.Invalid(nil).WithCause(err)
	}
	return nil
}

func actorPtr(r *http.Request) *string {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok || p.UserID == "" {
		return nil
	}
	id := p.UserID
	return &id
}

func strPtrParam(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intParam(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
