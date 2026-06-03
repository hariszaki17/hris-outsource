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

// --- mapping helpers ---

func toDomainUserFromEmail(u sqlcgen.GetUserByEmailRow) domain.User {
	return domain.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         auth.Role(u.Role),
		EmployeeID:   derefStr(u.EmployeeID),
		CompanyID:    derefStr(u.CompanyID),
		Status:       u.Status,
		FullName:     u.FullName,
		LastLoginAt:  u.LastLoginAt,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func toDomainUserFromID(u sqlcgen.GetUserByIDRow) domain.User {
	return domain.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         auth.Role(u.Role),
		EmployeeID:   derefStr(u.EmployeeID),
		CompanyID:    derefStr(u.CompanyID),
		Status:       u.Status,
		FullName:     u.FullName,
		LastLoginAt:  u.LastLoginAt,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
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
