# PRD · F3.1 — Agent Placement (create & activate)

> **Epic:** E3 Placement Management · **Feature:** F3.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

In the legacy system, "placement" was a free-text string on `employee_contracts` — unstructured, unqueryable, and impossible to validate. SWP needs to formally **place an agent at a client company, in a service line, for a contract period**, capturing the terms that govern that engagement. This record becomes the anchor for scheduling, attendance, leave, and overtime. Getting creation right (with the right rules and history) is the foundation of the whole product.

## 2. Goals & non-goals

**Goals**
- HR admin can create a placement in one flow: agent → company → service line → period → terms.
- Enforce the placement invariants (one active placement per agent; valid period; active company).
- Support both **immediate** activation and **scheduled** (future-dated) placements.
- Every placement is auditable and triggers the right notifications.

**Non-goals (this PRD)**
- Lifecycle transitions after creation (renewal/termination/expiry) → F3.2.
- Moving an already-placed agent → F3.3.
- Designating the company's shift leader → F3.4.
- Calculating payroll from `base_salary_ref` → payroll is read-only (E8).

## 3. Actors

- **HR / Placement Admin** (primary) — creates the placement.
- **Super Admin** — same, plus may backdate/override (e.g., for migration corrections).
- **System** — validates, sets status, persists, audits, notifies.
- **Agent**, **Shift Leader** — recipients of the resulting record/notification (read).

## 4. User stories

- **US-1** — *As an HR admin, I want to place an agent at a client company in a service line for a contract period, so that the agent is officially assigned and can be scheduled.*
- **US-2** — *As an HR admin, I want to schedule a placement to start on a future date, so that I can prepare assignments ahead of the agent's start.*
- **US-3** — *As an HR admin, I want the system to stop me from placing an agent who already has an active placement, so that I don't accidentally double-book a person.*
- **US-4** — *As an agent, I want to see my active placement (company, service line, period), so that I know where and when I'm assigned.*
- **US-5** — *As a shift leader, I want newly placed agents to appear in my company roster, so that I can schedule them.*

## 5. Functional requirements & business rules

| Ref | Rule |
|-----|------|
| BR-1 | A placement requires: agent, **employment agreement**, client company, **site** (E2 F2.6 — the specific location), service line, **position** (from master), start date. `end_date` is **optional** — open-ended placements are allowed (typical for `PKWTT`). |
| BR-1b | The placement period must fall **within the agent's employment-agreement validity**. For `PKWT`, if the placement `end_date` would exceed the agreement `end_date`, the system **auto-caps** it to the agreement `end_date` and notifies the creator; `PKWTT` imposes no upper bound. |
| BR-2 | **INV-1 + 1-day buffer** — the agent must have no `Active`/`Scheduled` placement overlapping the new period, AND the new `start_date` must be **at least 1 day after** any prior placement's `end_date` (no overlap, no same-day handover). Enforced at persist time (DB constraint), not just UI. |
| BR-3 | The client company must be `Active`. Placing into an inactive/archived company is blocked. |
| BR-3b | The **site** must belong to the chosen company and be `Active` (E2 F2.6 ST-4). The site defaults to the company's **primary "Main Site"** and can be changed to any other active site. Its geofence (or absence) drives E5 clock-in (CI-2). |
| BR-4 | When `end_date` is present it must be **after** `start_date`. |
| BR-5 | If `start_date <= today` → status `Active`. If `start_date > today` → status `Scheduled` (system auto-activates on the date — F3.2). |
| BR-6 | Backdating `start_date` is allowed for **HR admin and Super Admin**, requires a **reason**, and is recorded in the audit log. |
| BR-7 | On successful creation, write an audit-log entry and notify the agent and the company's shift leader (if one is assigned). |
| BR-8 | Creation is not blocked if the company has no shift leader yet, but the UI surfaces a warning prompting F3.4. |
| BR-9 | `position` is selected per placement from the E2 position master; the same agent may hold a **different position** at a different company. |

## 6. Data model (created fields)

| Field | Type | Required | Validation |
|-------|------|----------|------------|
| `employee_id` | FK → Employee | yes | employee exists & status = active |
| `employment_agreement_id` | FK → EmploymentAgreement | yes | belongs to the same agent; placement period ⊆ agreement validity (BR-1b) |
| `client_company_id` | FK → ClientCompany | yes | company status = active (BR-3) |
| `site_id` | FK → Site (E2 F2.6) | yes | belongs to `client_company_id` & status = active (BR-3b); defaults to the company's primary Main Site |
| `service_line_id` | FK → ServiceLine | yes | one of Facility / Building Mgmt / Parking |
| `position_id` | FK → Position (E2 master) | yes | per-placement; may differ across companies (BR-9) |
| `start_date` | date | yes | valid date; backdating needs reason (BR-6) |
| `end_date` | date | no | open-ended allowed; if present `> start_date` (BR-4) and within agreement (BR-1b) |
| `annual_leave_entitlement` | int (days) | no | `>= 0` |
| `base_salary_ref` | money | no | `>= 0`; reference only (payroll read-only, E8) |
| `predecessor_id` | FK → Placement | system | null on plain create; set by renewal/transfer (F3.2/F3.3) |
| `backdate_reason` | text | conditional | required when `start_date < today` (BR-6) |
| `notes` | text | no | — |
| `status` | enum | system | set by BR-5 |
| `created_by` | FK → User | system | actor id |

> `pkwt_reference` and the PKWT/PKWTT type now live on **EmploymentAgreement** (E2), not on the placement — see [DATA-MAPPING.md](../DATA-MAPPING.md).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Agent placement creation

  Background:
    Given I am signed in as an HR admin
    And an active agent "Budi" who has no active or scheduled placement
    And an active client company "Plaza Senayan"
    And the service line "Parking" exists

  Scenario: Create an immediately-active placement
    When I create a placement for "Budi" at "Plaza Senayan" in "Parking"
    And I set start date to today and a valid PKWT period
    Then the placement is created with status "Active"
    And an audit-log entry records the creation
    And "Budi" can see the active placement
    And the placement appears in the "Plaza Senayan" roster

  Scenario: Place an agent at a specific site of a multi-site company
    Given "Plaza Group" has sites "Main Site", "Plaza Senayan", and "Plaza Indonesia"
    When I create a placement for "Budi" and select the site "Plaza Senayan"
    Then the placement is created with site = "Plaza Senayan"
    And E5 clock-in for "Budi" validates against "Plaza Senayan"'s geofence

  Scenario: Site defaults to the company's primary Main Site
    Given the single-location company "Mall Kelapa Gading" has only its "Main Site"
    When I create a placement for "Budi" at "Mall Kelapa Gading" without choosing a site
    Then the placement is created with site = "Main Site"

  Scenario: Create a future-dated placement
    When I create a placement for "Budi" with a start date 14 days from today
    Then the placement is created with status "Scheduled"
    And it is not yet shown as active to "Budi"

  Scenario: Block double-booking an already-placed agent
    Given "Budi" already has an active placement at "Mall Kelapa Gading"
    When I try to place "Budi" at "Plaza Senayan" with an overlapping period
    Then creation is blocked with the message "Agent already has an active placement"
    And I am offered to end or transfer the existing placement

  Scenario: Reject an end date before the start date
    When I create a placement with end date earlier than start date
    Then creation is blocked with a validation error on the end date

  Scenario: Block placement into an inactive company
    Given the company "Old Tower" is archived
    When I try to place "Budi" at "Old Tower"
    Then creation is blocked with the message "Company is not active"

  Scenario: Warn when the company has no shift leader
    Given "Plaza Senayan" has no shift leader assigned
    When I create a placement for "Budi" at "Plaza Senayan"
    Then the placement is created successfully
    And I see a warning prompting me to assign a shift leader

  Scenario: Block a same-day handover (1-day buffer)
    Given "Budi" had a placement at "Mall Kelapa Gading" that ends on 2026-06-30
    When I create a new placement for "Budi" starting on 2026-06-30
    Then creation is blocked with the message "No overlap or same-day handover — start the day after the prior placement ends"
    And the earliest allowed start date is 2026-07-01

  Scenario: Create an open-ended placement for a permanent (PKWTT) agent
    Given "Budi" has a PKWTT employment agreement with no end date
    When I create a placement for "Budi" with a start date and no end date
    Then the placement is created successfully with an open-ended period

  Scenario: Auto-cap a placement to the PKWT agreement end
    Given "Budi" has a PKWT employment agreement ending 2026-12-31
    When I create a placement with an end date of 2027-03-31
    Then the placement is created with its end date auto-capped to 2026-12-31
    And I am notified that the end date was adjusted to the agreement end

  Scenario: HR admin backdates with a reason
    When I create a placement with a start date in the past and provide a reason
    Then the placement is created
    And the audit log records the backdating reason
```

## 8. Cases & edge cases

| # | Case | Expected behavior |
|---|------|-------------------|
| C-1 | New start is the day after the prior end (or later) | Allowed — no overlap, no same-day handover. |
| C-2 | New placement starts the same day as (or before) a previous one ends | **Blocked** — minimum 1-day buffer required (BR-2). Earliest valid start is the day after the prior `end_date`. |
| C-3 | Backdated start_date by Super Admin | Allowed with warning; audit notes backdating. |
| C-4 | Open-ended (no end_date) non-PKWT placement | Allowed; status follows BR-5; never auto-expires. |
| C-5 | Agent currently serving as shift leader at company A is placed at company B | Blocked by INV-1 (active placement at A); must transfer first (F3.3), which also vacates the leader role (F3.4). |
| C-6 | Two HR admins create overlapping placements for the same agent concurrently | Second commit fails the overlap check at persist time (DB constraint), not just UI — last writer gets BR-2 error. |
| C-7 | Company has no shift leader at creation | Created + warning (BR-8); notification to leader is skipped. |
| C-8 | "Today" across timezones | Use the org's configured timezone (Asia/Jakarta) to evaluate BR-5, not server UTC. |
| C-9 | Agent record is inactive/resigned | Blocked — only active employees can be placed. |
| C-10 | start_date far in the future (e.g. > 1 year) | Allowed but warns (likely data-entry error). |

## 9. Dependencies

- **E2** — Employee, ClientCompany, ServiceLine master data must exist.
- **E1** — audit log + RBAC (only HR admin / super admin may create).
- **E10** — notifications (agent / shift leader).
- **F3.4** — shift-leader assignment (referenced for notification + warning).

## 10. Decisions & open questions

**Resolved (2026-05-29):** C-2 → 1-day buffer, no same-day handover (BR-2). Position → master-data controlled, set per placement (BR-9). Open-ended placements valid, esp. PKWTT (BR-1). Backdating → HR admin + reason (BR-6).

**Resolved (2026-05-29, round 2):** Service line → **manual classification later** (after SWP confirms) → [DATA-MAPPING.md](../DATA-MAPPING.md) G-1. Buffer → **next day after prior end** is sufficient. PKWT overrun → **auto-cap** to agreement end (BR-1b). Designation window → defaults to the employment-agreement dates, adjustable per placement.

_No open questions remain for this feature._
