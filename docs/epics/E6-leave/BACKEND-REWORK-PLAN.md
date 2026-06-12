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

## Phase 4 — Rewire LeaveService (Opus, core swap)

- `leave_service.go`: replace every `s.grants.*` call in `submit/finalize/reject/cancelApproved/shorten` with `quotaMeter.*`. Set `leave_requests.quota_id` for quota-bearing types. LA-5 re-check at final approval reads the window remaining.
- Wiring in `cmd/api/main.go` / `server.go`: construct `QuotaMeter`, inject into `LeaveService`.
- Gate: build + existing leave_service tests adjusted minimally to green (full test rewrite in Phase 7).

## Phase 5 — Handlers / DTO / routes / OpenAPI (Sonnet)

- `dto.go` + handlers: balance views become per-type (`ListBalances`/`Balance` → per-type lines + window); add `POST /leave-quotas` (set/adjust a type quota) + `POST /leave-quotas/{id}:adjust`; deprecate/remove `/leave-grants*`.
- `server.go`: route swap grants→quotas.
- `docs/api/E6-leave/openapi.yaml`: flip `leave-balances` schemas `LeaveGrant`→`LeaveQuota` (per-type, `cap_basis`, `period_key`, `entitled/used/pending/remaining`); update paths. Then frontend `orval` regen.

## Phase 6 — Seed (Sonnet)

- `cmd/seed/seed.go` `seedLeave`: seed `leave_quotas` per-type fixtures (annual `ANNUAL_POOL` + a couple statutory windows) instead of `leave_grants`/`leave_consumptions`; keep E2E loop-closers (INV-3) intact.

## Phase 7 — Tests (Sonnet, large)

- Rewrite `leave_service_test.go`, `leave_handler_test.go`, `quota_handler_test.go`, `balance_list_handler_test.go`, `leave_testkit_test.go`, `create_leave_request_test.go` for the per-type model. Add per-cap_basis cases.

## Phase 8 — Retire grant-lots (Opus, destructive migration)

- Migration: drop `leave_grants`, `leave_consumptions`; drop legacy `leave_quotas` cols (`total/used/pending/period/period_start/period_end/closed/is_prorated/prorate_months`) + the legacy unique index; drop `leave_requests.balance_earmark/balance_allocation`.
- Delete `grant_service.go`, `grant_repo.go`, `grant_handler.go`, `db/queries/leave/leave_grants.sql`, grant DTOs; remove `LeaveGrantSource`/grant domain types; un-deprecate or remove old quota endpoints.
- Gate: `make gen` clean, full `go test ./...` green.

## Sequencing notes

- Phases 2→4 must land together-ish (4 needs 2/3) but each builds green. 5/6 after 4. 7 alongside 5–6. 8 last, only when nothing references grants.
- Frontend (`apps/web`, `apps/mobile`) leave screens are built from the updated `.pen` AFTER Phase 5 (the contract). The `.pen` is already reworked (per-type).
