// Package org (handler) is the HTTP boundary for E2 client companies and sites.
// Hand-written chi handlers — no server codegen (oapi-codegen cannot parse OpenAPI 3.1).
// RBAC is enforced in server.go route groups; scope guards are in the service layer.
package org

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/org"
)

// pageCursor is the local opaque cursor payload (JSON keys must match the service).
type pageCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// Handler holds the org service and handles all E2 company/site endpoints.
type Handler struct {
	svc *svc.Service
}

// NewHandler returns a Handler wired to the given service.
func NewHandler(s *svc.Service) *Handler {
	return &Handler{svc: s}
}

// --- Client Companies ---

// ListClientCompanies handles GET /client-companies.
// RBAC: super_admin, hr_admin, shift_leader (enforced in server.go).
func (h *Handler) ListClientCompanies(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := domain.CompanyFilter{
		Q:           queryStringPtr(q.Get("q")),
		Status:      queryStringPtr(q.Get("status")),
		ServiceLine: queryStringPtr(q.Get("service_line")),
		Limit:       parseLimit(q.Get("limit")),
	}

	if v := q.Get("has_leader"); v != "" {
		b := v == "true"
		filter.HasLeader = &b
	}

	if cursor := q.Get("cursor"); cursor != "" {
		var p pageCursor
		if err := httpx.DecodeCursor(cursor, &p); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorCreatedAt = &p.CreatedAt
		filter.CursorID = &p.ID
	}

	companies, nextCursor, err := h.svc.ListClientCompanies(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]clientCompanyResponse, 0, len(companies))
	for _, c := range companies {
		items = append(items, toClientCompanyResponse(c))
	}

	resp := httpx.PageResponse[clientCompanyResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// GetClientCompany handles GET /client-companies/{client_company_id}.
func (h *Handler) GetClientCompany(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "client_company_id")

	company, err := h.svc.GetClientCompany(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toClientCompanyResponse(company))
}

// CreateClientCompany handles POST /client-companies.
// Returns 201 + Location header.
func (h *Handler) CreateClientCompany(w http.ResponseWriter, r *http.Request) {
	var req createCompanyRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	fields := map[string]string{}
	if req.Name == nil || *req.Name == "" {
		fields["name"] = "Wajib diisi."
	}
	if req.Address == nil || *req.Address == "" {
		fields["address"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	params := svc.CreateCompanyParams{
		Name:        derefString(req.Name),
		Address:     derefString(req.Address),
		LeaderScope: derefString(req.LeaderScope),
		NPWP:        derefString(req.NPWP),
		PICName:     derefString(req.PICName),
		Phone:       derefString(req.Phone),
		Email:       derefString(req.Email),
	}

	company, err := h.svc.CreateClientCompany(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/client-companies/"+company.ID)
	httpx.WriteJSON(w, http.StatusCreated, toClientCompanyResponse(company))
}

// UpdateClientCompany handles PATCH /client-companies/{client_company_id}.
func (h *Handler) UpdateClientCompany(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "client_company_id")

	var req updateCompanyRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Load current company to carry forward unchanged fields (partial update).
	current, err := h.svc.GetClientCompany(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	params := svc.UpdateCompanyParams{
		ID:          id,
		Name:        coalesce(req.Name, current.Name),
		Address:     coalesce(req.Address, current.Address),
		LeaderScope: coalesce(req.LeaderScope, current.LeaderScope),
		NPWP:        coalescePtrStr(req.NPWP, current.NPWP),
		PICName:     coalescePtrStr(req.PICName, current.PICName),
		Phone:       coalescePtrStr(req.Phone, current.Phone),
		Email:       coalescePtrStr(req.Email, current.Email),
	}

	company, err := h.svc.UpdateClientCompany(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toClientCompanyResponse(company))
}

// DeactivateClientCompany handles POST /client-companies/{client_company_id}:deactivate.
// Reads optional ?force=true query param and optional JSON body with reason.
func (h *Handler) DeactivateClientCompany(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "client_company_id")
	force := r.URL.Query().Get("force") == "true"

	var req reasonRequest
	// Body is optional — ignore decode errors for empty or missing bodies.
	_ = decodeJSON(r, &req)

	reason := ""
	if req.Reason != nil {
		reason = *req.Reason
	}

	company, err := h.svc.DeactivateClientCompany(r.Context(), id, reason, force)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toClientCompanyResponse(company))
}

// ReactivateClientCompany handles POST /client-companies/{client_company_id}:reactivate.
func (h *Handler) ReactivateClientCompany(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "client_company_id")

	company, err := h.svc.ReactivateClientCompany(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toClientCompanyResponse(company))
}

// --- Sites ---

// ListSites handles GET /client-companies/{client_company_id}/sites.
func (h *Handler) ListSites(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "client_company_id")
	q := r.URL.Query()

	filter := domain.SiteFilter{
		Status: queryStringPtr(q.Get("status")),
		Limit:  parseLimit(q.Get("limit")),
	}

	if cursor := q.Get("cursor"); cursor != "" {
		var p pageCursor
		if err := httpx.DecodeCursor(cursor, &p); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorCreatedAt = &p.CreatedAt
		filter.CursorID = &p.ID
	}

	sites, nextCursor, err := h.svc.ListSites(r.Context(), companyID, filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]siteResponse, 0, len(sites))
	for _, s := range sites {
		items = append(items, toSiteResponse(s))
	}

	resp := httpx.PageResponse[siteResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// GetSite handles GET /sites/{site_id}.
func (h *Handler) GetSite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "site_id")

	site, err := h.svc.GetSite(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSiteResponse(site))
}

// CreateSite handles POST /client-companies/{client_company_id}/sites.
// Returns 201 + Location header.
func (h *Handler) CreateSite(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "client_company_id")

	var req createSiteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	fields := map[string]string{}
	if req.Name == nil || *req.Name == "" {
		fields["name"] = "Wajib diisi."
	}
	if req.Address == nil || *req.Address == "" {
		fields["address"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	params := svc.CreateSiteParams{
		Name:    derefString(req.Name),
		Address: derefString(req.Address),
		Code:    derefString(req.Code),
		PICName: derefString(req.PICName),
		Phone:   derefString(req.Phone),
	}
	if req.Geo != nil {
		params.GeoLat = &req.Geo.Lat
		params.GeoLng = &req.Geo.Lng
	}
	if req.GeofenceRadiusM != nil {
		params.GeofenceRadiusM = *req.GeofenceRadiusM
	}
	if req.IsPrimary != nil {
		params.IsPrimary = *req.IsPrimary
	}

	site, err := h.svc.CreateSite(r.Context(), companyID, params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/sites/"+site.ID)
	httpx.WriteJSON(w, http.StatusCreated, toSiteResponse(site))
}

// UpdateSite handles PATCH /sites/{site_id}.
func (h *Handler) UpdateSite(w http.ResponseWriter, r *http.Request) {
	siteID := chi.URLParam(r, "site_id")

	var req updateSiteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	params := svc.UpdateSiteParams{
		Name:    derefString(req.Name),
		Code:    derefString(req.Code),
		Address: derefString(req.Address),
		PICName: derefString(req.PICName),
		Phone:   derefString(req.Phone),
	}
	if req.Geo != nil {
		params.GeoLat = &req.Geo.Lat
		params.GeoLng = &req.Geo.Lng
	}
	if req.GeofenceRadiusM != nil {
		params.GeofenceRadiusM = *req.GeofenceRadiusM
	}
	if req.IsPrimary != nil {
		params.IsPrimary = *req.IsPrimary
	}

	site, err := h.svc.UpdateSite(r.Context(), siteID, params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSiteResponse(site))
}

// DeactivateSite handles POST /sites/{site_id}:deactivate.
func (h *Handler) DeactivateSite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "site_id")

	site, err := h.svc.DeactivateSite(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSiteResponse(site))
}

// --- shared helpers (local copies — not exported; no coupling to other packages) ---

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.Invalid(nil).WithCause(err)
	}
	return nil
}

func queryStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func parseLimit(s string) int {
	if s == "" {
		return 0 // ClampLimit applies default
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// coalesce returns the dereferenced pointer value, or fallback if nil/empty.
func coalesce(ptr *string, fallback string) string {
	if ptr != nil && *ptr != "" {
		return *ptr
	}
	return fallback
}

// coalescePtrStr returns the pointer if non-nil, else the existing pointer value as a non-nil string.
// Used for optional nullable string fields in partial updates.
func coalescePtrStr(ptr *string, existing *string) string {
	if ptr != nil {
		return *ptr
	}
	if existing != nil {
		return *existing
	}
	return ""
}
