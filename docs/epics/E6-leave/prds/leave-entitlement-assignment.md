# PRD — Leave Entitlement Assignment (HR-configured per-employee leave)

> **Status:** RATIFIED 2026-06-15 ([EPICS.md §8](../../../EPICS.md) "E6 — Leave"). Shipped 2026-06-15 (backend + API + frontend).
> **Supersedes:** the *rule-engine* parts of [`leave-quota-balances.md`](./leave-quota-balances.md) — specifically the auto-apply-to-everyone behaviour and the eligibility **gates** (LQ-15: gender / notice / min-service / lifetime), and **INV-7** in [FEATURE.md](../FEATURE.md). The **per-type cap_basis window + reset** mechanics (LQ-13/LQ-14/LQ-4) are **kept**.

## 1. Context

Today leave is **rule-driven**: every active `leave_type` applies to every employee, and eligibility is auto-enforced by hard gates (gender, advance-notice, minimum-service, lifetime-once) baked into the meter. That is rigid and assumes one statutory policy fits all — wrong for an outsourcing provider where different contracts/clients grant different leave packages.

This PRD flips it to an **assignment-driven** model: **HR explicitly enrols each employee into the leave types they get, with a quota per type, at hire.** Only assigned types are requestable. Quotas still **reset naturally per period** (unchanged `period_key` mechanism). HR can adjust quotas, add new leave types to the catalog, and add/remove a type from an employee at any time. **The eligibility gates are dropped** — HR's assignment *is* the eligibility decision. The system becomes a flexible entitlement ledger, not a labour-law engine.

## 2. Goals / Non-goals

**Goals**
- HR assigns, per employee, **which** leave types and **how many** days (manual, per-type).
- Only an employee's **assigned** types appear in their request picker and balance view.
- Period windows **auto-open at the assigned quota** and **reset on the existing `period_key` cadence** (annual / monthly / once / none).
- HR can: edit an employee's quota (base + current window), add a type to an employee, remove a type from an employee, and CRUD the leave-type catalog.
- Agent `Pengajuan › Cuti` shows entitlements as a **table** (type · remaining · used · pending · reset/expiry).

**Non-goals**
- Eligibility gates (gender / notice / min-service / lifetime-once **as a block**). **Dropped** — see §6.
- Document-upload enforcement (deferred; `requires_document` stays as metadata only).
- Leave packages / templates (decided **manual per-type** for now — revisit if onboarding proves slow).
- Approval routing changes (E11 unchanged).

## 3. Actors

- **HR/placement admin** — assigns entitlements at hire, adjusts quotas, manages the catalog.
- **Agent** — sees only assigned types; requests against them.
- **Super admin** — same as HR plus catalog governance.

## 4. Data model

### 4.1 New: `employee_leave_entitlements` (the assignment / "package line")

The **base policy**: which types an employee is entitled to and the per-period quota. One row per (employee, leave_type).

| Column | Type | Notes |
|---|---|---|
| `id` | text PK | `SWP-ELE-…` |
| `employee_id` | text FK → employees | |
| `leave_type_id` | text FK → leave_types | |
| `entitled_days` | integer NULL | Base quota per period. `NULL` = no fixed quota (event/uncapped types HR still toggles on). For COUNT types = occurrences. |
| `active` | boolean NOT NULL default true | Soft on/off — removing a type from an employee = `active=false`. |
| `note` | text | Optional HR note (why granted/changed). |
| `assigned_by` | text FK → users | Audit. |
| `created_at`/`updated_at` | timestamptz | |
| `deleted_at` | timestamptz | Soft-delete. |

- **Unique** `(employee_id, leave_type_id)` where `deleted_at IS NULL`.
- This is the **source of `entitled`** for `leave_quotas` window auto-open (replaces `cap_value` / `employment_agreements.annual_leave_entitlement_days`).

### 4.2 Relationship to `leave_quotas` (unchanged table)

- `employee_leave_entitlements` = **base policy** (which + how many, per period).
- `leave_quotas` = **per-period window instance** (`entitled/used/pending` for one `period_key`), **auto-opened from the entitlement** and reset by `period_key`. No schema change.

```
leave_type (catalog: code, name, cap_basis=reset cadence, requires_document?, applies_to)
   └─ employee_leave_entitlement (per employee: entitled_days, active)   ← HR assigns
         └─ leave_quota (per period_key: entitled, used, pending)        ← auto-opens from entitlement, resets
```

### 4.3 `leave_types` changes

- **Keep** `cap_basis` — now interpreted purely as **reset cadence**: `ANNUAL_POOL`→yearly · `PER_MONTH`→monthly · `PER_YEAR_COUNT`→yearly(count) · `LIFETIME_ONCE`/`SERVICE_UNPAID`→once(no reset) · `PER_EVENT`/`UNCAPPED`→no standing window. *(Optional later: rename to a plain `reset_period` enum `{ANNUAL, MONTHLY, ONCE, NONE}` for clarity — see §10.)*
- **Demote to metadata (not enforced):** `gender`, `notice_days`, `min_service_years`, `lead_days`, `trail_days`. Kept as columns for display/migration, **no longer gate** a request. May be dropped in a later cleanup.
- **Keep** `requires_document` as metadata (enforcement deferred until upload exists).
- `applies_to` / `common` (migr. 00062) stay — used to pre-select sensible types in the assign UI (a soft hint, not a rule).

## 5. Behaviour & business rules (ELE-#)

| # | Rule |
|---|---|
| ELE-1 | **Assignment is required to request.** An employee may request a leave type **only if** an active `employee_leave_entitlement` exists for `(employee, type)`. The picker and balance grid list **only** assigned types. |
| ELE-2 | **Window auto-opens from the entitlement.** On first request in a period, the `leave_quota` window opens at `entitled = entitlement.entitled_days` (`source=AUTO`). Replaces the old `cap_value` / agreement source. If `entitled_days IS NULL` (event/uncapped types), no day-quota is enforced — the request is allowed (subject to the per-event cap if `cap_value` is set on the type). |
| ELE-3 | **Reset is unchanged.** Windows reset by `period_key` per the type's `cap_basis` cadence (annual / monthly / once / none). A new period opens a fresh window at the **current** `entitlement.entitled_days`. **No carryover.** |
| ELE-4 | **HR edits the base entitlement** → affects **future** period windows (and the current one if it opens after the edit). Audited. |
| ELE-5 | **HR adjusts the current window** (existing `:adjust-entitled`) → one-off change to this period's `entitled` only. Audited (LQ-6, kept). |
| ELE-6 | **Add a type to an employee** = insert/reactivate an entitlement row. **Remove** = `active=false` (the type vanishes from the employee's picker/grid; existing windows + history are retained). |
| ELE-7 | **Catalog CRUD unchanged** — HR adds/edits/soft-deletes `leave_types` (E2 master). Soft-deleting a type hides it everywhere; existing entitlements referencing it are treated as inactive. |
| ELE-8 | **No eligibility gates.** Gender / notice / min-service / lifetime-once **do not block** a request (see §6). The only request-time checks that remain: date-range validity, overlap (LR-5), backdate, and **remaining quota** (`QUOTA_EXCEEDED` when `duration > remaining` for quota-bearing types). |
| ELE-9 | **No-negative invariant kept.** An adjustment cannot drop `entitled` below `used + pending` (INV-6). |

## 6. Dropped: eligibility gates

Removed from the request path (`quota_meter.go evaluateGates` + the lifetime-once pre-check):
- `GENDER_MISMATCH`, `INSUFFICIENT_NOTICE`, `INSUFFICIENT_SERVICE`, `ALREADY_USED_LIFETIME`.

Rationale: HR's per-employee assignment is the eligibility control. "Once per employment" is now expressed **naturally** by a `LIFETIME_ONCE` cap_basis (window opens once, never resets → exhausts on use) — no explicit gate needed. Gender/notice/service become **assignment-time** judgement calls by HR, not hard runtime blocks.

**Retained request-time checks:** `INVALID_DATE_RANGE`, `OVERLAPPING_LEAVE`, `BACKDATED_LEAVE`, `QUOTA_EXCEEDED`, (and `MISSING_REQUIRED_DOCUMENT` once upload lands).

## 7. UI changes

### 7.1 Agent `Pengajuan › Cuti` — table (replaces the card grid)

Columns: **Jenis Cuti · Sisa · Terpakai · Pending · Kuota · Reset / Kedaluwarsa**.
- "Reset / Kedaluwarsa": derive from `cap_basis` + `expires_at` — e.g. *"Tahunan · hangus 31 Des 2026"*, *"Bulanan"*, *"Sekali (seumur kerja)"*, *"Per kejadian"*.
- UNCAPPED / NULL-quota rows: Sisa = "Sesuai ketentuan".
- Only the employee's **assigned** types appear (ELE-1). The `common`/expander split is no longer needed (the list is already scoped to what HR granted).

### 7.2 HR — assign entitlements (manual per-type)

A per-employee **"Hak Cuti"** section (on the employee detail screen, or a dedicated screen): a table of assigned types with inline `entitled_days`, an **"Tambah Jenis Cuti"** action (pick a type from the catalog + set quota), and per-row remove (deactivate). Reuses the existing `:adjust-entitled` for current-window tweaks; **base-entitlement** edits are a new endpoint (§8). At hire (E2 onboarding), this section is filled in before activation.

## 8. API surface (new / changed)

- `GET  /employees/{id}/leave-entitlements` — list an employee's assignments (+ current balance per type).
- `POST /employees/{id}/leave-entitlements` — assign a type `{ leave_type_id, entitled_days, note }`.
- `PATCH /employees/{id}/leave-entitlements/{type_id}` — edit base `entitled_days` / reactivate.
- `DELETE /employees/{id}/leave-entitlements/{type_id}` — deactivate (remove from employee).
- **Unchanged:** `POST /leave-quotas:adjust-entitled` (current-window one-off), `GET /leave-balances/by-employee/{id}/types` (now returns only assigned types), leave-type catalog CRUD.

## 9. Migration & backfill

- New migration: `employee_leave_entitlements` table.
- **`entitlementFor()` reads the entitlement table**, falling back to `cap_value` when no row exists — so the system keeps working through the transition (nothing breaks before backfill).
- **Backfill** (seed + E9): create entitlement rows for existing employees from current assumptions — CT from `employment_agreements.annual_leave_entitlement_days`, plus the `applies_to ∈ {AGENT, ALL}` set at their `cap_value`. Demo personas seeded explicitly.
- Gate columns (`gender`/`notice_days`/`min_service_years`/`lead_days`/`trail_days`) left in place (metadata) — a later migration may drop them.

## 10. Open questions

- **Rename `cap_basis` → `reset_period`?** Cleaner for HR ("how often does it reset") vs the current statutory-flavoured taxonomy. Bigger refactor; deferred.
- **Event/uncapped types** (`PER_EVENT`/`UNCAPPED`): does HR assign a number, or just toggle on/off (quota = the type's `cap_value` per event)? Proposal: toggle on/off; `entitled_days NULL`; per-event cap from the type.
- **Default at hire**: manual per-type chosen — monitor onboarding effort; a "duplicate from another employee" or package shortcut may become necessary.
- **Requires-document**: keep the flag but unenforced until the upload flow exists, or hide doc-required types from the picker (current behaviour)?

## 11. Dependencies

E2 (employee onboarding screen hosts the assign UI; `leave_types` catalog), E6 (`leave_quotas` window + `:adjust-entitled` reused; `entitlementFor` rewired), E1 (audit on every assign/adjust), E9 (migration backfill), E11 (approval routing unchanged).

## 12. Decisions

- **Resolved 2026-06-15 (EPICS §8)** — Model is **HR-assigned per-employee entitlement**, **manual per-type** at hire, **eligibility gates dropped entirely**. Period reset retained via `period_key`/`cap_basis`.
