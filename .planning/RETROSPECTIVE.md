# Project Retrospective

A living record of what worked, what didn't, and what we learned — milestone by milestone.

## Milestone: v1.0 — Backend + Full-Stack E2E

**Shipped:** 2026-06-05
**Phases:** 11 | **Plans:** ~50 | **Timeline:** ~6 days (2026-05-30 → 2026-06-05)

### What Was Built
The entire SWP HRIS web console wired to a real Go API + ephemeral Postgres, epic by epic:
auth + Playwright harness (E1), org/people/placement (E2/E3, the placement differentiator with
INV-1..5), scheduling + conflict engine (E4), attendance verify/corrections (E5), two-level leave
with the INV-3 over-leave loop-closer (E6), overtime + holidays (E7), encrypted read-only payroll
with async River export (E8), and the reporting/dashboard/notifications + generalized export
framework that realized every prior epic's notification stubs (E10). Proven by a Go contract-test
drift gate per slice + exhaustive full-stack Playwright E2E (239 passed / 6 skipped / 0 failed).

### What Worked
- **Contract-first + per-slice drift gate.** Hand-written handlers kept faithful by Go contract
  tests (the OpenAPI stays the FE's Orval source) caught zero FE drift across 11 epics.
- **Frontend code as the authoritative reference (not just `.pen`).** Anchoring E2E selectors and
  BE response shapes on the real built components surfaced genuine contract bugs the `.pen` alone
  would have missed — e.g. the dedicated `/placements/expiring` path, the recurring `{data}`
  envelope double-unwrap, and `conflict_details`→`error.details`. The user's mid-run directive to
  prioritize `frontend/` paid off immediately.
- **Defense-in-depth invariants.** DB partial-unique indexes + `FOR UPDATE` row-locking made
  INV-1..5 race-proof and honestly testable, not just service-level checks.
- **Cross-epic loop-closers wired to real producers/consumers.** The over-leave loop (E6 writes
  `approved_leave_days` + cancels shifts → E4 conflict engine reads it) and the notification
  loop-closer (un-stubbed worker + nil-safe dispatch helper retro-wired into prior services) were
  proven end-to-end in E2E, not stubbed.
- **The plan → plan-check → execute → verify gate caught real issues before code.** The plan
  checker flagged a genuine blocker in E6 (writing `status='LEAVE'` would violate the E4 CHECK
  constraint — must be `CANCELLED_BY_LEAVE`) and the missing dedicated `/placements/expiring`
  endpoint in E5 — both would have caused runtime failures.
- **Resilient recovery.** Two executors were interrupted (session limit / connection drop) mid-plan;
  spot-checking actual git + build state (rather than trusting the agent's last message) let work
  resume cleanly — in one case the work was actually complete and only the docs commit was missing.

### What Was Inefficient
- **IDE diagnostics consistently lagged `sqlc` regeneration.** After every data-layer plan the
  linter reported dozens of "undefined: sqlcgen.*" errors that were stale; ground-truth `go build`
  was always green. Cost: a verification `go build` after each backend wave (cheap, but constant).
- **STATE/ROADMAP/REQUIREMENTS tracking drifted.** `gsd-tools state advance-plan` couldn't parse
  the evolving "Current Position" prose, and a linter kept reverting ROADMAP checkbox/progress
  reconciliations — requiring a holistic reconciliation pass before the audit.
- **One heavy plan (E10 11-02) needed splitting** into notifications+retro-wire / dashboard+report+
  exports to stay within context budget — caught at plan time, but the breadth was only obvious once
  the retro-wire scope (touching ~6 prior services) was concrete.

### Patterns Established
- **Per-epic vertical slice:** migration → sqlc → repository → service (apperr codes, audit-in-tx,
  GuardCompany scope) → hand-written chi handler → routes under RequireRole → Go contract tests →
  FE wiring (MSW off) + live Playwright E2E. One executor owns the shared `server.go`/`main.go`/
  `seed.go`/`jobs.go` edits per phase (sequential).
- **Honest deferral:** when a feature can't be delivered in-scope (over-leave needing leave data,
  notify coverage, PDF), build the real code path + a seeded fixture and document the deferral —
  never fake green.
- **Seed every E2E scenario target** (incl. negative-invariant + scope + DECRYPT_FAIL fixtures).
- **Recurring FE fix:** detail GETs are `{data}`-wrapped by handlers even when the OpenAPI declares
  the bare schema → unwrap with a bare fallback on the FE.

### Key Lessons
- Trust ground-truth `go build`/`go test` over both the IDE linter (stale) and an interrupted
  agent's last message — always spot-check git + build before resuming or proceeding.
- Cross-epic integration is where the real risk lives; wiring loop-closers to real producers/
  consumers (and proving them in E2E) is worth the extra plan-checker scrutiny.
- The real frontend code — not the design file — is the source of truth for what the BE must return.

### Cost Observations
- Model mix: planning/execution/verification predominantly **Opus** (the project's model strategy
  mandates Opus for invariant/state-machine epics; the whole milestone is invariant-heavy), with
  Sonnet for the verifier agent. Plan-checkers on Sonnet.
- Sessions: spanned 2 calendar days; one session-limit interruption (recovered).
- Notable: the plan → plan-check → execute → verify loop added cost up front but prevented at least
  two runtime-failure blockers (E6 status enum, E5 expiring endpoint) and surfaced ~10 real
  FE↔BE bugs during E2E that would otherwise have shipped broken.

## Cross-Milestone Trends

| Milestone | Phases | Plans | E2E result | Notable |
|-----------|--------|-------|-----------|---------|
| v1.0 | 11 | ~50 | 239 passed / 6 skipped / 0 failed | First milestone — full web console live against real BE |
