# Manual Test Cases — SWP HRIS rebuild

> Detailed manual-testing test cases for the QA team, generated from the FEATURE.md + PRD specs.
> Organized **per epic → per feature (F#.#) → per platform (Web console / Mobile) × per POV (role)**.
> Each file opens with a coverage matrix, then test cases with a run-tracking checkbox `[ ]`.

## How to use

- Each test case has: **checkbox** (tick when run) · **Platform · POV · Priority · Type** · **Objective** · **Preconditions** · **Steps** · **Expected result / Acceptance criteria** · **Traceability** (F#/BR-#/C-#/INV-#/US-#).
- **Priority:** P0 = blocker / must-pass before release · P1 = important · P2 = nice-to-have / cosmetic.
- **Type:** Happy · Negative · Edge · RBAC · Empty/Loading/Error · plus epic-specific (Invariant, Calc, Immutability, CountParity, etc.).
- **POV = role:** super admin · HR/placement admin · shift leader · agent.
- **Platforms:** Web console (super admin, HR admin, shift leader) · Mobile (agent, shift leader). E9 is SQL/CLI validation (no UI).
- ID scheme: `TC-E<epic>-F<feature>-<3-digit seq>`.

## Files

| Epic | File | Cases | Notes |
|------|------|-------|-------|
| E1 — Foundations & Platform | [TEST-CASES-E1-foundations.md](TEST-CASES-E1-foundations.md) | 61 | auth, RBAC denials, audit log, API conventions |
| E2 — Identity, Org & Master Data | [TEST-CASES-E2-identity.md](TEST-CASES-E2-identity.md) | 126 | employees, companies, sites/geofence, offboarding/instant revocation |
| E3 — Placement (differentiator) | [TEST-CASES-E3-placement.md](TEST-CASES-E3-placement.md) | 113 | INV-1..5 invariant-enforcement cases, transfer/history, leader assignment |
| E4 — Shift Config & Scheduling | [TEST-CASES-E4-shift-scheduling.md](TEST-CASES-E4-shift-scheduling.md) | 79 | shift master, roster builder, publish/notify — see ⚠ below |
| E5 — Attendance | [TEST-CASES-E5-attendance.md](TEST-CASES-E5-attendance.md) | 91 | GPS clock in/out, mandatory mobile photo, geofence, verification, corrections |
| E6 — Leave | [TEST-CASES-E6-leave.md](TEST-CASES-E6-leave.md) | 64 | per-type quota ledger, reserve/commit/release, request→approval |
| E7 — Overtime | [TEST-CASES-E7-overtime.md](TEST-CASES-E7-overtime.md) | 73 | PP 35/2021 multiplier Calc cases, holiday-calendar bootstrap |
| E8 — Payroll | [TEST-CASES-E8-payroll.md](TEST-CASES-E8-payroll.md) | 90 | compute-assist run, payslip immutability, payment evidence, period close |
| E9 — Data Migration | [TEST-CASES-E9-migration.md](TEST-CASES-E9-migration.md) | 33 | SQL/CLI data validation: parity, idempotency, identity remap, reconciliation |
| E10 — Reporting, Exports & Notifications | [TEST-CASES-E10-reporting.md](TEST-CASES-E10-reporting.md) | 63 | role dashboards, Excel/PDF export, notification dispatch matrix |
| E11 — Approvals | [TEST-CASES-E11-approvals.md](TEST-CASES-E11-approvals.md) | 61 | template chains, inbox, execution, delegation |
| **Total** | | **854** | |

## ⚠ Spec gaps surfaced during authoring (need a product decision before testing)

- **E4 roster-compliance indicators** (holiday-shift badge, missing-weekly-rest flag, >6-consecutive-workday warning) are **not specified anywhere in E4** — holidays live in E7, leave in E6, and E4 has no coverage/rest rules. Captured as TC-E4-F4.2-050..052 marked as spec-gap; against a faithful v1 build they should be N/A. Decide where these indicators are specified before asserting behavior.
- **E4 F4.4 agent swap / day-off requests** are deferred post-v1 (decided 2026-05-29) — those cases are tagged `[POST-V1]` with a v1 "UI absent/disabled" assertion.
- **E1** appendix flags open items affecting execution: password policy, token lifetimes, hr_admin role-assignment ambiguity (auth token model still open until E1 `/auth` is designed).
- **E6** reflects the *shipped* per-type ledger model (gates retired, doc-enforcement deferred), not the superseded PRD Gherkin — verify against current build.

> Dates resolved to absolute against reference **2026-06-17** (Asia/Jakarta). Regenerate the relevant file from its source PRD when a spec changes; keep TC IDs stable.
