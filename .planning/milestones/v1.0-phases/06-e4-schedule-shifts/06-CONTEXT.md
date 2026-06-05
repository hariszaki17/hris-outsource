# Phase 6: E4 Schedule & Shifts - Context

**Gathered:** 2026-06-04 (autonomous — recommended decisions auto-accepted per user's overnight directive)
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the FE-used E4 "schedule & shifts" endpoints against the real BE and wire the
screens off MSW, proven with exhaustive full-stack Playwright E2E. Delivers: the shift-master
catalog (CRUD + deactivate/reactivate), day-by-day schedule entries (CRUD), a conflict-check
engine, and bulk-apply (apply a shift across a date range with partial-success reporting).
Scheduling hangs off the Phase-5 placement record (a schedule entry links a `placement_id` →
company + service line + site). Attendance (Phase 7) verifies against these schedule entries
later; leave (Phase 8) feeds the over-leave conflict. Mobile agent views and agent-initiated
swap/day-off requests are out of scope (mobile-only / not FE-web-called).
</domain>

<decisions>
## Implementation Decisions

### Invariant & Conflict Enforcement (careful + scalable)
- **INV-1 (one schedule entry per agent per date):** DB partial/҂composite unique index on
  `schedule_entries(employee_id, work_date) WHERE deleted_at IS NULL` (race-proof backstop) +
  service pre-check → conflict code `DOUBLE_SHIFT`.
- **INV-2 (active placement required on the date):** service check that the agent has an
  ACTIVE/EXPIRING placement covering `work_date`; link the entry's `placement_id`. Violation →
  `OUTSIDE_PLACEMENT_PERIOD`.
- **INV-3 (leader scope):** shift_leader may only schedule agents at their own company —
  `rbac.GuardCompany` on writes + scoped list (leader sees own company only); HR/super_admin any.
  Violation → 403 `OUT_OF_SCOPE` (CONVENTIONS §17).
- **INV-4 (auto-publish + notify):** saving an entry is immediately effective; fire a
  notification **stub** (comment-marked TODO(Phase-11)) like the Phase-4/5 pattern. No draft state.
- **Conflict engine** (`POST /schedule:check` + enforced on writes) returns, per the E4 contract:
  `DOUBLE_SHIFT`, `OUTSIDE_PLACEMENT_PERIOD`, `SHIFT_NOT_FOR_SERVICE_LINE` (shift master's
  service_line ≠ placement's), `SHIFT_DEACTIVATED` (master is inactive), `BREAK_OUTSIDE_WINDOW`
  (break not within shift window), and the leave-dependent `SHIFT_OVER_LEAVE` / `CANCELLED_BY_LEAVE`.
- **CROSS-EPIC DEPENDENCY — over-leave conflicts (SHIFT_OVER_LEAVE / CANCELLED_BY_LEAVE) need
  leave data, which lands in Phase 8 (E6).** Recommended approach for the planner: implement the
  conflict-engine branch NOW (code path + code + envelope), reading approved leave from a leave
  source. Since E6's `leave_requests` table does not exist yet, gate this branch so it is real
  and testable WITHOUT pre-empting E6's schema: create a **minimal `approved_leave_days` read
  source** (a lightweight table or view this phase owns, recording employee_id + leave_date,
  which E6 will later populate/supersede) and seed one approved-leave day so the over-leave
  conflict is genuinely exercised in E2E. If the planner judges a new table risks colliding with
  E6, the fallback is a documented seeded-fixture + an E2E that asserts the over-leave code via a
  direct API call, with the note that E6 wires the production source. **Do NOT fake the conflict
  or skip the success criterion silently** — if over-leave cannot be honestly delivered now,
  surface it as a deferral in the SUMMARY and VERIFICATION, not as green.
- **Bulk-apply (`POST /schedule:bulk-apply`):** apply a shift master to an agent across a date
  range; per-date conflict check; **partial success** — return a per-date result list
  (applied / skipped-with-conflict-code) and an HTTP 200/207-style envelope per the openapi
  contract. Atomic per-date (one failing date does not roll back the successful ones).

### Build approach (mirror Phase-5 slice EXACTLY)
- migration → sqlc (`make gen`) → repository → service (apperr codes, audit, scope guards) →
  hand-written chi handlers → routes in server.go under RequireRole → Go contract tests →
  FE wiring (MSW off) + live Playwright E2E. Match `docs/api/E4-shift-scheduling/openapi.yaml`
  byte-for-byte on response shapes. Cursor pagination + filters on list endpoints (CONVENTIONS §11).
- SWP IDs: shift master `SHM` (or per ids.go/CONVENTIONS entity table — check ids.go, add prefix
  only if missing), schedule entry `SCH` (check existing). Soft-delete where the spec models it;
  schedule entries support hard DELETE per `DELETE /schedule/{id}`.
- New query dir `backend/db/queries/scheduling/`. Audit every write. action-suffix routes
  (`:deactivate`, `:reactivate`, `:check`, `:bulk-apply`) — chi `:` suffix matches natively.

### Endpoint scope = the 11 FE-used hooks ONLY (fe-endpoint-inventory.md E4)
- Shift masters: `GET /shift-masters`, `POST /shift-masters`, `PATCH /shift-masters/{id}`,
  `POST /shift-masters/{id}:deactivate`, `:reactivate`.
- Schedule: `GET /schedule`, `POST /schedule`, `PATCH /schedule/{id}`, `DELETE /schedule/{id}`,
  `POST /schedule:check`, `POST /schedule:bulk-apply`.
- Non-FE endpoints (swap requests, mobile agent views) deferred.

### Seed (in 06-02)
- A couple of shift masters (e.g. SWP-SHM-001 "Pagi" / 002 "Malam") tied to service lines used
  by the seeded companies; schedule entries for the seeded placements (Phase-5 SWP-PL-5001..5004)
  on known dates; one approved-leave day to exercise SHIFT_OVER_LEAVE; data that lets the conflict
  E2E trigger DOUBLE_SHIFT (two entries same agent/date) and OUTSIDE_PLACEMENT_PERIOD.
- **TZ note (from Phase-5 05-03):** the fixed E2E clock derives statuses at Asia/Jakarta vs UTC
  midnight — schedule date fixtures must use clearly-in-range dates; account for the TZ boundary.

### Plan split (4 plans, mirrors ROADMAP)
- **06-01** Migrations + sqlc (`shift_masters`, `schedule_entries`, minimal `approved_leave_days`
  read source if chosen); INV-1 unique index; FKs to placements/service_lines/employees.
- **06-02** Services + handlers: conflict engine, bulk-apply partial success, scope guards,
  shift-master CRUD, schedule CRUD, auto-publish notify stub, seed.
- **06-03** Go contract tests vs E4 openapi (all conflict codes incl. over-leave, bulk-apply
  partial-success envelope, scope 403).
- **06-04** Full-stack Playwright E2E (per Gherkin AC: shift-master CRUD, schedule grid CRUD,
  conflict negatives, bulk-apply partial success, leader-scope). Selectors derived from the REAL
  components (schedule-grid-screen.tsx, schedule-overlays.tsx, shift-masters-screen.tsx).

### Claude's Discretion
- Exact over-leave mechanism (minimal table vs seeded fixture) — planner picks the cleanest that
  honestly delivers the conflict without colliding with E6's future schema.
- schedule_entries soft-delete vs hard-delete (spec has DELETE) — match the contract.
- Conflict-check response grouping/shape — match the openapi example exactly.
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Platform kernel (httpx cursor/PageResponse, rbac roles + `GuardCompany` scope, audit, apperr +
  `error.details` envelope added in Phase 5, ids, idempotency, db.TxManager, i18n, Asia/Jakarta TZ).
- **Reference slice = Phase-5 placement** (closest analog: scope guards, conflict/invariant codes,
  partial-unique index, FOR UPDATE, atomic multi-row writes, seed pattern). Also org (service
  lines), people (employees), E3 placements (the FK target).
- E2E harness: real stack + resetDb + loginAs personas + `window.__swp_get_token__`; `e3-helpers.ts`
  (apiAs, pickCombobox) reusable. Existing E2E layout `frontend/e2e/tests/{e1,e2,e3,smoke}/` →
  add `e4/`.

### Established Patterns
- Partial unique index for "one X per key" invariants. apperr.Conflict()/ConflictWithDetails for
  409 + structured details. apperr struct literals for non-default HTTP status. Notification
  dispatch stubbed (TODO Phase-11). DataTable rows = `div.border-b`; toggles `role=switch`;
  `noValidate` on RHF+Zod number forms; `.js` import extensions in E2E; PERSONAS.* objects.

### Integration Points
- New `backend/db/queries/scheduling/` (sqlc glob). Routes in server.go authenticated group under
  RequireRole (writes: super_admin/hr_admin/shift_leader-scoped; reads scoped for leader). Seed
  extension in cmd/seed/seed.go. FE screens exist (e4-scheduling/*, built from .pen) calling
  `@swp/api-client` e4 hooks via MSW — wire to real BE. E2E under new frontend/e2e/tests/e4/.
</code_context>

<specifics>
## Specific Ideas
- The schedule grid (schedule-grid-screen.tsx) is the primary surface — E2E must drive real
  cell/overlay interactions, not invented selectors.
- Conflict E2E must trigger REAL conflict codes (not mocked): two entries same agent+date →
  DOUBLE_SHIFT; entry outside the placement period → OUTSIDE_PLACEMENT_PERIOD; (if delivered)
  entry on an approved-leave day → SHIFT_OVER_LEAVE.
- Bulk-apply E2E must assert partial success: a range where some dates apply and at least one is
  skipped with a conflict code.
- Leader-scope E2E: shift_leader persona cannot schedule/list another company's agents → 403.
</specifics>

<deferred>
## Deferred Ideas
- Agent-initiated swap / day-off requests + mobile agent schedule view (mobile-only / not FE-web).
- Notification dispatch implementation (stubbed; Phase-11).
- Production over-leave leave source — fully wired when E6 Leave (Phase 8) lands; this phase only
  needs enough to honestly exercise the conflict code.
- Attendance verification against schedule entries — Phase 7.
</deferred>

---

*Phase: 06-e4-schedule-shifts*
*Context gathered: 2026-06-04 (autonomous)*
