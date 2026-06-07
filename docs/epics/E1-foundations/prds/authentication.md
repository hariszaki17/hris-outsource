# PRD · F1.1 — Authentication & Sessions

> **Epic:** E1 Foundations & Platform · **Feature:** F1.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Every user — agents (mobile) and staff (web) — signs in with a single **identifier (phone OR email) + password**. **Phone is the universal identifier** (every agent has one, stored unique/normalized E.164 `+62`); **email is optional** and used mainly by staff. The platform needs secure login, password reset, and session handling that works for both a long-lived mobile app and a web console. Every employee is **auto-provisioned a login at create** (1:1, non-null — see §10), so there are no email-less or login-less users to special-case.

## 2. Goals & non-goals

**Goals**
- Phone-or-email + password login on web + mobile; secure password storage.
- Password reset; session/token lifecycle (incl. mobile "stay logged in").
- Record `last_login`; rate-limit/lockout on abuse.

**Non-goals**
- Roles/permissions (F1.2). Provisioning accounts (E2 F2.1). MFA (see §10).

## 3. Actors

All users (login), System (auth, sessions), Super Admin/HR (disable accounts).

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Mobile app** | Agent / Leader | Login, stay-signed-in, reset. |
| **Web console** | Staff | Login, reset. |
| **Go API** | — | Issues/validates sessions/tokens. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| AU-1 | Login is by **identifier (phone OR email) + password**; passwords stored hashed (argon2id/bcrypt), never plaintext. |
| AU-2 | Only **active** users may log in; disabled accounts are rejected. |
| AU-3 | Successful login records `last_login_at` and establishes a session/token (model per §10). |
| AU-4 | **Password reset** is available (token via email — see §10 for email-less agents). |
| AU-5 | Repeated failures trigger **rate-limiting / lockout** (thresholds §10). |
| AU-6 | Sessions can be **revoked** (logout-all / on disable / on **offboard**); mobile supports refresh for long sessions. Revocation is **instant**: a session-epoch (`users.tokens_valid_after`) bump + `RevokeAllRefreshForUser` invalidate every outstanding access token at the **next request**. The auth middleware does a **per-request user status + epoch check** (no longer purely stateless JWT). Revocation is tied to **employment-end** ([F2.7](../../E2-identity/prds/offboarding.md), OB-#); placement transfer/renewal/supersede/auto-end **never** revoke. |
| AU-7 | Auth events (login, failed login, reset, lockout) are **audited** (F1.3). |

## 6. Data model

`User` (phone unique+required, email nullable/optional-unique, password_hash, status, last_login_at — FEATURE §4) + sessions/tokens (model TBD §10).

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Authentication

  Scenario: Successful login
    Given an active user with valid credentials
    When they log in
    Then a session/token is issued and last_login is recorded

  Scenario: Disabled account cannot log in
    Given a disabled user
    When they try to log in
    Then login is rejected

  Scenario: Wrong password is rate-limited
    Given repeated failed attempts
    Then further attempts are throttled/locked and audited

  Scenario: Password reset
    Given a user requests a reset for a known email
    Then a reset link/token is sent
    And using it sets a new password

  Scenario: Mobile stays logged in
    Given an agent logged in on mobile
    Then the session persists across app restarts until expiry/logout

  Scenario: Revoke on disable
    Given an admin disables a user
    Then that user's session-epoch (tokens_valid_after) is bumped
    And their outstanding access tokens fail the next per-request middleware check
    # "invalidated" = rejected at the next request, not waiting for token expiry

  Scenario: Offboard revokes login
    Given HR ends an employee's employment (F2.7)
    Then the user's sessions are revoked instantly via epoch bump + RevokeAllRefreshForUser

  Scenario: Placement transfer does not revoke sessions
    Given an agent is transferred to a new placement (placement-end, not employment-end)
    Then their session-epoch is unchanged
    And their active sessions remain valid
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Agent without an email | Provisioning must assign one, or provide an alternative recovery (see §10). |
| C-2 | Reset requested for unknown email | Generic response (no account enumeration). |
| C-3 | Token expiry mid-session (mobile) | Silent refresh or graceful re-login. |
| C-4 | Concurrent sessions (web + mobile) | Allowed; both tracked/revocable. |

## 9. Dependencies

E2 (provisioning/email), F1.2 (role load post-login), F1.3 (audit), E10 (reset email/notifications).

## 10. Decisions & open questions

- ✅ Phone-or-email + password for all; hashed; reset; lockout.
- ✅ (2026-06-07) Every employee **auto-provisions a login at create** (Employee↔User 1:1, non-null); no data-only/no-login users and no opt-in provision step. Initial credential is a **system-generated temp password, shown once, force-rotated on first login**.
- **Open:** session model — **JWT (access+refresh)** vs server sessions; token lifetimes.
- **Open:** password policy + lockout thresholds; **MFA** for admin/HR in v1 or later?
