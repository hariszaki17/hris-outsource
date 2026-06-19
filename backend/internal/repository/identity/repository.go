// Package identity (repository) implements the identity service's Repository
// over sqlc-generated queries. It maps between the generated row structs and the
// dependency-free domain types, and translates pgx.ErrNoRows -> domain.ErrNotFound.
// Reads run on the pool; writes run on the caller's transaction.
//
// NOTE: the sqlcgen import resolves after `make gen` (sqlc generate). That is the
// contract-first build step, same as the frontend's Orval output.
package identity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	"github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/identity"
)

// Repository is the sqlc-backed implementation of svc.Repository.
type Repository struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// compile-time check that we satisfy the service's port.
var _ svc.Repository = (*Repository)(nil)

func New(pool *db.Pool) *Repository {
	return &Repository{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// GetUserByIdentifier is the login lookup (D2): matches by phone OR email.
func (r *Repository) GetUserByIdentifier(ctx context.Context, identifier string) (domain.User, error) {
	row, err := r.q.GetUserByIdentifier(ctx, identifier)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return toDomainUserFromIdentifier(row), nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return toDomainUserFromEmail(row), nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return toDomainUserFromID(row), nil
}

func (r *Repository) GetRefreshTokenByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	row, err := r.q.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return domain.RefreshToken{}, mapErr(err)
	}
	return domain.RefreshToken{
		ID:          row.ID,
		UserID:      row.UserID,
		TokenHash:   row.TokenHash,
		FamilyID:    row.FamilyID,
		RotatedFrom: row.RotatedFrom,
		ExpiresAt:   row.ExpiresAt,
		RevokedAt:   row.RevokedAt,
		CreatedAt:   row.CreatedAt,
	}, nil
}

func (r *Repository) InsertRefreshToken(ctx context.Context, tx pgx.Tx, p svc.NewRefreshToken) (domain.RefreshToken, error) {
	row, err := r.q.WithTx(tx).InsertRefreshToken(ctx, sqlcgen.InsertRefreshTokenParams{
		UserID:      p.UserID,
		TokenHash:   p.TokenHash,
		FamilyID:    p.FamilyID,
		RotatedFrom: p.RotatedFrom,
		UserAgent:   nullStr(p.UserAgent),
		Ip:          nullStr(p.IP),
		ExpiresAt:   p.ExpiresAt,
	})
	if err != nil {
		return domain.RefreshToken{}, mapErr(err)
	}
	return domain.RefreshToken{
		ID:          row.ID,
		UserID:      row.UserID,
		TokenHash:   row.TokenHash,
		FamilyID:    row.FamilyID,
		RotatedFrom: row.RotatedFrom,
		ExpiresAt:   row.ExpiresAt,
		RevokedAt:   row.RevokedAt,
		CreatedAt:   row.CreatedAt,
	}, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tx pgx.Tx, id int64) error {
	return mapErr(r.q.WithTx(tx).RevokeRefreshToken(ctx, id))
}

func (r *Repository) RevokeFamily(ctx context.Context, tx pgx.Tx, familyID string) error {
	return mapErr(r.q.WithTx(tx).RevokeFamily(ctx, familyID))
}

// SetLastLogin records the current time as the user's last login (AU-3).
func (r *Repository) SetLastLogin(ctx context.Context, tx pgx.Tx, id string) error {
	return mapErr(r.q.WithTx(tx).SetLastLogin(ctx, id))
}

// UpdatePassword sets a new bcrypt/argon2 password hash for the user (AU-4).
func (r *Repository) UpdatePassword(ctx context.Context, tx pgx.Tx, id, hash string) error {
	return mapErr(r.q.WithTx(tx).UpdatePassword(ctx, sqlcgen.UpdatePasswordParams{
		ID:           id,
		PasswordHash: hash,
	}))
}

// InsertResetToken persists a hashed password-reset token (AU-4).
func (r *Repository) InsertResetToken(ctx context.Context, tx pgx.Tx, userID, tokenHash string, expiresAt time.Time) (domain.PasswordResetToken, error) {
	row, err := r.q.WithTx(tx).InsertResetToken(ctx, sqlcgen.InsertResetTokenParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return domain.PasswordResetToken{}, mapErr(err)
	}
	return domain.PasswordResetToken{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt,
		UsedAt:    row.UsedAt,
	}, nil
}

// GetResetTokenByHash fetches a reset token by its SHA-256 hash (AU-4 verify).
func (r *Repository) GetResetTokenByHash(ctx context.Context, hash string) (domain.PasswordResetToken, error) {
	row, err := r.q.GetResetTokenByHash(ctx, hash)
	if err != nil {
		return domain.PasswordResetToken{}, mapErr(err)
	}
	return domain.PasswordResetToken{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt,
		UsedAt:    row.UsedAt,
	}, nil
}

// MarkResetTokenUsed marks the token consumed so it cannot be replayed (AU-4).
func (r *Repository) MarkResetTokenUsed(ctx context.Context, tx pgx.Tx, id int64) error {
	return mapErr(r.q.WithTx(tx).MarkResetTokenUsed(ctx, id))
}

// RevokeAllRefreshForUser invalidates every active session for the user (AU-6).
func (r *Repository) RevokeAllRefreshForUser(ctx context.Context, tx pgx.Tx, userID string) error {
	return mapErr(r.q.WithTx(tx).RevokeAllRefreshForUser(ctx, userID))
}

// GetLoginAttempt fetches a login attempt by normalized identifier (E1 F1.5).
func (r *Repository) GetLoginAttempt(ctx context.Context, identifier string) (domain.LoginAttempt, error) {
	var a domain.LoginAttempt
	row := r.pool.QueryRow(ctx,
		`SELECT id, identifier, attempt_count, locked_until FROM login_attempts WHERE identifier = $1`,
		identifier,
	)
	err := row.Scan(&a.ID, &a.Identifier, &a.AttemptCount, &a.LockedUntil)
	if err != nil {
		return domain.LoginAttempt{}, mapErr(err)
	}
	return a, nil
}

// UpsertLoginAttempt inserts or updates a login attempt record (E1 F1.5).
func (r *Repository) UpsertLoginAttempt(ctx context.Context, tx pgx.Tx, identifier string, count int, lockedUntil *time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO login_attempts (identifier, attempt_count, locked_until, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (identifier) DO UPDATE SET
		     attempt_count = EXCLUDED.attempt_count,
		     locked_until  = EXCLUDED.locked_until,
		     updated_at    = now()`,
		identifier, count, lockedUntil,
	)
	return mapErr(err)
}

// DeleteLoginAttempt removes the login attempt row for the identifier (E1 F1.5).
func (r *Repository) DeleteLoginAttempt(ctx context.Context, tx pgx.Tx, identifier string) error {
	_, err := tx.Exec(ctx, `DELETE FROM login_attempts WHERE identifier = $1`, identifier)
	return mapErr(err)
}

// --- mapping helpers ---

func toDomainUserFromIdentifier(u sqlcgen.GetUserByIdentifierRow) domain.User {
	return domain.User{
		ID:           u.ID,
		Email:        derefStr(u.Email),
		Phone:        derefStr(u.Phone),
		PasswordHash: u.PasswordHash,
		Role:         auth.Role(u.Role),
		EmployeeID:   derefStr(u.EmployeeID),
		CompanyID:    derefStr(u.CompanyID),
		Status:             u.Status,
		FullName:           u.FullName,
		LastLoginAt:        u.LastLoginAt,
		MustChangePassword: u.MustChangePassword,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
	}
}

func toDomainUserFromEmail(u sqlcgen.GetUserByEmailRow) domain.User {
	return domain.User{
		ID:           u.ID,
		Email:        derefStr(u.Email),
		Phone:        derefStr(u.Phone),
		PasswordHash: u.PasswordHash,
		Role:         auth.Role(u.Role),
		EmployeeID:   derefStr(u.EmployeeID),
		CompanyID:    derefStr(u.CompanyID),
		Status:             u.Status,
		FullName:           u.FullName,
		LastLoginAt:        u.LastLoginAt,
		MustChangePassword: u.MustChangePassword,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
	}
}

func toDomainUserFromID(u sqlcgen.GetUserByIDRow) domain.User {
	return domain.User{
		ID:           u.ID,
		Email:        derefStr(u.Email),
		Phone:        derefStr(u.Phone),
		PasswordHash: u.PasswordHash,
		Role:         auth.Role(u.Role),
		EmployeeID:   derefStr(u.EmployeeID),
		CompanyID:    derefStr(u.CompanyID),
		Status:             u.Status,
		FullName:           u.FullName,
		LastLoginAt:        u.LastLoginAt,
		MustChangePassword: u.MustChangePassword,
		TokensValidAfter:   u.TokensValidAfter,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
	}
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
