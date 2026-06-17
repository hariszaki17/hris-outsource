# Test Cases — E1 Foundations & Platform

> Manual QA test-case catalogue for **E1 — Foundations & Platform** (authentication, RBAC + company scoping, comprehensive audit log, platform conventions / app shell).
> Scope: F1.1 Authentication & Sessions · F1.2 RBAC, Roles & Scoping · F1.3 Comprehensive Audit Log · F1.4 Platform Conventions & App Shell.
> Source of truth: `docs/epics/E1-foundations/FEATURE.md` + PRDs, plus `docs/api/CONVENTIONS.md`. UI copy is **Bahasa Indonesia** (INV-6 / PC-1). All times are **Asia/Jakarta / WIB** (INV-6 / PC-2). Absolute dates used throughout.

---

## How to use this document

- Each test case has a `- [ ]` checkbox — tick it when the case passes on the build under test.
- **Roles** (FEATURE §2): Super Admin · HR/Placement Admin · Shift Leader · Agent. "Agent" = an `employee_type = FIELD` employee carrying only the `self.*` baseline (the `agent` *role* was retired 2026-06-15; baseline self-service is roleless).
- **Platforms** (FEATURE §6): Web console (Super Admin / HR / Shift Leader) · Mobile app (Agent / Shift Leader). The Go API serves both and is where auth/RBAC/audit are enforced.
- Priority: **P0** = blocks release (auth, RBAC denial, audit write, revocation) · **P1** = important · **P2** = polish/edge.

---

## 1. Coverage matrix

Cells: ✓ = feature is exercised for that platform/role in this doc; — = not applicable per the PRD's Actors / Platform sections.

| Feature | Web | Mobile | Super Admin | HR/Placement Admin | Shift Leader | Agent |
|---|---|---|---|---|---|---|
| **F1.1** Authentication & Sessions | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| **F1.2** RBAC, Roles & Scoping | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| **F1.3** Comprehensive Audit Log | ✓ | — | ✓ | ✓ | — | ✓ (denial only) |
| **F1.4** Platform Conventions & App Shell | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

> F1.3 audit *reads* are Web console + HR/Super Admin only (AL-7); the agent/shift-leader rows exist only to prove denial. Mobile has no audit-search surface. The API writes audit entries for mutations originating from any platform/role (INV-4 / §16.1).

---

## F1.1 — Authentication & Sessions

> Identifier (phone OR email) + password login for all users on web + mobile; secure hashed storage; reset; rate-limit/lockout; session/token lifecycle with **instant revocation** via `users.status` + session-epoch `tokens_valid_after`. Rules AU-1…AU-7, cases C-1…C-4. Public endpoints: `POST /auth/login`, `POST /auth/forgot-password` (CONVENTIONS §1).

### Web console · Super Admin POV

#### TC-E1-F1.1-001 · Login with email + password (happy path)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** A valid, active super admin can sign in with email + password and lands on the web console.
- **Preconditions:** Seeded active super-admin user with a known email + password; `status=active`.
- **Steps:**
  1. Open the web login screen (Bahasa Indonesia copy: "Masuk").
  2. Enter the email identifier and correct password.
  3. Submit ("Masuk").
- **Expected result / Acceptance criteria:** `POST /api/v1/auth/login` returns access token + refresh token; user is redirected to the console home; `last_login_at` is updated to the current WIB timestamp; nav reflects super-admin elevation (F1.4 PC-4).
- **Traceability:** F1.1, AU-1, AU-3, INV-5, US "Successful login".

#### TC-E1-F1.1-002 · Login with phone identifier (E.164)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Login works when the identifier is the phone number, not email.
- **Preconditions:** Same active super-admin user; phone stored normalized E.164 `+62…`.
- **Steps:**
  1. On the login screen, enter the phone number as identifier (test both `+62…` and local `08…` normalization if the UI accepts it).
  2. Enter the correct password and submit.
- **Expected result:** Login succeeds identically to email; token issued; `last_login_at` updated.
- **Traceability:** F1.1, AU-1, INV-5 (phone universal identifier).

#### TC-E1-F1.1-003 · Wrong password rejected (generic error)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Incorrect password is rejected without revealing whether the account exists.
- **Preconditions:** Active super-admin user known.
- **Steps:**
  1. Enter a valid identifier with a wrong password.
  2. Submit.
- **Expected result:** `401`/`400` per spec with a generic Indonesian message (e.g., "Identifier atau kata sandi salah."); no token issued; no field-level disclosure of which part was wrong; a **failed-login** event is audited (AU-7).
- **Traceability:** F1.1, AU-1, AU-7, C-2 (no enumeration principle).

### Web console · HR/Placement Admin POV

#### TC-E1-F1.1-004 · HR admin login (happy path)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Active HR admin signs in and lands on HR navigation.
- **Preconditions:** Seeded active hr_admin user.
- **Steps:** Enter identifier + password → submit.
- **Expected result:** Tokens issued; `last_login_at` recorded; nav shows HR elevation surfaces (employees, placements), not super-admin config (PC-4); login audited.
- **Traceability:** F1.1, AU-1, AU-3, AU-7.

#### TC-E1-F1.1-005 · Disabled HR account cannot log in
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** A user whose `status=disabled` is rejected at login even with correct credentials.
- **Preconditions:** hr_admin user with `status=disabled`.
- **Steps:**
  1. Enter correct identifier + password.
  2. Submit.
- **Expected result:** Login rejected (`401`/`403`); no token issued; Indonesian message indicating the account is inactive (no detailed reason that aids enumeration); rejection is auditable.
- **Traceability:** F1.1, AU-2, US "Disabled account cannot log in".

#### TC-E1-F1.1-006 · Forgot password — known email sends reset link
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Requesting a reset for a known email dispatches a reset token and lets the user set a new password.
- **Preconditions:** hr_admin user with an email on file.
- **Steps:**
  1. On login, click "Lupa kata sandi".
  2. Enter the known email and submit (`POST /api/v1/auth/forgot-password`).
  3. Use the emailed reset token/link, enter a new password, confirm.
  4. Log in with the new password.
- **Expected result:** Generic confirmation shown ("Jika akun terdaftar, tautan reset telah dikirim."); a reset token is sent; the token sets a new password; old password no longer works; login with new password succeeds; reset event audited.
- **Traceability:** F1.1, AU-4, AU-7, US "Password reset".

#### TC-E1-F1.1-007 · Forgot password — unknown email gives generic response (no enumeration)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Requesting reset for an email with no account returns the same generic response — no account enumeration.
- **Preconditions:** An email address that maps to no user.
- **Steps:** Submit the unknown email to "Lupa kata sandi".
- **Expected result:** Identical generic confirmation copy and timing as TC-006; no indication the account does not exist; no email sent.
- **Traceability:** F1.1, AU-4, **C-2**.

### Web console · Shift Leader POV

#### TC-E1-F1.1-008 · Shift leader login on web
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** A field employee with an active E3 shift-leader assignment can sign in on web and gets leader nav.
- **Preconditions:** Active employee with one active `shift_leader_assignments` row (E3) → derived role `shift_leader`, scope = that company.
- **Steps:** Enter identifier + password → submit.
- **Expected result:** Tokens issued; `last_login_at` recorded; post-login role load (F1.2) derives `shift_leader` + company scope; nav shows leader surfaces (roster, approvals), not config (PC-4).
- **Traceability:** F1.1, AU-3, INV-3, F1.4 PC-4.

### Mobile app · Agent POV

#### TC-E1-F1.1-009 · Agent login on mobile (happy path)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** A field agent signs in on the mobile app with phone + password.
- **Preconditions:** Active employee, `employee_type=FIELD`, no elevation; auto-provisioned login (1:1 with employee).
- **Steps:**
  1. Open the mobile app login screen.
  2. Enter phone identifier + password.
  3. Tap "Masuk".
- **Expected result:** Tokens issued; `last_login_at` recorded; lands on the agent self-service home (mobile nav: clock-in, schedule, leave/OT, payslip).
- **Traceability:** F1.1, AU-1, AU-3, INV-5.

#### TC-E1-F1.1-010 · First-login forced password rotation (temp password)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy/Edge
- **Objective:** A newly auto-provisioned agent logging in with the system temp password is forced to set a new one before continuing.
- **Preconditions:** Employee just auto-provisioned at create (E2); credential is a system-generated temp password shown once.
- **Steps:**
  1. Log in with the temp password.
  2. Observe the forced "ganti kata sandi" screen.
  3. Set a new password and confirm.
  4. Continue.
- **Expected result:** App blocks access to features until the password is rotated; after rotation the temp password is invalidated; subsequent logins use the new password; password stored hashed (argon2id/bcrypt), never plaintext.
- **Traceability:** F1.1, AU-1, §10 decision (2026-06-07 temp password force-rotate), INV-5.

#### TC-E1-F1.1-011 · Mobile stay-signed-in across app restarts
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** A logged-in agent stays signed in after closing and reopening the app, until expiry/logout.
- **Preconditions:** Agent logged in on mobile with a stored refresh token.
- **Steps:**
  1. Log in.
  2. Force-quit and reopen the app several times over a session.
- **Expected result:** Session persists across restarts without re-entering credentials; refresh token used to obtain new access tokens; session ends only at expiry or explicit logout.
- **Traceability:** F1.1, AU-6, US "Mobile stays logged in".

#### TC-E1-F1.1-012 · Token expiry mid-session — silent refresh / graceful re-login
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Access-token expiry mid-session is handled without data loss — silent refresh, or graceful re-login if refresh fails.
- **Preconditions:** Agent logged in; access token near expiry (or force-expire via test hook).
- **Steps:**
  1. Let the access token expire while using the app (e.g., on a list screen).
  2. Trigger an authenticated request.
- **Expected result:** Client silently refreshes via `POST /api/v1/auth/refresh` and the request succeeds; if refresh is invalid, the app renders a graceful re-login (`comp/EmptySessionExpired` equivalent) rather than a crash or raw `401`.
- **Traceability:** F1.1, **C-3**, AU-6, CONVENTIONS §3 (401 session-expired UX).

#### TC-E1-F1.1-013 · Disabled agent rejected on mobile
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Negative
- **Objective:** A disabled agent cannot log in on mobile.
- **Preconditions:** Agent user `status=disabled`.
- **Steps:** Enter correct credentials → tap Masuk.
- **Expected result:** Rejected; no token; inactive-account message; auditable.
- **Traceability:** F1.1, AU-2.

### Mobile app · Shift Leader POV

#### TC-E1-F1.1-014 · Shift leader login on mobile
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** A shift leader signs in on mobile and gets leader mobile nav scoped to their company.
- **Preconditions:** Active employee with one active E3 shift-leader assignment.
- **Steps:** Enter phone + password → Masuk.
- **Expected result:** Tokens issued; `last_login_at` recorded; mobile leader nav (`comp/SLMobileNav`) visible; scope derived = their company.
- **Traceability:** F1.1, AU-3, INV-3.

### Cross-platform · Auth security & sessions

#### TC-E1-F1.1-015 · Rate-limit / lockout on repeated failed logins
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Repeated wrong-password attempts trigger throttling/lockout and are audited.
- **Preconditions:** Known active user; clean rate-limit counter.
- **Steps:**
  1. Submit wrong password repeatedly past the lockout threshold (per §10 deferred thresholds — use the configured value).
  2. Continue attempting.
- **Expected result:** Further attempts are throttled/locked: API returns `429 Too Many Requests` with `error.code: "RATE_LIMITED"` + `Retry-After` header; UI shows an Indonesian "terlalu banyak percobaan" message; each failed attempt + the lockout are audited (AU-5, AU-7).
- **Traceability:** F1.1, AU-5, AU-7, US "Wrong password is rate-limited", CONVENTIONS §11/§19 (RATE_LIMITED).

#### TC-E1-F1.1-016 · Concurrent sessions web + mobile both valid and tracked
- [ ] **Platform:** Web + Mobile · **POV:** Shift Leader · **Priority:** P1 · **Type:** Edge
- **Objective:** A user logged in on web and mobile simultaneously has both sessions valid and independently revocable.
- **Preconditions:** Same user able to use both surfaces.
- **Steps:**
  1. Log in on web.
  2. Log in on mobile with the same account.
  3. Use authenticated actions on both.
- **Expected result:** Both sessions work concurrently; both are tracked; logging out of one does not by itself kill the other (unless logout-all is invoked).
- **Traceability:** F1.1, **C-4**, AU-6.

#### TC-E1-F1.1-017 · Disable user → outstanding access tokens fail at next request (instant revocation)
- [ ] **Platform:** Web (admin action) + Mobile (victim session) · **POV:** Super Admin disables; Agent session · **Priority:** P0 · **Type:** RBAC/Negative
- **Objective:** When an admin disables a user, that user's already-issued access token fails the next per-request middleware check — not after token expiry.
- **Preconditions:** Agent logged in on mobile with a live access token; super admin logged in on web.
- **Steps:**
  1. Super admin disables the agent's account (sets `status=disabled`, bumps `tokens_valid_after`, calls `RevokeAllRefreshForUser`).
  2. From the agent's still-open mobile app, trigger any authenticated request.
- **Expected result:** The agent's outstanding access token is rejected on the **next request** (`401`), because `tokens_valid_after >= token.iat` fails and/or `status=disabled`; refresh token is revoked so silent refresh fails; app drops to re-login; the disable action is audited.
- **Traceability:** F1.1, AU-2, AU-6, US "Revoke on disable", FEATURE note (per-request status+epoch check), CONVENTIONS §3 (not purely stateless).

#### TC-E1-F1.1-018 · Offboard (employment-end) revokes login instantly
- [ ] **Platform:** Web (HR action) + Mobile (victim) · **POV:** HR/Placement Admin offboards; Agent session · **Priority:** P0 · **Type:** Negative
- **Objective:** Ending an employee's employment (F2.7) revokes their sessions instantly via epoch bump + refresh revocation.
- **Preconditions:** Agent logged in on mobile; HR admin can run offboard (E2 F2.7).
- **Steps:**
  1. HR ends the employee's employment.
  2. From the agent's mobile session, trigger an authenticated request.
- **Expected result:** Session-epoch bumped; `RevokeAllRefreshForUser` invoked; agent's next request returns `401`; agent forced to re-login (and login then rejected because employment ended); offboard audited.
- **Traceability:** F1.1, AU-6, US "Offboard revokes login", FEATURE §7 (2026-06-06), CONVENTIONS §3.

#### TC-E1-F1.1-019 · Placement transfer does NOT revoke sessions
- [ ] **Platform:** Web (HR action) + Mobile (subject) · **POV:** HR transfers; Agent session · **Priority:** P0 · **Type:** Edge
- **Objective:** A placement-end event (transfer/renewal/supersede/auto-end) must NOT bump the session epoch or revoke sessions.
- **Preconditions:** Agent logged in on mobile with an active placement.
- **Steps:**
  1. HR transfers the agent to a new placement (placement-end, not employment-end).
  2. From the agent's mobile session, continue using authenticated actions.
- **Expected result:** `tokens_valid_after` unchanged; access + refresh tokens stay valid; the agent's session continues uninterrupted.
- **Traceability:** F1.1, AU-6, US "Placement transfer does not revoke sessions", FEATURE §7.

#### TC-E1-F1.1-020 · Logout-all revokes every session
- [ ] **Platform:** Web + Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** A user invoking logout-all has all their sessions (web + mobile) invalidated.
- **Preconditions:** Same user logged in on web and mobile.
- **Steps:**
  1. From one surface, invoke logout-all (or "Keluar dari semua perangkat").
  2. Attempt authenticated actions on the other surface.
- **Expected result:** Epoch bump + refresh revocation; all outstanding tokens fail next request; both surfaces drop to login.
- **Traceability:** F1.1, AU-6, **C-4**.

#### TC-E1-F1.1-021 · Password never returned/exposed; stored hashed
- [ ] **Platform:** API · **POV:** Super Admin · **Priority:** P0 · **Type:** Negative/Security
- **Objective:** No endpoint or response ever returns the password or password hash; storage is hashed.
- **Preconditions:** A user record; API access.
- **Steps:**
  1. Fetch a user via any read endpoint.
  2. Inspect login and reset responses.
- **Expected result:** No `password`/`password_hash` field in any response payload; DB stores argon2id/bcrypt hash, never plaintext.
- **Traceability:** F1.1, AU-1, INV-5.

#### TC-E1-F1.1-022 · Malformed login request → standard error envelope
- [ ] **Platform:** API · **POV:** any · **Priority:** P1 · **Type:** Error
- **Objective:** A structurally invalid login request returns the standard error envelope.
- **Preconditions:** —
- **Steps:**
  1. `POST /api/v1/auth/login` with missing `password` (or malformed JSON).
- **Expected result:** `400 Bad Request` with body `{ "error": { "code": "INVALID_REQUEST", "message": <ID>, "fields": {…}, "request_id": "…" } }`; `fields` maps the missing field; Indonesian message.
- **Traceability:** F1.1, CONVENTIONS §11/§12, PC-3.

---

## F1.2 — RBAC, Roles & Scoping

> Baseline `self.*` for every employee; fixed elevations `super_admin`, `hr_admin`, `lead`; `shift_leader` derived per request from E3. Server enforces permission **and** company scope on every request — UI hiding is defense-in-depth only. Rules RB-1…RB-7, cases C-1…C-4, INV-2/INV-3.

### API / Web console · Super Admin POV

#### TC-E1-F1.2-001 · Super admin reaches global admin endpoints
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Happy/RBAC
- **Objective:** Super admin can access user/role/config management (global scope).
- **Preconditions:** Logged-in super admin.
- **Steps:** Navigate to user management; call a `x-rbac: {roles: [super_admin], scope: global}` endpoint (e.g., assign-role).
- **Expected result:** `200`/`201`; action allowed across all companies; surfaces visible in nav.
- **Traceability:** F1.2, RB-5, CONVENTIONS §17.

#### TC-E1-F1.2-002 · Super admin assigns an elevation role → audited
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Happy/Audit
- **Objective:** Changing a user's elevation role is permitted for super admin and writes an audit entry.
- **Preconditions:** Target user with no/other elevation.
- **Steps:**
  1. Open the target user.
  2. Set elevation to `hr_admin` (or `lead`).
  3. Save.
- **Expected result:** Role updated; `200`; an `AuditLog` entry records actor (super admin), action (role change), entity (user), `before`/`after` role values, ip, timestamp.
- **Traceability:** F1.2, RB-6, US "Role change is audited", F1.3 AL-1/AL-2, INV-4.

#### TC-E1-F1.2-003 · Super admin acting as another role (highest privilege) — allowed + audited
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Super admin may perform actions belonging to lower roles; such actions are audited.
- **Preconditions:** Logged-in super admin.
- **Steps:** Perform an HR-scoped or company-scoped action as super admin.
- **Expected result:** Allowed (super admin has highest privilege, cross-company); action audited with super-admin actor.
- **Traceability:** F1.2, **C-3**, RB-5, INV-4.

### API / Web console · HR/Placement Admin POV

#### TC-E1-F1.2-004 · HR admin is cross-company
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy/RBAC
- **Objective:** HR admin can act across all companies (global), not limited to one.
- **Preconditions:** Logged-in hr_admin; data for ≥2 companies.
- **Steps:** Read/act on records belonging to different companies.
- **Expected result:** All companies accessible; no `OUT_OF_SCOPE`.
- **Traceability:** F1.2, RB-5, US "HR is cross-company".

#### TC-E1-F1.2-005 · HR admin role-assignment per policy → audited (or denied if policy excludes)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** RBAC/Audit
- **Objective:** Per the resolved decision, both super_admin and hr_admin may assign roles; verify hr_admin role-change is allowed and audited.
- **Preconditions:** Logged-in hr_admin; target user.
- **Steps:** hr_admin assigns/changes a user's elevation role and saves.
- **Expected result:** Allowed per FEATURE §7 ("super_admin and hr_admin may assign roles"); change audited (RB-6). If the build restricts to super_admin only, expect `403 FORBIDDEN` instead — flag the discrepancy against FEATURE §7.
- **Traceability:** F1.2, RB-6, FEATURE §7 (2026-05-29 role-assignment), RBAC open-item §10.

#### TC-E1-F1.2-006 · HR admin denied super-admin-only config
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** RBAC
- **Objective:** HR admin cannot reach super-admin-only system config / user-management endpoints.
- **Preconditions:** Logged-in hr_admin.
- **Steps:** Call a `roles: [super_admin]` endpoint directly via API; check the UI does not surface it.
- **Expected result:** API returns `403` with `error.code: "FORBIDDEN"`; UI hides the surface but tolerates a forced `403` defensively (renders `comp/EmptyNoPermission`).
- **Traceability:** F1.2, RB-2, RB-5, CONVENTIONS §3/§17.

### API / Web console · Shift Leader POV

#### TC-E1-F1.2-007 · Shift leader acts only on own-company agents (in scope)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy/RBAC
- **Objective:** A shift leader can perform company-scoped actions on agents of their assigned company.
- **Preconditions:** Shift leader of "Plaza Senayan" (active E3 assignment); agents placed there.
- **Steps:** Perform a company-scoped action (e.g., verify attendance) for an own-company agent.
- **Expected result:** Allowed (`200`); scope resolved server-side from the active `shift_leader_assignments` row.
- **Traceability:** F1.2, RB-3, INV-3, CONVENTIONS §17.

#### TC-E1-F1.2-008 · Shift leader denied cross-company action (OUT_OF_SCOPE)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** A shift leader of company A cannot act on company B's agent.
- **Preconditions:** Shift leader of "Plaza Senayan"; an agent of a different company.
- **Steps:** Attempt to verify/act on the other company's agent (direct API call to bypass UI hiding).
- **Expected result:** `403` with `error.code: "OUT_OF_SCOPE"`; no mutation; defense-in-depth — denial happens server-side regardless of UI.
- **Traceability:** F1.2, RB-3, US "Shift leader scoped to their company", CONVENTIONS §11 (OUT_OF_SCOPE), INV-3.

#### TC-E1-F1.2-009 · Ended assignment removes shift-leader scope (derived, fail-safe)
- [ ] **Platform:** Web · **POV:** Shift Leader → baseline · **Priority:** P0 · **Type:** Edge/RBAC
- **Objective:** When a shift leader's E3 assignment ends, they immediately lose shift-leader permissions on the next request (role is derived, not stored).
- **Preconditions:** Active shift leader; ability to end their E3 assignment.
- **Steps:**
  1. End the leader's E3 `shift_leader_assignments` row.
  2. From the same session, attempt a previously-allowed company-scoped action.
- **Expected result:** Action denied (`403`); the user falls back to baseline `self.*` only (deny, never escalate); no token reissue needed — resolved per request.
- **Traceability:** F1.2, RB-3, RB-7, US "Losing leader assignment removes scope", INV-3, CONVENTIONS §17 (derived; fail-safe).

#### TC-E1-F1.2-010 · Mixed scoped+unscoped endpoint filters per record
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Edge
- **Objective:** An endpoint returning records spanning multiple companies returns only the leader's in-scope records.
- **Preconditions:** Shift leader; data set spanning multiple companies.
- **Steps:** Call a list endpoint that could include cross-company rows.
- **Expected result:** Response contains only the leader's company records; out-of-scope rows filtered (not a blanket `403`).
- **Traceability:** F1.2, **C-2**, RB-3.

### API / Mobile · Agent POV

#### TC-E1-F1.2-011 · Agent self-scope happy path
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** A baseline agent can act on their own `self.*` records.
- **Preconditions:** Active field agent.
- **Steps:** Fetch own schedule/attendance/payslip via `scope: self` endpoints.
- **Expected result:** `200`; only the agent's own records returned; no role required.
- **Traceability:** F1.2, RB-1, RB-4, CONVENTIONS §17 (self baseline).

#### TC-E1-F1.2-012 · Agent denied another agent's data (self-scope enforced)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** An agent requesting another agent's record is denied server-side.
- **Preconditions:** Two distinct agents.
- **Steps:** Agent A requests Agent B's employee/attendance record by ID (direct API call).
- **Expected result:** Denied — `403`/`404` (404 used to avoid leaking existence per CONVENTIONS §7); no data returned.
- **Traceability:** F1.2, RB-4, US "Agent self-scope", CONVENTIONS §7.

#### TC-E1-F1.2-013 · Agent calls HR-only endpoint → 403 regardless of UI
- [ ] **Platform:** Mobile/API · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Server-side enforcement: a baseline agent hitting an HR-only endpoint is rejected even though the mobile UI never shows it.
- **Preconditions:** Active agent; an HR-only endpoint (`roles: [hr_admin, super_admin]`).
- **Steps:** Craft a direct API request to the HR-only endpoint with the agent's token.
- **Expected result:** `403` with `error.code: "FORBIDDEN"`; no action performed; proves RB-2 (enforcement is server-side, not UI hiding).
- **Traceability:** F1.2, RB-2, US "Permission enforced server-side".

#### TC-E1-F1.2-014 · No-elevation + no-assignment user gets baseline only (deny, never escalate)
- [ ] **Platform:** Mobile/API · **POV:** Agent · **Priority:** P1 · **Type:** Edge/RBAC
- **Objective:** A user with null `users.role` and no active E3 assignment falls back to baseline self-service and cannot escalate.
- **Preconditions:** Employee with `role=NULL`, no `shift_leader_assignments`.
- **Steps:** Attempt any company-scoped or elevation-gated action.
- **Expected result:** Denied; only `self.*` succeeds; resolver fail-safe strips scope on error/no-assignment.
- **Traceability:** F1.2, RB-7, INV-3, CONVENTIONS §17 (fail-safe).

### API · Edge / migration mapping

#### TC-E1-F1.2-015 · Migrated legacy "agent" → no elevation + FIELD
- [ ] **Platform:** API · **POV:** Super Admin (verifier) · **Priority:** P1 · **Type:** Edge
- **Objective:** A migrated user whose legacy role was `agent` carries no elevation and `employee_type=FIELD`.
- **Preconditions:** A migrated user (E9) with legacy role `agent`.
- **Steps:** Inspect the migrated user's `users.role` and the employee's `employee_type`.
- **Expected result:** `users.role = NULL` (baseline self-service); `employee_type = FIELD`; user gets `self.*` only.
- **Traceability:** F1.2, **C-4**, RB-1, E9 DATA-MAPPING.

#### TC-E1-F1.2-016 · Migrated legacy staff role → elevation + INTERNAL
- [ ] **Platform:** API · **POV:** Super Admin (verifier) · **Priority:** P1 · **Type:** Edge
- **Objective:** Non-agent legacy roles map to their elevation and `employee_type=INTERNAL`.
- **Preconditions:** A migrated user with a legacy staff role.
- **Steps:** Inspect mapped `users.role` and `employee_type`.
- **Expected result:** Elevation set to the mapped role; `employee_type = INTERNAL`.
- **Traceability:** F1.2, **C-4**, RB-5, E9 DATA-MAPPING.

#### TC-E1-F1.2-017 · Client UI hides unauthorized actions (defense-in-depth, not the gate)
- [ ] **Platform:** Web + Mobile · **POV:** Agent / Shift Leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** Unauthorized actions are hidden in the UI but the client still tolerates a forced `403`.
- **Preconditions:** Agent on mobile; shift leader on web.
- **Steps:**
  1. Confirm HR/super-admin controls are not rendered for these roles.
  2. Force a `403` (e.g., via expired scope) and observe the client.
- **Expected result:** Unauthorized controls absent from the UI; on a forced `403`, the client renders the no-permission state (`comp/EmptyNoPermission`) and does not crash.
- **Traceability:** F1.2, RB-2, CONVENTIONS §3/§17 (clients hide but tolerate 403).

---

## F1.3 — Comprehensive Audit Log

> Every mutation writes an immutable, queryable `AuditLog` entry (actor, action, entity, before/after, ip, time). Sensitive comp values masked. Access HR/Super Admin only. Rules AL-1…AL-7, cases C-1…C-4, INV-4. Web console only; no mobile audit surface.

### API · Write path (any platform/role originating mutation)

#### TC-E1-F1.3-001 · Mutation writes an audit entry with full fields
- [ ] **Platform:** API · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy/Audit
- **Objective:** A create/update/delete writes one audit entry capturing actor, action, entity, before/after, ip, time.
- **Preconditions:** HR admin logged in; a mutable entity (e.g., a placement or employee).
- **Steps:**
  1. HR admin performs an update (e.g., edit a placement field).
  2. Query `GET /api/v1/audit-log?entity_type=…&entity_id=…`.
- **Expected result:** Exactly one new entry with `actor_user_id` = HR admin, `action`, `entity_type`, `entity_id`, populated `before` and `after`, `ip`, and `created_at` (UTC stored, WIB rendered); `request_id` correlates to the request.
- **Traceability:** F1.3, AL-1, AL-2, INV-4, US "Mutation writes an audit entry", CONVENTIONS §16.1.

#### TC-E1-F1.3-002 · Sensitive comp/payroll values masked in before/after
- [ ] **Platform:** API/Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Audit/Security
- **Objective:** When a compensation/payroll field changes, the audit logs the fact-of-change without storing cleartext amounts.
- **Preconditions:** An entity with a comp field (e.g., agreement salary).
- **Steps:**
  1. Change a compensation amount.
  2. Inspect the resulting audit entry's `before`/`after`.
- **Expected result:** Comp/payroll values are masked (e.g., redacted/`***`); the entry records that the field changed but not the cleartext numbers.
- **Traceability:** F1.3, AL-4, US "Sensitive values masked".

#### TC-E1-F1.3-003 · System/automated action attributed to "system"
- [ ] **Platform:** API · **POV:** System · **Priority:** P1 · **Type:** Audit/Edge
- **Objective:** A cron/automated mutation (e.g., auto-clock-out) is attributed to `system` with context.
- **Preconditions:** Trigger an automated job that mutates a record (or simulate via test hook).
- **Steps:** Run the job; query the audit log for the affected entity.
- **Expected result:** Entry `actor_user_id` = `system` (not a user); context noted; standard fields present.
- **Traceability:** F1.3, AL-6, US "System actions attributed".

#### TC-E1-F1.3-004 · Migration writes attributed to system/migration run id
- [ ] **Platform:** API · **POV:** System · **Priority:** P2 · **Type:** Audit/Edge
- **Objective:** E9 migration-originated mutations are attributed to `system` with a migration run id.
- **Preconditions:** A migration run (E9) producing audited writes.
- **Steps:** Inspect audit entries created during migration.
- **Expected result:** Entries attributed to `system`/migration run id, not a human user.
- **Traceability:** F1.3, AL-6, **C-3**.

#### TC-E1-F1.3-005 · Bulk operation writes one entry per affected entity
- [ ] **Platform:** API · **POV:** Shift Leader · **Priority:** P1 · **Type:** Audit/Edge
- **Objective:** A bulk action (e.g., `:bulk-verify` of 10 items) writes one audit entry per affected entity.
- **Preconditions:** Shift leader with ≥10 verifiable attendance records in scope.
- **Steps:**
  1. Submit a bulk-verify of 10 records.
  2. Query the audit log for those entities.
- **Expected result:** 10 distinct audit entries (per CONVENTIONS §16.1); each with actor + before/after for that entity. (Confirm granularity per AL C-2.)
- **Traceability:** F1.3, AL-1, **C-2**, CONVENTIONS §16.1.

#### TC-E1-F1.3-006 · Audit write is async / never blocks the user action
- [ ] **Platform:** API · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge/Performance
- **Objective:** High audit write volume does not block or fail the originating user action.
- **Preconditions:** Mutation under normal/high load.
- **Steps:** Perform a mutation; observe response latency and success even if audit write path is queued.
- **Expected result:** The user action returns promptly and succeeds; audit write is append-optimized/async and does not block the response.
- **Traceability:** F1.3, **C-1**.

#### TC-E1-F1.3-007 · Audit entries are immutable (no edit/delete)
- [ ] **Platform:** API · **POV:** Super Admin · **Priority:** P0 · **Type:** Negative/Audit
- **Objective:** No actor — even super admin — can edit or delete an audit entry.
- **Preconditions:** Existing audit entries.
- **Steps:** Attempt `PATCH`/`PUT`/`DELETE` on an audit-log entry (direct API).
- **Expected result:** Not permitted — `403`/`405`/`404`; append-only; entry unchanged.
- **Traceability:** F1.3, AL-3, US "Immutability".

### Web console · HR/Placement Admin POV (read/search)

#### TC-E1-F1.3-008 · Search audit by entity (type + id)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** HR can retrieve a specific entity's change history by entity type + id.
- **Preconditions:** A placement with multiple audited changes.
- **Steps:** Open audit search; filter by the placement's id (`GET /api/v1/audit-log?entity_type=PLACEMENT&entity_id=SWP-PL-882`).
- **Expected result:** All changes for that placement listed chronologically with actor/time/before/after.
- **Traceability:** F1.3, AL-5, US "Search by entity".

#### TC-E1-F1.3-009 · Search audit by actor, action, and time range
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Audit log is searchable by actor, action, and time range filters.
- **Preconditions:** Mixed audit data across actors/actions/dates.
- **Steps:** Filter by `actor`, by `action`, and by a date range (e.g., `?created_at__gte=2026-06-01&created_at__lte=2026-06-17`).
- **Expected result:** Results match each filter; range filters use the `__gte`/`__lte` operators per CONVENTIONS §9.
- **Traceability:** F1.3, AL-5, CONVENTIONS §9.

#### TC-E1-F1.3-010 · Audit log cursor pagination conforms to contract
- [ ] **Platform:** Web/API · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Edge/Pagination
- **Objective:** Audit-log listing uses cursor pagination (offset forbidden for this >100k table).
- **Preconditions:** ≥1 page of audit entries.
- **Steps:**
  1. `GET /api/v1/audit-log?limit=50`.
  2. Follow `next_cursor` for the next page.
  3. On the last page, verify `has_more: false` and `next_cursor: null`.
- **Expected result:** Response shape `{ data, next_cursor, has_more }`; `limit` defaults 50 / max 200; paging consistent; no offset param honored.
- **Traceability:** F1.3, AL-5, CONVENTIONS §8.

#### TC-E1-F1.3-011 · Changing sort/filter mid-cursor → CURSOR_MISMATCH
- [ ] **Platform:** API · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Error
- **Objective:** Reusing a cursor with mismatched sort/filter params is rejected.
- **Preconditions:** A valid `next_cursor` from a prior sorted query.
- **Steps:** Reissue with a different `sort`/filter but the same cursor.
- **Expected result:** `400 Bad Request` with `error.code: "CURSOR_MISMATCH"`.
- **Traceability:** F1.3, CONVENTIONS §8.

#### TC-E1-F1.3-012 · Empty / loading / error states for audit search (no dead-flow)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** Audit search renders designed states for loading, empty results, and backend error.
- **Preconditions:** (a) slow/loading network; (b) a filter that matches no entries; (c) forced backend error.
- **Steps:**
  1. Apply a filter with no matches → observe empty state.
  2. Observe loading skeleton while results load.
  3. Force a `500` and observe error state.
- **Expected result:** Empty state shows Indonesian "Tidak ada data" (or equivalent), not a blank screen; loading shows skeleton/spinner; `500` renders an error state (`error.code: "INTERNAL"`) with retry — no dead-flow.
- **Traceability:** F1.3, AL-5, F1.4 PC-3, design-system no-dead-flow rule.

### RBAC denial on audit access

#### TC-E1-F1.3-013 · Agent denied audit-log access
- [ ] **Platform:** API · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** A baseline agent cannot view the audit log.
- **Preconditions:** Active agent.
- **Steps:** Call `GET /api/v1/audit-log` with the agent's token (no audit surface exists on mobile, so this is a direct API attempt).
- **Expected result:** `403` with `error.code: "FORBIDDEN"`; no entries returned.
- **Traceability:** F1.3, AL-7, US "Access restricted".

#### TC-E1-F1.3-014 · Shift leader denied audit-log access
- [ ] **Platform:** API · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** A shift leader (non HR/super-admin elevation) cannot read the audit log.
- **Preconditions:** Active shift leader.
- **Steps:** Call `GET /api/v1/audit-log` with the leader's token.
- **Expected result:** `403 FORBIDDEN`; no audit surface in leader nav.
- **Traceability:** F1.3, AL-7, RB-5.

#### TC-E1-F1.3-015 · Super admin can read audit log (positive control)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Happy/RBAC
- **Objective:** Super admin has audit-read access (alongside HR).
- **Preconditions:** Logged-in super admin.
- **Steps:** Open audit search and run a query.
- **Expected result:** Access granted; results returned.
- **Traceability:** F1.3, AL-7.

---

## F1.4 — Platform Conventions & App Shell

> Bahasa Indonesia i18n; Asia/Jakarta canonical timezone (store UTC, render WIB); standard API error envelope + pagination + validation; role-based app shell on web + mobile. Rules PC-1…PC-6, cases C-1…C-4, INV-6.

### Web console · Super Admin POV

#### TC-E1-F1.4-001 · UI renders in Bahasa Indonesia (web)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** The web console UI is in Bahasa Indonesia by default.
- **Preconditions:** Logged-in super admin; no language override.
- **Steps:** Open several screens (nav, forms, buttons).
- **Expected result:** All labels/buttons/messages are Indonesian (e.g., "Masuk", "Simpan", "Batal"); strings externalized (i18n-ready).
- **Traceability:** F1.4, PC-1, INV-6, US "Indonesian UI".

#### TC-E1-F1.4-002 · Super admin app shell shows config/user-management nav
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Navigation reflects highest elevation: super admin sees system config + user management.
- **Preconditions:** Logged-in super admin.
- **Steps:** Inspect the sidebar/nav.
- **Expected result:** Super-admin surfaces (users, roles, config, master data) present.
- **Traceability:** F1.4, PC-4.

#### TC-E1-F1.4-003 · Money renders as IDR; dates/numbers Indonesian-formatted
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Monetary values render IDR and dates/numbers use Indonesian formatting.
- **Preconditions:** A screen showing money + dates.
- **Steps:** Inspect currency, date, and number formatting.
- **Expected result:** Money as IDR (e.g., "Rp1.500.000"); dates Indonesian format; thousands separator per ID locale.
- **Traceability:** F1.4, PC-5, US "IDR + Indonesian formatting", confirm IDR-only (§10).

### Web console · HR/Placement Admin POV

#### TC-E1-F1.4-004 · Validation error surfaces field-level errors (standard envelope)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Error
- **Objective:** A failed form validation renders field-level errors wired from the standard envelope.
- **Preconditions:** A form with required/validated fields.
- **Steps:**
  1. Submit the form with an invalid/missing field.
- **Expected result:** API returns `400`/`422` with `error.fields` mapping each field to an Indonesian message; the form shows errors inline per field (Wave-3.3 wiring); `request_id` present.
- **Traceability:** F1.4, PC-3, US "Consistent API errors", CONVENTIONS §11/§12.

#### TC-E1-F1.4-005 · end_date < start_date → INVALID_REQUEST on date pair
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** A start/end date pair with `end < start` is rejected with a field error.
- **Preconditions:** A form with a date range.
- **Steps:** Enter end date earlier than start date; submit.
- **Expected result:** `400` `INVALID_REQUEST` with `fields.end_date` set; Indonesian message.
- **Traceability:** F1.4, PC-3, CONVENTIONS §12.

#### TC-E1-F1.4-006 · HR shell shows employees/placements, hides super-admin config
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy/RBAC
- **Objective:** Role-based nav: HR sees HR surfaces, not super-admin-only config.
- **Preconditions:** Logged-in hr_admin.
- **Steps:** Inspect nav.
- **Expected result:** Employees/placements/master data visible; super-admin config absent.
- **Traceability:** F1.4, PC-4.

### Web console · Shift Leader POV

#### TC-E1-F1.4-007 · Leader navigation = roster + approvals (not config)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** A shift leader sees leader navigation, not super-admin config.
- **Preconditions:** Logged-in shift leader (active E3 assignment).
- **Steps:** Inspect the nav after login.
- **Expected result:** Roster + approvals surfaces visible; no super-admin config; nav driven by derived `shift_leader` elevation.
- **Traceability:** F1.4, PC-4, US "Role-based navigation", INV-3.

### Mobile app · Agent POV

#### TC-E1-F1.4-008 · Mobile UI in Bahasa Indonesia
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** The mobile app renders in Bahasa Indonesia.
- **Preconditions:** Logged-in agent.
- **Steps:** Open core mobile screens.
- **Expected result:** All copy Indonesian; externalized strings.
- **Traceability:** F1.4, PC-1, INV-6.

#### TC-E1-F1.4-009 · Agent mobile nav = self-service backbone only
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** A no-elevation agent gets the self-service backbone (clock-in, schedule, leave/OT, payslip), no admin/leader surfaces.
- **Preconditions:** Logged-in field agent, no elevation.
- **Steps:** Inspect `comp/AgentMobileNav`.
- **Expected result:** Only self-service tabs present; no approvals/roster/config.
- **Traceability:** F1.4, PC-4, RB-1/RB-7.

#### TC-E1-F1.4-010 · Cross-midnight / WIB rendering consistent
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Times display in WIB and cross-midnight shifts render consistently with the E4/E5 start-date rule.
- **Preconditions:** An agent with a night/cross-midnight shift.
- **Steps:** View a shift spanning midnight in the schedule.
- **Expected result:** Times shown in Asia/Jakarta (WIB, UTC+7, no DST); cross-midnight display follows the start-date rule (no off-by-one); UTC stored, WIB rendered.
- **Traceability:** F1.4, PC-2, **C-1** (no DST), **C-3** (cross-midnight), INV-6.

### Mobile app · Shift Leader POV

#### TC-E1-F1.4-011 · Leader mobile nav scoped + leader surfaces
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Shift leader on mobile sees leader nav (`comp/SLMobileNav`) for their company.
- **Preconditions:** Logged-in shift leader (active E3 assignment).
- **Steps:** Inspect mobile nav.
- **Expected result:** Leader mobile surfaces present; scoped to their company; no super-admin config.
- **Traceability:** F1.4, PC-4, INV-3.

### Cross-platform / API conventions

#### TC-E1-F1.4-012 · Canonical timezone in evaluation (lateness uses WIB, not server UTC)
- [ ] **Platform:** API · **POV:** Shift Leader · **Priority:** P0 · **Type:** Edge
- **Objective:** Time-sensitive evaluation (lateness/auto-close/period boundary) uses Asia/Jakarta, not server UTC.
- **Preconditions:** A night-shift attendance evaluated for lateness; server clock in UTC.
- **Steps:** Evaluate lateness for a shift near the UTC/WIB boundary.
- **Expected result:** Calculation uses WIB (UTC+7); result matches the Jakarta-local time, not the UTC interpretation.
- **Traceability:** F1.4, PC-2, US "Canonical timezone", INV-6.

#### TC-E1-F1.4-013 · Timestamps returned as RFC 3339 UTC; local-time fields HH:MM WIB
- [ ] **Platform:** API · **POV:** any · **Priority:** P1 · **Type:** Edge
- **Objective:** API timestamps are ISO 8601 UTC; local-time fields are `HH:MM` (WIB); server is authoritative on created/updated.
- **Preconditions:** A resource with both a timestamp and a local-time field (e.g., shift `start_time`).
- **Steps:** Read the resource; inspect timestamp and local-time formats.
- **Expected result:** `created_at` like `"2026-06-17T07:00:00Z"`; `start_time` like `"09:00"`; clients cannot write `created_at`/`updated_at`; optional `tz_offset_minutes: +420` companion.
- **Traceability:** F1.4, PC-2, CONVENTIONS §10.

#### TC-E1-F1.4-014 · Standard error envelope shape on all 4xx/5xx
- [ ] **Platform:** API · **POV:** any · **Priority:** P0 · **Type:** Error
- **Objective:** Every error response uses the standard envelope with stable `code`, Indonesian `message`, optional `fields`, and `request_id`.
- **Preconditions:** Trigger a 400, 403, 404, 422, and 500.
- **Steps:** Inspect each error body.
- **Expected result:** All conform to `{ "error": { code, message, fields?, request_id } }`; `code` UPPER_SNAKE_CASE; `message` Indonesian by default (switchable via `Accept-Language: en-US`); `fields` present only on 400/422.
- **Traceability:** F1.4, PC-3, CONVENTIONS §11.

#### TC-E1-F1.4-015 · Accept-Language switches message to en-US
- [ ] **Platform:** API · **POV:** any · **Priority:** P2 · **Type:** Edge
- **Objective:** Error `message` localizes to English when `Accept-Language: en-US` is sent (i18n-ready; default ID).
- **Preconditions:** An endpoint that returns a 4xx.
- **Steps:** Send a failing request with `Accept-Language: en-US`.
- **Expected result:** `error.message` returned in English; `error.code` unchanged.
- **Traceability:** F1.4, PC-1, **C-4** (future second language), CONVENTIONS §11.

#### TC-E1-F1.4-016 · Unknown filter / cursor-mismatch error codes
- [ ] **Platform:** API · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Error
- **Objective:** Filtering on a non-existent field returns `UNKNOWN_FILTER`; pagination conventions hold.
- **Preconditions:** A list endpoint.
- **Steps:** Apply a filter operator on a non-existent field (e.g., `?nope__gte=1`).
- **Expected result:** `400` with `error.code: "UNKNOWN_FILTER"`.
- **Traceability:** F1.4, PC-3, CONVENTIONS §9.

#### TC-E1-F1.4-017 · Missing translation string falls back to key/default
- [ ] **Platform:** Web/Mobile · **POV:** any · **Priority:** P2 · **Type:** Edge
- **Objective:** A missing i18n key does not blank the UI — it falls back to a key/default and is flagged in dev.
- **Preconditions:** Build with a deliberately missing string (dev/test).
- **Steps:** Open the screen referencing the missing key.
- **Expected result:** Fallback to a default/key string rather than empty; dev build flags the missing key.
- **Traceability:** F1.4, PC-1, **C-2**.

#### TC-E1-F1.4-018 · Rate-limit headers present on responses
- [ ] **Platform:** API · **POV:** any · **Priority:** P2 · **Type:** Edge
- **Objective:** Responses carry rate-limit headers; 429 returns the standard envelope + Retry-After.
- **Preconditions:** Authenticated requests.
- **Steps:** Inspect response headers; exceed the per-user limit to force a 429.
- **Expected result:** `X-RateLimit-Limit/Remaining/Reset` on every response; `429` body uses `error.code: "RATE_LIMITED"` with a `Retry-After` header.
- **Traceability:** F1.4, PC-3, CONVENTIONS §19.

#### TC-E1-F1.4-019 · 401 mid-session renders session-expired pattern (no dead-flow)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Error
- **Objective:** A mid-session `401` renders the session-expired pattern + re-auth flow, not a raw error.
- **Preconditions:** Logged-in HR; force token expiry/epoch bump.
- **Steps:** Trigger an authenticated request after the token is invalidated.
- **Expected result:** Client renders `comp/EmptySessionExpired` + re-auth flow; after re-login, the user resumes.
- **Traceability:** F1.4, PC-3, CONVENTIONS §3 (session-expired UX), no-dead-flow.

#### TC-E1-F1.4-020 · 403 renders no-permission state (no dead-flow)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Error/RBAC
- **Objective:** A `403` from the API renders the no-permission empty state, not a crash.
- **Preconditions:** Force a `403` (out-of-scope or forbidden action).
- **Steps:** Attempt the forbidden action.
- **Expected result:** Client renders `comp/EmptyNoPermission`; no crash; Indonesian copy.
- **Traceability:** F1.4, PC-3, CONVENTIONS §3, RB-2.

---

## Appendix — Open items affecting test execution

These are unresolved in the PRDs; record actual configured values when executing:
- **Password policy / lockout thresholds** — deferred (F1.1 §10). TC-E1-F1.1-015 uses the configured threshold.
- **Session token lifetimes** (access ~15–60 min; refresh) — deferred (F1.1 §10 / CONVENTIONS §3). TC-E1-F1.1-012/017 depend on these.
- **Audit retention / archival** — deferred (F1.3 §10). Not directly testable in v1 beyond presence of entries.
- **Bulk-audit granularity** (per-row vs summarized) — confirm against TC-E1-F1.3-005 (AL C-2).
- **hr_admin role-assignment permission** — FEATURE §7 resolves "super_admin and hr_admin may assign roles"; the F1.2 PRD §10 still lists it open. TC-E1-F1.2-005 follows FEATURE §7 (authoritative per EPICS §8) and flags any build that restricts to super_admin only.
- **IDR-only / no multi-currency** — confirm (F1.4 §10) per TC-E1-F1.4-003.
