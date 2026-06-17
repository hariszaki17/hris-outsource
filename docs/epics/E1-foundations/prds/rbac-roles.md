# PRD · F1.2 — RBAC, Roles & Scoping

> **Epic:** E1 Foundations & Platform · **Feature:** F1.2 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Access must be controlled consistently: self-service is a **baseline every employee carries** (`self.*`), and **elevation roles** layer admin/operational power on top — and, critically, a **shift leader / lead can only act on their own company's** agents. The API enforces both the permission and the company scope on every request, so no UI bug can leak cross-company access. *(`agent` retired as a role 2026-06-15 — see EPICS §8 E1; "agent" is now only the domain term for an `employee_type = FIELD` employee.)*

## 2. Goals & non-goals

**Goals**
- `self.*` self-service baseline for **every** employee + seeded **elevation** roles: super_admin, hr_admin, lead (`shift_leader` derived via E3, not stored).
- Enforce permission + **company scope** (shift leader / lead → their company via E3) on every API call.

**Non-goals**
- User-configurable roles/permissions (not v1). Authentication (F1.1).

## 3. Actors

All users (subject to RBAC), System (enforce), Super Admin (assign roles).

## 4. Platform / clients

API-enforced for web + mobile; role drives navigation/visibility in both shells (F1.4).

## 5. Business rules

| Ref | Rule |
|-----|------|
| RB-1 | `self.*` self-service is a **baseline** every employee carries (no role needed). Elevation roles are **fixed**: `super_admin`, `hr_admin`, `lead` (+ `shift_leader`, **derived** per request from E3, not stored); permissions seeded per role (INV-2). `agent` is **not** a role (retired 2026-06-15) — it is the domain term for `employee_type = FIELD`. |
| RB-2 | **Enforcement is server-side** on every request (not just UI hiding). |
| RB-3 | **Company scope:** a `shift_leader` (or `lead`) may act only on agents/records of the company from their active E3 assignment / `lead_assignments` (INV-3). |
| RB-4 | Every employee may act only on **their own** records for `self.*` endpoints (self-scope); no role required. |
| RB-5 | `hr_admin` and `super_admin` are **cross-company**; super_admin additionally manages users/roles/config; `lead` is scoped to its assigned company set. |
| RB-6 | Role (elevation) assignment/changes are restricted (super_admin; hr_admin per policy) and **audited** (F1.3). |
| RB-7 | A user with no active leader assignment / no elevation falls back to the **baseline self-service** capabilities only (deny, never escalate). |

## 6. Data model

`Role` (enum — **elevations only**: `super_admin`, `hr_admin`, `lead`; `shift_leader` derived) + `RolePermission` (seeded). `self.*` baseline is implicit for every employee (no row, no role). `users.role` is **nullable** — null = baseline self-service. Company scope resolved at request time from E3 `ShiftLeaderAssignment` / `lead_assignments`. No per-user permission rows in v1.

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
| C-4 | Migrated legacy role values | Legacy `agent` → **no elevation** (baseline) + `employee_type=FIELD`; other legacy roles → their elevation + `employee_type=INTERNAL` (E9 DATA-MAPPING). |

## 9. Dependencies

F1.1 (authenticated identity), E3 (shift-leader scope), F1.3 (audit), E2 (role remap on migration).

## 10. Decisions & open questions

- ✅ Fixed roles + server-side enforcement + company scoping from E3.
- **Open:** can `hr_admin` assign roles, or super_admin only?
- **Open:** any read-only/finance sub-role needed (raised in E8)?
