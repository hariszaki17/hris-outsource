# PRD · F2.1 — Employee & Agent Profile (+ login provisioning)

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Every agent and staff member needs a clean person record — identity, contact, statutory IDs (NIK, NPWP, BPJS), and bank details — that the rest of the system references. Legacy kept this in `employees`, separate from `users` (login). hris-outsource keeps the split but makes it **1:1** and adds **hybrid login provisioning** so agents can self-serve from the mobile app.

## 2. Goals & non-goals

**Goals**
- One authoritative Employee record per person.
- Optionally provision a **self-service User login** (E1) at creation, defaulting to the `agent` role.
- Let agents view (and make limited edits to) their own profile on mobile.

**Non-goals**
- Auth/session/RBAC mechanics (E1). Employment terms (F2.2). Placement (E3).

## 3. Actors

HR/Placement Admin & Super Admin (author), **Agent** (self, mobile), System (validate, provision, audit).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR/Super Admin | Full CRUD on all employee records; provision/deactivate logins. |
| **Mobile app** | Agent | View own profile; request edits to a limited set (phone, address, bank) → HR approval. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| EP-1 | An Employee requires at minimum: full name, NIK, join date. |
| EP-2 | `NIK` and (if a login is provisioned) `email` are **unique**. |
| EP-3 | Provisioning a login creates a linked `User` (E1) with default role `agent`; an invite / temp password is issued. Provisioning is **optional at create** and can be done later. |
| EP-4 | Employee ↔ User is **1:1** (a User cannot be shared across employees). |
| EP-5 | Agent self-edits are limited to non-statutory fields (phone, address, bank) and require HR approval before they take effect. |
| EP-6 | Statutory fields (NIK, NPWP, name) are HR-only. |
| EP-7 | Employees are **deactivated, not hard-deleted** (referenced by placements/attendance/history); deactivating also disables the login. |
| EP-8 | All create/update/deactivate actions are audited (E1). |

## 6. Data model

`Employee`: `id, user_id (FK, nullable), full_name, nik (unique), nip, join_at, gender, birth_date, birth_place, phone, email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan, bank_account, status, created_by`. Linked `User` (E1) carries email/password/role.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Employee profile & login provisioning

  Scenario: Create an employee with a self-service login
    Given I am an HR admin
    When I create an employee with name, NIK, join date, and an email, choosing to provision a login
    Then an Employee record is created
    And a linked User with role "agent" is created
    And an invite is sent to set a password

  Scenario: Create a data-only employee (no login)
    When I create an employee without provisioning a login
    Then the Employee is saved with user_id empty
    And a login can be provisioned later

  Scenario: Reject duplicate NIK
    Given an employee with NIK "327xxxx" exists
    When I create another employee with the same NIK
    Then creation is blocked with a uniqueness error

  Scenario: Agent edits limited fields from mobile
    Given I am the agent "Budi" on the mobile app
    When I update my phone number and bank account
    Then the change is queued for HR approval
    And my statutory fields remain read-only

  Scenario: Deactivate an employee disables the login
    When an HR admin deactivates "Budi"
    Then his Employee status becomes inactive
    And his User login is disabled
    And his historical records remain intact
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Provision a login for an existing data-only employee | Creates the linked User later; same uniqueness checks. |
| C-2 | Email already used by another User | Blocked (EP-2). |
| C-3 | Re-activate a previously deactivated employee | Allowed; login re-enabled or re-invited. |
| C-4 | Agent self-edit of a statutory field | Not offered/blocked (EP-6). |
| C-5 | Bulk import on migration | Uniqueness + crosswalk applied; conflicts to review queue (E9). |

## 9. Dependencies

E1 (User, auth, RBAC, audit), E10 (invite/notification email), E9 (migration import).

## 10. Decisions & open questions

- ✅ Hybrid login; Employee 1:1 User; default role `agent`.
- ✅ Login provisioning = **opt-in at create** (provisionable later); not auto-invite.
- **Open:** which exact fields are agent-editable on mobile (proposed: phone, address, bank)?
