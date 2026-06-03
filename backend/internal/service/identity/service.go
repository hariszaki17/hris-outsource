// Package identity is the E1 authentication service: login, refresh-token
// rotation with reuse detection, and logout. It is the reference vertical slice
// — handler -> service -> repository — that the other epics mirror.
package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
)

// Repository is the data dependency, defined by this consumer (Go idiom) so it
// can be mocked in tests. The repository layer implements it over sqlc.
type Repository interface {
	// Reads run on the pool.
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	GetUserByID(ctx context.Context, id string) (domain.User, error)
	GetRefreshTokenByHash(ctx context.Context, hash string) (domain.RefreshToken, error)
	// Writes take the active tx so they commit atomically with audit/jobs.
	InsertRefreshToken(ctx context.Context, tx pgx.Tx, p NewRefreshToken) (domain.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tx pgx.Tx, id int64) error
	RevokeFamily(ctx context.Context, tx pgx.Tx, familyID string) error
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

// Clock is injectable for deterministic tests; defaults to time.Now.
type Clock func() time.Time

type Service struct {
	repo       Repository
	txm        *db.TxManager
	issuer     *auth.Issuer
	refreshTTL time.Duration
	now        Clock
}

func NewService(repo Repository, txm *db.TxManager, issuer *auth.Issuer, refreshTTL time.Duration) *Service {
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
}

// Login verifies credentials and issues a token pair.
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

// issuePair mints an access token + a new refresh token. When rotatedFrom is
// set, the old token is revoked in the SAME tx as the new one is inserted.
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
		return err
	})
	if err != nil {
		return Result{}, apperr.Internal(err)
	}

	return Result{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     plaintext,
		RefreshExpiresAt: refreshExp,
		Principal:        principal,
	}, nil
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
