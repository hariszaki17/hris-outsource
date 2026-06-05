// Package leave (handler) — the 3 quota endpoints:
// GET /leave-quotas · POST /leave-quotas/{id}:adjust · POST /leave-quotas:bulk-grant.
package leave

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
)

// ListLeaveQuotas handles GET /leave-quotas (cursor-paged; remaining=total-used-pending).
func (h *Handler) ListLeaveQuotas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := svc.QuotaFilter{
		EmployeeID:    strPtrParam(q.Get("employee_id")),
		LeaveTypeID:   strPtrParam(q.Get("leave_type_id")),
		Period:        intPtrParam(q.Get("period")),
		CompanyID:     strPtrParam(q.Get("company_id")),
		ServiceLine:   strPtrParam(q.Get("service_line")),
		IncludeClosed: q.Get("include_closed") == "true",
		Limit:         intParam(q.Get("limit")),
	}
	if cursor := q.Get("cursor"); cursor != "" {
		ca, id, err := svc.DecodeQuotaCursor(cursor)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		f.CursorCreated = ca
		f.CursorID = id
	}
	rows, next, hasMore, err := h.quota.List(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]leaveQuotaResponse, 0, len(rows))
	for _, q := range rows {
		items = append(items, toLeaveQuotaResponse(q))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[leaveQuotaResponse]{
		Data:       items,
		NextCursor: next,
		HasMore:    hasMore,
	})
}

// AdjustLeaveQuota handles POST /leave-quotas/{id}:adjust (delta + reason required).
func (h *Handler) AdjustLeaveQuota(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req adjustRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len([]rune(req.Reason)) < 5 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."}))
		return
	}
	q, err := h.quota.Adjust(r.Context(), id, req.Delta, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveQuotaResponse]{Data: toLeaveQuotaResponse(q)})
}

// BulkGrantLeaveQuotas handles POST /leave-quotas:bulk-grant (pro-rate, partial
// success, preview). 200 always (the bulk envelope carries succeeded/failed).
func (h *Handler) BulkGrantLeaveQuotas(w http.ResponseWriter, r *http.Request) {
	var req bulkGrantRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.LeaveTypeID == "" || req.Period == 0 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"leave_type_id": "Wajib diisi.", "period": "Wajib diisi."}))
		return
	}
	entitlement := 12
	if req.DefaultEntitlementDays != nil {
		entitlement = *req.DefaultEntitlementDays
	}
	periodStart := time.Date(req.Period, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(req.Period, 12, 31, 0, 0, 0, 0, time.UTC)
	result, err := h.quota.BulkGrant(r.Context(), dom.LeaveQuotaBulkGrantParams{
		LeaveTypeID:     req.LeaveTypeID,
		Period:          req.Period,
		PeriodStart:     periodStart,
		PeriodEnd:       periodEnd,
		EntitlementDays: entitlement,
		EmployeeIDs:     req.EmployeeIDs,
		ProRate:         req.ProRate,
		Preview:         req.Preview,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toBulkGrantResponse(result))
}
