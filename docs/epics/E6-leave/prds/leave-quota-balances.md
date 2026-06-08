# PRD · F6.1 — Leave Balance Ledger (grant-lots)

> **Epic:** E6 Leave Management · **Feature:** F6.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_
> **Model:** per-employee **grant-lot ledger** *(resolved 2026-06-08 — supersedes the per-type `LeaveQuota` model; see [EPICS.md §8](../../../EPICS.md) "E6 — Leave" + FEATURE §4/§7)*.

---

## 1. Context & problem

Leave balance is **one pool per employee**, held as **grant-lots**. Each lot is a single insert — an amount of days with its **own hard expiry** — sourced from the annual auto-grant, an HR adjustment, a bonus, or a pre-funded statutory/maternity allocation. A request **draws down** lots **FIFO by soonest expiry**; the drawdown is recorded per-lot so cancel/restore can reverse exactly what was taken.

`leave_type` is **no longer a balance axis**. It survives only as a **label + document gate (`requires_document`) + calendar color**. Ordinary requests of any type draw the **one unearmarked pool**; an **earmarked** lot (e.g. maternity) is drawn **only** by a request of that purpose and is invisible to ordinary FIFO. This replaces "one `LeaveQuota` row per (employee, leave_type, calendar-year) expiring at year-end."

## 2. Goals & non-goals

**Goals**
- Hold balance as a **per-employee pool of `LeaveGrant` lots**, each with its own `expires_at`.
- Auto-grant the annual entitlement as a single `ANNUAL` lot (`expires_at` = period end).
- Let HR **grant / adjust** a lot (amount, `expires_at`, `earmark`, `remark`), audited — including **pre-funding** long/statutory leave.
- **Allocate FIFO** by soonest expiry across eligible (unexpired, matching-earmark) lots; record per-lot `LeaveConsumption` rows.
- **Reserve** at submit (`pending_days`), **commit** at approve (`consumed_days`), **release/reverse** on reject/cancel/shorten.
- **Expire** lapsed lots (hard per-lot expiry; no carryover).
- **Never** allow a negative balance.

**Non-goals**
- Requesting/approving leave (F6.2/F6.3 — they call the allocation primitives here). Schedule effect (F6.4).
- A per-type quota axis (dropped). Half-day units (full days only).

## 3. Actors

System (annual auto-grant, FIFO allocation, expiry sweep), HR/Super Admin (grants/adjustments/pre-funding), Agent (views balance).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | View ledger; grant/adjust a lot; pre-fund statutory/maternity; bulk annual grant. |
| **Mobile app** | Agent | View own balance — total pool + per-earmarked-lot lines with expiry. |
| System | — | Annual auto-grant; FIFO reserve/commit/release; per-lot expiry sweep. |

## 5. Business rules

> **ID remap note (2026-06-08):** LQ-* are renumbered to the grant-lot model. IDs referenced outside this PRD (LQ-1 grant, LQ-2/LQ-3 deduct/restore, LQ-5 no-negative, LQ-6 HR adjust) are **kept stable** but restated; **LQ-4** (year-end expiry) and **LQ-7** (one quota per type/period) are **superseded/dropped** and annotated. New rules LQ-9..LQ-12 cover lots, FIFO, earmark, and pre-funding.

| Ref | Rule |
|-----|------|
| LQ-1 | **Grant = a lot, not a per-type row.** The annual auto-grant (period start) inserts **one** `LeaveGrant` with `source = ANNUAL`, `amount_days = entitlement`, `expires_at = period_end`, `earmark = null`. Entitlement sources `employment_agreements.annual_leave_entitlement_days` (E2). *(was: create a `LeaveQuota` per quota-tracked type.)* |
| LQ-2 | On a leave **Approved** (F6.3), the request's **reservation is committed**: each reserved lot's `pending_days -= d_lot` and `consumed_days += d_lot`, and a `LeaveConsumption` row (`leave_request_id, grant_id, days`) is written per lot. *(restated from "deduct quota".)* |
| LQ-3 | On an approved leave **cancelled/shortened**, the **exact** `LeaveConsumption` rows are reversed: each lot's `consumed_days -= row.days` (only for restored days, oldest-consumed-first on a shorten); deleted/zeroed consumption rows are audited. *(restated from "restore quota".)* |
| LQ-4 | **Per-lot hard expiry.** A lot is zeroed by the expiry sweep when `now ≥ expires_at` (its remaining can no longer be allocated). **No year-end global expiry, no carryover minting.** *(supersedes the calendar-year "expire at period end" rule — 2026-06-08.)* |
| LQ-5 | Balance can **never go negative** — a request allocates **only** available (unexpired, matching-earmark) lots; if `duration_days` exceeds available, it is **blocked** (F6.2 INV-1). The over-balance path is "HR adds a lot" (LQ-11), never a negative remaining. |
| LQ-6 | HR may **grant or adjust a lot** with a required `remark` — set/adjust `amount_days`, `expires_at`, `earmark` — audited. A negative adjustment cannot bring a lot's `amount_days` below its `consumed_days + pending_days`. *(was: adjust `total`/`remaining`.)* |
| LQ-7 | *(dropped 2026-06-08 — there are no per-type/period quota rows; a lot is the unit. Multiple unexpired lots per employee are normal.)* |
| LQ-8 | **Pro-rate** the annual auto-grant for probation (first 12 months) and mid-year joiners — `amount_days ≈ entitlement × remaining_months / 12` (half-up). Applies to the `ANNUAL` lot's `amount_days` only. |
| LQ-9 | **FIFO allocation.** A request allocates across eligible lots **ordered by soonest `expires_at`** (ties broken by `granted_at`, then `id`), taking from each lot's remaining (`amount − consumed − pending`) until `duration_days` is satisfied. Allocation may **span multiple lots**. |
| LQ-10 | **Earmark isolation.** A lot with `earmark != null` is eligible **only** for a request whose purpose matches that earmark; it is **invisible** to ordinary (unearmarked) FIFO. Unearmarked lots (`earmark = null`) form the flat pool drawn by ordinary requests. |
| LQ-11 | **HR pre-funds long/statutory leave** by inserting an earmarked lot (e.g. `source = MATERNITY, earmark = MATERNITY, amount_days, expires_at, remark`). The employee then files a request of that purpose, which draws **only** that lot (LQ-10). No bypass flag, no separate table. |
| LQ-12 | **Reservation lifecycle.** Submit → FIFO **reserve** (`pending_days += d_lot` per allocated lot, recorded as a provisional allocation). Approve → **commit** (LQ-2). Reject/withdraw/expire-of-draft → **release** (`pending_days -= d_lot`). Reservations count against availability so the agent's pre-check matches approval. |

## 6. Data model

**`LeaveGrant`** (`SWP-LG-*`) — one lot:
`id, employee_id (FK), amount_days (int ≥0), granted_at (date), effective_from (date), expires_at (date), source (enum: ANNUAL | ADJUSTMENT | MATERNITY | STATUTORY | MIGRATION | BONUS), earmark (text, nullable — null = general pool; non-null = purpose code), remark (text), consumed_days (int ≥0), pending_days (int ≥0), created_by, created_at, updated_at`.
Derived: `remaining_days = amount_days − consumed_days − pending_days`. A lot is **active** when `now < expires_at`.

**`LeaveConsumption`** (`SWP-LC-*`) — one lot-drawdown row:
`id, leave_request_id (FK → LeaveRequest), grant_id (FK → LeaveGrant), days (int >0), created_at`.
One request produces one row **per lot** it drew from (multiple when allocation spans lots). Reversal on cancel/shorten deletes/zeroes the affected rows and restores `consumed_days`.

**Balance** (per employee, for the UI):
`unearmarked_total = Σ(amount − consumed − pending)` over active lots with `earmark = null`; plus one line **per active earmarked lot** with `{ earmark, remaining, expires_at }`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Leave balance ledger (grant-lots)

  Scenario: Annual auto-grant writes one ANNUAL lot
    Given "Budi" has an employment agreement with annual_leave_entitlement_days = 12
    When the period-start annual grant runs for 2026
    Then one LeaveGrant lot is created with source ANNUAL, amount_days 12,
      earmark null, and expires_at 2026-12-31

  Scenario: HR grants an adjustment lot
    When HR grants Budi a lot of 2 days with source ADJUSTMENT, expires_at 2026-12-31, remark "Koreksi"
    Then a new unearmarked LeaveGrant lot of amount_days 2 exists and the action is audited
    And Budi's unearmarked pool increases by 2

  Scenario: FIFO consumption across lots by soonest expiry
    Given Budi has lot A (amount 3, expires 2026-06-30) and lot B (amount 5, expires 2026-12-31), both unearmarked
    When a 4-day ordinary leave is approved
    Then 3 days are consumed from lot A and 1 day from lot B
    And two LeaveConsumption rows are written (lot A: 3, lot B: 1)
    And lot A remaining becomes 0 and lot B remaining becomes 4

  Scenario: Earmarked maternity lot is drawn only by a maternity request
    Given HR pre-funds Budi a lot source MATERNITY earmark MATERNITY amount 90 expires 2027-03-31
    And Budi also has an unearmarked pool of 8 days
    When Budi files an ordinary annual request
    Then the maternity lot is invisible to it (it draws only the unearmarked pool)
    When Budi files a maternity request of 90 days
    Then it draws only the MATERNITY lot and the unearmarked pool is untouched

  Scenario: Lot expiry zeroes remaining, no carryover
    Given Budi has lot A with remaining 4 and expires_at 2026-12-31
    When the expiry sweep runs on 2027-01-01
    Then lot A's remaining can no longer be allocated and nothing carries to a new lot

  Scenario: HR pre-funds when a request exceeds the pool (no negative)
    Given Budi's available pool is 2 days
    When Budi requests 5 ordinary days
    Then it is blocked for insufficient balance
    When HR grants a lot of 3 days
    Then Budi may resubmit and the 5 days allocate FIFO across lots — balance never goes negative

  Scenario: Reserve at submit, commit at approve, release on reject
    Given Budi has an unearmarked pool of 5 days
    When Budi submits a 3-day request
    Then 3 days are reserved as pending_days across lots (available drops to 2)
    When HR rejects it
    Then the reservation is released and available returns to 5
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Mid-period join | Pro-rate the `ANNUAL` lot's `amount_days` (LQ-8); other lots unaffected. |
| C-2 | FIFO across differing expiries | Soonest `expires_at` consumed first (LQ-9); allocation may span lots, producing multiple `LeaveConsumption` rows. |
| C-3 | Earmark isolation | An ordinary request **never** touches an earmarked lot; a purpose request draws **only** the matching earmark (LQ-10). |
| C-4 | Lot expires mid-request | Eligibility is evaluated **at allocation time** (submit-reserve and approve-commit each re-check `now < expires_at`); a lot that lapses between submit and approve drops out of FIFO and the commit re-allocates from remaining active lots — if that now under-funds, the approval is blocked (LA-5 re-check) and HR pre-funds. |
| C-5 | Concurrent approvals racing the balance | Reserve/commit are atomic per lot; the second approval re-checks lot remaining and re-allocates or blocks. |
| C-6 | Cancel/shorten reversal | Reverse the **exact** `LeaveConsumption` rows (LQ-3); on shorten, restore only the trailing restored days. |
| C-7 | Migration backfill | Legacy `employee_leave_quotas.leave_remaining` → **one `MIGRATION` lot per employee** with `amount_days = remaining`, `consumed_days = 0`, `earmark = null`, and `expires_at` = the legacy period end (or a configured cutover horizon). No per-type rows; see DATA-MAPPING. |

## 9. Dependencies

E2 (leave-type label/gate; `employment_agreements.annual_leave_entitlement_days` as annual source), F6.2/F6.3 (request/approval call reserve/commit/release), E1 (audit), scheduled jobs (annual auto-grant, expiry sweep), E9 (migration backfill of a `MIGRATION` lot).

## 10. Decisions & open questions

- ✅ **Per-employee grant-lot ledger** (2026-06-08) — one pool; lots with hard per-lot expiry; FIFO-by-soonest-expiry consumption; optional earmark; no negative; HR pre-funds statutory/long leave. `leave_type` = label + document gate + color only. See [EPICS.md §8](../../../EPICS.md) + [FEATURE §4/§7](../FEATURE.md).
- ✅ **Pro-rated** annual lot for probation + mid-year joiners (LQ-8).
- **Open:** does an `ANNUAL` lot's `expires_at` follow strict calendar-year end, or an anniversary horizon for late joiners? (Default: calendar-year end; confirm with SWP.)
- **Open:** earmark vocabulary — is `earmark` free-text purpose codes or a closed enum aligned to statutory leave types? (Default: free-text purpose code matched to the request's leave_type code.)
