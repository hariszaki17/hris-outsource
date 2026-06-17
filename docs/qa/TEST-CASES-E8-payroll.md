# Manual Test Cases · E8 — Payroll

> **Epic:** E8 Payroll · **Status:** Draft v1 · **Date:** 2026-06-17
> **Sources:** [FEATURE.md](../epics/E8-payroll/FEATURE.md) · PRDs: [payslip-history](../epics/E8-payroll/prds/payslip-history.md) (F8.1) · [payroll-archive](../epics/E8-payroll/prds/payroll-archive.md) (F8.2) · [payroll-run](../epics/E8-payroll/prds/payroll-run.md) (F8.3) · [payroll-payment](../epics/E8-payroll/prds/payroll-payment.md) (F8.4) · [payroll-period-close](../epics/E8-payroll/prds/payroll-period-close.md) (F8.5) · [api/CONVENTIONS.md](../api/CONVENTIONS.md)

## 1. Scope

Exhaustive **manual** test cases for E8 Payroll, covering the two money jobs this epic owns: (1) read-only **migrated history** (agent summaries on mobile; HR full archive on web) and (2) the new **compute-assist monthly run** — period close gate → assemble from E2/E5/E6/E7 → review/adjust → post immutable payslips → record manual payment with evidence.

Cases are organized **per platform (Web / Mobile) × per POV (super admin · HR/placement admin · shift leader · agent)**, then per feature. Each feature's cases derive from its PRD's Actors, Platform/clients table, business rules (BR/PR/PY/PA/PH/PC-#), Gherkin AC, edge cases (C-#), and the epic invariants (INV-1..8).

**Key role facts (from FEATURE §2, §6 and per-PRD Actors):**
- **Super admin** and **HR/placement admin** have the same payroll capabilities in v1 (run, review, adjust, post, pay, archive, period-close lock) — except **force-lock** and **reopen** of a period are **super-admin only** (PC-7, PC-12, PC-15). Tested as one POV pair, with super-admin-only deltas called out.
- **Shift leader**: no payroll surface except **answering a clarification request** scope-checked to their company (PC-13). Otherwise denied (RBAC).
- **Agent**: mobile only — views **own** payslip **summaries** (INV-3), and answers clarifications addressed to them. Cannot see components/benefits (INV-4), cannot see other agents (INV-3), cannot edit (INV-1).
- **Money model:** monthly base (INV-6), two pay models by `employee_type` — FIELD prorates on attendance, INTERNAL is fixed-salary (only unpaid leave / approved OT adjust). No bank/BPJS/tax API (INV-8, INV-5); statutory lines are editable Manual lines.

Error-code conventions per [CONVENTIONS.md](../api/CONVENTIONS.md): `403 FORBIDDEN` (role lacks permission), `404 NOT_FOUND` (no visibility — treated same to avoid leaking), `409 CONFLICT`/`INV_<N>_VIOLATION` (state conflict), `422 RULE_VIOLATION`/`PERIOD_HAS_BLOCKERS` (business-rule), `409 PERIOD_RUN_POSTED`.

---

## 2. Coverage matrix

Legend: ✅ = primary surface tested · 🔒 = denied/RBAC-only (negative coverage) · — = not applicable.

| Feature | Web · Super admin | Web · HR admin | Web · Shift leader | Web · Agent | Mobile · Agent | Mobile · Shift leader |
|---|---|---|---|---|---|---|
| **F8.1** Payslip history & summaries | ✅ view any | ✅ view any | 🔒 | 🔒 | ✅ own only | 🔒 |
| **F8.2** Payroll archive & retention | ✅ full + export | ✅ full + export | 🔒 | 🔒 | — | 🔒 |
| **F8.3** Compute-assist payroll run | ✅ run/post | ✅ run/post | 🔒 | 🔒 | — | — |
| **F8.4** Payment recording & evidence | ✅ record/void | ✅ record | 🔒 | 🔒 | view status (via F8.1) | 🔒 |
| **F8.5** Payroll period close | ✅ lock + force-lock + reopen | ✅ lock (no force/reopen) | 🔒 (answer clarification only) | 🔒 | answer clarification | answer clarification |

---

## 3. Test cases

### F8.1 — Payslip History & Summaries (read-only)

#### Mobile · Agent POV

#### TC-E8-F8.1-001 · Agent views own unified payslip list (migrated + generated)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent sees a single chronological list combining migrated and generated payslips.
- **Preconditions:** Logged-in agent "Budi" (`SWP-EMP-1042`) has ≥2 migrated payslips (`source=Migrated`) and ≥1 generated (`source=Generated`).
- **Steps:**
  1. Open the mobile app, authenticate as Budi.
  2. Navigate to "My payslips" (Slip Gaji Saya).
- **Expected result / Acceptance criteria:** A unified list ordered by period (newest first). Each row shows period (year/month), `paid_on`, and `take_home_pay`; migrated and generated rows are visually indistinguishable in ordering. IDR formatted consistently.
- **Traceability:** F8.1, PH-1, INV-3.

#### TC-E8-F8.1-002 · Agent opens a payslip summary (totals only, no breakdown)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Opening a payslip shows the summary fields only.
- **Preconditions:** Budi has a payslip for 2025-12.
- **Steps:**
  1. From "My payslips", tap the 2025-12 row.
- **Expected result / Acceptance criteria:** Detail shows period, `paid_on`, `working_days`, `gross_earnings`, `gross_deductions`, `take_home_pay`, and `payment_status` (Unpaid/Paid). **No** `SalaryComponent` line items and **no** benefits are shown anywhere.
- **Traceability:** F8.1, PH-1, PH-5, INV-3, INV-4.

#### TC-E8-F8.1-003 · Agent cannot access another agent's payslip
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Scope is self-only; cross-agent access is denied (defense-in-depth + server gate).
- **Preconditions:** Budi authenticated; payslip id of another agent "Siti" (`SWP-EMP-1099`) known.
- **Steps:**
  1. Attempt to fetch Siti's payslip directly (manipulate the request / deep link to her payslip id).
- **Expected result / Acceptance criteria:** Server returns `404 NOT_FOUND` (visibility hidden to avoid leaking, per CONVENTIONS) — never Siti's data. UI surfaces a not-found / no-access state, not a crash.
- **Traceability:** F8.1, PH-2, INV-3, CONVENTIONS §404.

#### TC-E8-F8.1-004 · New-hire agent with no payslips — empty state
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** Empty state renders, not an error.
- **Preconditions:** Newly onboarded agent with zero payslips (migrated or generated).
- **Steps:**
  1. Open "My payslips".
- **Expected result / Acceptance criteria:** Friendly empty state (e.g. "Belum ada slip gaji") with no error/spinner-forever. Pull-to-refresh works.
- **Traceability:** F8.1, C-1.

#### TC-E8-F8.1-005 · Loading and network-error states
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** Loading skeleton then graceful error on failure.
- **Preconditions:** Network throttling / server-error injection available.
- **Steps:**
  1. Open "My payslips" with slow network → observe loading.
  2. Force a 5xx → observe error.
  3. Retry after restoring network.
- **Expected result / Acceptance criteria:** Loading skeleton/spinner during fetch; on error, an inline error with retry (no white screen); retry recovers the list.
- **Traceability:** F8.1, E8 G-1 (never silently null).

#### TC-E8-F8.1-006 · Decryption failure on one record — flagged, list usable
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** A single undecryptable migrated record does not break the list.
- **Preconditions:** One migrated payslip with corrupt/undecryptable monetary fields.
- **Steps:**
  1. Open "My payslips" and the affected payslip.
- **Expected result / Acceptance criteria:** Amounts show "unavailable" / flagged; the rest of the list remains usable; no crash; record flagged for migration review server-side.
- **Traceability:** F8.1, C-2, INV-2.

#### TC-E8-F8.1-007 · Payslip predating current employment agreement still shown
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** History is independent of the current EmploymentAgreement.
- **Preconditions:** Budi's current PKWT started 2025-06; a migrated payslip exists for 2024-11 (prior agreement).
- **Steps:**
  1. Open "My payslips".
- **Expected result / Acceptance criteria:** The 2024-11 payslip appears and opens normally.
- **Traceability:** F8.1, C-3.

#### TC-E8-F8.1-008 · Agent attempt to edit a payslip is not permitted
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Immutability
- **Objective:** No edit affordance; any edit attempt rejected.
- **Preconditions:** Budi has any payslip.
- **Steps:**
  1. Open a payslip; confirm no edit control exists.
  2. Attempt a write (e.g. crafted PATCH to the payslip).
- **Expected result / Acceptance criteria:** No edit UI; write rejected (`403`/`405`); payslip unchanged.
- **Traceability:** F8.1, PH-4, INV-1.

#### Mobile · Shift leader POV

#### TC-E8-F8.1-009 · Shift leader has no payslip-history surface
- [ ] **Platform:** Mobile · **POV:** Shift leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** Payslip history is agent/HR only; SL sees nothing here.
- **Preconditions:** Logged-in shift leader.
- **Steps:**
  1. Inspect mobile nav for "My payslips"; attempt to fetch another agent's payslip.
- **Expected result / Acceptance criteria:** SL has access only to their own payslip (as an employee), never any other agent's; cross-agent fetch → `404`/`403`. No team-payslip view exists.
- **Traceability:** F8.1, PH-2, INV-3.

#### Web · Super admin & HR admin POV

#### TC-E8-F8.1-010 · HR views any agent's payslip summary on web
- [ ] **Platform:** Web · **POV:** HR admin / Super admin · **Priority:** P0 · **Type:** Happy
- **Objective:** HR can open any agent's summary figures.
- **Preconditions:** HR logged in; Budi has a 2025-12 payslip.
- **Steps:**
  1. Open the payslip-history view (or via the employee), select Budi 2025-12.
- **Expected result / Acceptance criteria:** Summary figures (period, paid_on, working_days, gross_earnings, gross_deductions, take_home_pay, payment_status) shown; monetary fields decrypted on read; view audited (who viewed whose).
- **Traceability:** F8.1, PH-2, PH-3, PH-6, INV-2.

#### TC-E8-F8.1-011 · Viewing a payslip is audited
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Happy
- **Objective:** Audit log captures payslip views.
- **Preconditions:** Audit log accessible to super admin.
- **Steps:**
  1. As HR, open Budi's 2025-12 payslip.
  2. As super admin, inspect the audit log.
- **Expected result / Acceptance criteria:** An audit entry records the viewer, the subject (Budi), the payslip, and timestamp.
- **Traceability:** F8.1, PH-6.

---

### F8.2 — Payroll Archive & Retention (HR)

#### Web · Super admin & HR admin POV

#### TC-E8-F8.2-001 · HR views a full payslip with components and benefits
- [ ] **Platform:** Web · **POV:** HR admin / Super admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Archive exposes full payslip + SalaryComponent line items + Benefit records.
- **Preconditions:** Budi has a 2025-12 payslip with components and at least one Benefit.
- **Steps:**
  1. Open Payroll Archive; search Budi 2025-12; open the payslip.
- **Expected result / Acceptance criteria:** Take-home plus all `SalaryComponent` lines (name, kind Earning/Deduction, category, value, source Auto/Manual, basis) and the employee's benefits are shown; amounts decrypted on read.
- **Traceability:** F8.2, PA-1, PA-5, INV-2, INV-4.

#### TC-E8-F8.2-002 · Search archive by employee and period
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Search by employee + year returns all matching payslips.
- **Preconditions:** Budi has ≥3 payslips across 2025.
- **Steps:**
  1. Open Archive; search employee "Budi", year 2025.
- **Expected result / Acceptance criteria:** All Budi 2025 payslips listed; cursor pagination if many; date-range filter also available (PA-3).
- **Traceability:** F8.2, PA-3.

#### TC-E8-F8.2-003 · Export Budi's 2025 payroll for audit
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Export produces a file with components + benefits and a confidentiality marking; export is audited.
- **Preconditions:** Budi has 2025 payslips.
- **Steps:**
  1. Search Budi 2025; trigger Export.
- **Expected result / Acceptance criteria:** Export file includes components and benefits; carries a confidentiality marking (PA-5); an audit entry records who exported what/when (PA-7).
- **Traceability:** F8.2, PA-5, PA-7.

#### TC-E8-F8.2-004 · Large export is queued/streamed
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** A large archive export does not time out the request.
- **Preconditions:** A company/period selection yielding a very large export.
- **Steps:**
  1. Trigger a broad export (e.g. all payslips for a large company across multiple years).
- **Expected result / Acceptance criteria:** Export is queued or streamed (async/job or chunked download); UI shows progress/ready state; no synchronous timeout.
- **Traceability:** F8.2, C-4.

#### TC-E8-F8.2-005 · Archived payslip is read-only (edit rejected)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Immutability
- **Objective:** Editing an archived (migrated/posted) payslip is not permitted.
- **Preconditions:** A migrated payslip in the archive.
- **Steps:**
  1. Open the payslip; attempt to change a component value (no edit control expected); attempt a crafted write.
- **Expected result / Acceptance criteria:** No edit affordance; write rejected; only an audited HR **annotation/note** is allowed if a correction policy applies (no value mutation).
- **Traceability:** F8.2, PA-4, INV-1.

#### TC-E8-F8.2-006 · Component totals vs payslip summary mismatch (legacy) — surfaced, not reconciled
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Legacy mismatch is flagged, both figures shown, no silent reconciliation.
- **Preconditions:** A migrated payslip where Σ component values ≠ stored summary totals.
- **Steps:**
  1. Open the payslip in the archive.
- **Expected result / Acceptance criteria:** Both the summary totals and the component-sum are shown; a discrepancy flag is visible; the system does not auto-adjust either.
- **Traceability:** F8.2, C-3.

#### TC-E8-F8.2-007 · Benefit with no linked payslip shown under employee
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Benefits independent of a specific payslip are still visible.
- **Preconditions:** A Benefit row for Budi not tied to any payslip.
- **Steps:**
  1. Open Budi's archive / benefits.
- **Expected result / Acceptance criteria:** The benefit appears under the employee's benefits, independent of any payslip.
- **Traceability:** F8.2, C-2.

#### TC-E8-F8.2-008 · Decryption failure on a record — flagged, rest usable
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** One undecryptable record does not break the archive.
- **Preconditions:** A migrated payslip with undecryptable fields.
- **Steps:**
  1. Open the archive containing the record; open the record.
- **Expected result / Acceptance criteria:** Record flagged + surfaced to migration review; the rest of the archive remains usable; no crash.
- **Traceability:** F8.2, C-1, INV-2.

#### TC-E8-F8.2-009 · Empty archive search — empty state
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** No-results search shows an empty state.
- **Preconditions:** Search a term with no matches.
- **Steps:**
  1. Search a non-existent employee.
- **Expected result / Acceptance criteria:** Empty-results state, not an error.
- **Traceability:** F8.2.

#### Web · Shift leader & Agent POV (RBAC)

#### TC-E8-F8.2-010 · Shift leader cannot open the payroll archive
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Archive is HR/Super-admin only.
- **Preconditions:** Shift leader with web access (if any) authenticated.
- **Steps:**
  1. Attempt to navigate to the payroll archive; attempt the archive API directly.
- **Expected result / Acceptance criteria:** UI hides the archive entry; direct API call → `403 FORBIDDEN`; `comp/EmptyNoPermission` state surfaced.
- **Traceability:** F8.2, PA-2, INV-4, CONVENTIONS §403.

#### TC-E8-F8.2-011 · Agent cannot access the archive (components/benefits)
- [ ] **Platform:** Web/Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Agents never see the component breakdown / benefits.
- **Preconditions:** Agent authenticated.
- **Steps:**
  1. Attempt to reach the archive endpoint for own or any payslip's full breakdown.
- **Expected result / Acceptance criteria:** `403 FORBIDDEN` (or no surface at all); agents are limited to summary-level F8.1.
- **Traceability:** F8.2, PA-2, INV-3, INV-4.

---

### F8.3 — Compute-Assist Payroll Run (assemble → review → post)

#### Web · Super admin & HR admin POV

#### TC-E8-F8.3-001 · Open a run and assemble draft payslips
- [ ] **Platform:** Web · **POV:** HR admin / Super admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Opening a run creates one draft payslip per eligible agent assembled from upstream.
- **Preconditions:** Period 2026-06 **LOCKED** (PC-10); placed agents with active EmploymentAgreements overlapping 2026-06 within scope "Plaza Senayan".
- **Steps:**
  1. Open a payroll run: year 2026, month 06, scope = Plaza Senayan, cutoff 2026-06-25.
  2. View the assembled draft list.
- **Expected result / Acceptance criteria:** One draft payslip per eligible agent; each draft shows a base line, OT line, leave/absence effects, statutory/allowance Manual lines, and any pending adjustments; run status = `Draft`.
- **Traceability:** F8.3, PR-1, PR-2, PR-3, INV-7, PC-10.

#### TC-E8-F8.3-002 · Run requires a LOCKED period (precondition)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** A run cannot be opened/assembled for an OPEN period.
- **Preconditions:** Period 2026-06 is `OPEN` (not locked).
- **Steps:**
  1. Attempt to open a 2026-06 payroll run.
- **Expected result / Acceptance criteria:** Blocked with a clear message ("period not locked"); the run reads `PeriodEmployeeSummary`, which does not exist until lock. No draft created.
- **Traceability:** F8.3, PC-10, F8.5 PC-10.

#### TC-E8-F8.3-003 · Only verified/approved upstream counts; pending excluded
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Calc
- **Objective:** Unverified attendance / pending OT are excluded from the draft.
- **Preconditions:** Agent "Andi" has 1 unverified attendance day and 4h PENDING OT for 2026-06; period locked.
- **Steps:**
  1. Assemble the 2026-06 run including Andi.
  2. Inspect Andi's draft lines.
- **Expected result / Acceptance criteria:** The unverified day and pending OT are **not** present in the draft; only VERIFIED/AUTO_APPROVED attendance and Approved OT contribute.
- **Traceability:** F8.3, PR-2, PR-5, INV-7, AC "Only verified/approved upstream counts".

#### TC-E8-F8.3-004 · OT line = hours × multiplier × hourly base (Permenaker /173)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Calc
- **Objective:** OT line is correctly computed.
- **Preconditions:** Agent with `base_salary` = IDR 4,000,000; 10 Approved rest-day OT hours; rest-day multiplier 2.0; hourly base = 4,000,000 / 173 = 23,121.39.
- **Steps:**
  1. Assemble the run; open the agent's OT line.
- **Expected result / Acceptance criteria:** OT line = 10 × 2.0 × 23,121.39 = **IDR 462,427.75** (≈ rounding per config), grouped by day-type tier with traceable basis (hours + multiplier). Auto source.
- **Traceability:** F8.3, PR-6, INV-6, AC "OT line uses multiplier and hourly base".

#### TC-E8-F8.3-005 · FIELD agent base prorated by non-payable attendance
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Calc
- **Objective:** FIELD pay model — verified non-payable days reduce pay.
- **Preconditions:** FIELD agent, monthly base IDR 4,650,000; June has 30 calendar days; 2 verified days on a **non-payable** AttendanceCode (e.g. unpaid absence); calendar-day divisor (default §10).
- **Steps:**
  1. Assemble the run; inspect base + proration/absence line.
- **Expected result / Acceptance criteria:** Absence deduction = 2 × (4,650,000 / 30) = 2 × 155,000 = **IDR 310,000**; payable days do not reduce pay. Net base effect = 4,650,000 − 310,000 = 4,340,000 before OT/leave/statutory.
- **Traceability:** F8.3, PR-4, PR-5, INV-6.

#### TC-E8-F8.3-006 · INTERNAL staff salary is fixed — attendance does not prorate
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Calc
- **Objective:** INTERNAL pay model — `is_payable` ignored; only unpaid leave deducts and approved OT adds.
- **Preconditions:** INTERNAL staff, monthly base IDR 7,000,000; 1 verified absent day (no leave); 0 OT; 0 unpaid leave.
- **Steps:**
  1. Assemble the run; inspect the draft.
- **Expected result / Acceptance criteria:** Base stays **IDR 7,000,000** (the absent day does NOT prorate it). Only unpaid leave would deduct and approved OT would add.
- **Traceability:** F8.3, INV-6 (two pay models), PR-4, PR-7.

#### TC-E8-F8.3-007 · INTERNAL staff unpaid leave deducts; approved OT adds
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Calc
- **Objective:** Validate the only two INTERNAL adjustments.
- **Preconditions:** INTERNAL staff, base IDR 7,000,000; 1 unpaid-leave day (June, 30 days); 5h Approved OT, multiplier 1.5; hourly base = 7,000,000/173 = 40,462.43.
- **Steps:**
  1. Assemble; inspect lines.
- **Expected result / Acceptance criteria:** Unpaid-leave deduction = 7,000,000/30 = **IDR 233,333.33**; OT add = 5 × 1.5 × 40,462.43 = **IDR 303,468.21**. Take-home = 7,000,000 − 233,333.33 + 303,468.21 = **IDR 7,070,134.88** (before statutory lines).
- **Traceability:** F8.3, INV-6, PR-6, PR-7.

#### TC-E8-F8.3-008 · Paid leave does not deduct; unpaid leave deducts (FIELD)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Calc
- **Objective:** Leave effect per PR-7.
- **Preconditions:** FIELD agent with 1 E6 **paid** leave day and 1 E6 **unpaid** leave day; base 4,500,000 / 30.
- **Steps:**
  1. Assemble; inspect leave effect.
- **Expected result / Acceptance criteria:** Paid leave → no deduction line; unpaid leave → deduction of 4,500,000/30 = **IDR 150,000**.
- **Traceability:** F8.3, PR-7.

#### TC-E8-F8.3-009 · HR edits a Manual statutory line; totals recompute live
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Calc
- **Objective:** Entering BPJS/PPh21 Manual lines recomputes take-home immediately.
- **Preconditions:** A draft payslip with gross_earnings = IDR 5,000,000, no deductions yet.
- **Steps:**
  1. Enter BPJS employee deduction IDR 100,000 (Manual, Deduction, category BPJS).
  2. Enter PPh21 IDR 250,000 (Manual, Deduction, category PPh21).
- **Expected result / Acceptance criteria:** gross_deductions = 350,000; take_home_pay recomputes live to 5,000,000 − 350,000 = **IDR 4,650,000**. Both lines marked `source=Manual`.
- **Traceability:** F8.3, PR-8, PR-10, AC "HR edits a manual statutory line".

#### TC-E8-F8.3-010 · Statutory lines are editable, not auto-computed (no engine)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Confirm no BPJS/PPh21 auto-engine — only optional prefill, fully editable.
- **Preconditions:** Stored statutory config (BPJS %, PTKP) present (optional prefill).
- **Steps:**
  1. Assemble a draft; inspect statutory lines.
  2. Overwrite a prefilled BPJS value with a different amount.
- **Expected result / Acceptance criteria:** Statutory amounts are Manual lines; any prefill is editable and fully overridable; the system does not lock them to a computed value.
- **Traceability:** F8.3, PR-8, §10 (optional prefill), INV-5 (out-of-scope confirmation: no statutory engine).

#### TC-E8-F8.3-011 · Prior-period adjustments appended and marked Applied on post
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Calc
- **Objective:** Pending PayrollAdjustments are added as signed Adjustment lines and consumed on post.
- **Preconditions:** Agent has a PENDING PayrollAdjustment (origin 2026-05, +IDR 200,000) within scope.
- **Steps:**
  1. Assemble the 2026-06 run including the agent; inspect Adjustment line.
  2. Post the run; re-check the adjustment status.
- **Expected result / Acceptance criteria:** An Adjustment line of +200,000 appears (signed, category Adjustment); on post the adjustment `status` → `Applied` with `applied_run_id` set; take-home includes it.
- **Traceability:** F8.3, PR-9, AC "Posting makes payslips immutable".

#### TC-E8-F8.3-012 · Totals derivation (gross/net) is correct
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Calc
- **Objective:** End-to-end total = Σ earnings − Σ deductions.
- **Preconditions:** Draft with base 4,650,000 (after −310,000 absence), OT +462,427.75, paid leave (no effect), Adjustment +200,000; deductions BPJS 100,000 + PPh21 250,000.
- **Steps:**
  1. Inspect computed totals.
- **Expected result / Acceptance criteria:** gross_earnings = 4,650,000 + 462,427.75 + 200,000 = 5,312,427.75; gross_deductions = 350,000; take_home_pay = **IDR 4,962,427.75**.
- **Traceability:** F8.3, PR-10.

#### TC-E8-F8.3-013 · Posting makes payslips immutable
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Immutability
- **Objective:** After post, payslips/lines cannot be edited.
- **Preconditions:** A reviewed Draft run.
- **Steps:**
  1. Post the run.
  2. Attempt to edit a component value on a posted payslip (UI + crafted write).
- **Expected result / Acceptance criteria:** Run status → `Posted`; payslips `is_posted=true`, `source=Generated`; edit affordances gone; write rejected (`409`/`422` invariant). Posted payslip totals/lines unchanged.
- **Traceability:** F8.3, PR-11, INV-1, AC "Posting makes payslips immutable".

#### TC-E8-F8.3-014 · Draft→Posted is one-way (no re-open default)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Immutability
- **Objective:** A posted run cannot be reverted to Draft (v1 default).
- **Preconditions:** A `Posted` run.
- **Steps:**
  1. Attempt to re-open / set the run back to Draft.
- **Expected result / Acceptance criteria:** Rejected; no re-open control; corrections must flow via PayrollAdjustment.
- **Traceability:** F8.3, PR-12, §10 (no re-open default).

#### TC-E8-F8.3-015 · Cannot post a run with zero eligible agents
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Empty run cannot be posted.
- **Preconditions:** A scope with no agents holding an active agreement overlapping the period.
- **Steps:**
  1. Open the run; attempt to post.
- **Expected result / Acceptance criteria:** Post blocked with a clear message; run stays Draft (or cannot be created).
- **Traceability:** F8.3, PR-12.

#### TC-E8-F8.3-016 · Block duplicate active run for overlapping period+scope
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Prevent two concurrent runs for the same period+scope.
- **Preconditions:** A Draft run for 2026-06 / Plaza Senayan exists.
- **Steps:**
  1. Attempt to open a second 2026-06 run for an overlapping scope (e.g. "all" or Plaza Senayan).
- **Expected result / Acceptance criteria:** Blocked (`409 CONFLICT`) with a warning identifying the existing run; no duplicate created.
- **Traceability:** F8.3, C-4.

#### TC-E8-F8.3-017 · Late verification after post → carry-forward adjustment (never edits posted payslip)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Immutability
- **Objective:** Post-cutoff upstream change becomes a PENDING adjustment for the next run.
- **Preconditions:** 2026-06 run `Posted`; an E5 attendance correction for an agent verified on 2026-07-02 (after cutoff/post).
- **Steps:**
  1. Verify the late correction.
  2. Inspect adjustments and the posted 2026-06 payslip.
  3. Open the next run (2026-07) for the agent.
- **Expected result / Acceptance criteria:** A `PayrollAdjustment(status=Pending, origin=2026-06)` is created; the 2026-06 posted payslip is **unchanged**; the adjustment appears as a line in the 2026-07 run.
- **Traceability:** F8.3, PR-13, INV-1, AC "Late verification carries forward", PC-9.

#### TC-E8-F8.3-018 · Agent joins mid-period — base prorated from start
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Calc
- **Objective:** Mid-period join is included with prorated base.
- **Preconditions:** PKWT start 2026-06-16; June has 30 days; base IDR 4,500,000; calendar-day divisor.
- **Steps:**
  1. Assemble; inspect base line.
- **Expected result / Acceptance criteria:** Base prorated for 15 payable days = 4,500,000 × (15/30) = **IDR 2,250,000**; agent included.
- **Traceability:** F8.3, C-1, PR-4.

#### TC-E8-F8.3-019 · Agent offboarded mid-period — base prorated to end
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Calc
- **Objective:** Final-period pay prorated to employment end (F2.7).
- **Preconditions:** Employment end 2026-06-20; base IDR 6,000,000; 30-day June; calendar-day divisor.
- **Steps:**
  1. Assemble; inspect base line.
- **Expected result / Acceptance criteria:** Base = 6,000,000 × (20/30) = **IDR 4,000,000**; final-period pay included.
- **Traceability:** F8.3, C-2.

#### TC-E8-F8.3-020 · Agent with no attendance — flagged, not auto-zeroed
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Zero-attendance draft is surfaced for HR decision.
- **Preconditions:** FIELD agent with active agreement but zero attendance rows in 2026-06.
- **Steps:**
  1. Assemble; inspect the agent's draft.
- **Expected result / Acceptance criteria:** A draft is created and **flagged**; pay is not silently zeroed; HR decides (absence deduction vs base).
- **Traceability:** F8.3, C-3.

#### TC-E8-F8.3-021 · OT approved but its attendance later rejected
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Rejected underlying attendance handling depends on cutoff timing.
- **Preconditions:** Approved OT whose attendance is rejected (a) before cutoff and (b) after post.
- **Steps:**
  1. Reject before cutoff → assemble.
  2. In a separate posted scenario, reject after post.
- **Expected result / Acceptance criteria:** (a) OT excluded from draft; (b) a **negative** PayrollAdjustment created for the next period; posted payslip unchanged.
- **Traceability:** F8.3, C-5, PR-13.

#### TC-E8-F8.3-022 · Negative take-home allowed but flagged before post
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Deductions > earnings is permitted but flagged.
- **Preconditions:** A draft where deductions exceed earnings (e.g. large adjustment + statutory).
- **Steps:**
  1. Inspect totals; attempt to post.
- **Expected result / Acceptance criteria:** Negative `take_home_pay` allowed but flagged for HR review before posting; HR must acknowledge.
- **Traceability:** F8.3, C-6.

#### TC-E8-F8.3-023 · Decryption/config failure during assembly — line unavailable + flagged
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** A failed line is never silently null.
- **Preconditions:** Inject a decryption or config (e.g. multiplier/divisor) failure during assembly.
- **Steps:**
  1. Assemble; inspect the affected line.
- **Expected result / Acceptance criteria:** The line shows "unavailable" + flagged; assembly does not produce a silent zero/null; HR can see what failed (E8 G-1).
- **Traceability:** F8.3, C-7, INV-2.

#### TC-E8-F8.3-024 · Adjustment waits if agent out of run scope
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Pending adjustments only apply when the agent is in a run.
- **Preconditions:** Agent has a PENDING adjustment but is outside the current run's scope.
- **Steps:**
  1. Assemble + post a run that excludes the agent.
  2. Later assemble a run that includes them.
- **Expected result / Acceptance criteria:** The adjustment stays PENDING in the first run; it appears and is applied only when a run includes the agent.
- **Traceability:** F8.3, C-8, PR-9.

#### TC-E8-F8.3-025 · Re-assemble a Draft run preserves HR Manual edits
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Re-pull refreshes Auto lines but keeps unchanged Manual edits.
- **Preconditions:** Draft run where HR entered BPJS/PPh21 (Manual); upstream attendance changes since.
- **Steps:**
  1. Trigger re-assemble.
- **Expected result / Acceptance criteria:** Auto lines (base/proration/OT/leave/adjustments) refresh; HR's Manual lines are preserved where their basis is unchanged; no loss of HR entries.
- **Traceability:** F8.3, C-9.

#### TC-E8-F8.3-026 · Run/post/edit actions are audited
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Happy
- **Objective:** Draft creation, Manual edits, and post are audited (who/when/what).
- **Preconditions:** Audit log accessible.
- **Steps:**
  1. Open a run, edit a Manual line, post; inspect audit.
- **Expected result / Acceptance criteria:** Audit entries for assemble/edit/post with actor, timestamp, and changed values; encrypted fields handled per INV-2.
- **Traceability:** F8.3, PR-14.

#### TC-E8-F8.3-027 · Confirm out-of-scope: no client invoicing in the run
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Negative
- **Objective:** The run computes agent pay only — no billable-hours / client-rate computation surfaces.
- **Preconditions:** Any run.
- **Steps:**
  1. Inspect all draft lines and totals.
- **Expected result / Acceptance criteria:** No client invoice, billable-hours rate, or revenue figure anywhere; pay is monthly-wage based (INV-5, INV-6); billing stays hours-only outside (E10).
- **Traceability:** F8.3, INV-5, INV-6.

#### Web · Shift leader & Agent POV (RBAC)

#### TC-E8-F8.3-028 · Shift leader cannot run/post payroll
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Payroll run is HR/Super-admin only; server enforces.
- **Preconditions:** Shift leader authenticated.
- **Steps:**
  1. Attempt to access the payroll-run screen; attempt the open-run / post API directly.
- **Expected result / Acceptance criteria:** No nav entry; direct API → `403 FORBIDDEN`; `comp/EmptyNoPermission`.
- **Traceability:** F8.3, PR-15, CONVENTIONS §403.

#### TC-E8-F8.3-029 · Agent cannot run/post or view drafts
- [ ] **Platform:** Web/Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Agents are not actors in F8.3.
- **Preconditions:** Agent authenticated.
- **Steps:**
  1. Attempt to reach run/draft endpoints.
- **Expected result / Acceptance criteria:** `403 FORBIDDEN`; agents only consume posted payslips via F8.1.
- **Traceability:** F8.3, §3 Actors, PR-15.

---

### F8.4 — Payment Recording & Transfer Evidence (manual)

#### Web · Super admin & HR admin POV

#### TC-E8-F8.4-001 · Record a single bank-transfer payment with evidence
- [ ] **Platform:** Web · **POV:** HR admin / Super admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Recording with date + reference + evidence marks the payslip Paid.
- **Preconditions:** Posted, Unpaid payslip for Budi, take_home_pay = IDR 4,650,000.
- **Steps:**
  1. Open the posted run's payment list; select Budi's payslip.
  2. Record: method=BankTransfer, paid_on=2026-07-01, reference_no="TRX-9981", upload receipt image.
- **Expected result / Acceptance criteria:** Payslip `payment_status=Paid`, `paid_on=2026-07-01`; a `PayrollPayment` created with the evidence file linked; `amount` defaults to 4,650,000; action audited.
- **Traceability:** F8.4, PY-1, PY-2, PY-3, PY-7, INV-8, AC "Record a single payment with evidence".

#### TC-E8-F8.4-002 · Evidence is required — payment without evidence rejected
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** No evidence → no Paid status.
- **Preconditions:** Posted, Unpaid payslip.
- **Steps:**
  1. Attempt to record a payment with date + reference but no evidence file.
- **Expected result / Acceptance criteria:** Rejected (`422`, field error on evidence); no `PayrollPayment` created; payslip stays `Unpaid`.
- **Traceability:** F8.4, PY-2, INV-8, AC "Evidence is required".

#### TC-E8-F8.4-003 · reference_no required for BankTransfer
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative
- **Objective:** BankTransfer requires a reference number.
- **Preconditions:** Posted, Unpaid payslip.
- **Steps:**
  1. Record method=BankTransfer with evidence + date but blank reference_no.
- **Expected result / Acceptance criteria:** Rejected with a field-level error on reference_no; payslip stays Unpaid. (Cash method does not require reference_no.)
- **Traceability:** F8.4, PY-2.

#### TC-E8-F8.4-004 · Cash payment with evidence (no reference required)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Happy
- **Objective:** Cash method recorded with evidence, no reference_no.
- **Preconditions:** Posted, Unpaid payslip.
- **Steps:**
  1. Record method=Cash, paid_on, evidence (e.g. signed receipt), no reference_no.
- **Expected result / Acceptance criteria:** Payslip Paid; `PayrollPayment.method=Cash`; reference_no optional/empty accepted.
- **Traceability:** F8.4, PY-2, PY-3.

#### TC-E8-F8.4-005 · Batch payment — one receipt for 20 payslips
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Batch creates one PayrollPayment per payslip linking shared evidence.
- **Preconditions:** 20 posted, Unpaid payslips in the run.
- **Steps:**
  1. Select all 20; record one bulk transfer: shared paid_on, reference_no, single receipt upload.
- **Expected result / Acceptance criteria:** 20 `PayrollPayment` rows created, all linking the same evidence and reference; all 20 payslips → `Paid`; batch action audited (lists affected payslips).
- **Traceability:** F8.4, PY-4, PY-7, AC "Batch payment for a run".

#### TC-E8-F8.4-006 · Batch containing an already-paid payslip — skipped with notice
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Already-paid items are skipped, not double-paid.
- **Preconditions:** A batch selection of 10 where 2 are already Paid.
- **Steps:**
  1. Record the batch.
- **Expected result / Acceptance criteria:** Only the 8 Unpaid are processed; the 2 Paid are skipped with a clear notice; no duplicate payments.
- **Traceability:** F8.4, C-4, PY-5.

#### TC-E8-F8.4-007 · Cannot double-pay a payslip
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** A Paid payslip cannot be paid again.
- **Preconditions:** A Paid payslip.
- **Steps:**
  1. Attempt to record another payment against it.
- **Expected result / Acceptance criteria:** Rejected (`409`/`422`); no second `PayrollPayment` created; status unchanged.
- **Traceability:** F8.4, PY-5, AC "Cannot double-pay".

#### TC-E8-F8.4-008 · Cannot record payment on an unposted (Draft) payslip
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Payment only against Posted runs.
- **Preconditions:** A Draft run (not posted).
- **Steps:**
  1. Attempt to reach a payment-record action for a draft payslip.
- **Expected result / Acceptance criteria:** Blocked; no payment surface for draft; API rejects (`409`).
- **Traceability:** F8.4, PY-1.

#### TC-E8-F8.4-009 · Void a payment — reverses to Unpaid, history preserved
- [ ] **Platform:** Web · **POV:** HR admin / Super admin · **Priority:** P0 · **Type:** Immutability
- **Objective:** Void creates an audited reversal; does not delete history.
- **Preconditions:** A Paid payslip with a recorded payment.
- **Steps:**
  1. Void the payment with a reason.
  2. Inspect history + audit.
- **Expected result / Acceptance criteria:** Payslip returns to `Unpaid`; a **reversal record** is created (original `PayrollPayment` not deleted); reason + actor + timestamp audited; can re-record afterward.
- **Traceability:** F8.4, PY-5, PY-7, AC "Void a payment".

#### TC-E8-F8.4-010 · Partial / different amount allowed with reason + flag
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Calc
- **Objective:** Amount defaults to take-home and must match unless HR overrides with a reason.
- **Preconditions:** Posted, Unpaid payslip, take_home_pay = IDR 4,650,000.
- **Steps:**
  1. Record a payment with amount = IDR 4,000,000 without a reason → observe.
  2. Record with amount = IDR 4,000,000 + reason "partial".
- **Expected result / Acceptance criteria:** (1) Rejected/blocked because amount ≠ take-home and no reason; (2) Allowed, stored amount 4,000,000 flagged as partial; **payslip take_home_pay unchanged** (immutable).
- **Traceability:** F8.4, PY-8, C-1, INV-1.

#### TC-E8-F8.4-011 · Evidence upload fails mid-record — no payment created
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** Atomicity — failed upload leaves payslip Unpaid.
- **Preconditions:** Inject an upload failure.
- **Steps:**
  1. Start recording a payment; the evidence upload fails.
- **Expected result / Acceptance criteria:** No `PayrollPayment` created; payslip stays `Unpaid`; clear error; HR can retry.
- **Traceability:** F8.4, C-3.

#### TC-E8-F8.4-012 · Large / unsupported evidence file rejected with clear message
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative
- **Objective:** File size/type validation.
- **Preconditions:** A file over the size limit and a disallowed type (e.g. .exe).
- **Steps:**
  1. Attempt to upload each as evidence.
- **Expected result / Acceptance criteria:** Both rejected with clear validation messages (size and type); no partial record.
- **Traceability:** F8.4, C-5.

#### TC-E8-F8.4-013 · Offboarded agent still payable after posting
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Offboarding after posting does not block payment.
- **Preconditions:** Agent offboarded (F2.7) after the run was posted, before payment; payslip Unpaid.
- **Steps:**
  1. Record the payment normally.
- **Expected result / Acceptance criteria:** Payment recorded; payslip Paid; no offboarding block on payment.
- **Traceability:** F8.4, C-6.

#### TC-E8-F8.4-014 · Payment amount encrypted; evidence HR-only
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** RBAC
- **Objective:** Amount encrypted at rest; evidence access role-gated.
- **Preconditions:** A recorded payment with evidence.
- **Steps:**
  1. Confirm amount is decrypted on read for HR only.
  2. Attempt to fetch the evidence file as a non-HR role.
- **Expected result / Acceptance criteria:** `amount` encrypted at rest (INV-2); evidence file only accessible to HR/Super admin (`403` otherwise).
- **Traceability:** F8.4, PY-6, INV-2.

#### TC-E8-F8.4-015 · Confirm out-of-scope: no bank/BPJS/tax integration
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Negative
- **Objective:** No external money movement or reconciliation.
- **Preconditions:** Any posted run.
- **Steps:**
  1. Inspect the payment flow for any "transfer now" / bank-connect / auto-reconcile control.
- **Expected result / Acceptance criteria:** None exist; the system only **records** a manual transfer + evidence; no money moves; no bank-statement reconciliation.
- **Traceability:** F8.4, INV-8, Non-goals.

#### Mobile · Agent POV

#### TC-E8-F8.4-016 · Agent sees payment status + paid date (no breakdown)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent's payslip reflects Paid + paid_on after recording.
- **Preconditions:** Budi's payslip recorded as Paid on 2026-07-01.
- **Steps:**
  1. Budi opens the payslip on mobile.
- **Expected result / Acceptance criteria:** Shows `payment_status=Paid` and `paid_on=2026-07-01`; **no** component breakdown; no payment reference/evidence visible to the agent.
- **Traceability:** F8.4, AC "Agent sees payment status", F8.1 PH-1, INV-3.

#### Web · Shift leader & Agent POV (RBAC)

#### TC-E8-F8.4-017 · Shift leader cannot record/void payments
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Payment recording is HR/Super-admin only.
- **Preconditions:** Shift leader authenticated.
- **Steps:**
  1. Attempt to reach the payment-recording screen / API.
- **Expected result / Acceptance criteria:** No surface; direct API → `403 FORBIDDEN`.
- **Traceability:** F8.4, PY-9, CONVENTIONS §403.

#### TC-E8-F8.4-018 · Agent cannot record payments or view evidence
- [ ] **Platform:** Web/Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Agents see status only.
- **Preconditions:** Agent authenticated.
- **Steps:**
  1. Attempt to record a payment or fetch evidence for own payslip.
- **Expected result / Acceptance criteria:** `403 FORBIDDEN`; agents have status-only visibility (F8.1).
- **Traceability:** F8.4, PY-9, INV-3.

---

### F8.5 — Payroll Period Close (month-end reconciliation gate)

#### Web · Super admin & HR admin POV

#### TC-E8-F8.5-001 · Open the cockpit auto-creates an OPEN period
- [ ] **Platform:** Web · **POV:** HR admin / Super admin · **Priority:** P0 · **Type:** Happy
- **Objective:** First access to a month with data auto-creates an OPEN PayrollPeriod.
- **Preconditions:** 2026-06 has attendance/schedule data; no PayrollPeriod row yet.
- **Steps:**
  1. Open the period cockpit for 2026-06.
- **Expected result / Acceptance criteria:** A `payroll_periods` row `(2026, 6, OPEN)` is created; cockpit shows period picker + 3 tabs (Attendance · Overtime · Leave) with per-tab blocker counts.
- **Traceability:** F8.5, PC-1.

#### TC-E8-F8.5-002 · Cannot lock with open exceptions (Lock disabled)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Lock is gated on zero FIELD blockers across all 3 tabs.
- **Preconditions:** 2026-06 has 3 FIELD attendance records PENDING and 1 OT PENDING.
- **Steps:**
  1. Open the 2026-06 cockpit; observe tab counts.
  2. Attempt to Lock.
- **Expected result / Acceptance criteria:** Attendance tab shows 3 blockers, Overtime tab 1; Lock disabled. Direct lock API → `422 PERIOD_HAS_BLOCKERS`.
- **Traceability:** F8.5, PC-3, PC-4, PC-7, AC "Cannot lock with open exceptions".

#### TC-E8-F8.5-003 · Attendance completeness computation correct
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Calc
- **Objective:** expected/recorded/clean/on_leave + coverage gap computed per PC-2.
- **Preconditions:** A FIELD agent with: 20 workable E4 entries (is_day_off=false, status SCHEDULED/MODIFIED); 18 attendance rows VERIFIED/AUTO_APPROVED; 1 approved leave day; 1 day with no record and no leave.
- **Steps:**
  1. Open the completeness view; inspect the agent's row.
- **Expected result / Acceptance criteria:** expected=20, clean=18, on_leave=1, coverage gap = 20 − (18 + 1) = **1** (uncaptured shift → blocker: confirm ABSENT or manual entry).
- **Traceability:** F8.5, PC-2, PC-3.

#### TC-E8-F8.5-004 · Each attendance blocker type is recognized
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** All PC-3 blocker categories surface.
- **Preconditions:** A period with: a PENDING record, an ESCALATED record, an open record (check_out_at NULL), a no-shift record with is_payable NULL, and a coverage gap.
- **Steps:**
  1. Open the Attendance tab.
- **Expected result / Acceptance criteria:** All five are counted as blockers and listed with drill-down to their resolution action.
- **Traceability:** F8.5, PC-3.

#### TC-E8-F8.5-005 · Resolve routes to existing actions (no new approval UI)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Blockers clear via F5.3 verify / F5.4 correct / F5.6 manual / set-payable / F5.2 auto-close.
- **Preconditions:** A period with assorted attendance blockers.
- **Steps:**
  1. From the Attendance tab, drill into a PENDING record → verify (F5.3).
  2. Resolve a no-shift NULL-payable via set-payable.
- **Expected result / Acceptance criteria:** Each resolution uses the existing E5 action; the blocker count decrements live; no duplicate approval UI in the cockpit.
- **Traceability:** F8.5, PC-6, §10 (block-and-link).

#### TC-E8-F8.5-006 · OT/Leave tabs link out to existing approval screens
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Pending OT (E7) / leave (E6) are surfaced as blockers linking to their approval screens.
- **Preconditions:** 1 PENDING OT and 1 PENDING leave overlapping 2026-06.
- **Steps:**
  1. Open the Overtime tab → click through to E7 approval, approve.
  2. Open the Leave tab → click through to E6 approval, approve.
- **Expected result / Acceptance criteria:** Each tab links to the existing approval screen; after approval the blocker clears; cockpit does not duplicate the approval UI.
- **Traceability:** F8.5, PC-4, PC-6, §10 (OT/Leave block-and-link).

#### TC-E8-F8.5-007 · Lock emits immutable PeriodEmployeeSummary per FIELD employee
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Immutability
- **Objective:** Locking with zero blockers materializes the authoritative summary.
- **Preconditions:** All 2026-06 FIELD blockers across the 3 tabs cleared.
- **Steps:**
  1. Lock 2026-06.
  2. Inspect the summary set.
- **Expected result / Acceptance criteria:** Period → `LOCKED` with `locked_by`/`locked_at`; one immutable `PeriodEmployeeSummary` per FIELD employee with attendance (payable_days, present, late, absent, no_shift_payable_days, worked_minutes), OT (approved_ot_minutes by tier), leave (paid/unpaid days); figures immutable.
- **Traceability:** F8.5, PC-7, PC-8, AC "Lock emits the immutable summary".

#### TC-E8-F8.5-008 · INTERNAL staff shown but never block the lock
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** FIELD gates the lock; INTERNAL is informational only.
- **Preconditions:** An INTERNAL staffer with a PENDING attendance record; all FIELD blockers cleared.
- **Steps:**
  1. Open the cockpit; observe the INTERNAL row; attempt to Lock.
- **Expected result / Acceptance criteria:** INTERNAL PENDING is shown for discipline but does NOT count toward blockers; Lock is enabled and succeeds.
- **Traceability:** F8.5, PC-5, C-PC-1.

#### TC-E8-F8.5-009 · Post-lock change carries forward, never mutates the summary
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Immutability
- **Objective:** A correction to a locked month emits a PayrollAdjustment, not a summary edit.
- **Preconditions:** 2026-06 LOCKED; an agent's approved correction adds a payable June day.
- **Steps:**
  1. Process the correction (F5.4) on the locked-month record.
  2. Inspect the June summary and adjustments.
- **Expected result / Acceptance criteria:** June `PeriodEmployeeSummary` is **unchanged**; a `PayrollAdjustment(status=Pending, source_type=Attendance/Correction, origin 2026-06, amount=null)` is created for the next open run.
- **Traceability:** F8.5, PC-9, C-PC-4, AC "Post-lock change carries forward".

#### TC-E8-F8.5-010 · Adjustment amount left null for the run to value
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Calc
- **Objective:** Period-close emits the event with amount=null; valuation belongs to F8.3.
- **Preconditions:** A post-lock change creating an adjustment.
- **Steps:**
  1. Inspect the created `PayrollAdjustment`.
- **Expected result / Acceptance criteria:** `amount` is null/unvalued; status PENDING; the run will value + apply it at next-run time (PR-9).
- **Traceability:** F8.5, PC-9, §11 (adjustment valuation owned by run), F8.3 PR-9.

#### TC-E8-F8.5-011 · Auto-reconcile (F5.7) is inert on a LOCKED period
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** A late shift inserts but does not re-derive locked figures.
- **Preconditions:** 2026-06 LOCKED; an UNSCHEDULED June record exists.
- **Steps:**
  1. Create a late shift covering the unscheduled June record (triggers F5.7).
  2. Inspect the locked record/summary.
- **Expected result / Acceptance criteria:** The shift inserts but F5.7 does not re-derive the locked record; the summary is unchanged; HR-driven correction would instead emit an adjustment.
- **Traceability:** F8.5, PC-11, C-PC-4, AC "Auto-reconcile inert on a locked period".

#### TC-E8-F8.5-012 · Payroll run blocked until period is locked
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** PC-10 seam — run requires LOCKED.
- **Preconditions:** 2026-06 OPEN.
- **Steps:**
  1. Attempt a 2026-06 payroll run (F8.3).
- **Expected result / Acceptance criteria:** Blocked until 2026-06 is locked; once locked, the run reads `PeriodEmployeeSummary` not live records.
- **Traceability:** F8.5, PC-10, AC "Payroll requires a locked period", F8.3 PC-10.

#### TC-E8-F8.5-013 · Month with no attendance locks with empty summary set
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** A month with no placements/all-leave still locks cleanly.
- **Preconditions:** A month with no attendance data (or all leave).
- **Steps:**
  1. Open the cockpit; Lock.
- **Expected result / Acceptance criteria:** Period locks with an empty `PeriodEmployeeSummary` set; no error.
- **Traceability:** F8.5, C-PC-8.

#### TC-E8-F8.5-014 · Cross-midnight shift counts in its work_date month
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** A shift starting on the last day of June belongs to June, not July.
- **Preconditions:** A cross-midnight shift starting 2026-06-30 23:00, clocking out 2026-07-01.
- **Steps:**
  1. Inspect which period's completeness includes it.
- **Expected result / Acceptance criteria:** Counted in June (shift start `work_date`), not July's clock-out month.
- **Traceability:** F8.5, C-PC-2.

#### TC-E8-F8.5-015 · Pending leave spanning two months blocks both
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** A cross-month pending leave is a blocker in each overlapped period.
- **Preconditions:** A PENDING leave 2026-06-29 → 2026-07-02.
- **Steps:**
  1. Open both the June and July cockpits' Leave tabs.
- **Expected result / Acceptance criteria:** The pending leave is counted in both June and July; both periods are blocked until it is decided.
- **Traceability:** F8.5, C-PC-10, PC-4.

#### TC-E8-F8.5-016 · Pre-lock approval applies in place (no adjustment)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Only post-lock changes adjust; pre-lock changes apply directly.
- **Preconditions:** 2026-06 OPEN; a correction/OT/leave approved before lock.
- **Steps:**
  1. Approve the change; then lock.
- **Expected result / Acceptance criteria:** The change is reflected directly in the summary at lock time; no PayrollAdjustment created for it.
- **Traceability:** F8.5, C-PC-5, PC-9.

#### TC-E8-F8.5-017 · Approved leave covering a gap is not a blocker
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** on_leave days fill coverage, not blockers.
- **Preconditions:** A FIELD agent with a coverage gap fully covered by approved leave.
- **Steps:**
  1. Open the Attendance tab.
- **Expected result / Acceptance criteria:** The covered day counts as `on_leave`, not a blocker.
- **Traceability:** F8.5, C-PC-3, PC-2.

#### TC-E8-F8.5-018 · Lock and transitions are audited
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Happy
- **Objective:** PERIOD_LOCKED audited with before/after status + counts.
- **Preconditions:** A lockable period.
- **Steps:**
  1. Lock; inspect audit.
- **Expected result / Acceptance criteria:** Audit entry `PERIOD_LOCKED` with actor, timestamp, before/after status, resolved/forced counts.
- **Traceability:** F8.5, PC-14.

#### Web · Super admin-only deltas

#### TC-E8-F8.5-019 · Super-admin force-lock with reason overrides nonzero blockers
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Force-lock for genuinely-stuck records, reason required + stored.
- **Preconditions:** 2026-06 has 1 absconded agent's ABSENT-pending record (blocker); super admin logged in.
- **Steps:**
  1. Force-lock 2026-06 with reason "agent absconded, record stuck".
- **Expected result / Acceptance criteria:** Period locks despite nonzero count; `force_locked=true`, `force_lock_reason` stored; audited as `PERIOD_FORCE_LOCKED`.
- **Traceability:** F8.5, PC-7, PC-14, PC-15, AC "Super-admin force-lock with reason".

#### TC-E8-F8.5-020 · HR admin cannot force-lock (super-admin only)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** RBAC
- **Objective:** Force-lock is restricted to super admin.
- **Preconditions:** HR admin (not super) logged in; a period with blockers.
- **Steps:**
  1. Attempt force-lock (with reason).
- **Expected result / Acceptance criteria:** Force-lock control unavailable / `403 FORBIDDEN`; HR can only lock when blockers=0.
- **Traceability:** F8.5, PC-7, PC-15.

#### TC-E8-F8.5-021 · Super-admin reopen (no Posted run) voids summary
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Reopen returns to OPEN and voids the summary.
- **Preconditions:** 2026-06 LOCKED; no payroll run Posted for June.
- **Steps:**
  1. Reopen 2026-06 with reason; inspect status + summary.
- **Expected result / Acceptance criteria:** Period → `OPEN` (via REOPENED); `PeriodEmployeeSummary` voided; `reopened_by`/`reopened_at` stamped; audited `PERIOD_REOPENED`.
- **Traceability:** F8.5, PC-12, PC-14, AC "Reopen only before posting".

#### TC-E8-F8.5-022 · Reopen rejected after a run is Posted
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Once a run is Posted, history is fixed — adjustments only.
- **Preconditions:** 2026-06 LOCKED and its payroll run Posted.
- **Steps:**
  1. Attempt to reopen 2026-06.
- **Expected result / Acceptance criteria:** Rejected with `409 PERIOD_RUN_POSTED`; period stays LOCKED; corrections must use PayrollAdjustment.
- **Traceability:** F8.5, PC-12, C-PC-9, AC "Reopen only before posting".

#### TC-E8-F8.5-023 · HR admin cannot reopen (super-admin only)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** RBAC
- **Objective:** Reopen restricted to super admin.
- **Preconditions:** HR admin; a LOCKED period (no Posted run).
- **Steps:**
  1. Attempt reopen.
- **Expected result / Acceptance criteria:** No reopen control / `403 FORBIDDEN`.
- **Traceability:** F8.5, PC-12, PC-15.

#### TC-E8-F8.5-024 · Rerun reads the same summary; adjustments apply once
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Two runs touching the same locked month are consistent.
- **Preconditions:** 2026-06 LOCKED; a run executed, then a rerun.
- **Steps:**
  1. Execute a run for June, then rerun.
- **Expected result / Acceptance criteria:** Period stays LOCKED; both reads use the same `PeriodEmployeeSummary`; any adjustment applies only once.
- **Traceability:** F8.5, C-PC-7.

#### Clarification back-channel (Web + Mobile)

#### TC-E8-F8.5-025 · HR raises a clarification to the record's shift leader
- [ ] **Platform:** Web · **POV:** HR admin / Super admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Raise a one-round clarification on an ambiguous record.
- **Preconditions:** An ambiguous June attendance record whose company has a shift leader.
- **Steps:**
  1. From the cockpit drill-down, raise a `ClarificationRequest` with a question.
- **Expected result / Acceptance criteria:** `clarification_requests` row `status=OPEN`; target auto-resolved to the record's company shift leader; an E10 notification (push + in-app, deep-linked to the record) is sent; the cockpit shows "awaiting reply"; audited `CLARIFICATION_RAISED`.
- **Traceability:** F8.5, PC-13, PC-14.

#### TC-E8-F8.5-026 · Clarification does not block the lock (advisory)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** An OPEN clarification surfaces but blocks nothing on its own.
- **Preconditions:** A period with an OPEN clarification but zero FIELD blockers.
- **Steps:**
  1. Attempt to Lock.
- **Expected result / Acceptance criteria:** Lock is allowed; the OPEN clarification is advisory only (surfaced as "awaiting reply"), not a blocker.
- **Traceability:** F8.5, PC-13.

#### TC-E8-F8.5-027 · Clarification target = agent without app access falls back to SL
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Target resolution fallback chain.
- **Preconditions:** A record whose agent has no app access; company has a shift leader.
- **Steps:**
  1. Raise a clarification.
- **Expected result / Acceptance criteria:** Target falls back to the record's shift leader; if none, the placement's HR owner.
- **Traceability:** F8.5, PC-13, C-PC-6.

#### TC-E8-F8.5-028 · HR re-asks after one round; resolve/cancel closes the loop
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** One-round model with re-ask and terminal states.
- **Preconditions:** A clarification answered (ANSWERED).
- **Steps:**
  1. Resolve the clarification; in a separate case, cancel an OPEN one; in a third, re-ask after an answer.
- **Expected result / Acceptance criteria:** States move `OPEN → ANSWERED → RESOLVED` (or `CANCELLED`); HR may re-ask (new round); each transition audited.
- **Traceability:** F8.5, PC-13, PC-14.

#### TC-E8-F8.5-029 · Shift leader answers a clarification (scope-checked)
- [ ] **Platform:** Web/Mobile · **POV:** Shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** SL receives the notification, answers, optionally files a correction.
- **Preconditions:** A clarification raised to the SL's own company record.
- **Steps:**
  1. SL opens the deep-linked notification; answers with a note.
  2. Optionally files an F5.4 correction.
- **Expected result / Acceptance criteria:** SL can answer (status → ANSWERED, `answered_by` stamped); an optional F5.4 correction routes normally; SL has no other period actions.
- **Traceability:** F8.5, PC-13, PC-15, AC "Clarification round-trip".

#### TC-E8-F8.5-030 · Shift leader cannot answer a clarification outside their scope
- [ ] **Platform:** Web/Mobile · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Scope check — SL can only answer clarifications for their own company.
- **Preconditions:** A clarification raised on a record from another company.
- **Steps:**
  1. Attempt to answer it (crafted request).
- **Expected result / Acceptance criteria:** `403 OUT_OF_SCOPE` (or `404`); SL cannot answer/see out-of-scope clarifications.
- **Traceability:** F8.5, PC-13, PC-15, CONVENTIONS §OUT_OF_SCOPE.

#### TC-E8-F8.5-031 · Agent answers a clarification addressed to them (mobile)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Agent target can answer on mobile.
- **Preconditions:** A clarification targeted at agent Budi.
- **Steps:**
  1. Budi opens the push/in-app notification; answers with a note.
- **Expected result / Acceptance criteria:** Status → ANSWERED; agent may also file an F5.4 correction; no other period actions available.
- **Traceability:** F8.5, PC-13, PC-15.

#### Web · Shift leader & Agent POV (RBAC — cockpit)

#### TC-E8-F8.5-032 · Shift leader cannot open the period cockpit
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Cockpit (review/lock/reopen/raise) is HR/Super-admin only.
- **Preconditions:** Shift leader authenticated.
- **Steps:**
  1. Attempt to open the cockpit / lock / raise-clarification APIs.
- **Expected result / Acceptance criteria:** `403 FORBIDDEN`; SL has no period actions beyond answering clarifications.
- **Traceability:** F8.5, PC-15.

#### TC-E8-F8.5-033 · Agent cannot access the period cockpit
- [ ] **Platform:** Web/Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Agents have no cockpit access.
- **Preconditions:** Agent authenticated.
- **Steps:**
  1. Attempt to reach any period-close endpoint except answering own clarification.
- **Expected result / Acceptance criteria:** `403 FORBIDDEN` for all cockpit actions; only own-clarification answer permitted.
- **Traceability:** F8.5, PC-15.

#### TC-E8-F8.5-034 · Cockpit loading / error / empty states
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Cockpit handles loading, error, and a clean (zero-blocker) state gracefully.
- **Preconditions:** Throttle/inject errors; also a fully-reconciled period.
- **Steps:**
  1. Open the cockpit on slow network → loading.
  2. Force a 5xx on completeness fetch → error with retry.
  3. Open a fully-reconciled period → all tabs show 0 blockers, Lock enabled.
- **Expected result / Acceptance criteria:** Loading skeleton; graceful error + retry (no white screen); clean state shows 0 blockers and an enabled Lock with a clear "ready to lock" affordance.
- **Traceability:** F8.5, E8 G-1.
