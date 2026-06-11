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

HR/Placement Admin & Super Admin (author), **Agent** (self, mobile), **Shift leader** (change-request approver via Inbox; HR fallback), System (validate, provision, audit).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR/Super Admin | Full CRUD on all employee records; provision/deactivate logins; **offboard an employee (revoke all sessions)** per F2.7. |
| **Mobile app / Agent web console** | Agent | View own profile; **instant-edit** a small non-statutory set (photo, address, app language); **request edits** to phone, emergency contact, bank → approval. Statutory fields are read-only. |
| **Web console — Inbox** | Shift leader (default) / HR (fallback) | Review pending change requests (old→new diff) via `change_requests.approve`; approve (applies + audits) or reject (reason). Bank changes flagged sensitive. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| EP-1 | An Employee requires at minimum: full name, NIK, join date. |
| EP-2 | `NIK` is **unique**; the login `phone` is **required and unique**; the login `email` is **unique when present**. |
| EP-3 | Creating an employee **automatically and mandatorily** provisions a linked `User` (E1) with default role `agent`, identified by phone (or email), and issues a **one-time temp password** (EP-3 / D3). Provisioning is **not optional** and **not deferrable** — there is no create-without-login path. |
| EP-4 | Employee ↔ User is **1:1** (a User cannot be shared across employees). |
| EP-5 | Agent self-edits split into two tiers. **Instant (no approval)** — profile photo, address, app language: take effect immediately, still audited. **Approval-required** — phone, emergency contact (name + phone), bank: queued as a **change request**, take effect only after approval (EP-5b, EP-5c). |
| EP-5b | Approval-tier fields are gated for a reason: **phone** is the login identifier (security), **emergency contact** is a safety record that must stay verified, **bank** drives payroll. They cannot self-apply. Address is low-risk contact info → instant. |
| EP-5c | A change request is approved by the holder of **`change_requests.approve`** and surfaced in the unified **Inbox** (`Kotak Masuk`, E10) — same routing model as leave/OT: the agent's **shift leader** is the default approver, **HR** is fallback/oversight (no-leader → HR sole approver). The review UI shows a per-field **old→new diff**. Approve applies the field(s) + audits; reject requires a reason (E1 audit). |
| EP-5d | **Bank-account** changes are split out to the HR-only sub-permission **`change_requests.approve.bank`** (fraud/payroll risk); a shift leader does **not** have it. A request mixing bank + non-bank fields is **partially actionable**: the SL approves the non-bank fields and the **bank field escalates to HR** (stays pending until a `change_requests.approve.bank` holder acts) — never silently applied. The review UI disables the bank-field action and shows "Perlu HR" for approvers lacking the sub-permission. |
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

  Scenario: Agent edits limited fields from mobile
    Given I am the agent "Budi" on the mobile app
    When I update my phone number and bank account
    Then the change is queued as a change request
    And it appears in the approver's Inbox (shift leader, HR fallback) with an old→new diff
    And the bank change is flagged sensitive (payroll)
    And my statutory fields remain read-only

  Scenario: Approver acts on a change request from the Inbox
    Given a pending change request for agent "Budi" is in my Inbox
    And I hold "change_requests.approve" for Budi's company
    When I approve it
    Then the requested field values are applied to Budi's profile
    And the action is audited
    And rejecting instead would require a reason

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
| C-6 | Agent edits an instant-tier field (photo/emergency contact/language) | Applies immediately, no approval queue; audit entry written (EP-5, EP-8). |
| C-7 | Agent submits instant + approval fields in one save | Instant fields apply now; approval fields queue separately — partial-apply, not all-or-nothing. |
| C-8 | Shift leader reviews a change request containing a bank change | SL approves non-bank fields; the bank field is **disabled ("Perlu HR")** and **escalates to HR** (`change_requests.approve.bank`) — stays pending, never applied by the SL (EP-5d). |

## 9. Dependencies

E1 (User, auth, RBAC, audit), E10 (invite/notification email), E9 (migration import).

## 10. Decisions & open questions

- ✅ Employee **1:1 NON-NULL** User; default role `agent`.
- ✅ Login provisioning = **AUTOMATIC at create** (1:1, non-null); identifier **phone-or-email**; temp password **show-once** (2026-06-07, supersedes opt-in).
- ✅ **Agent-editable field tiers** *(resolved 2026-06-11)* — **Instant (no persetujuan):** profile photo, address, app language. **HR-approval-required:** phone, emergency contact (name + phone), bank account. **Read-only (HR-only):** NIK, NIP, name, birth date, NPWP, BPJS, placement, contract, compensation. Supersedes the earlier open "proposed: phone, address, bank" (which omitted the instant tier). Reflected in the Agent web console **Ubah Profil** modal (`brainstorm.pen`).
