// Package domain holds the plain, dependency-free business types shared between
// the service and repository layers (so neither has to import the other for
// types, and the generated sqlc structs never leak past the repository).
package domain

import (
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// User is a login credential + role (CONVENTIONS §4 SWP-USR).
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         auth.Role
	EmployeeID   string // "" when unset
	CompanyID    string // "" when unset (set for shift_leader)
	Status       string // active | disabled
	FullName     string // denormalized from Employee; "" for users not yet linked
	LastLoginAt  *time.Time // nil on first-ever login; set on every successful login
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u User) IsActive() bool { return u.Status == "active" }

// Principal projects a User into the token Principal (CompanyID/EmployeeID carry
// the RBAC scope).
func (u User) Principal() auth.Principal {
	return auth.Principal{
		UserID:     u.ID,
		EmployeeID: u.EmployeeID,
		Role:       u.Role,
		CompanyID:  u.CompanyID,
	}
}

// RefreshToken is a stored (hashed) refresh credential in a rotation family.
type RefreshToken struct {
	ID          int64
	UserID      string
	TokenHash   string
	FamilyID    string
	RotatedFrom *int64
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}

func (t RefreshToken) IsLive(now time.Time) bool {
	return t.RevokedAt == nil && t.ExpiresAt.After(now)
}

// PasswordResetToken is a single-use, time-limited reset credential (AU-4).
// Only the SHA-256 hash is stored in the DB; the plaintext is sent to the user
// in an email (or surfaced via the test-only DB helper).
type PasswordResetToken struct {
	ID        int64
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// IsLive returns true when the token has not been consumed and has not expired.
func (t PasswordResetToken) IsLive(now time.Time) bool {
	return t.UsedAt == nil && t.ExpiresAt.After(now)
}
