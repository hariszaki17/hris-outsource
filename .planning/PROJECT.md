# SWP HRIS — Backend implementation + full-stack E2E (milestone v1.0-be)

## What This Is

The Go backend for SWP's internal HRIS (outsourced-agent management: placement,
scheduling, attendance, leave, overtime, payroll). The web console (Vite/React) and its
OpenAPI contract already exist; this milestone makes the web app work against a **real Go
API** by implementing the endpoints the FE actually calls, and proving each works with
**full-stack Playwright E2E** (real FE ↔ real BE ↔ ephemeral Postgres).

## Core Value

Every screen the web app shows today works end-to-end against the real backend — provable
by a Playwright test that exercises the real FE against the real Go API.

## Requirements

### Active

- [ ] Implement the backend endpoints **the FE web calls today** (see
  `.planning/reference/fe-endpoint-inventory.md`), matching each `docs/api/*/openapi.yaml`
  contract exactly, following `.planning/reference/backend-build-conventions.md`.
- [ ] Wire FE auth to the real BE (login/refresh/logout/forgot/reset) — currently stubbed.
- [ ] A full-stack Playwright E2E harness with headless / headful / UI modes, real BE +
  ephemeral Postgres + seeded personas (see `.planning/reference/e2e-harness-spec.md`).
- [ ] Exhaustive E2E: every Gherkin AC / case (C-#) per FE feature becomes its own test.

### Out of Scope

- Endpoints in the specs the FE does NOT call yet — deferred (re-scope when FE adds them).
- E9 migration (MySQL→Postgres) — separate effort, no API.
- Mobile (React Native) surface and any mobile-only endpoints.
- Server-side OpenAPI codegen — oapi-codegen can't parse the 3.1 specs; handlers are
  hand-written and kept faithful by Go contract tests. (Specs stay the FE's Orval source.)
- Production infra/CI/CD, deploy target — separate infra phase.

## Context

- Backend scaffold already exists in `backend/` and **builds clean** (`go build ./...`).
  Stack: Go 1.23 · chi · sqlc + pgx · goose · EdDSA JWT + rotating refresh · River
  (Postgres queue, no Redis) · slog + OpenTelemetry. The **E1 auth slice
  (`internal/{handler,service,repository}/identity`) is the reference pattern to copy.**
- The platform kernel (`internal/platform/`) already implements the cross-cutting contract:
  error→envelope, cursor pagination, RBAC roles+scope guards, tx-atomic audit, Idempotency
  store, SWP-ID allocator, i18n, jobs. Reuse it — don't reinvent.
- FE: `frontend/apps/web`, fully Orval-generated client (`@swp/api-client`), MSW togglable
  via `VITE_ENABLE_MSW`, base URL via `VITE_API_BASE_URL`. FE auth = in-memory bearer token.
- Specs are authoritative: `docs/api/CONVENTIONS.md` + per-epic `openapi.yaml`; behavior in
  `docs/epics/E#-*/FEATURE.md` (INV-#) + `prds/*.md` (BR-#, Gherkin, C-#).

## Constraints

- **Tech stack**: locked (see Context) — do not introduce new frameworks. — keeps web+mobile sharing the contract.
- **Contract**: BE responses MUST match `docs/api/*/openapi.yaml` exactly — the FE client is generated from them; drift breaks the FE.
- **No server codegen**: hand-written handlers + Go contract tests. — oapi-codegen lacks OpenAPI 3.1 support.
- **Security**: HRIS with payroll/PII — RBAC enforced server-side, audit on every write, no secrets in logs.
- **Quality gate**: `make verify` (sqlc clean + lint + tests) + the phase's Playwright E2E green.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Scope = FE-used endpoints only | Least effort to make the whole web app live; max value | — Pending |
| No server-side OpenAPI codegen; hand-write handlers | oapi-codegen can't parse 3.1 specs (5/9 fail) | ✓ Good |
| Full-stack Playwright E2E (real BE + ephemeral PG) | Only way to catch FE↔BE contract mismatches | — Pending |
| Exhaustive E2E per Gherkin AC | Max coverage/traceability (user choice) | — Pending |
| One phase per epic, dependency-ordered, auth first | Auth gates E2E; epics build on each other | — Pending |

---
*Last updated: 2026-06-03 after milestone planning (v1.0-be)*
