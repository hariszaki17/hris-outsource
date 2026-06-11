// Package people — white-box unit tests for SelfProfileService (E2 F2.1 agent
// self-service): the instant-tier apply (audited, no notify), the photo-upload
// content-type/size validation + ticket, and the GuardSelf cross-employee guard.
package people

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/storage"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// selfRepo is an in-memory SelfProfileRepository that records the instant patch.
type selfRepo struct {
	employees map[string]domain.Employee
	lastPatch *UpdateEmployeeSelfInstantParams
}

func newSelfRepo() *selfRepo { return &selfRepo{employees: map[string]domain.Employee{}} }

func (r *selfRepo) GetEmployeeByID(_ context.Context, id string) (domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	return e, nil
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
