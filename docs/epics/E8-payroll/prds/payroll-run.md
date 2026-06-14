# PRD · F8.3 — Compute-Assist Payroll Run

> **Epic:** E8 Payroll · **Feature:** F8.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

SWP pays placed agents a **monthly wage** each period. Doing it by hand (spreadsheets per company) is error-prone and loses the trace from attendance → pay. This feature makes the system **assemble** each agent's draft pay from authoritative upstream data (E2 base salary, E5 verified attendance, E6 leave, E7 approved OT), let HR **review and adjust** editable lines, then **post** immutable payslips. It is **compute-assist**, not full automation: statutory amounts (BPJS/PPh21) are editable lines, not an auto-engine, and **no money moves** here (that's F8.4). It owns the **agent-pay (cost)** flow only — client billing (revenue) stays hours-only outside the system (INV-5).

## 2. Goals & non-goals

**Goals**
- HR opens a monthly run scoped to a population; the system assembles a draft payslip per eligible agent.
- Assembly traces every auto line to its source (base, proration, OT, leave, adjustments).
- HR edits **Manual** component lines (allowances, BPJS, PPh21, ad-hoc); totals recompute live.
- Posting generates **immutable** payslips (INV-1) and consumes pending prior-period adjustments.
- Late-verified upstream changes after cutoff become next-period adjustments — posted payslips never change.

**Non-goals**
- Moving money / bank integration (F8.4, manual).
- Automatic BPJS/PPh21 statutory calculation engine (editable lines only).
- Client invoicing / applying client rates (outside; E10 hours-only).
- Editing a posted payslip (immutable).

## 3. Actors

HR / Super Admin (run, review, adjust, post), System (assemble, recompute, enforce immutability, create adjustments, audit). Agents are not actors here (they consume via F8.1).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | Open run, review draft payslips, edit Manual lines, post. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| PR-1 | A run targets one **period** (`year`, `month`) and a **scope** (all / client company); the scope filter is snapshotted on the run. |
| PR-2 | The run has a **cutoff_date**; only upstream records **verified/approved on or before cutoff** are assembled (INV-7). |
| PR-3 | **Eligibility:** an agent is included if they have an active `EmploymentAgreement` (E2) overlapping the period within scope. One draft payslip per eligible agent per run. |
| PR-4 | **Base line** = `EmploymentAgreement.base_salary` (monthly, INV-6), prorated only for mid-period join/leave and unpaid days. *(default: calendar-day divisor — see §10)* |
| PR-5 | **Attendance effect:** verified E5 records on **non-payable** `AttendanceCode` reduce pay (proration/absence line); payable codes do not. Unverified records are excluded (PR-2). |
| PR-6 | **Overtime line** = Σ Approved E7 `OvertimeRecord` hours × `OvertimeRule.multiplier` × **hourly base**; hourly base = `base_salary / 173` *(default, configurable)*. Grouped by day-type tier for traceability. |
| PR-7 | **Leave effect:** E6 **paid** leave → no deduction; **unpaid** leave → deduction line. |
| PR-8 | **Statutory & allowance lines** are **Manual** (`source=Manual`): BPJS employee portion, PPh21, allowances — HR-entered/editable, optionally pre-filled from stored config. Not auto-computed. |
| PR-9 | **Prior-period adjustments:** all `PayrollAdjustment(status=Pending)` for an included agent are appended as `Adjustment` lines (signed) and marked `Applied` on post. |
| PR-10 | Totals derived: `gross_earnings` = Σ Earning lines; `gross_deductions` = Σ Deduction lines; `take_home_pay` = earnings − deductions. Recomputed on every draft edit. |
| PR-11 | **Post** sets every payslip `is_posted=true`, `source=Generated`; the run → `Posted`. Posted payslips are **immutable** (INV-1). |
| PR-12 | Draft→Posted is **one-way** *(default: no re-open; see §10)*. A run with zero eligible agents cannot be posted. |
| PR-13 | **Late upstream change** (E5 correction / E7 approval) for a period whose run is **posted** → system creates a `PayrollAdjustment(status=Pending, origin=that period)`; consumed by the next run (PR-9). Never edits the posted payslip. |
| PR-14 | All monetary fields **encrypted at rest** (INV-2); draft + post + edits are **audited** (who/when/what). |
| PR-15 | **Scope/RBAC:** only HR/Super Admin run payroll; client RBAC is defense-in-depth, server enforces (per ENGINEERING). |

## 6. Data model

Writes `PayrollRun`, `Payslip`, `SalaryComponent`, `PayrollAdjustment` (FEATURE §4). Reads E2 `EmploymentAgreement`, E5 `Attendance`/`AttendanceCode`, E6 leave records, E7 `OvertimeRecord`/`OvertimeRule`. No client-billing entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Compute-assist payroll run

  Scenario: Open a run and assemble drafts
    Given I am HR
    And there are placed agents with active employment agreements for 2026-06
    When I open a payroll run for 2026-06 scoped to "Plaza Senayan" with cutoff 2026-06-25
    Then the system creates one draft payslip per eligible agent
    And each draft shows a base line, OT line, leave/absence effects, and any pending adjustments

  Scenario: Only verified/approved upstream counts
    Given an agent has unverified attendance and pending OT for the period
    When the run assembles
    Then those records are excluded from the draft

  Scenario: OT line uses multiplier and hourly base
    Given an agent has 10 approved rest-day OT hours and a rest-day multiplier of 2.0
    And base_salary divided by 173 is the hourly base
    When the run assembles
    Then the OT line equals 10 * 2.0 * hourly_base

  Scenario: HR edits a manual statutory line
    Given a draft payslip
    When I enter the BPJS employee deduction and PPh21
    Then take-home recomputes as earnings minus deductions

  Scenario: Posting makes payslips immutable
    Given a reviewed draft run
    When I post the run
    Then payslips are marked posted and can no longer be edited
    And pending prior-period adjustments are marked applied

  Scenario: Late verification carries forward
    Given the run for 2026-06 is already posted
    When an attendance correction for an agent is verified after the cutoff
    Then the system creates a pending adjustment for that agent
    And it appears as an adjustment line in the next run, not on the posted payslip
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent joins mid-period (PKWT start after the 1st) | Base prorated from start date; included. |
| C-2 | Agent offboarded mid-period (F2.7) | Base prorated to employment end; final-period pay included. |
| C-3 | Agent with no attendance at all in period | Draft created; HR decides (absence deduction vs base) — flagged, not auto-zeroed. |
| C-4 | Two runs accidentally opened for the same period+scope | Block duplicate active run for an overlapping period+scope; warn HR. |
| C-5 | OT approved but its attendance later rejected | If before cutoff, excluded; if after post, negative adjustment next period. |
| C-6 | Negative take-home (deductions > earnings) | Allowed but flagged for HR review before post. |
| C-7 | Decryption/config failure during assembly | Line shown unavailable + flagged; never silently null (consistent E8 G-1). |
| C-8 | Agent has pending adjustments but is out of this run's scope | Adjustments wait; only applied when the agent is in a run. |
| C-9 | Re-assemble a Draft run after upstream changes | Re-pull refreshes Auto lines; preserves HR's Manual edits where unchanged. |

## 9. Dependencies

E1 (RBAC, audit, file/crypto), E2 (employment agreement, base salary, attendance-code config), E5 (verified attendance), E6 (leave paid/unpaid), E7 (approved OT, rule multipliers), F8.1 (agent view of generated payslips), F8.4 (payment).

## 10. Decisions & open questions

- ✅ **Compute-assist, not auto** — statutory lines editable; no BPJS/PPh21 engine in v1.
- ✅ **Monthly base** (INV-6); OT hourly base = `base_salary / 173` *(default, configurable)*.
- ✅ **Verified/approved-only** assembly (INV-7); late changes → carry-forward adjustments (PR-13).
- ✅ **Immutable posted payslips** (INV-1).
- **Open:** proration divisor — **calendar-day vs working-day** *(default: calendar-day)*; confirm with payroll.
- **Open:** allow **re-open** of a posted run **before any payment** vs adjustment-only *(default: no re-open)*.
- **Open:** should statutory config (BPJS %, PTKP/PPh21 table) be stored as a **prefill helper** now, or fully manual in v1? *(default: optional prefill, fully editable)*.
