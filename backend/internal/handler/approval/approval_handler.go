// Package approval (handler) — the 8 E11 endpoints:
//
//	GET/PUT/DELETE /client-companies/{companyId}/approval-template (F11.1)
//	GET /approval-instances (+mine,request_type,company_id,status,cursor,limit) (F11.3)
//	GET /approval-instances/{id} (F11.3)
//	POST /approval-instances/{id}:approve · :reject · :bypass (F11.2)
package approval

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/approval"
)

// --- F11.1 templates ---

// GetApprovalTemplate handles GET /client-companies/{companyId}/approval-template.
// 404 when no template (the company uses the super-admin fallback, INV-7).
func (h *Handler) GetApprovalTemplate(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "companyId")
	tpl, err := h.svc.GetTemplate(r.Context(), companyID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toTemplateResponse(tpl))
}

// UpsertApprovalTemplate handles PUT /client-companies/{companyId}/approval-template.
// Full replacement of lines + members; bumps version + resets pending (INV-6).
func (h *Handler) UpsertApprovalTemplate(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "companyId")
	var req approvalTemplateUpsert
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	lines := make([][]string, 0, len(req.Lines))
	for _, l := range req.Lines {
		lines = append(lines, l.Members)
	}
	tpl, err := h.svc.UpsertTemplate(r.Context(), companyID, lines)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toTemplateResponse(tpl))
}

// DeleteApprovalTemplate handles DELETE /client-companies/{companyId}/approval-template.
// 204; the company reverts to the super-admin fallback (TM-7). 404 if none.
func (h *Handler) DeleteApprovalTemplate(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "companyId")
	if err := h.svc.DeleteTemplate(r.Context(), companyID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusNoContent, nil)
}

// --- F11.3 instances list / detail ---

// ListApprovalInstances handles GET /approval-instances (inbox + per-domain).
func (h *Handler) ListApprovalInstances(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := svc.InstanceFilter{
		Mine:        q.Get("mine") == "true",
		CompanyID:   strPtrParam(q.Get("company_id")),
		RequestType: strPtrParam(q.Get("request_type")),
		Status:      strPtrParam(q.Get("status")),
		Limit:       intParam(q.Get("limit")),
	}
	if cursor := q.Get("cursor"); cursor != "" {
		ca, id, err := svc.DecodeInstanceCursor(cursor)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		f.CursorCreated = ca
		f.CursorID = id
	}
	rows, next, hasMore, err := h.svc.List(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]instanceResponse, 0, len(rows))
	for _, rec := range rows {
		items = append(items, toInstanceResponse(rec))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[instanceResponse]{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// GetApprovalInstance handles GET /approval-instances/{id} (chain + actions trail).
func (h *Handler) GetApprovalInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toInstanceDetailResponse(detail))
}

// --- F11.2 execution actions ---

// ApproveApprovalInstance handles POST /approval-instances/{id}:approve (optional note).
func (h *Handler) ApproveApprovalInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req approveBody
	if err := decodeOptionalJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	inst, err := h.svc.Approve(r.Context(), id, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toInstanceResponse(inst))
}

// RejectApprovalInstance handles POST /approval-instances/{id}:reject (reason required).
func (h *Handler) RejectApprovalInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req decisionReason
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Reason == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"reason": "Wajib diisi."}))
		return
	}
	inst, err := h.svc.Reject(r.Context(), id, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toInstanceResponse(inst))
}

// BypassApprovalInstance handles POST /approval-instances/{id}:bypass (super-admin
// only; reason required).
func (h *Handler) BypassApprovalInstance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req decisionReason
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Reason == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"reason": "Wajib diisi."}))
		return
	}
	inst, err := h.svc.Bypass(r.Context(), id, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toInstanceResponse(inst))
}
