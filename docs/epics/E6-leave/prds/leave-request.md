# PRD · F6.2 — Leave Request (documents, delegate)

> **Epic:** E6 Leave Management · **Feature:** F6.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Agents need to request time off from mobile — choosing a leave type, a date range, optionally naming a delegate to cover, and attaching a document when the type requires it (e.g., sick note). Annual requests must respect the remaining balance up front so agents aren't surprised at approval.

## 2. Goals & non-goals

**Goals**
- Submit a leave request (type, dates, computed duration, reason).
- Enforce document upload for document-required types; capture an optional delegate.
- Pre-check balance for annual types (block over-balance).
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
| LR-3 | For **annual** types, `duration_days` must be ≤ the active quota `remaining`, else **blocked** (INV-1). |
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

  Scenario: Block an annual request over balance
    When I request 12 annual days
    Then it is blocked with "Insufficient leave balance"

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
| C-3 | Range spanning two quota periods | Split/validate against each period — confirm rule. |
| C-4 | Non-annual type (no quota) | No balance check; still follows approval. |
| C-5 | Delegate is unavailable/also on leave | Warn (delegate informational unless coverage enforced — FEATURE §7 Q5). |

## 9. Dependencies

E2 (leave types), F6.1 (balance), F6.3 (approval), E10 (notifications/upload), E1 (audit).

## 10. Decisions & open questions

- ✅ Document mandatory when type requires; annual over-balance blocked; withdraw before final.
- **Open:** half-day leave support in v1?
- **Open:** duration counting rule (calendar vs working/scheduled days) — FEATURE §7 Q1.
