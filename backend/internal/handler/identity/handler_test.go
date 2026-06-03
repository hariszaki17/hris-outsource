// Package identity_test contains contract tests for the auth handler endpoints.
// These tests assert the EXACT JSON field names and types that the OpenAPI spec
// requires for LoginResponse and MeResponse — the shapes the FE generated client
// consumes. They use httptest with a real Service wired to a fakeRepo (no DB).
package identity_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	identityhandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/identity"
	identitysvc "github.com/hariszaki17/hris-outsource/backend/internal/service/identity"
)

// ---------------------------------------------------------------------------
// Fake TxRunner
// ---------------------------------------------------------------------------

type fakeTxRunner struct{}

func (f *fakeTxRunner) InTx(_ context.Context, fn func(pgx.Tx) error) error { return fn(nil) }

// ---------------------------------------------------------------------------
// Fake repository (mirrors the one in service_test.go — same package boundary)
// ---------------------------------------------------------------------------

type fakeRepo struct {
	users         map[string]domain.User
	usersByID     map[string]domain.User
	refreshTokens map[string]domain.RefreshToken
	resetTokens   map[string]domain.PasswordResetToken
	resetNextID   int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:         make(map[string]domain.User),
		usersByID:     make(map[string]domain.User),
		refreshTokens: make(map[string]domain.RefreshToken),
		resetTokens:   make(map[string]domain.PasswordResetToken),
	}
}

func toLower(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + ('a' - 'A')
		}
	}
	return string(out)
}

func (f *fakeRepo) addUser(u domain.User) {
	f.users[toLower(u.Email)] = u
	f.usersByID[u.ID] = u
}

func (f *fakeRepo) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	u, ok := f.users[toLower(email)]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) GetUserByID(_ context.Context, id string) (domain.User, error) {
	u, ok := f.usersByID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) GetRefreshTokenByHash(_ context.Context, hash string) (domain.RefreshToken, error) {
	t, ok := f.refreshTokens[hash]
	if !ok {
		return domain.RefreshToken{}, domain.ErrNotFound
	}
	return t, nil
}

func (f *fakeRepo) InsertRefreshToken(_ context.Context, _ pgx.Tx, p identitysvc.NewRefreshToken) (domain.RefreshToken, error) {
	t := domain.RefreshToken{ID: 1, UserID: p.UserID, TokenHash: p.TokenHash, FamilyID: p.FamilyID, ExpiresAt: p.ExpiresAt}
	f.refreshTokens[p.TokenHash] = t
	return t, nil
}

func (f *fakeRepo) RevokeRefreshToken(_ context.Context, _ pgx.Tx, _ int64) error  { return nil }
func (f *fakeRepo) RevokeFamily(_ context.Context, _ pgx.Tx, _ string) error       { return nil }
func (f *fakeRepo) RevokeAllRefreshForUser(_ context.Context, _ pgx.Tx, _ string) error { return nil }
func (f *fakeRepo) SetLastLogin(_ context.Context, _ pgx.Tx, _ string) error       { return nil }
func (f *fakeRepo) UpdatePassword(_ context.Context, _ pgx.Tx, _, _ string) error  { return nil }
func (f *fakeRepo) MarkResetTokenUsed(_ context.Context, _ pgx.Tx, _ int64) error  { return nil }

func (f *fakeRepo) InsertResetToken(_ context.Context, _ pgx.Tx, userID, tokenHash string, expiresAt time.Time) (domain.PasswordResetToken, error) {
	f.resetNextID++
	t := domain.PasswordResetToken{ID: f.resetNextID, UserID: userID, TokenHash: tokenHash, ExpiresAt: expiresAt}
	f.resetTokens[tokenHash] = t
	return t, nil
}

func (f *fakeRepo) GetResetTokenByHash(_ context.Context, hash string) (domain.PasswordResetToken, error) {
	t, ok := f.resetTokens[hash]
	if !ok {
		return domain.PasswordResetToken{}, domain.ErrNotFound
	}
	return t, nil
}

var _ identitysvc.Repository = (*fakeRepo)(nil)

// ---------------------------------------------------------------------------
// Test harness setup
// ---------------------------------------------------------------------------

const (
	testAccessTTL = 30 * time.Minute
)

type testHarness struct {
	router  *chi.Mux
	repo    *fakeRepo
	issuer  *auth.Issuer
	handler *identityhandler.Handler
}

func newHarness(t *testing.T) *testHarness {
	t.Helper()
	privB64, pubB64, err := auth.GenerateKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	issuer, err := auth.NewIssuer(privB64, pubB64, testAccessTTL)
	if err != nil {
		t.Fatalf("new issuer: %v", err)
	}
	authn := auth.NewAuthenticator(issuer)

	repo := newFakeRepo()
	svc := identitysvc.NewService(repo, &fakeTxRunner{}, issuer, 7*24*time.Hour)

	h := identityhandler.NewHandler(svc, identityhandler.CookieConfig{}, testAccessTTL)

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	// Public routes
	r.Post("/auth/login", h.Login)
	r.Post("/auth/refresh", h.Refresh)
	r.Post("/auth/logout", h.Logout)
	r.Post("/auth/forgot-password", h.ForgotPassword)
	r.Post("/auth/reset-password", h.ResetPassword)
	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(authn.Require)
		r.Get("/auth/me", h.Me)
	})

	return &testHarness{router: r, repo: repo, issuer: issuer, handler: h}
}

// doJSON makes a JSON request and returns the response recorder.
func (h *testHarness) doJSON(method, path string, body any) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// doJSONWithToken makes a JSON request with a Bearer token.
func (h *testHarness) doJSONWithToken(method, path, token string, body any) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// addActiveUser registers a hashed user in the fake repo.
func (h *testHarness) addActiveUser(id, email, password, role, fullName, companyID string) {
	hash, _ := auth.HashPassword(password)
	var cid string
	if companyID != "" {
		cid = companyID
	}
	h.repo.addUser(domain.User{
		ID:           id,
		Email:        email,
		PasswordHash: hash,
		Role:         auth.Role(role),
		EmployeeID:   fmt.Sprintf("SWP-EMP-%s", strings.TrimPrefix(id, "SWP-USR-")),
		CompanyID:    cid,
		Status:       "active",
		FullName:     fullName,
	})
}

// ---------------------------------------------------------------------------
// Helper: decode JSON body
// ---------------------------------------------------------------------------

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, rr.Body.String())
	}
	return m
}

// ---------------------------------------------------------------------------
// Login contract tests
// ---------------------------------------------------------------------------

func TestLogin_SpecShape(t *testing.T) {
	h := newHarness(t)
	h.addActiveUser("SWP-USR-1042", "sari.hadi@swp.test", "Pass1ng-Garuda!", "hr_admin", "Sari Hadi", "")

	rr := h.doJSON("POST", "/auth/login", map[string]any{
		"email":    "sari.hadi@swp.test",
		"password": "Pass1ng-Garuda!",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Required top-level fields per LoginResponse spec.
	if _, ok := body["access_token"]; !ok {
		t.Error("missing access_token")
	}
	if tt, ok := body["token_type"]; !ok || tt != "Bearer" {
		t.Errorf("token_type = %v, want Bearer", body["token_type"])
	}
	ei, ok := body["expires_in"]
	if !ok {
		t.Fatal("missing expires_in")
	}
	// JSON numbers decode as float64; expires_in must be a positive number.
	expiresIn, ok := ei.(float64)
	if !ok {
		t.Fatalf("expires_in is not a number, got %T: %v", ei, ei)
	}
	if expiresIn <= 0 {
		t.Errorf("expires_in = %v, want > 0", expiresIn)
	}
	// It should equal 1800 (30 min).
	if expiresIn != 1800 {
		t.Errorf("expires_in = %v, want 1800", expiresIn)
	}

	// User sub-object.
	userRaw, ok := body["user"]
	if !ok {
		t.Fatal("missing user field")
	}
	user, ok := userRaw.(map[string]any)
	if !ok {
		t.Fatalf("user is not an object, got %T", userRaw)
	}
	if user["email"] != "sari.hadi@swp.test" {
		t.Errorf("user.email = %v, want sari.hadi@swp.test", user["email"])
	}
	if user["status"] != "ACTIVE" {
		t.Errorf("user.status = %v, want ACTIVE (uppercase)", user["status"])
	}
	if user["role"] != "hr_admin" {
		t.Errorf("user.role = %v, want hr_admin", user["role"])
	}
	if user["full_name"] != "Sari Hadi" {
		t.Errorf("user.full_name = %v, want Sari Hadi", user["full_name"])
	}
	// scope
	scopeRaw, ok := user["scope"]
	if !ok {
		t.Fatal("missing user.scope")
	}
	scope, ok := scopeRaw.(map[string]any)
	if !ok {
		t.Fatalf("user.scope is not an object, got %T", scopeRaw)
	}
	if scope["type"] != "global" {
		t.Errorf("scope.type = %v, want global (for hr_admin)", scope["type"])
	}
	if scope["company_id"] != nil {
		t.Errorf("scope.company_id = %v, want null (for hr_admin)", scope["company_id"])
	}
}

func TestLogin_WrongPassword_401(t *testing.T) {
	h := newHarness(t)
	h.addActiveUser("SWP-USR-1042", "sari.hadi@swp.test", "Pass1ng-Garuda!", "hr_admin", "Sari Hadi", "")

	rr := h.doJSON("POST", "/auth/login", map[string]any{
		"email":    "sari.hadi@swp.test",
		"password": "wrongpassword",
	})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "INVALID_CREDENTIALS" {
		t.Errorf("error.code = %v, want INVALID_CREDENTIALS", errObj["code"])
	}
}

func TestLogin_DisabledAccount_403(t *testing.T) {
	h := newHarness(t)
	hash, _ := auth.HashPassword("Pass1ng-Garuda!")
	h.repo.addUser(domain.User{
		ID:           "SWP-USR-9999",
		Email:        "disabled@swp.test",
		PasswordHash: hash,
		Role:         auth.RoleAgent,
		Status:       "disabled",
	})

	rr := h.doJSON("POST", "/auth/login", map[string]any{
		"email":    "disabled@swp.test",
		"password": "Pass1ng-Garuda!",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "ACCOUNT_DISABLED" {
		t.Errorf("error.code = %v, want ACCOUNT_DISABLED", errObj["code"])
	}
}

// ---------------------------------------------------------------------------
// /auth/me contract tests
// ---------------------------------------------------------------------------

func TestMe_SpecShape(t *testing.T) {
	h := newHarness(t)
	h.addActiveUser("SWP-USR-1042", "sari.hadi@swp.test", "Pass1ng-Garuda!", "hr_admin", "Sari Hadi", "")

	// Login to get access token.
	loginRR := h.doJSON("POST", "/auth/login", map[string]any{
		"email":    "sari.hadi@swp.test",
		"password": "Pass1ng-Garuda!",
	})
	if loginRR.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", loginRR.Code, loginRR.Body.String())
	}
	var loginBody map[string]any
	json.NewDecoder(loginRR.Body).Decode(&loginBody)
	accessToken := loginBody["access_token"].(string)

	// GET /auth/me with the token.
	rr := h.doJSONWithToken("GET", "/auth/me", accessToken, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Assert MeResponse shape.
	assertStr := func(field string, want string) {
		t.Helper()
		if body[field] != want {
			t.Errorf("%s = %v, want %q", field, body[field], want)
		}
	}
	assertStr("email", "sari.hadi@swp.test")
	assertStr("role", "hr_admin")
	assertStr("status", "ACTIVE")
	assertStr("full_name", "Sari Hadi")

	if _, ok := body["id"]; !ok {
		t.Error("missing id field")
	}
	if _, ok := body["employee_id"]; !ok {
		t.Error("missing employee_id field")
	}

	scopeRaw, ok := body["scope"]
	if !ok {
		t.Fatal("missing scope")
	}
	scope := scopeRaw.(map[string]any)
	if scope["type"] != "global" {
		t.Errorf("scope.type = %v, want global", scope["type"])
	}
}

func TestMe_ShiftLeader_CompanyScope(t *testing.T) {
	h := newHarness(t)
	h.addActiveUser("SWP-USR-1108", "rudi.wijaya@swp.test", "Lead3r-Senayan!", "shift_leader", "Rudi Wijaya", "SWP-CMP-0021")

	loginRR := h.doJSON("POST", "/auth/login", map[string]any{
		"email":    "rudi.wijaya@swp.test",
		"password": "Lead3r-Senayan!",
	})
	if loginRR.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", loginRR.Code, loginRR.Body.String())
	}
	var loginBody map[string]any
	json.NewDecoder(loginRR.Body).Decode(&loginBody)
	accessToken := loginBody["access_token"].(string)

	rr := h.doJSONWithToken("GET", "/auth/me", accessToken, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	scope := body["scope"].(map[string]any)
	if scope["type"] != "company" {
		t.Errorf("scope.type = %v, want company (shift_leader)", scope["type"])
	}
	if scope["company_id"] != "SWP-CMP-0021" {
		t.Errorf("scope.company_id = %v, want SWP-CMP-0021", scope["company_id"])
	}
}

// ---------------------------------------------------------------------------
// /auth/forgot-password contract tests
// ---------------------------------------------------------------------------

func TestForgotPassword_KnownEmail_202(t *testing.T) {
	h := newHarness(t)
	h.addActiveUser("SWP-USR-1042", "sari.hadi@swp.test", "Pass1ng-Garuda!", "hr_admin", "Sari Hadi", "")

	rr := h.doJSON("POST", "/auth/forgot-password", map[string]any{
		"email": "sari.hadi@swp.test",
	})
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if _, ok := body["message"]; !ok {
		t.Error("missing message field")
	}
}

func TestForgotPassword_UnknownEmail_202_SameBody(t *testing.T) {
	h := newHarness(t)
	// No users registered — unknown email.

	rrKnown := h.doJSON("POST", "/auth/forgot-password", map[string]any{
		"email": "known@swp.test",
	})
	rrUnknown := h.doJSON("POST", "/auth/forgot-password", map[string]any{
		"email": "nobody@swp.test",
	})

	// Both must be 202.
	if rrKnown.Code != http.StatusAccepted {
		t.Errorf("known email: expected 202, got %d", rrKnown.Code)
	}
	if rrUnknown.Code != http.StatusAccepted {
		t.Errorf("unknown email: expected 202, got %d", rrUnknown.Code)
	}

	// Both must have the same body (anti-enumeration C-2).
	var bKnown, bUnknown map[string]any
	json.NewDecoder(rrKnown.Body).Decode(&bKnown)
	json.NewDecoder(rrUnknown.Body).Decode(&bUnknown)
	if bKnown["message"] != bUnknown["message"] {
		t.Errorf("anti-enumeration violated: known=%q vs unknown=%q", bKnown["message"], bUnknown["message"])
	}
}

func TestForgotPassword_MissingEmail_400(t *testing.T) {
	h := newHarness(t)
	rr := h.doJSON("POST", "/auth/forgot-password", map[string]any{
		"email": "",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// /auth/reset-password contract tests
// ---------------------------------------------------------------------------

func TestResetPassword_Valid_204(t *testing.T) {
	h := newHarness(t)
	h.addActiveUser("SWP-USR-1042", "sari@swp.test", "Pass1ng-Garuda!", "hr_admin", "Sari Hadi", "")

	// First get a reset token via forgot-password.
	h.doJSON("POST", "/auth/forgot-password", map[string]any{"email": "sari@swp.test"})

	// The fake repo has one token now — get its hash from the store and
	// reconstruct the plaintext: we can't do this from the test because the
	// plaintext is internal. Instead, test the service path directly via
	// inserting a known token into the fake store.
	plaintext, hash := auth.NewRefreshToken()
	h.repo.resetTokens[hash] = domain.PasswordResetToken{
		ID:        999,
		UserID:    "SWP-USR-1042",
		TokenHash: hash,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	rr := h.doJSON("POST", "/auth/reset-password", map[string]any{
		"reset_token":  plaintext,
		"new_password": "NewP@ss1234",
	})
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestResetPassword_ExpiredToken_401(t *testing.T) {
	h := newHarness(t)
	plaintext, hash := auth.NewRefreshToken()
	pastTime := time.Now().Add(-2 * time.Hour)
	h.repo.resetTokens[hash] = domain.PasswordResetToken{
		ID:        1,
		UserID:    "SWP-USR-1042",
		TokenHash: hash,
		ExpiresAt: pastTime,
	}

	rr := h.doJSON("POST", "/auth/reset-password", map[string]any{
		"reset_token":  plaintext,
		"new_password": "ValidPass1ng!",
	})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "RESET_TOKEN_EXPIRED" {
		t.Errorf("error.code = %v, want RESET_TOKEN_EXPIRED", errObj["code"])
	}
}

func TestResetPassword_WeakPassword_422(t *testing.T) {
	h := newHarness(t)
	plaintext, hash := auth.NewRefreshToken()
	h.repo.resetTokens[hash] = domain.PasswordResetToken{
		ID:        1,
		UserID:    "SWP-USR-1042",
		TokenHash: hash,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	rr := h.doJSON("POST", "/auth/reset-password", map[string]any{
		"reset_token":  plaintext,
		"new_password": "weak",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "WEAK_PASSWORD" {
		t.Errorf("error.code = %v, want WEAK_PASSWORD", errObj["code"])
	}
	// fields.new_password must be present (per spec example).
	fields, ok := errObj["fields"].(map[string]any)
	if !ok {
		t.Fatal("error.fields missing")
	}
	if _, ok := fields["new_password"]; !ok {
		t.Error("error.fields.new_password missing")
	}
}

// ---------------------------------------------------------------------------
// Refresh + Logout contract tests
// ---------------------------------------------------------------------------

func TestRefresh_ValidToken_200(t *testing.T) {
	h := newHarness(t)
	h.addActiveUser("SWP-USR-1042", "sari@swp.test", "Pass1ng-Garuda!", "hr_admin", "Sari Hadi", "")

	// Login with bearer transport to get refresh token in body.
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(`{"email":"sari@swp.test","password":"Pass1ng-Garuda!"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Transport", "bearer")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rr.Code, rr.Body.String())
	}
	var loginBody map[string]any
	json.NewDecoder(rr.Body).Decode(&loginBody)
	refreshToken := loginBody["refresh_token"].(string)

	// Now refresh.
	rr2 := h.doJSON("POST", "/auth/refresh", map[string]any{
		"refresh_token": refreshToken,
	})
	if rr2.Code != http.StatusOK {
		t.Fatalf("refresh failed: %d %s", rr2.Code, rr2.Body.String())
	}
	body := decodeBody(t, rr2)
	if _, ok := body["access_token"]; !ok {
		t.Error("refresh response missing access_token")
	}
	if body["token_type"] != "Bearer" {
		t.Errorf("refresh token_type = %v, want Bearer", body["token_type"])
	}
	if _, ok := body["expires_in"]; !ok {
		t.Error("refresh response missing expires_in")
	}
}

func TestLogout_204(t *testing.T) {
	h := newHarness(t)
	h.addActiveUser("SWP-USR-1042", "sari@swp.test", "Pass1ng-Garuda!", "hr_admin", "Sari Hadi", "")

	// Login with bearer transport.
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(`{"email":"sari@swp.test","password":"Pass1ng-Garuda!"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Transport", "bearer")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	var loginBody map[string]any
	json.NewDecoder(rr.Body).Decode(&loginBody)
	refreshToken := loginBody["refresh_token"].(string)

	// Logout.
	rr2 := h.doJSON("POST", "/auth/logout", map[string]any{
		"refresh_token": refreshToken,
	})
	if rr2.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr2.Code, rr2.Body.String())
	}
}
