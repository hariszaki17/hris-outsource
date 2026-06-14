# PRD · F11.3 — Approval Inbox (web + mobile)

> **Epic:** E11 Approvals · **Feature:** F11.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

An approver's daily question is "**what needs my decision right now?**" (NAVIGATION-AND-RBAC §2). With routing now by **line membership** (F11.2), the inbox must surface every non-terminal instance whose **current** line includes the viewer — across request types — and let them approve/reject in place. On-site approvers (shift leaders, leads) are phone-first, so the inbox has **web + mobile** parity. The inbox is a **view** over the same instances the per-domain approval tabs show (single source of truth), not a second queue.

## 2. Goals & non-goals

**Goals**
- One aggregated "needs my decision" queue: current-line instances the viewer can act on, excluding their own requests (INV-3).
- Approve/reject from the list or the request-detail chain timeline; reason required on reject.
- Web + mobile parity for on-site approvers.

**Non-goals**
- Defining templates (F11.1). Engine mechanics (F11.2). Notification delivery (E10). The super-admin bypass surface (F11.2, a super-admin action, not the line-member inbox).

## 3. Actors

Line member (HR, lead, shift leader, super admin — any current-line member), System (filter by membership + scope, render chain progress).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console — Kotak Masuk** | Line members | Aggregated current-line queue; open detail; approve/reject. Mirrors the per-domain approval tabs. |
| **Mobile app — Approvals/Inbox** | On-site line members (shift leader, lead) | Same queue, phone-first; approve/reject the current line. |
| **Web / mobile — request detail** | Requester + approvers | Chain-progress timeline: each line, its members, who acted (approve/reject/bypass), and the pending line. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| IB-1 | The inbox lists non-terminal `ApprovalInstance`s where the viewer is a **member of `current_line`** and is **not** the requester (INV-3), filtered by the viewer's data scope (company membership, server-enforced). |
| IB-2 | Each item shows: request type + summary, requester, company, current line position (e.g. "Line 1 of 2"), and submitted-at. Grouped/filterable by request type. |
| IB-3 | Approve/reject act on the **current line** via F11.2 (EX-3/EX-5); reject requires a reason. A cleared/advanced line drops out of the viewer's inbox (it is a no-op if re-acted, EX-11). |
| IB-4 | The **request-detail chain timeline** renders every line, its members, each recorded action (actor, decision, reason, time) from `approval_actions`, and the current pending line — including `BYPASS` rows. |
| IB-5 | The inbox is a **view** over the same instances as the per-domain approval tabs (Cuti → Approvals, Lembur → Approvals); both read one source (no divergence). |
| IB-6 | Inbox visibility (nav) is gated by `approvals.act`; **acting** is still gated by line membership server-side (defense-in-depth, ENGINEERING C1). |
| IB-7 | After a template reset (INV-6), the inbox reflects the new line-1 membership on the next load. |

## 6. Data model

Reads `ApprovalInstance` + `ApprovalAction` (F11.2) joined to the domain request (E6/E7) for the summary. No new entities.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Approval inbox

  Background:
    Given I am Rudi, a member of line 1 for "Plaza Senayan"
    And Budi's leave request is pending at line 1

  Scenario: See and act on a current-line item
    When I open Kotak Masuk
    Then I see Budi's leave request showing "Line 1 of 2"
    When I approve it
    Then it advances to line 2 and leaves my inbox

  Scenario: Reject from the inbox
    When I reject Budi's request with a reason
    Then it becomes REJECTED and leaves my inbox

  Scenario: My own request is not in my inbox
    Given I (Rudi) submitted my own overtime and I am on its line 1
    Then it does not appear in my inbox to self-approve (INV-3)

  Scenario: Chain timeline on detail
    When I open the request detail
    Then I see line 1 (members + who acted) and line 2 (pending), in order

  Scenario: Item advanced by another member
    Given Sari approved line 1 before me
    When I refresh my inbox
    Then Budi's request is gone from my line-1 queue (now at line 2)

  Scenario: Mobile parity
    Given I am a shift leader on the mobile app
    Then I see the same current-line queue and can approve/reject on my phone
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Viewer is on a **later** line (not current) | Item does **not** show until the chain advances to that line. |
| C-2 | Viewer on line 1 **and** line 2 | Item shows while line 1 is current; reappears at line 2 after line 1 clears (still cannot self-approve, INV-3). |
| C-3 | Empty inbox | `comp/EmptyInbox`-style empty state (no dead flow). |
| C-4 | Super-admin viewing the inbox | Sees items where they're a current-line member; **bypass** is a separate super-admin action (F11.2), not the line-member approve. |
| C-5 | Instance reset by a template edit while open | On refresh the item reflects the new line-1 membership (IB-7). |

## 9. Dependencies

F11.2 (instances, actions, act endpoints), E10 (Inbox shell `apps/web/.../inbox-screen`, notifications), E1 (RBAC `approvals.act`, scope), E6/E7 (request summaries).

## 10. Decisions & open questions

- ✅ Membership-filtered aggregated inbox; web + mobile parity; view over single source (2026-06-14, EPICS §8 E11).
- ✅ Nav gated by `approvals.act`; acting gated by membership server-side.
- **Open:** dedicated paginated full-queue endpoint vs reuse of the dashboard panel (mirrors NAVIGATION-AND-RBAC §7 open item; v1 may reuse).
- **Open:** inbox live counts / badges (follow-up with E10).
