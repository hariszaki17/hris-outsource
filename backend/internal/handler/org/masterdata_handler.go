// Package org (handler) — master-data handler for leave types, attendance codes,
// and overtime rules. Hand-written chi handlers; RBAC enforced in server.go route
// groups. Reuses decodeJSON / parseLimit / queryStringPtr / derefString helpers
// declared in companies_handler.go (same package, no redeclaration).
package org

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/org"
)

// MasterDataHandler serves the 12 operational master-data endpoints:
// List/Create/Update/SoftDelete for leave types, attendance codes, and overtime rules.
type MasterDataHandler struct {
	svc *svc.MasterDataService
}

// NewMasterDataHandler returns a MasterDataHandler wired to the given service.
func NewMasterDataHandler(s *svc.MasterDataService) *MasterDataHandler {
	return &MasterDataHandler{svc: s}
}

// =============================================================================
// Leave Types
// =============================================================================

// ListLeaveTypes handles GET /leave-types.
// RBAC: super_admin, hr_admin, shift_leader, agent (all roles — see server.go).
func (h *MasterDataHandler) ListLeaveTypes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := domain.LeaveTypeFilter{
		Status: queryStringPtr(q.Get("status")),
		Limit:  parseLimit(q.Get("limit")),
	}

	if v := q.Get("is_annual"); v != "" {
		b := v == "true"
		filter.IsAnnual = &b
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

	rows, nextCursor, err := h.svc.ListLeaveTypes(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]leaveTypeResponse, 0, len(rows))
	for _, lt := range rows {
		items = append(items, toLeaveTypeResponse(lt))
	}

	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[leaveTypeResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	})
}

// CreateLeaveType handles POST /leave-types. Returns 201.
// RBAC: super_admin, hr_admin.
func (h *MasterDataHandler) CreateLeaveType(w http.ResponseWriter, r *http.Request) {
	var req createLeaveTypeRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	fields := map[string]string{}
	if req.Name == nil || *req.Name == "" {
		fields["name"] = "Wajib diisi."
	}
	if req.Code == nil || *req.Code == "" {
		fields["code"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	params := svc.CreateLeaveTypeParams{
		Name:             derefString(req.Name),
		Code:             derefString(req.Code),
		Description:      derefString(req.Description),
		IsAnnual:         derefBool(req.IsAnnual),
		RequiresDocument: derefBool(req.RequiresDocument),
		Color:            derefString(req.Color),
	}
	if req.DefaultAnnualQuota != nil {
		params.DefaultAnnualQuota = *req.DefaultAnnualQuota
	}

	lt, err := h.svc.CreateLeaveType(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/leave-types/"+lt.ID)
	httpx.WriteJSON(w, http.StatusCreated, toLeaveTypeResponse(lt))
}

// UpdateLeaveType handles PATCH /leave-types/{leave_type_id}.
// RBAC: super_admin, hr_admin.
func (h *MasterDataHandler) UpdateLeaveType(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "leave_type_id")

	var req updateLeaveTypeRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Load current record for partial-update carry-forward.
	current, err := h.svc.GetLeaveType(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	quota := current.DefaultAnnualQuota
	if req.DefaultAnnualQuota != nil {
		quota = *req.DefaultAnnualQuota
	}

	params := svc.UpdateLeaveTypeParams{
		ID:                 id,
		Name:               coalesce(req.Name, current.Name),
		Code:               coalesce(req.Code, current.Code),
		Description:        coalesce(req.Description, current.Description),
		DefaultAnnualQuota: quota,
		IsAnnual:           coalesceB(req.IsAnnual, current.IsAnnual),
		RequiresDocument:   coalesceB(req.RequiresDocument, current.RequiresDocument),
		Color:              coalesce(req.Color, current.Color),
	}

	lt, err := h.svc.UpdateLeaveType(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toLeaveTypeResponse(lt))
}

// SoftDeleteLeaveType handles DELETE /leave-types/{leave_type_id}. Returns 204.
// RBAC: super_admin, hr_admin.
func (h *MasterDataHandler) SoftDeleteLeaveType(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "leave_type_id")

	if err := h.svc.SoftDeleteLeaveType(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// Attendance Codes
// =============================================================================

// ListAttendanceCodes handles GET /attendance-codes.
// RBAC: super_admin, hr_admin, shift_leader, agent.
func (h *MasterDataHandler) ListAttendanceCodes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := domain.AttendanceCodeFilter{
		Status: queryStringPtr(q.Get("status")),
		Limit:  parseLimit(q.Get("limit")),
	}

	if v := q.Get("is_billable"); v != "" {
		b := v == "true"
		filter.IsBillable = &b
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

	rows, nextCursor, err := h.svc.ListAttendanceCodes(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]attendanceCodeResponse, 0, len(rows))
	for _, ac := range rows {
		items = append(items, toAttendanceCodeResponse(ac))
	}

	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[attendanceCodeResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	})
}

// CreateAttendanceCode handles POST /attendance-codes. Returns 201.
// RBAC: super_admin, hr_admin.
func (h *MasterDataHandler) CreateAttendanceCode(w http.ResponseWriter, r *http.Request) {
	var req createAttendanceCodeRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	fields := map[string]string{}
	if req.Code == nil || *req.Code == "" {
		fields["code"] = "Wajib diisi."
	}
	if req.Label == nil || *req.Label == "" {
		fields["label"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	params := svc.CreateAttendanceCodeParams{
		Code:              derefString(req.Code),
		Label:             derefString(req.Label),
		Description:       derefString(req.Description),
		Color:             derefString(req.Color),
		IsWorkday:         derefBool(req.IsWorkday),
		IsPaid:            derefBool(req.IsPaid),
		IsBillable:        derefBool(req.IsBillable),
		NeedsVerification: derefBool(req.NeedsVerification),
	}

	ac, err := h.svc.CreateAttendanceCode(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/attendance-codes/"+ac.ID)
	httpx.WriteJSON(w, http.StatusCreated, toAttendanceCodeResponse(ac))
}

// UpdateAttendanceCode handles PATCH /attendance-codes/{attendance_code_id}.
// RBAC: super_admin, hr_admin.
func (h *MasterDataHandler) UpdateAttendanceCode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "attendance_code_id")

	var req updateAttendanceCodeRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	current, err := h.svc.GetAttendanceCode(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	params := svc.UpdateAttendanceCodeParams{
		ID:                id,
		Code:              coalesce(req.Code, current.Code),
		Label:             coalesce(req.Label, current.Label),
		Description:       coalesce(req.Description, current.Description),
		Color:             coalesce(req.Color, current.Color),
		IsWorkday:         coalesceB(req.IsWorkday, current.IsWorkday),
		IsPaid:            coalesceB(req.IsPaid, current.IsPaid),
		IsBillable:        coalesceB(req.IsBillable, current.IsBillable),
		NeedsVerification: coalesceB(req.NeedsVerification, current.NeedsVerification),
	}

	ac, err := h.svc.UpdateAttendanceCode(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toAttendanceCodeResponse(ac))
}

// SoftDeleteAttendanceCode handles DELETE /attendance-codes/{attendance_code_id}. Returns 204.
// RBAC: super_admin, hr_admin.
func (h *MasterDataHandler) SoftDeleteAttendanceCode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "attendance_code_id")

	if err := h.svc.SoftDeleteAttendanceCode(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// Overtime Rules
// =============================================================================

// ListOvertimeRules handles GET /overtime-rules.
// RBAC: super_admin, hr_admin, shift_leader (agent excluded — see spec x-rbac).
func (h *MasterDataHandler) ListOvertimeRules(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := domain.OvertimeRuleFilter{
		Status:      queryStringPtr(q.Get("status")),
		ServiceLine: queryStringPtr(q.Get("service_line")),
		Limit:       parseLimit(q.Get("limit")),
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

	rows, nextCursor, err := h.svc.ListOvertimeRules(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]overtimeRuleResponse, 0, len(rows))
	for _, otr := range rows {
		items = append(items, toOvertimeRuleResponse(otr))
	}

	httpx.WriteJSON(w, http.StatusOK, httpx.PageResponse[overtimeRuleResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	})
}

// CreateOvertimeRule handles POST /overtime-rules. Returns 201.
// RBAC: super_admin, hr_admin.
// OR-1: min_minutes < 30 → 422 RULE_VIOLATION (enforced in service).
func (h *MasterDataHandler) CreateOvertimeRule(w http.ResponseWriter, r *http.Request) {
	var req createOvertimeRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	fields := map[string]string{}
	if req.Name == nil || *req.Name == "" {
		fields["name"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	params := svc.CreateOvertimeRuleParams{
		Name:                derefString(req.Name),
		ServiceLineID:       req.ServiceLineID,
		PreApprovalRequired: derefBool(req.PreApprovalRequired),
	}
	if req.WeekdayRate != nil {
		params.WeekdayRate = *req.WeekdayRate
	}
	if req.RestdayRate != nil {
		params.RestdayRate = *req.RestdayRate
	}
	if req.HolidayRate != nil {
		params.HolidayRate = *req.HolidayRate
	}
	if req.MinMinutes != nil {
		params.MinMinutes = *req.MinMinutes
	}
	if req.MaxMinutesPerDay != nil {
		params.MaxMinutesPerDay = *req.MaxMinutesPerDay
	}

	otr, err := h.svc.CreateOvertimeRule(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/overtime-rules/"+otr.ID)
	httpx.WriteJSON(w, http.StatusCreated, toOvertimeRuleResponse(otr))
}

// UpdateOvertimeRule handles PATCH /overtime-rules/{overtime_rule_id}.
// RBAC: super_admin, hr_admin.
func (h *MasterDataHandler) UpdateOvertimeRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "overtime_rule_id")

	var req updateOvertimeRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Service carries forward zero/nil fields from current record.
	params := svc.UpdateOvertimeRuleParams{
		ID:                  id,
		Name:                derefString(req.Name),
		ServiceLineID:       req.ServiceLineID,
		PreApprovalRequired: derefBool(req.PreApprovalRequired),
	}
	if req.WeekdayRate != nil {
		params.WeekdayRate = *req.WeekdayRate
	}
	if req.RestdayRate != nil {
		params.RestdayRate = *req.RestdayRate
	}
	if req.HolidayRate != nil {
		params.HolidayRate = *req.HolidayRate
	}
	if req.MinMinutes != nil {
		params.MinMinutes = *req.MinMinutes
	}
	if req.MaxMinutesPerDay != nil {
		params.MaxMinutesPerDay = *req.MaxMinutesPerDay
	}

	otr, err := h.svc.UpdateOvertimeRule(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toOvertimeRuleResponse(otr))
}

// SoftDeleteOvertimeRule handles DELETE /overtime-rules/{overtime_rule_id}. Returns 204.
// RBAC: super_admin, hr_admin.
func (h *MasterDataHandler) SoftDeleteOvertimeRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "overtime_rule_id")

	if err := h.svc.SoftDeleteOvertimeRule(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// Local helper extensions (reuses decodeJSON/parseLimit/queryStringPtr/derefString
// from companies_handler.go — no redeclaration needed)
// =============================================================================

// derefBool safely dereferences a *bool, returning false if nil.
func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// coalesceB returns the dereferenced bool pointer value, or fallback if nil.
func coalesceB(ptr *bool, fallback bool) bool {
	if ptr != nil {
		return *ptr
	}
	return fallback
}
