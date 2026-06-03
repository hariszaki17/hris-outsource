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
	return toDomainUser(row), nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return toDomainUser(row), nil
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

// --- mapping helpers ---

func toDomainUser(u sqlcgen.User) domain.User {
	return domain.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         auth.Role(u.Role),
		EmployeeID:   derefStr(u.EmployeeID),
		CompanyID:    derefStr(u.CompanyID),
		Status:       u.Status,
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
