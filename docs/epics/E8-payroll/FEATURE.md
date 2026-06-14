# E8 — Payroll (historical archive + compute-assist runs) · Feature Document

> **Epic:** E8 Payroll · **Status:** Draft v2 · **Parent:** [EPICS.md](../../EPICS.md)
> Two jobs: **(1)** preserve and display **migrated historical** payroll (read-only), and **(2)** run a **monthly compute-assist payroll** — the system computes each agent's pay from E2/E5/E6/E7, HR reviews and posts immutable payslips, then records the **manually executed** bank transfer with evidence. **No external API** (no bank / BPJS / tax-authority integration) in v1.

> **Scope change (2026-06-11):** v1 was *data-only* (no active runs). Ratified to **active compute-assist payroll** — see [EPICS.md §8 E8](../../EPICS.md). Migrated history stays read-only and immutable; new runs add F8.3/F8.4.

---

## 0. Two money flows — keep separate (anchor)

The differentiator of an outsourcing business is two **distinct** money flows; this epic owns only the second.

| Flow | Direction | Unit | Owner |
|---|---|---|---|
| **Client billing** | client → SWP (revenue) | **hours** (rate applied **outside** the system) | E5 verified attendance → E10 billable report |
| **Agent payroll** | SWP → agent (cost) | **monthly wage** (base + additions − deductions) | **E8 (this epic)** |

**Agent payroll is NOT computed from billable hours.** Billable hours invoice the *client*. An agent is paid a **monthly salary** (E2 `EmploymentAgreement.base_salary`), modulated per period by verified attendance, approved OT, and leave. This matches Indonesian alih-daya practice (monthly UMR/UMP-based wage) and the legacy model (`gaji_pokok` monthly + `total_hari_kerja` working-days), **not** an hourly wage. See INV-6.

## 1. Goal & outcome

1. **History (continuity):** migrate SWP's existing payroll and surface it read-only — agents see their own past payslip summaries (mobile); HR keeps the full archive (components + benefits) for compliance.
2. **Compute-assist runs (new):** each month HR opens a **payroll run** scoped to a population; the system **assembles** per-agent pay from authoritative upstream data (E2 base, E5 verified attendance, E6 leave, E7 approved OT), HR reviews/adjusts editable component lines, **posts** to generate **immutable** payslips, then records the **manual** transfer + uploads evidence. Late-verified upstream changes after cutoff carry forward as next-period adjustments — posted payslips never change.

**Explicitly out for v1:** bank/BPJS/tax integration; an automatic statutory (BPJS/PPh21) calculation engine (statutory amounts are entered as **editable component lines**, assisted by stored config); client invoicing (stays hours-only, outside).

## 2. Actors & roles

| Actor | Involvement |
|---|---|
| **Agent** | Views own payslip summaries — migrated + generated (mobile, read-only). |
| **HR / Super Admin** | Runs payroll (open → review → post), edits component lines, records payment + uploads transfer evidence; full archive read + export. |
| **System** | Assembles pay from E2/E5/E6/E7; enforces read-only history, immutability of posted payslips, encryption, scope; carries late changes forward as adjustments; audits. |

> Finance sub-role: **none in v1** — HR performs payroll runs and payments (consistent with EPICS §8). A dedicated Finance role is a future option.

## 3. Scope

**In scope:** migrated payslip history (agent summary + HR archive, read-only); **monthly compute-assist payroll run** (assemble → review/adjust → post immutable payslips); **payment recording** (manual transfer reference + evidence upload, per payslip or batch); **prior-period carry-forward adjustments**; encrypted comp at rest; audit + export.

**Out of scope (v1):** automatic BPJS/PPh21 calculation engine (statutory lines are HR-entered/editable); bank / BPJS / tax-authority API integration; client invoicing & rate application (E10 hours-only, outside); editing a **posted** payslip; multi-currency (IDR only).

## 4. Domain entities

```mermaid
erDiagram
    PAYROLL_RUN ||--o{ PAYSLIP : "generates"
    EMPLOYEE ||--o{ PAYSLIP : "paid via"
    PAYSLIP ||--o{ SALARY_COMPONENT : "line items"
    PAYSLIP ||--o| PAYROLL_PAYMENT : "settled by"
    EMPLOYEE ||--o{ BENEFIT : "archive"
    EMPLOYEE ||--o{ PAYROLL_ADJUSTMENT : "accrues"
    PAYROLL_RUN ||--o{ PAYROLL_ADJUSTMENT : "consumes"

    PAYROLL_RUN {
        bigint id PK
        int year
        int month
        json scope "filter snapshot: all|company"
        date cutoff_date "attendance/OT cutoff"
        string status "Draft|Posted|Closed"
        int payslip_count
        bigint posted_by FK "null until posted"
        datetime posted_at
        bigint created_by FK
    }
    PAYSLIP {
        bigint id PK
        bigint employee_id FK
        bigint payroll_run_id FK "null = migrated historical"
        string source "Migrated|Generated"
        int year
        int month
        date paid_on "null until paid"
        int working_days
        decimal gross_earnings "encrypted"
        decimal gross_deductions "encrypted"
        decimal take_home_pay "encrypted"
        boolean is_posted
        string payment_status "Unpaid|Paid"
    }
    SALARY_COMPONENT {
        bigint id PK
        bigint payslip_id FK
        string name
        string kind "Earning|Deduction"
        string category "Base|OT|Allowance|BPJS|PPh21|Adjustment|Other"
        decimal value "encrypted"
        string source "Auto|Manual"
        json basis "traceability: e.g. OT hours+multiplier, days"
        boolean for_bpjs
    }
    PAYROLL_PAYMENT {
        bigint id PK
        bigint payslip_id FK
        date paid_on
        decimal amount "encrypted"
        string method "BankTransfer|Cash"
        string reference_no
        bigint evidence_file_id FK "uploaded bukti transfer"
        bigint paid_by FK
        datetime created_at
    }
    PAYROLL_ADJUSTMENT {
        bigint id PK
        bigint employee_id FK
        string source_type "Attendance|Overtime|Correction|Manual"
        bigint source_id "null for manual"
        int origin_year
        int origin_month
        string note
        decimal amount "encrypted, signed"
        string status "Pending|Applied"
        bigint applied_run_id FK "null until applied"
    }
    BENEFIT {
        bigint id PK
        bigint employee_id FK
        string name
        decimal value "encrypted"
    }
```

**Invariants:**
- **INV-1:** a **posted** payslip is **immutable** — no edits to its totals or component lines after `is_posted` (migrated payslips are posted on import). Corrections flow forward via `PayrollAdjustment`, never by editing a posted payslip.
- **INV-2:** all monetary fields (`*_earnings`, `*_deductions`, `take_home_pay`, component `value`, payment `amount`, benefit `value`, adjustment `amount`) are **encrypted at rest**; access is role-gated; decrypt on read for authorized viewers only.
- **INV-3:** an agent sees **only their own** payslip **summaries** (take-home, gross earnings, gross deductions, working days, period, payment status) — **not** the component breakdown.
- **INV-4:** the full breakdown (`SALARY_COMPONENT`) + benefits are **HR/Super Admin only**.
- **INV-5:** payroll math is **agent-pay only inside SWP** — **no client invoicing / rate application** in E8 (that stays outside, hours-only per E10).
- **INV-6:** the agent pay base is the **monthly** `EmploymentAgreement.base_salary` (E2), **not** an hourly rate. Attendance/OT/leave **modulate** the monthly amount (proration + additions/deductions); they do not define an hourly wage.
- **INV-7:** a payroll run **assembles** from authoritative upstream data (E2/E5/E6/E7) at run time; only **verified** attendance (E5) and **Approved** OT (E7) are eligible (consistent with E5/E10 "verified-only").
- **INV-8:** payment is **manual** — the system records a transfer reference + evidence; it does **not** move money. A payslip is `Paid` only once a `PayrollPayment` with evidence exists.

## 5. Features

| ID | Feature | PRD |
|----|---------|-----|
| **F8.1** | Payslip History & Summaries (read-only; migrated + generated) | [payslip-history.md](prds/payslip-history.md) |
| **F8.2** | Payroll Archive & Retention (HR) | [payroll-archive.md](prds/payroll-archive.md) |
| **F8.3** | Compute-Assist Payroll Run (assemble → review → post) | [payroll-run.md](prds/payroll-run.md) |
| **F8.4** | Payment Recording & Transfer Evidence (manual) | [payroll-payment.md](prds/payroll-payment.md) |

## 6. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | View own payslip summaries (migrated + generated), read-only. |
| **Web console** | HR / Super Admin | Run payroll, review/adjust, post; record payment + upload evidence; full archive + export. |

---

### F8.1 — Payslip History & Summaries (read-only)

Agents view their own payslips at summary level (take-home, gross earnings, gross deductions, working days, pay date, period, **payment status**) — both **migrated** historical and newly **generated** ones (single unified list). HR can view any agent's. No component breakdown at this level.

```mermaid
flowchart LR
    subgraph AG[Agent - mobile]
        A1([My payslips]) --> A2[List by period: take-home, gross, paid date, status]
        A2 --> A3[Open a payslip summary]
    end
    subgraph SYS[System]
        A3 --> S1{Scope = self?}
        S1 -- No --> S2[Deny]
        S1 -- Yes --> S3[Decrypt + render summary]
    end
```

**Entities:** reads `Payslip`. **Depends on:** E2 (employee), E9 (migrated payslips), F8.3 (generated payslips).

---

### F8.2 — Payroll Archive & Retention (HR)

The full payroll dataset (payslips + their salary-component line items + benefits + payments), preserved for HR lookup, audit, and compliance retention, with export. Covers both migrated and generated payslips; generated ones additionally expose the run + payment trail.

```mermaid
flowchart LR
    subgraph HR[HR / Super Admin - web]
        B1([Payroll archive]) --> B2[Search by employee / period / run]
        B2 --> B3[View payslip + components + benefits + payment]
        B3 --> B4{Export for audit?}
        B4 -- Yes --> B5[Export + audit log]
    end
    subgraph SYS[System]
        B2 --> Q1[Role-gated, decrypt on read]
    end
```

**Entities:** reads `Payslip`, `SalaryComponent`, `Benefit`, `PayrollPayment`. **Depends on:** E9 (migration), E1 (RBAC/audit), F8.3/F8.4.

---

### F8.3 — Compute-Assist Payroll Run

HR opens a monthly run, the system **assembles** each agent's draft payslip from upstream data, HR reviews and adjusts **editable** component lines, then **posts** to generate immutable payslips. Late-verified upstream changes after the cutoff are carried forward as next-period `PayrollAdjustment` lines.

**Assembly (per agent, draft):**
- **Base** = `EmploymentAgreement.base_salary` (E2), prorated only for mid-period join/leave and unpaid absence/leave (full-month workers not prorated). *(default)*
- **Proration / absence** = from **verified** E5 attendance against `AttendanceCode.is_payable`; non-payable codes reduce pay.
- **Overtime** = sum of **Approved** E7 `OvertimeRecord` hours × `OvertimeRule.multiplier` × **hourly base**, where hourly base = `base_salary / 173` *(default — Permenaker formula, configurable)*.
- **Leave** = E6 paid leave (no deduction) vs unpaid leave (deduction).
- **Statutory & allowances** = **editable component lines** (BPJS employee portion, PPh21, allowances) — assisted by stored config but **HR-entered/editable**, not an auto-engine.
- **Prior-period adjustments** = any `PayrollAdjustment(status=Pending)` for the agent appended as `Adjustment` lines.
- Totals: `gross_earnings`, `gross_deductions`, `take_home_pay` recomputed live in draft.

```mermaid
flowchart TD
    R1([HR: open run year/month + scope + cutoff]) --> R2[System assembles draft payslips]
    R2 --> R3[Per agent: base +/- proration + OT + leave + statutory lines + pending adjustments]
    R3 --> R4[HR reviews; edit Manual component lines]
    R4 --> R5{Totals OK?}
    R5 -- No --> R4
    R5 -- Yes --> R6[HR posts run]
    R6 --> R7[Payslips set is_posted=true, immutable]
    R7 --> R8[Pending adjustments marked Applied]
    R8 --> R9[Ready for payment F8.4]
    L1[Late E5/E7 change after cutoff on a posted period] --> L2[System creates PayrollAdjustment Pending]
    L2 -. consumed next run .-> R3
```

**Eligibility:** only **verified** attendance (E5) and **Approved** OT (E7) (INV-7). Pending/unverified upstream records are excluded from the draft and, if verified after cutoff, become adjustments.

**Entities:** writes `PayrollRun`, `Payslip`, `SalaryComponent`, `PayrollAdjustment`. Reads E2 `EmploymentAgreement`, E5 `Attendance`/`AttendanceCode`, E6 leave, E7 `OvertimeRecord`/`OvertimeRule`. **Depends on:** E1 (RBAC/audit), E2, E5, E6, E7.

---

### F8.4 — Payment Recording & Transfer Evidence (manual)

After a run is posted, HR executes transfers in their own bank channel (outside the system) and records each payment with a reference and **uploaded bukti transfer**. A payslip becomes `Paid` only when a `PayrollPayment` with evidence exists (INV-8). Supports per-payslip and batch recording.

```mermaid
flowchart LR
    subgraph HR[HR - web]
        P1([Posted run: payment list]) --> P2[Transfer in bank channel - outside system]
        P2 --> P3[Record payment: paid_on, reference_no, upload evidence]
        P3 --> P4[Batch or per-payslip]
    end
    subgraph SYS[System]
        P3 --> Q1{Evidence attached?}
        Q1 -- No --> Q2[Stay Unpaid]
        Q1 -- Yes --> Q3[payment_status=Paid, set paid_on, audit]
    end
```

**Entities:** writes `PayrollPayment`, updates `Payslip.payment_status`/`paid_on`. **Depends on:** F8.3, E1 (audit + file storage).

---

## 7. Decisions & open questions

**Resolved (2026-05-29) — history layer:**
- ✅ Migrated payroll is **read-only** and **immutable** (HR may annotate via an audited note; no edits).
- ✅ Agents view **own payslip summaries** (mobile); HR has the full archive; **summary-level** for agents, components HR-only.
- ✅ **No forward payroll-input export** from E8 (E5/E7 exports feed external client-billing; E8 owns agent pay).

**Resolved (2026-06-11) — active payroll (see [EPICS §8 E8](../../EPICS.md)):**
- ✅ **Flip to compute-assist payroll** — system assembles from E2/E5/E6/E7, HR posts **immutable** payslips. (Reverses the 2026-05-29 "data-only v1".)
- ✅ **Monthly wage base**, not hourly (INV-6) — consistent with E2 `base_salary` (locked 2026-06-07) and legacy.
- ✅ **Manual payment** — system records transfer reference + evidence; no bank API (INV-8).
- ✅ **No statutory auto-engine** — BPJS/PPh21 are **editable component lines** (assisted by config); a full statutory calculator is a future epic.
- ✅ **Late-verified upstream → next-period carry-forward adjustment** (INV-1 protects posted payslips).
- ✅ OT pay = hours × `OvertimeRule.multiplier` × hourly base; **hourly base = base_salary / 173** *(default, configurable — Permenaker)*.
- ✅ Finance sub-role **none in v1** — HR runs payroll.

**Still open:**
1. Payroll **retention period** + whether purge is ever permitted (compliance input).
2. Proration rule for mid-period join/leave — calendar-day vs working-day divisor *(default: calendar-day)*; confirm with payroll.
3. Whether a posted run can be **re-opened before payment** (vs adjustment-only). *(default: no re-open; Draft→Posted is one-way)*.
4. Payslip **PDF** generation/download (deferred from history layer) — still later.
