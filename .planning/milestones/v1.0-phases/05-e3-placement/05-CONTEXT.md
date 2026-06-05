# Phase 5: E3 Placement - Context

**Gathered:** 2026-06-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the FE-used E3 "placement" endpoints against the real BE and wire the screens off
MSW, proven with exhaustive full-stack Playwright E2E (real FE ↔ real Go API ↔ ephemeral
Postgres). Placement is the project's first-class differentiator: an agent is *placed* at a
client company, in a service line, at exactly one site, for a contract period, with full
lifecycle history. This phase delivers: placement CRUD + lifecycle actions
(renew/transfer/end/resign/terminate), the company roster, and shift-leader assignment
(create/replace/end) — all with invariants INV-1..4 enforced. Org & master data (Phase 3:
companies, sites, service-lines) and People (Phase 4: employees/agents, agreements) are done
and reused (a placement references an employee + company + site + service-line). Scheduling,
attendance, leave, overtime (later phases) hang off the placement record but are out of scope
here.
</domain>

<decisions>
## Implementation Decisions

### Invariant Enforcement (careful + scalable — Claude's recommendation, user-approved)
- **INV-1 (≤1 active placement per agent):** defense-in-depth. (a) DB **partial unique index**
  on `placements(employee_id)` `WHERE lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START','SCHEDULED')`
  (mirrors Phase-4 EA-2 technique) as the race-proof backstop, AND (b) a service-level pre-check
  inside the write tx that returns a friendly **409 `INV_1_VIOLATION`** before hitting the index.
- **INV-2/3/4 + period overlap:** enforced by **transactional service checks with row locking**
  (`SELECT … FOR UPDATE` on the relevant company/site leadership rows and the agent's placements)
  so concurrent requests cannot violate an invariant between check and write. Map to the exact
  contract codes: `INV_2_VIOLATION` (a leadership unit with active placements has exactly one
  leader), `INV_3_VIOLATION` (a leader leads exactly one unit), `INV_4_VIOLATION` /
  `LEADER_NOT_ELIGIBLE` (designated leader must be actively placed in the unit),
  `SHIFT_LEADER_AT_COMPANY` / `SHIFT_LEADER_AT_DESTINATION` (already-a-leader conflicts),
  `ACTIVE_LEADER`, `COMPANY_INACTIVE`, `PLACEMENT_PERIOD_OVERLAP`, `PLACEMENT_OUTSIDE_CONTRACT`,
  `TERMINAL_STATE_IMMUTABLE`, `PLACEMENT_ENDED`. All per `docs/api/E3-placement/openapi.yaml`.
- **Leader scope (INV-5 / `ClientCompany.leader_scope`):** model and enforce **both** `company`
  and `site` leadership units in the data model + service logic (scalable / future-proof). The FE
  roster + shift-leader screens primarily exercise **company-scope**, so seed + E2E target the
  company-scope path thoroughly; site-scope code paths are exercised by Go contract/unit tests.
- **INV-5 (placement at exactly one site):** `placements.site_id` required (FK to Phase-3 sites);
  validate site belongs to the placement's client company.

### Lifecycle State Machine (Accept all)
- **Date-derived statuses at the DTO boundary** (Asia/Jakarta TZ layer): base persisted status is
  simple; `lifecycle_status` resolves ACTIVE / EXPIRING (end_date ≤ today+N) / PENDING_START
  (start_date > today) / SCHEDULED at read time — same technique as Phase-4 EXPIRING. Terminal
  states (ENDED, TERMINATED, RESIGNED, TRANSFERRED, SUPERSEDED) are persisted.
- **History on every action:** every lifecycle transition writes a `placement_history` row
  (action, actor, reason, effective dates, before/after status) **and** an audit_log entry within
  the same tx (success criterion 2).
- **Terminal-state immutability:** any mutation targeting a placement already in a terminal state
  → **409 `TERMINAL_STATE_IMMUTABLE`**.
- **Transfer mechanics:** transfer ends the source placement (`ended_reason=TRANSFERRED`,
  `ended_at = effective_date − 1 day`) and creates the successor placement **atomically** in one
  tx; if the agent was a shift leader of the vacated unit, auto-end that leadership
  (`vacated_reason = PLACEMENT_ENDED`). Renew supersedes predecessor (`SUPERSEDED`) before insert
  to release the partial unique index (mirrors Phase-4 RenewAgreement).

### Seed Data & E2E (Accept all)
- Seed **active placements** for the seeded personas (SWP-EMP-1042 Sari Hadi, 1108 Rudi Wijaya,
  2891 Budi) at SWP-CMP-0021/0022 + SWP-SITE-0001/0002 so the roster and `/auth/me` placement
  context resolve. Seed **one active shift-leader assignment** at SWP-CMP-0021 (the persona
  shift-leader's company). Seed **one expiring-soon placement** (end_date within N days) so the
  expiring list renders.
- **Exhaustive Playwright E2E (real BE + real FE):** one `test()` per Gherkin scenario / case
  across the E3 PRDs (agent-placement, placement-lifecycle, replacement-transfer,
  shift-leader-assignment, company-roster), **including negative invariant conflicts**: INV_1
  (duplicate active placement → 409), INV_2 (second leader → 409), terminal-state mutation → 409,
  period overlap → 409, COMPANY_INACTIVE, LEADER_NOT_ELIGIBLE. Plus RBAC negatives. Named by
  scenario / BR-# / C-#. Green against the real stack.

### Plan Split & Scope (Accept all)
- Keep ROADMAP's **4 plans**:
  - **05-01** Migrations + sqlc queries (`placements`, `placement_history`, `shift_leader_assignments`).
  - **05-02** Services + handlers: INV-1..4 enforcement, lifecycle state machine, transfer/renew
    atomicity, scope guards (RBAC x-rbac), notification stub points.
  - **05-03** Go contract tests vs `docs/api/E3-placement/openapi.yaml` examples (incl. invariant
    error envelopes, site-scope leadership paths).
  - **05-04** Full-stack Playwright E2E for E3 (per Gherkin AC incl. invariant conflicts).
- **Endpoint scope = FE-used only** (13 hooks per `fe-endpoint-inventory.md` E3): `GET /placements`
  (+ expiring filter), `GET /placements/{id}`, `POST /placements`, `:renew`, `:transfer`, `:end`,
  `:resign`, `:terminate`; `GET /client-companies/{companyId}/roster`; `POST
  /shift-leader-assignments`, `:replace`, `:end`. Non-FE endpoints deferred.
- **Notifications** on lifecycle resolution: stub dispatch points (comment-marked), wire in the
  later notifications epic — same pattern as Phase-4 change-request resolution.
- **Shared-file coordination:** BE slices that edit `server.go` / `main.go` / `cmd/seed/seed.go`
  run **sequentially** via marker coordination (established pattern).

### Claude's Discretion
- Exact `placement_history` row shape and whether base status is a separate column vs derived —
  pick the cleanest that supports history + DTO-boundary lifecycle_status.
- Row-locking granularity (per-agent vs per-company unit) for the invariant checks — pick the
  narrowest lock that is still correct under concurrency.
- Buffer-rule soft-warning surfacing for transfers (contract notes a non-error soft warning) —
  implement if cheap, else note as a stub.
- `expiring_within_days` default N — match the contract example.
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Platform kernel (`internal/platform/*`): httpx cursor/PageResponse, rbac roles+scope guards,
  tx-atomic audit, apperr codes+envelope, ids (SWP-ID allocator — PL/SLA prefixes likely need
  adding to `ids.go`), idempotency, db.TxManager, i18n, Asia/Jakarta TZ layer.
- **Four reference slices to mirror:** identity, foundations, org, **people (Phase 4 — closest
  analog: lifecycle + partial-unique-index + history + atomic supersede)**.
- Phase-3 tables (companies, sites, service_lines) + Phase-4 tables (employees) to FK against.
- E2E harness: boots real stack + resetDb (TRUNCATE + reseed) + loginAs personas + db helpers;
  `window.__swp_get_token__` for authed page.evaluate() requests.

### Established Patterns
- migration → sqlc (`make gen`) → repository (domain mapping, tx writes) → service (apperr codes,
  audit, row-locking for invariants) → hand-written chi handler → routes in `server.go` under
  RequireRole → Go contract tests → FE wiring (drop MSW) + live Playwright E2E.
- Partial unique index for "one active X" invariants (Phase-4 EA-2). Supersede-before-insert to
  release the index on renew (Phase-4 RenewAgreement). apperr struct literals for non-default
  HTTP status (e.g. 409 conflicts via `apperr.Conflict()`).
- DataTable rows are `div.border-b` (not `tr`) — E2E row locators use the `div.border-b.filter()`
  pattern. Toggles are `role=switch`. `noValidate` on RHF+Zod number forms.

### Integration Points
- New query dir `backend/db/queries/placement/` (sqlc glob picks it up). New routes in `server.go`
  authenticated group under RequireRole. New action-suffix routes (`:renew`, `:transfer`, etc.) —
  chi `:` suffix matches natively (Phase-2 finding). `GET /client-companies/{id}/roster` mounts
  under the existing org companies router or a new placement router. Seed extension in
  `cmd/seed/seed.go`. FE screens exist under `frontend/apps/web/src/features/e3-placement/*`
  (built from .pen) calling `@swp/api-client` E3 hooks via MSW — wire to real BE. E2E patterns in
  `frontend/e2e/tests/` + `frontend/e2e/lib/`.
</code_context>

<specifics>
## Specific Ideas
- The seeded shift-leader persona's company (SWP-CMP-0021) must have a coherent placement +
  leadership state so INV-2/4 hold in the seed (leader is actively placed at the unit they lead).
- E2E must trigger the real invariant 409s (not mocked): create a second active placement for an
  already-placed agent → `INV_1_VIOLATION`; assign a second leader to a unit that already has one
  → `INV_2_VIOLATION` / `ACTIVE_LEADER`; mutate a terminal placement → `TERMINAL_STATE_IMMUTABLE`.
- Transfer + renew atomicity must be E2E-observable (source ends, successor active, history rows
  present) — assert via the roster + placement detail after the action.
</specifics>

<deferred>
## Deferred Ideas
- Non-FE E3 endpoints not in the inventory.
- Notification dispatch implementation (stubbed here; later notifications epic).
- Scheduling / attendance / leave / overtime that hang off placement — later phases.
- Full site-scope leadership E2E (modeled + enforced + contract-tested here; FE E2E targets
  company-scope which is what the web roster calls).
</deferred>

---

*Phase: 05-e3-placement*
*Context gathered: 2026-06-04*
