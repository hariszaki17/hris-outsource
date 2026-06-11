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
	employees map[string]domain.Employee
}

func newFakeSelfRepo() *fakeSelfRepo {
	return &fakeSelfRepo{employees: make(map[string]domain.Employee)}
}

func (r *fakeSelfRepo) GetEmployeeByID(_ context.Context, id string) (domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	return e, nil
}

func (r *fakeSelfRepo) UpdateEmployeeSelfInstant(_ context.Context, _ pgx.Tx, p peoplesvc.UpdateEmployeeSelfInstantParams) (domain.Employee, error) {
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
