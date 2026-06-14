# Phase 2: E1 Foundations - Context

**Gathered:** 2026-06-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the FE-used E1 Foundations endpoints against the real BE: user management
(list/create/update + role change + deactivate/reactivate + send-password-reset), the
audit-log read API (list with filters + cursor pagination, entry detail), and platform
settings read. Wire the E1 foundations FE screens off MSW onto the real BE and prove with
exhaustive Playwright E2E. Auth (Phase 1) is done and reused. Non-E1 epics are out of scope.
</domain>

<decisions>
## Implementation Decisions

### Scope (exact endpoints — FE-used only)
- `GET /users`, `POST /users`, `PATCH /users/{user_id}`, `POST /users/{user_id}:change-role`, `:deactivate`, `:reactivate`, `:send-password-reset`.
- `GET /audit-log`, `GET /audit-log/{audit_log_id}`.
- `GET /platform/settings`.
- NOT in scope (FE doesn't call): `/users:bulk-deactivate`. Defer.

### Build approach
- Follow `.planning/reference/backend-build-conventions.md` per-endpoint recipe and copy the E1 identity slice shape. Hand-written chi handlers; sqlc queries (`make gen`); match `docs/api/E1-foundations/openapi.yaml` shapes EXACTLY (the FE client is generated from it).
- `users` and `audit_log` tables already exist; users gained `full_name`/`last_login_at` in migration 00006. Add a `platform_settings` table (or seed-backed singleton) for `GET /platform/settings` per the spec schema.
- RBAC: user management + settings are super_admin/hr_admin (per spec x-rbac); audit-log read per spec. Enforce with `rbac.RequireRole` + scope guards. Audit every write (CONVENTIONS §16.1). `send-password-reset` reuses the Phase-1 reset-token mechanism (generates a token; no email in this phase). `:change-role`/`:deactivate`/`:reactivate` are action endpoints (idempotency where the spec flags it).
- Cursor pagination for `/users` and `/audit-log` via `httpx.PageResponse` + cursor codec. Audit-log filters per spec (`actor`, `entity_type`, date range, `q`).
- Seed: extend `backend/cmd/seed` with a few extra users + some audit_log rows + platform_settings so the E1 screens render in E2E.

### E2E coverage (exhaustive)
- One `test()` per Gherkin scenario/case in `docs/epics/E1-foundations/prds/` (rbac-roles.md, audit-log.md, platform-conventions.md). Cover: list/create/edit user, change role, deactivate→reactivate, send password reset; audit-log list+filter+pagination+detail; settings load; RBAC negative cases (non-admin gets 403/no-permission state). Run against the real stack; each test named by scenario/BR-#/C-#.

### Claude's Discretion
- platform_settings storage shape (table vs config-backed) — pick simplest matching the spec response.
- Whether send-password-reset returns the token in dev/test (for E2E) or the E2E reads it from the DB like Phase 1 did.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope & build rules
- `.planning/reference/fe-endpoint-inventory.md` (E1 section = endpoints in scope)
- `.planning/reference/backend-build-conventions.md` (per-endpoint recipe, hard rules, DoD)
- `.planning/reference/e2e-harness-spec.md` (E2E modes/topology/coverage)

### Contract & behavior
- `docs/api/E1-foundations/openapi.yaml` — users (line 408+), audit-log (942+), platform/settings (1136+); match request/response/x-rbac exactly.
- `docs/api/CONVENTIONS.md` §5 naming, §7 status, §8 cursor pagination, §9 filtering, §11 errors, §16.1 audit, §17 RBAC.
- `docs/epics/E1-foundations/prds/rbac-roles.md` — user/role management AC + RBAC matrix.
- `docs/epics/E1-foundations/prds/audit-log.md` — audit-log behavior + Gherkin AC.
- `docs/epics/E1-foundations/prds/platform-conventions.md` — platform settings.
- `docs/epics/E1-foundations/FEATURE.md` — invariants/flows.

### Reference implementation (copy shape)
- `backend/internal/{handler,service,repository}/identity` (auth slice), `backend/internal/platform/*` (httpx cursor/pagination, rbac, audit, apperr, ids), `backend/db/queries/identity/users.sql`.
- `backend/cmd/seed/seed.go` (extend the seed).
- FE screens: `frontend/apps/web/src/features/e1-foundations/{users-screen,user-overlays,audit-log-screen,audit-detail-drawer,settings-*}.tsx`; hooks in `frontend/packages/api-client/src/e1.ts`.
- E2E patterns: `frontend/e2e/tests/e1/authentication.spec.ts`, `frontend/e2e/lib/*`.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Platform kernel: `httpx` (cursor, PageResponse, WriteJSON/WriteError), `rbac` (RequireRole, GuardCompany/Self), `audit.Record`, `apperr`, `ids`, `idempotency`, `db.TxManager`.
- `users` table + sqlc user queries already exist (extend with list/update/role/status queries). `audit_log` table exists (add list/get queries).
- E2E harness (Phase 1) boots the real stack + seeds personas; `loginAs` fixture + `resetDb`.

### Established Patterns
- Hand-written handlers → service (apperr codes) → repository (sqlc, tx writes). Audit + (optional) River notify in the same tx. Cursor pagination + typed filters.

### Integration Points
- New routes mounted in `backend/internal/server/server.go` (authenticated group, under RequireRole).
- New sqlc query files under `backend/db/queries/identity/` (users mgmt) + a new `backend/db/queries/foundations/` for audit-log + platform settings (or keep under identity). `make gen` after.
- Seed extension in `backend/cmd/seed`.
</code_context>

<specifics>
## Specific Ideas
- send-password-reset reuses Phase-1 password_reset_tokens; E2E obtains the token from the DB (same pattern as Phase-1 reset flow) rather than email.
</specifics>

<deferred>
## Deferred Ideas
- `/users:bulk-deactivate` (FE doesn't call it yet).
- Audit-log write API (audit is written implicitly by every mutation, not a public write endpoint).
</deferred>

---

*Phase: 02-e1-foundations*
*Context gathered: 2026-06-04*
