# PRD · F5.5 — Attendance Records & Dashboard

> **Epic:** E5 Attendance · **Feature:** F5.5 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Everyone needs to *see* attendance: agents review their own history, shift leaders monitor today's roster and exceptions, and HR pulls **billable** and payable rollups for clients and payroll context. Because outsourcing revenue depends on hours actually worked at client sites, the **billable** view (attendance codes flagged billable, E2) is a first-class output feeding reporting (E10).

## 2. Goals & non-goals

**Goals**
- Agent self-history (mobile) with on-screen **date-range** + **status** quick-filters; leader/HR team views with exception highlighting.
- Filters (company, site, position, date, status, exception) + billable/payable rollups.
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
| AR-3 | Filters: **company, site, position**, date range, status (Present/Late/Incomplete/Absent/On-Leave), verification status, exception-only. (`company`/`site`/`position` map 1:1 to the denormalized columns on `Attendance`; `position` is free-text.) |
| AR-4 | **Billable rollup:** sum worked records whose attendance code `is_billable` (E2), grouped by company/position/period — feeds client billing reports (E10). |
| AR-5 | **Payable rollup:** records whose code `is_payable`, for payroll context (E8). |
| AR-6 | Exports (Excel/PDF/CSV) reflect applied filters and are **audited** (who exported what). |
| AR-7 | Read-only; row actions deep-link to verify (F5.3) or correct (F5.4). |
| AR-8 | Times render in Asia/Jakarta; cross-midnight records display spanning two days. |
| AR-9 | **Leader scope is locked to the led company:** for `shift_leader` the `company` filter is server-pinned to their E3 assignment; `site`/`position` only narrow *within* that company. A cross-company `company`/`site` value → `403 OUT_OF_SCOPE` (defense-in-depth; the UI never offers out-of-scope options). |
| AR-10 | **Agent riwayat date-range filter (mobile):** the agent's own history is scoped by a date range, chosen via a range chip → bottom sheet of presets (**Bulan ini**, **30 hari terakhir**, **Bulan lalu**) or **Custom…** → a calendar range picker (tap start, then end, **Terapkan**). Default range = current month. Maps to `date_from`/`date_to` (`GET /attendance:`, inclusive, shift-start-date basis) on the self-scoped list (AR-1); no extra scope — the agent already only sees own records. |
| AR-11 | **Status quick-filter (mobile):** the summary count chips double as filters — **single-select** with **Semua** as the reset (= all). Chips cover the statuses present in range (at minimum **Hadir**/**Terlambat**/**Tidak lengkap**; **Absen**/**Cuti** when present). Tapping a status chip filters the list to that `status` and shows that chip in the active (filled) state; **Semua** (or re-tapping the active chip) clears it. Each chip's **count reflects the current date range** (AR-10). Maps to the `status` query param (a multi-value array on `GET /attendance:`; the mobile single-select sends one value, omitted when Semua). |

> **Design (mobile riwayat):** `brainstorm.pen` (agent lane) — *Agen · Riwayat Kehadiran*: default filter bar (`GJI1a`), status-filtered example (`l6UYy`), date-range presets sheet (`txgoB`), custom calendar range picker (`x2rDk`). Filter chips are status-colored; `Semua` active = `primary-soft`. No dead-flow: chip tap → filtered list / `EmptyFilteredZero` (C-6); range chip → sheet → calendar.

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

  Scenario: Agent filters own history by status
    Given I am the agent "Budi" viewing "Riwayat Kehadiran"
    And the active range is "Bulan ini"
    When I tap the "Terlambat" summary chip
    Then the list shows only my Late records in that range
    And the "Terlambat" chip is shown active
    When I tap "Semua"
    Then the filter resets and all statuses are shown

  Scenario: Agent picks a custom date range
    Given I am viewing "Riwayat Kehadiran"
    When I open the date-range chip and choose "Custom…"
    And I pick 5 Mei as the start and 18 Mei as the end and tap "Terapkan"
    Then the list and the summary-chip counts reflect 5–18 Mei

  Scenario: Leader views team attendance with exceptions
    Given I am the shift leader of "Plaza Senayan"
    When I filter by exception-only for this week
    Then I see only the late/out-of-geofence/incomplete/absent records for my company

  Scenario: Billable rollup for a client
    Given I am HR
    When I run the billable rollup for "Plaza Senayan" for June
    Then I get worked hours for billable attendance codes grouped by position

  Scenario: Export reflects filters and is audited
    Given I filtered by position "Security Guard" and status "Present"
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
| C-6 | Agent filter (range/status) with zero results | `EmptyFilteredZero` state with a reset-to-Semua / widen-range CTA (no dead-flow). |

## 9. Dependencies

F5.1–F5.4 (data), E2 (codes: billable/payable), E3 (placement/company), E10 (export/reporting), E8 (payable context), E1 (scope/audit).

## 10. Decisions & open questions

- ✅ Scoped read views; billable/payable rollups; audited exports.
- **Open (C-3):** do **unverified** records count toward billable rollups, or only verified ones?
- **Open:** is full billing (rates × billable hours) here or purely in E10? (assumed: E5 provides the hours, E10 does billing.)
