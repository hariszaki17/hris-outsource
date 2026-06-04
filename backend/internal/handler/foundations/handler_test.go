// Package foundations_test contains contract tests for the E1 foundations handler
// endpoints. These tests assert the EXACT JSON field names, types, and status codes
// required by the OpenAPI spec — the drift gate that replaces server-side codegen.
//
// Pattern: httptest + real Service wired to an in-memory fakeRepo (no DB).
// Principal injection via auth.WithPrincipal on the request context.
package foundations_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	foundationshandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/foundations"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	foundationssvc "github.com/hariszaki17/hris-outsource/backend/internal/service/foundations"
)

// ---------------------------------------------------------------------------
// Fake pgx.Tx — only Exec is needed (for audit.Record); all other methods panic.
// ---------------------------------------------------------------------------

type fakeTx struct{}

func (f *fakeTx) Begin(_ context.Context) (pgx.Tx, error) { return f, nil }
func (f *fakeTx) Commit(_ context.Context) error          { return nil }
func (f *fakeTx) Rollback(_ context.Context) error        { return nil }
func (f *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	panic("fakeTx: Query not implemented")
}
func (f *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("fakeTx: QueryRow not implemented")
}
func (f *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	panic("fakeTx: CopyFrom not implemented")
}
func (f *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults {
	panic("fakeTx: SendBatch not implemented")
}
func (f *fakeTx) LargeObjects() pgx.LargeObjects {
	panic("fakeTx: LargeObjects not implemented")
}
func (f *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	panic("fakeTx: Prepare not implemented")
}
func (f *fakeTx) Conn() *pgx.Conn { return nil }

var _ pgx.Tx = (*fakeTx)(nil)

// ---------------------------------------------------------------------------
// Fake TxRunner — passes a real fakeTx so audit.Record can call Exec.
// ---------------------------------------------------------------------------

type fakeTxRunner struct{}

func (f *fakeTxRunner) InTx(_ context.Context, fn func(pgx.Tx) error) error {
	return fn(&fakeTx{})
}

// ---------------------------------------------------------------------------
// Fake repository — in-memory, implements foundationssvc.Repository
// ---------------------------------------------------------------------------

type fakeFoundationsRepo struct {
	users        map[string]domain.User   // keyed by ID
	usersByEmail map[string]domain.User   // keyed by lowercase email
	auditEntries []domain.AuditEntry      // ordered slice; append-on-create
	settings     []domain.PlatformSetting // keyed by Key inside slice
	resetTokens  []resetTokenRow
	nextAuditIdx int
}

type resetTokenRow struct {
	userID    string
	tokenHash string
	expiresAt time.Time
}

func newFakeFoundationsRepo() *fakeFoundationsRepo {
	return &fakeFoundationsRepo{
		users:        make(map[string]domain.User),
		usersByEmail: make(map[string]domain.User),
	}
}

func (r *fakeFoundationsRepo) addUser(u domain.User) {
	r.users[u.ID] = u
	r.usersByEmail[lowerEmail(u.Email)] = u
}

func lowerEmail(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

// --- Repository interface implementation ---

func (r *fakeFoundationsRepo) ListUsers(_ context.Context, f domain.UserFilter) ([]domain.User, error) {
	// Collect into a slice, sorted by CreatedAt asc then ID for stable ordering.
	var all []domain.User
	for _, u := range r.users {
		// status filter
		if f.Status != nil && u.Status != *f.Status {
			continue
		}
		// role filter
		if f.Role != nil && string(u.Role) != *f.Role {
			continue
		}
		// cursor: only rows where (created_at, id) > cursor
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if u.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if u.CreatedAt.Equal(*f.CursorCreatedAt) && u.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, u)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID < all[j].ID
		}
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	limit := f.Limit
	if limit > 0 && len(all) > limit {
		return all[:limit], nil
	}
	return all, nil
}

func (r *fakeFoundationsRepo) GetUserByID(_ context.Context, id string) (domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (r *fakeFoundationsRepo) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	u, ok := r.usersByEmail[lowerEmail(email)]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (r *fakeFoundationsRepo) ListAuditLog(_ context.Context, f domain.AuditFilter) ([]domain.AuditEntry, error) {
	var all []domain.AuditEntry
	for _, e := range r.auditEntries {
		// entity_type filter
		if f.EntityType != nil && e.EntityType != *f.EntityType {
			continue
		}
		if f.ActorUserID != nil && (e.ActorUserID == nil || *e.ActorUserID != *f.ActorUserID) {
			continue
		}
		if f.Action != nil && e.Action != *f.Action {
			continue
		}
		// cursor
		if f.CursorCreatedAt != nil && f.CursorID != nil {
			if e.CreatedAt.Before(*f.CursorCreatedAt) {
				continue
			}
			if e.CreatedAt.Equal(*f.CursorCreatedAt) && e.ID <= *f.CursorID {
				continue
			}
		}
		all = append(all, e)
	}
	// Sort by created_at asc then id asc — same as the SQL ORDER BY.
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID < all[j].ID
		}
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	limit := f.Limit
	if limit > 0 && len(all) > limit {
		return all[:limit], nil
	}
	return all, nil
}

func (r *fakeFoundationsRepo) GetAuditLogByID(_ context.Context, id string) (domain.AuditEntry, error) {
	for _, e := range r.auditEntries {
		if e.ID == id {
			return e, nil
		}
	}
	return domain.AuditEntry{}, domain.ErrNotFound
}

func (r *fakeFoundationsRepo) ListPlatformSettings(_ context.Context) ([]domain.PlatformSetting, error) {
	return r.settings, nil
}

func (r *fakeFoundationsRepo) CreateUser(_ context.Context, _ pgx.Tx, p foundationssvc.CreateUserParams) (domain.User, error) {
	id := "SWP-USR-" + time.Now().Format("150405000")
	now := time.Now().UTC()
	u := domain.User{
		ID:           id,
		Email:        p.Email,
		PasswordHash: p.PasswordHash,
		Role:         auth.Role(p.Role),
		FullName:     p.FullName,
		EmployeeID:   p.EmployeeID,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	r.addUser(u)
	return u, nil
}

func (r *fakeFoundationsRepo) UpdateUserEmail(_ context.Context, _ pgx.Tx, id, email string) (domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	delete(r.usersByEmail, lowerEmail(u.Email))
	u.Email = email
	u.UpdatedAt = time.Now().UTC()
	r.users[id] = u
	r.usersByEmail[lowerEmail(email)] = u
	return u, nil
}

func (r *fakeFoundationsRepo) ChangeUserRole(_ context.Context, _ pgx.Tx, id, role string) (domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	u.Role = auth.Role(role)
	u.UpdatedAt = time.Now().UTC()
	r.users[id] = u
	r.usersByEmail[lowerEmail(u.Email)] = u
	return u, nil
}

func (r *fakeFoundationsRepo) SetUserStatus(_ context.Context, _ pgx.Tx, id, status string) (domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	u.Status = status
	u.UpdatedAt = time.Now().UTC()
	r.users[id] = u
	r.usersByEmail[lowerEmail(u.Email)] = u
	return u, nil
}

func (r *fakeFoundationsRepo) InsertResetToken(_ context.Context, _ pgx.Tx, userID, tokenHash string, expiresAt time.Time) error {
	r.resetTokens = append(r.resetTokens, resetTokenRow{userID: userID, tokenHash: tokenHash, expiresAt: expiresAt})
	return nil
}

// Compile-time interface check.
var _ foundationssvc.Repository = (*fakeFoundationsRepo)(nil)

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

// principalMiddleware injects an auth.Principal into every request context.
func principalMiddleware(p auth.Principal) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type foundationsHarness struct {
	router *chi.Mux
	repo   *fakeFoundationsRepo
	// principal is mutable per-test via setPrincipal.
	principal auth.Principal
}

// newHarness creates a harness with the RBAC role-check active.
// The injected principal is set to hr_admin by default; swap via h.principal before a request.
func newHarness(t *testing.T) *foundationsHarness {
	t.Helper()
	repo := newFakeFoundationsRepo()
	svc := foundationssvc.NewService(repo, &fakeTxRunner{})

	h := foundationshandler.NewHandler(svc)

	fh := &foundationsHarness{
		repo:      repo,
		principal: auth.Principal{UserID: "SWP-USR-0001", Role: auth.RoleHRAdmin},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	// Dynamic principal injection — reads fh.principal per request.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), fh.principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	// RBAC guard — mirrors server.go
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Get("/users", h.ListUsers)
		r.Post("/users", h.CreateUser)
		r.Patch("/users/{user_id}", h.UpdateUser)
		r.Post("/users/{user_id}:change-role", h.ChangeUserRole)
		r.Post("/users/{user_id}:deactivate", h.DeactivateUser)
		r.Post("/users/{user_id}:reactivate", h.ReactivateUser)
		r.Post("/users/{user_id}:send-password-reset", h.SendUserPasswordReset)
		r.Get("/audit-log", h.ListAuditLog)
		r.Get("/audit-log/{audit_log_id}", h.GetAuditLogEntry)
		r.Get("/platform/settings", h.GetPlatformSettings)
	})

	fh.router = r
	return fh
}

// do sends a request and returns the recorder.
func (h *foundationsHarness) do(method, path string, body any) *httptest.ResponseRecorder {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// decodeBody decodes the JSON body into a map.
func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, rr.Body.String())
	}
	return m
}

// seedUsers adds n users to the fake repo with sequential created_at values.
func (h *foundationsHarness) seedUsers(n int) []domain.User {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	users := make([]domain.User, n)
	for i := 0; i < n; i++ {
		id := "SWP-USR-" + zpad(i+1)
		u := domain.User{
			ID:        id,
			Email:     "user" + zpad(i+1) + "@swp.test",
			Role:      auth.RoleAgent,
			Status:    "active",
			FullName:  "User " + zpad(i+1),
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
			UpdatedAt: base.Add(time.Duration(i) * time.Minute),
		}
		h.repo.addUser(u)
		users[i] = u
	}
	return users
}

func zpad(n int) string {
	s := "00000"
	ns := itoa(n)
	if len(ns) >= len(s) {
		return ns
	}
	return s[len(ns):] + ns
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// seedAuditEntries adds n audit entries to the fake repo.
func (h *foundationsHarness) seedAuditEntries(n int) []domain.AuditEntry {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	entries := make([]domain.AuditEntry, n)
	for i := 0; i < n; i++ {
		actorID := "SWP-USR-00001"
		actorRole := "hr_admin"
		reqID := "req_" + zpad(i+1)
		entityType := "user"
		if i%2 == 1 {
			entityType = "placement"
		}
		e := domain.AuditEntry{
			ID:          "SWP-AL-" + zpad(i+1),
			ActorUserID: &actorID,
			ActorRole:   &actorRole,
			Action:      "CREATE",
			EntityType:  entityType,
			EntityID:    "SWP-USR-" + zpad(i+1),
			Before:      nil,
			After:       map[string]any{"email": "user" + zpad(i+1) + "@swp.test"},
			RequestID:   &reqID,
			CreatedAt:   base.Add(time.Duration(i) * time.Minute),
		}
		h.repo.auditEntries = append(h.repo.auditEntries, e)
		entries[i] = e
	}
	return entries
}

// seedPlatformSettings seeds the standard 7 keys (matching the spec).
func (h *foundationsHarness) seedPlatformSettings() {
	h.repo.settings = []domain.PlatformSetting{
		{Key: "locale", Value: "id-ID", Label: "Locale", Locked: true},
		{Key: "timezone", Value: "Asia/Jakarta", Label: "Timezone", Locked: true},
		{Key: "date_format", Value: "DD/MM/YYYY", Label: "Date Format", Locked: false},
		{Key: "currency", Value: "IDR", Label: "Currency", Locked: true},
		{Key: "version", Value: "2.0.0", Label: "Version", Locked: true},
		{Key: "stack", Value: "Go/React/Postgres", Label: "Stack", Locked: true},
		{Key: "legacy_data_source", Value: "lumen_swp (MySQL)", Label: "Legacy Data Source", Locked: true},
	}
}

// ---------------------------------------------------------------------------
// Task 1: Users management + RBAC
// ---------------------------------------------------------------------------

func TestListUsers_ShapeAndEnvelope(t *testing.T) {
	h := newHarness(t)
	h.seedUsers(3)

	rr := h.do("GET", "/users", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Envelope keys must be present.
	if _, ok := body["data"]; !ok {
		t.Error("missing key: data")
	}
	if _, ok := body["next_cursor"]; !ok {
		t.Error("missing key: next_cursor")
	}
	if _, ok := body["has_more"]; !ok {
		t.Error("missing key: has_more")
	}

	data, ok := body["data"].([]any)
	if !ok || len(data) == 0 {
		t.Fatalf("data is not a non-empty array: %T %v", body["data"], body["data"])
	}

	// Assert first item has all expected User keys.
	first := data[0].(map[string]any)
	requiredKeys := []string{
		"id", "email", "role", "status",
		"employee_id", "full_name",
		"company_id", "company_name",
		"last_login_at", "created_at", "updated_at",
	}
	for _, k := range requiredKeys {
		if _, ok := first[k]; !ok {
			t.Errorf("data[0] missing key: %s", k)
		}
	}

	// status must be UPPERCASE.
	if first["status"] != "ACTIVE" {
		t.Errorf("data[0].status = %v, want ACTIVE (uppercase)", first["status"])
	}
}

func TestListUsers_CursorAdvances(t *testing.T) {
	h := newHarness(t)
	h.seedUsers(5) // seed 5 users, fetch 2 at a time

	// First page: limit=2
	rr := h.do("GET", "/users?limit=2", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("page1 expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body1 := decodeBody(t, rr)

	hasMore, _ := body1["has_more"].(bool)
	if !hasMore {
		t.Error("expected has_more=true on page 1 with 5 users and limit=2")
	}
	nextCursor, _ := body1["next_cursor"].(string)
	if nextCursor == "" {
		t.Error("expected non-empty next_cursor on page 1")
	}
	data1 := body1["data"].([]any)
	firstPageIDs := make(map[string]bool)
	for _, item := range data1 {
		u := item.(map[string]any)
		firstPageIDs[u["id"].(string)] = true
	}

	// Second page using the cursor.
	rr2 := h.do("GET", "/users?limit=2&cursor="+nextCursor, nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("page2 expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	body2 := decodeBody(t, rr2)
	data2 := body2["data"].([]any)
	if len(data2) == 0 {
		t.Fatal("page 2 returned no items")
	}

	// Page 2 IDs must be different from page 1.
	for _, item := range data2 {
		u := item.(map[string]any)
		id := u["id"].(string)
		if firstPageIDs[id] {
			t.Errorf("id %s appeared on both page 1 and page 2", id)
		}
	}
}

func TestCreateUser_201(t *testing.T) {
	h := newHarness(t)

	rr := h.do("POST", "/users", map[string]any{
		"email": "new.user@swp.test",
		"role":  "hr_admin",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Location header must be set.
	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Error("missing Location header")
	}

	body := decodeBody(t, rr)
	requiredKeys := []string{"id", "email", "role", "status", "created_at", "updated_at"}
	for _, k := range requiredKeys {
		if _, ok := body[k]; !ok {
			t.Errorf("create response missing key: %s", k)
		}
	}
	if body["email"] != "new.user@swp.test" {
		t.Errorf("email = %v, want new.user@swp.test", body["email"])
	}
	if body["status"] != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", body["status"])
	}
}

func TestCreateUser_409_EmailTaken(t *testing.T) {
	h := newHarness(t)
	// Seed a user with that email.
	h.repo.addUser(domain.User{
		ID:        "SWP-USR-EXIST",
		Email:     "taken@swp.test",
		Role:      auth.RoleAgent,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	rr := h.do("POST", "/users", map[string]any{
		"email": "taken@swp.test",
		"role":  "agent",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "CONFLICT" {
		t.Errorf("error.code = %v, want CONFLICT", errObj["code"])
	}
}

func TestChangeUserRole_422_RoleNotAllowed(t *testing.T) {
	h := newHarness(t)
	// Seed a user to change.
	h.repo.addUser(domain.User{
		ID:        "SWP-USR-ROLE1",
		Email:     "role.user@swp.test",
		Role:      auth.RoleAgent,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	rr := h.do("POST", "/users/SWP-USR-ROLE1:change-role", map[string]any{
		"new_role": "wizard", // invalid role
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "ROLE_NOT_ALLOWED" {
		t.Errorf("error.code = %v, want ROLE_NOT_ALLOWED", errObj["code"])
	}
}

func TestDeactivateReactivate(t *testing.T) {
	h := newHarness(t)
	h.repo.addUser(domain.User{
		ID:        "SWP-USR-DACRE",
		Email:     "dacre@swp.test",
		Role:      auth.RoleAgent,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	// Deactivate -> 200 with status DISABLED.
	rr := h.do("POST", "/users/SWP-USR-DACRE:deactivate", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("deactivate 1: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["status"] != "DISABLED" {
		t.Errorf("after deactivate status = %v, want DISABLED", body["status"])
	}

	// Deactivate again -> 409 (already disabled).
	rr2 := h.do("POST", "/users/SWP-USR-DACRE:deactivate", nil)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("deactivate 2: expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}

	// Reactivate -> 200 with status ACTIVE.
	rr3 := h.do("POST", "/users/SWP-USR-DACRE:reactivate", nil)
	if rr3.Code != http.StatusOK {
		t.Fatalf("reactivate 1: expected 200, got %d: %s", rr3.Code, rr3.Body.String())
	}
	body3 := decodeBody(t, rr3)
	if body3["status"] != "ACTIVE" {
		t.Errorf("after reactivate status = %v, want ACTIVE", body3["status"])
	}

	// Reactivate again -> 409 (already active).
	rr4 := h.do("POST", "/users/SWP-USR-DACRE:reactivate", nil)
	if rr4.Code != http.StatusConflict {
		t.Fatalf("reactivate 2: expected 409, got %d: %s", rr4.Code, rr4.Body.String())
	}
}

func TestSendPasswordReset_202(t *testing.T) {
	h := newHarness(t)
	h.repo.addUser(domain.User{
		ID:        "SWP-USR-RESET",
		Email:     "reset.me@swp.test",
		Role:      auth.RoleAgent,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	rr := h.do("POST", "/users/SWP-USR-RESET:send-password-reset", nil)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if _, ok := body["message"]; !ok {
		t.Error("response missing 'message' field")
	}

	// A reset token must have been inserted.
	if len(h.repo.resetTokens) == 0 {
		t.Error("expected a reset token to be inserted into the fake repo")
	}
}

func TestRBAC_NonAdmin_403(t *testing.T) {
	endpoints := []struct {
		method string
		path   string
		body   any
	}{
		{"GET", "/users", nil},
		{"POST", "/users", map[string]any{"email": "x@swp.test", "role": "agent"}},
		{"GET", "/audit-log", nil},
		{"GET", "/platform/settings", nil},
	}

	for _, role := range []auth.Role{auth.RoleAgent, auth.RoleShiftLeader} {
		for _, ep := range endpoints {
			ep := ep
			role := role
			t.Run(string(role)+":"+ep.method+ep.path, func(t *testing.T) {
				h := newHarness(t)
				h.principal = auth.Principal{UserID: "SWP-USR-9999", Role: role, CompanyID: "SWP-CMP-0001"}

				rr := h.do(ep.method, ep.path, ep.body)
				if rr.Code != http.StatusForbidden {
					t.Fatalf("%s %s as %s: expected 403, got %d: %s",
						ep.method, ep.path, role, rr.Code, rr.Body.String())
				}
				body := decodeBody(t, rr)
				errObj, _ := body["error"].(map[string]any)
				if errObj["code"] != "FORBIDDEN" {
					t.Errorf("error.code = %v, want FORBIDDEN", errObj["code"])
				}
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Task 2: Audit-log + Platform settings
// ---------------------------------------------------------------------------

func TestListAuditLog_ShapeAndFilters(t *testing.T) {
	h := newHarness(t)
	// Seed 4 entries: indices 0,2 are entity_type=user, 1,3 are entity_type=placement.
	h.seedAuditEntries(4)

	// GET /audit-log without filters — should return all 4.
	rr := h.do("GET", "/audit-log", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Envelope keys.
	for _, k := range []string{"data", "next_cursor", "has_more"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing envelope key: %s", k)
		}
	}

	data, _ := body["data"].([]any)
	if len(data) == 0 {
		t.Fatal("data array is empty")
	}

	// Assert AuditLogEntrySummary keys on data[0].
	first := data[0].(map[string]any)
	summaryKeys := []string{
		"id", "actor_user_id", "actor_label",
		"action", "entity_type", "entity_id",
		"change_summary", "ip", "created_at",
	}
	for _, k := range summaryKeys {
		if _, ok := first[k]; !ok {
			t.Errorf("data[0] missing AuditLogEntrySummary key: %s", k)
		}
	}

	// Filter by entity_type=user: indices 0 and 2 are "user".
	rr2 := h.do("GET", "/audit-log?entity_type=user", nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("filtered: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	body2 := decodeBody(t, rr2)
	data2, _ := body2["data"].([]any)
	for _, item := range data2 {
		u := item.(map[string]any)
		if u["entity_type"] != "user" {
			t.Errorf("filter entity_type=user: got entity_type=%v", u["entity_type"])
		}
	}
	if len(data2) != 2 {
		t.Errorf("expected 2 user entries after filter, got %d", len(data2))
	}
}

func TestListAuditLog_CursorAdvances(t *testing.T) {
	h := newHarness(t)
	h.seedAuditEntries(5)

	rr := h.do("GET", "/audit-log?limit=2", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("page1 expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body1 := decodeBody(t, rr)

	hasMore, _ := body1["has_more"].(bool)
	if !hasMore {
		t.Error("expected has_more=true on page 1 with 5 entries and limit=2")
	}
	nextCursor, _ := body1["next_cursor"].(string)
	if nextCursor == "" {
		t.Error("expected non-empty next_cursor")
	}
	data1 := body1["data"].([]any)
	page1IDs := make(map[string]bool)
	for _, item := range data1 {
		e := item.(map[string]any)
		page1IDs[e["id"].(string)] = true
	}

	// Second page.
	rr2 := h.do("GET", "/audit-log?limit=2&cursor="+nextCursor, nil)
	if rr2.Code != http.StatusOK {
		t.Fatalf("page2 expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	body2 := decodeBody(t, rr2)
	data2 := body2["data"].([]any)
	if len(data2) == 0 {
		t.Fatal("page 2 returned no items")
	}
	for _, item := range data2 {
		e := item.(map[string]any)
		id := e["id"].(string)
		if page1IDs[id] {
			t.Errorf("audit id %s appeared on both page 1 and page 2", id)
		}
	}
}

func TestGetAuditLogEntry_200(t *testing.T) {
	h := newHarness(t)
	actorID := "SWP-USR-00001"
	actorRole := "hr_admin"
	reqID := "req_test_001"
	entry := domain.AuditEntry{
		ID:          "SWP-AL-00001",
		ActorUserID: &actorID,
		ActorRole:   &actorRole,
		Action:      "CREATE",
		EntityType:  "user",
		EntityID:    "SWP-USR-00002",
		Before:      nil,
		After:       map[string]any{"email": "created@swp.test", "role": "agent"},
		RequestID:   &reqID,
		CreatedAt:   time.Now().UTC(),
	}
	h.repo.auditEntries = append(h.repo.auditEntries, entry)

	rr := h.do("GET", "/audit-log/SWP-AL-00001", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// AuditLogEntry detail must have before, after, request_id.
	if _, ok := body["before"]; !ok {
		t.Error("missing key: before")
	}
	if _, ok := body["after"]; !ok {
		t.Error("missing key: after")
	}
	if _, ok := body["request_id"]; !ok {
		t.Error("missing key: request_id")
	}
	if body["request_id"] != reqID {
		t.Errorf("request_id = %v, want %s", body["request_id"], reqID)
	}

	// All summary keys must also be present in the detail view.
	detailKeys := []string{
		"id", "actor_user_id", "actor_label",
		"action", "entity_type", "entity_id",
		"change_summary", "ip", "created_at",
	}
	for _, k := range detailKeys {
		if _, ok := body[k]; !ok {
			t.Errorf("detail response missing key: %s", k)
		}
	}
}

func TestGetAuditLogEntry_404(t *testing.T) {
	h := newHarness(t)

	rr := h.do("GET", "/audit-log/SWP-AL-NONEXIST", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("error.code = %v, want NOT_FOUND", errObj["code"])
	}
}

func TestGetPlatformSettings_200(t *testing.T) {
	h := newHarness(t)
	h.seedPlatformSettings()

	rr := h.do("GET", "/platform/settings", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)

	// Must have exactly the 7 keys.
	expectedKeys := []string{
		"locale", "timezone", "date_format",
		"currency", "version", "stack", "legacy_data_source",
	}
	for _, k := range expectedKeys {
		raw, ok := body[k]
		if !ok {
			t.Errorf("platform/settings missing key: %s", k)
			continue
		}
		// Each value must be an object with value, label, locked.
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Errorf("platform/settings.%s is not an object", k)
			continue
		}
		for _, sk := range []string{"value", "label", "locked"} {
			if _, ok := entry[sk]; !ok {
				t.Errorf("platform/settings.%s missing sub-key: %s", k, sk)
			}
		}
	}

	// Specific assertions per the spec.
	if locale, ok := body["locale"].(map[string]any); ok {
		if locale["value"] != "id-ID" {
			t.Errorf("locale.value = %v, want id-ID", locale["value"])
		}
		if locale["locked"] != true {
			t.Errorf("locale.locked = %v, want true", locale["locked"])
		}
	} else {
		t.Error("locale is missing or not an object")
	}

	if _, ok := body["legacy_data_source"]; !ok {
		t.Error("missing key: legacy_data_source")
	}
}

func TestAuditLog_RBAC_403(t *testing.T) {
	h := newHarness(t)
	// Use agent role — already covered by TestRBAC_NonAdmin_403 but explicit here
	// for audit-log specific visibility.
	h.principal = auth.Principal{UserID: "SWP-USR-AGENT", Role: auth.RoleAgent}

	rr := h.do("GET", "/audit-log", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("agent accessing /audit-log: expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("error.code = %v, want FORBIDDEN", errObj["code"])
	}
}
