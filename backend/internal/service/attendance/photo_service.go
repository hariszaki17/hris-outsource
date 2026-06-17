// Package attendance — PhotoService implements the agent clock-in/out selfie upload
// (E5 F5.1 / CI-10, 2026-06-17). An agent uploads a photo BEFORE clocking; the service
// validates size (≤10 MB → 413 FILE_TOO_LARGE) and MIME (image/jpeg|image/png → 415
// UNSUPPORTED_MEDIA_TYPE), stores the bytes, and returns an SWP-FILE-* id the client
// passes to clock-in/out as photo_id. Mirrors people.AgreementService.UploadAttachment
// (in-DB bytea blob, SWP-FILE id minting), but the row is standalone — not attached to
// any attendance record at upload time. Orphan uploads expire after 24h.
package attendance

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
)

// maxPhotoBytes is the upload ceiling (spec: 10 MB).
const maxPhotoBytes = 10 * 1024 * 1024

// allowedPhotoMIME is the upload content-type allowlist (spec: JPEG + PNG only).
var allowedPhotoMIME = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
}

// UploadedPhoto is the stored-file projection returned to the handler (→ UploadedFile).
type UploadedPhoto struct {
	ID         string
	FileName   string
	MIME       string
	SizeBytes  int64
	UploadedAt time.Time
}

// CreateAttendancePhotoParams carries one selfie upload to the repo.
type CreateAttendancePhotoParams struct {
	EmployeeID string
	Caption    string
	FileName   string
	MIME       string
	SizeBytes  int64
	Blob       []byte
}

// PhotoRepository is the data dependency for the photo service (consumer-defined).
type PhotoRepository interface {
	CreateAttendancePhoto(ctx context.Context, tx pgx.Tx, p CreateAttendancePhotoParams) (UploadedPhoto, error)
	GetAttendancePhotoByID(ctx context.Context, id string) (domain.Attachment, error)
}

// PhotoService implements the attendance-selfie upload business logic.
type PhotoService struct {
	repo PhotoRepository
	txm  TxRunner
}

// NewPhotoService wires the photo service with its dependencies.
func NewPhotoService(repo PhotoRepository, txm TxRunner) *PhotoService {
	return &PhotoService{repo: repo, txm: txm}
}

// Upload validates + stores one clock-in/out selfie, returning the SWP-FILE-* id.
//   - size > 10 MB → 413 FILE_TOO_LARGE (fields size_bytes / max_size_bytes)
//   - mime ∉ {jpeg,png} → 415 UNSUPPORTED_MEDIA_TYPE (field mime)
//
// TODO(orphan-expiry): the row carries expires_at = created_at + 24h. A sweep that
// deletes orphaned (never-attached) photos past their horizon is NOT built here (no
// cron). When CI-10's presigned-direct-upload migration lands this becomes a bucket
// lifecycle rule; until then the rows accumulate (bounded; harmless).
func (s *PhotoService) Upload(ctx context.Context, employeeID string, p CreateAttendancePhotoParams) (UploadedPhoto, error) {
	if p.SizeBytes > maxPhotoBytes {
		return UploadedPhoto{}, &apperr.Error{
			Code:       "FILE_TOO_LARGE",
			HTTPStatus: http.StatusRequestEntityTooLarge,
			Fields: map[string]string{
				"size_bytes":     itoa64(p.SizeBytes),
				"max_size_bytes": itoa64(maxPhotoBytes),
			},
		}
	}
	if !allowedPhotoMIME[p.MIME] {
		return UploadedPhoto{}, &apperr.Error{
			Code:       "UNSUPPORTED_MEDIA_TYPE",
			HTTPStatus: http.StatusUnsupportedMediaType,
			Fields:     map[string]string{"mime": p.MIME},
		}
	}

	p.EmployeeID = employeeID

	var created UploadedPhoto
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateAttendancePhoto(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "attendance_photo",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"employee_id": employeeID,
				"file_name":   p.FileName,
				"mime":        p.MIME,
				"size_bytes":  p.SizeBytes,
				"source":      "attendance_photo_upload",
			},
		})
	}); err != nil {
		return UploadedPhoto{}, asAppErr(err)
	}
	return created, nil
}

// Get returns selfie metadata + blob for the authenticated file-download handler.
func (s *PhotoService) Get(ctx context.Context, fileID string) (domain.Attachment, error) {
	att, err := s.repo.GetAttendancePhotoByID(ctx, fileID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Attachment{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Attachment{}, apperr.Internal(err)
	}
	return att, nil
}

// itoa64 formats a non-negative int64 for the string error-field map.
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
