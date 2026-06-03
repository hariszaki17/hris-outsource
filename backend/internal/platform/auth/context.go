// Package auth issues and verifies access tokens (stateless EdDSA JWT) and
// manages rotating opaque refresh tokens (stored hashed in Postgres). It also
// provides the authentication middleware that puts the caller's Principal into
// the request context. Authorization (roles/scope) lives in package rbac.
package auth

import "context"

// Role mirrors the four roles in CONVENTIONS §17 / CLAUDE.md.
type Role string

const (
	RoleSuperAdmin  Role = "super_admin"
	RoleHRAdmin     Role = "hr_admin"
	RoleShiftLeader Role = "shift_leader"
	RoleAgent       Role = "agent"
)

// Principal is the authenticated caller, derived from a verified access token.
// CompanyID is set only for shift_leader (their single assigned company) and
// drives `scope: company` enforcement. EmployeeID is set for staff who have an
// employee record (everyone except a bare super_admin login).
type Principal struct {
	UserID     string // SWP-USR-…
	EmployeeID string // SWP-EMP-… (may be empty)
	Role       Role
	CompanyID  string // SWP-CMP-… (shift_leader only)
}

type ctxKey int

const principalKey ctxKey = iota

// WithPrincipal stores the caller in context.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// PrincipalFrom returns the caller and whether one is present.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}
