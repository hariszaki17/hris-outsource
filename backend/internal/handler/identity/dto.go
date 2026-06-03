package identity

import (
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/service/identity"
)

// loginRequest is the POST /auth/login body.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// refreshRequest is the POST /auth/refresh body (mobile/bearer transport). Web
// sends the refresh token in the httpOnly cookie instead, so this is optional.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// tokenResponse is returned by login/refresh. refresh_token is populated ONLY
// for the bearer (mobile) transport; for the cookie (web) transport it is empty
// and the token rides in the Set-Cookie header.
type tokenResponse struct {
	AccessToken      string    `json:"access_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	User             userDTO   `json:"user"`
}

type userDTO struct {
	ID         string `json:"id"`
	Role       string `json:"role"`
	EmployeeID string `json:"employee_id,omitempty"`
	CompanyID  string `json:"company_id,omitempty"`
}

func toTokenResponse(r identity.Result, includeRefreshInBody bool) tokenResponse {
	resp := tokenResponse{
		AccessToken:      r.AccessToken,
		AccessExpiresAt:  r.AccessExpiresAt,
		RefreshExpiresAt: r.RefreshExpiresAt,
		User: userDTO{
			ID:         r.Principal.UserID,
			Role:       string(r.Principal.Role),
			EmployeeID: r.Principal.EmployeeID,
			CompanyID:  r.Principal.CompanyID,
		},
	}
	if includeRefreshInBody {
		resp.RefreshToken = r.RefreshToken
	}
	return resp
}
