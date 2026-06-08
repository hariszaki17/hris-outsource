// Package leave (handler) — the F6.1 grant-lot ledger + balance endpoints:
// GET/POST /leave-grants · GET/PATCH /leave-grants/{id} ·
// GET /leave-balances/by-employee/{employee_id}. Replaces the deprecated
// /leave-quotas* model (2026-06-08). x-rbac perms mirror the openapi.
package leave

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// ListLeaveGrants handles GET /leave-grants (FIFO-ordered ledger, cursor-paged).
func (h *Handler) ListLeaveGrants(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := svc.GrantFilter{
		EmployeeID:     strPtrParam(q.Get("employee_id")),
		Earmark:        strPtrParam(q.Get("earmark")),
		Source:         strPtrParam(q.Get("source")),
		CompanyID:      strPtrParam(q.Get("company_id")),
		IncludeExpired: q.Get("include_expired") == "true",
		Limit:          intParam(q.Get("limit")),
	}
	if cursor := q.Get("cursor"); cursor != "" {
		ea, id, err := svc.DecodeGrantCursor(cursor)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		f.CursorExpires = ea
		f.CursorID = id
	}
	rows, next, hasMore, err := h.grant.List(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	now := time.Now()
	items := make([]leaveGrantResponse, 0, len(rows))
	for _, g := range rows {
		items = append(items, toLeaveGrantResponse(g, now))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[leaveGrantResponse]{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// CreateLeaveGrant handles POST /leave-grants (HR grants one lot; remark required).
func (h *Handler) CreateLeaveGrant(w http.ResponseWriter, r *http.Request) {
	var req grantWriteRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.EmployeeID == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"employee_id": "Wajib diisi."}))
		return
	}
	expires, perr := time.Parse("2006-01-02", req.ExpiresAt)
	if perr != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"expires_at": "Format tanggal tidak valid (YYYY-MM-DD)."}))
		return
	}
	p := svc.CreateGrantParams{
		EmployeeID: req.EmployeeID,
		Amount:     req.AmountDays,
		Source:     dom.LeaveGrantSource(req.Source),
		Earmark:    req.Earmark,
		Remark:     &req.Remark,
		ExpiresAt:  expires,
	}
	if req.EffectiveFrom != nil && *req.EffectiveFrom != "" {
		if ef, eerr := time.Parse("2006-01-02", *req.EffectiveFrom); eerr == nil {
			p.EffectiveFrom = ef
		}
	}
	g, err := h.grant.Create(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/leave-grants/"+g.ID)
	httpx.WriteJSON(w, http.StatusCreated, toLeaveGrantResponse(g, time.Now()))
}

// GetLeaveGrant handles GET /leave-grants/{id} (with consumptions[]).
func (h *Handler) GetLeaveGrant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	includeConsumptions := r.URL.Query().Get("include_consumptions") != "false"
	g, err := h.grant.Get(r.Context(), id, includeConsumptions)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toLeaveGrantResponse(g, time.Now()))
}

// PatchLeaveGrant handles PATCH /leave-grants/{id} (adjust amount/expires_at/earmark).
func (h *Handler) PatchLeaveGrant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// Decode into a raw map first so we can tell "earmark: null" (set to pool) from
	// "earmark absent" (leave unchanged) per the OpenAPI 3.1 null convention.
	var raw map[string]json.RawMessage
	if err := decodeJSON(r, &raw); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req grantPatchRequest
	if rm, ok := raw["amount_days"]; ok {
		var v int
		if json.Unmarshal(rm, &v) == nil {
			req.AmountDays = &v
		}
	}
	if rm, ok := raw["expires_at"]; ok {
		var v string
		if json.Unmarshal(rm, &v) == nil {
			req.ExpiresAt = &v
		}
	}
	if rm, ok := raw["earmark"]; ok {
		req.SetEarmark = true
		var v *string
		if json.Unmarshal(rm, &v) == nil {
			req.Earmark = v
		}
	}
	if rm, ok := raw["remark"]; ok {
		_ = json.Unmarshal(rm, &req.Remark)
	}
	if len([]rune(req.Remark)) < 5 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"remark": "Wajib diisi (minimum 5 karakter)."}))
		return
	}
	p := svc.PatchGrantParams{ID: id, Amount: req.AmountDays, SetEarmark: req.SetEarmark, Earmark: req.Earmark, Remark: req.Remark}
	if req.ExpiresAt != nil {
		if ea, eerr := time.Parse("2006-01-02", *req.ExpiresAt); eerr == nil {
			p.ExpiresAt = &ea
		}
	}
	g, err := h.grant.Patch(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toLeaveGrantResponse(g, time.Now()))
}

// GetLeaveBalanceByEmployee handles GET /leave-balances/by-employee/{employee_id}.
func (h *Handler) GetLeaveBalanceByEmployee(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employee_id")
	includeExpired := r.URL.Query().Get("include_expired_lots") == "true"
	bal, err := h.grant.Balance(r.Context(), employeeID, includeExpired)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveBalanceResponse]{Data: toLeaveBalanceResponse(bal, time.Now())})
}
