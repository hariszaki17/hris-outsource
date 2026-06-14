package rbac

import (
	"context"
	"testing"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

func codeOf(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected *apperr.Error, got %v", err)
	}
	return ae.Code
}

// GuardCompany for a stored `lead`: passes when the resource company is in the
// lead's resolved company SET (Principal.CompanyIDs), OUT_OF_SCOPE otherwise.
func TestGuardCompany_Lead(t *testing.T) {
	leadCtx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:     "SWP-USR-LEAD",
		EmployeeID: "SWP-EMP-3004",
		Role:       auth.RoleLead,
		CompanyIDs: []string{"SWP-CMP-0021", "SWP-CMP-0022"},
	})

	// In-set → nil.
	if err := GuardCompany(leadCtx, "SWP-CMP-0021"); err != nil {
		t.Fatalf("in-set company: unexpected err %v", err)
	}
	if err := GuardCompany(leadCtx, "SWP-CMP-0022"); err != nil {
		t.Fatalf("in-set company (2nd): unexpected err %v", err)
	}

	// Out-of-set → OUT_OF_SCOPE.
	if got := codeOf(t, GuardCompany(leadCtx, "SWP-CMP-0099")); got != "OUT_OF_SCOPE" {
		t.Fatalf("out-of-set company: code = %s, want OUT_OF_SCOPE", got)
	}
}

// A lead with an EMPTY company set (resolver error / unassigned) is denied — the
// middleware leaves CompanyIDs empty on failure, so GuardCompany fails safe.
func TestGuardCompany_LeadEmptySetDenied(t *testing.T) {
	leadCtx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:     "SWP-USR-LEAD",
		EmployeeID: "SWP-EMP-3004",
		Role:       auth.RoleLead,
		CompanyIDs: nil,
	})
	if got := codeOf(t, GuardCompany(leadCtx, "SWP-CMP-0021")); got != "OUT_OF_SCOPE" {
		t.Fatalf("empty set: code = %s, want OUT_OF_SCOPE", got)
	}
}

// super_admin / hr_admin stay global through GuardCompany regardless of company.
func TestGuardCompany_StaffGlobal(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleSuperAdmin, auth.RoleHRAdmin} {
		ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "SWP-USR-X", Role: role})
		if err := GuardCompany(ctx, "SWP-CMP-ANY"); err != nil {
			t.Fatalf("role %s should be global, got %v", role, err)
		}
	}
}
