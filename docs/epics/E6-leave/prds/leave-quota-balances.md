# PRD · F6.1 — Leave Quota & Balances

> **Epic:** E6 Leave Management · **Feature:** F6.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Annual leave (cuti tahunan) is entitlement-based: each agent gets a set number of days per period, draws against it, and loses the unused remainder at period end. The system must grant, track (`total`/`used`/`remaining`), and expire these quotas so that requests (F6.2) and approvals (F6.3) can enforce balance.

## 2. Goals & non-goals

**Goals**
- Grant the annual entitlement as a **lump sum per period**.
- Track `used`/`remaining` as leave is approved/cancelled.
- **Expire** unused balance at period end (no carryover).
- Allow HR manual adjustments with audit.

**Non-goals**
- Requesting/approving leave (F6.2/F6.3). Non-annual rule-based types (see FEATURE §7 Q4).

## 3. Actors

System (grant/expire jobs, deductions), HR/Super Admin (grants/adjustments), Agent (views balance).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | View/adjust quotas; trigger/repair grants. |
| **Mobile app** | Agent | View own balance (total/used/remaining + period). |
| System | — | Period-start grant + period-end expiry jobs; deduct on approval. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| LQ-1 | At each **calendar-year** period start, create a `LeaveQuota` **per quota-tracked leave type** (annual + any per-type-quota types, e.g. sick) with `total = entitlement`, `used = 0`, `remaining = total`. |
| LQ-2 | On a leave **Approved** (F6.3), `used += duration_days`, `remaining -= duration_days`. |
| LQ-3 | On an approved leave **cancelled/shortened**, restore `used`/`remaining` accordingly. |
| LQ-4 | At **period end**, unused `remaining` **expires** (no carryover); the quota is closed. |
| LQ-5 | `remaining` can **never go negative** — enforced at request time (F6.2 INV-1). |
| LQ-6 | HR may **manually adjust** `total`/`remaining` with a required reason; audited. |
| LQ-7 | One active quota per employee per **quota-tracked leave type** per period. |
| LQ-8 | **Pro-rate** the grant for probation (first 12 months) and mid-year joiners — `total ≈ entitlement × remaining-months / 12` (rounding rule TBD). |

## 6. Data model

`LeaveQuota`: `id, employee_id (FK), leave_type_id (FK — any quota-tracked type), period_start, period_end (calendar year), total, used, remaining, created_by`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Leave quota & balances

  Scenario: Grant annual quota at period start
    Given "Budi" is entitled to 12 annual leave days
    When the period-start grant runs
    Then a LeaveQuota is created with total 12, used 0, remaining 12

  Scenario: Approved leave deducts from balance
    Given Budi has remaining 12
    When a 3-day annual leave is approved
    Then used becomes 3 and remaining becomes 9

  Scenario: Cancelled leave restores balance
    Given an approved 3-day leave is cancelled
    Then used decreases by 3 and remaining increases by 3

  Scenario: Unused balance expires at period end
    Given Budi has remaining 4 at period end
    When the expiry job runs
    Then remaining becomes 0 and the quota is closed (no carryover)

  Scenario: HR adjusts a quota with a reason
    When HR sets Budi's total to 14 with a reason
    Then the change is applied and audited
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Mid-period join | Pro-rate the grant? — see §10. |
| C-2 | Period basis (anniversary vs calendar year) | Per FEATURE §7 Q2. |
| C-3 | Concurrent approvals racing the balance | Deduction is atomic; second approval re-checks remaining. |
| C-4 | Migration import | `employee_leave_quotas` → LeaveQuota with period + totals (DATA-MAPPING). |

## 9. Dependencies

E2 (annual leave type + entitlement), F6.2/F6.3 (request/approval deductions), E1 (audit), scheduled jobs.

## 10. Decisions & open questions

- ✅ Lump grant per **calendar-year** period; expire at period end; block negative.
- ✅ **Pro-rated** for probation + mid-year joiners (LQ-8).
- ✅ **Per-type quotas** (not just annual) — annual + sick etc. each carry a quota. See EPICS.md §8.
