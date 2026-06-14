# PRD · F8.4 — Payment Recording & Transfer Evidence (manual)

> **Epic:** E8 Payroll · **Feature:** F8.4 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Once a payroll run is **posted** (F8.3), HR pays agents by **bank transfer executed in their own banking channel** (outside the system) — v1 has **no bank API** (INV-8). The system's job is to **record** that a payment happened: capture the transfer date, reference, and **uploaded bukti transfer** (proof), then mark the payslip `Paid`. This closes the loop (computed → paid → evidenced) for audit and for the agent's payslip view, without integrating any external service.

## 2. Goals & non-goals

**Goals**
- For each posted payslip, HR records a payment: `paid_on`, `method`, `reference_no`, and an **evidence file** (the transfer receipt).
- A payslip becomes `Paid` only when a `PayrollPayment` with evidence exists (INV-8).
- Support **batch** recording (one evidence file / reference covering many payslips, e.g. a bulk-transfer receipt) and **per-payslip** recording.
- Everything audited; agents see payment status + `paid_on` (F8.1).

**Non-goals**
- Moving money / connecting to a bank, BPJS, or tax authority (manual, outside).
- Reconciling against bank statements automatically.
- Editing payslip amounts (immutable, F8.3).

## 3. Actors

HR / Super Admin (record payment, upload evidence), System (validate evidence, set status, audit), Agent (sees status only, F8.1).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | View posted run's payment list; record per-payslip or batch payment + upload evidence. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| PY-1 | Payment can only be recorded against a payslip whose run is **Posted** and that is currently `Unpaid`. |
| PY-2 | A `PayrollPayment` requires `paid_on`, `method` (BankTransfer / Cash), and an **evidence file**; `reference_no` required for BankTransfer. |
| PY-3 | On a valid payment, set the payslip `payment_status=Paid` and `paid_on` (INV-8). |
| PY-4 | **Batch payment:** HR selects N unpaid payslips and records one payment event (shared `paid_on`/`reference_no`/evidence); the system creates a `PayrollPayment` per payslip linking the same evidence. |
| PY-5 | A payslip already `Paid` cannot be paid again; **reversing** a payment requires an audited void (creates a reversal record; does not delete history). |
| PY-6 | Evidence files are stored via the platform file store; access is **HR/Super Admin only**; payment `amount` is **encrypted** (INV-2). |
| PY-7 | Recording, batch recording, and void are **audited** (who/when/which payslips). |
| PY-8 | The recorded `amount` **defaults to** the payslip `take_home_pay` and must match it unless HR overrides with a reason (e.g. partial) — *(default: must equal take-home; partials flagged)*. |
| PY-9 | **Scope/RBAC:** only HR/Super Admin record payments; server enforces (client RBAC defense-in-depth). |

## 6. Data model

Writes `PayrollPayment`; updates `Payslip.payment_status` + `paid_on` (FEATURE §4). Uses E1 file storage for evidence. No new entities beyond `PayrollPayment`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Payment recording and transfer evidence

  Scenario: Record a single payment with evidence
    Given I am HR
    And a posted payslip for agent "Budi" is unpaid
    When I record a bank transfer with date, reference number, and upload the receipt
    Then the payslip is marked paid with that pay date
    And the action is audited

  Scenario: Evidence is required
    Given a posted unpaid payslip
    When I try to record a payment without uploading evidence
    Then it is rejected and the payslip stays unpaid

  Scenario: Batch payment for a run
    Given 20 posted unpaid payslips
    When I select all 20 and record one bulk transfer with a single receipt
    Then 20 payment records are created linking the same evidence
    And all 20 payslips are marked paid

  Scenario: Cannot double-pay
    Given a paid payslip
    When I try to record another payment
    Then it is rejected

  Scenario: Void a payment
    Given a paid payslip
    When I void the payment with a reason
    Then a reversal is recorded, the payslip returns to unpaid, and history is preserved

  Scenario: Agent sees payment status
    Given Budi's payslip is paid
    When Budi opens it on mobile
    Then he sees it as paid with the pay date (no component breakdown)
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Partial / different amount transferred | Allowed with reason + flag (PY-8); amount stored, payslip take-home unchanged. |
| C-2 | Wrong evidence uploaded | HR voids (PY-5) and re-records; both audited. |
| C-3 | Evidence upload fails mid-record | No `PayrollPayment` created; payslip stays unpaid. |
| C-4 | Batch contains an already-paid payslip | Skipped with a notice; only unpaid ones processed. |
| C-5 | Large evidence file / unsupported type | Validate size/type; reject with a clear message. |
| C-6 | Agent offboarded after posting but before payment | Still payable; payment recorded normally. |

## 9. Dependencies

F8.3 (posted payslips), E1 (RBAC, audit, file storage, crypto), F8.1 (agent payment-status view).

## 10. Decisions & open questions

- ✅ **Manual payment** — record reference + evidence; no bank API (INV-8).
- ✅ **Evidence required** to mark Paid; per-payslip and batch supported.
- ✅ **Void, never delete** — reversals are audited.
- **Open:** allowed evidence file types/size limits (build-phase config).
- **Open:** whether partial/over payments are permitted in v1 *(default: must equal take-home, partials flagged)*.
