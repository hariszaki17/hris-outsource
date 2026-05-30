# PRD · F2.2 — Employment Agreement (PKWT/PKWTT + comp)

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Under Indonesian alih-daya law, the employment relationship is between the agent and **SWP** — a fixed-term `PKWT` or indefinite `PKWTT` agreement. This is distinct from a *placement* (E3), which is just a work designation to a client. The agreement is the legal anchor and the source of **current compensation** (base salary, BPJS, tax) that overtime and leave logic read. Legacy buried all of this in the encrypted `employee_contracts` blob mixed with placement data; this feature gives it a clean home.

## 2. Goals & non-goals

**Goals**
- Model PKWT (period-bound) and PKWTT (open-ended) agreements, one active per agent.
- Hold current compensation terms (encrypted), referenced by E7/E8.
- Renew via a linked successor; close on resignation/termination.
- Bound placement validity (E3 reads this for its window/auto-cap).

**Non-goals**
- Placement (E3). Payslip generation/history (E8 — read-only). Payroll runs (out of scope, v1).

## 3. Actors

HR/Placement Admin & Super Admin (author), **Agent** (view own summary), System (validate, encrypt, audit).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR/Super Admin | Create/renew/close agreements; view & edit compensation (role-gated). |
| **Mobile app** | Agent | View own agreement **summary** (type, period, status) — **compensation hidden** unless policy allows payslip view (E8). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| EA-1 | Type ∈ {`PKWT`, `PKWTT`}. `PKWT` **requires** `start_date` + `end_date`; `PKWTT` has `start_date` and **no** `end_date`. |
| EA-2 | An employee has **at most one active** agreement at a time (EObserve INV-2 of E2). |
| EA-3 | Renewal creates a **linked successor** (`predecessor_id`); the prior agreement is closed (status `Superseded`). |
| EA-4 | Compensation fields (`base_salary`, `bpjs_terms`, `tax_profile`) are **encrypted at rest** and visible only to authorized roles. |
| EA-5 | Closing an agreement (resign/terminate/end-of-term) requires a reason + effective date and cascades to active placements (E3) for review. |
| EA-6 | An agreement's validity **bounds placement periods** (E3 BR-1b): PKWT placements auto-cap to the agreement end. |
| EA-7 | All actions audited; comp changes audited with old/new (values masked in the log). |

## 6. Data model

`EmploymentAgreement`: `id, employee_id (FK), type, agreement_no, start_date, end_date (nullable), status, predecessor_id (FK), successor_id (FK), closed_reason, closed_at, created_by`.

`CompensationRecord` (**effective-dated history**): `id, employment_agreement_id (FK), effective_date, base_salary (enc), bpjs_terms (enc json), tax_profile (enc), created_by`. The agreement's "current comp" = the latest record effective as of today; back-dated OT/payroll reads the record effective on the relevant date.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Employment agreement

  Scenario: Create a PKWT agreement
    Given I am an HR admin and "Budi" has no active agreement
    When I create a PKWT agreement with start, end, agreement no., and compensation
    Then the agreement is created as Active
    And the compensation is stored encrypted

  Scenario: Create an open-ended PKWTT agreement
    When I create a PKWTT agreement with a start date and no end date
    Then the agreement is created as Active with no end date

  Scenario: Reject a PKWT without an end date
    When I create a PKWT agreement without an end date
    Then creation is blocked requiring an end date

  Scenario: Renewal creates a linked successor
    Given "Budi" has an active PKWT agreement ending 2026-12-31
    When I renew it for 2027
    Then a new agreement is created with predecessor set to the old one
    And the old agreement becomes "Superseded"

  Scenario: Only one active agreement at a time
    Given "Budi" already has an active agreement
    When I create another active agreement without closing the first
    Then it is blocked or the first is superseded per the renewal flow

  Scenario: Agent cannot see compensation on mobile
    Given I am the agent "Budi"
    When I view my agreement on mobile
    Then I see type, period, and status
    And I do not see salary or BPJS amounts
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Resignation mid-term | Agreement closed `Resigned` at effective date; active placements flagged (E3). |
| C-2 | PKWT renewed as PKWTT (converted to permanent) | Allowed; successor is PKWTT, open-ended. |
| C-3 | Compensation update mid-agreement | Allowed by authorized role; audited (masked); effective-dated. |
| C-4 | Migration: encrypted legacy comp | Decrypt → re-encrypt; failures to review queue (E9). |
| C-5 | PKWT end already passed (historical) | Imported as closed `EndOfTerm`. |

## 9. Dependencies

F2.1 (employee), E1 (RBAC/audit), E3 (placement window/auto-cap), E7 (OT base), E8 (payslip history), E9 (decrypt-migrate).

## 10. Decisions & open questions

- ✅ Agreement carries current comp; PKWT bounds placement; renewal = successor.
- ✅ Mid-agreement comp changes are **effective-dated and historized** via `CompensationRecord` (not overwrite-in-place).
- ✅ Agents view their **own historical payslips (summary)** on mobile (E8); the live agreement compensation amounts remain hidden on mobile.
