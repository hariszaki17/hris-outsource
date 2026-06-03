// Package server assembles the chi router: the global middleware chain, health
// and metrics endpoints, and the /api/v1 routes (public vs authenticated). New
// epics mount their (oapi-codegen) routers under the authenticated group.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	identityhttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/identity"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/idempotency"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/obs"
)

// Deps are everything the router needs, built in cmd/api.
type Deps struct {
	AllowedOrigins []string
	RatePerMinute  int
	RateBurst      int

	Auth        *identityhttp.Handler
	Authn       *auth.Authenticator
	Idempotency *idempotency.Middleware
	Obs         *obs.Providers
}

// New builds the root HTTP handler.
func New(d Deps) http.Handler {
	r := chi.NewRouter()

	// Global chain (order matters): request id -> access log -> panic recovery
	// -> secure headers -> CORS. otelhttp wraps the whole handler at the end.
	r.Use(httpx.RequestIDMiddleware)
	r.Use(httpx.AccessLog)
	r.Use(httpx.Recover)
	r.Use(httpx.SecureHeaders)
	r.Use(httpx.CORS(d.AllowedOrigins))

	// Ops endpoints (no auth, not under /api/v1).
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Handle("/metrics", d.Obs.MetricsHandler())

	rl := httpx.NewRateLimiter(d.RatePerMinute, d.RateBurst, auth.RateLimitKey)

	r.Route("/api/v1", func(r chi.Router) {
		// --- Public (CONVENTIONS §1: only login + forgot-password are public;
		//     refresh/logout carry their own refresh credential, not an access token).
		r.Group(func(r chi.Router) {
			r.Use(rl.Middleware) // keyed by IP for unauthenticated calls
			r.Post("/auth/login", d.Auth.Login)
			r.Post("/auth/refresh", d.Auth.Refresh)
			r.Post("/auth/logout", d.Auth.Logout)
			r.Post("/auth/forgot-password", d.Auth.ForgotPassword)
			r.Post("/auth/reset-password", d.Auth.ResetPassword)
		})

		// --- Authenticated: access token required, then per-user rate limit.
		r.Group(func(r chi.Router) {
			r.Use(d.Authn.Require)
			r.Use(rl.Middleware)
			r.Get("/auth/me", d.Auth.Me)

			// Mount resource epics here, e.g.:
			//   e2.HandlerFromMux(e2impl, r)        // oapi-codegen ServerInterface
			//   wrapped with rbac.RequireRole(...) and d.Idempotency.Handler where flagged.
			_ = d.Idempotency // used by write endpoints as they land
		})
	})

	return otelhttp.NewHandler(r, "http.server")
}
