// Package foundations (handler) is the HTTP boundary for E1 user management,
// audit-log reads, and platform settings. Hand-written chi handlers — no server
// codegen (oapi-codegen cannot parse OpenAPI 3.1 specs).
package foundations

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/foundations"
)

// pageCursor is the local mirror of the service's pageCursor payload so the
// handler can decode the opaque cursor string without importing the service's
// unexported type. The JSON keys must match exactly.
type pageCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// Handler holds the foundations service and handles all E1 resource endpoints.
type Handler struct {
	svc *svc.Service
}

// NewHandler returns a Handler wired to the given service.
func NewHandler(s *svc.Service) *Handler {
	return &Handler{svc: s}
}

// --- Users ---

// ListUsers handles GET /users with optional filters + cursor pagination.
// RBAC: super_admin, hr_admin (middleware-enforced in server.go).
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := domain.UserFilter{
		Q:         queryStringPtr(q.Get("q")),
		Role:      queryStringPtr(q.Get("role")),
		Status:    queryStringPtr(q.Get("status")),
		CompanyID: queryStringPtr(q.Get("company_id")),
		Limit:     parseLimit(q.Get("limit")),
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

	users, nextCursor, err := h.svc.ListUsers(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]userResponse, 0, len(users))
	for _, u := range users {
		items = append(items, toUserResponse(u))
	}

	resp := httpx.PageResponse[userResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// CreateUser handles POST /users.
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	fields := map[string]string{}
	if req.Email == "" {
		fields["email"] = "Wajib diisi."
	}
	if req.Role == "" {
		fields["role"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	sendInvite := false
	if req.SendInvitationEmail != nil {
		sendInvite = *req.SendInvitationEmail
	}

	user, err := h.svc.CreateUser(r.Context(), req.Email, req.Role, req.EmployeeID, sendInvite)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/users/"+user.ID)
	httpx.WriteJSON(w, http.StatusCreated, toUserResponse(user))
}

// UpdateUser handles PATCH /users/{user_id}.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "user_id")

	var req updateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Email == nil || *req.Email == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"email": "Wajib diisi."}))
		return
	}

	user, err := h.svc.UpdateUser(r.Context(), id, *req.Email)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toUserResponse(user))
}

// ChangeUserRole handles POST /users/{user_id}:change-role.
func (h *Handler) ChangeUserRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "user_id")

	var req changeRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.NewRole == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"new_role": "Wajib diisi."}))
		return
	}

	reason := ""
	if req.Reason != nil {
		reason = *req.Reason
	}

	user, err := h.svc.ChangeUserRole(r.Context(), id, req.NewRole, reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toUserResponse(user))
}

// DeactivateUser handles POST /users/{user_id}:deactivate.
func (h *Handler) DeactivateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "user_id")

	var req reasonRequest
	// Body is optional — ignore decode errors for empty bodies.
	_ = decodeJSON(r, &req)

	reason := ""
	if req.Reason != nil {
		reason = *req.Reason
	}

	user, err := h.svc.DeactivateUser(r.Context(), id, reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toUserResponse(user))
}

// ReactivateUser handles POST /users/{user_id}:reactivate.
func (h *Handler) ReactivateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "user_id")

	var req reasonRequest
	_ = decodeJSON(r, &req)

	reason := ""
	if req.Reason != nil {
		reason = *req.Reason
	}

	user, err := h.svc.ReactivateUser(r.Context(), id, reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toUserResponse(user))
}

// SendUserPasswordReset handles POST /users/{user_id}:send-password-reset.
// No Idempotency-Key required on this endpoint (send-password-reset is not flagged).
func (h *Handler) SendUserPasswordReset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "user_id")

	email, err := h.svc.SendUserPasswordReset(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{
		"message": fmt.Sprintf("Tautan reset dikirim ke %s.", email),
	})
}

// --- Audit log ---

// ListAuditLog handles GET /audit-log with optional filters + cursor pagination.
func (h *Handler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := domain.AuditFilter{
		Q:           queryStringPtr(q.Get("q")),
		ActorUserID: queryStringPtr(q.Get("actor_user_id")),
		Action:      queryStringPtr(q.Get("action")),
		EntityType:  queryStringPtr(q.Get("entity_type")),
		EntityID:    queryStringPtr(q.Get("entity_id")),
		Limit:       parseLimit(q.Get("limit")),
	}

	if v := q.Get("created_at__gte"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.CreatedGTE = &t
		}
	}
	if v := q.Get("created_at__lte"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.CreatedLTE = &t
		}
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

	entries, nextCursor, err := h.svc.ListAuditLog(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]auditSummaryResponse, 0, len(entries))
	for _, e := range entries {
		items = append(items, toAuditSummary(e))
	}

	resp := httpx.PageResponse[auditSummaryResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// GetAuditLogEntry handles GET /audit-log/{audit_log_id}.
func (h *Handler) GetAuditLogEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "audit_log_id")

	entry, err := h.svc.GetAuditLogEntry(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toAuditEntry(entry))
}

// --- Platform settings ---

// GetPlatformSettings handles GET /platform/settings.
func (h *Handler) GetPlatformSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.GetPlatformSettings(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPlatformSettingsResponse(settings))
}

// --- shared helpers ---

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
		return 0 // ClampLimit will apply the default
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
