# E6 Backend Rework — grant-lots → per-type quota ledger

> Sequenced plan to move the **live** leave balance path from the grant-lot model
> (`leave_grants` + `leave_consumptions` + `GrantService`, migr. 00044) back to the
> **per-type quota** model (`leave_quotas` keyed by window + `cap_basis`), per
> [EPICS §8 "E6 — Leave" 2026-06-12](../../EPICS.md) + [F6.1 PRD](prds/leave-quota-balances.md).
> Each phase ends **build + tests green**. Model column = which model to run it on
> (Opus = logic/destructive; Sonnet = mechanical), per [CLAUDE.md model strategy].

## Current state (verified 2026-06-12)

- **Live path = grant-lots.** `LeaveService.submit` → `GrantService.reserve`; `finalize` → `commit`/`allocate`; `reject` → `release`; cancel/shorten → `reverseConsumptions`. Writes `leave_grants` + `leave_consumptions`.
- **Dead path = old per-type quotas.** `leave_quotas` (00029), `QuotaService`/`quota_repo`, routes `/leave-quotas*` — tagged DEPRECATED 2026-06-08, NOT in the approval state machine.
- Routes/openapi treat `/leave-grants` + `/leave-balances` as authoritative.

## Phase 1 — Foundation schema ✅ DONE (Opus)

- `db/migrations/00051_leave_quotas_pertype.sql`: additive — `leave_quotas` gains `period_key, entitled_days, used_days, pending_days, source(AUTO|ADJUSTMENT|MIGRATION), remark, expires_at, created_by`, backfilled from legacy cols; unique `(employee_id, leave_type_id, period_key)`; `leave_requests.quota_id` FK.
- `db/queries/leave/leave_quotas.sql`: 7 bare-model queries switched to `lq.*`/`RETURNING *` so they keep mapping to the widened model (dead path stays compiling).
- Result: `make gen-sqlc` + `go build ./...` clean; 72 leave tests pass. **Legacy columns kept** (dropped in Phase 8).

## Phase 2 — Per-type quota queries + repo ✅ DONE (Opus)

- `db/queries/leave/leave_quotas.sql`: added `ResolveQuotaWindow` (FOR UPDATE by emp,type,period_key), `OpenQuotaWindow` (upsert at entitled=cap_value/annual entitlement), `ReserveQuotaDays`/`CommitQuotaDays`/`ReleaseQuotaDays`/`ReverseCommittedQuotaDays`, `AdjustQuotaEntitled` (audited), `CountApprovedRequestsForType` (PER_YEAR_COUNT/LIFETIME_ONCE gate).
- `internal/domain/leave/leave.go`: `LeaveTypeCapBasis` enum + `QuotaBearing()`; `QuotaSource`; `QuotaWindowSpec`; `LeaveQuota` gains `PeriodKey/EntitledDays/UsedDays/PendingDays/Source/Remark/ExpiresAt/CreatedBy` + `RemainingPerType()`.
- `internal/repository/leave/quota_repo.go` (+ `mapping.go`): 8 concrete repo methods over the new queries; `mapQuotaFromModel` populates the new fields. **Not yet added to `svc.QuotaRepository`** (avoids breaking in-flight mocks; the meter interface lands in Phase 3).
- Gate met: `make gen-sqlc` + `go build ./...` clean; 72 leave tests pass; vet clean. *(LeaveType cap fields read in Phase 3 via a join query — not needed on a domain LeaveType yet.)*

## Phase 3 — Cap-basis metering + eligibility gates ✅ DONE (Opus)

- `db/queries/leave/leave_meter.sql`: `GetLeaveTypeCap`, `GetEmployeeGateInfo` (gender+join_at), `GetAnnualEntitlementForEmployee` (active agreement).
- `internal/domain/leave/leave.go`: `LeaveTypeCap`, `EmployeeGateInfo`.
- `internal/service/leave/quota_meter.go`: `QuotaMeter` + `QuotaMeterStore`/`QuotaMeterReader` interfaces + `GateError`. `Reserve/Commit/Release/Reverse` dispatch by `cap_basis` (quota-bearing → resolve-or-auto-open + remaining check; `PER_EVENT` → per-occurrence cap, no row; `UNCAPPED` → no day cap; COUNT → charge 1; lifetime/service → EMP window + once gate). Gates: gender / notice_days / min_service_years / lifetime-once. Pure helpers `windowFor`/`chargeFor`/`dayCapped`/`evaluateGates`/`entitlementFor`.
- `internal/repository/leave/quota_repo.go`: 3 reader methods + `var _ QuotaMeterStore/Reader = (*QuotaRepo)(nil)`.
- `internal/service/leave/quota_meter_test.go`: 9 unit tests.
- Gate met: `go build ./...` clean; **81 leave tests pass**; vet clean. **Meter not yet wired into `LeaveService`** (Phase 4). *(Annual pro-ration LQ-8 deferred to the auto-grant job; on-demand auto-open uses full entitlement.)*

## Phase 4 — Rewire LeaveService ✅ DONE (Opus, core swap — strangler)

- `internal/service/leave/quota_meter.go`: `Commit`/`Release`/`Reverse` re-resolve the window from (employee, type, start_date) → **no `quota_id` persistence needed** (avoided churning domain/mapping/queries). `CommitInput`/`WindowOp`; `resolveOrOpen` helper; commit applies the LA-5 remaining recheck unless `Override`.
- `leave_service.go`: added `meter` field + `SetMeter`. All 5 flows (submit / finalize-approve / reject / cancel / cancel-approved / shorten) branch **`if s.meter != nil` → meter path, else legacy grant path**. `mapMeterErr` (GateError → `QUOTA_EXCEEDED`/`RULE_VIOLATION`).
- `cmd/api/main.go`: `leaveSvc.SetMeter(NewQuotaMeter(quotaRepo, quotaRepo))` — **production now meters per-type**.
- Gate met: `go build ./...` clean; **80 leave tests pass**; vet clean; all test binaries compile.
- **Strangler note:** the 2 grant-era service tests leave `meter` nil → still exercise the FIFO fallback (green). The meter path is unit-tested (`quota_meter_test.go`) but **not yet integration-tested through `LeaveService`** — Phase 7 migrates those tests to the meter and removes the grant branch; Phase 8 deletes GrantService.

## Phase 5 — Handlers / DTO / routes / OpenAPI 🟡 PARTIAL (Opus)

**Done — per-type balance read (the FE/mobile-blocking piece):**
- `db/queries/leave/leave_meter.sql` `ListEmployeeLeaveBalances`: every active type LEFT JOIN the employee's current-window quota (CASE on `cap_basis` → year / year-month / EMP).
- `internal/domain/leave/leave.go`: `TypeBalance` + `Remaining()`.
- `internal/repository/leave/quota_repo.go`: `ListEmployeeTypeBalances`; `internal/service/leave/ports.go` interface + `quota_service.go` `EmployeeTypeBalances`; testkit mock updated.
- `internal/handler/leave/{quota_handler,dto}.go`: `GetEmployeeTypeBalances` + `typeBalanceResponse`.
- `internal/server/server.go`: `GET /leave-balances/by-employee/{employee_id}/types`.
- `docs/api/E6-leave/openapi.yaml`: `LeaveTypeBalance` schema + the new path (YAML validated).
- Gate met: `go build ./...` clean; **80 leave tests pass**; vet clean.

**Done — HR per-type quota mutation:**
- `quota_meter.go`: `AdjustEntitled` (resolve-or-open window, refuse entitled < used+pending, audited adj) + `AdjustQuotaEntitled` added to `QuotaMeterStore`.
- `leave_service.go`: `AdjustTypeQuota` (tx + audit wrapper).
- `quota_handler.go`/`dto.go`: `AdjustTypeQuota` + `adjustEntitledRequest`.
- `server.go`: `POST /leave-quotas:adjust-entitled`. openapi path added (validated).

**Deferred to Phase 8 (where GrantService is deleted):** remove `/leave-grants*` + the aggregate grant-pool `GET /leave-balances`; broader openapi `LeaveGrant`→`LeaveQuota`; frontend `orval` regen. *(Kept live now because the grant path is still the meter-nil test fallback.)*

## Phase 6 — Seed ✅ DONE (Opus)

- `cmd/seed/seed.go` `seedLeave`: added `leave_quotas` ANNUAL_POOL windows (Dewi `SWP-LQ-8001` 12/4, Budi `SWP-LQ-8002` 12/11; `period_key=<year>`, expires year-end, `created_by` NULL) mirroring the grant amounts. Grant fixtures kept (harmless; removed Phase 8).
- **DB-verified end-to-end:** `make migrate-up` (00050/00051 applied, schema v54) + `go run ./cmd/seed` clean; `leave_quotas` rows + `leave_requests.quota_id` present; the per-type balance CASE-join returns all 18 types with CT joining the annual window (`has_win=t`, 12/4) and the rest `has_win=f`.

## Phase 7 — Tests ✅ DONE (meter integration; Opus)

- `internal/service/leave/leave_service_meter_test.go` (new): in-memory `QuotaMeterStore`/`Reader` + `newMeterSvc` wires the meter THROUGH `LeaveService`. 5 integration tests — submit opens+reserves, approve-final commits (pending→used), over-cap block, PER_EVENT opens no window, gender gate. Closes the "meter not integration-tested through LeaveService" gap.
- Gate met: **85 leave tests pass** (was 80); build + vet clean.
- **Deferred to Phase 8:** the legacy grant-era tests (`leave_service_test.go` grant cases, `leave_testkit_test.go` grant fakes) are **deleted alongside GrantService** — they validate the meter-nil fallback until then, so removing them now would lose coverage of a still-live path.

## Phase 8 — Retire grant-lots (Opus, destructive) ✅ DONE (B1–B6)

**Landed 2026-06-12** on `feat/backend-impl` in 2 green commits: `cb8124f` (B1–B4: meter-only `LeaveService`, grant code + the grant-lot expiry sweep deleted, tests migrated to an in-memory meter — `meter_fakes_test.go` + service `memStore`) + `eb4ffcf` (B5–B6: migration `00055_drop_grant_lots.sql` drops `leave_grants`/`leave_consumptions` + `leave_requests.balance_earmark/balance_allocation`; `make gen-sqlc`; seed de-granted). Verified: `go build ./...` + `go test ./...` (508 pass) + goose→v55 + `go run ./cmd/seed` on local pg. **Behavior change:** over-cap at approve now surfaces `QUOTA_EXCEEDED` (meter) instead of `BALANCE_RECHECK_FAILED`.

**Deliberately deferred (separate follow-up, NOT in these commits):** the legacy `leave_quotas` columns (`total/used/pending/period/period_start/period_end/closed/is_prorated/prorate_months`) + the dead deprecated `QuotaService.List/Adjust/BulkGrant`/`CheckQuota` + `/leave-quotas` legacy handlers (`OpenQuotaWindow` still inserts the legacy `period*` cols transitionally → own migration + Go-surface removal); and the **OpenAPI grant/deprecated-quota path+schema removal + orval regen** (`docs/api/E6-leave/openapi.yaml` still describes the retired endpoints, ~225 lines).

---

### Original Phase 8 partial notes (kept for reference)

**Done — production surface retired (build-safe):**
- `internal/server/server.go`: removed the grant routes (`GET/POST/PATCH /leave-grants`, aggregate `GET /leave-balances`) + deprecated `/leave-quotas` (GET, `:adjust`, `:bulk-grant`). Handlers/`GrantService` remain as dead code (unrouted) so the tree + 85 tests stay green; testkit registers its own routes so handler tests are unaffected.

**Remaining (atomic destructive — fresh-session runbook below). Start from green: `go build ./... && go test ./internal/...` (85 leave tests pass at this checkpoint).**

### Phase 8 fresh-session runbook (execute in order; build between blocks)

**B1 — meter-only `leave_service.go` (remove fallback).** In each of the 5 flows the pattern is `if s.meter != nil { <meter> } else { <grant> }` (approve ~L300, reject ~L464, submit ~L703, cancel ~L763, cancel-approved ~L818, shorten ~L887). Keep the meter block, delete the `else`/`else if` grant block. Then delete now-unused locals: `quotaTracked`, `earmark`, `committed`, `var alloc`, `availPtr` (and adjust the `writeSnapshot(... earmark, alloc/committed)` calls → pass `nil, nil`). Delete the `grants *GrantService` + `gr GrantRepository` struct fields, the `NewLeaveService` `grants` param (→ `NewLeaveService(repo, schedule, txm)`), and the `if s.grants != nil { s.grants.SetClock(c) }` in `SetClock`.

**B2 — wiring.** `cmd/api/main.go`: drop `grantRepo`/`grantSvc`; `leaveSvc := leavesvc.NewLeaveService(leaveRepo, scheduleRepo, txm)`; keep `SetMeter(NewQuotaMeter(quotaRepo, quotaRepo))`. Delete `quotaSvc := NewQuotaService(...)` if only the deprecated handlers used it (the per-type `EmployeeTypeBalances`/`AdjustTypeQuota` are on `QuotaService` → **keep `QuotaService`**, just drop the grant svc).

**B3 — delete grant code.** `rm` `internal/service/leave/grant_service.go`, `internal/repository/leave/grant_repo.go`, `internal/handler/leave/grant_handler.go`, `db/queries/leave/leave_grants.sql`. Remove from `internal/domain/leave/leave.go`: `LeaveGrant`, `LeaveConsumption`, `LeaveGrantSource`+`ValidGrantSource`, `AllocationLine`, `BalanceCheck.Allocation`/`Earmark` (and the grant `EmployeeLeaveBalance`/`LeaveBalance` types if grant-only). Remove `GrantRepository`/`GrantService` ports + `Grant*Params` from `ports.go`. Remove grant DTOs (`leaveGrantResponse`, `employeeLeaveBalanceResponse`, `leaveBalanceResponse`, `toLeaveBalanceResponse`, grant mappers) from `dto.go`/`mapping.go`. Remove grant handler fields from `Handler` struct + `NewHandler` param.

**B4 — tests.** `rm internal/handler/leave/{grant_handler_test,balance_list_handler_test}.go` (grant endpoints gone). In `leave_service_test.go`: delete the grant-balance assertion tests (those touching `gr`/`lot`/`newFakeGrantRepo`/alloc/maternity); point `newSvc` at an in-memory meter (reuse `memStore`/`memReader` from `leave_service_meter_test.go`) so the surviving state-machine/auth tests run the meter path. In `leave_testkit_test.go`: delete `fakeGrantRepo` + grant route registrations; flows now meter-backed. The new `leave_service_meter_test.go` already covers reserve/commit/release/per-event/gender.

**B5 — drop migration** `000NN_drop_grant_lots.sql`: `DROP TABLE leave_consumptions; DROP TABLE leave_grants;` · `ALTER TABLE leave_requests DROP COLUMN balance_earmark, DROP COLUMN balance_allocation;` · `ALTER TABLE leave_quotas DROP COLUMN total, used, pending, period, period_start, period_end, closed, is_prorated, prorate_months;` + `DROP INDEX leave_quotas_emp_type_period_uq;` (the legacy `period`-keyed unique). Keep a `-- +goose Down` that recreates them (or document irreversibility).

**B6 — verify.** `make gen-sqlc` (clean — no orphaned grant queries) · `go build ./...` · `go test ./...` · `make migrate-up` against local pg. Update openapi: remove `LeaveGrant*`/grant paths + the deprecated `/leave-quotas*`; `orval` regen for the frontend.

## Sequencing notes

- Phases 2→4 must land together-ish (4 needs 2/3) but each builds green. 5/6 after 4. 7 alongside 5–6. 8 last, only when nothing references grants.
- Frontend (`apps/web`, `apps/mobile`) leave screens are built from the updated `.pen` AFTER Phase 5 (the contract). The `.pen` is already reworked (per-type).
