# E6 — Leave · ims-system → hris-outsource Data Mapping

> Migration analysis for leave. Source: SWP prod (`lumen_swp`, MySQL). Target: hris-outsource (Postgres).
> Parent: [FEATURE.md](FEATURE.md) · Related epic: E9 Data Migration. Status: Draft v1.

---

## 1. Shape of the change

Legacy `leaves` carries a single `status` **string** (no multi-level approval) and `employee_leave_quotas` tracks **one annual quota per period per employee** (no leave-type link). hris-outsource adds **two-level approval** (reconstructed as final state for history) and replaces the per-type quota with a **per-employee grant-lot ledger** *(2026-06-08)* — the legacy quota's remaining balance backfills as **one unearmarked `MIGRATION` `LeaveGrant` lot** per employee (`leave_type` is no longer a balance axis). Documents were attached via the polymorphic `File` table, not a column.

## 2. Source tables

| Table | → Target | Key columns |
|-------|----------|-------------|
| `leaves` | LeaveRequest | `id, type(int), employee_id, delegate_id, status, notes, admin_notes, duration, start_date, end_date, issued_date, deleted_at` |
| `employee_leave_quotas` | LeaveGrant (one `MIGRATION` lot/employee) | `id, employee_id, leave_total, leave_used, leave_remaining, start_period, end_period, deleted_at` |
| `leave_types` | LeaveType (E2) | mapped in E2 |
| `files` (morph `fileable`) | LeaveRequest.document_url | leave documents attached polymorphically |

## 3. Field mapping

### `leaves` → LeaveRequest
| Legacy | → | Notes |
|---|---|---|
| `id` | `legacy_leave_id` | crosswalk |
| `type` (int) | `leave_type_id` | remap to migrated LeaveType (build lookup; confirm legacy type ids, G-1) |
| `employee_id` | `employee_id` | remap |
| `delegate_id` | `delegate_id` | remap (nullable) |
| `status` (string) | `status` | normalize → {Pending, LeaderApproved, Approved, Rejected, Cancelled}; historical approved → `Approved` (G-2) |
| `notes` | `notes` | — |
| `admin_notes` | `admin_notes` | — |
| `duration` | `duration_days` | carry as-is; semantics (calendar vs working) unconfirmed (G-6) |
| `start_date` / `end_date` | same | datetime → date |
| `issued_date` | `issued_at` | — |
| `deleted_at` | soft delete | — |
| `File` morph | `document_url` | map attached file if present, else null (G-3) |
| — | `LeaveApproval` rows | historical single-status → no per-level approver data; create one imputed `Approved`/`Rejected` record or leave empty (G-2) |

### `employee_leave_quotas` → LeaveGrant *(grant-lot ledger, 2026-06-08 — supersedes the old LeaveQuota mapping)*
Legacy quota rows backfill the new per-employee ledger as **one `MIGRATION` grant-lot per employee** (no per-type rows, no `LeaveType` link — leave_type is no longer a balance axis). See [leave-quota-balances PRD C-7](prds/leave-quota-balances.md) + [EPICS.md §8](../../EPICS.md).

| Legacy | → | Notes |
|---|---|---|
| `employee_id` | `LeaveGrant.employee_id` | remap |
| `leave_remaining` | `LeaveGrant.amount_days` | carry the **remaining** balance as the lot's amount (`consumed_days = 0`, `pending_days = 0`) — `leave_total`/`leave_used` are historical and not re-projected (G-5). |
| — | `LeaveGrant.source` | constant `MIGRATION`. |
| — | `LeaveGrant.earmark` | `null` (legacy quota is the general pool, annual-only, G-5). |
| `end_period` | `LeaveGrant.expires_at` | datetime → date; legacy period end, or a configured cutover horizon if absent (G-5). |
| `start_period` | `LeaveGrant.effective_from` / `granted_at` | datetime → date. |
| — | `LeaveGrant.remark` | constant note, e.g. `"Backfill saldo cuti dari lumen_swp"`. |
| `id` | `legacy_quota_id` | crosswalk for idempotency. |

## 4. Gaps & decisions

| # | Gap | Handling |
|---|-----|----------|
| G-1 | **`leaves.type` is an int** | Map to migrated LeaveType ids; confirm the legacy type→name mapping from prod data. |
| G-2 | **No multi-level approval history** | Legacy single `status` → import final state only; `LeaveApproval` history not reconstructable (impute one record or leave empty). |
| G-3 | **Documents via `File` morph** | Pull leave documents from the polymorphic `files` table (fileable = leave); map to `document_url`; else null. |
| G-4 | **Identity remap** | `employee_id`, `delegate_id` via crosswalk. |
| G-5 | **Quota is annual-only** | Legacy quota is the general (annual) pool; backfill its `leave_remaining` as **one unearmarked `MIGRATION` grant-lot** per employee (no per-type rows, no `LeaveType` link — leave_type is no longer a balance axis, 2026-06-08). `leave_total`/`leave_used` are historical context only. |
| G-6 | **Duration semantics** | Legacy `duration` (calendar vs working days) unknown — carry as-is; confirm the rule for go-forward (FEATURE §7 Q1). |
| G-7 | **No schedule/attendance link** | Historical leave won't retro-cancel schedules; only go-forward leaves trigger F6.4 integration. |

## 5. Migration rules

1. Runs after E2 (leave types) and E3 (employees). Quotas + leaves import with final status; no re-approval.
2. Idempotent via `legacy_leave_id` / quota crosswalk.
3. Historical leaves do **not** retroactively alter migrated schedules/attendance (G-7).
4. Orchestration in E9; this doc owns the E6 field-level mapping.
