# PRD · F1.1 — Authentication & Sessions

> **Epic:** E1 Foundations & Platform · **Feature:** F1.1 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

Every user — agents (mobile) and staff (web) — signs in with **email + password**. The platform needs secure login, password reset, and session handling that works for both a long-lived mobile app and a web console. (One caveat: legacy `email_personal` was nullable, so some agents may lack email — addressed at provisioning, see §10.)

## 2. Goals & non-goals

**Goals**
- Email/password login on web + mobile; secure password storage.
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
| AU-1 | Login is **email + password**; passwords stored hashed (bcrypt/argon2), never plaintext. |
| AU-2 | Only **active** users may log in; disabled accounts are rejected. |
| AU-3 | Successful login records `last_login_at` and establishes a session/token (model per §10). |
| AU-4 | **Password reset** is available (token via email — see §10 for email-less agents). |
| AU-5 | Repeated failures trigger **rate-limiting / lockout** (thresholds §10). |
| AU-6 | Sessions can be **revoked** (logout-all / on disable); mobile supports refresh for long sessions. |
| AU-7 | Auth events (login, failed login, reset, lockout) are **audited** (F1.3). |

## 6. Data model

`User` (email unique, password_hash, status, last_login_at — FEATURE §4) + sessions/tokens (model TBD §10).

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
    Then that user's active sessions are invalidated
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

- ✅ Email + password for all; hashed; reset; lockout.
- **Open:** session model — **JWT (access+refresh)** vs server sessions; token lifetimes.
- **Open (C-1):** how email-less agents reset/recover (assign email at provisioning? phone fallback?).
- **Open:** password policy + lockout thresholds; **MFA** for admin/HR in v1 or later?
