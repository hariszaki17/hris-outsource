# PRD · F3.4 — Shift-Leader Assignment

> **Epic:** E3 Placement Management · **Feature:** F3.4 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Each **leadership unit** runs its on-site team under **exactly one shift leader** — the person who builds rosters, verifies attendance, and approves leave/overtime there. The unit is set by `ClientCompany.leader_scope` (E2 F2.6): **`company`** = one leader for the whole company (the default, today's behavior); **`site`** = one leader **per active site** (for multi-site clients where each location needs its own on-site supervisor). The shift leader is themselves an agent placed within that unit, granted elevated authority scoped to it. This PRD owns **designating, reassigning, and vacating** that role, and is the gate that turns an ordinary agent into the unit's approver.

## 2. Goals & non-goals

**Goals**
- Assign exactly one shift leader per **leadership unit** — company or site, per `leader_scope` (INV-2).
- Enforce that the leader is an agent **actively placed within that unit** (INV-4) and leads **only that one unit** (INV-3).
- Grant the shift-leader role scoped to the unit on assignment; revoke on vacancy/reassignment.
- Keep an assignment history.

**Non-goals**
- What the shift leader can *do* once assigned (roster, approvals) → lives in E4/E5/E6/E7; this PRD only grants the scope.
- Creating/transferring the agent's placement → F3.1/F3.3.

## 3. Actors

- **HR / Placement Admin** (primary), **Super Admin** — assign/reassign/vacate.
- **System** — validates invariants, grants/revokes role scope, audits, notifies.
- **Shift Leader (incoming/outgoing)**, **Agents at the company** — notified.

## 4. Workflow

```mermaid
flowchart TD
    subgraph HR[HR / Placement Admin]
        D1([Assign shift leader]) --> D2[Select client company]
        D2 --> D3[Pick a candidate agent placed there]
        D3 --> D6[Confirm]
    end
    subgraph SYS[System]
        D6 --> C1{Candidate has an active<br/>placement at THIS company?}
        C1 -- No --> C2[Block: must be placed here] --> D3
        C1 -- Yes --> C3{Candidate already leads<br/>another company?}
        C3 -- Yes --> C4[Block: leader is strictly 1:1] --> D3
        C3 -- No --> C5{Company already<br/>has a leader?}
        C5 -- Yes --> C6[End previous assignment + revoke scope]
        C5 -- No --> C7[Proceed]
        C6 --> C7
        C7 --> C8[Create ShiftLeaderAssignment + grant role scope = company]
        C8 --> C9[(Persist + audit)]
        C9 --> C10[Notify new leader, outgoing leader, agents]
    end
```

## 5. Business rules

| Ref | Rule |
|-----|------|
| SL-0 | The **leadership unit** is the company when `leader_scope=company`, else each **site** (E2 F2.6). All rules below apply per unit; `ShiftLeaderAssignment.site_id` is null for company-scope and set to the site for site-scope. |
| SL-1 | A leadership unit has **at most one active** shift-leader assignment (INV-2) — uniqueness is on `client_company_id` (company-scope) or `(client_company_id, site_id)` (site-scope). |
| SL-2 | The candidate must have an **active placement within that unit** (INV-4) — at the company (company-scope) or **at that site** (site-scope). |
| SL-3 | A person may lead **only one unit at a time** (INV-3) — assigning someone who already leads another company/site is blocked (reassign/vacate the other first). |
| SL-4 | Assigning a new leader where one exists **ends the previous assignment** (`unassigned_at = now`) and revokes its role scope, atomically. |
| SL-5 | Assignment **grants the shift-leader role scoped to the company**; the agent retains their base agent capabilities for their own attendance/leave. |
| SL-6 | When the leader's **placement at the company ends** (terminate/resign/transfer/expire — F3.2/F3.3), their assignment is **auto-vacated** and a vacancy is raised. |
| SL-7 | A company **may temporarily have no leader** (vacancy); approvals that require a leader escalate to HR admin until filled. |
| SL-8 | All assignments/vacancies are audited and notify the incoming leader, outgoing leader, and the company's agents (E10). |
| SL-9 | Assignment history is retained (never hard-deleted). |

## 6. Data model

| Field | Type | Notes |
|-------|------|-------|
| `id` | PK | |
| `client_company_id` | FK | the unit's company; **unique among active** for company-scope (SL-1) |
| `site_id` | FK (nullable) | null for company-scope; the site for site-scope — `(client_company_id, site_id)` **unique among active** (SL-1) |
| `employee_id` | FK | the leader; must have active placement within the unit (SL-2) |
| `assigned_at` | datetime | |
| `unassigned_at` | datetime | null while active |
| `assigned_by` | FK → User | actor |
| `vacated_reason` | enum | `Reassigned` \| `PlacementEnded` \| `Manual` |

> RBAC scope: assignment writes a company-scoped `shift_leader` grant (E1). Revoked on `unassigned_at`.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Shift-leader assignment

  Background:
    Given I am signed in as an HR admin
    And "Plaza Senayan" is an active client company
    And "Budi" has an active placement at "Plaza Senayan"

  Scenario: Assign a shift leader
    When I assign "Budi" as the shift leader of "Plaza Senayan"
    Then "Budi" gains shift-leader access scoped to "Plaza Senayan"
    And "Budi" and the company's agents are notified

  Scenario: Reject a candidate not placed at the company
    Given "Andi" is not placed at "Plaza Senayan"
    When I try to assign "Andi" as its shift leader
    Then the assignment is blocked with "Candidate must be placed at this company"

  Scenario: Enforce one company per leader
    Given "Budi" already leads "Mall Kelapa Gading"
    When I try to assign "Budi" as the leader of "Plaza Senayan"
    Then the assignment is blocked because a shift leader is strictly 1:1 with a company

  Scenario: Reassigning ends the previous leader
    Given "Budi" is the current leader of "Plaza Senayan"
    And "Citra" has an active placement at "Plaza Senayan"
    When I assign "Citra" as the leader of "Plaza Senayan"
    Then "Budi"'s assignment is ended with reason "Reassigned" and his scope is revoked
    And "Citra" gains the shift-leader scope

  Scenario: Leader's placement ending auto-vacates the role
    Given "Budi" leads "Plaza Senayan"
    When his placement at "Plaza Senayan" is terminated
    Then his shift-leader assignment is vacated with reason "PlacementEnded"
    And a vacancy is raised for "Plaza Senayan"

  Scenario: Approvals escalate while a company has no leader
    Given "Plaza Senayan" has no active shift leader
    When an agent there submits a leave request
    Then the approval is routed to an HR admin

  Scenario: Per-site leadership when leader_scope = site
    Given "Plaza Group" has leader_scope = site
    And sites "Plaza Senayan" and "Plaza Indonesia" each have active placements
    When I assign "Budi" (placed at "Plaza Senayan") as leader of site "Plaza Senayan"
    And I assign "Sari" (placed at "Plaza Indonesia") as leader of site "Plaza Indonesia"
    Then each site has exactly one shift leader scoped to that site
    And "Budi" cannot also be assigned to lead "Plaza Indonesia" (strict 1:1 per unit)
```

## 8. Cases & edge cases

| # | Case | Expected behavior |
|---|------|-------------------|
| C-1 | Assign the company's first-ever leader | No previous assignment to end; straightforward grant. |
| C-2 | Candidate's placement is `Scheduled` (not yet active) | Blocked — leader must be *actively* placed (SL-2). |
| C-3 | Outgoing leader is also being transferred (F3.3) | Vacate is idempotent — transfer-vacate and reassign-end converge to one ended record. |
| C-4 | Two HR admins assign different leaders to the same company concurrently | Unique-active constraint (SL-1) makes the second commit fail; retried as a reassignment. |
| C-5 | Company archived while it has a leader | Leader assignment vacated; company no longer accepts placements (F3.1 BR-3). |
| C-6 | Self-assignment (HR admin who is also placed there) | Allowed if invariants hold; audited. |

## 9. Dependencies

- **F3.1** (active placement prerequisite), **F3.2/F3.3** (auto-vacate triggers), **E1** (RBAC scope + audit), **E10** (notifications), and downstream **E5/E6/E7** consume the granted scope.

## 10. Decisions & open questions

- ✅ One leader per **leadership unit**; one unit per leader (strict 1:1). *(2026-06-03: unit = company **or** site per `ClientCompany.leader_scope`; default `company` preserves prior behavior.)*
- ✅ Leader must be actively placed **within the unit** (at the company / at that site).
- ✅ Vacancy allowed; approvals escalate to HR admin meanwhile.
- **Open:** when a company switches `leader_scope` company→site, existing company-level assignment is flagged for re-designation per site (F2.6 C-3) — confirm the transition UX.
- **Open:** can an HR admin act as a **stand-in approver** for a specific company indefinitely, or is escalation only a stop-gap until a leader is named? (assumed: stop-gap.)
