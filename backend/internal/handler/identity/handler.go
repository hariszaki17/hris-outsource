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
	svc          *identity.Service
	cookie       CookieConfig
	accessTTLSec int // used to emit expires_in (int seconds) per spec
}

func NewHandler(svc *identity.Service, cookie CookieConfig, accessTTL time.Duration) *Handler {
	return &Handler{
		svc:          svc,
		cookie:       cookie,
		accessTTLSec: int(accessTTL.Seconds()),
	}
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

	res, err := h.svc.Login(r.Context(), req.Identifier, req.Password, r.UserAgent(), httpx.ClientIP(r))
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

	bearer := r.Header.Get("X-Auth-Transport") == transportBearer
	if bearer {
		h.clearCookie(w)
	} else {
		h.setCookie(w, res.RefreshToken, res.RefreshExpiresAt)
	}
	// Refresh returns the slim RefreshResponse (no user field required by spec).
	httpx.WriteJSON(w, http.StatusOK, toRefreshResponse(res, bearer, h.accessTTLSec))
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

// Me handles GET /auth/me — loads the full user from DB (so email, full_name,
// status, scope are always fresh) and emits the spec-shaped MeResponse.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.Unauthenticated())
		return
	}
	user, err := h.svc.Me(r.Context(), p.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resp := meFromUser(user)
	// GAP 3: a shift_leader's scope.company_id is derived at request time (the auth
	// middleware put the live E3 leader-assignment company on the principal), not read
	// from the possibly-stale users.company_id — so /auth/me reflects a reassignment
	// immediately, consistent with the authorization gate.
	if p.Role == auth.RoleShiftLeader {
		resp.Scope = scopeFromRole(p.Role, p.CompanyID)
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// ForgotPassword handles POST /auth/forgot-password.
// Per C-2 of authentication.md the response is ALWAYS 202 with the same generic
// message, regardless of whether the email is registered (anti-enumeration).
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Email == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"email": "Wajib diisi."}))
		return
	}
	// Intentionally ignore any error (including not-found) — anti-enumeration.
	_ = h.svc.ForgotPassword(r.Context(), req.Email)

	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{
		"message": "Jika email terdaftar, tautan reset telah dikirim.",
	})
}

// ResetPassword handles POST /auth/reset-password.
// 204 on success; 401 RESET_TOKEN_EXPIRED for invalid/expired/used tokens;
// 422 WEAK_PASSWORD for policy violations.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	fields := map[string]string{}
	if req.ResetToken == "" {
		fields["reset_token"] = "Wajib diisi."
	}
	if req.NewPassword == "" {
		fields["new_password"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		httpx.WriteError(w, r, apperr.Invalid(fields))
		return
	}

	if err := h.svc.ResetPassword(r.Context(), req.ResetToken, req.NewPassword); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword handles POST /auth/change-password (authenticated). Sets a new
// password for the current user — the EP-3 forced temp-password rotation and
// voluntary changes. 204 on success; 422 WEAK_PASSWORD on policy violation.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, apperr.Unauthenticated())
		return
	}
	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.NewPassword == "" {
		httpx.WriteError(w, r, apperr.Invalid(map[string]string{"new_password": "Wajib diisi."}))
		return
	}
	if err := h.svc.ChangeOwnPassword(r.Context(), p.UserID, req.NewPassword); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeTokens emits the login response with the access token in the body and the
// refresh token via the transport the client asked for.
func (h *Handler) writeTokens(w http.ResponseWriter, r *http.Request, res identity.Result) {
	bearer := r.Header.Get("X-Auth-Transport") == transportBearer
	if bearer {
		h.clearCookie(w) // ensure no stale web cookie lingers for a mobile client
	} else {
		h.setCookie(w, res.RefreshToken, res.RefreshExpiresAt)
	}
	httpx.WriteJSON(w, http.StatusOK, toLoginResponse(res, bearer, h.accessTTLSec))
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
	if req.Identifier == "" {
		fields["identifier"] = "required"
	}
	if req.Password == "" {
		fields["password"] = "required"
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}
