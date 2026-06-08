# PRD · F5.5 — Attendance Records & Dashboard

> **Epic:** E5 Attendance · **Feature:** F5.5 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Everyone needs to *see* attendance: agents review their own history, shift leaders monitor today's roster and exceptions, and HR pulls **billable** and payable rollups for clients and payroll context. Because outsourcing revenue depends on hours actually worked at client sites, the **billable** view (attendance codes flagged billable, E2) is a first-class output feeding reporting (E10).

## 2. Goals & non-goals

**Goals**
- Agent self-history (mobile); leader/HR team views with exception highlighting.
- Filters (company, site, service line, position, date, status, exception) + billable/payable rollups.
- Export feeding E10.

**Non-goals**
- Clocking (F5.1), evaluation (F5.2), verification (F5.3), corrections (F5.4). Full reporting suite (E10).

## 3. Actors

Agent (self), Shift Leader (own company), HR/Super Admin (all), System (query, scope, export).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | Own attendance history + statuses + correction status. |
| **Web / mobile** | Shift Leader | Team attendance, today's roster, exceptions for their company. |
| **Web console** | HR / Super Admin | Cross-company views, billable/payable rollups, exports. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| AR-1 | **Scope:** agent sees only own; leader sees own company; HR/Super Admin see all. |
| AR-2 | Records show: date, scheduled shift, check-in/out times, geofence result, status, verification status, attendance code, corrections. |
| AR-3 | Filters: **company, site, service line, position**, date range, status (Present/Late/Incomplete/Absent), verification status, exception-only. (`company`/`site`/`service_line`/`position` map 1:1 to the denormalized columns on `Attendance`.) |
| AR-4 | **Billable rollup:** sum worked records whose attendance code `is_billable` (E2), grouped by company/service line/period — feeds client billing reports (E10). |
| AR-5 | **Payable rollup:** records whose code `is_payable`, for payroll context (E8). |
| AR-6 | Exports (Excel/PDF/CSV) reflect applied filters and are **audited** (who exported what). |
| AR-7 | Read-only; row actions deep-link to verify (F5.3) or correct (F5.4). |
| AR-8 | Times render in Asia/Jakarta; cross-midnight records display spanning two days. |
| AR-9 | **Leader scope is locked to the led company:** for `shift_leader` the `company` filter is server-pinned to their E3 assignment; `site`/`position` only narrow *within* that company. A cross-company `company`/`site` value → `403 OUT_OF_SCOPE` (defense-in-depth; the UI never offers out-of-scope options). |

## 6. Data model

Read-only projection over `Attendance` + `Schedule` + `ShiftMaster` + `AttendanceCode` + `Placement` + `ClientCompany`. No new entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Attendance records & dashboard

  Scenario: Agent views own history
    Given I am the agent "Budi"
    When I open "My attendance"
    Then I see my records with status and any corrections
    And I cannot see other agents' attendance

  Scenario: Leader views team attendance with exceptions
    Given I am the shift leader of "Plaza Senayan"
    When I filter by exception-only for this week
    Then I see only the late/out-of-geofence/incomplete/absent records for my company

  Scenario: Billable rollup for a client
    Given I am HR
    When I run the billable rollup for "Plaza Senayan" for June
    Then I get worked hours for billable attendance codes grouped by service line

  Scenario: Export reflects filters and is audited
    Given I filtered by service line "Parking" and status "Present"
    When I export to Excel
    Then the file contains only those records
    And the export is recorded in the audit log

  Scenario: Scope enforced for leaders
    When a leader opens attendance for a company they don't lead
    Then access is denied
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent with no records | Empty state. |
| C-2 | High-volume company/date range | Server-side pagination; large exports queued/streamed. |
| C-3 | Pending (unverified) records in a billable rollup | Flag/exclude unverified from billing until verified (confirm policy). |
| C-4 | Cross-midnight record | Displays spanning two days; counted once to the start date. |
| C-5 | Corrected record | Shows current values + indicator that a correction was applied. |

## 9. Dependencies

F5.1–F5.4 (data), E2 (codes: billable/payable), E3 (placement/company), E10 (export/reporting), E8 (payable context), E1 (scope/audit).

## 10. Decisions & open questions

- ✅ Scoped read views; billable/payable rollups; audited exports.
- **Open (C-3):** do **unverified** records count toward billable rollups, or only verified ones?
- **Open:** is full billing (rates × billable hours) here or purely in E10? (assumed: E5 provides the hours, E10 does billing.)
