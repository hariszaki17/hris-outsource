# PRD · F2.1 — Employee & Agent Profile (+ login provisioning)

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Every agent and staff member needs a clean person record — identity, contact, statutory IDs (NIK, NPWP, BPJS), and bank details — that the rest of the system references. Legacy kept this in `employees`, separate from `users` (login). hris-outsource keeps the split but makes it **1:1 NON-NULL**: every employee **auto-provisions a login at create** (identifier = phone, or email), so agents can self-serve from the mobile app. There is no data-only employee.

## 2. Goals & non-goals

**Goals**
- One authoritative Employee record per person.
- Auto-provision a **self-service User login** (E1) at creation (default role `agent`), keyed on phone (or email).
- Let agents view (and make limited edits to) their own profile on mobile.

**Non-goals**
- Auth/session/RBAC mechanics (E1). Employment terms (F2.2). Placement (E3).
- Login/session revocation **mechanism** stays in E1, but it is no longer simply "out of scope" here: revocation is now **triggered by E2 offboarding** (F2.7) when an employee is deactivated/offboarded — see EP-7.

## 3. Actors

HR/Placement Admin & Super Admin (author), **Agent** (self, mobile/web — instant self-edit of the editable set), System (validate, provision, audit).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR/Super Admin | Full CRUD on all employee records; provision/deactivate logins; **offboard an employee (revoke all sessions)** per F2.7. |
| **Mobile app / Agent web console** | Agent | View own profile; **instant-edit** the editable set — photo, address, app language, **phone, emergency contact, and bank account** — applied immediately (no approval), still audited. Statutory fields are read-only. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| EP-1 | An Employee requires at minimum: full name, NIK, join date. |
| EP-2 | `NIK` is **unique**; the login `phone` is **required and unique**; the login `email` is **unique when present**. |
| EP-3 | Creating an employee **automatically and mandatorily** provisions a linked `User` (E1) with default role `agent`, identified by phone (or email), and issues a **one-time temp password** (EP-3 / D3). Provisioning is **not optional** and **not deferrable** — there is no create-without-login path. |
| EP-4 | Employee ↔ User is **1:1** (a User cannot be shared across employees). |
| EP-5 | **Agent self-edits are instant (no approval)** *(reworked 2026-06-14, EPICS §8 E11)*. The editable set — profile photo, address, app language, **phone, emergency contact (name + phone), and bank account** — takes effect **immediately**, still audited (EP-8). The previous change-request approval queue is **removed** (no `SWP-CHG`, no `change_requests.*`, no Inbox review for profile edits). |
| EP-5b | **Phone uniqueness on self-edit.** Because `phone` is the login identifier, a self-edited phone must remain **unique** (EP-2) and normalized (E.164/`+62`); a collision is rejected inline. Changing it updates the login identifier for the next sign-in. |
| ~~EP-5c~~ | _Removed 2026-06-14_ — no change-request routing/Inbox/diff for profile edits (edits apply instantly). |
| ~~EP-5d~~ | _Removed 2026-06-14_ — no bank-field approval split; bank account is **instant self-edit** like the other fields. |
| EP-6 | Statutory/identity & terms fields (NIK, NIP, name, birth date, NPWP, BPJS, placement, contract, compensation) are **read-only to the agent** — HR/admin-only. |
| EP-7 | Employees are **deactivated, not hard-deleted** (referenced by placements/attendance/history). Deactivation = **offboarding** (F2.7): it cascades to disable the linked `User` **and instantly revoke all of that user's sessions/refresh tokens** (F2.7 OB-1/OB-9, mechanism in E1). |
| EP-8 | All create/update/deactivate actions are audited (E1). |

## 6. Data model

`Employee`: `id, user_id (FK, NOT NULL), full_name, nik (unique), nip, join_at, gender, birth_date, birth_place, phone (required — login identifier), email_personal, address, npwp, bpjs_kesehatan, bpjs_ketenagakerjaan, bank_account, status, created_by`. Linked `User` (E1) carries phone (required) + email (optional) + password + role.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Employee profile & login provisioning

  Scenario: Create an employee always provisions a self-service login
    Given I am an HR admin
    When I create an employee with name, NIK, join date, and a phone number (or email)
    Then an Employee record is created
    And a linked User with role "agent" is created automatically
    And a one-time temp password is returned for me to hand over (show-once)

  Scenario: Reject duplicate NIK
    Given an employee with NIK "327xxxx" exists
    When I create another employee with the same NIK
    Then creation is blocked with a uniqueness error

  Scenario: Agent edits the editable set from mobile (instant)
    Given I am the agent "Budi" on the mobile app
    When I update my phone number, emergency contact, and bank account
    Then the changes apply immediately (no approval queue)
    And each change is audited
    And my statutory fields remain read-only

  Scenario: Self-edited phone must stay unique
    Given another user already uses phone "+62812..."
    When Budi changes his phone to "+62812..."
    Then the edit is rejected inline (uniqueness, EP-2/EP-5b)

  Scenario: Deactivate an employee disables the login
    When an HR admin deactivates "Budi"
    Then his Employee status becomes inactive
    And his active placement is ended
    And his User login is disabled
    And his existing sessions and refresh tokens are revoked immediately (F2.7 OB-1/OB-9)
    And his historical records remain intact
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| ~~C-1~~ | _Removed (2026-06-07)_ — provisioning a login for a data-only employee is no longer possible; every employee auto-provisions a login at create (EP-3, D1). | — |
| C-2 | Email already used by another User (when an email is supplied) | Blocked (EP-2). |
| C-3 | Re-activate a previously deactivated employee | Allowed; login re-enabled or re-invited. |
| C-4 | Agent self-edit of a statutory field | Not offered/blocked (EP-6). |
| C-5 | Bulk import on migration | Uniqueness + crosswalk applied; conflicts to review queue (E9). |
| C-6 | Agent edits any field in the editable set (photo/address/language/phone/emergency/bank) | Applies immediately, no approval queue; audit entry written (EP-5, EP-8). |
| C-7 | Agent edits several fields in one save | All apply now (instant); each audited. Phone collision rejects only that field (EP-5b). |
| ~~C-8~~ | _Removed 2026-06-14_ — no change-request review / bank escalation (bank is instant self-edit). | — |

## 9. Dependencies

E1 (User, auth, RBAC, audit), E10 (invite/notification email), E9 (migration import).

## 10. Decisions & open questions

- ✅ Employee **1:1 NON-NULL** User; default role `agent`.
- ✅ Login provisioning = **AUTOMATIC at create** (1:1, non-null); identifier **phone-or-email**; temp password **show-once** (2026-06-07, supersedes opt-in).
- ✅ **Agent self-edits are instant — no approval queue** *(reworked 2026-06-14, EPICS §8 E11 — supersedes the 2026-06-11 two-tier model)*. **Instant (no persetujuan):** profile photo, address, app language, **phone, emergency contact (name + phone), bank account** — apply immediately, audited; phone keeps its uniqueness check (EP-5b). **Read-only (HR-only):** NIK, NIP, name, birth date, NPWP, BPJS, placement, contract, compensation. The change-request approval feature (`SWP-CHG`, `change_requests.*`, the Inbox profile-review, the bank-escalation split) is **removed entirely** — profile approval is gone; the only configurable approval engine is **E11** (leave/overtime). Reflected in the Agent **Ubah Profil** modal (`brainstorm.pen`, now no "menunggu persetujuan" state).
