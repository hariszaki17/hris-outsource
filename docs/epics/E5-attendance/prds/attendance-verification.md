# PRD · F5.3 — Shift-Leader Verification (exceptions only)

> **Epic:** E5 Attendance · **Feature:** F5.3 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Clean attendance (on-time, in-geofence, complete) auto-approves, so the shift leader only reviews **exceptions** — late, out-of-geofence, auto-closed, absent, or code-flagged records. This keeps verification workload sane for large 24/7 teams while keeping a human in the loop where it matters (and where client billing may be disputed).

## 2. Goals & non-goals

**Goals**
- An exceptions-only verification queue scoped to the leader's company.
- Approve (→ Verified) or reject (→ Rejected, prompting a correction).
- Escalate to HR when a company has no leader.

**Non-goals**
- Evaluation/routing (F5.2). Editing times — that's a correction (F5.4).

## 3. Actors

Shift Leader (primary), HR/Super Admin (escalation/oversight), System (queue, persist, notify), Agent (notified).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web / mobile** | Shift Leader | Review & decide exception records for their company. |
| **Web console** | HR / Super Admin | Cross-company queue; handle escalations. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| VF-1 | The queue contains only `verification_status=Pending` records (the exceptions per F5.2 EV-5). |
| VF-2 | A leader sees only **their company's** queue (F3.4 scope); HR/Super Admin see all. |
| VF-3 | Each item shows the exception reason(s), times, GPS/geofence result, and scheduled shift for context. |
| VF-4 | **Approve** → `Verified` (+ `verified_by`/`verified_at`); the worked record counts for OT (E7) and billing (E10). |
| VF-5 | **Reject** → `Rejected` with a required reason; prompts/links a correction (F5.4) to fix the underlying data. |
| VF-6 | **Bulk approve** of multiple clean-but-flagged items is allowed (e.g., a batch of slightly-late records). |
| VF-7 | If the company has **no shift leader** (F3.4 SL-7), Pending items route to **HR** for verification. |
| VF-8 | Decisions are audited; the agent is notified of approve/reject. |

## 6. Data model

Updates `Attendance`: `verification_status, verified_by, verified_at`. Reject reason stored (audit + optional field).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Attendance verification (exceptions only)

  Background:
    Given I am the shift leader of "Plaza Senayan"

  Scenario: Only exceptions appear in the queue
    Given Budi has an on-time, in-geofence, complete record
    And Citra has a late record
    When I open the verification queue
    Then I see Citra's late record
    And I do not see Budi's auto-approved record

  Scenario: Approve a late record
    When I approve Citra's late record
    Then its verification_status becomes Verified with my id and timestamp
    And Citra is notified

  Scenario: Reject an out-of-geofence record
    When I reject Budi's out-of-geofence record with a reason
    Then it becomes Rejected
    And a correction is prompted to fix it (F5.4)

  Scenario: Bulk approve
    Given five slightly-late records are pending
    When I select all and bulk approve
    Then all five become Verified

  Scenario: Scope is enforced
    Given a pending record at a company I don't lead
    Then it does not appear in my queue

  Scenario: Escalate when no leader
    Given "Mall X" has no shift leader
    Then its pending records appear in the HR verification queue
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Already-verified record reopened | Only via correction (F5.4) or HR override; not from the queue. |
| C-2 | Stale pending (never verified) | Surfaced/aged in the queue + reporting; optional reminder. |
| C-3 | Absent record | Appears in queue; leader confirms absent or triggers correction if the agent did work. |
| C-4 | Migrated historical record | Imported `Verified` (G-5) — never floods the live queue. |
| C-5 | Leader verifies their own attendance | Disallowed/escalated to HR (separation), confirm. |

## 9. Dependencies

F5.2 (routing), F3.4 (leader scope/escalation), F5.4 (reject→correct), E10 (notifications), E1 (audit), E7/E10 (consume verified records).

## 10. Decisions & open questions

- ✅ Exceptions-only queue; approve/reject; HR escalation when no leader.
- **Open (C-5):** can a leader verify their own exception records, or must those go to HR?
- **Open:** auto-verify SLA — if a Pending record isn't actioned within N days, auto-verify or escalate?
