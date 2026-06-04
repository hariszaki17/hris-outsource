package people

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
)

// ChangeRequestHandler holds the change-request service and handles all
// E2 change-request queue endpoints (GET list, GET detail, POST :approve, POST :reject).
// RBAC: hr_admin and super_admin (enforced via route groups in server.go).
type ChangeRequestHandler struct {
	svc *svc.ChangeRequestService
}

// NewChangeRequestHandler returns a ChangeRequestHandler wired to the given service.
func NewChangeRequestHandler(s *svc.ChangeRequestService) *ChangeRequestHandler {
	return &ChangeRequestHandler{svc: s}
}

// crHandlerCursor is the handler-side opaque cursor payload for change-request pagination.
// Must match crPageCursor in the service (both encode/decode the same JSON blob).
type crHandlerCursor struct {
	SubmittedAt time.Time `json:"s"`
	ID          string    `json:"i"`
}

// ListPendingChangeRequests handles GET /change-requests.
// Supports filters: status, employee_id, request_type, q (reserved/pass-through).
// Default queue view uses status=PENDING from the FE; endpoint supports all statuses.
func (h *ChangeRequestHandler) ListPendingChangeRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := domain.ChangeRequestFilter{
		Status:      queryStringPtr(q.Get("status")),
		EmployeeID:  queryStringPtr(q.Get("employee_id")),
		RequestType: queryStringPtr(q.Get("request_type")),
		Q:           queryStringPtr(q.Get("q")),
		Limit:       parseLimit(q.Get("limit")),
	}

	if cursor := q.Get("cursor"); cursor != "" {
		var p crHandlerCursor
		if err := httpx.DecodeCursor(cursor, &p); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		filter.CursorSubmittedAt = &p.SubmittedAt
		filter.CursorID = &p.ID
	}

	crs, nextCursor, err := h.svc.ListChangeRequests(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items := make([]changeRequestResponse, 0, len(crs))
	for _, cr := range crs {
		items = append(items, toChangeRequestResponse(cr))
	}

	resp := httpx.PageResponse[changeRequestResponse]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != nil,
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// GetChangeRequest handles GET /change-requests/{change_request_id}.
// Returns the detail view including employee{id,full_name,nip} and diff{field:{old,new}}.
func (h *ChangeRequestHandler) GetChangeRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "change_request_id")

	detail, err := h.svc.GetChangeRequestDetail(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toChangeRequestDetailResponse(detail))
}

// ApproveChangeRequest handles POST /change-requests/{change_request_id}:approve.
// No request body required. Actor is derived from the principal.
// Returns 200 with the updated change request.
// Returns 409 CONFLICT if the request is not pending.
func (h *ChangeRequestHandler) ApproveChangeRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "change_request_id")

	principal, _ := auth.PrincipalFrom(r.Context())
	actor := principal.UserID

	cr, err := h.svc.ApproveChangeRequest(r.Context(), id, actor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toChangeRequestResponse(cr))
}

// RejectChangeRequest handles POST /change-requests/{change_request_id}:reject.
// Body: {reason: string} — required, minLength 3, maxLength 500.
// Returns 200 with the updated change request.
// Returns 400 if reason is missing/too short.
// Returns 409 CONFLICT if the request is not pending.
func (h *ChangeRequestHandler) RejectChangeRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "change_request_id")

	var req rejectRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, apperr.Invalid(nil).WithCause(err))
		return
	}

	principal, _ := auth.PrincipalFrom(r.Context())
	actor := principal.UserID

	cr, err := h.svc.RejectChangeRequest(r.Context(), id, req.Reason, actor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toChangeRequestResponse(cr))
}
