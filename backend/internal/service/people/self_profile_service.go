// Package people — SelfProfileService implements the agent self-service profile
// surface (E2 F2.1 / EP-5; E11 instant-edit redesign 2026-06-14, EPICS §8 A):
//   - UpdateMyProfile: INSTANT self apply of every editable profile field —
//     {address, app_language, photo_object_key, phone, emergency_contact,
//     bank_account}. No approval queue, no notification — audited. The profile
//     change-request approval surface was hard-deleted (E11): there is no longer
//     a FIELD_REQUIRES_APPROVAL tier. Phone, when changed, is normalized to
//     E.164 and guarded for login-identifier uniqueness (409 CONFLICT); the
//     login phone on the linked users row is updated in the same tx so the agent
//     can keep signing in with the new number.
//   - InitPhotoUpload: validates the content-type/size and returns a presigned
//     PUT UploadTicket (server-built key in the caller's own namespace) via the
//     storage client. The client PUTs the bytes, then applies the returned
//     object_key through UpdateMyProfile.
//
// The employee is always the authenticated principal (scope:self, GuardSelf);
// there is no path id. Separate struct in the same package — mirrors the
// AgreementService pattern.
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
	// UserPhoneTaken reports whether a (non-deleted) login user already uses this
	// E.164 phone — the login-identifier uniqueness guard (mirrors EmployeeRepository).
	UserPhoneTaken(ctx context.Context, phone string) (bool, error)
	// UpdateEmployeeSelfInstant applies the instant self-edit fields (COALESCE keeps a
	// column unchanged when the param is nil) and returns the updated employee. When
	// Phone is non-nil it ALSO updates the linked users.phone login identifier in the
	// same tx so login stays consistent.
	UpdateEmployeeSelfInstant(ctx context.Context, tx pgx.Tx, p UpdateEmployeeSelfInstantParams) (domain.Employee, error)
}

// UpdateEmployeeSelfInstantParams carries the optional instant self-edit fields for
// the COALESCE patch. A nil pointer leaves the column unchanged. E11: phone,
// emergency_contact, and bank_account joined the instant set (the change-request
// approval queue was removed).
type UpdateEmployeeSelfInstantParams struct {
	ID                    string
	Address               *string
	AppLanguage           *string
	PhotoObjectKey        *string
	Phone                 *string // normalized E.164; also propagated to users.phone
	EmergencyContactName  *string
	EmergencyContactPhone *string
	BankName              *string
	BankAccountNumber     *string
	BankAccountHolderName *string
}

// SelfProfileInput is the validated PATCH /me/profile body (SelfProfileUpdate).
// At least one field must be present (handler/DTO enforces minProperties:1). E11:
// phone / emergency_contact / bank_account are now instant self-edits too.
type SelfProfileInput struct {
	Address        *string
	AppLanguage    *string
	PhotoObjectKey *string
	Phone          *string
	// EmergencyContact: a non-nil pointer means "update the emergency contact" —
	// both name and phone are written (the FE always submits the full object).
	EmergencyContact *domain.EmergencyContact
	// BankAccount: a non-nil pointer means "update the bank account" — all three
	// fields are written together (the FE always submits the full object).
	BankAccount *domain.BankAccount
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

// UpdateMyProfile INSTANTLY applies the caller's own editable profile fields
// (E11). The employee is the authenticated principal (GuardSelf). app_language is
// validated (id|en). A photo_object_key must belong to the caller's own
// profile-photo key namespace (else 422 — no cross-employee key smuggling). When
// phone changes it is normalized to E.164 and checked for login-identifier
// uniqueness (409 CONFLICT); the linked users.phone is updated in the same tx.
// emergency_contact and bank_account apply instantly too. Audited; no notify.
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

	// Phone: normalize to E.164 and (if actually changed) guard login-identifier
	// uniqueness. A blank phone is rejected — phone is the required login id.
	var phoneParam *string
	if in.Phone != nil {
		normalized := normalizePhone(*in.Phone)
		if normalized == "" {
			return domain.Employee{}, apperr.Invalid(map[string]string{
				"phone": "Wajib diisi (dipakai sebagai identitas login).",
			})
		}
		currentPhone := ""
		if before.Phone != nil {
			currentPhone = *before.Phone
		}
		if normalized != currentPhone {
			taken, terr := s.repo.UserPhoneTaken(ctx, normalized)
			if terr != nil {
				return domain.Employee{}, apperr.Internal(terr)
			}
			if taken {
				return domain.Employee{}, loginPhoneConflictErr()
			}
			phoneParam = &normalized
		}
		// else: unchanged → leave phoneParam nil so we neither re-check nor rewrite.
	}

	// emergency_contact / bank_account: a non-nil object means "write all of it".
	var ecName, ecPhone *string
	if in.EmergencyContact != nil {
		n := strings.TrimSpace(in.EmergencyContact.Name)
		ph := strings.TrimSpace(in.EmergencyContact.Phone)
		ecName, ecPhone = &n, &ph
	}
	var bankName, bankNo, bankHolder *string
	if in.BankAccount != nil {
		bn := strings.TrimSpace(in.BankAccount.BankName)
		an := strings.TrimSpace(in.BankAccount.AccountNumber)
		ah := strings.TrimSpace(in.BankAccount.AccountHolderName)
		bankName, bankNo, bankHolder = &bn, &an, &ah
	}

	var updated domain.Employee
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdateEmployeeSelfInstant(ctx, tx, UpdateEmployeeSelfInstantParams{
			ID:                    employeeID,
			Address:               in.Address,
			AppLanguage:           in.AppLanguage,
			PhotoObjectKey:        in.PhotoObjectKey,
			Phone:                 phoneParam,
			EmergencyContactName:  ecName,
			EmergencyContactPhone: ecPhone,
			BankName:              bankName,
			BankAccountNumber:     bankNo,
			BankAccountHolderName: bankHolder,
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
		return domain.Employee{}, wrapTxErr(err)
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

// selfProfileSnap captures the instant self-edit fields for the audit diff (E11:
// now includes phone / emergency_contact / bank_account alongside the originals).
func selfProfileSnap(e domain.Employee) map[string]any {
	return map[string]any{
		"address":          derefPtrStr(e.Address),
		"app_language":     e.AppLanguage,
		"photo_object_key": derefPtrStr(e.PhotoObjectKey),
		"phone":            derefPtrStr(e.Phone),
		"emergency_contact": map[string]any{
			"name":  e.EmergencyContact.Name,
			"phone": e.EmergencyContact.Phone,
		},
		"bank_account": map[string]any{
			"bank_name":           e.BankAccount.BankName,
			"account_number":      e.BankAccount.AccountNumber,
			"account_holder_name": e.BankAccount.AccountHolderName,
		},
	}
}

// derefPtrStr safely dereferences a *string (returns "" if nil).
func derefPtrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
