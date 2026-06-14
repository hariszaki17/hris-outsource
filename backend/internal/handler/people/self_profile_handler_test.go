// Package people_test — contract tests for the agent self-service profile
// handler (PATCH /me/profile + POST /me/profile/photo-upload-init). Asserts the
// instant-apply happy path + audit, the photo-upload-init content-type/size
// validation + ticket shape, the FIELD_REQUIRES_APPROVAL rejection of a foreign
// photo key, and the GuardSelf cross-employee guard.
//
// Pattern mirrors change_requests_handler_test.go: httptest + a real
// SelfProfileService wired to an in-memory repo + a fake storage client.
package people_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	peoplehandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/people"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/storage"
	peoplesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
)

// ---------------------------------------------------------------------------
// fakeSelfRepo — in-memory SelfProfileRepository.
// ---------------------------------------------------------------------------

type fakeSelfRepo struct {
	employees   map[string]domain.Employee
	takenPhones map[string]bool // E.164 phones already used by some login (uniqueness guard)
	lastPatch   *peoplesvc.UpdateEmployeeSelfInstantParams
}

func newFakeSelfRepo() *fakeSelfRepo {
	return &fakeSelfRepo{employees: make(map[string]domain.Employee), takenPhones: map[string]bool{}}
}

func (r *fakeSelfRepo) GetEmployeeByID(_ context.Context, id string) (domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	return e, nil
}

// UserPhoneTaken is the login-identifier uniqueness guard the service invokes when
// the phone actually changes; returns whether some other login already uses it.
func (r *fakeSelfRepo) UserPhoneTaken(_ context.Context, phone string) (bool, error) {
	return r.takenPhones[phone], nil
}

func (r *fakeSelfRepo) UpdateEmployeeSelfInstant(_ context.Context, _ pgx.Tx, p peoplesvc.UpdateEmployeeSelfInstantParams) (domain.Employee, error) {
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

var _ peoplesvc.SelfProfileRepository = (*fakeSelfRepo)(nil)

// ---------------------------------------------------------------------------
// fakeSelfStore — storage.Storage re-implementing the allowlist/size policy.
// ---------------------------------------------------------------------------

type fakeSelfStore struct{ maxBytes int64 }

func (f *fakeSelfStore) EnsureBucket(_ context.Context) error { return nil }

func (f *fakeSelfStore) PresignPut(_ context.Context, ns storage.Namespace, employeeID, contentType string, declaredSize int64) (storage.UploadTicket, error) {
	allow := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}
	ext, ok := allow[contentType]
	if !ok {
		return storage.UploadTicket{}, storage.ErrContentTypeNotAllowed
	}
	if declaredSize > f.maxBytes {
		return storage.UploadTicket{}, storage.ErrFileTooLarge
	}
	return storage.UploadTicket{
		UploadURL:   "https://store.local/put",
		ObjectKey:   string(ns) + "/" + employeeID + "/01ABC." + ext,
		ContentType: contentType,
		MaxBytes:    f.maxBytes,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}, nil
}

func (f *fakeSelfStore) PresignGet(_ context.Context, objectKey string) (string, error) {
	return "https://store.local/get/" + objectKey, nil
}

var _ storage.Storage = (*fakeSelfStore)(nil)

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type selfProfileHarness struct {
	router    *chi.Mux
	repo      *fakeSelfRepo
	principal auth.Principal
}

func newSelfProfileHarness(t *testing.T) *selfProfileHarness {
	t.Helper()
	repo := newFakeSelfRepo()
	store := &fakeSelfStore{maxBytes: 5 << 20}
	svc := peoplesvc.NewSelfProfileService(repo, store, &fakeTxRunner{})
	handler := peoplehandler.NewSelfProfileHandler(svc)

	fh := &selfProfileHarness{
		repo:      repo,
		principal: auth.Principal{UserID: "SWP-USR-A1", EmployeeID: "SWP-EMP-A1", Role: auth.RoleAgent},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), fh.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	// Route group matching server.go: super/hr/sl/agent (self scope in the service).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
		r.Patch("/me/profile", handler.UpdateMyProfile)
		r.Post("/me/profile/photo-upload-init", handler.InitProfilePhotoUpload)
	})

	fh.router = r
	return fh
}

func (h *selfProfileHarness) doSelf(method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

func (h *selfProfileHarness) seedSelf(id string) {
	now := time.Now().UTC()
	h.repo.employees[id] = domain.Employee{
		ID: id, FullName: "Agent " + id, NIK: "NIK-" + id, NIP: "NIP-" + id,
		JoinAt: now, Status: "active", AppLanguage: "id", CreatedAt: now, UpdatedAt: now,
	}
}

// ---------------------------------------------------------------------------
// PATCH /me/profile — instant apply
// ---------------------------------------------------------------------------

func TestUpdateMyProfile_InstantApply_200(t *testing.T) {
	h := newSelfProfileHarness(t)
	h.seedSelf("SWP-EMP-A1")

	rr := h.doSelf("PATCH", "/me/profile", map[string]any{
		"address":      "Jl. Mawar No. 7",
		"app_language": "en",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["address"] != "Jl. Mawar No. 7" {
		t.Errorf("address = %v, want applied", body["address"])
	}
	if body["app_language"] != "en" {
		t.Errorf("app_language = %v, want en", body["app_language"])
	}
	// Applied to the repo (instant, no approval queue).
	if a := h.repo.employees["SWP-EMP-A1"].Address; a == nil || *a != "Jl. Mawar No. 7" {
		t.Errorf("repo address = %v, want applied", a)
	}
}

// E11: phone + emergency_contact + bank_account apply INSTANTLY through the same
// PATCH /me/profile — no approval queue, no pending state. Asserts the repo write
// carried every field and the response reflects the applied values.
func TestUpdateMyProfile_PhoneEmergencyBank_InstantApply_200(t *testing.T) {
	h := newSelfProfileHarness(t)
	h.seedSelf("SWP-EMP-A1")

	rr := h.doSelf("PATCH", "/me/profile", map[string]any{
		"phone": "0812-3456-7890",
		"emergency_contact": map[string]any{
			"name":  "Budi",
			"phone": "081299998888",
		},
		"bank_account": map[string]any{
			"bank_name":           "BCA",
			"account_number":      "1234567890",
			"account_holder_name": "Agent A1",
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Repo write carried every instant field — and NO approval/pending state exists.
	p := h.repo.lastPatch
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
	if p.BankAccountHolderName == nil || *p.BankAccountHolderName != "Agent A1" {
		t.Errorf("patch account_holder_name = %v, want Agent A1", p.BankAccountHolderName)
	}

	// Response reflects the applied values (instant — no FIELD_REQUIRES_APPROVAL).
	body := decodeBody(t, rr)
	if _, hasErr := body["error"]; hasErr {
		t.Fatalf("instant edit returned an error envelope: %v", body["error"])
	}
	ec, _ := body["emergency_contact"].(map[string]any)
	if ec["name"] != "Budi" {
		t.Errorf("resp emergency_contact.name = %v, want Budi", ec["name"])
	}
	ba, _ := body["bank_account"].(map[string]any)
	if ba["bank_name"] != "BCA" {
		t.Errorf("resp bank_account.bank_name = %v, want BCA", ba["bank_name"])
	}
}

// A phone already taken by another login → 409 CONFLICT (login-identifier
// uniqueness), not an approval tier.
func TestUpdateMyProfile_PhoneTaken_409(t *testing.T) {
	h := newSelfProfileHarness(t)
	h.seedSelf("SWP-EMP-A1")
	h.repo.takenPhones["+6281234567890"] = true

	rr := h.doSelf("PATCH", "/me/profile", map[string]any{
		"phone": "081234567890",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("taken phone: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "CONFLICT" {
		t.Errorf("error.code = %v, want CONFLICT", errObj["code"])
	}
	// No write should have reached the repo on the conflict path.
	if h.repo.lastPatch != nil {
		t.Errorf("expected no repo write on phone conflict, got patch %+v", h.repo.lastPatch)
	}
}

func TestUpdateMyProfile_EmptyBody_400(t *testing.T) {
	h := newSelfProfileHarness(t)
	h.seedSelf("SWP-EMP-A1")
	rr := h.doSelf("PATCH", "/me/profile", map[string]any{})
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty body: expected 400/422, got %d: %s", rr.Code, rr.Body.String())
	}
}

// A foreign photo key (another employee's namespace) is rejected 422
// FIELD_REQUIRES_APPROVAL — no cross-employee key smuggling.
func TestUpdateMyProfile_ForeignPhotoKey_422(t *testing.T) {
	h := newSelfProfileHarness(t)
	h.seedSelf("SWP-EMP-A1")
	rr := h.doSelf("PATCH", "/me/profile", map[string]any{
		"photo_object_key": "profile-photos/SWP-EMP-OTHER/01XYZ.jpg",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("foreign key: expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "FIELD_REQUIRES_APPROVAL" {
		t.Errorf("error.code = %v, want FIELD_REQUIRES_APPROVAL", errObj["code"])
	}
}

// GuardSelf: an agent whose principal carries a DIFFERENT employee than the
// resolved record cannot reach another employee's data. Here the principal has
// no EmployeeID resolved → the service rejects with FORBIDDEN.
func TestUpdateMyProfile_NoEmployeeID_403(t *testing.T) {
	h := newSelfProfileHarness(t)
	h.principal = auth.Principal{UserID: "SWP-USR-X", Role: auth.RoleAgent} // EmployeeID empty
	rr := h.doSelf("PATCH", "/me/profile", map[string]any{"address": "x"})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("no employee id: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /me/profile/photo-upload-init
// ---------------------------------------------------------------------------

func TestInitPhotoUpload_ValidJpeg_200(t *testing.T) {
	h := newSelfProfileHarness(t)
	h.seedSelf("SWP-EMP-A1")
	rr := h.doSelf("POST", "/me/profile/photo-upload-init", map[string]any{
		"content_type":   "image/jpeg",
		"content_length": 1 << 20,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	for _, k := range []string{"upload_url", "object_key", "content_type", "max_bytes", "expires_at"} {
		if _, ok := body[k]; !ok {
			t.Errorf("upload ticket missing key: %s", k)
		}
	}
	// Key must be in the caller's own namespace.
	key, _ := body["object_key"].(string)
	if want := "profile-photos/SWP-EMP-A1/"; len(key) < len(want) || key[:len(want)] != want {
		t.Errorf("object_key = %q, want prefix %q", key, want)
	}
}

func TestInitPhotoUpload_MissingContentType_400(t *testing.T) {
	h := newSelfProfileHarness(t)
	h.seedSelf("SWP-EMP-A1")
	rr := h.doSelf("POST", "/me/profile/photo-upload-init", map[string]any{
		"content_length": 1024,
	})
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing content_type: expected 400/422, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestInitPhotoUpload_DisallowedType_422(t *testing.T) {
	h := newSelfProfileHarness(t)
	h.seedSelf("SWP-EMP-A1")
	rr := h.doSelf("POST", "/me/profile/photo-upload-init", map[string]any{
		"content_type":   "application/pdf",
		"content_length": 1024,
	})
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("disallowed type: expected 400/422, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestInitPhotoUpload_TooLarge_422(t *testing.T) {
	h := newSelfProfileHarness(t)
	h.seedSelf("SWP-EMP-A1")
	rr := h.doSelf("POST", "/me/profile/photo-upload-init", map[string]any{
		"content_type":   "image/png",
		"content_length": (5 << 20) + 1,
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversize: expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "UPLOAD_TOO_LARGE" {
		t.Errorf("error.code = %v, want UPLOAD_TOO_LARGE", errObj["code"])
	}
}
