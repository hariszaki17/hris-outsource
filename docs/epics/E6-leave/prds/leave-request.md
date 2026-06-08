# PRD · F6.2 — Leave Request (documents, delegate)

> **Epic:** E6 Leave Management · **Feature:** F6.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Agents need to request time off from mobile — choosing a leave type, a date range, optionally naming a delegate to cover, and attaching a document when the type requires it (e.g., sick note). Requests must respect the **available balance** up front (the per-employee grant-lot pool, F6.1) so agents aren't surprised at approval. The leave_type is only a **label + document gate** — balance is drawn from the **one pool** via FIFO across lots (F6.1 LQ-9/LQ-10).

## 2. Goals & non-goals

**Goals**
- Submit a leave request (type, dates, computed duration, reason).
- Enforce document upload for document-required types; capture an optional delegate.
- Pre-check **available balance** (FIFO across eligible grant-lots) and **reserve** it (`pending_days`); block over-balance.
- Allow withdraw/cancel before final approval.

**Non-goals**
- Approval (F6.3). Balance grant/expiry (F6.1). Schedule effect (F6.4).

## 3. Actors

Agent (requester, mobile), Shift Leader/HR (may file on behalf), System (validate, persist, notify).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent | Create/withdraw requests; upload documents; view status. |
| **Web** | Shift Leader / HR | File on behalf; review incoming. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| LR-1 | A request requires: leave type (E2), start/end date, reason. `duration_days` is computed from the range (counting rule per FEATURE §7 Q1). |
| LR-2 | If the leave type `is_document_required` (E2), a **document upload is mandatory** before submit (INV-5). |
| LR-3 | `duration_days` must be ≤ the **available balance** — the sum over eligible (unexpired, matching-earmark) grant-lots of `amount − consumed − pending` (F6.1) — else **blocked** (INV-1). On a clean submit the system **FIFO-reserves** `pending_days` across lots by soonest expiry (F6.1 LQ-9/LQ-12). Ordinary requests draw **only** unearmarked lots; a purpose request (e.g. maternity) draws **only** its matching earmark (F6.1 LQ-10). |
| LR-4 | An optional **delegate** (another agent) may be named to cover. |
| LR-5 | A request cannot **overlap** an existing non-rejected leave for the same agent. |
| LR-6 | A request starts `Pending` and notifies the **shift leader** (F6.3). |
| LR-7 | The agent may **withdraw** a request before it reaches final approval (status `Cancelled`). |
| LR-8 | Backdated requests (e.g., sick leave after the fact) are allowed per type policy and flagged. |
| LR-9 | All actions audited. |

## 6. Data model

Creates `LeaveRequest` (see FEATURE §4): `employee_id, leave_type_id, delegate_id (nullable), start_date, end_date, duration_days, status, notes, document_url, issued_at`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Leave request

  Background:
    Given I am the agent "Budi" with 9 annual days remaining

  Scenario: Submit an annual leave within balance
    When I request 3 annual days with a reason
    Then a Pending request is created and my shift leader is notified

  Scenario: Block a request over available balance
    When I request 12 days
    Then it is blocked with "Insufficient leave balance" (no lots are reserved)
    And HR may pre-fund a lot so I can resubmit (F6.1 LQ-11)

  Scenario: Document required
    Given "Sick Leave" requires a document
    When I request sick leave without attaching a document
    Then submission is blocked until I attach one

  Scenario: Name a delegate
    When I request leave and name "Citra" as delegate
    Then the request records Citra as delegate

  Scenario: Prevent overlapping requests
    Given I already have a pending leave 2026-06-10 to 2026-06-12
    When I request leave 2026-06-11 to 2026-06-13
    Then it is blocked as overlapping

  Scenario: Withdraw a pending request
    Given I have a pending request
    When I withdraw it
    Then its status becomes Cancelled
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Backdated sick leave | Allowed per type; flagged; may need document. |
| C-2 | Single-day / half-day | Single-day supported; half-day → see §10. |
| C-3 | Range spanning two lots' expiries | Allocation simply FIFO-spans lots (F6.1 LQ-9); no per-period split — the soonest-expiring lot is drawn first. |
| C-4 | Earmarked (e.g. maternity) request | Draws **only** the matching earmarked lot; if HR hasn't pre-funded one, the request is blocked until they do (F6.1 LQ-10/LQ-11). |
| C-5 | Delegate is unavailable/also on leave | Warn (delegate informational unless coverage enforced — FEATURE §7 Q5). |

## 9. Dependencies

E2 (leave types), F6.1 (balance), F6.3 (approval), E10 (notifications/upload), E1 (audit).

## 10. Decisions & open questions

- ✅ Document mandatory when type requires; over-balance blocked (FIFO reserve across grant-lots, F6.1); withdraw before final (releases the reservation).
- **Open:** half-day leave support in v1?
- **Open:** duration counting rule (calendar vs working/scheduled days) — FEATURE §7 Q1.
