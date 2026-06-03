package auth

import (
	"net/http"
	"strings"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// Authenticator verifies the Bearer access token and injects the Principal.
// Mount it on all routes except the public ones (login, forgot-password).
type Authenticator struct {
	issuer *Issuer
}

func NewAuthenticator(issuer *Issuer) *Authenticator { return &Authenticator{issuer: issuer} }

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
