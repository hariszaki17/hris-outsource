---
phase: 01-test-harness-auth
plan: "03"
subsystem: backend-auth
tags: [auth, jwt, password-reset, contract-tests, migrations, sqlc]
dependency_graph:
  requires: [01-02]
  provides: [AUTH-01, AUTH-02, AUTH-03]
  affects: [e2e-harness, frontend-generated-client]
tech_stack:
  added:
    - TxRunner interface (service layer) for testable transaction injection
  patterns:
    - fake-repo + fake-tx-runner pattern for service unit tests
    - httptest-based handler contract tests asserting exact JSON shapes
    - sha256 hex hash for password reset tokens (reuses NewRefreshToken helper)
    - anti-enumeration via identical 202 for known/unknown email in forgot-password
key_files:
  created:
    - backend/db/migrations/00006_user_profile_fields.sql
    - backend/db/migrations/00007_password_reset_tokens.sql
    - backend/db/queries/identity/password_reset_tokens.sql
    - backend/internal/repository/sqlc/password_reset_tokens.sql.go
    - backend/internal/service/identity/service_test.go
    - backend/internal/handler/identity/handler_test.go
  modified:
    - backend/db/queries/identity/users.sql
    - backend/db/queries/identity/refresh_tokens.sql
    - backend/internal/domain/identity.go
    - backend/internal/repository/identity/repository.go
    - backend/internal/service/identity/service.go
    - backend/internal/handler/identity/dto.go
    - backend/internal/handler/identity/handler.go
    - backend/internal/platform/i18n/i18n.go
    - backend/internal/server/server.go
    - backend/cmd/api/main.go
    - backend/cmd/seed/seed.go
decisions:
  - "TxRunner extracted as interface in service package to allow fake-based unit tests without testcontainers"
  - "reset token plaintext not emailed in Phase 1: E2E tests obtain it by querying password_reset_tokens via DB helper or by inserting a known token in the fake store"
  - "Password policy: min 10 chars + uppercase + lowercase + digit + symbol (matches spec AU-4)"
  - "Reset token TTL: 1 hour (standard for forgot-password flows)"
  - "Refresh endpoint returns slim RefreshResponse (no user field required); Login returns full LoginResponse with user embedded"
  - "status field uppercased in MeResponse via strings.ToUpper (DB stores lowercase 'active'/'disabled')"
metrics:
  duration_seconds: 690
  completed_date: "2026-06-03"
  tasks_completed: 3
  files_modified: 16
  files_created: 6
  tests_added: 18
key_decisions:
  - "TxRunner interface: service.TxRunner instead of *db.TxManager (testability without real DB)"
  - "Reset token email: no-op in Phase 1; E2E harness queries DB directly"
  - "Password policy: 10+ chars, all character classes (upper/lower/digit/symbol)"
---

# Phase 1 Plan 03: Auth Contract Conformance Summary

**One-liner:** Spec-conformant login/me/forgot/reset-password auth endpoints with sha256 reset tokens, last_login_at recording, session revocation, and 18 contract tests pinning JSON shapes to the OpenAPI spec.

## What Was Built

### Final LoginResponse JSON shape (POST /auth/login 200)
```json
{
  "access_token": "<Ed25519 JWT>",
  "refresh_token": "<opaque base64url>",
  "token_type": "Bearer",
  "expires_in": 1800,
  "user": {
    "id": "SWP-USR-1042",
    "email": "sari.hadi@swp.test",
    "role": "hr_admin",
    "status": "ACTIVE",
    "employee_id": "SWP-EMP-1042",
    "full_name": "Sari Hadi",
    "last_login_at": "2026-06-03T07:14:52Z",
    "scope": { "type": "global", "company_id": null }
  }
}
```

### MeResponse JSON shape (GET /auth/me 200)
Same as the `user` object above. Scope derivation:
- `hr_admin` / `super_admin` → `{ type: "global", company_id: null }`
- `shift_leader` → `{ type: "company", company_id: <users.company_id> }`
- `agent` → `{ type: "self", company_id: null }`

Status is uppercased at the handler boundary (`strings.ToUpper`) since the DB stores `"active"` / `"disabled"`.

### Reset Token Lifetime and E2E Token Access
- **Lifetime:** 1 hour (`now + 1h` set in `service.ForgotPassword`)
- **Storage:** SHA-256 hex hash of an opaque 32-byte base64url plaintext (reuses `auth.NewRefreshToken()` helper)
- **Email dispatch:** Out-of-scope in Phase 1 — no mailer is wired. The E2E spec in plan 01-05 obtains the reset token by **querying `password_reset_tokens` directly through the seeded DB**, or by injecting a known plaintext+hash into the table before calling `/auth/reset-password`. This is documented as the test-seam approach; a real mailer will be wired in a later phase.

### Password Policy (AU-4)
Enforced in `service.validatePasswordPolicy`:
- Minimum 10 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit
- At least one symbol (punctuation or unicode symbol)

Violations return `apperr.Rule("WEAK_PASSWORD", {"new_password": "..."})` → HTTP 422.

### New Migration Numbers
- `00006_user_profile_fields.sql` — adds `full_name text NOT NULL DEFAULT ''` and `last_login_at timestamptz` to `users`
- `00007_password_reset_tokens.sql` — creates `password_reset_tokens` table with `id, user_id, token_hash (sha256 hex UNIQUE), expires_at, used_at, created_at`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] seed.go CreateUserParams missing FullName**
- **Found during:** Task 1 (after sqlc regeneration added FullName to CreateUserParams)
- **Issue:** The existing seed.go used `CreateUserParams` without `FullName`; sqlc added `FullName string` to the params struct which made it a required field (zero value would work but personas got real names)
- **Fix:** Added `fullName` field to `persona` struct; populated with real names for all 4 test personas (`"Sari Hadi"`, `"Rudi Wijaya"`, `"Super Admin"`, `"Budi Santoso"`)
- **Files modified:** `backend/cmd/seed/seed.go`
- **Commit:** a807710

**2. [Rule 1 - Architecture] TxRunner interface extracted from service**
- **Found during:** Task 3 (writing service tests)
- **Issue:** `db.TxManager` is a concrete struct with no interface; tests needed to inject a no-op tx runner without a real database
- **Fix:** Added `TxRunner interface { InTx(ctx, fn) error }` to `internal/service/identity/service.go`; changed `NewService` parameter from `*db.TxManager` to `TxRunner`; `*db.TxManager` satisfies the interface (structural typing); `fakeTxRunner` in tests implements it with `fn(nil)` (fakeRepo ignores the nil pgx.Tx)
- **Files modified:** `backend/internal/service/identity/service.go`
- **Commit:** 44b110f

## Contracts Pinned by Tests

| Test | Endpoint | Assertion |
|------|----------|-----------|
| `TestLogin_SpecShape` | POST /auth/login | token_type=="Bearer", expires_in==1800, user.status=="ACTIVE", user.scope.type=="global" |
| `TestLogin_WrongPassword_401` | POST /auth/login | 401 error.code=="INVALID_CREDENTIALS" |
| `TestLogin_DisabledAccount_403` | POST /auth/login | 403 error.code=="ACCOUNT_DISABLED" |
| `TestMe_SpecShape` | GET /auth/me | all MeResponse fields present, status uppercase |
| `TestMe_ShiftLeader_CompanyScope` | GET /auth/me | scope.type=="company", company_id non-null |
| `TestForgotPassword_KnownEmail_202` | POST /auth/forgot-password | 202 + message field |
| `TestForgotPassword_UnknownEmail_202_SameBody` | POST /auth/forgot-password | identical 202 body (anti-enumeration C-2) |
| `TestForgotPassword_MissingEmail_400` | POST /auth/forgot-password | 400 on empty email |
| `TestResetPassword_Valid_204` | POST /auth/reset-password | 204 on valid token |
| `TestResetPassword_ExpiredToken_401` | POST /auth/reset-password | 401 RESET_TOKEN_EXPIRED |
| `TestResetPassword_WeakPassword_422` | POST /auth/reset-password | 422 WEAK_PASSWORD + fields.new_password |
| `TestRefresh_ValidToken_200` | POST /auth/refresh | access_token + token_type + expires_in |
| `TestLogout_204` | POST /auth/logout | 204 |

Service unit tests additionally assert: SetLastLogin called on login (AU-3), disabled account error, forgot-password creates token only for known+active users, password policy (5 variants), expired/used token rejection, full success path (UpdatePassword + MarkResetTokenUsed + RevokeAllRefreshForUser).

## Self-Check: PASSED

All key files found on disk. All task commits (a807710, 8f7e56e, 44b110f) verified in git log. `go build ./... && go vet ./...` exit 0. `go test ./internal/service/identity/... ./internal/handler/identity/...` → 2 packages, all tests pass.
