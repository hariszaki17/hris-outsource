// Package placement (handler) — hand-written chi handlers for the E3 placement
// endpoints (CRUD + lifecycle + roster + shift-leader). One Handler struct
// aggregates the placement + shift-leader services; server.Deps holds a single
// *placement.Handler. Mirrors the people handler shape.
package placement

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/placement"
)

// Handler holds the placement + shift-leader services.
type Handler struct {
	placements *svc.PlacementService
	leaders    *svc.ShiftLeaderService
}

// NewHandler wires the handler to its services.
func NewHandler(p *svc.PlacementService, l *svc.ShiftLeaderService) *Handler {
	return &Handler{placements: p, leaders: l}
}

// jakarta returns the current calendar date in Asia/Jakarta for DTO derivation.
func jakartaToday() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	n := time.Now().In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
}

// --- list / get ---

// ListPlacements handles GET /placements.
func (h *Handler) ListPlacements(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := domain.PlacementFilter{
		CompanyID:     strPtrParam(q.Get("company_id")),
		ServiceLineID: strPtrParam(q.Get("service_line_id")),
		EmployeeID:    strPtrParam(q.Get("employee_id")),
		AgreementID:   strPtrParam(q.Get("agreement_id")),
		// Param names are `status` / `status__in`; both filter lifecycle_status.
		Status:         strPtrParam(q.Get("status")),
		StatusIn:       csvParam(q.Get("status__in")),
		Q:              strPtrParam(q.Get("q")),
		IncludeHistory: boolParam(q.Get("include_history")),
		Limit:          intParam(q.Get("limit")),
	}
	if v := q.Get("expiring_within_days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cutoff := jakartaToday().AddDate(0, 0, n)
			filter.EndDateLTE = &cutoff
		}
	}
	if cursor := q.Get("cursor"); cursor != "" {
		var c struct {
			StatusChangedAt time.Time `json:"c"`
			ID              string    `json:"i"`
		}
		if err := httpx.DecodeCursor(cursor, &c); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorStatusChangedAt = &c.StatusChangedAt
		filter.CursorID = &c.ID
	}

	rows, next, err := h.placements.ListPlacements(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPlacementListResponse(rows, next))
}

// ListExpiringPlacements handles GET /placements/expiring (dedicated path).
func (h *Handler) ListExpiringPlacements(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	withinDays := 30
	if v := q.Get("within_days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			withinDays = n
		}
	}
	companyID := strPtrParam(q.Get("company_id"))
	limit := intParam(q.Get("limit"))

	var cursorEndDate *time.Time
	var cursorID *string
	if c := q.Get("cursor"); c != "" {
		var dec struct {
			EndDate time.Time `json:"e"`
			ID      string    `json:"i"`
		}
		if err := httpx.DecodeCursor(c, &dec); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		cursorEndDate = &dec.EndDate
		cursorID = &dec.ID
	}

	rows, next, err := h.placements.ListExpiringPlacements(r.Context(), withinDays, companyID, limit, cursorEndDate, cursorID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPlacementListResponse(rows, next))
}

// GetPlacement handles GET /placements/{id}.
func (h *Handler) GetPlacement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := h.placements.GetPlacement(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	today := jakartaToday()
	resp := placementDetailResponse{
		Placement:    toPlacementResponse(detail.Placement, today),
		HistoryChain: make([]placementSummaryResponse, 0, len(detail.HistoryChain)),
	}
	for _, h := range detail.HistoryChain {
		resp.HistoryChain = append(resp.HistoryChain, toPlacementSummaryResponse(h, today))
	}
	if detail.CurrentLeader != nil {
		resp.CurrentShiftLeader = toShiftLeaderSummaryResponse(*detail.CurrentLeader)
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// --- create / update ---

// CreatePlacement handles POST /placements (201 + Location).
func (h *Handler) CreatePlacement(w http.ResponseWriter, r *http.Request) {
	var req placementWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	startDate, err := parseDate(req.StartDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"start_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	endDate, err := parseOptDate(req.EndDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"end_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}

	actor := actorPtr(r)
	created, err := h.placements.CreatePlacement(r.Context(), svc.CreatePlacementParams{
		EmployeeID:      req.EmployeeID,
		AgreementID:     req.AgreementID,
		ClientCompanyID: req.ClientCompanyID,
		SiteID:          req.SiteID,
		ServiceLineID:   req.ServiceLineID,
		PositionID:      req.PositionID,
		StartDate:       startDate,
		EndDate:         endDate,
		Notes:           req.Notes,
		BackdateReason:  req.BackdateReason,
		CreatedBy:       actor,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/placements/"+created.ID)
	httpx.WriteJSON(w, http.StatusCreated, toPlacementResponse(created, jakartaToday()))
}

// UpdatePlacement handles PATCH /placements/{id}.
func (h *Handler) UpdatePlacement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req placementPatchRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Reject read-only fields.
	roFields := map[string]bool{}
	if req.EmployeeID != nil {
		roFields["employee_id"] = true
	}
	if req.AgreementID != nil {
		roFields["agreement_id"] = true
	}
	if req.ClientCompanyID != nil {
		roFields["client_company_id"] = true
	}
	if req.ServiceLineID != nil {
		roFields["service_line_id"] = true
	}
	if req.StartDate != nil {
		roFields["start_date"] = true
	}
	if req.LifecycleStatus != nil {
		roFields["lifecycle_status"] = true
	}
	if req.PredecessorID != nil {
		roFields["predecessor_id"] = true
	}
	if req.SuccessorID != nil {
		roFields["successor_id"] = true
	}
	if len(roFields) > 0 {
		fields := map[string]string{}
		for k := range roFields {
			fields[k] = "Field tidak dapat diubah."
		}
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	params := svc.UpdatePlacementParams{
		ID:    id,
		Notes: req.Notes,
	}
	if req.PositionID != nil {
		params.PositionID = *req.PositionID
	}
	if req.EndDate != nil && *req.EndDate != "" {
		ed, derr := parseDate(*req.EndDate)
		if derr != nil {
			httpx.WriteError(w, r, apperr.Invalid(map[string]string{"end_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
			return
		}
		params.EndDate = &ed
	}

	updated, err := h.placements.UpdatePlacement(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPlacementResponse(updated, jakartaToday()))
}

// --- lifecycle actions ---

// TransferPlacement handles POST /placements/{id}:transfer (201).
func (h *Handler) TransferPlacement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req transferRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	startDate, err := parseDate(req.NewStartDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"new_start_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	endDate, err := parseOptDate(req.NewEndDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"new_end_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}

	res, err := h.placements.TransferPlacement(r.Context(), svc.TransferParams{
		ID:                 id,
		NewClientCompanyID: req.NewClientCompanyID,
		NewServiceLineID:   req.NewServiceLineID,
		NewPositionID:      req.NewPositionID,
		NewStartDate:       startDate,
		NewEndDate:         endDate,
		NewAgreementID:     req.NewAgreementID,
		TransferReason:     req.TransferReason,
		ActorUserID:        actorPtr(r),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	today := jakartaToday()
	resp := transferResponse{
		Predecessor: toPlacementResponse(res.Predecessor, today),
		Successor:   toPlacementResponse(res.Successor, today),
		Warnings:    res.Warnings,
	}
	if resp.Warnings == nil {
		resp.Warnings = []string{}
	}
	if res.VacatedAssignment != nil {
		a := toShiftLeaderAssignmentResponse(*res.VacatedAssignment)
		resp.VacatedAssignment = &a
	}
	w.Header().Set("Location", "/api/v1/placements/"+res.Successor.ID)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// RenewPlacement handles POST /placements/{id}:renew (201).
func (h *Handler) RenewPlacement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req renewRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	startDate, err := parseDate(req.NewStartDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"new_start_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	endDate, err := parseOptDate(req.NewEndDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"new_end_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	params := svc.RenewParams{
		ID:             id,
		NewStartDate:   startDate,
		NewEndDate:     endDate,
		NewAgreementID: req.NewAgreementID,
		NewPositionID:  req.NewPositionID,
		Notes:          req.Notes,
		ActorUserID:    actorPtr(r),
	}

	res, err := h.placements.RenewPlacement(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	today := jakartaToday()
	resp := renewResponse{
		Predecessor: toPlacementResponse(res.Predecessor, today),
		Successor:   toPlacementResponse(res.Successor, today),
		Warnings:    res.Warnings,
	}
	if resp.Warnings == nil {
		resp.Warnings = []string{}
	}
	w.Header().Set("Location", "/api/v1/placements/"+res.Successor.ID)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// EndPlacement handles POST /placements/{id}:end (200).
func (h *Handler) EndPlacement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req endRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	eff, err := parseDate(req.EffectiveDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"effective_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	updated, err := h.placements.EndPlacement(r.Context(), svc.EndParams{
		ID: id, Reason: req.Reason, EffectiveDate: eff, Notes: req.Notes, ActorUserID: actorPtr(r),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPlacementResponse(updated, jakartaToday()))
}

// ResignPlacement handles POST /placements/{id}:resign (200).
func (h *Handler) ResignPlacement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req resignRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resignAt, err := parseDate(req.ResignAt)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"resign_at": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	updated, err := h.placements.ResignPlacement(r.Context(), svc.ResignParams{
		ID: id, ResignAt: resignAt, Reason: req.ResignationReason, Notes: req.Notes, ActorUserID: actorPtr(r),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPlacementResponse(updated, jakartaToday()))
}

// TerminatePlacement handles POST /placements/{id}:terminate (200).
func (h *Handler) TerminatePlacement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req terminateRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	effPtr, err := parseOptDate(req.EffectiveDate)
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"effective_date": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	updated, err := h.placements.TerminatePlacement(r.Context(), svc.TerminateParams{
		ID:                  id,
		TerminationReason:   req.TerminationReason,
		EffectiveDate:       effPtr,
		TypeCompanyNameConf: req.TypeCompanyNameConf,
		ActorUserID:         actorPtr(r),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPlacementResponse(updated, jakartaToday()))
}

// --- shared helpers ---

func toPlacementListResponse(rows []domain.Placement, next *string) placementListResponse {
	today := jakartaToday()
	items := make([]placementResponse, 0, len(rows))
	for _, p := range rows {
		items = append(items, toPlacementResponse(p, today))
	}
	return placementListResponse{Data: items, NextCursor: next, HasMore: next != nil}
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return apperr.Invalid(nil).WithCause(err)
	}
	return nil
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func parseOptDate(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := parseDate(*s)
	if err != nil {
		return nil, err
	}
	return &t, nil
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

func csvParam(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func boolParam(s string) bool {
	return s == "true" || s == "1"
}

func intParam(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
