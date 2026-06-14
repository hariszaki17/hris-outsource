# PRD · F6.2 — Leave Request (documents, delegate)

> **Epic:** E6 Leave Management · **Feature:** F6.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Agents need to request time off from mobile — choosing a leave type, a date range, optionally naming a delegate to cover, and attaching a document when the type requires it (e.g., sick note). Requests must respect the **type's own cap** up front (its `cap_basis` window, F6.1) so agents aren't surprised at approval. `leave_type` is the **cap axis**: each type meters in its own window (annual pool, per-event, per-month, per-year-count, lifetime-once, uncapped) and statutory/sick/religious leave **never depletes the annual pool**.

## 2. Goals & non-goals

**Goals**
- Submit a leave request (type, dates, computed duration, reason).
- Enforce document upload for document-required types; capture an optional delegate.
- Pre-check the **type's cap in its window** (F6.1) and **reserve** it (`pending_days`); block over-cap.
- Enforce **eligibility gates** (gender, advance-notice, min-service, lifetime-once) at submit.
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
| LR-2 | If the leave type `requires_document` (E2), a **document upload is mandatory** before submit (INV-5). |
| LR-3 | The request must fit the type's **`cap_basis` window** (F6.1 LQ-13): quota-bearing types need `duration_days` (or 1 occurrence for `PER_YEAR_COUNT`) ≤ the window's `remaining` — else **blocked** (INV-1); the system **reserves** `pending_days` on that window (F6.1 LQ-12). `PER_EVENT` types need `duration_days ≤ cap_value`; `UNCAPPED` types impose no day cap (document gate only). A request **never** charges another type's entitlement. |
| LR-3b | **Eligibility gates (INV-7, F6.1 LQ-15)** checked at submit: `gender` match, `start_date − today ≥ notice_days`, tenure `≥ min_service_years`, and no prior approved request for `LIFETIME_ONCE`/`SERVICE_UNPAID` types. The failing gate is the block reason. |
| LR-4 | An optional **delegate** (another agent) may be named to cover. |
| LR-5 | A request cannot **overlap** an existing non-rejected leave for the same agent. |
| LR-6 | A request starts `Pending` and notifies the **shift leader** (F6.3). |
| LR-7 | The agent may **withdraw** a request before it reaches final approval (status `Cancelled`). |
| LR-8 | Backdated requests (e.g., sick leave after the fact) are allowed per type policy and flagged. |
| LR-9 | All actions audited. |

## 6. Data model

Creates `LeaveRequest` (see FEATURE §4): `employee_id, leave_type_id, quota_id (nullable — set for quota-bearing types), delegate_id (nullable), start_date, end_date, duration_days, status, notes, document_url, issued_at`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Leave request

  Background:
    Given I am the agent "Budi" with 9 days remaining on my CT annual quota

  Scenario: Submit an annual leave within the annual cap
    When I request 3 CT (annual) days with a reason
    Then a Pending request is created and my shift leader is notified

  Scenario: Block an annual request over the annual cap
    When I request 12 CT days
    Then it is blocked with "Insufficient leave balance" (nothing is reserved)
    And HR may adjust my CT quota so I can resubmit (F6.1 LQ-6)

  Scenario: Statutory leave does not consume the annual quota
    When I take 2 approved CKM (bereavement) days
    Then my CT annual remaining is still 9

  Scenario: Block a per-event request over its occurrence cap
    When I request 3 CKM days (cap 2 per occurrence)
    Then it is blocked as over the per-occurrence cap

  Scenario: Eligibility gate blocks a mismatched request
    When a male agent requests CH (menstrual, gender FEMALE)
    Then it is blocked with a gender-eligibility reason

  Scenario: Document required
    Given "SDSKD" (sick with letter) requires a document
    When I request it without attaching a document
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
| C-1 | Backdated sick leave | Allowed per type; flagged; may need document; `notice_days` waived for backdated sick. |
| C-2 | Single-day / half-day | Single-day supported; half-day → see §10. |
| C-3 | `PER_MONTH` request spanning a month boundary | Blocked in v1 — must fall within one window (F6.1 C-2). |
| C-4 | `LIFETIME_ONCE` already used | Blocked as already used (F6.1 LQ-15); HR override only via quota adjust + reason. |
| C-5 | Delegate is unavailable/also on leave | Warn (delegate informational unless coverage enforced — FEATURE §7 Q5). |

## 9. Dependencies

E2 (leave types), F6.1 (balance), F6.3 (approval), E10 (notifications/upload), E1 (audit).

## 10. Decisions & open questions

- ✅ Document mandatory when type requires; over-cap blocked (reserve on the type's window quota, F6.1); eligibility gates (gender/notice/service/lifetime) at submit; withdraw before final (releases the reservation).
- **Open:** half-day leave support in v1?
- **Open:** duration counting rule (calendar vs working/scheduled days) — FEATURE §7 Q1.
