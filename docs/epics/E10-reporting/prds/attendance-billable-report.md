# PRD · F10.3 — Attendance & Billable-Hours Report (v1 priority)

> **Epic:** E10 Reporting & Notifications · **Feature:** F10.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

This is the core outsource revenue artifact: **how many verified, billable hours** each agent worked at each client site over a period. SWP uses it to bill clients (billing math happens outside the system) and to analyze utilization. It's the **must-have v1 report**.

## 2. Goals & non-goals

**Goals**
- Aggregate **verified** attendance on **billable** codes into hours by agent / client company / service line / period.
- Filterable; exportable (via F10.4); internal-only.

**Non-goals**
- Applying rates / computing invoice amounts (done outside the system — hours only). Client delivery/portal (internal-only).

## 3. Actors

HR / Super Admin (all), Shift Leader (own company), System (aggregate, scope).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Shift Leader | Run, view, filter, and export the report. |
| **Mobile** | — | Not a primary surface (heavy tabular report). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| BR-1 | Counts only **verified** attendance (E5) recorded against **billable** attendance codes (E2 `is_billable`) (INV-4). |
| BR-2 | Aggregations: hours by **agent**, **client company**, **service line**, and **period** (day/week/month). |
| BR-3 | Distinguishes **billable** vs **payable** vs total worked hours where relevant. |
| BR-4 | **Scope:** HR/Super Admin see all; a shift leader sees **their company** only. |
| BR-5 | **Hours only** — no rates/amounts applied here (assumed; see §10). |
| BR-6 | Excludes **unverified** records from billable totals (may show pending separately). |
| BR-7 | Exports go through F10.4 and are **audited**; cross-midnight hours counted once to the start date. |

## 6. Data model

Read projection over `Attendance` + `AttendanceCode` + `Placement` + `ClientCompany` + `ServiceLine`. No new entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Attendance & billable-hours report

  Scenario: Billable hours per client for a month
    Given I am HR
    When I run the report for "Plaza Senayan", June, grouped by service line
    Then I get verified billable hours per agent and service-line totals

  Scenario: Only verified billable codes count
    Given some June records are unverified or on non-billable codes
    Then those are excluded from the billable totals

  Scenario: Leader sees only their company
    Given I am the shift leader of "Plaza Senayan"
    When I run the report
    Then I can only report on "Plaza Senayan"

  Scenario: Export the report
    When I export the filtered report
    Then a file is produced via the export framework and the export is audited

  Scenario: Hours only (no amounts)
    Then the report shows hours, not invoice amounts
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Pending (unverified) records at report time | Excluded from billable; optionally shown as "pending". |
| C-2 | Corrections after report run (E5 F5.4) | Re-running reflects corrected hours; prior exports are point-in-time. |
| C-3 | Agent placed at multiple companies in the period (transfer) | Hours split per company/placement window. |
| C-4 | Cross-midnight shift | Counted once to the start date. |

## 9. Dependencies

E5 (verified attendance), E2 (billable codes), E3 (placement/company/service line), F10.4 (export), E1 (scope/audit).

## 10. Decisions & open questions

- ✅ Verified + billable only; hours by agent/company/service-line/period; internal; export-capable.
- **Open (§ INV-4 / C-1):** confirm unverified records never count.
- **Open:** does this report ever apply **rates** (hours × rate → amount), or strictly hours with billing computed outside? (assumed: hours only.)
