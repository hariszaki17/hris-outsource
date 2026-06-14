# PRD · F11.2 — Approval Execution Engine

> **Epic:** E11 Approvals · **Feature:** F11.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

When a domain request is submitted (leave F6.2, overtime F7.2), it must be **routed** through the submitting agent's company approval chain, advanced line-by-line as members act, and **finalized** — firing the request type's side-effects on approval (leave: commit the per-type quota reservation + schedule integration; overtime: count hours by day-type). The engine is **request-type-agnostic**: it owns the instance lifecycle, the decision trail, self-approval blocking, and super-admin bypass; each request type contributes only an `OnApproved`/`OnRejected` **hook**.

## 2. Goals & non-goals

**Goals**
- Build an `ApprovalInstance` from the company template (or super-admin fallback) at request submission.
- Advance lines on `APPROVE` (OR within a line), block self-approval, terminate on `REJECT`, allow super-admin `BYPASS`.
- Fire the per-type side-effect hook **exactly once** on terminal approval (and `OnRejected` on reject), in the engine's transaction.
- Re-base non-terminal instances when the company template changes (INV-6).

**Non-goals**
- Defining templates (F11.1). The inbox UI (F11.3). The domain side-effects themselves (E6/E7 own those — the engine only calls the hook).

## 3. Actors

Requester (submits; cannot self-approve), Line member (approves/rejects the current line), Super Admin (bypass), System (route, advance, fire hooks, audit, notify).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web / mobile** | Line member | Approve/reject the current line of an instance (via Inbox F11.3 or the domain approval tab). |
| **Web console** | Super Admin | `BYPASS` a pending instance (force-approve, reason required). |
| **Web / mobile** | Requester | View chain-progress timeline; withdraw the underlying request (domain-owned) while non-terminal. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| EX-1 | On domain-request submit, the engine creates an `ApprovalInstance` (`request_type`, `request_id`, `company_id`, `requester_id`). If the company has a template → `template_id` set, `template_version = template.version`, `current_line = 1`, `status = PENDING`. If not → super-admin **fallback** (`template_id = null`, single implicit super-admin line) (INV-7). |
| EX-2 | The **current line** = the line at `current_line`. An eligible approver is a **member of that line** (or any super admin, EX-6). Routing is by **membership**, not by a `*.approve` permission. |
| EX-3 | **OR within a line:** the first `APPROVE` by an eligible member clears the line. If more lines remain → `current_line++`, notify the next line. If it was the **last** configured line → `status = APPROVED` (EX-7). Each approve appends an `APPROVE` action (INV-9). |
| EX-4 | **No self-approval (INV-3):** if the requester is a member of the current line, they cannot approve it; another member must. If they are the **sole** member, the line clears only by super-admin bypass (EX-6). |
| EX-5 | **Reject is terminal (INV-4):** any current-line member's `REJECT` (reason required) → `status = REJECTED`; fire `OnRejected`; notify requester. |
| EX-6 | **Super-admin bypass (INV-5):** a super admin (`approvals.bypass`) may force-approve any non-terminal instance from any state, skipping remaining lines even if not a member; reason required; appends a `BYPASS` action; finalizes as approved (EX-7). |
| EX-7 | **On finalize-approved** (last line cleared or bypass): set `status = APPROVED` and fire the registered `OnApproved(ctx, tx, request_id)` hook for `request_type` **in the same transaction** (INV-8); notify requester. The hook owns the domain side-effects (leave commit/integration F6.1/F6.4; OT day-type count F7.4). |
| EX-8 | **Live template + reset (INV-6):** when F11.1 bumps a company's template version, every non-terminal instance for that company resets to `current_line = 1` and the new `template_version`; prior actions retained as audit, not counted; new line-1 members notified. |
| EX-9 | **Re-check at finalize:** before firing `OnApproved`, the hook may signal a domain block (e.g. leave LA-5 insufficient remaining at final approval); if the hook errors, the transaction rolls back and the instance stays at its current line (flagged). |
| EX-10 | Every action (approve/reject/bypass) is **append-only** (`approval_actions`, INV-9), stamped with the `template_version` in force, and audited (E1). |
| EX-11 | An instance is created/advanced **idempotently** — re-posting the same approve with an `Idempotency-Key` returns the same result; a second member acting on an already-cleared line is a no-op (`409 LINE_ALREADY_CLEARED`). |

## 6. Data model

`ApprovalInstance`: `id (SWP-APV-*), request_type (LEAVE|OVERTIME|…), request_id, company_id, template_id (nullable), template_version, current_line, status (PENDING|APPROVED|REJECTED), requester_id, created_at, updated_at`.
`ApprovalAction`: `id (SWP-APA-*), instance_id (FK), line_no, template_version, actor_user_id, action (APPROVE|REJECT|BYPASS), reason (required for REJECT/BYPASS), created_at`. Append-only.

**Hook registry (server):** `request_type → { OnApproved(ctx, tx, request_id), OnRejected(ctx, tx, request_id) }`. Leave + overtime register at startup.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Approval execution engine

  Background:
    Given "Plaza Senayan" has a template: line 1 [Rudi, Sari], line 2 [Sari Hadi]
    And Budi (an agent at Plaza Senayan) submits a 3-day leave request

  Scenario: Sequential OR advance to final approval
    When Rudi approves line 1
    Then the instance advances to line 2 (Rudi cleared line 1; Sari need not act)
    When Sari Hadi approves line 2
    Then status becomes APPROVED and the leave OnApproved hook fires (quota committed, schedule integrated)

  Scenario: Reject is terminal
    When Rudi rejects line 1 with a reason
    Then status becomes REJECTED, the leave OnRejected hook fires, and Budi is notified

  Scenario: No self-approval
    Given Rudi is a member of line 1 and Rudi submitted this request
    Then Rudi cannot approve line 1; Sari must act
    And if Rudi were the only line-1 member, only a super-admin bypass could clear it

  Scenario: Super-admin bypass
    Given the instance is pending at line 1
    When a super admin bypasses it with a reason
    Then status becomes APPROVED (remaining lines skipped) and the OnApproved hook fires

  Scenario: No template falls back to super admin
    Given the company "Baru Jaya" has no template
    When an agent there submits overtime
    Then the instance routes to the super-admin fallback line (never auto-approved, never blocked)

  Scenario: Template edit re-bases a pending instance
    Given Budi's instance is at line 2
    When HR edits the template and saves
    Then Budi's instance resets to line 1 on the new version and new line-1 members are notified

  Scenario: Second approver on a cleared line
    Given Rudi already cleared line 1
    When Sari also tries to approve line 1
    Then it is a no-op (409 LINE_ALREADY_CLEARED)
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Underlying request withdrawn (domain) while pending | Domain cancels the request and the engine marks the instance cancelled/closed; no hook fires. |
| C-2 | `OnApproved` hook errors (e.g. leave LA-5 insufficient remaining) | Transaction rolls back; instance stays at current line, flagged; HR adjusts (F6.1) or super admin bypass. |
| C-3 | Member offboarded while on the current line | Line still clears if it has another active member; otherwise super-admin bypass / HR re-edit (INV-6/J). |
| C-4 | Concurrent approves by two line-1 members | First commits and advances; second → `409 LINE_ALREADY_CLEARED` (EX-11). |
| C-5 | Bypass on an already-terminal instance | `409` (already APPROVED/REJECTED). |
| C-6 | Request type with no registered hook | Engine still routes/finalizes but logs a missing-hook warning; finalize has no side-effect (config error). |

## 9. Dependencies

F11.1 (template + reset), E6/E7 (hook registrations + domain side-effects), E1 (RBAC `approvals.bypass`, audit, idempotency), E10 (notifications: line advanced, decided).

## 10. Decisions & open questions

- ✅ Membership-routed, OR-within-line, sequential, self-blocked, terminal-reject, super-admin bypass, fallback, live-reset, per-type hook (2026-06-14, EPICS §8 E11).
- ✅ Side-effects fire **in the engine transaction**, exactly once on terminal approval (INV-8).
- **Open:** SLA/auto-escalation timer (v1: none).
- **Open (C-2):** surface the hook-block as a distinct instance sub-state vs a generic flag (v1: flag + audit).
