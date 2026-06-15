// Package leave (handler) — HR-assigned leave entitlement endpoints (PRD
// leave-entitlement-assignment, 2026-06-15):
//
//	GET    /employees/{employee_id}/leave-entitlements
//	POST   /employees/{employee_id}/leave-entitlements
//	PATCH  /employees/{employee_id}/leave-entitlements/{leave_type_id}
//	DELETE /employees/{employee_id}/leave-entitlements/{leave_type_id}
//
// Write = hr_admin / super_admin (enforced at the router). Decode → validate →
// service → httpx.WriteJSON.
package leave

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// ListEmployeeLeaveEntitlements handles GET /employees/{id}/leave-entitlements — an
// employee's active leave-type assignments (HR "Hak Cuti" grid).
func (h *Handler) ListEmployeeLeaveEntitlements(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employee_id")
	rows, err := h.entitlement.List(r.Context(), employeeID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]leaveEntitlementResponse, 0, len(rows))
	for _, e := range rows {
		items = append(items, toLeaveEntitlementResponse(e))
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[[]leaveEntitlementResponse]{Data: items})
}

// AssignEmployeeLeaveEntitlement handles POST /employees/{id}/leave-entitlements —
// assign (or reactivate) a leave type for an employee.
func (h *Handler) AssignEmployeeLeaveEntitlement(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employee_id")
	var req assignEntitlementRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.LeaveTypeID == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"leave_type_id": "Wajib diisi."}))
		return
	}
	if req.EntitledDays != nil && *req.EntitledDays < 0 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"entitled_days": "Tidak boleh negatif."}))
		return
	}
	e, err := h.entitlement.Assign(r.Context(), employeeID, req.LeaveTypeID, req.EntitledDays, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, dataResponse[leaveEntitlementResponse]{Data: toLeaveEntitlementResponse(e)})
}

// UpdateEmployeeLeaveEntitlement handles PATCH /employees/{id}/leave-entitlements/{type}
// — edit the base entitled_days / reactivate.
func (h *Handler) UpdateEmployeeLeaveEntitlement(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employee_id")
	leaveTypeID := chi.URLParam(r, "leave_type_id")
	var req updateEntitlementRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.EntitledDays != nil && *req.EntitledDays < 0 {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"entitled_days": "Tidak boleh negatif."}))
		return
	}
	e, err := h.entitlement.Update(r.Context(), employeeID, leaveTypeID, req.EntitledDays, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dataResponse[leaveEntitlementResponse]{Data: toLeaveEntitlementResponse(e)})
}

// RemoveEmployeeLeaveEntitlement handles DELETE /employees/{id}/leave-entitlements/{type}
// — deactivate the assignment (soft on/off; the type vanishes from the picker/grid).
func (h *Handler) RemoveEmployeeLeaveEntitlement(w http.ResponseWriter, r *http.Request) {
	employeeID := chi.URLParam(r, "employee_id")
	leaveTypeID := chi.URLParam(r, "leave_type_id")
	if _, err := h.entitlement.Remove(r.Context(), employeeID, leaveTypeID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
