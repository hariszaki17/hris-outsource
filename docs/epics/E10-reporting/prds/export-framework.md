# PRD · F10.4 — Export Framework

> **Epic:** E10 Reporting & Notifications · **Feature:** F10.4 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Many features need to export data — attendance/billable (F10.3) now; OT (E7), leave (E6), placement rosters (E3), payroll archive (E8) as they come. Rather than bespoke exporters, a **single reusable export service** handles formats, large jobs, filters, and audit consistently.

## 2. Goals & non-goals

**Goals**
- One service producing **Excel / PDF / CSV** for any report.
- Honor the caller's filters/scope; queue large jobs; audit every export.

**Non-goals**
- Defining individual reports (those features own their queries). Scheduled/emailed delivery or BI (out of v1).

## 3. Actors

HR / Shift Leader (request, scoped), System (generate, queue, audit).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Shift Leader | Request exports; download when ready. |
| System | — | Generate inline or queued; write audit + file. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| EX-1 | Supports **xlsx, pdf, csv**; the calling report chooses sensible defaults. |
| EX-2 | Exports honor the **same filters and role scope** as the on-screen report (no scope escalation via export). |
| EX-3 | **Large** result sets are **queued**; the requester is notified (F10.1) when the file is ready. |
| EX-4 | Each export creates an `EXPORT_JOB` and an **audit entry** (who, report_type, filters, format, time). |
| EX-5 | Files are stored with **access control + expiry**; sensitive exports (payroll) carry a confidentiality marking. |
| EX-6 | Exports are **point-in-time** snapshots of the query at run time. |

## 6. Data model

`ExportJob` (id, requester_id, report_type, filters json, format, status, file_url, created_at).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Export framework

  Scenario: Export a report inline
    Given a small filtered report
    When I export to Excel
    Then the file is generated immediately and downloadable
    And an audit entry records who/what/when

  Scenario: Large export is queued
    Given a large result set
    When I export
    Then the job is queued and I'm notified when it's ready

  Scenario: Export respects scope
    Given I am a shift leader scoped to one company
    When I export
    Then the file contains only my company's data

  Scenario: Sensitive export marked
    When I export a payroll archive (E8)
    Then the file carries a confidentiality marking and access control

  Scenario: Point-in-time
    Given data changes after I export
    Then my exported file reflects the data at export time
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Export fails mid-generation | Job marked failed; requester notified; no partial file served. |
| C-2 | Very large PDF | Prefer xlsx/csv for huge tabular data; warn on PDF size. |
| C-3 | Concurrent exports by one user | Allowed; each is a separate job. |
| C-4 | File retention | Files expire after a set window (confirm §10). |

## 9. Dependencies

The report features (F10.3, plus E3/E6/E7/E8 reports), F10.1 (ready notification), E1 (scope/audit), file storage.

## 10. Decisions & open questions

- ✅ Reusable xlsx/pdf/csv exporter; scoped; queued for large jobs; audited.
- **Open:** export file **retention/expiry** window.
- **Open:** size threshold that triggers queuing vs inline generation.
