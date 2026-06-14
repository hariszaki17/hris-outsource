# PRD · F6.3 — Leave Approval (via the E11 engine)

> **Epic:** E6 Leave Management · **Feature:** F6.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_
> **Reworked 2026-06-14 (EPICS §8 E11):** leave no longer runs its own `shift_leader → HR/lead` state machine. It **routes through the configurable E11 approval engine** (per-company chain). This PRD now specifies only leave's **on-approval side-effects** (the `OnApproved`/`OnRejected` hooks) and what leave contributes to the chain; **routing, lines, self-block, reject, bypass, and the decision trail are owned by [E11](../../E11-approvals/FEATURE.md)**.

---

## 1. Context & problem

A leave request needs sign-off before it consumes balance and changes the schedule. Routing **who** signs off (and in what order) is now the job of the **per-company approval template** (E11 F11.1) — typically the on-site shift leader, then HR/account-manager, optionally a super-admin sign-off. Leave's remaining responsibility is to **react** to the engine's terminal decision: commit the per-type quota reservation (F6.1) and trigger the schedule/attendance integration (F6.4) on approval, and release the reservation on reject.

## 2. Goals & non-goals

**Goals**
- On request submit, **create an E11 `ApprovalInstance`** (`request_type = LEAVE`) for the agent's company; reserve `pending_days` (F6.1 LQ-2).
- Register the leave **`OnApproved`** hook: re-check remaining at finalize, **commit** `pending_days → used_days` (F6.1), trigger integration (F6.4).
- Register the leave **`OnRejected`/cancel** hook: **release** the reservation (no quota consumed).

**Non-goals**
- Routing / lines / OR-membership / sequential advance / self-block / super-admin bypass / decision trail — **all E11** (F11.2). Creating requests (F6.2). Quota mechanics (F6.1). Schedule changes (F6.4 does the work).

## 3. Actors

Line members (per the company's E11 template — e.g. shift leader, lead, HR), Super Admin (E11 bypass), System (create instance, run hooks, commit/release quota, integrate, notify), Agent (requester; notified).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web / mobile — Kotak Masuk** | Line members | Approve/reject the current line of a leave instance (E11 F11.3 inbox). |
| **Web console — Cuti → Approvals** | Line members | Same instances via the per-domain tab (view over the E11 source, IB-5). |
| **Web / mobile** | Agent | Submit (F6.2); watch the chain-progress timeline (E11). |

## 5. Business rules

| Ref | Rule |
|-----|------|
| LA-1 | **Routing is E11.** On submit, leave creates an `ApprovalInstance` (`request_type = LEAVE`, `request_id`, `company_id` = the agent's company). The chain (lines, members, order), OR-within-line, sequential advance, self-block (INV-3), terminal reject (INV-4), and super-admin bypass (INV-5) are **owned by E11 F11.2**. *(Supersedes the old two-level `shift_leader → HR/lead` LA-1/LA-2/LA-3.)* |
| LA-2 | **Reserve at submit.** Creating the request reserves `pending_days` on the type's window quota (F6.1 LQ-2) for quota-bearing types (`PER_EVENT`/`UNCAPPED` reserve nothing). |
| LA-3 | **No template → super-admin fallback** (E11 INV-7) — the request still routes (never auto-approves, never blocks). *(Replaces the old "no leader → HR sole approver" escalation.)* |
| LA-4 | **`OnApproved` hook (terminal approve/bypass).** The engine fires leave's hook in its transaction (E11 INV-8): **re-check** the type's remaining (LA-5), then **commit** the reservation (F6.1 LQ-2 — `pending_days → used_days`; `PER_EVENT`/`UNCAPPED` nothing to commit), then trigger schedule/attendance integration (F6.4). |
| LA-5 | **Remaining re-checked at finalize.** The window may have been consumed/rolled since submit (F6.1 C-5). If now insufficient, the hook **fails the transaction** (E11 EX-9) → the instance stays at its line, flagged; HR adjusts the quota (F6.1 LQ-6) or a super admin bypasses. |
| LA-6 | **`OnRejected` / cancel hook.** On terminal reject (E11 INV-4) or withdrawal before finalize, **release** the reservation (no quota consumed). |
| LA-7 | **Decision trail = `approval_actions`** (E11 INV-9) — one append-only row per approve/reject/bypass (actor, decision, reason, time), audited; agent notified at each step (E10). *(Replaces the `leave_approvals` table.)* |

## 6. Data model

No leave-owned approval table. The engine owns `ApprovalInstance` + `ApprovalAction` (E11). Leave keeps `LeaveRequest.status` in sync with the instance (`PENDING` while routing; `APPROVED`/`REJECTED`/`CANCELLED` on terminal) and owns the quota rows (F6.1). The old `LeaveApproval` entity is **removed** (superseded by `approval_actions`).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Leave approval via the E11 engine

  Background:
    Given Budi has a Pending 3-day annual leave request at "Plaza Senayan"
    And Plaza Senayan's approval template is line 1 [Sari], line 2 [HR]
    And submitting reserved 3 pending_days on Budi's annual quota

  Scenario: Chain approves, side-effects fire
    When line 1 [Sari] approves and line 2 [HR] approves (E11)
    Then the leave OnApproved hook re-checks remaining, commits 3 days (pending → used), and fires schedule integration
    And the request status becomes APPROVED

  Scenario: Reject releases the reservation
    When a current-line member rejects with a reason (E11)
    Then the request becomes REJECTED, the 3 pending_days are released (no quota consumed), and Budi is notified

  Scenario: No template falls back to super admin
    Given Budi's company has no approval template
    When Budi submits leave
    Then it routes to the E11 super-admin fallback line (never auto-approved)

  Scenario: Balance re-check fails at finalize
    Given Budi's annual remaining dropped below 3 since submission
    When the final line approves
    Then the OnApproved hook fails the transaction; the instance stays pending and is flagged (HR adjusts the quota or super admin bypasses)

  Scenario: Cannot self-approve (E11 INV-3)
    Given a line member files their own leave and is on its chain
    Then they cannot clear that line; another member must (or super-admin bypass)
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent withdraws while pending mid-chain | Allowed before finalize → Cancelled; reservation released (LA-6); instance closed (E11 C-1). |
| C-2 | `PER_EVENT`/`UNCAPPED` request (bereavement, sick) | Same routing; nothing to commit — only the per-occurrence cap / document gate at submit (F6.1 LQ-13). |
| C-3 | Window rolled over between submit and finalize | Re-check at finalize (LA-5); if insufficient, hook blocks (E11 EX-9). |
| C-4 | Super-admin bypass | Force-approve (E11 INV-5); the `OnApproved` hook still runs (commit + integrate). |
| C-5 | Template edited while the request is mid-chain | Instance resets to line 1 on the new chain (E11 INV-6); the reservation is untouched (still pending). |

## 9. Dependencies

**E11** (approval engine — routing, instance, actions, bypass, fallback, reset), F6.2 (request + reserve), F6.1 (quota commit/re-check), F6.4 (integration), E10 (notifications), E1 (audit).

## 10. Decisions & open questions

- ✅ **Leave routes through the E11 engine** (per-company configurable chain); leave owns only the `OnApproved`/`OnRejected` side-effects (2026-06-14). Supersedes the two-level `shift_leader → HR/lead` model (LA-1/LA-2/LA-3) and the HR-override (now super-admin bypass).
- ✅ Reserve at submit; commit at terminal approve with re-check; release on reject/cancel.
- **Open:** SLA/auto-escalation if a line sits un-actioned (tracked in E11, v1: none).
