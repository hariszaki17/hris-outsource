---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: executing
stopped_at: Completed 04-e2-people/04-06-PLAN.md
last_updated: "2026-06-04T13:13:25.475Z"
last_activity: "2026-06-04 — Plan 03-05 complete: Go contract tests for all 29 E2 org/master endpoints (companies, sites, service-lines, positions, leave-types, attendance-codes, overtime-rules); drift gate for FE OpenAPI client. `go test ./... -count=1` exits 0."
progress:
  total_phases: 11
  completed_phases: 4
  total_plans: 21
  completed_plans: 21
  percent: 8
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-03)

**Core value:** Every screen the web app shows today works end-to-end against the real backend.
**Current focus:** Phase 1 — Test Harness + Auth

## Current Position

Phase: 3 of 11 (E2 Org/Master Data)
Plan: 5 of 5 in current phase — Phase 03 COMPLETE
Status: In progress
Last activity: 2026-06-04 — Plan 03-05 complete: Go contract tests for all 29 E2 org/master endpoints (companies, sites, service-lines, positions, leave-types, attendance-codes, overtime-rules); drift gate for FE OpenAPI client. `go test ./... -count=1` exits 0.

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
| Phase 03-e2-org-master-data P01 | 25 | 3 tasks | 22 files |
| Phase 03-e2-org-master-data P02 | 452 | 3 tasks | 8 files |
| Phase 03-e2-org-master-data P03 | 6 | 3 tasks | 8 files |
| Phase 03-e2-org-master-data P04 | 12 | 3 tasks | 8 files |
| Phase 03-e2-org-master-data P05 | 20 | 3 tasks | 4 files |
| Phase 03-e2-org-master-data P06 | 75 | 3 tasks | 9 files |
| Phase 04-e2-people P01 | 271 | 2 tasks | 13 files |
| Phase 04-e2-people P02 | 447 | 3 tasks | 9 files |
| Phase 04-e2-people P03 | 385 | 3 tasks | 9 files |
| Phase 04-e2-people P04 | 329 | 3 tasks | 8 files |
| Phase 04-e2-people P05 | 25 | 3 tasks | 3 files |
| Phase 04-e2-people P06 | 5400 | 4 tasks | 9 files |

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
- [Phase 03-e2-org-master-data]: geo_lat/geo_lng stored as nullable double precision; geofence_active derived at DTO boundary (not stored)
- [Phase 03-e2-org-master-data]: ListClientCompanies service_line+has_leader narg params accepted but (IS NULL OR TRUE) — no placements table in Phase 3
- [Phase 03-e2-org-master-data]: ids.go NOT modified — CMP/SITE/SVC/POS/LT/AC/OTR prefixes already existed
- [Phase 03-e2-org-master-data]: OrgCompanies Deps field in server.go; siblings 03-03/03-04 append their own r.Group{} after the ORG slice coordination point
- [Phase 03-e2-org-master-data]: GEOFENCE_RADIUS_INVALID uses apperr.Error{HTTPStatus:400} struct literal — bypasses statusForCode (which defaults to 422)
- [Phase 03-e2-org-master-data]: Seed uses explicit IDs SWP-CMP-0021/0022 + SWP-SITE-0001/0002 via direct INSERT with ON CONFLICT (id) DO NOTHING for deterministic E2E targets
- [Phase 03-e2-org-master-data]: ServiceLineService is a separate struct from Service in the same org package; ServiceLineHandler in same orghttp package — OrgServiceLines Deps field type = *orghttp.ServiceLineHandler
- [Phase 03-e2-org-master-data]: SoftDeletePosition uses repo.SoftDeletePosition (sets deleted_at) not SetPositionStatus — hard soft-delete matching 03-01 decision; SERVICE_LINE_IN_USE when CountActivePositionsForLine > 0; POSITION_IN_USE on unique (line,name) violation; seed uses explicit IDs SWP-SVC-001/002/003 + SWP-POS-014/015
- [Phase 03-e2-org-master-data]: MasterDataService is a separate struct from Service and ServiceLineService in org package; MasterDataHandler in same orghttp package; OrgMasterData Deps field type = *orghttp.MasterDataHandler
- [Phase 03-e2-org-master-data]: min_minutes<30 validation uses apperr.Rule('RULE_VIOLATION') before tx; OvertimeRule uses float64 in domain+DTO (float32 in sqlc); service_line_id is *string (nullable JSON null, never omitempty)
- [Phase 03-e2-org-master-data]: 3 master-data route groups: LT+AC reads all 4 roles; OTR reads excl agent (spec x-rbac); writes super_admin+hr_admin; seed explicit IDs SWP-LT-001/002 + SWP-AC-001/002 + SWP-OTR-001
- [Phase 03-e2-org-master-data]: Conflict toast text: t('errors.conflict')='Terjadi konflik dengan kondisi saat ini.' — regex /konflik/i not /conflict/i
- [Phase 03-e2-org-master-data]: noValidate required on RHF+Zod modal forms with type=number inputs to prevent browser native validation blocking submission
- [Phase 03-e2-org-master-data]: Toggle role=switch (not checkbox/button) per toggle.tsx — Playwright must use getByRole('switch')
- [Phase 04-e2-people]: Bytea blob for agreement_attachments: simplest approach that passes E2E and survives container teardown via reseed; no external storage dependency
- [Phase 04-e2-people]: EA-2 enforced at DB level via partial unique index on employment_agreements(employee_id) WHERE status='active' AND deleted_at IS NULL
- [Phase 04-e2-people]: File prefix FILE added to ids.go for SWP-FILE attachment IDs
- [Phase 04-e2-people]: GET /employees/{id} RBAC: web roles only (super_admin, hr_admin, shift_leader) — agent excluded; agent self-service is mobile-only in Phase 4
- [Phase 04-e2-people]: EP-3 login provisioning stub: provision_login/login_email accepted but UserID stays NULL; no E1 user created in Phase 4 employees milestone
- [Phase 04-e2-people]: seedEmployees() called before persona user loop in Seed() — ordering contract for /auth/me employee resolution
- [Phase 04-e2-people]: EXPIRING virtual status computed at DTO boundary (toAgreementResponse): status=active+PKWT+end_date<now+30d → emit EXPIRING; persisted DB status stays active
- [Phase 04-e2-people]: FILE_TOO_LARGE uses apperr.Error{HTTPStatus:413} struct literal — bypasses statusForCode (no 413 mapping); same technique as GEOFENCE_RADIUS_INVALID in Phase 3
- [Phase 04-e2-people]: ACTIVE_AGREEMENT_EXISTS uses apperr.Conflict() (409) not apperr.Rule() (422 default) — state-of-record constraint, not a semantic rule
- [Phase 04-e2-people]: Approve applies whitelisted fields only (phone/address/bank_account): buildApproveParams overlays CR.Changes onto a full copy of current employee; statutory fields never touched
- [Phase 04-e2-people]: Notification dispatch on CR resolution deferred: stub comment in ApproveChangeRequest + RejectChangeRequest marks Phase N (notifications epic) integration point
- [Phase 04-e2-people]: Old->new diff computed live in GetChangeRequestDetail (not stored snapshot): old = current employee values at query time; audit_log captures exact before/after at approve time
- [Phase 04-e2-people]: FILE_TOO_LARGE 413 and PKWT_PERIOD_EXCEEDS_MAX 422 confirmed by contract tests: apperr.Error struct literals bypass statusForCode
- [Phase 04-e2-people]: BankAccount json tags: added snake_case json tags to domain.BankAccount so diff serialization uses keys FE formatDiffValue expects; also fixes jsonb unmarshal from DB seed
- [Phase 04-e2-people]: RenewAgreement: supersede predecessor before insert — releases partial unique index on active employee; prevents ACTIVE_AGREEMENT_EXISTS on renew
- [Phase 04-e2-people]: window.__swp_get_token__ E2E helper: exposes in-memory access token on window in VITE_ENABLE_MSW=false mode; allows page.evaluate() to make authenticated API requests

### Pending Todos

None.

### Blockers/Concerns

- Phase 1 depends on Docker being available for the ephemeral Postgres in the E2E harness.

## Session Continuity

Last session: 2026-06-04T13:06:33.318Z
Stopped at: Completed 04-e2-people/04-06-PLAN.md
Resume file: None
