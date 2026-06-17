// Package attendance (handler) — agent clock-in/out photo upload (F5.1 / CI-10):
// POST /attendance:photo-upload (201, multipart). The agent is always self (the
// service derives employee_id from the token). Mirrors people.AgreementHandler's
// multipart UploadAttachment shape (CONVENTIONS §15): field "file" required, optional
// "caption"; ≤10 MB; image/jpeg|image/png only. Returns an UploadedFile with an
// SWP-FILE-* id the client passes to clock-in/out as photo_id.
package attendance

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// maxPhotoUploadBytes bounds the multipart parse buffer (matches the 10 MB ceiling).
const maxPhotoUploadBytes = 10 << 20

// maxCaptionLen is the spec's caption maxLength.
const maxCaptionLen = 200

// PhotoHandler holds the attendance-selfie upload service.
type PhotoHandler struct {
	photos *svc.PhotoService
}

// NewPhotoHandler wires the handler to the photo service.
func NewPhotoHandler(p *svc.PhotoService) *PhotoHandler {
	return &PhotoHandler{photos: p}
}

// uploadedFileResponse is the openapi UploadedFile schema:
// {id, url, name, size_bytes, mime, uploaded_at}. url points at the authenticated
// download route /api/v1/files/{id}.
type uploadedFileResponse struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Name       string `json:"name"`
	SizeBytes  int64  `json:"size_bytes"`
	MIME       string `json:"mime"`
	UploadedAt string `json:"uploaded_at"` // RFC3339
}

// UploadPhoto handles POST /attendance:photo-upload (201, UploadedFile).
func (h *PhotoHandler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.Unauthenticated())
		return
	}
	if p.EmployeeID == "" {
		httpx.WriteError(w, r, apperr.OutOfScope())
		return
	}

	if err := r.ParseMultipartForm(maxPhotoUploadBytes); err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"file": "Gagal mem-parsing multipart form."}))
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"file": "Field 'file' wajib diisi."}))
		return
	}
	defer file.Close()

	blob, err := io.ReadAll(file)
	if err != nil {
		httpx.WriteError(w, r, apperr.Internal(fmt.Errorf("read file: %w", err)))
		return
	}

	caption := r.FormValue("caption")
	if len(caption) > maxCaptionLen {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"caption": "Keterangan melebihi 200 karakter."}))
		return
	}

	// MIME: prefer the part header, fall back to content sniffing.
	mime := fileHeader.Header.Get("Content-Type")
	if mime == "" {
		mime = http.DetectContentType(blob)
	}

	stored, err := h.photos.Upload(r.Context(), p.EmployeeID, svc.CreateAttendancePhotoParams{
		Caption:   caption,
		FileName:  fileHeader.Filename,
		MIME:      mime,
		SizeBytes: int64(len(blob)),
		Blob:      blob,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, uploadedFileResponse{
		ID:         stored.ID,
		URL:        "/api/v1/files/" + stored.ID,
		Name:       stored.FileName,
		SizeBytes:  stored.SizeBytes,
		MIME:       stored.MIME,
		UploadedAt: stored.UploadedAt.UTC().Format(time.RFC3339),
	})
}

// DownloadPhoto serves an attendance selfie's bytes (authenticated). It is exposed as a
// FALLBACK for GET /files/{file_id}: the people agreement-attachment download owns that
// route and serves its own (disjoint) id space; on a miss it delegates here so selfies
// uploaded via :photo-upload are renderable at the spec's /api/v1/files/{id} URL.
// fileID is passed explicitly (the route param is read by the owning handler).
func (h *PhotoHandler) DownloadPhoto(w http.ResponseWriter, r *http.Request, fileID string) bool {
	att, err := h.photos.Get(r.Context(), fileID)
	if err != nil {
		return false // not an attendance photo (or not found) — let the caller 404.
	}
	w.Header().Set("Content-Type", att.MIME)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, att.FileName))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(att.Blob)
	return true
}
