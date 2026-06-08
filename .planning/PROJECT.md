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

## Current Milestone: v1.2 Mobile MVP (Agent App)

**Goal:** An agent can run their full daily work loop from the phone — clock in/out at the
right site, view their schedule, fix mistakes, request leave/OT, and see their pay — against
the real Go backend. Full agent persona; shift-leader app is a later milestone.

**Build model:** Full-stack vertical slices. Each phase ships the needed Go endpoint(s) + the
RN screen(s) together, proven end-to-end. Builds on the v1.1 Expo scaffold (`feat/mobile-scaffold`).

**Phases (13–20):**
- 13 App shell + auth + Beranda + notifications (backend READY)
- 14 Clock in/out + geofence + my-attendance (backend NEW + open-route) — the killer loop
- 15 Attendance correction (backend NEW)
- 16 My schedule (backend open-route)
- 17 Leave request + doc upload (backend NEW)
- 18 Overtime request/confirm (backend NEW)
- 19 Payslip history (backend open-route)
- 20 Profile self-service + change-request (backend open-route + NEW)

**Key context (backend coverage audit 2026-06-08):** auth, notifications, dashboard, and ALL
shift-leader endpoints are already implemented. The agent gap is (a) clock-in/out + photo —
genuinely missing, the biggest build; (b) several agent reads (attendance, schedule, payslip,
profile) exist but the route guard excludes `agent` (spec x-rbac already allows agent self-scope)
— work is "open an agent-scoped route + self-filter"; (c) agent create flows (correction, leave,
OT, change-request) need new POST routes. Planned in worktree `feat/mobile-scaffold` (pre-merge).
NOTE: backend changes here may conflict with parallel backend work on `main` — coordinate merges.

**Previous milestone:** v1.1 Mobile Foundation (Expo scaffold) shipped 2026-06-08 —
`milestones/v1.1-REQUIREMENTS.md`.

## Requirements

### Validated

- ✓ Backend endpoints the FE web calls today — implemented behind the locked `docs/api/*/openapi.yaml` contracts across all 11 epics — v1.0
- ✓ FE auth wired to the real BE (login/refresh/logout/forgot/reset) — v1.0
- ✓ Full-stack Playwright E2E harness (headless/headful/UI, real BE + ephemeral Postgres + seeded personas, hardened detached-worker boot) — v1.0
- ✓ Exhaustive E2E per Gherkin AC + a Go contract-test drift gate per slice — v1.0 (final suite 239 passed / 6 skipped / 0 failed)

### Active (next milestone — candidates)

- [ ] Notification dispatch coverage beyond leave/OT/attendance (placement, payroll, change-requests, quotas — currently nil-safe no-op stubs).
- [ ] PDF export (currently `EXPORT_FORMAT_UNSUPPORTED`; Excel only in v1.0).
- [ ] One independent human `pnpm e2e` pass to close the 6 phases verified `human_needed`.

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
| Scope = FE-used endpoints only | Least effort to make the whole web app live; max value | ✓ Good — whole web console live end-to-end |
| No server-side OpenAPI codegen; hand-write handlers | oapi-codegen can't parse 3.1 specs (5/9 fail) | ✓ Good — Go contract tests held the line, zero FE drift |
| Full-stack Playwright E2E (real BE + ephemeral PG) | Only way to catch FE↔BE contract mismatches | ✓ Good — caught real bugs (INV-1 banner, `{data}` envelope, conflict_details) |
| Exhaustive E2E per Gherkin AC | Max coverage/traceability (user choice) | ✓ Good — 239 passing E2E across 11 epics |
| One phase per epic, dependency-ordered, auth first | Auth gates E2E; epics build on each other | ✓ Good — clean cross-epic seams (placement backbone, over-leave loop) |
| Defense-in-depth invariants (DB partial-unique index + FOR UPDATE) | Race-proof INV enforcement under concurrency | ✓ Good — INV-1..5 honest, contract-tested |
| Frontend code (not just `.pen`) as authoritative visual/interaction reference | Real components are what ships + what E2E drives | ✓ Good — surfaced contract mismatches `.pen` alone would miss |
| Cross-epic loop-closers (over-leave, notification dispatch) wired to real producers/consumers | Avoid stub-only "appears to work" | ✓ Good — proven end-to-end in E2E |

## Current State

**Shipped v1.0 (2026-06-05).** The Go backend (`backend/`) implements every FE-used endpoint
across all 11 epics (E1 foundations → E10 reporting) behind the locked OpenAPI contracts;
migrations 00001–00036; River worker for async export + notification dispatch; AES-256-GCM
encryption-at-rest for payroll. The web console (`frontend/apps/web`) runs against the real BE
with MSW off. Quality spine: a Go contract-test drift gate per slice + exhaustive full-stack
Playwright E2E (239 passed / 6 skipped / 0 failed).

**Known tech debt (carried to v1.1):** notification dispatch limited to leave/OT/attendance
(other stubs are nil-safe no-ops); PDF export deferred (Excel only); 6 phases' E2E is
executor-run green pending one independent human `pnpm e2e` pass. Out of scope and untouched:
E9 migration (MySQL→Postgres), mobile (React Native), production infra/CI/CD.

---
*Last updated: 2026-06-05 after v1.0 milestone (Backend + Full-Stack E2E)*
