package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mintSL issues a valid shift_leader access token carrying a (deliberately stale)
// cmp claim, to prove the middleware ignores it in favour of the resolver.
func mintSL(t *testing.T, iss *Issuer, staleCompany string) string {
	t.Helper()
	tok, _, err := iss.Issue(Principal{
		UserID:     "SWP-USR-SL",
		EmployeeID: "SWP-EMP-SL",
		Role:       RoleShiftLeader,
		CompanyID:  staleCompany, // baked into cmp at mint
	}, time.Now())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return tok
}

func newTestIssuer(t *testing.T) *Issuer {
	t.Helper()
	priv, pub, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	iss, err := NewIssuer(priv, pub, 30*time.Minute)
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	return iss
}

// captures the Principal the handler actually sees.
func capturePrincipal(seen *Principal) http.Handler {
	return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if p, ok := PrincipalFrom(r.Context()); ok {
			*seen = p
		}
	})
}

func doAuthed(a *Authenticator, seen *Principal, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	a.Require(capturePrincipal(seen)).ServeHTTP(rr, req)
	return rr
}

// GAP 3: a shift_leader's CompanyID is derived at request time from the resolver
// (live E3 assignment), overriding the stale cmp claim.
func TestRequire_ShiftLeaderCompanyDerivedFromResolver(t *testing.T) {
	iss := newTestIssuer(t)
	a := NewAuthenticator(iss).WithCompanyResolver(
		func(_ context.Context, employeeID string) (string, error) {
			if employeeID != "SWP-EMP-SL" {
				t.Fatalf("resolver got employee %q", employeeID)
			}
			return "SWP-CMP-FRESH", nil
		},
	)

	var seen Principal
	rr := doAuthed(a, &seen, mintSL(t, iss, "SWP-CMP-STALE"))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected pass-through 200, got %d", rr.Code)
	}
	if seen.CompanyID != "SWP-CMP-FRESH" {
		t.Errorf("CompanyID = %q, want derived SWP-CMP-FRESH (cmp claim must not win)", seen.CompanyID)
	}
}

// Fail-safe: resolver error (e.g. leader has no active assignment) strips scope to
// "" so GuardCompany denies every company-scoped action — never an escalation.
func TestRequire_ShiftLeaderNoAssignmentStripsScope(t *testing.T) {
	iss := newTestIssuer(t)
	a := NewAuthenticator(iss).WithCompanyResolver(
		func(_ context.Context, _ string) (string, error) {
			return "", context.DeadlineExceeded // any error
		},
	)

	var seen Principal
	doAuthed(a, &seen, mintSL(t, iss, "SWP-CMP-STALE"))
	if seen.CompanyID != "" {
		t.Errorf("CompanyID = %q, want empty (scope stripped on resolver error)", seen.CompanyID)
	}
}

// Non-leader roles never consult the resolver (and keep global scope).
func TestRequire_NonLeaderSkipsResolver(t *testing.T) {
	iss := newTestIssuer(t)
	called := false
	a := NewAuthenticator(iss).WithCompanyResolver(
		func(_ context.Context, _ string) (string, error) {
			called = true
			return "SWP-CMP-X", nil
		},
	)

	tok, _, err := iss.Issue(Principal{UserID: "SWP-USR-HR", Role: RoleHRAdmin}, time.Now())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	var seen Principal
	doAuthed(a, &seen, tok)
	if called {
		t.Error("resolver must not be consulted for non-shift_leader roles")
	}
	if seen.CompanyID != "" {
		t.Errorf("CompanyID = %q, want empty for HR (global scope)", seen.CompanyID)
	}
}
