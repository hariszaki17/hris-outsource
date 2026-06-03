// Package identity (handler) is the HTTP boundary for E1 auth. It supports two
// refresh-token transports off the same endpoints:
//   - cookie  (web SPA): refresh token in an httpOnly, SameSite=Lax cookie
//     scoped to the shared parent domain (*.swp.example.com).
//   - bearer  (mobile):  refresh token in the JSON body / response body.
//
// The client selects with the X-Auth-Transport header (default: cookie).
//
// These endpoints are intentionally hand-written (cookie handling is bespoke);
// resource endpoints in other epics implement the oapi-codegen ServerInterface.
package identity

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/service/identity"
)

const (
	refreshCookieName = "swp_refresh"
	cookiePath        = "/api/v1/auth"
	transportBearer   = "bearer"
)

// CookieConfig controls the refresh cookie attributes (from platform/config.Auth).
type CookieConfig struct {
	Domain string
	Secure bool
}

type Handler struct {
	svc    *identity.Service
	cookie CookieConfig
}

func NewHandler(svc *identity.Service, cookie CookieConfig) *Handler {
	return &Handler{svc: svc, cookie: cookie}
}

// Login handles POST /auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if fields := validateLogin(req); fields != nil {
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	res, err := h.svc.Login(r.Context(), req.Email, req.Password, r.UserAgent(), httpx.ClientIP(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.writeTokens(w, r, res)
}

// Refresh handles POST /auth/refresh. The refresh token comes from the cookie
// (web) or the request body (mobile).
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	token := h.readRefreshToken(r)
	if token == "" {
		httpx.WriteError(w, r, apperr.Unauthenticated())
		return
	}
	res, err := h.svc.Refresh(r.Context(), token, r.UserAgent(), httpx.ClientIP(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.writeTokens(w, r, res)
}

// Logout handles POST /auth/logout — revokes the presented refresh token and
// clears the web cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if token := h.readRefreshToken(r); token != "" {
		if err := h.svc.Logout(r.Context(), token); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /auth/me — echoes the authenticated principal (needs auth).
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.Unauthenticated())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, userDTO{
		ID:         p.UserID,
		Role:       string(p.Role),
		EmployeeID: p.EmployeeID,
		CompanyID:  p.CompanyID,
	})
}

// writeTokens emits the access token in the body and the refresh token via the
// transport the client asked for.
func (h *Handler) writeTokens(w http.ResponseWriter, r *http.Request, res identity.Result) {
	bearer := r.Header.Get("X-Auth-Transport") == transportBearer
	if bearer {
		h.clearCookie(w) // ensure no stale web cookie lingers for a mobile client
	} else {
		h.setCookie(w, res.RefreshToken, res.RefreshExpiresAt)
	}
	httpx.WriteJSON(w, http.StatusOK, toTokenResponse(res, bearer))
}

func (h *Handler) readRefreshToken(r *http.Request) string {
	if c, err := r.Cookie(refreshCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	var req refreshRequest
	if err := decodeJSON(r, &req); err == nil {
		return req.RefreshToken
	}
	return ""
}

func (h *Handler) setCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     cookiePath,
		Domain:   h.cookie.Domain,
		Expires:  expires,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode, // web+api are same-site (shared parent domain)
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     cookiePath,
		Domain:   h.cookie.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.Invalid(nil).WithCause(err)
	}
	return nil
}

func validateLogin(req loginRequest) map[string]string {
	fields := map[string]string{}
	if req.Email == "" {
		fields["email"] = "required"
	}
	if req.Password == "" {
		fields["password"] = "required"
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}
