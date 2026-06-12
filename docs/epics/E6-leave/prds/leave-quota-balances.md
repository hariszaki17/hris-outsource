# PRD · F6.1 — Leave Entitlement Ledger (per-type quotas)

> **Epic:** E6 Leave Management · **Feature:** F6.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_
> **Model:** **per-type entitlement ledger** *(resolved 2026-06-12 — supersedes the 2026-06-08 grant-lot/one-pool model; see [EPICS.md §8](../../../EPICS.md) "E6 — Leave" + FEATURE §4/§7)*. Each `leave_type` carries its own `cap_basis`; entitlement is metered **per type, in its own window**.

---

## 1. Context & problem

Leave entitlement is **per leave type**, not one pool. SWP's `Fitur Ijin` policy defines **18 types** each with its own statutory cap, and under Indonesian law (Pasal 93 vs Pasal 79 UU 13/2003 / PP 35/2021) event/sick/religious leave is **separate from** the 12-day annual leave — a marriage or bereavement must **not** deplete the annual pool. So `leave_type` is the **cap axis**: each type's `cap_basis` (E2 master) decides how it meters.

| `cap_basis` | Window | Stored as | Example codes |
|---|---|---|---|
| `ANNUAL_POOL` | calendar year, **expires year-end, no carryover** | `LeaveQuota` per (emp, type, year), `entitled` from E2 agreement | CTHO, CT |
| `PER_MONTH` | calendar month, **resets monthly** | `LeaveQuota` per (emp, type, year-month), `entitled = cap_value` | CH, KGD |
| `PER_YEAR_COUNT` | calendar year, counts **occurrences** | `LeaveQuota` per (emp, type, year), `entitled = cap_value` (COUNT) | STSD |
| `LIFETIME_ONCE` | once per employment | `LeaveQuota` per (emp, type, `EMP`), one-time | CM, CIH, CIU, CPR |
| `SERVICE_UNPAID` | once per employment, eligibility-gated, **unpaid** | `LeaveQuota` per (emp, type, `EMP`) | CLTP |
| `PER_EVENT` | per occurrence, **no standing row** | validated at request: `duration ≤ cap_value` | CIM, CKA, CMA, CKM, CRM |
| `UNCAPPED` | unbounded, **no standing row** | document gate only | SDSKD, CTN, CAP |

This **reinstates** the per-type `LeaveQuota` (dropped 2026-06-08) and **drops** `LeaveGrant`/`LeaveConsumption` (the grant-lot pool, FIFO, earmark, and per-lot expiry are no longer used).

## 2. Goals & non-goals

**Goals**
- Meter entitlement **per leave type** via its `cap_basis` window (table above).
- Auto-grant the **`ANNUAL_POOL`** quota at year start from `employment_agreements.annual_leave_entitlement_days` (E2), pro-rated for probation / mid-year joiners; expire it at year-end (no carryover).
- Auto-open quota-bearing windows on first use (`PER_MONTH`/`PER_YEAR_COUNT`/`LIFETIME_ONCE`/`SERVICE_UNPAID`) at `entitled = cap_value`.
- Let HR **adjust** a quota (`entitled_days`, `remark`), audited.
- **Reserve** at submit (`pending_days`), **commit** at approve (`used_days`), **release** on reject/cancel/shorten.
- Enforce **eligibility gates** (gender, notice, min-service, lifetime-once) and **never** a negative balance.

**Non-goals**
- Requesting/approving leave (F6.2/F6.3 — they call the metering primitives here). Schedule effect (F6.4).
- The leave-type catalog/cap definitions themselves (E2 master, operational-master-data §5a). Half-day units (full days only).
- Earmarks / grant-lots / FIFO across lots (dropped 2026-06-12).

## 3. Actors

System (annual auto-grant, window auto-open, year/month rollover), HR/Super Admin (quota adjustments), Agent (views per-type balance).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | View per-type ledger; adjust a type's quota; bulk annual grant. |
| **Mobile app** | Agent | View own balance — a line **per leave type** with its remaining + window. |
| System | — | Annual auto-grant; window auto-open; reserve/commit/release; year/month rollover (expire/reset). |

## 5. Business rules

> **ID note (2026-06-12):** LQ-* are restored to the per-type model. LQ-1 (annual grant), LQ-2/LQ-3 (deduct/restore), LQ-4 (expiry), LQ-5 (no-negative), LQ-6 (HR adjust), LQ-7 (one quota per type/window) are **kept and restated**. The grant-lot rules LQ-9..LQ-12 (FIFO, earmark, pre-fund) are **dropped**. New rules LQ-13..LQ-16 cover `cap_basis` metering and gates.

| Ref | Rule |
|-----|------|
| LQ-1 | **Annual auto-grant.** At year start the system inserts the employee's **`ANNUAL_POOL`** quota (whichever annual type applies — CTHO or CT) with `entitled_days = annual_leave_entitlement_days` (E2), `period_key = <year>`, `expires_at = <year>-12-31`, `source = AUTO`. |
| LQ-2 | On a leave **Approved** (F6.3), the request **commits** its reservation on the type's window quota: `pending_days -= d` and `used_days += d`. |
| LQ-3 | On an approved leave **cancelled/shortened**, the commit is reversed on the **same** window quota: `used_days -= d_restored` (audited). A `PER_EVENT`/`UNCAPPED` request has no quota row to adjust. |
| LQ-4 | **Window expiry/reset.** `ANNUAL_POOL` quota expires at `expires_at` (year-end) — **no carryover, no carry-in**. `PER_MONTH` and `PER_YEAR_COUNT` windows **reset** each new month/year (a fresh quota opens at `cap_value`). `LIFETIME_ONCE`/`SERVICE_UNPAID` never reset. |
| LQ-5 | A request's window **remaining = `entitled − used − pending`** can **never go negative** — over-cap is **blocked** (F6.2 INV-1). The over-cap path is "HR adjusts the quota" (LQ-6), never a negative remaining. |
| LQ-6 | HR may **adjust** a type's quota with a required `remark` — set `entitled_days` — audited. A negative adjustment cannot bring `entitled_days` below `used_days + pending_days`. |
| LQ-7 | **One quota per (employee, leave_type, window).** Quota-bearing bases hold exactly one row per window; `PER_EVENT`/`UNCAPPED` hold none. |
| LQ-8 | **Pro-rate** the `ANNUAL_POOL` grant for probation (first 12 months) and mid-year joiners — `entitled ≈ entitlement × remaining_months / 12` (half-up). Other windows are not pro-rated (statutory caps apply in full). |
| LQ-13 | **`cap_basis` metering.** `ANNUAL_POOL`/`PER_MONTH`/`SERVICE_UNPAID` charge **days** against the window; `PER_YEAR_COUNT` charges **1 occurrence** per request (and still records `duration_days` on the request); `LIFETIME_ONCE` charges the days and **exhausts** on first approval; `PER_EVENT` enforces `duration_days ≤ cap_value` per occurrence with **no** standing row; `UNCAPPED` enforces only the document gate. |
| LQ-14 | **Auto-open windows.** When a request targets a quota-bearing window with no row yet, the system **opens** it at `entitled = cap_value` (`source = AUTO`), then reserves. `ANNUAL_POOL` is the exception — its `entitled` comes from E2 (LQ-1), not `cap_value`. |
| LQ-15 | **Eligibility gates (INV-7).** Block at request time unless: `gender = ANY` **or** matches `employee.gender`; `start_date − today ≥ notice_days`; employee tenure `≥ min_service_years`; for `LIFETIME_ONCE`/`SERVICE_UNPAID` **no prior approved request** of that type. The failing gate is returned as the block reason. |
| LQ-16 | **Paid flag.** `LeaveType.paid = false` (e.g. `CLTP`) marks the approved days **unpaid** — surfaced to payroll (E8); does not change metering. |
| LQ-12 | **Reservation lifecycle.** Submit → **reserve** (`pending_days += d` on the window). Approve → **commit** (LQ-2). Reject/withdraw → **release** (`pending_days -= d`). Reservations count against `remaining` so the agent's pre-check matches approval. |

## 6. Data model

**`LeaveQuota`** (`SWP-LQ-*`) — one per-type window:
`id, employee_id (FK), leave_type_id (FK), period_key (text — `<year>` | `<year-month>` | `EMP`), entitled_days (int ≥0 — occurrence count when `cap_unit = COUNT`), used_days (int ≥0), pending_days (int ≥0), expires_at (date, nullable — set for ANNUAL_POOL), source (enum: AUTO | ADJUSTMENT | MIGRATION), remark (text), created_by, created_at, updated_at`.
Derived: `remaining = entitled_days − used_days − pending_days`. Unique on `(employee_id, leave_type_id, period_key)` among non-deleted rows.

**Reads `LeaveType`** (E2): `cap_basis, cap_value, cap_unit, paid, gender, requires_document, notice_days, min_service_years, lead_days, trail_days` — the metering rules.

**Balance** (per employee, for the UI): one line **per active leave type** — `{ code, name, cap_basis, remaining, window_label, expires_at? }`. `UNCAPPED` types show "sesuai ketentuan"; `PER_EVENT` show the per-occurrence cap.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Leave entitlement ledger (per-type)

  Scenario: Annual auto-grant opens the ANNUAL_POOL quota
    Given "Budi" has an employment agreement with annual_leave_entitlement_days = 12
    And his annual leave type is CT (cap_basis ANNUAL_POOL)
    When the year-start annual grant runs for 2026
    Then a LeaveQuota row exists for (Budi, CT, "2026") with entitled_days 12, source AUTO,
      and expires_at 2026-12-31

  Scenario: Statutory event leave does not touch the annual pool
    Given Budi has 9 days remaining on his CT 2026 quota
    When Budi takes 2 approved days of CKM (bereavement, PER_EVENT cap 2)
    Then his CT 2026 remaining is still 9
    And CKM is enforced as ≤ 2 days for that occurrence with no standing quota row

  Scenario: PER_MONTH cap resets each month
    Given CH (menstrual) is PER_MONTH cap 2 and Budi's employee is female "Sari"
    When Sari takes 2 approved CH days in March
    Then her CH March remaining is 0
    And in April a fresh CH window opens at 2

  Scenario: PER_YEAR_COUNT charges occurrences
    Given STSD is PER_YEAR_COUNT cap 5
    When the agent files a 1-day STSD request 5 times in 2026, all approved
    Then the 6th STSD request in 2026 is blocked (count exhausted)

  Scenario: LIFETIME_ONCE exhausts after first use
    Given CM (own marriage) is LIFETIME_ONCE cap 3
    When the agent takes 3 approved CM days
    Then any later CM request is blocked as already used

  Scenario: Gender gate
    Given CH is gender FEMALE
    When a male employee requests CH
    Then it is blocked with a gender-eligibility reason

  Scenario: Notice gate for religious leave
    Given CIU is LIFETIME_ONCE with notice_days 30
    When the agent requests CIU starting in 10 days
    Then it is blocked for insufficient advance notice

  Scenario: HR adjusts a quota (no negative)
    Given Budi's CT 2026 quota is entitled 12, used 9, pending 0
    When HR adjusts entitled to 8 with a remark
    Then it is blocked because 8 < used+pending (9)

  Scenario: Reserve at submit, commit at approve, release on reject
    Given Budi's CT 2026 remaining is 5
    When Budi submits a 3-day CT request
    Then 3 days are reserved as pending_days (remaining drops to 2)
    When HR rejects it
    Then the reservation is released and remaining returns to 5
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Mid-period join | Pro-rate the `ANNUAL_POOL` quota (LQ-8); statutory caps apply in full. |
| C-2 | Request spans a month boundary (`PER_MONTH` type) | Block in v1 — a `PER_MONTH` request must fall within one window; split across months not supported (confirm with SWP). |
| C-3 | `UNCAPPED` sick (SDSKD) | No quota row; only the document gate applies; duration recorded on the request for reporting. |
| C-4 | `PER_EVENT` over cap | A CKM request of 3 days is blocked (cap 2); no quota row is created. |
| C-5 | Concurrent approvals racing a window | Reserve/commit are atomic on the window row; the second approval re-checks `remaining` and blocks if exhausted. |
| C-6 | Cancel/shorten reversal | Reverse the commit on the **same** window quota (LQ-3); `PER_EVENT`/`UNCAPPED` have nothing to reverse. |
| C-7 | Migration backfill | Legacy `employee_leave_quotas.leave_remaining` → **one `ANNUAL_POOL` `MIGRATION` quota** per employee (`entitled = remaining`, `used = 0`, `expires_at` = legacy period end). No per-statutory-type backfill (legacy had no per-type quotas); see DATA-MAPPING. |
| C-8 | `CIH` hajj duration | Validated by the program dates on the document + `lead_days`/`trail_days`; `LIFETIME_ONCE`, no fixed `cap_value`. |

## 9. Dependencies

E2 (leave-type cap mechanics; `employment_agreements.annual_leave_entitlement_days` as `ANNUAL_POOL` source; `employees.gender`, tenure for gates), F6.2/F6.3 (request/approval call reserve/commit/release), E1 (audit), scheduled jobs (annual auto-grant, year/month rollover), E8 (unpaid-leave payroll effect), E9 (migration backfill of the annual quota).

## 10. Decisions & open questions

- ✅ **Per-type entitlement ledger** (2026-06-12) — `leave_type` is the cap axis; each type meters in its own `cap_basis` window; statutory/sick/religious leave never depletes the annual pool; no negative; HR adjusts a type's quota. Drops grant-lots/earmarks/FIFO. See [EPICS.md §8](../../../EPICS.md) + [FEATURE §4/§7](../FEATURE.md) + [E2 catalog §5a](../../E2-identity/prds/operational-master-data.md).
- ✅ **Pro-rated** `ANNUAL_POOL` for probation + mid-year joiners (LQ-8); statutory caps in full.
- **Open:** `PER_MONTH` request spanning a month boundary — block (C-2) or split? (Default: block; confirm with SWP.)
- **Open:** does STSD count **occurrences** or **days** toward its yearly cap? Policy says "5 kali setahun" → occurrences (modeled as `PER_YEAR_COUNT`); confirm a multi-day STSD still counts as one occurrence and whether it then requires a doctor's letter (→ SDSKD).
- **Open:** working-day vs calendar-day duration for 24/7 shift workers (FEATURE §7 still-open Q1) — affects how `duration_days` charges day-based windows.
