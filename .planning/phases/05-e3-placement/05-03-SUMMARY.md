---
phase: 05-e3-placement
plan: 03
subsystem: backend-contract-tests
tags: [go, contract-tests, drift-gate, placement, invariants, INV-1, INV-2, INV-3, INV-4, shift-leader, roster, site-scope, rbac]

# Dependency graph
requires:
  - phase: 05-e3-placement
    plan: 02
    provides: 13 FE-used E3 handlers + services + apperr/error.details envelope + INV-1..4 enforcement
  - phase: 04-e2-people
    provides: fakeRepo + fakeTx + chi httptest harness pattern (agreements_handler_test.go)
provides:
  - contract-test drift gate for every E3 endpoint vs docs/api/E3-placement/openapi.yaml
  - documented coverage map so 05-04 E2E avoids duplicating site-scope (company-scope is FE E2E)
  - reusable in-memory fakePlacementRepo + fakeShiftLeaderRepo (shared placement state) harness
affects: [05-04 E2E]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "fakeShiftLeaderRepo embeds a *fakePlacementRepo pointer so INV-4 (active placement at company) + current-leader joins resolve off the SAME fixtures the placement tests seed"
    - "deterministic clock via SetClock(fixedNow=2026-06-04 12:00 WIB) on BOTH services; date fixtures use UTC-midnight dates"
    - "captured-filter assertion (repo.lastListFilter) proves ?q= + ?status= are wired through to the repo, not silently dropped"

key-files:
  created:
    - backend/internal/handler/placement/placements_handler_test.go
    - backend/internal/handler/placement/shift_leader_handler_test.go
    - backend/internal/handler/placement/roster_handler_test.go
  modified: []

key-decisions:
  - "INV-4 no-placement case asserts details.company_id + details.employee_id + suggested_actions=[assign_after_placement] (employee_placements_at_company is omitempty → empty when no placement); the PENDING_START case (C-2) proves the populated array"
  - "site-scope coverage is contract-only: a leader for a DIFFERENT site of a site-scoped company succeeds while another site is already led, proving the per-site leadership unit is distinct from the company-level active-leader check (SiteID==nil)"
  - "ACTIVE-on-create asserted via a backdated start (with backdate_reason) rather than start==today, because Asia/Jakarta-midnight vs UTC-midnight boundary makes a same-day start derive PENDING_START under the fixed clock"

requirements-completed: [PLC-01, PLC-02, PLC-03, PLC-04]

# Metrics
duration: ~7min
completed: 2026-06-04
---

# Phase 5 Plan 03: E3 Placement Contract Tests Summary

**Go contract tests for every FE-used E3 endpoint, asserting response shapes + status codes + error envelopes match `docs/api/E3-placement/openapi.yaml` exactly (the drift gate that replaces server-side codegen). 34 test functions across 3 files: placement CRUD + lifecycle (incl. INV-1 409 details + terminal-immutability + transfer/renew chains), shift-leader assign/replace/end (INV-2/3/4 envelopes + PENDING_START C-2 + site-scope), and the company roster (shape + OUT_OF_SCOPE + include_history). `go test ./... -count=1` exits 0; no regressions; gofmt clean on the new files.**

## Accomplishments

- **Harness (mirrors Phase-4 agreements):** in-memory `fakePlacementRepo` (implements `svc.PlacementRepository`) + `fakeShiftLeaderRepo` (implements `svc.ShiftLeaderRepository`) sharing placement state via a pointer, a `fakeTx` (Exec no-op so `audit.Record` works inside `InTx`), and a chi router with a mutable-principal middleware to swap roles per case. Deterministic clock on both services.
- **Placement tests (Task 1):** list envelope + row field set; **search+status filter passthrough** (asserts the repo received `q="Sari"` + `status="ACTIVE"`); **dedicated `/placements/expiring`** (within-window, end_date asc, `within_days` defaults to 30); detail shape + 404; create 201+Location; **INV_1_VIOLATION 409** with `details.invariant=INV_1` + `current_placement.id` + `suggested_actions⊇{transfer,end}`; COMPANY_INACTIVE 409; end<=start 400 (fields.end_date); PLACEMENT_OUTSIDE_CONTRACT 422; PATCH terminal → TERMINAL_STATE_IMMUTABLE 409; end/resign/terminate (+ wrong company-name 400); transfer (predecessor=TRANSFERRED, successor.predecessor_id set) + same-company-same-line RULE_VIOLATION 422; renew (predecessor=SUPERSEDED) + 1-day-buffer PLACEMENT_PERIOD_OVERLAP 422; agent POST → 403.
- **Shift-leader tests (Task 2):** first leader 201; second-leader-no-replace INV_2_VIOLATION 409 (+ current_assignment + suggested_actions=[replace]); replace=true 201 with replaced_assignment (vacated_reason=REASSIGNED, active=false); INV_3_VIOLATION 409 (existing_assignment); INV_4_VIOLATION 409 (company_id/employee_id); **PENDING_START fails INV-4 (C-2)** with the PENDING_START placement surfaced in `employee_placements_at_company`; LEADER_NOT_ELIGIBLE 422; **site-scope path** (distinct site of a site-scoped company gets its own leader); ALREADY_ENDED 409 on replace + double-end.
- **Roster tests (Task 2):** CompanyRosterResponse shape + summary buckets; SL own-company 200 vs cross-company **OUT_OF_SCOPE 403**; include_history toggles terminal-state placements.

## Task Commits

1. **Task 1: placement CRUD + lifecycle contract tests** — `8e350bb` (test)
2. **Task 2: shift-leader + roster contract tests (INV-2/3/4 + site-scope + OUT_OF_SCOPE)** — `edc8550` (test)

## Contract Coverage Map (for 05-04 — avoid duplicating)

| Endpoint | Happy | Error cases contract-tested |
| --- | --- | --- |
| GET /placements | shape + envelope | q + status passthrough (filters wired) |
| GET /placements/expiring | within-window, end_date asc, within_days default 30 | — |
| GET /placements/{id} | detail shape | NOT_FOUND 404 |
| POST /placements | 201 + Location | **INV_1_VIOLATION 409 (full details)**, COMPANY_INACTIVE 409, end<=start 400, PLACEMENT_OUTSIDE_CONTRACT 422, agent 403 |
| PATCH /placements/{id} | — | TERMINAL_STATE_IMMUTABLE 409 |
| POST :transfer | 201 (TRANSFERRED + successor.predecessor_id) | same-company-same-line RULE_VIOLATION 422 |
| POST :renew | 201 (SUPERSEDED) | PLACEMENT_PERIOD_OVERLAP 422 |
| POST :end / :resign / :terminate | 200 | terminate wrong company-name 400 |
| POST /shift-leader-assignments | 201 (company-scope + site-scope) | INV_2/3/4_VIOLATION 409, LEADER_NOT_ELIGIBLE 422, **PENDING_START→INV-4 (C-2)** |
| POST :replace | — | ALREADY_ENDED 409 |
| POST :end (SLA) | 200 (active:false, unassigned_at) | ALREADY_ENDED 409 on re-end |
| GET /client-companies/{id}/roster | HR + SL-own 200 | **OUT_OF_SCOPE 403** (SL cross-company), include_history toggle |

**Site-scope is fully contract-tested here** — 05-04 FE E2E should target the **company-scope** roster + assign paths only (the deferred decision from CONTEXT.md).

## Deviations from Plan

### Spec/impl reconciliations (no code changed — test fixtures adjusted to match the spec the BE already implements)

**1. [Adjustment] ACTIVE-on-create fixture uses a backdated start, not start==today**
- The `fixedNow` clock is `2026-06-04 12:00 WIB`; a `start_date=2026-06-04` (parsed as UTC midnight) derives `PENDING_START` because UTC-midnight is after Asia/Jakarta-midnight under the date-derivation boundary. To assert ACTIVE deterministically the test uses a backdated start (`2026-06-01`) + `backdate_reason`. Behaviour-correct; documents a real TZ boundary 05-04 must respect when picking E2E dates.

**2. [Adjustment] INV-4 no-placement case asserts company_id/employee_id + suggested_actions**
- `INVViolationDetails.EmployeePlacementsAtCompany` is `omitempty`, so the no-placement INV-4 response omits the array entirely (correct — there is nothing to show). The no-placement test asserts `details.company_id`, `details.employee_id`, and `suggested_actions=[assign_after_placement]`; the dedicated PENDING_START case (C-2) proves the populated `employee_placements_at_company` array. Matches the openapi `inv4` example (`employee_placements_at_company: []` there is also empty).

No production code was modified. No architectural changes. No auth gates.

### Deferred (out-of-scope, pre-existing)
- `gofmt -l .` lists several pre-existing Phase-1..4 files (people/identity/domain DTOs, etc.) that were never gofmt-clean. Out of scope for 05-03 (SCOPE BOUNDARY — only files this plan touched). The three new placement test files are gofmt-clean. `make verify`'s golangci-lint config-version drift (logged in 05-02) is likewise pre-existing.

## Authentication Gates

None — pure test code, no external service auth.

## Self-Check: PASSED

- All 3 created files present on disk.
- Commits `8e350bb` + `edc8550` in git history.
- `go test ./... -count=1` exits 0 (all packages ok, incl. placement; no regressions in identity/foundations/org/people).
- Task-1 + Task-2 verify gates (grep INV_1/INV_2/INV_4/OUT_OF_SCOPE + /placements/expiring + within_days/q) all OK.

---
*Phase: 05-e3-placement*
*Completed: 2026-06-04*
