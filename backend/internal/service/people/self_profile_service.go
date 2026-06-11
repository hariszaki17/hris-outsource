// Package people — SelfProfileService implements the agent self-service profile
// surface (E2 F2.1 / EP-5, 2026-06-11 redesign):
//   - UpdateMyProfile: instant-tier self apply of {address, app_language,
//     photo_object_key}. No approval, no notification — audited. Approval-tier
//     fields (phone/emergency_contact/bank_account) are rejected here with
//     FIELD_REQUIRES_APPROVAL (the agent must file a change request instead).
//   - InitPhotoUpload: validates the content-type/size and returns a presigned
//     PUT UploadTicket (server-built key in the caller's own namespace) via the
//     storage client. The client PUTs the bytes, then applies the returned
//     object_key through UpdateMyProfile.
//
// The employee is always the authenticated principal (scope:self, GuardSelf);
// there is no path id. Separate struct in the same package — mirrors the
// AgreementService / ChangeRequestService pattern.
package people

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/storage"
)

// SelfProfileRepository is the data dependency for the self-profile service.
type SelfProfileRepository interface {
	GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error)
	// UpdateEmployeeSelfInstant applies the instant-tier fields (COALESCE keeps a
	// column unchanged when the param is nil) and returns the updated employee
	// (with the new emergency_contact/app_language/photo fields populated).
	UpdateEmployeeSelfInstant(ctx context.Context, tx pgx.Tx, p UpdateEmployeeSelfInstantParams) (domain.Employee, error)
}

// UpdateEmployeeSelfInstantParams carries the optional instant-tier fields for
// the COALESCE patch. A nil pointer leaves the column unchanged.
type UpdateEmployeeSelfInstantParams struct {
	ID             string
	Address        *string
	AppLanguage    *string
	PhotoObjectKey *string
}

// SelfProfileInput is the validated PATCH /me/profile body (SelfProfileUpdate).
// At least one field must be present (handler/DTO enforces minProperties:1).
type SelfProfileInput struct {
	Address        *string
	AppLanguage    *string
	PhotoObjectKey *string
}

// SelfProfileService implements the agent self-service profile logic.
type SelfProfileService struct {
	repo  SelfProfileRepository
	store storage.Storage
	txm   TxRunner // reuse TxRunner defined in employees_service.go
}

// NewSelfProfileService wires the service with its dependencies.
func NewSelfProfileService(repo SelfProfileRepository, store storage.Storage, txm TxRunner) *SelfProfileService {
	return &SelfProfileService{repo: repo, store: store, txm: txm}
}

// UpdateMyProfile instant-applies the caller's own instant-tier fields. The
// employee is the authenticated principal (GuardSelf). app_language is validated
// (id|en). A photo_object_key must belong to the caller's own profile-photo key
// namespace (else 422 — no cross-employee key smuggling). Audited; no notify.
func (s *SelfProfileService) UpdateMyProfile(ctx context.Context, in SelfProfileInput) (domain.Employee, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return domain.Employee{}, apperr.Unauthenticated()
	}
	employeeID := p.EmployeeID
	if employeeID == "" {
		return domain.Employee{}, apperr.Forbidden()
	}
	// Defense-in-depth: only the agent's own record (RequireRole gates the route).
	if serr := rbac.GuardSelf(ctx, employeeID); serr != nil {
		return domain.Employee{}, serr
	}

	// Validate app_language whitelist.
	if in.AppLanguage != nil {
		lang := strings.TrimSpace(*in.AppLanguage)
		if lang != "id" && lang != "en" {
			return domain.Employee{}, apperr.Invalid(map[string]string{
				"app_language": "Bahasa harus 'id' atau 'en'.",
			})
		}
		in.AppLanguage = &lang
	}

	// Validate a supplied photo_object_key belongs to the caller's namespace
	// (profile-photos/{employee_id}/...). Rejects cross-employee keys (422).
	if in.PhotoObjectKey != nil {
		key := strings.TrimSpace(*in.PhotoObjectKey)
		prefix := string(storage.NSProfilePhotos) + "/" + employeeID + "/"
		if !strings.HasPrefix(key, prefix) {
			return domain.Employee{}, &apperr.Error{
				Code:       "FIELD_REQUIRES_APPROVAL",
				HTTPStatus: 422,
				Message:    "Kunci foto tidak valid untuk akun ini.",
				Fields:     map[string]string{"photo_object_key": "Kunci foto tidak dikeluarkan untuk akun ini."},
			}
		}
		in.PhotoObjectKey = &key
	}

	if in.Address != nil {
		addr := strings.TrimSpace(*in.Address)
		in.Address = &addr
	}

	before, err := s.repo.GetEmployeeByID(ctx, employeeID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Employee{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Employee{}, apperr.Internal(err)
	}

	var updated domain.Employee
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdateEmployeeSelfInstant(ctx, tx, UpdateEmployeeSelfInstantParams{
			ID:             employeeID,
			Address:        in.Address,
			AppLanguage:    in.AppLanguage,
			PhotoObjectKey: in.PhotoObjectKey,
		})
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("employee.self_update"),
			EntityType: "employee",
			EntityID:   employeeID,
			Before:     selfProfileSnap(before),
			After:      selfProfileSnap(updated),
		})
	}); err != nil {
		return domain.Employee{}, apperr.Internal(err)
	}

	return updated, nil
}

// PhotoUploadInput is the validated POST /me/profile/photo-upload-init body.
type PhotoUploadInput struct {
	ContentType   string
	ContentLength int64 // optional; 0 = unspecified
}

// InitPhotoUpload validates the content-type/size and returns a presigned PUT
// ticket scoped to the caller's own profile-photo namespace. The storage client
// pins the content-type + size into the signature, so a deviating upload fails at
// the storage layer. Self scope — the employee is the authenticated principal.
func (s *SelfProfileService) InitPhotoUpload(ctx context.Context, in PhotoUploadInput) (storage.UploadTicket, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return storage.UploadTicket{}, apperr.Unauthenticated()
	}
	employeeID := p.EmployeeID
	if employeeID == "" {
		return storage.UploadTicket{}, apperr.Forbidden()
	}
	if serr := rbac.GuardSelf(ctx, employeeID); serr != nil {
		return storage.UploadTicket{}, serr
	}

	ticket, err := s.store.PresignPut(ctx, storage.NSProfilePhotos, employeeID, in.ContentType, in.ContentLength)
	switch {
	case errors.Is(err, storage.ErrContentTypeNotAllowed):
		return storage.UploadTicket{}, apperr.Invalid(map[string]string{
			"content_type": "Tipe file tidak didukung. Gunakan JPEG, PNG, atau WebP.",
		})
	case errors.Is(err, storage.ErrFileTooLarge):
		return storage.UploadTicket{}, &apperr.Error{
			Code:       "UPLOAD_TOO_LARGE",
			HTTPStatus: 422,
			Message:    "Ukuran foto melebihi batas yang diizinkan.",
			Fields:     map[string]string{"content_length": "Ukuran melebihi batas maksimum."},
		}
	case err != nil:
		return storage.UploadTicket{}, apperr.Internal(err)
	}
	return ticket, nil
}

// PhotoURL builds a short-TTL presigned GET URL for an employee's photo object
// key (nil/empty → "", no URL). Used by the DTO boundary to render photo_url.
func (s *SelfProfileService) PhotoURL(ctx context.Context, objectKey *string) string {
	if objectKey == nil || *objectKey == "" {
		return ""
	}
	url, err := s.store.PresignGet(ctx, *objectKey)
	if err != nil {
		return ""
	}
	return url
}

// selfProfileSnap captures the instant-tier fields for the audit diff.
func selfProfileSnap(e domain.Employee) map[string]any {
	return map[string]any{
		"address":          derefPtrStr(e.Address),
		"app_language":     e.AppLanguage,
		"photo_object_key": derefPtrStr(e.PhotoObjectKey),
	}
}
