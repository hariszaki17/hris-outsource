// Package attendance_test — HTTP contract tests for the clock-in/out selfie upload
// (F5.1 / CI-10). Mounts the REAL PhotoHandler + PhotoService over a fake
// PhotoRepository on a chi router with an agent principal, and asserts: a JPEG upload
// returns 201 + an SWP-FILE-* id, an oversize file → 413 FILE_TOO_LARGE, and a
// disallowed type → 415 UNSUPPORTED_MEDIA_TYPE.
package attendance_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	attendancehandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// fakePhotoRepo implements svc.PhotoRepository for the upload handler test.
type fakePhotoRepo struct {
	created *svc.CreateAttendancePhotoParams
}

func (f *fakePhotoRepo) CreateAttendancePhoto(_ context.Context, _ pgx.Tx, p svc.CreateAttendancePhotoParams) (svc.UploadedPhoto, error) {
	f.created = &p
	return svc.UploadedPhoto{
		ID:         "SWP-FILE-att-9182",
		FileName:   p.FileName,
		MIME:       p.MIME,
		SizeBytes:  p.SizeBytes,
		UploadedAt: time.Unix(0, 0).UTC(),
	}, nil
}

func (f *fakePhotoRepo) GetAttendancePhotoByID(_ context.Context, _ string) (domain.Attachment, error) {
	return domain.Attachment{}, domain.ErrNotFound
}

// photoHarness mounts PhotoHandler.UploadPhoto over the fake repo with an agent principal.
func photoHarness(repo *fakePhotoRepo) http.Handler {
	psvc := svc.NewPhotoService(repo, &fakeTxRunner{})
	h := attendancehandler.NewPhotoHandler(psvc)
	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), auth.Principal{
				UserID: "SWP-USR-1", EmployeeID: "SWP-EMP-0001", Role: auth.RoleAgent,
			})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/attendance:photo-upload", h.UploadPhoto)
	return r
}

// multipartPhoto builds a multipart body with one "file" part (given content-type +
// bytes) and an optional caption.
func multipartPhoto(t *testing.T, contentType string, content []byte, caption string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="selfie.jpg"`)
	hdr.Set("Content-Type", contentType)
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if caption != "" {
		_ = mw.WriteField("caption", caption)
	}
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func postPhoto(t *testing.T, h http.Handler, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/attendance:photo-upload", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestPhotoUploadHandler(t *testing.T) {
	t.Run("jpeg upload: 201 + SWP-FILE id", func(t *testing.T) {
		repo := &fakePhotoRepo{}
		body, ct := multipartPhoto(t, "image/jpeg", []byte("fake-jpeg-bytes"), "clock-in")
		rr := postPhoto(t, photoHarness(repo), body, ct)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
		}
		resp := decodeBody(t, rr)
		if id := strOf(resp["id"]); id != "SWP-FILE-att-9182" {
			t.Errorf("id = %q, want SWP-FILE-att-9182", id)
		}
		if mime := strOf(resp["mime"]); mime != "image/jpeg" {
			t.Errorf("mime = %q, want image/jpeg", mime)
		}
		if url := strOf(resp["url"]); url != "/api/v1/files/SWP-FILE-att-9182" {
			t.Errorf("url = %q, want /api/v1/files/SWP-FILE-att-9182", url)
		}
		if repo.created == nil {
			t.Fatal("no row was stored")
		}
		if repo.created.EmployeeID != "SWP-EMP-0001" {
			t.Errorf("stored employee_id = %q, want SWP-EMP-0001 (self)", repo.created.EmployeeID)
		}
		if repo.created.Caption != "clock-in" {
			t.Errorf("stored caption = %q, want clock-in", repo.created.Caption)
		}
	})

	t.Run("oversize file: 413 FILE_TOO_LARGE", func(t *testing.T) {
		repo := &fakePhotoRepo{}
		big := make([]byte, 10*1024*1024+1) // 10 MB + 1 byte
		body, ct := multipartPhoto(t, "image/jpeg", big, "")
		rr := postPhoto(t, photoHarness(repo), body, ct)

		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413 (body: %s)", rr.Code, rr.Body.String())
		}
		if code := strOf(errObject(t, decodeBody(t, rr))["code"]); code != "FILE_TOO_LARGE" {
			t.Errorf("error code = %q, want FILE_TOO_LARGE", code)
		}
		if repo.created != nil {
			t.Error("a row was stored, want none on oversize")
		}
	})

	t.Run("wrong type: 415 UNSUPPORTED_MEDIA_TYPE", func(t *testing.T) {
		repo := &fakePhotoRepo{}
		body, ct := multipartPhoto(t, "image/heic", []byte("fake-heic"), "")
		rr := postPhoto(t, photoHarness(repo), body, ct)

		if rr.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("status = %d, want 415 (body: %s)", rr.Code, rr.Body.String())
		}
		if code := strOf(errObject(t, decodeBody(t, rr))["code"]); code != "UNSUPPORTED_MEDIA_TYPE" {
			t.Errorf("error code = %q, want UNSUPPORTED_MEDIA_TYPE", code)
		}
		if repo.created != nil {
			t.Error("a row was stored, want none on bad type")
		}
	})
}
