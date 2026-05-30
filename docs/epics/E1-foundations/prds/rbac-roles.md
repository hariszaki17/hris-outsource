# PRD · F1.2 — RBAC, Roles & Scoping

> **Epic:** E1 Foundations & Platform · **Feature:** F1.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Access must be controlled consistently: four fixed roles, each with defined permissions, and — critically — a **shift leader can only act on their own company's** agents. The API enforces both the permission and the company scope on every request, so no UI bug can leak cross-company access.

## 2. Goals & non-goals

**Goals**
- Four fixed roles with seeded permissions: super_admin, hr_admin, shift_leader, agent.
- Enforce permission + **company scope** (shift leader → their company via E3) on every API call.

**Non-goals**
- User-configurable roles/permissions (not v1). Authentication (F1.1).

## 3. Actors

All users (subject to RBAC), System (enforce), Super Admin (assign roles).

## 4. Platform / clients

API-enforced for web + mobile; role drives navigation/visibility in both shells (F1.4).

## 5. Business rules

| Ref | Rule |
|-----|------|
| RB-1 | Roles are **fixed**: `super_admin`, `hr_admin`, `shift_leader`, `agent`; permissions seeded per role (INV-2). |
| RB-2 | **Enforcement is server-side** on every request (not just UI hiding). |
| RB-3 | **Company scope:** a `shift_leader` may act only on agents/records of the company from their active E3 `ShiftLeaderAssignment` (INV-3). |
| RB-4 | An `agent` may act only on **their own** records (self-scope). |
| RB-5 | `hr_admin` and `super_admin` are **cross-company**; super_admin additionally manages users/roles/config. |
| RB-6 | Role assignment/changes are restricted (super_admin; hr_admin per policy) and **audited** (F1.3). |
| RB-7 | A user with no active leader assignment loses shift-leader scope (falls back to base agent capabilities). |

## 6. Data model

`Role` (enum) + `RolePermission` (seeded). Company scope resolved at request time from E3 `ShiftLeaderAssignment`. No per-user permission rows in v1.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: RBAC & scoping

  Scenario: Permission enforced server-side
    Given an agent calls an HR-only endpoint
    Then the API returns 403 regardless of UI

  Scenario: Shift leader scoped to their company
    Given a shift leader of "Plaza Senayan"
    When they try to verify attendance for another company's agent
    Then it is denied

  Scenario: Agent self-scope
    Given an agent
    When they request another agent's data
    Then it is denied

  Scenario: HR is cross-company
    Given an HR admin
    Then they can act across all companies

  Scenario: Losing leader assignment removes scope
    Given a shift leader whose assignment ended (E3)
    Then they no longer have shift-leader permissions

  Scenario: Role change is audited
    When a super admin changes a user's role
    Then the change is recorded in the audit log
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | User is leader of one company + agent elsewhere? | Not possible — leader is 1:1 with their company and placed there (E3 INV). |
| C-2 | Endpoint mixes scoped + unscoped data | Scope filters applied per record. |
| C-3 | Super admin acting as another role | Allowed (highest privilege); audited. |
| C-4 | Migrated legacy role values | Mapped to the four roles (E2 DATA-MAPPING G-1). |

## 9. Dependencies

F1.1 (authenticated identity), E3 (shift-leader scope), F1.3 (audit), E2 (role remap on migration).

## 10. Decisions & open questions

- ✅ Fixed roles + server-side enforcement + company scoping from E3.
- **Open:** can `hr_admin` assign roles, or super_admin only?
- **Open:** any read-only/finance sub-role needed (raised in E8)?
