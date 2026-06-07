# PRD · F2.7 — Employee Offboarding & Session Revocation

> **Epic:** E2 Identity, Org & Master Data · **Feature:** F2.7 · **Status:** Draft v1
> **Parent:** [FEATURE.md](../FEATURE.md) · **Owner:** _TBD_

---

## 1. Context & problem

When an agent's relationship with **SWP** ends, three things must happen together: the **employment agreement** (F2.2) closes, the **employee** record goes inactive (F2.1, EP-7), and — critically — the person's **login can no longer be used**. Today only the first two are modelled; deactivation flips a status flag and the linked login keeps working until the access token expires naturally. For an **outsourcing provider whose staff sit on client premises**, a terminated agent retaining a valid session is a real security and compliance gap (they can still clock in, view schedules, see client data).

This PRD owns **offboarding** — the deliberate end of the SWP↔agent employment relationship — and the **session revocation** that must accompany it. It draws a hard line the legacy system never did:

- **Offboarding ≠ placement-end.** Ending a *placement* (E3) only ends a work *designation*; the agent stays employed by SWP and may be re-placed (transfer, renewal). Placement-end **never** revokes a login.
- **Offboarding = employment-end.** Closing the *employment agreement* ends the relationship with SWP. **This, and only this, revokes the login.**

It also resolves the Phase-2/Phase-4 deferral recorded across the planning notes ("session revocation on deactivate — out of scope / deferred"): revocation is now **in scope and specified here**.

## 2. Goals & non-goals

**Goals**
- One authoritative **offboarding action** that atomically closes the agreement, deactivates the employee, and revokes the login.
- Cover **every distinct separation case** (contract end, non-renewal, resignation, termination/PHK, retirement, death, absconding) with a captured **reason**.
- **Instant** session revocation — a revoked agent cannot make an authenticated request, not "eventually after the token expires".
- A **deliberate decision point** for contract expiry: the system *flags*, HR *decides* (continue vs end). Nothing auto-terminates employment.
- A **grace** posture: when a PKWT lapses without an HR decision, the agent keeps access until HR acts — no surprise lockouts from a midnight job.

**Non-goals**
- The placement state machine (E3 F3.2) — offboarding *consumes* placement terminal states, it doesn't redefine them.
- Auth primitives themselves (token format, refresh rotation) — owned by E1; this PRD specifies the **revocation hook** E1 must expose (§6, OB-9).
- Payroll/final-pay settlement (E8, out of scope v1).
- Re-onboarding / rehire of a previously offboarded person beyond reactivation (treated as a fresh employment agreement).

## 3. Actors

- **HR / Placement Admin** — initiates offboarding; makes the contract-expiry decision (continue/end).
- **Super Admin** — same, plus corrections on terminal records.
- **System** (scheduled job, **Asia/Jakarta**) — flags expiring agreements, raises the decision task; **never** ends employment or revokes a login on its own.
- **Agent** — subject; loses access on offboard; notified.

## 4. Platform / clients

| Surface | Who | What |
|---|---|---|
| **Web console** | HR / Super Admin | Initiate offboarding (resign / terminate / death / etc.); act on the contract-expiry decision task; view offboarding history + reason. |
| **Inbox** (web) | HR / Super Admin | Receives the "contract expiring — decide" task (continue vs end). |
| **Mobile app** | Agent | None directly; a revoked agent is signed out on the next request and shown a re-login screen with a "no active employment" message. |

## 5. Business rules

| Ref | Rule |
|-----|------|
| **OB-1** | An **offboard** is the atomic end of employment: it closes the active `EmploymentAgreement` (status `closed` + `closed_reason` + `closed_at`), sets the `Employee.status = inactive`, ends any non-terminal `Placement` (E3), disables the linked `User`, and **revokes all of that user's sessions** (OB-9). All in one transaction; all audited (OB-12). |
| **OB-2** | Offboarding is **employment-level**. Placement-only transitions — `Transferred`, `Superseded` (renewal), and a placement auto-ending while the agreement stays active — **do not** offboard and **do not** revoke the login. (Reinforces the E2/E3 domain split; corrects the implication that any placement-end deactivates.) |
| **OB-3** | **`closed_reason` ∈ {`END_OF_TERM`, `RESIGNED`, `TERMINATED`, `RETIRED`, `DECEASED`, `ABSCONDED`, `OTHER`}.** `TERMINATED` and `ABSCONDED` (mangkir) require a free-text reason; `OTHER` requires a note. *(Extends the prior 4-value set `END_OF_TERM`/`RESIGNED`/`TERMINATED`/`OTHER` — see §10.)* |
| **OB-4** | **Trigger Class A — expiry-driven (flag → decide).** A `PKWT` agreement nearing `end_date` (**30 days**, Asia/Jakarta) is flagged **`expiring`** and a decision task is raised to HR. HR must choose: **Continue** → renew (new agreement + placement via F2.2 EA-3 / F3.2 LC-7; **no** offboard), or **End** → offboard with `closed_reason = END_OF_TERM` (or a more specific reason). `PKWTT` (open-ended) **never** enters `expiring`. |
| **OB-5** | **Trigger Class B — event-driven (HR/manual).** HR initiates offboarding directly for `RESIGNED`, `TERMINATED`, `RETIRED`, `DECEASED`, `ABSCONDED`. Each requires a **reason** (per OB-3) and an **effective date** (may be future-dated for resignation — last working day; see OB-7). |
| **OB-6** | **Grace — no auto-offboard.** If a flagged `expiring` PKWT reaches and passes `end_date` with **no HR decision**, the agreement stays `expiring`, the **login stays valid**, and the decision task **escalates** (re-notify). Employment ends only by an explicit HR **End** action (OB-4) or an event (OB-5). The system never closes an agreement or revokes a login on a timer. |
| **OB-7** | **Effective-dated offboarding.** When `effective_date` is **today or past**, revocation (OB-9) fires immediately on submit. When it is **future** (e.g. a resignation last-working-day), the agreement is marked to close and the login to revoke **at** `effective_date` (scheduled job); until then the agent retains access. The pending offboard is visible and cancellable before it fires. |
| **OB-8** | **Reactivation is the inverse, and bounded.** Reactivating an offboarded employee (F2.1 C-3) re-enables the login but requires a **new active agreement** (you cannot reactivate into a closed agreement). Sessions are **not** restored — the agent re-authenticates fresh. |
| **OB-9** | **Revocation is instant and complete.** Revoking a user (a) bumps a **session epoch** so every already-issued access token for that user is rejected on its next use, and (b) revokes all live refresh tokens (`RevokeAllRefreshForUser`). After revocation no token minted before the revocation instant is honoured. *(E1 dependency — mechanism in §6.)* |
| **OB-10** | **Authorization.** Only **HR/Placement Admin** and **Super Admin** may offboard or act on the decision task. A shift leader or agent **cannot**. Terminal/already-offboarded records are immutable except by Super Admin override. |
| **OB-11** | **Idempotency & conflicts.** Offboarding an already-inactive employee, or one with no active agreement, returns **409 CONFLICT** (nothing to close). Acting on an already-decided expiry task returns **409**. |
| **OB-12** | **Audit.** Every offboard, expiry-decision, future-dated schedule/cancel, and revocation writes an audit entry (actor or `system`, before/after, `reason`) per E1. The reason and effective date are part of the record. |
| **OB-13** | **Dependency gate.** Every employee has a linked `User` (F2.1 EP-3 / EP-4 — login is auto-provisioned at create, 1:1 non-null), so revocation **always applies**: offboarding closes the agreement, deactivates the employee, and revokes the login in the same transaction (OB-1). There is no data-only no-op branch — the revocation step always runs. |

## 6. Data model & mechanism

**Employment agreement** (F2.2) — `closed_reason` enum extended per OB-3; add an `expiring` signal:

| Field | Type | Notes |
|-------|------|-------|
| `status` | enum | `active` \| `expiring` \| `superseded` \| `closed` — **`expiring` is new** (PKWT within 30d of `end_date`). |
| `closed_reason` | enum | `END_OF_TERM` \| `RESIGNED` \| `TERMINATED` \| `RETIRED` \| `DECEASED` \| `ABSCONDED` \| `OTHER` (null while open). **Extended** (§10). |
| `closed_at` | date | effective end (OB-7). |
| `close_note` | text | required for `OTHER`; carries the free-text cause for `TERMINATED`/`ABSCONDED`. |

**Offboarding record** (new, append-only) — captures intent independent of the agreement row:

`Offboarding`: `id, employee_id (FK), employment_agreement_id (FK), reason (enum, OB-3), note, effective_date, initiated_by, initiated_at, status (pending | applied | cancelled), applied_at`.

> `status = pending` covers the future-dated window (OB-7); the job flips it to `applied` and fires revocation at `effective_date`.

**Session revocation mechanism (E1 — `users`):**

```sql
-- E1 migration (driven by this PRD): session epoch for instant revocation
ALTER TABLE users ADD COLUMN tokens_valid_after timestamptz NOT NULL DEFAULT '-infinity';
```

- **Revoke** = `SetUserStatus('disabled')` **+** `tokens_valid_after = effective_instant` **+** `RevokeAllRefreshForUser(user_id)` (the last primitive already exists).
- **Auth middleware** gains a per-request user lookup (it was purely stateless JWT verification): after verifying the EdDSA signature, reject **401** if `user.status != 'active'` **OR** `token.iat < user.tokens_valid_after`. At SWP scale (internal staff only) this is one indexed PK read per request; an in-memory short-TTL cache is an optional later optimisation.
- This makes the otherwise-unrevocable stateless access token instantly revocable **without** a per-token denylist or jti tracking: a single epoch bump invalidates every token issued before it.

**Contract-expiry decision task (Inbox):** raised by the daily job (OB-4) for each PKWT entering `expiring`; payload = employee, agreement, `end_date`, days-remaining; actions = **Continue** (→ renewal flow) / **End** (→ offboard). Escalates per OB-6.

## 7. Acceptance criteria (Gherkin)

```gherkin
Feature: Employee offboarding & session revocation

  Scenario: Terminate for cause revokes the session immediately
    Given the agent "Budi" has an active agreement and a working login
    When an HR admin offboards him with reason "TERMINATED" and a documented cause, effective today
    Then his employment agreement is closed with reason "TERMINATED"
    And his employee status becomes inactive
    And his non-terminal placement is ended
    And his user login is disabled and all his sessions are revoked
    And his next authenticated request is rejected with 401

  Scenario: Placement transfer does not revoke the login
    Given "Budi" is actively placed at "Plaza Senayan"
    When an HR admin transfers him to "Grand Indonesia"
    Then his old placement becomes "Transferred" and a new one opens
    And his employment agreement stays active
    And his login keeps working

  Scenario: PKWT nearing end raises a decision, not a termination
    Given "Budi" has a PKWT agreement ending in 30 days
    When the expiry job runs in Asia/Jakarta time
    Then his agreement status becomes "expiring"
    And a contract-expiry decision task is raised to HR
    And his login keeps working

  Scenario: HR chooses to end an expiring contract
    Given "Budi" has an expiring PKWT and an open decision task
    When an HR admin chooses "End" with reason "END_OF_TERM"
    Then he is offboarded and his sessions are revoked
    And the decision task is closed

  Scenario: Grace — lapsed PKWT keeps access until HR decides
    Given "Budi" has an expiring PKWT whose end date passed yesterday with no decision
    When the daily job runs
    Then his agreement remains "expiring"
    And his login still works
    And the decision task is escalated

  Scenario: Open-ended PKWTT never auto-expires
    Given "Siti" has an active PKWTT with no end date
    When the expiry job runs
    Then her agreement remains "active"
    And no decision task is raised

  Scenario: Future-dated resignation revokes on the last working day
    Given "Budi" resigns effective the end of next month
    When an HR admin records the resignation with that effective date
    Then a pending offboarding is recorded
    And his login keeps working until that date
    And on that date his agreement closes "RESIGNED" and his sessions are revoked

  Scenario: Death is recorded and access removed
    When an HR admin offboards "Budi" with reason "DECEASED" and a note
    Then his agreement closes with reason "DECEASED"
    And his login is disabled and sessions revoked

  Scenario: Cannot offboard an already-inactive employee
    Given "Budi" is already inactive
    When an HR admin tries to offboard him
    Then the action is rejected with 409 CONFLICT

  Scenario: Every offboard revokes the login
    Given "Eka" has an active agreement and a working login
    When an HR admin offboards her with reason "RESIGNED" effective today
    Then her agreement closes and her status becomes inactive
    And her user login is disabled and all her sessions are revoked
```

## 8. Cases & edge cases

| # | Case | Expected |
|---|------|----------|
| C-1 | Mangkir (unauthorized absence past threshold) | HR offboards with `ABSCONDED` + note; placement → `Terminated`; sessions revoked. No automatic threshold-trigger in v1 (HR-initiated). |
| C-2 | Retirement at mandatory age | HR offboards with `RETIRED`; treated like an end-of-relationship; sessions revoked. Age threshold is policy/config, not enforced in v1. |
| C-3 | Future-dated resignation, then agent reconsiders before the date | The `pending` offboarding is **cancellable** (OB-7); agreement stays active, no revocation fired. |
| C-4 | Offboard fires while the agent is mid-session on mobile | Next request after the revocation instant returns 401; the app routes to a "no active employment" re-login screen (no dead-flow). |
| C-5 | Access token issued seconds before revocation | Rejected — `token.iat < tokens_valid_after` (OB-9). The short access-token window is closed by the epoch, not left to expiry. |
| C-6 | Reactivate an offboarded employee | Requires a **new** active agreement (OB-8); login re-enabled or re-invited; old sessions stay dead. |
| C-7 | Expiry job missed a day (downtime) | Catch-up safe (same posture as F3.2 C-6): evaluates all PKWTs due by date. Because there is no auto-offboard (OB-6), a missed day only delays the *flag*, never causes an unintended termination. |
| C-8 | Employee inactive but agreement somehow still `active` (legacy/migration drift) | Reconciliation: surfaced in E9 review queue; offboarding treats the agreement as the source of truth and closes it. |
| C-9 | Super Admin corrects a wrong `closed_reason` after offboard | Allowed (override); re-audited; does **not** re-issue sessions. |
| C-10 | Two HR admins act on the same expiry task concurrently | First wins; the second gets 409 (OB-11). |

## 9. Dependencies

- **F2.1** (employee status, login provisioning EP-3, reactivation C-3) — offboarding deactivates the employee; every employee has a linked `User` (1:1 non-null, auto-provisioned at create), so revocation always runs.
- **F2.2** (employment agreement) — extends its `status` (`expiring`) and `closed_reason` enum; consumes EA-5 (close-with-reason) and EA-3 (renewal as the "Continue" path).
- **E3 F3.2** (placement lifecycle) — offboarding ends the non-terminal placement; transfer/renewal are the explicitly *non*-offboarding paths (OB-2).
- **E1** — exposes the revocation hook: `users.tokens_valid_after` epoch, the middleware status/epoch check, and the existing `RevokeAllRefreshForUser`. Supersedes the E1 "revoke on disable" scenario's deferral and the AU-6 promise (now realised here).
- **E10** — Inbox decision task + notifications (expiry flag, escalation, offboard confirmation).
- **A scheduled job runner** (platform, Asia/Jakarta) for OB-4 flagging, OB-6 escalation, OB-7 future-dated firing.

## 10. Decisions & open questions

**Resolved (2026-06-06):**
- ✅ **Grace, not auto-offboard** — a lapsed PKWT keeps the login alive until HR explicitly ends it (OB-6). Avoids surprise lockouts; the cost is HR must act on the decision task.
- ✅ **Instant revocation via session epoch** (`users.tokens_valid_after`) + refresh revocation, with a per-request user check in middleware (OB-9, §6). Chosen over eventual/refresh-only (a terminated-for-cause agent must lose access *now*, not at token expiry) and over a jti denylist (epoch is simpler, no per-token state, no Redis — fits the River/no-Redis stack).
- ✅ **Extended reason enum** — add `RETIRED`, `DECEASED`, `ABSCONDED` (mangkir) to `closed_reason` (OB-3) for reporting fidelity; `OTHER` remains the catch-all with a required note.
- ✅ **Offboarding is employment-level, separate from placement-end** (OB-2) — encodes the E2/E3 domain split into the revocation rule.
- ✅ **Future-dated offboarding** supported and cancellable (OB-7) for resignation last-working-day.

**Open:**
- **Auto-mangkir:** should N consecutive unexcused absences (E5) auto-raise an `ABSCONDED` offboarding task? (v1: manual, HR-initiated — C-1.)
- **Retirement age:** configurable mandatory-retirement threshold that auto-flags? (v1: not enforced — C-2.)
- **Final-pay / settlement** handoff to E8 on offboard — out of scope v1; confirm the integration point when E8 payroll lands.
- **Middleware cache:** per-request `users` read vs short-TTL in-memory cache — start with the direct read; revisit if it shows up in latency profiling.
```
