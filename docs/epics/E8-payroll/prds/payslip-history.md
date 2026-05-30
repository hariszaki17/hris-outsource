# PRD · F8.1 — Payslip History (read-only summaries)

> **Epic:** E8 Payroll Data · **Feature:** F8.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

After migration, agents and HR need to see **past payslips** for continuity — agents to check what they were paid, HR for lookups. Per decision this is **read-only, summary-level** (take-home, gross earnings, gross deductions, working days, period). No active payroll, no component breakdown at this level.

## 2. Goals & non-goals

**Goals**
- Agents view their **own** payslip summaries on mobile (read-only).
- HR views any agent's payslip summaries.
- Decrypt monetary fields on read for authorized users only.

**Non-goals**
- Component breakdown / benefits (HR archive, F8.2). Payroll runs (out of scope). Editing (read-only).

## 3. Actors

Agent (own, mobile), HR/Super Admin (all), System (scope, decrypt, audit).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | List + open own payslip summaries. |
| **Web console** | HR / Super Admin | View any agent's payslip summaries. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| PH-1 | A payslip summary shows: period (year/month), `paid_on`, `working_days`, `gross_earnings`, `gross_deductions`, `take_home_pay`. |
| PH-2 | **Scope:** an agent sees only their **own** payslips; HR/Super Admin see all. |
| PH-3 | Monetary fields are **decrypted on read** only for authorized viewers; transport is secured. |
| PH-4 | **Read-only** — no creation/edits in-app (INV-1). |
| PH-5 | No component breakdown here (that's HR-only, F8.2). |
| PH-6 | Viewing/exporting a payslip is **audited** (who viewed whose). |

## 6. Data model

Reads `Payslip` (FEATURE §4). No new entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Payslip history (summary)

  Scenario: Agent views own payslips
    Given I am the agent "Budi" with migrated payslips
    When I open "My payslips" on mobile
    Then I see a list by period with take-home, gross, and pay date
    And I can open a payslip summary

  Scenario: Agent cannot see others' payslips
    When "Budi" tries to access another agent's payslip
    Then access is denied

  Scenario: HR views any agent's payslip summary
    Given I am HR
    When I open Budi's payslip for 2025-12
    Then I see the summary figures

  Scenario: No component breakdown at summary level
    When Budi opens a payslip
    Then he sees totals only, not the salary-component line items

  Scenario: Read-only
    When anyone attempts to edit a payslip
    Then it is not permitted
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent with no payslip history (new hire) | Empty state. |
| C-2 | Decryption failure on a record | Show unavailable + flag (migration review), don't crash. |
| C-3 | Payslip predates the agent's current employment agreement | Still shown (history independent of current agreement). |
| C-4 | Currency/format | Render IDR consistently. |

## 9. Dependencies

E2 (employee), E9 (migrated payslips, decryption), E1 (RBAC/audit).

## 10. Decisions & open questions

- ✅ Read-only summary; agent own (mobile) + HR all.
- **Open:** can agents **download/print** a payslip PDF, or view-only in-app?
