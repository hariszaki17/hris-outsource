# PRD · F7.3 — Overtime Approval (via the E11 engine)

> **Epic:** E7 Overtime Tracking · **Feature:** F7.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_
> **Reworked 2026-06-14 (EPICS §8 E11):** overtime no longer runs its own `shift_leader → HR/lead` state machine. It **routes through the configurable E11 approval engine** (per-company chain). This PRD now specifies only overtime's **on-approval side-effect** (the `OnApproved` hook: count hours by day-type) and what OT contributes to the chain; **routing, lines, self-block, reject, bypass, bulk, and the decision trail are owned by [E11](../../E11-approvals/FEATURE.md)**.

---

## 1. Context & problem

Overtime costs money and is billable, so it needs sign-off before it counts. **Who** signs off (and in what order) is now the **per-company approval template** (E11 F11.1) — typically the on-site shift leader who knows whether the extra work was real, then HR/account-manager. Overtime's remaining responsibility is to **react** to the engine's terminal decision: count the approved hours by `day_type` tier (F7.4) on approval. Same engine as leave.

## 2. Goals & non-goals

**Goals**
- On capture (request or confirmed auto-detect), **create an E11 `ApprovalInstance`** (`request_type = OVERTIME`) for the agent's company.
- Register the OT **`OnApproved`** hook: count approved hours by `day_type` tier (F7.4).
- Support bulk approval via E11 (multiple instances' current lines at once).

**Non-goals**
- Routing / lines / OR-membership / sequential advance / self-block / super-admin bypass / decision trail — **all E11** (F11.2). Capture (F7.2). Rules (F7.1). Pay calc (out of scope).

## 3. Actors

Line members (per the company's E11 template — e.g. shift leader, lead, HR), Super Admin (E11 bypass), System (create instance, run hook, count hours, notify), Agent (requester/confirmer; notified).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web / mobile — Kotak Masuk** | Line members | Approve/reject the current line of an OT instance (E11 F11.3 inbox). |
| **Web console — Lembur → Approvals** | Line members | Same instances via the per-domain tab (view over the E11 source, IB-5); bulk approve. |
| **Web / mobile** | Agent | Request / confirm auto-detected OT (F7.2); watch the chain timeline (E11). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| OA-1 | **Routing is E11.** On capture, OT creates an `ApprovalInstance` (`request_type = OVERTIME`, `record_id`, `company_id` = the agent's company). Chain, OR-within-line, sequential advance, self-block (INV-3), terminal reject (INV-4), super-admin bypass (INV-5) are **owned by E11 F11.2**. *(Supersedes the old two-level `shift_leader → HR/lead` OA-1/OA-2/OA-3.)* |
| OA-2 | **Auto-detected candidates** enter the engine **after** any required agent confirmation (F7.2 OC-7) — the candidate is confirmed, then an instance is created. |
| OA-3 | **No template → super-admin fallback** (E11 INV-7) — OT still routes (never auto-approves, never blocks). *(Replaces the old "no leader → HR" escalation.)* |
| OA-4 | **`OnApproved` hook (terminal approve/bypass).** The engine fires OT's hook in its transaction (E11 INV-8): the OT **counts toward reporting (F7.4), classified by `day_type`** (HOLIDAY > RESTDAY > WORKDAY). |
| OA-5 | **`OnRejected` hook.** On terminal reject (E11 INV-4), nothing counts; the underlying attendance record is unaffected. |
| OA-6 | **Decision trail = `approval_actions`** (E11 INV-9) — append-only per approve/reject/bypass, audited; agent notified (E10). *(Replaces the `overtime_approvals` table.)* |
| OA-7 | **Bulk approval** is supported via E11 (`:bulk-approve` over instance ids; CONVENTIONS §14) — each clears its current line for the caller. |

## 6. Data model

No OT-owned approval table. The engine owns `ApprovalInstance` + `ApprovalAction` (E11). OT keeps `OvertimeRecord.status` in sync with the instance (`PENDING` while routing; `APPROVED`/`REJECTED`/`CANCELLED` on terminal). The old `OvertimeApproval` entity is **removed** (superseded by `approval_actions`).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Overtime approval via the E11 engine

  Background:
    Given Budi has a Pending 2-hour OT record at "Plaza Senayan"
    And Plaza Senayan's approval template is line 1 [Sari], line 2 [HR]

  Scenario: Chain approves, hours count
    When line 1 [Sari] approves and line 2 [HR] approves (E11)
    Then the OT OnApproved hook counts the hours by day_type and the record becomes APPROVED

  Scenario: Reject ends it
    When a current-line member rejects with a reason (E11)
    Then the record becomes REJECTED, nothing counts, and Budi is notified

  Scenario: Auto-detected candidate
    Given an auto-detected OT candidate awaiting Budi's confirmation
    When Budi confirms it
    Then an E11 instance is created and routed through the chain

  Scenario: No template falls back to super admin
    Given Budi's company has no approval template
    Then his OT routes to the E11 super-admin fallback line

  Scenario: Cannot self-approve (E11 INV-3)
    Given a line member has their own OT record and is on its chain
    Then they cannot clear that line; another member must (or super-admin bypass)

  Scenario: Bulk approve
    Given five pending OT instances where I am the current-line member
    When I bulk approve them (E11)
    Then each clears my line and advances
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Auto-detected candidate rejected | Status Rejected (E11); the attendance record itself is unaffected (OA-5). |
| C-2 | Underlying attendance corrected after OT approval | OT re-derived/flagged for re-review (E5 correction propagation) — outside the engine. |
| C-3 | Agent withdraws a pending request | Allowed before finalize → Cancelled; instance closed (E11 C-1). |
| C-4 | Approve OT for a now-ended placement | Allowed (work already happened); audited. |
| C-5 | Super-admin bypass | Force-approve (E11 INV-5); `OnApproved` still counts the hours. |

## 9. Dependencies

**E11** (approval engine — routing, instance, actions, bypass, fallback, bulk), F7.2 (records/capture), F7.4 (counting), E5 (correction propagation), E10 (notifications), E1 (audit).

## 10. Decisions & open questions

- ✅ **Overtime routes through the E11 engine** (per-company configurable chain); OT owns only the `OnApproved` (count-by-day-type) side-effect (2026-06-14). Supersedes the two-level `shift_leader → HR/lead` model (OA-1/OA-2/OA-3) and the HR-override (now super-admin bypass).
- **Open:** SLA/auto-escalation if a line sits un-actioned (tracked in E11, v1: none).
- **Open (C-2):** how attendance corrections propagate to already-approved OT (recompute vs flag).
