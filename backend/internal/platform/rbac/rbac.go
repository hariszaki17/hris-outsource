// Package rbac is the server-side authorization layer (CONVENTIONS §17). The
// API is the source of truth; the client's x-rbac map is only defense-in-depth.
//
// Two axes, mirroring the OpenAPI x-rbac extension:
//   - roles: which of {super_admin, hr_admin, shift_leader, agent, lead} may call
//   - scope: global | company | self | company_or_global
//
// Company scope is single-company for shift_leader (Principal.CompanyID) and
// multi-company for lead (Principal.CompanyIDs — the set of assigned client
// companies). super_admin / hr_admin are global; agent is self-scoped.
//
// Handlers declare the allowed roles (RequireRole middleware); services enforce
// row-level scope with the Guard* helpers once they know the resource's owner.
package rbac

import (
	"context"
	"net/http"
	"slices"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// RequireRole rejects callers whose role isn't in allowed (403 FORBIDDEN).
// Generated from each operation's `x-rbac.roles`.
func RequireRole(allowed ...auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, apperr.Unauthenticated())
				return
			}
			if !slices.Contains(allowed, p.Role) {
				httpx.WriteError(w, r, apperr.Forbidden())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GuardCompany enforces `scope: company` / `company_or_global`: a shift_leader
// may only touch resources in their own company; HR/super-admin pass through.
// Call this in the service once you've loaded the resource's company id.
func GuardCompany(ctx context.Context, resourceCompanyID string) error {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apperr.Unauthenticated()
	}
	switch p.Role {
	case auth.RoleSuperAdmin, auth.RoleHRAdmin:
		return nil // global
	case auth.RoleShiftLeader:
		if p.CompanyID != "" && p.CompanyID == resourceCompanyID {
			return nil
		}
		return apperr.OutOfScope()
	case auth.RoleLead:
		if slices.Contains(p.CompanyIDs, resourceCompanyID) {
			return nil
		}
		return apperr.OutOfScope()
	default:
		return apperr.OutOfScope()
	}
}

// CanApproveBank reports whether the caller holds the bank sub-permission
// `change_requests.approve.bank` (CONVENTIONS §17). Modeled role-keyed: only
// hr_admin / super_admin may finalize a bank_account change. A shift_leader
// approving a mixed request applies the non-bank fields and escalates the bank
// field to HR (PARTIALLY_APPROVED) instead of writing it.
func CanApproveBank(ctx context.Context) bool {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return false
	}
	switch p.Role {
	case auth.RoleSuperAdmin, auth.RoleHRAdmin:
		return true
	default:
		return false
	}
}

// GuardSelf enforces `scope: self`: an agent may only touch their own employee
// record; staff roles (HR/super-admin) pass through.
func GuardSelf(ctx context.Context, resourceEmployeeID string) error {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apperr.Unauthenticated()
	}
	switch p.Role {
	case auth.RoleSuperAdmin, auth.RoleHRAdmin:
		return nil
	default:
		if p.EmployeeID != "" && p.EmployeeID == resourceEmployeeID {
			return nil
		}
		return apperr.OutOfScope()
	}
}
