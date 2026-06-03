---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: in_progress
stopped_at: Completed 01-test-harness-auth/01-01-PLAN.md
last_updated: "2026-06-04T00:00:46Z"
last_activity: 2026-06-04 — Plan 01-01 complete: @swp/e2e harness skeleton (playwright config, globalSetup, loginAs fixture, smoke spec).
progress:
  total_phases: 11
  completed_phases: 0
  total_plans: 5
  completed_plans: 2
  percent: 5
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-03)

**Core value:** Every screen the web app shows today works end-to-end against the real backend.
**Current focus:** Phase 1 — Test Harness + Auth

## Current Position

Phase: 1 of 11 (Test Harness + Auth)
Plan: 2 of 5 in current phase
Status: In progress
Last activity: 2026-06-04 — Plan 01-01 complete: @swp/e2e harness skeleton (playwright config, globalSetup, loginAs fixture, smoke spec).

Progress: [█░░░░░░░░░] 5%

## Performance Metrics

**Velocity:**
- Total plans completed: 0
- Average duration: —
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

## Accumulated Context
| Phase 01-test-harness-auth P02 | 2 | 2 tasks | 3 files |

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

### Pending Todos

None yet.

### Blockers/Concerns

- Phase 1 depends on Docker being available for the ephemeral Postgres in the E2E harness.

## Session Continuity

Last session: 2026-06-04T00:00:46Z
Stopped at: Completed 01-test-harness-auth/01-01-PLAN.md
Resume file: None
