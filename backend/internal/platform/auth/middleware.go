package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// UserStateFunc reports a user's current status and session-epoch (tokens_valid_after)
// for the F2.7 per-request revocation check. When nil, the middleware is the original
// stateless JWT verify (used by unit tests).
type UserStateFunc func(ctx context.Context, userID string) (status string, tokensValidAfter time.Time, err error)

// CompanyResolverFunc returns the company an employee currently LEADS via an active
// E3 shift_leader_assignment, or an error (incl. not-found) when they lead none. It
// is the single source of truth for shift-leader identity: the middleware derives
// both the effective ROLE and the company SCOPE from it at request time, so the
// stored users.role / users.company_id and the JWT `cmp` claim are advisory. An
// employee with an active assignment IS a shift_leader scoped to that company;
// without one they are a plain agent. Reassigning/ending a leader therefore takes
// effect on their next request — no re-login, no drift.
type CompanyResolverFunc func(ctx context.Context, employeeID string) (companyID string, err error)

// Authenticator verifies the Bearer access token and injects the Principal.
// Mount it on all routes except the public ones (login, forgot-password).
type Authenticator struct {
	issuer          *Issuer
	userState       UserStateFunc
	companyResolver CompanyResolverFunc
}

func NewAuthenticator(issuer *Issuer) *Authenticator { return &Authenticator{issuer: issuer} }

// WithUserState wires the F2.7 per-request check: a verified token is additionally
// rejected if the user is not active, or if the token was issued before the user's
// session epoch (instant revocation on offboard/disable). Returns the receiver for
// chaining.
func (a *Authenticator) WithUserState(fn UserStateFunc) *Authenticator {
	a.userState = fn
	return a
}

// WithCompanyResolver wires read-time shift_leader company-scope derivation (GAP 3):
// the verified Principal's CompanyID is overwritten from the live E3 leader-assignment
// instead of trusting the (possibly stale) JWT `cmp` claim. Returns the receiver for
// chaining.
func (a *Authenticator) WithCompanyResolver(fn CompanyResolverFunc) *Authenticator {
	a.companyResolver = fn
	return a
}

// Require is middleware that rejects unauthenticated requests with 401
// (CONVENTIONS §3: missing/expired token -> UNAUTHENTICATED, client re-auths).
func (a *Authenticator) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			httpx.WriteError(w, r, apperr.Unauthenticated())
			return
		}
		p, err := a.issuer.Verify(token)
		if err != nil {
			httpx.WriteError(w, r, apperr.Unauthenticated().WithCause(err))
			return
		}
		// F2.7: stateful revocation check. Reject a cryptographically-valid token if the
		// user was disabled or their session epoch advanced past this token's iat.
		if a.userState != nil {
			status, validAfter, err := a.userState(r.Context(), p.UserID)
			if err != nil || status != "active" || p.IssuedAt.Before(validAfter) {
				httpx.WriteError(w, r, apperr.Unauthenticated())
				return
			}
		}
		// Read-time shift-leader derivation: a field employee's effective role AND
		// company scope come from the live E3 leader-assignment, never the stored
		// users.role / users.company_id / `cmp` claim. An employee (stored agent or
		// shift_leader) with an active assignment IS a shift_leader scoped to that
		// company; without one they are a plain agent. Staff roles (super_admin /
		// hr_admin) are global and never derived. On resolver error scope is stripped
		// and the role falls back to agent — fail-safe, never an escalation.
		if a.companyResolver != nil && (p.Role == RoleShiftLeader || p.Role == RoleAgent) {
			companyID, err := a.companyResolver(r.Context(), p.EmployeeID)
			if err != nil || companyID == "" {
				p.Role = RoleAgent
				p.CompanyID = ""
			} else {
				p.Role = RoleShiftLeader
				p.CompanyID = companyID
			}
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
	})
}

// RateLimitKey returns a per-user limiter key (subject) when authenticated, else
// the client IP. Wired into httpx.RateLimiter so this package owns the "who is
// the caller" logic without httpx depending on auth. Mount the rate limiter
// AFTER Require on protected routes so the verified Principal is in context.
func RateLimitKey(r *http.Request) string {
	if p, ok := PrincipalFrom(r.Context()); ok && p.UserID != "" {
		return "u:" + p.UserID
	}
	return "ip:" + httpx.ClientIP(r)
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):]), true
	}
	return "", false
}
