---
phase: 02-e1-foundations
plan: "02"
subsystem: backend-api
tags: [go, chi, sqlc, foundations, users, audit-log, platform-settings, rbac, cursor-pagination]

# Dependency graph
requires:
  - phase: 02-e1-foundations
    plan: "01"
    provides: "sqlc query methods (ListUsers, ListAuditLog, etc.), platform_settings migration"
  - phase: 01-test-harness-auth
    provides: "identity slice pattern, audit.Record, auth.Principal, httpx cursor helpers"
provides:
  - "GET /users + POST /users + PATCH /users/{id} handlers with RBAC + audit"
  - "POST /users/{id}:change-role / :deactivate / :reactivate / :send-password-reset handlers"
  - "GET /audit-log + GET /audit-log/{id} with cursor pagination and 7 optional filters"
  - "GET /platform/settings returning the 7-key PlatformSettings object"
  - "foundations domain types (AuditEntry, PlatformSetting, UserFilter, AuditFilter)"
  - "foundations repository, service, handler packages mirroring the identity slice"
affects: [wave-3 contract tests, wave-4 E2E, cmd/seed extra data for E1 screens]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "chi ':' action suffix routes: '/users/{user_id}:change-role' — chi matches the literal colon-suffix as part of the path; no sub-router needed"
    - "Consumer-defined Repository port in service package; var _ svc.Repository compile-check in repo"
    - "pageCursor{CreatedAt,ID} encoded via httpx.EncodeCursor / decoded via httpx.DecodeCursor in both service and handler"
    - "status mapping: DB lowercase 'active'/'disabled' -> API UPPER 'ACTIVE'/'DISABLED' in DTO; filter lowercased before query"
    - "Foundations handler decodes cursor in handler (local pageCursor mirror), passes decoded fields into domain.Filter"

key-files:
  created:
    - backend/internal/domain/foundations.go
    - backend/internal/repository/foundations/repository.go
    - backend/internal/service/foundations/service.go
    - backend/internal/handler/foundations/handler.go
    - backend/internal/handler/foundations/dto.go
  modified:
    - backend/internal/server/server.go
    - backend/internal/platform/i18n/i18n.go
    - backend/cmd/seed/seed.go
    - backend/cmd/api/main.go

key-decisions:
  - "chi ':' action suffix works natively — '/users/{user_id}:change-role' is matched correctly by chi as a literal suffix after the path param; no fallback sub-router required"
  - "status mapping: DB 'active'/'disabled' uppercased to ACTIVE/DISABLED only at the DTO boundary; all internal code uses lowercase"
  - "actor_label derivation: nil ActorUserID -> 'system'; else '<role>:<user_id>' (full name resolution deferred to E2 employee endpoint)"
  - "change_summary: best-effort one-liner from before/after maps; 'field: before -> after' for known keys"
  - "ip field: always nil — audit_log table has no ip column (migration 00004 omitted it); gap documented here"
  - "send-password-reset: reuses auth.NewRefreshToken()+InsertResetToken (sha256 hash, 1h TTL); no mailer wired; E2E reads token from DB directly"
  - "Session revocation on deactivate: out of scope for Phase 2; deactivate sets status only; auth-side revocation is a separate concern"
  - "CreateUser password placeholder: uses auth.NewRefreshToken() hash as placeholder so the insert satisfies the NOT NULL constraint; user must reset via invite link"
  - "CURSOR_MISMATCH error code: already handled in apperr.statusForCode (400); added i18n messages for both id and en"

# Metrics
duration: 25min
completed: "2026-06-04"
---

# Phase 02 Plan 02: E1 Foundations Vertical Slice (handlers, service, repository)

**Complete E1 user management + audit-log + platform-settings vertical slice: 10 chi handlers, RBAC-gated, cursor-paginated, audited on every write, chi ':action' suffix routes working**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-06-04T01:00:00Z
- **Completed:** 2026-06-04T01:25:00Z
- **Tasks:** 3
- **Files modified:** 9 (5 created, 4 extended)

## Accomplishments

- Created `domain/foundations.go`: AuditEntry, PlatformSetting, UserFilter, AuditFilter dependency-free types
- Created `repository/foundations/repository.go`: sqlc-backed repo with 11 methods; jsonb decode for before/after audit states; var _ svc.Repository compile-time check
- Created `service/foundations/service.go`: full business logic for all 10 E1 operations, 7x audit.Record calls (one per write path), ROLE_NOT_ALLOWED + CONFLICT apperr codes per spec, cursor pagination via pageCursor{CreatedAt,ID}
- Created `handler/foundations/handler.go` + `dto.go`: 10 chi handlers, exact spec JSON shapes, status UPPER mapping, actor_label/change_summary/ip derivation
- Updated `server/server.go`: Foundations handler in Deps; RBAC group with rbac.RequireRole(super_admin, hr_admin); all 10 routes including literal ':' action suffix routes
- Updated `i18n.go`: ROLE_NOT_ALLOWED + CURSOR_MISMATCH in both id and en catalogs
- Updated `cmd/api/main.go`: fndRepo->fndSvc->fndHandler wired exactly like identity slice
- Extended `cmd/seed/seed.go`: 3 extra personas (dewi.lestari agent, agus.pratama shift_leader, bambang.admin hr_admin) + seedAuditLog() with 5 idempotent rows (2x user.CREATE, 1x user.change_role, 1x placement.CREATE, 1x user.deactivate; one system row with NULL actor)
- `go build ./... && go vet ./...` clean; gofmt -l clean on all foundations dirs

## Task Commits

1. **Task 1: domain types + foundations repository** - `9e65ce5` (feat)
2. **Task 2: foundations service with audit on every write** - `5dd3ad8` (feat)
3. **Task 3: handlers + DTOs + routes + i18n + seed extension** - `4077165` (feat)

## Files Created/Modified

**Created:**
- `backend/internal/domain/foundations.go` — AuditEntry, PlatformSetting, UserFilter, AuditFilter
- `backend/internal/repository/foundations/repository.go` — sqlc-backed repo (11 methods, jsonb decode, mapErr)
- `backend/internal/service/foundations/service.go` — business logic (10 ops, 7 audit calls, cursor pagination)
- `backend/internal/handler/foundations/handler.go` — 10 chi handlers (decode, validate, call svc, write JSON)
- `backend/internal/handler/foundations/dto.go` — request/response structs + toUserResponse/toAuditSummary/toPlatformSettingsResponse helpers

**Modified:**
- `backend/internal/server/server.go` — Foundations field in Deps; RBAC group + 10 routes mounted
- `backend/internal/platform/i18n/i18n.go` — ROLE_NOT_ALLOWED + CURSOR_MISMATCH (id + en)
- `backend/cmd/api/main.go` — foundations slice construction (repo->svc->handler)
- `backend/cmd/seed/seed.go` — extraPersonas (3 users) + seedAuditLog (5 rows)

## Decisions Made

### chi ':' action suffix routing
chi matches `/users/{user_id}:change-role` as a literal colon-suffix on the path segment. The param `{user_id}` captures up to (but not including) the colon, so `:change-role` is treated as a literal path suffix. No sub-router or custom pattern was needed. Confirmed by code compilation and route table inspection.

### status case mapping
DB `users.status` is lowercase `active`/`disabled`. The API spec uses `ACTIVE`/`DISABLED`. Mapping happens exclusively at the DTO boundary (`strings.ToUpper` in `toUserResponse`). All internal code (service, repository, domain) works with lowercase. Filters from the API are lowercased in the service before passing to the repository.

### actor_label derivation (gap: full name resolution)
`actor_label` in audit responses is best-effort:
- `actor_user_id == nil` → `"system"` (for automated/migration actions)
- otherwise → `"<actor_role>:<actor_user_id>"` (e.g. `"hr_admin:SWP-USR-00001"`)

Full name resolution (e.g. `"Sari Hadi (hr_admin)"`) requires the employee/user name lookup endpoint from E2. TODO(E2): resolve actor name from `users.full_name` JOIN.

### change_summary derivation (best-effort)
Iterates `after` keys; for each key present in `before`, emits `"key: before → after"`. Falls back to `"ACTION: entity_type/entity_id"` when maps are empty. No structured format is guaranteed — the field is informational only.

### ip field (missing column gap)
The `audit_log` table created in migration 00004 has no `ip` column (it was omitted by design — the audit package captures actor + request_id but not IP). The `ip` field in the API response is always `null`. If IP tracking is needed later, a migration adding `ip inet` to `audit_log` is required.

### send-password-reset mechanism
Reuses `auth.NewRefreshToken()` (32 random bytes, sha256 hashed) + `InsertResetToken` (same sqlc query as identity flow). Token has 1h TTL. No mailer is wired in Phase 2 — the E2E harness obtains the token directly from `password_reset_tokens` as in Phase 1. Action is audited with `"user.send_password_reset"`.

### Session revocation on deactivate
`DeactivateUser` sets `status = 'disabled'` only. Active sessions (refresh tokens) for the deactivated user are NOT revoked at this layer — that is auth-side work (e.g. call `RevokeAllRefreshForUser`). Documented as out-of-scope for Phase 2; add in a follow-up or when E1 session-management is fully tested.

### CreateUser placeholder password
The `users.password_hash` column is NOT NULL. When creating a user via the API (without a preset password), a fresh `auth.NewRefreshToken()` hex hash is used as the placeholder. The user resets via the invite link (send_invitation_email=true also inserts a reset token). The placeholder is never usable for login directly.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] apperr.Conflict type assertion removed**
- **Found during:** Task 2 (go build)
- **Issue:** `apperr.Conflict("CONFLICT").(*apperr.Error)` is invalid because `apperr.Conflict` already returns `*apperr.Error` (not an interface)
- **Fix:** Removed the type assertion; return `apperr.Conflict("CONFLICT")` directly
- **Files modified:** `backend/internal/service/foundations/service.go`
- **Commit:** `5dd3ad8` (fixed in same commit)

**2. [Rule 3 - Formatting] dto.go gofmt alignment**
- **Found during:** Task 3 gofmt check
- **Issue:** Struct field tag comment alignment not matching gofmt canonical output
- **Fix:** `gofmt -w internal/handler/foundations/dto.go`
- **Files modified:** `backend/internal/handler/foundations/dto.go`
- **Commit:** `4077165` (fixed before commit)

## Self-Check

---
## Self-Check: PASSED

Files exist:
- `backend/internal/domain/foundations.go` — FOUND
- `backend/internal/repository/foundations/repository.go` — FOUND
- `backend/internal/service/foundations/service.go` — FOUND
- `backend/internal/handler/foundations/handler.go` — FOUND
- `backend/internal/handler/foundations/dto.go` — FOUND

Commits:
- `9e65ce5` — feat(02-02): domain types + foundations repository — FOUND
- `5dd3ad8` — feat(02-02): foundations service with audit on every write — FOUND
- `4077165` — feat(02-02): handlers + DTOs + routes + i18n + seed extension — FOUND

Build gate: `go build ./... && go vet ./...` — PASSED
gofmt gate: all foundations dirs — PASSED
audit.Record count: 7 (>= 5 required) — PASSED
Routes: /users, /audit-log, /platform/settings, rbac.RequireRole — PASSED
