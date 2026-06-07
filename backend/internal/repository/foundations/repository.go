// Package foundations (repository) implements the foundations service's Repository
// over sqlc-generated queries. It mirrors the identity repository pattern exactly:
// reads on the pool, writes via r.q.WithTx(tx), pgx.ErrNoRows -> domain.ErrNotFound.
package foundations

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/foundations"
)

// Repository is the sqlc-backed implementation of svc.Repository.
type Repository struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// compile-time check: Repository satisfies the service port.
var _ svc.Repository = (*Repository)(nil)

// New returns a new Repository backed by pool.
func New(pool *db.Pool) *Repository {
	return &Repository{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// ListUsers returns a page of users matching the filter (cursor pagination,
// limit+1 already requested by the service layer to detect has_more).
func (r *Repository) ListUsers(ctx context.Context, f domain.UserFilter) ([]domain.User, error) {
	rows, err := r.q.ListUsers(ctx, sqlcgen.ListUsersParams{
		Role:            f.Role,
		Status:          f.Status,
		CompanyID:       f.CompanyID,
		Q:               f.Q,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.User{
			ID:          row.ID,
			Email:       derefStr(row.Email),
			Phone:       derefStr(row.Phone),
			Role:        auth.Role(row.Role),
			EmployeeID:  derefStr(row.EmployeeID),
			CompanyID:   derefStr(row.CompanyID),
			Status:      row.Status,
			FullName:    row.FullName,
			LastLoginAt: row.LastLoginAt,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return out, nil
}

// GetUserByID fetches a single user by SWP-USR id.
func (r *Repository) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return domain.User{
		ID:           row.ID,
		Email:        derefStr(row.Email),
		Phone:        derefStr(row.Phone),
		PasswordHash: row.PasswordHash,
		Role:         auth.Role(row.Role),
		EmployeeID:   derefStr(row.EmployeeID),
		CompanyID:    derefStr(row.CompanyID),
		Status:       row.Status,
		FullName:     row.FullName,
		LastLoginAt:  row.LastLoginAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

// GetUserByEmail fetches a user by email (for uniqueness pre-check).
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return domain.User{
		ID:           row.ID,
		Email:        derefStr(row.Email),
		Phone:        derefStr(row.Phone),
		PasswordHash: row.PasswordHash,
		Role:         auth.Role(row.Role),
		EmployeeID:   derefStr(row.EmployeeID),
		CompanyID:    derefStr(row.CompanyID),
		Status:       row.Status,
		FullName:     row.FullName,
		LastLoginAt:  row.LastLoginAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

// UpdateUserEmail patches a user's email (PATCH /users/{id}).
func (r *Repository) UpdateUserEmail(ctx context.Context, tx pgx.Tx, id, email string) (domain.User, error) {
	row, err := r.q.WithTx(tx).UpdateUserEmail(ctx, sqlcgen.UpdateUserEmailParams{
		ID:    id,
		Email: nullStr(email),
	})
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return fromUpdateRow(row.ID, row.Email, row.Phone, row.Role, row.EmployeeID, row.CompanyID,
		row.Status, row.FullName, row.LastLoginAt, row.CreatedAt, row.UpdatedAt), nil
}

// ChangeUserRole updates the user's role in a transaction.
func (r *Repository) ChangeUserRole(ctx context.Context, tx pgx.Tx, id, role string) (domain.User, error) {
	row, err := r.q.WithTx(tx).ChangeUserRole(ctx, sqlcgen.ChangeUserRoleParams{
		ID:   id,
		Role: role,
	})
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return fromUpdateRow(row.ID, row.Email, row.Phone, row.Role, row.EmployeeID, row.CompanyID,
		row.Status, row.FullName, row.LastLoginAt, row.CreatedAt, row.UpdatedAt), nil
}

// SetUserStatus sets status to 'active' or 'disabled' in a transaction.
func (r *Repository) SetUserStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.User, error) {
	row, err := r.q.WithTx(tx).SetUserStatus(ctx, sqlcgen.SetUserStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return domain.User{}, mapErr(err)
	}
	return fromUpdateRow(row.ID, row.Email, row.Phone, row.Role, row.EmployeeID, row.CompanyID,
		row.Status, row.FullName, row.LastLoginAt, row.CreatedAt, row.UpdatedAt), nil
}

// RevokeUserSessions bumps the session epoch and revokes all refresh tokens for a
// user (F2.7 instant revocation), in the caller's tx.
func (r *Repository) RevokeUserSessions(ctx context.Context, tx pgx.Tx, userID string) error {
	q := r.q.WithTx(tx)
	if err := q.BumpTokensValidAfter(ctx, userID); err != nil {
		return mapErr(err)
	}
	return mapErr(q.RevokeAllRefreshForUser(ctx, userID))
}

// InsertResetToken reuses the identity reset-token mechanism (sha256 hash, 1h TTL).
func (r *Repository) InsertResetToken(ctx context.Context, tx pgx.Tx, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.q.WithTx(tx).InsertResetToken(ctx, sqlcgen.InsertResetTokenParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	return mapErr(err)
}

// ListAuditLog returns a page of audit-log entries matching the filter.
func (r *Repository) ListAuditLog(ctx context.Context, f domain.AuditFilter) ([]domain.AuditEntry, error) {
	rows, err := r.q.ListAuditLog(ctx, sqlcgen.ListAuditLogParams{
		ActorUserID:     f.ActorUserID,
		Action:          f.Action,
		EntityType:      f.EntityType,
		EntityID:        f.EntityID,
		CreatedGte:      f.CreatedGTE,
		CreatedLte:      f.CreatedLTE,
		Q:               f.Q,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.AuditEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAuditEntry(row))
	}
	return out, nil
}

// GetAuditLogByID fetches a single audit-log entry by SWP-AL id.
func (r *Repository) GetAuditLogByID(ctx context.Context, id string) (domain.AuditEntry, error) {
	row, err := r.q.GetAuditLogByID(ctx, id)
	if err != nil {
		return domain.AuditEntry{}, mapErr(err)
	}
	return toAuditEntry(row), nil
}

// ListPlatformSettings returns all 7 locked v1 platform settings (sorted).
func (r *Repository) ListPlatformSettings(ctx context.Context) ([]domain.PlatformSetting, error) {
	rows, err := r.q.ListPlatformSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PlatformSetting, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.PlatformSetting{
			Key:    row.Key,
			Value:  row.Value,
			Label:  row.Label,
			Locked: row.Locked,
		})
	}
	return out, nil
}

// --- mapping helpers ---

func toAuditEntry(row sqlcgen.AuditLog) domain.AuditEntry {
	return domain.AuditEntry{
		ID:          row.ID,
		ActorUserID: row.ActorUserID,
		ActorRole:   row.ActorRole,
		Action:      row.Action,
		EntityType:  row.EntityType,
		EntityID:    row.EntityID,
		Before:      jsonbToMap(row.BeforeState),
		After:       jsonbToMap(row.AfterState),
		RequestID:   row.RequestID,
		CreatedAt:   row.CreatedAt,
	}
}

// fromUpdateRow converts the shared management-row fields returned by
// UpdateUserEmail/ChangeUserRole/SetUserStatus into a domain.User.
func fromUpdateRow(
	id string,
	email, phone *string,
	role string,
	employeeID, companyID *string,
	status, fullName string,
	lastLoginAt *time.Time,
	createdAt, updatedAt time.Time,
) domain.User {
	return domain.User{
		ID:          id,
		Email:       derefStr(email),
		Phone:       derefStr(phone),
		Role:        auth.Role(role),
		EmployeeID:  derefStr(employeeID),
		CompanyID:   derefStr(companyID),
		Status:      status,
		FullName:    fullName,
		LastLoginAt: lastLoginAt,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

// jsonbToMap deserializes a JSONB []byte into map[string]any.
// Returns nil map when the DB column is NULL.
func jsonbToMap(b []byte) map[string]any {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
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
