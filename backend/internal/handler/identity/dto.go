package identity

import (
	"strings"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/service/identity"
)

// loginRequest is the POST /auth/login body.
type loginRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	StaySignedIn bool  `json:"stay_signed_in"`
}

// refreshRequest is the POST /auth/refresh body (mobile/bearer transport). Web
// sends the refresh token in the httpOnly cookie instead, so this is optional.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// forgotPasswordRequest is the POST /auth/forgot-password body.
type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// resetPasswordRequest is the POST /auth/reset-password body.
type resetPasswordRequest struct {
	ResetToken  string `json:"reset_token"`
	NewPassword string `json:"new_password"`
}

// scopeDTO is the MeResponse.scope field (CONVENTIONS §17).
type scopeDTO struct {
	Type      string  `json:"type"`                // global | company | self
	CompanyID *string `json:"company_id"`          // non-null only for shift_leader
}

// meResponse is the MeResponse schema from the OpenAPI spec.
// Returned by GET /auth/me and embedded in loginResponse.user.
type meResponse struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`       // UPPERCASE per spec (ACTIVE | DISABLED)
	EmployeeID  string     `json:"employee_id"`
	FullName    string     `json:"full_name"`
	LastLoginAt *time.Time `json:"last_login_at"` // RFC3339 UTC; null on first login
	Scope       scopeDTO   `json:"scope"`
}

// loginResponse is the LoginResponse schema from the OpenAPI spec.
// Returned by POST /auth/login and POST /auth/refresh.
type loginResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token,omitempty"` // body only for bearer transport
	TokenType    string      `json:"token_type"`               // always "Bearer"
	ExpiresIn    int         `json:"expires_in"`               // access TTL in seconds
	User         meResponse  `json:"user"`
}

// refreshResponse is the RefreshResponse schema — same as loginResponse but
// without the full user object (spec only requires access_token + token_type + expires_in).
// We reuse loginResponse because the spec allows user to be absent in the schema
// but our implementation returns it anyway for convenience. The spec only requires
// the three fields for refresh, but including user is strictly additive and harmless.
// (Spec says refresh_token MAY be present if rotated; user field not listed as required.)
// Use a minimal struct to match the spec exactly for refresh.
type refreshResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token,omitempty"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int         `json:"expires_in"`
}

// meFromUser maps a domain.User to the MeResponse DTO, computing scope from role.
func meFromUser(u domain.User) meResponse {
	scope := scopeFromRole(u.Role, u.CompanyID)
	return meResponse{
		ID:          u.ID,
		Email:       u.Email,
		Role:        string(u.Role),
		Status:      strings.ToUpper(u.Status), // "active" → "ACTIVE" per spec
		EmployeeID:  u.EmployeeID,
		FullName:    u.FullName,
		LastLoginAt: u.LastLoginAt,
		Scope:       scope,
	}
}

// scopeFromRole derives the RBAC scope from the user's role:
//   - super_admin / hr_admin → { type: "global", company_id: null }
//   - shift_leader           → { type: "company", company_id: <users.company_id> }
//   - agent                  → { type: "self",    company_id: null }
func scopeFromRole(role auth.Role, companyID string) scopeDTO {
	switch role {
	case auth.RoleShiftLeader:
		var cid *string
		if companyID != "" {
			cid = &companyID
		}
		return scopeDTO{Type: "company", CompanyID: cid}
	case auth.RoleAgent:
		return scopeDTO{Type: "self", CompanyID: nil}
	default: // super_admin, hr_admin
		return scopeDTO{Type: "global", CompanyID: nil}
	}
}

// toLoginResponse builds the spec-conformant login body from a service Result.
// accessTTL is used to derive expires_in (int seconds), ensuring the field is
// an integer as the spec requires — not a computed timestamp difference.
func toLoginResponse(r identity.Result, includeRefreshInBody bool, accessTTLSecs int) loginResponse {
	resp := loginResponse{
		AccessToken: r.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   accessTTLSecs,
		User:        meFromUser(r.User),
	}
	if includeRefreshInBody {
		resp.RefreshToken = r.RefreshToken
	}
	return resp
}

// toRefreshResponse builds the minimal RefreshResponse body.
func toRefreshResponse(r identity.Result, includeRefreshInBody bool, accessTTLSecs int) refreshResponse {
	resp := refreshResponse{
		AccessToken: r.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   accessTTLSecs,
	}
	if includeRefreshInBody {
		resp.RefreshToken = r.RefreshToken
	}
	return resp
}
