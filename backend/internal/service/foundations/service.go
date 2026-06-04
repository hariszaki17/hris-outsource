// Package foundations is the E1 foundations service: user management (list,
// create, update, change-role, deactivate, reactivate, send-password-reset),
// audit-log read, and platform settings. It mirrors the identity service pattern:
// repository port defined here (consumer-defined), writes in txm.InTx + audit.Record.
package foundations

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// Repository is the data dependency, defined by this consumer (Go idiom). The
// repository layer in internal/repository/foundations implements it over sqlc.
type Repository interface {
	// Reads run on the pool.
	ListUsers(ctx context.Context, f domain.UserFilter) ([]domain.User, error)
	GetUserByID(ctx context.Context, id string) (domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	ListAuditLog(ctx context.Context, f domain.AuditFilter) ([]domain.AuditEntry, error)
	GetAuditLogByID(ctx context.Context, id string) (domain.AuditEntry, error)
	ListPlatformSettings(ctx context.Context) ([]domain.PlatformSetting, error)
	// Writes take the active transaction.
	CreateUser(ctx context.Context, tx pgx.Tx, p CreateUserParams) (domain.User, error)
	UpdateUserEmail(ctx context.Context, tx pgx.Tx, id, email string) (domain.User, error)
	ChangeUserRole(ctx context.Context, tx pgx.Tx, id, role string) (domain.User, error)
	SetUserStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.User, error)
	InsertResetToken(ctx context.Context, tx pgx.Tx, userID, tokenHash string, expiresAt time.Time) error
}

// CreateUserParams carries the fields for inserting a new user.
type CreateUserParams struct {
	Email        string
	PasswordHash string
	Role         string
	FullName     string
	EmployeeID   string
}

// TxRunner is a thin interface over db.TxManager (injectable for tests).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock is injectable for deterministic tests; defaults to time.Now.
type Clock func() time.Time

// Service implements the E1 foundations business logic.
type Service struct {
	repo Repository
	txm  TxRunner
	now  Clock
}

// NewService wires the service with its dependencies.
func NewService(repo Repository, txm TxRunner) *Service {
	return &Service{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *Service) SetClock(c Clock) { s.now = c }

// pageCursor is the opaque JSON payload encoded into the cursor string.
// "c" = created_at (compact key), "i" = id.
type pageCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// --- Valid role values (CONVENTIONS §17) ---

var validRoles = map[string]bool{
	string(auth.RoleSuperAdmin):  true,
	string(auth.RoleHRAdmin):     true,
	string(auth.RoleShiftLeader): true,
	string(auth.RoleAgent):       true,
}

// ListUsers returns a cursor-paginated page of users.
func (s *Service) ListUsers(ctx context.Context, f domain.UserFilter) ([]domain.User, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1 // fetch one extra to detect has_more

	// Lowercase the status filter before the query (DB stores lowercase).
	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		f.Status = &lower
	}

	rows, err := s.repo.ListUsers(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// CreateUser inserts a new user, enforces email uniqueness (409 CONFLICT), and
// audits the creation. If sendInvite is true, a reset token is also created
// (reusing the identity reset-token mechanism) so invite/reset flows work.
func (s *Service) CreateUser(ctx context.Context, email, role, employeeID string, sendInvite bool) (domain.User, error) {
	if !validRoles[role] {
		return domain.User{}, apperr.Rule("ROLE_NOT_ALLOWED", map[string]string{
			"role": "Nilai peran tidak valid.",
		})
	}

	// Email uniqueness pre-check.
	_, err := s.repo.GetUserByEmail(ctx, email)
	if err == nil {
		// Found — conflict.
		return domain.User{}, apperr.Conflict("CONFLICT")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, apperr.Internal(err)
	}

	// Generate a placeholder password (user must reset via invite link).
	_, placeholder := auth.NewRefreshToken()
	pwHash := placeholder // send_invitation_email flow: the hash is never used directly

	var created domain.User
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateUser(ctx, tx, CreateUserParams{
			Email:        email,
			PasswordHash: pwHash,
			Role:         role,
			FullName:     "",
			EmployeeID:   employeeID,
		})
		if inErr != nil {
			return inErr
		}

		if err := audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "user",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"email": created.Email,
				"role":  string(created.Role),
			},
		}); err != nil {
			return err
		}

		if sendInvite {
			plaintext, hash := auth.NewRefreshToken()
			_ = plaintext // E2E reads from DB; no mailer in this phase
			expiresAt := s.now().Add(time.Hour)
			if err := s.repo.InsertResetToken(ctx, tx, created.ID, hash, expiresAt); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return domain.User{}, err
	}

	return created, nil
}

// UpdateUser patches the user's email and audits the change.
func (s *Service) UpdateUser(ctx context.Context, id, email string) (domain.User, error) {
	current, err := s.repo.GetUserByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, apperr.NotFound()
	}
	if err != nil {
		return domain.User{}, apperr.Internal(err)
	}

	var updated domain.User
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdateUserEmail(ctx, tx, id, email)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "user",
			EntityID:   id,
			Before:     map[string]any{"email": current.Email},
			After:      map[string]any{"email": updated.Email},
		})
	}); err != nil {
		return domain.User{}, err
	}

	return updated, nil
}

// ChangeUserRole changes the user's role, validates the new role, and audits.
func (s *Service) ChangeUserRole(ctx context.Context, id, newRole, reason string) (domain.User, error) {
	if !validRoles[newRole] {
		return domain.User{}, apperr.Rule("ROLE_NOT_ALLOWED", map[string]string{
			"new_role": "Nilai peran tidak valid.",
		})
	}

	current, err := s.repo.GetUserByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, apperr.NotFound()
	}
	if err != nil {
		return domain.User{}, apperr.Internal(err)
	}

	var updated domain.User
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.ChangeUserRole(ctx, tx, id, newRole)
		if inErr != nil {
			return inErr
		}
		afterSnap := map[string]any{"role": newRole}
		if reason != "" {
			afterSnap["reason"] = reason
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("user.change_role"),
			EntityType: "user",
			EntityID:   id,
			Before:     map[string]any{"role": string(current.Role)},
			After:      afterSnap,
		})
	}); err != nil {
		return domain.User{}, err
	}

	return updated, nil
}

// DeactivateUser sets status to 'disabled'. Returns 409 CONFLICT if already disabled.
// Session revocation is auth-side and out of scope for this phase.
func (s *Service) DeactivateUser(ctx context.Context, id, reason string) (domain.User, error) {
	current, err := s.repo.GetUserByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, apperr.NotFound()
	}
	if err != nil {
		return domain.User{}, apperr.Internal(err)
	}
	if current.Status == "disabled" {
		return domain.User{}, apperr.Conflict("CONFLICT")
	}

	var updated domain.User
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetUserStatus(ctx, tx, id, "disabled")
		if inErr != nil {
			return inErr
		}
		afterSnap := map[string]any{"status": "disabled"}
		if reason != "" {
			afterSnap["reason"] = reason
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("user.deactivate"),
			EntityType: "user",
			EntityID:   id,
			Before:     map[string]any{"status": current.Status},
			After:      afterSnap,
		})
	}); err != nil {
		return domain.User{}, err
	}

	return updated, nil
}

// ReactivateUser sets status to 'active'. Returns 409 CONFLICT if already active.
func (s *Service) ReactivateUser(ctx context.Context, id, reason string) (domain.User, error) {
	current, err := s.repo.GetUserByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, apperr.NotFound()
	}
	if err != nil {
		return domain.User{}, apperr.Internal(err)
	}
	if current.Status == "active" {
		return domain.User{}, apperr.Conflict("CONFLICT")
	}

	var updated domain.User
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetUserStatus(ctx, tx, id, "active")
		if inErr != nil {
			return inErr
		}
		afterSnap := map[string]any{"status": "active"}
		if reason != "" {
			afterSnap["reason"] = reason
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("user.reactivate"),
			EntityType: "user",
			EntityID:   id,
			Before:     map[string]any{"status": current.Status},
			After:      afterSnap,
		})
	}); err != nil {
		return domain.User{}, err
	}

	return updated, nil
}

// SendUserPasswordReset creates a reset token for the user (reusing the identity
// mechanism). No mailer is wired in this phase — the E2E harness reads the token
// directly from the password_reset_tokens table.
func (s *Service) SendUserPasswordReset(ctx context.Context, id string) (string, error) {
	user, err := s.repo.GetUserByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return "", apperr.NotFound()
	}
	if err != nil {
		return "", apperr.Internal(err)
	}

	_, hash := auth.NewRefreshToken()
	expiresAt := s.now().Add(time.Hour)

	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.InsertResetToken(ctx, tx, user.ID, hash, expiresAt); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("user.send_password_reset"),
			EntityType: "user",
			EntityID:   user.ID,
			Before:     nil,
			After:      map[string]any{"action": "password_reset_requested"},
		})
	}); err != nil {
		return "", err
	}

	return user.Email, nil
}

// ListAuditLog returns a cursor-paginated page of audit-log entries.
func (s *Service) ListAuditLog(ctx context.Context, f domain.AuditFilter) ([]domain.AuditEntry, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1

	rows, err := s.repo.ListAuditLog(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// GetAuditLogEntry fetches a single audit-log entry by id.
func (s *Service) GetAuditLogEntry(ctx context.Context, id string) (domain.AuditEntry, error) {
	entry, err := s.repo.GetAuditLogByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.AuditEntry{}, apperr.NotFound()
	}
	if err != nil {
		return domain.AuditEntry{}, apperr.Internal(err)
	}
	return entry, nil
}

// GetPlatformSettings returns all 7 platform-settings rows.
func (s *Service) GetPlatformSettings(ctx context.Context) ([]domain.PlatformSetting, error) {
	settings, err := s.repo.ListPlatformSettings(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return settings, nil
}
