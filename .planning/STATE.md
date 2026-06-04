---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: executing
stopped_at: Completed 02-e1-foundations/02-04-PLAN.md
last_updated: "2026-06-04T02:40:33.873Z"
last_activity: "2026-06-04 — Plan 01-04 complete: login/forgot/reset/logout wired to real @swp/api-client E1 hooks; SessionUser from MeResponse; credentials:'include' for cross-origin cookie refresh transport."
progress:
  total_phases: 11
  completed_phases: 2
  total_plans: 9
  completed_plans: 9
  percent: 8
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-03)

**Core value:** Every screen the web app shows today works end-to-end against the real backend.
**Current focus:** Phase 1 — Test Harness + Auth

## Current Position

Phase: 1 of 11 (Test Harness + Auth)
Plan: 4 of 5 in current phase (next: 01-05 E2E auth spec)
Status: In progress
Last activity: 2026-06-04 — Plan 01-04 complete: login/forgot/reset/logout wired to real @swp/api-client E1 hooks; SessionUser from MeResponse; credentials:'include' for cross-origin cookie refresh transport.

Progress: [█░░░░░░░░░] 8%

## Performance Metrics

**Velocity:**
- Total plans completed: 3
- Average duration: ~35min
- Total execution time: ~1.75 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-test-harness-auth | 3 done / 5 total | ~105min | ~35min |
| Phase 01 P03 | 690 | 3 tasks | 16 files |
| Phase 01-test-harness-auth P05 | 2413 | 2 tasks | 4 files |
| Phase 02-e1-foundations P01 | 2 | 2 tasks | 7 files |
| Phase 02-e1-foundations P02 | 25 | 3 tasks | 9 files |
| Phase 02-e1-foundations P03 | 15 | 2 tasks | 1 files |
| Phase 02-e1-foundations P04 | 107 | 2 tasks | 7 files |

## Accumulated Context

| Phase 01-test-harness-auth P02 | 2 | 2 tasks | 3 files |
| Phase 01-test-harness-auth P04 | 2 | 2 tasks | 8 files |

### Decisions

Full log in PROJECT.md Key Decisions. Recent:
- Scope = FE-used endpoints only (`.planning/reference/fe-endpoint-inventory.md` is the contract).
- No server-side OpenAPI codegen (oapi-codegen lacks 3.1 support) — hand-written handlers + Go contract tests.
- Full-stack Playwright E2E (real BE + ephemeral Postgres); exhaustive per Gherkin AC.
- One phase per epic, dependency-ordered, auth first.
- [Phase 01-test-harness-auth]: shift_leader company_id = SWP-CMP-0021 literal (FK not enforced until Phase 3 companies migration)
- [Phase 01-test-harness-auth]: cmd/seed exported password constants live in seed.go co-located with hashing logic; sequential inserts (no tx) for idempotent skip-if-exists
- [01-01]: webServer uses `vite dev` not `vite preview` — avoids build step; dev server reads VITE_* env vars at startup
- [01-01]: DB isolation = TRUNCATE app tables + reseed (not per-worker transactions — incompatible with real HTTP server)
- [01-01]: Ed25519 keypair generated fresh per run via `go run ./cmd/seed -genkeys` stdout (line1=privkey, line2=pubkey)
- [01-04]: buildSessionUser sets companyName = scope.company_id literal for shift_leader (no company-name endpoint in Phase 1); TODO(Phase-3) to resolve via companies endpoint
- [01-04]: credentials:'include' added to mutator.ts customFetch so ALL generated hooks send the refresh cookie cross-origin; BE sets CORS allow-origin for :4173/:5173
- [01-04]: logout handler lives in shell.tsx (useAuthLogout) and is passed to UserMenu as onLogout prop; UserMenu stays stateless re: auth
- [01-04]: forgot-password always advances to 'sent' even on network error (anti-enumeration, authentication.md C-2)
- [01-04]: reset-password minLength raised from 8 to 10 to match BE platform password policy (AU-4)
- [Phase 01-test-harness-auth]: TxRunner extracted as interface in service package to allow fake-based unit tests without testcontainers
- [Phase 01-test-harness-auth]: Reset token plaintext not emailed in Phase 1; E2E harness obtains token by querying password_reset_tokens directly (no mailer wired)
- [Phase 01-test-harness-auth]: Reset-token E2E acquisition: seedResetToken(email, plaintext) inserts sha256(plaintext) directly into password_reset_tokens — no mailer needed; E2E controls the plaintext presented to the browser
- [Phase 01-test-harness-auth]: Docker Scout CLI hook (config.json 'scout.hooks: pull') intercepts docker pull and hangs; workaround is to remove 'pull' from hooks and pull via curl --unix-socket docker.sock POST /images/create
- [Phase 02-e1-foundations]: ids.go unchanged — platform_settings keys are plain text (not SWP-prefixed); USR and AL prefixes already existed
- [Phase 02-e1-foundations]: foundations/ query package at db/queries/foundations/ — sqlc glob db/queries/* picks up new subdirectories automatically
- [Phase 02-e1-foundations]: platform_settings stored as flat key/value table matching openapi PlatformSettings shape; wave-2 maps rows to response object
- [Phase 02-e1-foundations]: chi ':' action suffix routes match natively: '/users/{user_id}:change-role' works without sub-router
- [Phase 02-e1-foundations]: status mapping: DB lowercase 'active'/'disabled' uppercased to ACTIVE/DISABLED only at DTO boundary
- [Phase 02-e1-foundations]: ip field always null in audit responses — audit_log table has no ip column (migration 00004 omission)
- [Phase 02-e1-foundations]: send-password-reset reuses auth.NewRefreshToken()+InsertResetToken (sha256, 1h TTL); no mailer; E2E reads from DB
- [Phase 02-e1-foundations]: Session revocation on deactivate: out of scope in Phase 2; only status set; auth-side revocation deferred
- [Phase 02-e1-foundations]: fakeTx instead of nil pgx.Tx: foundations service calls audit.Record inside InTx closures (unlike identity service); nil tx panics; fakeTx implements pgx.Tx with only Exec as no-op
- [Phase 02-e1-foundations]: Dynamic principal injection: harness.principal is a mutable field read by a closure middleware, so tests can swap roles without re-building the chi router
- [Phase 02-e1-foundations]: tryRestoreSession hydrates in-memory accessToken from httpOnly cookie before React mounts — enables page.goto() on authed routes in E2E
- [Phase 02-e1-foundations]: DataTable rows are div.border-b not tr — all E2E row locators must use div.border-b.filter() pattern
- [Phase 02-e1-foundations]: playwright.config.ts timeout: 90s to accommodate cold Vite compilation on first test run

### Pending Todos

None.

### Blockers/Concerns

- Phase 1 depends on Docker being available for the ephemeral Postgres in the E2E harness.

## Session Continuity

Last session: 2026-06-04T02:40:33.870Z
Stopped at: Completed 02-e1-foundations/02-04-PLAN.md
Resume file: None
