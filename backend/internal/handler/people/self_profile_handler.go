package people

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
)

// SelfProfileHandler serves the agent self-service profile endpoints
// (PATCH /me/profile + POST /me/profile/photo-upload-init). x-rbac roles:[agent]
// scope:self — the service resolves the employee from the principal.
type SelfProfileHandler struct {
	svc *svc.SelfProfileService
}

// NewSelfProfileHandler returns a SelfProfileHandler wired to the given service.
func NewSelfProfileHandler(s *svc.SelfProfileService) *SelfProfileHandler {
	return &SelfProfileHandler{svc: s}
}

// UpdateMyProfile handles PATCH /me/profile — instant-tier self apply of
// {address?, app_language?, photo_object_key?}. Approval-tier fields are not in
// the schema and are rejected by the service. Returns 200 with the updated
// employee (incl. presigned photo_url).
func (h *SelfProfileHandler) UpdateMyProfile(w http.ResponseWriter, r *http.Request) {
	var body selfProfileUpdateBody
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		httpx.WriteError(w, r, apperr.Invalid(nil).WithCause(err))
		return
	}

	// minProperties:1 — at least one instant-tier field must be present.
	if body.Address == nil && body.AppLanguage == nil && body.PhotoObjectKey == nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{
			"changes": "Minimal satu field perubahan wajib diisi.",
		}))
		return
	}

	updated, err := h.svc.UpdateMyProfile(r.Context(), svc.SelfProfileInput{
		Address:        body.Address,
		AppLanguage:    body.AppLanguage,
		PhotoObjectKey: body.PhotoObjectKey,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	photoURL := h.svc.PhotoURL(r.Context(), updated.PhotoObjectKey)
	httpx.WriteJSON(w, http.StatusOK, toSelfProfileResponse(updated, photoURL))
}

// InitProfilePhotoUpload handles POST /me/profile/photo-upload-init — validates
// the content-type/size and returns a presigned PUT UploadTicket. Returns 200.
func (h *SelfProfileHandler) InitProfilePhotoUpload(w http.ResponseWriter, r *http.Request) {
	var body photoUploadInitBody
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		httpx.WriteError(w, r, apperr.Invalid(nil).WithCause(err))
		return
	}
	if body.ContentType == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{
			"content_type": "Wajib diisi.",
		}))
		return
	}

	ticket, err := h.svc.InitPhotoUpload(r.Context(), svc.PhotoUploadInput{
		ContentType:   body.ContentType,
		ContentLength: body.ContentLength,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, uploadTicketResponse{
		UploadURL:   ticket.UploadURL,
		ObjectKey:   ticket.ObjectKey,
		ContentType: ticket.ContentType,
		MaxBytes:    ticket.MaxBytes,
		ExpiresAt:   ticket.ExpiresAt.UTC().Format(time.RFC3339),
	})
}
