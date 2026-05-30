# PRD · F8.2 — Payroll Archive & Retention (HR)

> **Epic:** E8 Payroll Data · **Feature:** F8.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Beyond agent-facing summaries, HR needs the **full migrated payroll dataset** retained for compliance, audit, and lookups — payslips with their **salary-component line items** and **benefits**. This is a read-only archive: the system of record for "what was paid historically," preserved when the legacy monolith is retired.

## 2. Goals & non-goals

**Goals**
- HR read access to full payslips + components + benefits.
- Search by employee / period; export for audit/compliance.
- Encrypted at rest; access role-gated and audited.

**Non-goals**
- Agent access (F8.1 summaries only). Payroll runs / forward export (out of scope). Editing.

## 3. Actors

HR / Super Admin (read + export), System (scope, decrypt, audit).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | Browse/search full payroll archive; view components & benefits; export. |
| **Mobile app** | — | Not surfaced. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| PA-1 | The archive exposes full `Payslip` + its `SalaryComponent` line items + `Benefit` records. |
| PA-2 | **HR/Super Admin only** (INV-4); not visible to agents or shift leaders. |
| PA-3 | Searchable by employee, period (year/month), and date range. |
| PA-4 | **Read-only**; corrections to migrated errors (if allowed at all) are HR-only, annotated, and audited (see §10). |
| PA-5 | Monetary fields decrypted on read for authorized users; exports include a confidentiality marking. |
| PA-6 | Retention follows policy (see §10); records are not purged unless policy permits. |
| PA-7 | Every view/export is audited (who, what, when). |

## 6. Data model

Reads `Payslip` + `SalaryComponent` + `Benefit` (FEATURE §4). No new entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Payroll archive (HR)

  Scenario: HR views a full payslip with components
    Given I am HR
    When I open Budi's 2025-12 payslip in the archive
    Then I see the take-home plus the salary-component line items and benefits

  Scenario: Agents and leaders cannot access the archive
    When a shift leader or agent tries to open the payroll archive
    Then access is denied

  Scenario: Search by employee and period
    When I search the archive for Budi in 2025
    Then I see all his 2025 payslips

  Scenario: Export for audit
    When I export Budi's 2025 payroll
    Then I get a file with components and benefits
    And the export is recorded in the audit log with a confidentiality marking

  Scenario: Read-only
    When I attempt to edit an archived payslip
    Then it is not permitted (unless an HR correction policy applies, audited)
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Decryption failure | Record flagged; surfaced to migration review; rest of archive usable. |
| C-2 | Benefit with no linked payslip | Shown under the employee's benefits, independent of a specific payslip. |
| C-3 | Component totals vs payslip summary mismatch (legacy data) | Surface both; flag discrepancies for HR (no silent reconciliation). |
| C-4 | Large export | Queued/streamed. |

## 9. Dependencies

E9 (migrated payslips/components/benefits, decryption), E1 (RBAC/audit), E10 (export tooling).

## 10. Decisions & open questions

- ✅ HR-only full archive; read-only; encrypted; audited.
- **Open:** **retention period** + purge policy (compliance).
- **Open:** are HR **corrections/annotations** to migrated payroll errors allowed, or strictly immutable?
- **Open:** is there a **finance-only** sub-role distinct from general HR for payroll access?
