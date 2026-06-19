# PRD · F3.1 — Agent Placement (create & activate)

> **Epic:** E3 Placement Management · **Feature:** F3.1 · **Status:** Draft v2
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

In the legacy system, "placement" was a free-text string on `employee_contracts` — unstructured, unqueryable, and impossible to validate. SWP needs to formally **place an agent at a client company (site)**, capturing the terms that govern that engagement. This record becomes the anchor for scheduling, attendance, leave, and overtime. Placement is a **"currently active at company X"** record — `start_date` is always today, `end_date` is always nil (open-ended); no Scheduled/future-dated state, no backdating, no period validation. The agent's **position** is read from `Employee.position` (E2 F2.1), not entered on the placement. *(Resolved 2026-06-19.)*

## 2. Goals & non-goals

**Goals**
- HR admin can create a placement in one flow: agent → company → site.
- Enforce the placement invariants (one active placement per agent; active company).
- Placement is always immediate activation (`Active` status, start_date = today).
- Every placement is auditable and triggers the right notifications.

**Non-goals (this PRD)**
- Lifecycle transitions after creation (end/renewal/termination) → F3.2.
- Moving an already-placed agent → F3.3.
- Designating the company's shift leader → F3.4.
- Position management (lives on Employee, E2 F2.1). Compensation terms (lives on EmploymentAgreement, E2 F2.2).

## 3. Actors

- **HR / Placement Admin** (primary) — creates the placement.
- **Super Admin** — same, plus may backdate/override (e.g., for migration corrections).
- **System** — validates, sets status, persists, audits, notifies.
- **Agent**, **Shift Leader** — recipients of the resulting record/notification (read).

## 4. User stories

- **US-1** — *As an HR admin, I want to place an agent at a client company (site), so that the agent is officially assigned and can be scheduled.*
- **US-2** — *As an HR admin, I want the system to stop me from placing an agent who already has an active placement, so that I don't accidentally double-book a person.*
- **US-3** — *As an agent, I want to see my active placement (company, site, position), so that I know where I'm assigned.*
- **US-4** — *As a shift leader, I want newly placed agents to appear in my company roster, so that I can schedule them.*

## 5. Functional requirements & business rules

| Ref | Rule |
|-----|------|
| BR-1 | A placement requires: agent, client company, **site** (E2 F2.6 — the specific location). `start_date` is always today (immediate), `end_date` is always nil (open-ended). The agent's **position** is read from `Employee.position` (E2 F2.1) and displayed read-only on the create form — it is not entered on the placement. *(Resolved 2026-06-19 — placement is decoupled from time; position on Employee.)* |
| BR-2 | **INV-1** — the agent must have no `Active` placement. If the agent is already placed, the existing placement must be ended or transferred first (F3.2/F3.3). |
| BR-3 | The client company must be `Active`. Placing into an inactive/archived company is blocked. |
| BR-3b | The **site** must belong to the chosen company and be `Active` (E2 F2.6 ST-4). The site defaults to the company's **primary "Main Site"** and can be changed to any other active site. Its geofence (or absence) drives E5 clock-in (CI-2). |
| BR-4 | On successful creation, placement status is `Active`. Write an audit-log entry and notify the agent and the company's shift leader (if one is assigned). |
| BR-5 | Creation is not blocked if the company has no shift leader yet, but the UI surfaces a warning prompting F3.4. |


## 6. Data model (created fields)

| Field | Type | Required | Validation |
|-------|------|----------|------------|
| `employee_id` | FK → Employee | yes | employee exists & status = active |
| `client_company_id` | FK → ClientCompany | yes | company status = active (BR-3) |
| `site_id` | FK → Site (E2 F2.6) | yes | belongs to `client_company_id` & status = active (BR-3b); defaults to the company's primary Main Site |
| `start_date` | date | system | always set to today |
| `end_date` | date | system | always nil (open-ended) |
| `predecessor_id` | FK → Placement | system | null on plain create; set by renewal/transfer (F3.2/F3.3) |
| `notes` | text | no | — |
| `status` | enum | system | always `Active` on create |
| `created_by` | FK → User | system | actor id |

> Position is read from `Employee.position` (E2 F2.1) and is **not stored on Placement**. The employment agreement is **document-only** (E2 F2.2) — `employment_agreement_id` and `awaiting_agreement` are removed from Placement. Compensation (base salary) and annual-leave entitlement are EmploymentAgreement terms (E2), not placement fields. `service_line_id` and `position_id` are removed. `pkwt_reference` lives on EmploymentAgreement (E2).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Agent placement creation

  Background:
    Given I am signed in as an HR admin
    And an active agent "Budi" who has no active placement
    And an active client company "Plaza Senayan"

  Scenario: Create an immediately-active placement
    When I create a placement for "Budi" at "Plaza Senayan"
    Then the placement is created with status "Active" and start_date = today
    And "Budi"'s position is shown from Employee.position
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

  Scenario: Block double-booking an already-placed agent
    Given "Budi" already has an active placement at "Mall Kelapa Gading"
    When I try to place "Budi" at "Plaza Senayan"
    Then creation is blocked with the message "Agent already has an active placement"
    And I am offered to end or transfer the existing placement

  Scenario: Block placement into an inactive company
    Given the company "Old Tower" is archived
    When I try to place "Budi" at "Old Tower"
    Then creation is blocked with the message "Company is not active"

  Scenario: Warn when the company has no shift leader
    Given "Plaza Senayan" has no shift leader assigned
    When I create a placement for "Budi" at "Plaza Senayan"
    Then the placement is created successfully
    And I see a warning prompting me to assign a shift leader
```

## 8. Cases & edge cases

| # | Case | Expected behavior |
|---|------|-------------------|
| C-1 | Agent currently serving as shift leader at company A is placed at company B | Blocked by INV-1 (active placement at A); must transfer first (F3.3), which also vacates the leader role (F3.4). |
| C-2 | Two HR admins create overlapping placements for the same agent concurrently | Second commit fails the overlap check at persist time (DB constraint), not just UI — last writer gets BR-2 error. |
| C-3 | Company has no shift leader at creation | Created + warning (BR-5); notification to leader is skipped. |
| C-4 | Agent record is inactive/resigned | Blocked — only active employees can be placed. |
| C-5 | Renew or transfer a placement | Successor placement is created per F3.2/F3.3; position is unchanged (on Employee). |

## 9. Dependencies

- **E2** — Employee (with position), ClientCompany, Site master data must exist.
- **E1** — audit log + RBAC (only HR admin / super admin may create).
- **E10** — notifications (agent / shift leader).
- **F3.4** — shift-leader assignment (referenced for notification + warning).

## 10. Decisions & open questions

**Resolved (2026-06-19 — placement decoupled from time, position on Employee):**
- ✅ **Placement is decoupled from time** — `start_date` always today, `end_date` always nil. No Scheduled/future-dated state, no backdating, no period validation (BR-1, BR-2).
- ✅ **Position is on Employee** (E2 F2.1) — Placement no longer has a position field (BR-1). The same agent always has the same position regardless of placement/transfer. *(Supersedes the prior per-placement free-text position, BR-9.)*
- ✅ **No buffer** — the 1-day buffer between placements is removed; ending/transferring an existing placement makes the agent immediately placeable (BR-2).
- ✅ **Employment agreement is document-only** — `employment_agreement_id` and `awaiting_agreement` removed from Placement (E2 F2.2).

**Resolved (previous rounds — retained):**
- ✅ **INV-1** — one active placement per agent.
- ✅ **Site required** — placement targets a Site (E2 F2.6, BR-3b).
- ✅ **Company must be active** — BR-3.
- ✅ **Leader not required** — warn but don't block (BR-5).

_No open questions remain for this feature._
