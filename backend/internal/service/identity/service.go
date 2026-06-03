// Package identity is the E1 authentication service: login, refresh-token
// rotation with reuse detection, logout, forgot/reset password. It is the
// reference vertical slice — handler -> service -> repository — that the other
// epics mirror.
package identity

import (
	"context"
	"errors"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// Repository is the data dependency, defined by this consumer (Go idiom) so it
// can be mocked in tests. The repository layer implements it over sqlc.
type Repository interface {
	// Reads run on the pool.
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	GetUserByID(ctx context.Context, id string) (domain.User, error)
	GetRefreshTokenByHash(ctx context.Context, hash string) (domain.RefreshToken, error)
	GetResetTokenByHash(ctx context.Context, hash string) (domain.PasswordResetToken, error)
	// Writes take the active tx so they commit atomically with audit/jobs.
	InsertRefreshToken(ctx context.Context, tx pgx.Tx, p NewRefreshToken) (domain.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tx pgx.Tx, id int64) error
	RevokeFamily(ctx context.Context, tx pgx.Tx, familyID string) error
	RevokeAllRefreshForUser(ctx context.Context, tx pgx.Tx, userID string) error
	SetLastLogin(ctx context.Context, tx pgx.Tx, id string) error
	UpdatePassword(ctx context.Context, tx pgx.Tx, id, hash string) error
	InsertResetToken(ctx context.Context, tx pgx.Tx, userID, tokenHash string, expiresAt time.Time) (domain.PasswordResetToken, error)
	MarkResetTokenUsed(ctx context.Context, tx pgx.Tx, id int64) error
}

// NewRefreshToken carries the fields needed to persist a rotated token.
type NewRefreshToken struct {
	UserID      string
	TokenHash   string
	FamilyID    string
	RotatedFrom *int64
	UserAgent   string
	IP          string
	ExpiresAt   time.Time
}

// TxRunner is a thin interface over db.TxManager so tests can inject a fake.
// Production code passes *db.TxManager (which satisfies this interface).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock is injectable for deterministic tests; defaults to time.Now.
type Clock func() time.Time

type Service struct {
	repo       Repository
	txm        TxRunner
	issuer     *auth.Issuer
	refreshTTL time.Duration
	now        Clock
}

func NewService(repo Repository, txm TxRunner, issuer *auth.Issuer, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, txm: txm, issuer: issuer, refreshTTL: refreshTTL, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *Service) SetClock(c Clock) { s.now = c }

// Result is the token pair returned by login/refresh. RefreshToken is the
// one-time plaintext (cookie for web, secure storage for mobile).
type Result struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	Principal        auth.Principal
	User             domain.User // full user for LoginResponse (includes FullName, LastLoginAt, etc.)
}

// Login verifies credentials and issues a token pair. Records last_login_at (AU-3).
func (s *Service) Login(ctx context.Context, email, password, userAgent, ip string) (Result, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return Result{}, errInvalidCredentials()
	}
	if err != nil {
		return Result{}, apperr.Internal(err)
	}
	if err := auth.VerifyPassword(password, user.PasswordHash); err != nil {
		// Same generic error whether the email or the password was wrong.
		return Result{}, errInvalidCredentials()
	}
	if !user.IsActive() {
		return Result{}, errAccountDisabled()
	}
	return s.issuePair(ctx, user, uuid.NewString(), nil, userAgent, ip)
}

// Refresh rotates a refresh token. A revoked/expired token triggers family
// revocation (reuse detection) and a re-auth.
func (s *Service) Refresh(ctx context.Context, plaintext, userAgent, ip string) (Result, error) {
	hash := auth.HashRefreshToken(plaintext)
	current, err := s.repo.GetRefreshTokenByHash(ctx, hash)
	if errors.Is(err, domain.ErrNotFound) {
		return Result{}, errInvalidRefresh()
	}
	if err != nil {
		return Result{}, apperr.Internal(err)
	}

	now := s.now()
	if current.RevokedAt != nil {
		// Reuse of an already-rotated token: revoke the whole family.
		_ = s.txm.InTx(ctx, func(tx pgx.Tx) error {
			return s.repo.RevokeFamily(ctx, tx, current.FamilyID)
		})
		return Result{}, errInvalidRefresh()
	}
	if !current.IsLive(now) {
		return Result{}, errInvalidRefresh()
	}

	user, err := s.repo.GetUserByID(ctx, current.UserID)
	if err != nil {
		return Result{}, apperr.Internal(err)
	}
	if !user.IsActive() {
		return Result{}, errAccountDisabled()
	}
	return s.issuePair(ctx, user, current.FamilyID, &current.ID, userAgent, ip)
}

// Logout revokes the presented refresh token (idempotent).
func (s *Service) Logout(ctx context.Context, plaintext string) error {
	hash := auth.HashRefreshToken(plaintext)
	current, err := s.repo.GetRefreshTokenByHash(ctx, hash)
	if errors.Is(err, domain.ErrNotFound) {
		return nil // already gone — treat as success
	}
	if err != nil {
		return apperr.Internal(err)
	}
	return s.txm.InTx(ctx, func(tx pgx.Tx) error {
		return s.repo.RevokeRefreshToken(ctx, tx, current.ID)
	})
}

// Me loads the full user record for the /auth/me endpoint.
func (s *Service) Me(ctx context.Context, userID string) (domain.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, apperr.NotFound()
	}
	if err != nil {
		return domain.User{}, apperr.Internal(err)
	}
	return user, nil
}

// ForgotPassword creates a single-use reset token for the email if found and
// active. Per C-2 of authentication.md, it always returns nil (no enumeration):
// the caller MUST emit 202 regardless of whether the email matched. The plaintext
// token is NOT emailed in this phase — the E2E harness obtains it directly from
// the password_reset_tokens table.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return nil // anti-enumeration: no error, no token
	}
	if err != nil {
		return apperr.Internal(err)
	}
	if !user.IsActive() {
		return nil // disabled account: still no error (anti-enumeration)
	}

	_, hash := auth.NewRefreshToken() // reuse opaque-token + sha256 helper
	expiresAt := s.now().Add(time.Hour)

	return s.txm.InTx(ctx, func(tx pgx.Tx) error {
		_, err := s.repo.InsertResetToken(ctx, tx, user.ID, hash, expiresAt)
		return err
	})
}

// ResetPassword validates a plaintext reset token, enforces the password policy,
// sets the new password, and revokes all of the user's sessions (AU-6).
func (s *Service) ResetPassword(ctx context.Context, plaintext, newPassword string) error {
	if err := validatePasswordPolicy(newPassword); err != nil {
		return err
	}

	hash := auth.HashRefreshToken(plaintext) // same sha256 helper
	token, err := s.repo.GetResetTokenByHash(ctx, hash)
	if errors.Is(err, domain.ErrNotFound) {
		return errResetTokenExpired()
	}
	if err != nil {
		return apperr.Internal(err)
	}
	if !token.IsLive(s.now()) {
		return errResetTokenExpired()
	}

	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return apperr.Internal(err)
	}

	return s.txm.InTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.UpdatePassword(ctx, tx, token.UserID, newHash); err != nil {
			return err
		}
		if err := s.repo.MarkResetTokenUsed(ctx, tx, token.ID); err != nil {
			return err
		}
		return s.repo.RevokeAllRefreshForUser(ctx, tx, token.UserID)
	})
}

// issuePair mints an access token + a new refresh token. When rotatedFrom is
// set, the old token is revoked in the SAME tx as the new one is inserted.
// Also records last_login_at (AU-3) in the same transaction.
func (s *Service) issuePair(ctx context.Context, user domain.User, familyID string, rotatedFrom *int64, userAgent, ip string) (Result, error) {
	now := s.now()
	principal := user.Principal()

	access, accessExp, err := s.issuer.Issue(principal, now)
	if err != nil {
		return Result{}, apperr.Internal(err)
	}

	plaintext, hash := auth.NewRefreshToken()
	refreshExp := now.Add(s.refreshTTL)

	err = s.txm.InTx(ctx, func(tx pgx.Tx) error {
		if rotatedFrom != nil {
			if err := s.repo.RevokeRefreshToken(ctx, tx, *rotatedFrom); err != nil {
				return err
			}
		}
		_, err := s.repo.InsertRefreshToken(ctx, tx, NewRefreshToken{
			UserID:      user.ID,
			TokenHash:   hash,
			FamilyID:    familyID,
			RotatedFrom: rotatedFrom,
			UserAgent:   userAgent,
			IP:          ip,
			ExpiresAt:   refreshExp,
		})
		if err != nil {
			return err
		}
		// AU-3: record the last login time in the same tx.
		return s.repo.SetLastLogin(ctx, tx, user.ID)
	})
	if err != nil {
		return Result{}, apperr.Internal(err)
	}

	// Patch the user's LastLoginAt so the response reflects the just-set value
	// without an extra DB round-trip.
	user.LastLoginAt = &now

	return Result{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     plaintext,
		RefreshExpiresAt: refreshExp,
		Principal:        principal,
		User:             user,
	}, nil
}

// validatePasswordPolicy enforces the platform password policy (AU-4).
// Min 10 chars, must contain upper, lower, digit, and symbol.
func validatePasswordPolicy(pw string) error {
	if len(pw) < 10 {
		return errWeakPassword()
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSymbol {
		return errWeakPassword()
	}
	return nil
}

// --- domain-specific errors (i18n messages live in platform/i18n) ---

func errInvalidCredentials() *apperr.Error {
	return &apperr.Error{Code: "INVALID_CREDENTIALS", HTTPStatus: 401}
}
func errAccountDisabled() *apperr.Error {
	return &apperr.Error{Code: "ACCOUNT_DISABLED", HTTPStatus: 403}
}
func errInvalidRefresh() *apperr.Error {
	return &apperr.Error{Code: "INVALID_REFRESH", HTTPStatus: 401}
}
func errResetTokenExpired() *apperr.Error {
	return &apperr.Error{Code: "RESET_TOKEN_EXPIRED", HTTPStatus: 401}
}
func errWeakPassword() *apperr.Error {
	return apperr.Rule("WEAK_PASSWORD", map[string]string{
		"new_password": "Minimal 10 karakter, harus mengandung huruf besar, kecil, angka, dan simbol.",
	})
}
