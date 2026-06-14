// Package people — white-box unit tests for SelfProfileService (E2 F2.1 agent
// self-service): the instant-tier apply (audited, no notify), the photo-upload
// content-type/size validation + ticket, and the GuardSelf cross-employee guard.
package people

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/storage"
)

// ---------------------------------------------------------------------------
// Shared test helpers (previously in change_requests_service_test.go, removed in
// E11 when the change-request approval surface was hard-deleted).
// ---------------------------------------------------------------------------

// errCode extracts the apperr.Error machine code ("" if not an *apperr.Error).
func errCode(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

// crTx is a minimal pgx.Tx whose Exec is a no-op (audit.Record calls Exec).
type crTx struct{}

func (crTx) Begin(context.Context) (pgx.Tx, error) { return crTx{}, nil }
func (crTx) Commit(context.Context) error          { return nil }
func (crTx) Rollback(context.Context) error        { return nil }
func (crTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (crTx) Query(context.Context, string, ...any) (pgx.Rows, error) { panic("Query") }
func (crTx) QueryRow(context.Context, string, ...any) pgx.Row        { panic("QueryRow") }
func (crTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("CopyFrom")
}
func (crTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("SendBatch") }
func (crTx) LargeObjects() pgx.LargeObjects                         { panic("LargeObjects") }
func (crTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("Prepare")
}
func (crTx) Conn() *pgx.Conn { return nil }

// crTxRunner runs the body with a no-op tx (commit-on-success semantics implicit).
type crTxRunner struct{}

func (crTxRunner) InTx(_ context.Context, fn func(pgx.Tx) error) error { return fn(crTx{}) }

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// selfRepo is an in-memory SelfProfileRepository that records the instant patch.
type selfRepo struct {
	employees map[string]domain.Employee
	takenPhones map[string]bool // E.164 phones already used by some login
	lastPatch *UpdateEmployeeSelfInstantParams
}

func newSelfRepo() *selfRepo {
	return &selfRepo{employees: map[string]domain.Employee{}, takenPhones: map[string]bool{}}
}

func (r *selfRepo) GetEmployeeByID(_ context.Context, id string) (domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	return e, nil
}

func (r *selfRepo) UserPhoneTaken(_ context.Context, phone string) (bool, error) {
	return r.takenPhones[phone], nil
}

func (r *selfRepo) UpdateEmployeeSelfInstant(_ context.Context, _ pgx.Tx, p UpdateEmployeeSelfInstantParams) (domain.Employee, error) {
	cp := p
	r.lastPatch = &cp
	e := r.employees[p.ID]
	if p.Address != nil {
		e.Address = p.Address
	}
	if p.AppLanguage != nil {
		e.AppLanguage = *p.AppLanguage
	}
	if p.PhotoObjectKey != nil {
		e.PhotoObjectKey = p.PhotoObjectKey
	}
	if p.Phone != nil {
		e.Phone = p.Phone
	}
	if p.EmergencyContactName != nil {
		e.EmergencyContact.Name = *p.EmergencyContactName
	}
	if p.EmergencyContactPhone != nil {
		e.EmergencyContact.Phone = *p.EmergencyContactPhone
	}
	if p.BankName != nil {
		e.BankAccount.BankName = *p.BankName
	}
	if p.BankAccountNumber != nil {
		e.BankAccount.AccountNumber = *p.BankAccountNumber
	}
	if p.BankAccountHolderName != nil {
		e.BankAccount.AccountHolderName = *p.BankAccountHolderName
	}
	e.UpdatedAt = time.Now().UTC()
	r.employees[p.ID] = e
	return e, nil
}

var _ SelfProfileRepository = (*selfRepo)(nil)

// fakeStore is a storage.Storage that re-implements the real allowlist/size
// policy so InitPhotoUpload's validation can be exercised without MinIO.
type fakeStore struct {
	maxBytes int64
	presigned bool // set true when PresignPut is reached (validation passed)
}

func (f *fakeStore) EnsureBucket(context.Context) error { return nil }

func (f *fakeStore) PresignPut(_ context.Context, ns storage.Namespace, employeeID, contentType string, declaredSize int64) (storage.UploadTicket, error) {
	allow := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}
	ext, ok := allow[contentType]
	if !ok {
		return storage.UploadTicket{}, storage.ErrContentTypeNotAllowed
	}
	if declaredSize > f.maxBytes {
		return storage.UploadTicket{}, storage.ErrFileTooLarge
	}
	f.presigned = true
	return storage.UploadTicket{
		UploadURL:   "https://store.local/put",
		ObjectKey:   string(ns) + "/" + employeeID + "/01ABC." + ext,
		ContentType: contentType,
		MaxBytes:    f.maxBytes,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}, nil
}

func (f *fakeStore) PresignGet(_ context.Context, objectKey string) (string, error) {
	return "https://store.local/get/" + objectKey, nil
}

var _ storage.Storage = (*fakeStore)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newSelfService(t *testing.T) (*SelfProfileService, *selfRepo, *fakeStore) {
	t.Helper()
	repo := newSelfRepo()
	store := &fakeStore{maxBytes: 5 << 20}
	svc := NewSelfProfileService(repo, store, crTxRunner{})
	return svc, repo, store
}

func agentCtx(empID string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "SWP-USR-" + empID, EmployeeID: empID, Role: auth.RoleAgent,
	})
}

// ---------------------------------------------------------------------------
// UpdateMyProfile — instant apply (address + app_language), audited, no notify
// ---------------------------------------------------------------------------

func TestUpdateMyProfile_InstantApply(t *testing.T) {
	svc, repo, _ := newSelfService(t)
	repo.employees["SWP-EMP-S1"] = domain.Employee{ID: "SWP-EMP-S1", AppLanguage: "id"}

	newAddr := "Jl. Baru No. 9"
	en := "en"
	updated, err := svc.UpdateMyProfile(agentCtx("SWP-EMP-S1"), SelfProfileInput{
		Address:     &newAddr,
		AppLanguage: &en,
	})
	if err != nil {
		t.Fatalf("UpdateMyProfile err = %v", err)
	}
	if updated.Address == nil || *updated.Address != newAddr {
		t.Errorf("address = %v, want %q", updated.Address, newAddr)
	}
	if updated.AppLanguage != "en" {
		t.Errorf("app_language = %q, want en", updated.AppLanguage)
	}
	// The instant patch reached the repo (audited write path executed).
	if repo.lastPatch == nil {
		t.Fatal("instant patch not applied to repo")
	}
}

// E11: phone + emergency_contact + bank_account apply INSTANTLY (no approval
// queue / no pending state). Asserts the repo patch carried every field and the
// phone was normalized to E.164.
func TestUpdateMyProfile_PhoneEmergencyBank_Instant(t *testing.T) {
	svc, repo, _ := newSelfService(t)
	repo.employees["SWP-EMP-S5"] = domain.Employee{ID: "SWP-EMP-S5", AppLanguage: "id"}

	phone := "0812-3456-7890"
	updated, err := svc.UpdateMyProfile(agentCtx("SWP-EMP-S5"), SelfProfileInput{
		Phone: &phone,
		EmergencyContact: &domain.EmergencyContact{Name: "Budi", Phone: "081299998888"},
		BankAccount: &domain.BankAccount{
			BankName: "BCA", AccountNumber: "1234567890", AccountHolderName: "Agent S5",
		},
	})
	if err != nil {
		t.Fatalf("UpdateMyProfile err = %v", err)
	}

	p := repo.lastPatch
	if p == nil {
		t.Fatal("instant patch not applied to repo")
	}
	if p.Phone == nil || *p.Phone != "+6281234567890" {
		t.Errorf("patch phone = %v, want normalized +6281234567890", p.Phone)
	}
	if p.EmergencyContactName == nil || *p.EmergencyContactName != "Budi" {
		t.Errorf("patch emergency name = %v, want Budi", p.EmergencyContactName)
	}
	if p.EmergencyContactPhone == nil || *p.EmergencyContactPhone != "081299998888" {
		t.Errorf("patch emergency phone = %v, want 081299998888", p.EmergencyContactPhone)
	}
	if p.BankName == nil || *p.BankName != "BCA" {
		t.Errorf("patch bank_name = %v, want BCA", p.BankName)
	}
	if p.BankAccountNumber == nil || *p.BankAccountNumber != "1234567890" {
		t.Errorf("patch account_number = %v, want 1234567890", p.BankAccountNumber)
	}
	if p.BankAccountHolderName == nil || *p.BankAccountHolderName != "Agent S5" {
		t.Errorf("patch account_holder_name = %v, want Agent S5", p.BankAccountHolderName)
	}
	// Returned employee reflects the applied values immediately (instant).
	if updated.Phone == nil || *updated.Phone != "+6281234567890" {
		t.Errorf("updated phone = %v, want +6281234567890", updated.Phone)
	}
	if updated.EmergencyContact.Name != "Budi" {
		t.Errorf("updated emergency name = %q, want Budi", updated.EmergencyContact.Name)
	}
	if updated.BankAccount.BankName != "BCA" {
		t.Errorf("updated bank_name = %q, want BCA", updated.BankAccount.BankName)
	}
}

// A phone already taken by another login → 409 CONFLICT, never an approval tier.
func TestUpdateMyProfile_PhoneTaken_Conflict(t *testing.T) {
	svc, repo, _ := newSelfService(t)
	repo.employees["SWP-EMP-S6"] = domain.Employee{ID: "SWP-EMP-S6"}
	repo.takenPhones["+6281234567890"] = true

	phone := "081234567890"
	_, err := svc.UpdateMyProfile(agentCtx("SWP-EMP-S6"), SelfProfileInput{Phone: &phone})
	if code := errCode(err); code != "CONFLICT" {
		t.Errorf("taken phone code = %q, want CONFLICT", code)
	}
	// No write reached the repo on the conflict path.
	if repo.lastPatch != nil {
		t.Errorf("expected no repo write on phone conflict, got %+v", repo.lastPatch)
	}
}

// An unchanged phone (same E.164 as the current value) is neither re-checked for
// uniqueness nor rewritten — confirms the guard only fires on an actual change.
func TestUpdateMyProfile_PhoneUnchanged_NoConflictCheck(t *testing.T) {
	svc, repo, _ := newSelfService(t)
	cur := "+6281234567890"
	repo.employees["SWP-EMP-S7"] = domain.Employee{ID: "SWP-EMP-S7", Phone: &cur}
	repo.takenPhones["+6281234567890"] = true // would 409 if the guard ran

	same := "081234567890" // normalizes to the current phone
	_, err := svc.UpdateMyProfile(agentCtx("SWP-EMP-S7"), SelfProfileInput{Phone: &same})
	if err != nil {
		t.Fatalf("unchanged phone err = %v", err)
	}
	if repo.lastPatch == nil || repo.lastPatch.Phone != nil {
		t.Errorf("patch phone = %v, want nil (unchanged → not rewritten)", repo.lastPatch)
	}
}

func TestUpdateMyProfile_BadLanguage_422(t *testing.T) {
	svc, repo, _ := newSelfService(t)
	repo.employees["SWP-EMP-S2"] = domain.Employee{ID: "SWP-EMP-S2"}
	bad := "fr"
	_, err := svc.UpdateMyProfile(agentCtx("SWP-EMP-S2"), SelfProfileInput{AppLanguage: &bad})
	if code := errCode(err); code != "INVALID_REQUEST" {
		t.Errorf("bad language code = %q, want INVALID_REQUEST", code)
	}
}

// A photo_object_key outside the caller's own namespace is rejected (no
// cross-employee key smuggling).
func TestUpdateMyProfile_ForeignPhotoKey_Rejected(t *testing.T) {
	svc, repo, _ := newSelfService(t)
	repo.employees["SWP-EMP-S3"] = domain.Employee{ID: "SWP-EMP-S3"}
	foreign := "profile-photos/SWP-EMP-OTHER/01XYZ.jpg"
	_, err := svc.UpdateMyProfile(agentCtx("SWP-EMP-S3"), SelfProfileInput{PhotoObjectKey: &foreign})
	if code := errCode(err); code != "FIELD_REQUIRES_APPROVAL" {
		t.Errorf("foreign key code = %q, want FIELD_REQUIRES_APPROVAL", code)
	}
}

func TestUpdateMyProfile_OwnPhotoKey_Applied(t *testing.T) {
	svc, repo, _ := newSelfService(t)
	repo.employees["SWP-EMP-S4"] = domain.Employee{ID: "SWP-EMP-S4"}
	own := "profile-photos/SWP-EMP-S4/01ABC.jpg"
	updated, err := svc.UpdateMyProfile(agentCtx("SWP-EMP-S4"), SelfProfileInput{PhotoObjectKey: &own})
	if err != nil {
		t.Fatalf("own photo key err = %v", err)
	}
	if updated.PhotoObjectKey == nil || *updated.PhotoObjectKey != own {
		t.Errorf("photo key = %v, want %q", updated.PhotoObjectKey, own)
	}
}

// GuardSelf: an agent without a resolved EmployeeID cannot self-edit.
func TestUpdateMyProfile_NoEmployeeID_Forbidden(t *testing.T) {
	svc, _, _ := newSelfService(t)
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "SWP-USR-X", Role: auth.RoleAgent, // EmployeeID empty
	})
	addr := "x"
	_, err := svc.UpdateMyProfile(ctx, SelfProfileInput{Address: &addr})
	if code := errCode(err); code != "FORBIDDEN" {
		t.Errorf("no-employee code = %q, want FORBIDDEN", code)
	}
}

// ---------------------------------------------------------------------------
// InitPhotoUpload — content-type / size validation + ticket
// ---------------------------------------------------------------------------

func TestInitPhotoUpload_ValidJpeg_Ticket(t *testing.T) {
	svc, _, store := newSelfService(t)
	ticket, err := svc.InitPhotoUpload(agentCtx("SWP-EMP-P1"), PhotoUploadInput{
		ContentType:   "image/jpeg",
		ContentLength: 1 << 20,
	})
	if err != nil {
		t.Fatalf("InitPhotoUpload err = %v", err)
	}
	if !store.presigned {
		t.Error("PresignPut not reached on a valid upload")
	}
	// Ticket key must live in the caller's own profile-photo namespace.
	wantPrefix := "profile-photos/SWP-EMP-P1/"
	if len(ticket.ObjectKey) < len(wantPrefix) || ticket.ObjectKey[:len(wantPrefix)] != wantPrefix {
		t.Errorf("object_key = %q, want prefix %q", ticket.ObjectKey, wantPrefix)
	}
	if ticket.ContentType != "image/jpeg" {
		t.Errorf("ticket content_type = %q, want image/jpeg", ticket.ContentType)
	}
}

func TestInitPhotoUpload_DisallowedType_422(t *testing.T) {
	svc, _, _ := newSelfService(t)
	_, err := svc.InitPhotoUpload(agentCtx("SWP-EMP-P2"), PhotoUploadInput{
		ContentType:   "application/pdf",
		ContentLength: 1024,
	})
	if code := errCode(err); code != "INVALID_REQUEST" {
		t.Errorf("bad content-type code = %q, want INVALID_REQUEST", code)
	}
}

func TestInitPhotoUpload_TooLarge_422(t *testing.T) {
	svc, _, store := newSelfService(t)
	_, err := svc.InitPhotoUpload(agentCtx("SWP-EMP-P3"), PhotoUploadInput{
		ContentType:   "image/png",
		ContentLength: store.maxBytes + 1,
	})
	if code := errCode(err); code != "UPLOAD_TOO_LARGE" {
		t.Errorf("oversize code = %q, want UPLOAD_TOO_LARGE", code)
	}
}

// ---------------------------------------------------------------------------
// PhotoURL — presigned GET passthrough
// ---------------------------------------------------------------------------

func TestPhotoURL_NilAndSet(t *testing.T) {
	svc, _, _ := newSelfService(t)
	if url := svc.PhotoURL(context.Background(), nil); url != "" {
		t.Errorf("PhotoURL(nil) = %q, want empty", url)
	}
	key := "profile-photos/SWP-EMP-P4/01ABC.jpg"
	if url := svc.PhotoURL(context.Background(), &key); url == "" {
		t.Error("PhotoURL(key) = empty, want a presigned GET url")
	}
}
