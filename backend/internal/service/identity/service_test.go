package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/identity"
)

// ---------------------------------------------------------------------------
// Fake TxRunner — executes the closure with a nil tx.
// The fakeRepo methods accept pgx.Tx but never dereference it.
// ---------------------------------------------------------------------------

type fakeTxRunner struct{}

func (f *fakeTxRunner) InTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return fn(nil)
}

// ---------------------------------------------------------------------------
// Fake repository
// ---------------------------------------------------------------------------

type fakeRepo struct {
	users         map[string]domain.User               // keyed by lower-cased email
	usersByID     map[string]domain.User               // keyed by id
	refreshTokens map[string]domain.RefreshToken        // keyed by hash
	resetTokens   map[string]domain.PasswordResetToken  // keyed by hash
	resetNextID   int64

	// call tracking
	setLastLoginCalls       []string
	updatePasswordCalls     []updatePasswordCall
	insertResetTokenCalls   []insertResetTokenCall
	markResetTokenUsedCalls []int64
	revokeAllForUserCalls   []string
	revokeTokenCalls        []int64
	revokeFamilyCalls       []string
}

type updatePasswordCall struct{ userID, hash string }
type insertResetTokenCall struct {
	userID, tokenHash string
	expiresAt         time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:         make(map[string]domain.User),
		usersByID:     make(map[string]domain.User),
		refreshTokens: make(map[string]domain.RefreshToken),
		resetTokens:   make(map[string]domain.PasswordResetToken),
	}
}

func lower(s string) string {
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

func (f *fakeRepo) addUser(u domain.User) {
	f.users[lower(u.Email)] = u
	f.usersByID[u.ID] = u
}

func (f *fakeRepo) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	u, ok := f.users[lower(email)]
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

func (f *fakeRepo) InsertRefreshToken(_ context.Context, _ pgx.Tx, p svc.NewRefreshToken) (domain.RefreshToken, error) {
	t := domain.RefreshToken{
		ID:          1,
		UserID:      p.UserID,
		TokenHash:   p.TokenHash,
		FamilyID:    p.FamilyID,
		RotatedFrom: p.RotatedFrom,
		ExpiresAt:   p.ExpiresAt,
	}
	f.refreshTokens[p.TokenHash] = t
	return t, nil
}

func (f *fakeRepo) RevokeRefreshToken(_ context.Context, _ pgx.Tx, id int64) error {
	f.revokeTokenCalls = append(f.revokeTokenCalls, id)
	return nil
}

func (f *fakeRepo) RevokeFamily(_ context.Context, _ pgx.Tx, familyID string) error {
	f.revokeFamilyCalls = append(f.revokeFamilyCalls, familyID)
	return nil
}

func (f *fakeRepo) RevokeAllRefreshForUser(_ context.Context, _ pgx.Tx, userID string) error {
	f.revokeAllForUserCalls = append(f.revokeAllForUserCalls, userID)
	return nil
}

func (f *fakeRepo) SetLastLogin(_ context.Context, _ pgx.Tx, id string) error {
	f.setLastLoginCalls = append(f.setLastLoginCalls, id)
	return nil
}

func (f *fakeRepo) UpdatePassword(_ context.Context, _ pgx.Tx, id, hash string) error {
	f.updatePasswordCalls = append(f.updatePasswordCalls, updatePasswordCall{id, hash})
	return nil
}

func (f *fakeRepo) InsertResetToken(_ context.Context, _ pgx.Tx, userID, tokenHash string, expiresAt time.Time) (domain.PasswordResetToken, error) {
	f.resetNextID++
	id := f.resetNextID
	t := domain.PasswordResetToken{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}
	f.resetTokens[tokenHash] = t
	f.insertResetTokenCalls = append(f.insertResetTokenCalls, insertResetTokenCall{userID, tokenHash, expiresAt})
	return t, nil
}

func (f *fakeRepo) GetResetTokenByHash(_ context.Context, hash string) (domain.PasswordResetToken, error) {
	t, ok := f.resetTokens[hash]
	if !ok {
		return domain.PasswordResetToken{}, domain.ErrNotFound
	}
	return t, nil
}

func (f *fakeRepo) MarkResetTokenUsed(_ context.Context, _ pgx.Tx, id int64) error {
	f.markResetTokenUsedCalls = append(f.markResetTokenUsedCalls, id)
	return nil
}

// compile-time interface check
var _ svc.Repository = (*fakeRepo)(nil)

// ---------------------------------------------------------------------------
// Test service factory
// ---------------------------------------------------------------------------

func newTestService(t *testing.T, repo svc.Repository, fixedTime time.Time) *svc.Service {
	t.Helper()
	privB64, pubB64, err := auth.GenerateKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	issuer, err := auth.NewIssuer(privB64, pubB64, 30*time.Minute)
	if err != nil {
		t.Fatalf("new issuer: %v", err)
	}
	s := svc.NewService(repo, &fakeTxRunner{}, issuer, 7*24*time.Hour)
	s.SetClock(func() time.Time { return fixedTime })
	return s
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestLogin_SetLastLoginCalled(t *testing.T) {
	repo := newFakeRepo()
	pw := "Pass1ng-Garuda!"
	hash, _ := auth.HashPassword(pw)
	repo.addUser(domain.User{
		ID:           "SWP-USR-1042",
		Email:        "sari@test.swp",
		PasswordHash: hash,
		Role:         auth.RoleHRAdmin,
		Status:       "active",
	})

	fixed := time.Date(2026, 6, 3, 7, 14, 52, 0, time.UTC)
	s := newTestService(t, repo, fixed)

	result, err := s.Login(context.Background(), "sari@test.swp", pw, "go-test", "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if len(repo.setLastLoginCalls) == 0 {
		t.Fatal("expected SetLastLogin to be called after login")
	}
	if repo.setLastLoginCalls[0] != "SWP-USR-1042" {
		t.Errorf("SetLastLogin called with %q, want SWP-USR-1042", repo.setLastLoginCalls[0])
	}
	// LastLoginAt should be set to the fixed time
	if result.User.LastLoginAt == nil {
		t.Fatal("result.User.LastLoginAt should be non-nil after login")
	}
	if !result.User.LastLoginAt.Equal(fixed) {
		t.Errorf("LastLoginAt = %v, want %v", *result.User.LastLoginAt, fixed)
	}
}

func TestLogin_DisabledAccount(t *testing.T) {
	repo := newFakeRepo()
	pw := "Pass1ng-Garuda!"
	hash, _ := auth.HashPassword(pw)
	repo.addUser(domain.User{
		ID:           "SWP-USR-9999",
		Email:        "disabled@test.swp",
		PasswordHash: hash,
		Role:         auth.RoleAgent,
		Status:       "disabled",
	})
	s := newTestService(t, repo, time.Now())
	_, err := s.Login(context.Background(), "disabled@test.swp", pw, "", "")
	if err == nil {
		t.Fatal("expected error for disabled account")
	}
	if err.Error() != "ACCOUNT_DISABLED" {
		t.Errorf("error = %q, want ACCOUNT_DISABLED", err.Error())
	}
}

func TestForgotPassword_KnownEmail_CreatesToken(t *testing.T) {
	repo := newFakeRepo()
	pw := "TestP@ss1234"
	hash, _ := auth.HashPassword(pw)
	repo.addUser(domain.User{
		ID:           "SWP-USR-1111",
		Email:        "known@test.swp",
		PasswordHash: hash,
		Role:         auth.RoleHRAdmin,
		Status:       "active",
	})
	s := newTestService(t, repo, time.Now())

	err := s.ForgotPassword(context.Background(), "known@test.swp")
	if err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}
	if len(repo.insertResetTokenCalls) != 1 {
		t.Errorf("expected 1 InsertResetToken call, got %d", len(repo.insertResetTokenCalls))
	}
	if repo.insertResetTokenCalls[0].userID != "SWP-USR-1111" {
		t.Errorf("InsertResetToken userID = %q, want SWP-USR-1111", repo.insertResetTokenCalls[0].userID)
	}
}

func TestForgotPassword_UnknownEmail_NoError(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(t, repo, time.Now())

	err := s.ForgotPassword(context.Background(), "nobody@test.swp")
	if err != nil {
		t.Fatalf("ForgotPassword with unknown email should return nil, got: %v", err)
	}
	if len(repo.insertResetTokenCalls) != 0 {
		t.Errorf("expected 0 InsertResetToken calls for unknown email, got %d", len(repo.insertResetTokenCalls))
	}
}

func TestResetPassword_WeakPassword(t *testing.T) {
	tests := []struct {
		name string
		pw   string
	}{
		{"too_short", "short"},
		{"no_upper", "nouppercase1!"},
		{"no_lower", "NOLOWERCASE1!"},
		{"no_digit", "NoDigitsHere!"},
		{"no_symbol", "NoSymbols1234"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			s := newTestService(t, repo, time.Now())
			err := s.ResetPassword(context.Background(), "any-token", tt.pw)
			if err == nil {
				t.Fatalf("expected WEAK_PASSWORD error for %q", tt.pw)
			}
			if err.Error() != "WEAK_PASSWORD" {
				t.Errorf("error = %q, want WEAK_PASSWORD", err.Error())
			}
		})
	}
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	repo := newFakeRepo()
	plaintext, hash := auth.NewRefreshToken()
	pastTime := time.Now().Add(-2 * time.Hour)
	repo.resetTokens[hash] = domain.PasswordResetToken{
		ID:        1,
		UserID:    "SWP-USR-1042",
		TokenHash: hash,
		ExpiresAt: pastTime, // already expired
	}

	s := newTestService(t, repo, time.Now())
	err := s.ResetPassword(context.Background(), plaintext, "ValidPass1ng!")
	if err == nil {
		t.Fatal("expected RESET_TOKEN_EXPIRED error")
	}
	if err.Error() != "RESET_TOKEN_EXPIRED" {
		t.Errorf("error = %q, want RESET_TOKEN_EXPIRED", err.Error())
	}
}

func TestResetPassword_ValidToken_Success(t *testing.T) {
	repo := newFakeRepo()
	plaintext, hash := auth.NewRefreshToken()
	const userID = "SWP-USR-1042"
	repo.resetTokens[hash] = domain.PasswordResetToken{
		ID:        42,
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	s := newTestService(t, repo, time.Now())
	err := s.ResetPassword(context.Background(), plaintext, "ValidPass1ng!")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	if len(repo.updatePasswordCalls) != 1 {
		t.Errorf("expected 1 UpdatePassword call, got %d", len(repo.updatePasswordCalls))
	}
	if repo.updatePasswordCalls[0].userID != userID {
		t.Errorf("UpdatePassword userID = %q, want %q", repo.updatePasswordCalls[0].userID, userID)
	}
	if len(repo.markResetTokenUsedCalls) != 1 || repo.markResetTokenUsedCalls[0] != 42 {
		t.Errorf("MarkResetTokenUsed calls = %v, want [42]", repo.markResetTokenUsedCalls)
	}
	if len(repo.revokeAllForUserCalls) != 1 || repo.revokeAllForUserCalls[0] != userID {
		t.Errorf("RevokeAllRefreshForUser calls = %v, want [%q]", repo.revokeAllForUserCalls, userID)
	}
}

func TestResetPassword_UsedToken_Rejected(t *testing.T) {
	repo := newFakeRepo()
	plaintext, hash := auth.NewRefreshToken()
	usedAt := time.Now().Add(-time.Minute)
	repo.resetTokens[hash] = domain.PasswordResetToken{
		ID:        99,
		UserID:    "SWP-USR-1042",
		TokenHash: hash,
		ExpiresAt: time.Now().Add(time.Hour), // not expired by time, but is consumed
		UsedAt:    &usedAt,
	}

	s := newTestService(t, repo, time.Now())
	err := s.ResetPassword(context.Background(), plaintext, "ValidPass1ng!")
	if err == nil {
		t.Fatal("expected RESET_TOKEN_EXPIRED error for used token")
	}
	if err.Error() != "RESET_TOKEN_EXPIRED" {
		t.Errorf("error = %q, want RESET_TOKEN_EXPIRED", err.Error())
	}
}

// TestForgotPassword_DisabledEmail verifies that a disabled account also returns
// nil (anti-enumeration — the response must be identical regardless).
func TestForgotPassword_DisabledEmail_NoToken(t *testing.T) {
	repo := newFakeRepo()
	pw := "TestP@ss1234"
	hash, _ := auth.HashPassword(pw)
	repo.addUser(domain.User{
		ID:           "SWP-USR-2222",
		Email:        "disabled@test.swp",
		PasswordHash: hash,
		Role:         auth.RoleAgent,
		Status:       "disabled",
	})
	s := newTestService(t, repo, time.Now())

	err := s.ForgotPassword(context.Background(), "disabled@test.swp")
	if err != nil {
		t.Fatalf("ForgotPassword for disabled account should return nil, got: %v", err)
	}
	if len(repo.insertResetTokenCalls) != 0 {
		t.Errorf("expected 0 InsertResetToken calls for disabled account, got %d", len(repo.insertResetTokenCalls))
	}
}

// Sentinel — ensures errors package is used.
var _ = errors.New
