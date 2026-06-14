# Backend build conventions (every phase MUST follow)

How each endpoint is implemented. The platform kernel + the **E1 auth slice
(`backend/internal/{handler,service,repository}/identity`) is the reference pattern —
copy its shape.** Don't re-architect; fill in epics.

## Authoritative sources (read before coding a phase)
- `CLAUDE.md` — repo rules. `docs/eng/ENGINEERING.md` — principles. `docs/eng/WEB-STACK.md`.
- `docs/api/CONVENTIONS.md` — the API contract (errors, pagination, RBAC, IDs, idempotency, audit).
- `docs/api/E#-*/openapi.yaml` — per-operation contract (request/response/examples/x-rbac). **The FE client is generated from these; BE responses MUST match byte-for-shape.**
- `docs/epics/E#-*/FEATURE.md` (invariants INV-#, flows) + `prds/*.md` (BR-#, Gherkin AC, cases C-#) — the behavior spec + the E2E test source.
- `backend/README.md` — stack + layout.

## Per-endpoint recipe
1. **Migration** (if the entity's table doesn't exist): `backend/db/migrations/NNNNN_*.sql` (goose), soft-delete (`deleted_at`), SWP id via `'SWP-XXX-' || swp_next_id('XXX')`. Add the prefix to `internal/platform/ids/ids.go` if missing.
2. **Queries**: `backend/db/queries/<epic>/*.sql` (sqlc annotations). `make gen` → `internal/repository/sqlc`. Never hand-edit generated code.
3. **Repository** `internal/repository/<epic>/`: implement the service's port interface over sqlc; map rows → `internal/domain` types; `pgx.ErrNoRows → domain.ErrNotFound`; writes take `pgx.Tx`.
4. **Service** `internal/service/<epic>/`: business logic. Enforce invariants (INV-#) and business rules (BR-#) here, returning `apperr` with the **exact code from CONVENTIONS** (409 `INV_*_VIOLATION`/`DOUBLE_SHIFT`; 422 `QUOTA_EXCEEDED`/`OUT_OF_GEOFENCE`/`RULE_VIOLATION`; etc.). Wrap multi-write + audit + job-enqueue in `db.TxManager.InTx`.
5. **Handler** `internal/handler/<epic>/`: hand-written chi handlers (NO server codegen — oapi-codegen can't parse the 3.1 specs). Decode/validate request → call service → `httpx.WriteJSON`. Errors via `httpx.WriteError`. Cursor lists via `httpx.PageResponse` + `httpx.EncodeCursor/DecodeCursor`.
6. **RBAC**: wrap routes with `rbac.RequireRole(...)` from the op's `x-rbac.roles`; enforce scope in the service with `rbac.GuardCompany` / `rbac.GuardSelf` once the resource's owner is loaded.
7. **Idempotency**: wrap create/action/bulk endpoints flagged in the spec with `d.Idempotency.Handler`.
8. **Audit**: every write calls `audit.Record(ctx, tx, ...)` inside the tx (CONVENTIONS §16.1).
9. **Notifications**: auto-dispatch actions (CONVENTIONS §16.2) enqueue a River `NotificationArgs` job via `jobs.Client.EnqueueTx` in the same tx.
10. **Routing**: mount the epic's routes in `internal/server/server.go` under the authenticated `/api/v1` group.
11. **Contract test** (Go): for each endpoint, a test asserting the response shape matches the spec example (table-driven, hits the handler with a seeded DB via testcontainers). This is the drift gate that replaces server codegen.

## Hard rules
- Match the OpenAPI spec exactly (paths, snake_case JSON, status codes, error envelope). If the spec and reality disagree, the spec wins — fix code, or raise it as a blocker (don't silently diverge).
- All copy/messages: error messages via `i18n` (Bahasa default). Timestamps UTC; local-time fields `HH:MM` Asia/Jakarta.
- Cursor pagination only. No offset.
- `make gen && go build ./... && go vet ./... && gofmt -l` must be clean. `make verify` is the gate.
- Never hand-edit `internal/repository/sqlc` or other generated output.

## Definition of done for an endpoint
Handler+service+repository+queries+migration written · RBAC + scope + idempotency + audit + notifications wired per spec · Go contract test green · the FE screen that calls it works against the real BE in a Playwright E2E (see `e2e-harness-spec.md`).
