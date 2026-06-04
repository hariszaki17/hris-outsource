// Service-line and position handlers for E2 F2.4 (ORG-03).
// Hand-written chi handlers — no server codegen. RBAC enforced in server.go route groups.
// decodeJSON, parseLimit, queryStringPtr, and derefString helpers are declared in
// companies_handler.go (same package) — do NOT redeclare them here.
package org

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svcsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/org"
)

// serviceLineCursor is the opaque cursor payload for service-line list pagination.
type serviceLineCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// ServiceLineHandler holds the service-line service and handles all
// /service-lines and /positions endpoints.
type ServiceLineHandler struct {
	svc *svcsvc.ServiceLineService
}

// NewServiceLineHandler returns a ServiceLineHandler wired to the given service.
func NewServiceLineHandler(s *svcsvc.ServiceLineService) *ServiceLineHandler {
	return &ServiceLineHandler{svc: s}
}

// --- Service Lines ---

// ListServiceLines handles GET /service-lines.
// RBAC: super_admin, hr_admin, shift_leader, agent (enforced in server.go).
func (h *ServiceLineHandler) ListServiceLines(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := domain.ServiceLineFilter{
		Status: queryStringPtr(q.Get("status")),
		Limit:  parseLimit(q.Get("limit")),
	}

	if cursor := q.Get("cursor"); cursor != "" {
		var p serviceLineCursor
		if err := httpx.DecodeCursor(cursor, &p); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorCreatedAt = &p.CreatedAt
		filter.CursorID = &p.ID
	}

	lines, nextCursor, err := h.svc.ListServiceLines(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]serviceLineResponse, 0, len(lines))
	for _, sl := range lines {
		items = append(items, toServiceLineResponse(sl))
	}

	resp := httpx.PageResponse[serviceLineResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// GetServiceLine handles GET /service-lines/{service_line_id}.
// RBAC: super_admin, hr_admin, shift_leader, agent (enforced in server.go).
func (h *ServiceLineHandler) GetServiceLine(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "service_line_id")

	line, err := h.svc.GetServiceLine(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toServiceLineResponse(line))
}

// CreateServiceLine handles POST /service-lines.
// RBAC: super_admin only (enforced in server.go). Returns 201 + Location.
func (h *ServiceLineHandler) CreateServiceLine(w http.ResponseWriter, r *http.Request) {
	var req createServiceLineRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if req.Name == nil || *req.Name == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"name": "Wajib diisi."}))
		return
	}

	line, err := h.svc.CreateServiceLine(r.Context(), *req.Name)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/service-lines/"+line.ID)
	httpx.WriteJSON(w, http.StatusCreated, toServiceLineResponse(line))
}

// UpdateServiceLine handles PATCH /service-lines/{service_line_id}.
// RBAC: super_admin only (enforced in server.go).
func (h *ServiceLineHandler) UpdateServiceLine(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "service_line_id")

	var req updateServiceLineRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if req.Name == nil || *req.Name == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"name": "Wajib diisi."}))
		return
	}

	line, err := h.svc.UpdateServiceLine(r.Context(), id, *req.Name)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toServiceLineResponse(line))
}

// DiscontinueServiceLine handles POST /service-lines/{service_line_id}:discontinue.
// RBAC: super_admin only (enforced in server.go). Returns 200 with updated line.
// 409 SERVICE_LINE_IN_USE if active positions reference the line.
func (h *ServiceLineHandler) DiscontinueServiceLine(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "service_line_id")

	line, err := h.svc.DiscontinueServiceLine(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toServiceLineResponse(line))
}

// --- Positions ---

// ListPositionsInServiceLine handles GET /service-lines/{service_line_id}/positions.
// RBAC: super_admin, hr_admin, shift_leader, agent (enforced in server.go).
func (h *ServiceLineHandler) ListPositionsInServiceLine(w http.ResponseWriter, r *http.Request) {
	lineID := chi.URLParam(r, "service_line_id")
	q := r.URL.Query()

	filter := domain.PositionFilter{
		Status: queryStringPtr(q.Get("status")),
		Limit:  parseLimit(q.Get("limit")),
	}

	if cursor := q.Get("cursor"); cursor != "" {
		var p serviceLineCursor
		if err := httpx.DecodeCursor(cursor, &p); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorCreatedAt = &p.CreatedAt
		filter.CursorID = &p.ID
	}

	positions, nextCursor, err := h.svc.ListPositions(r.Context(), lineID, filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]positionResponse, 0, len(positions))
	for _, p := range positions {
		items = append(items, toPositionResponse(p))
	}

	resp := httpx.PageResponse[positionResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// CreatePosition handles POST /service-lines/{service_line_id}/positions.
// RBAC: super_admin, hr_admin (enforced in server.go). Returns 201 + Location.
// 409 POSITION_IN_USE on duplicate (line, name).
func (h *ServiceLineHandler) CreatePosition(w http.ResponseWriter, r *http.Request) {
	lineID := chi.URLParam(r, "service_line_id")

	var req createPositionRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if req.Name == nil || *req.Name == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"name": "Wajib diisi."}))
		return
	}

	params := svcsvc.CreatePositionParams{
		Name:  derefString(req.Name),
		Alias: derefString(req.Alias),
	}

	pos, err := h.svc.CreatePosition(r.Context(), lineID, params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/positions/"+pos.ID)
	httpx.WriteJSON(w, http.StatusCreated, toPositionResponse(pos))
}

// UpdatePosition handles PATCH /positions/{position_id}.
// RBAC: super_admin, hr_admin (enforced in server.go).
// 409 POSITION_IN_USE on duplicate name within the service line.
func (h *ServiceLineHandler) UpdatePosition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "position_id")

	var req updatePositionRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	params := svcsvc.UpdatePositionParams{
		Name:  derefString(req.Name),
		Alias: derefString(req.Alias),
	}

	pos, err := h.svc.UpdatePosition(r.Context(), id, params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPositionResponse(pos))
}

// SoftDeletePosition handles DELETE /positions/{position_id}.
// RBAC: super_admin, hr_admin (enforced in server.go). Returns 204 No Content.
// 409 POSITION_IN_USE when active placements reference it (stubbed, TODO Phase-5).
func (h *ServiceLineHandler) SoftDeletePosition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "position_id")

	if err := h.svc.SoftDeletePosition(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
