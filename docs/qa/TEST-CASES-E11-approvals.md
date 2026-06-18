# Test Cases — E11 Approvals

> **Epic:** E11 Approvals (cross-cutting configurable approval engine) · **Status:** Manual QA test-case suite v1 · **Date authored:** 2026-06-17
> **Sources:** [FEATURE.md](../epics/E11-approvals/FEATURE.md) · [F11.1 approval-template-management](../epics/E11-approvals/prds/approval-template-management.md) · [F11.2 approval-execution](../epics/E11-approvals/prds/approval-execution.md) · [F11.3 approval-inbox](../epics/E11-approvals/prds/approval-inbox.md) · [api/CONVENTIONS.md](../api/CONVENTIONS.md)

---

## 1. Scope

This document is an **exhaustive manual-testing suite** for E11 — the generic, per-company approval engine that routes any request type (leave F6.2, overtime F7.2, …) through an HR-configured chain of 2–3 ordered lines. It covers all three features:

- **F11.1 — Approval Template Management** (web-only; HR / Super Admin configure a per-company template of 2–3 ordered OR-set lines; saving re-bases pending instances, INV-6).
- **F11.2 — Approval Execution Engine** (instance lifecycle: sequential line advance on OR-approve, terminal reject, super-admin bypass, no-template fallback, idempotency, side-effect hooks).
- **F11.3 — Approval Inbox** (web + mobile "needs my decision" queue, request-detail chain timeline).

Cases are organized **per platform (Web / Mobile) × per POV (super admin · HR/placement admin · shift leader · agent)**, derived from each PRD's Actors and Platform/clients tables.

**Key domain facts honored throughout:**
- Routing is by **line membership**, not role (EX-2). A "line member" can be any active SWP staff user (shift leader, lead, HR, super admin).
- **Agents** are almost always **requesters** (cannot self-approve, INV-3); they appear as line members only if explicitly assigned — and even then self-approval is blocked.
- **Template management is web-only** (no mobile surface). **Bypass is web-only** and is a super-admin action, not a line-member inbox action (F11.2 / F11.3 non-goal).
- **Profile change-request approval is removed** — no test cases for it (out of scope; profile edits are instant self-edit per E2).
- Error envelope, `409` for INV-* / conflicts, `403` permission, `Idempotency-Key`, cursor pagination, `comp/Empty*` states per [CONVENTIONS.md](../api/CONVENTIONS.md).

**Test data baseline** (reused across cases unless overridden):
- Company **"Plaza Senayan"** (`SWP-CMP-12`) template `SWP-APT-1`, version 1: **Line 1 = [Rudi, Sari]**, **Line 2 = [Sari Hadi]**.
- Company **"Baru Jaya"** (`SWP-CMP-30`) has **no template** (fallback).
- **Budi** = agent at Plaza Senayan (requester). **Rudi** = shift leader (line-1 member). **Sari** = lead (line-1 member). **Sari Hadi** = HR (line-2 member). **Pak Super** = super admin.

---

## 2. Coverage matrix

| Feature | Web · Super Admin | Web · HR/Placement Admin | Web · Shift Leader | Web · Agent | Mobile · Super Admin | Mobile · HR Admin | Mobile · Shift Leader | Mobile · Agent |
|---|---|---|---|---|---|---|---|---|
| **F11.1 Template Management** | ✅ CRUD + fallback | ✅ CRUD | ⛔ 403 (RBAC) | ⛔ 403 (RBAC) | n/a (web-only) | n/a | n/a | n/a |
| **F11.2 Execution Engine** | ✅ bypass, finalize, fallback | ✅ act (if member), reset trigger | ✅ act (current-line member) | ◻ requester view / self-block | ◻ bypass n/a (web-only) | ✅ act (if member) | ✅ act (current-line member) | ◻ requester view / self-block |
| **F11.3 Inbox + timeline** | ✅ list/act + bypass-separate | ✅ list/act | ✅ list/act | ◻ no inbox (requester only) | ✅ list/act | ✅ list/act | ✅ list/act (phone-first) | ◻ requester status view |

Legend: ✅ primary tested surface · ◻ secondary / view-only / negative · ⛔ RBAC-denied (tested as negative) · n/a not applicable.

**BR / Case coverage:**
- F11.1: TM-1…TM-8, C-1…C-6, INV-1, INV-3, INV-J.
- F11.2: EX-1…EX-11, C-1…C-6, INV-2…INV-9.
- F11.3: IB-1…IB-7, C-1…C-5, INV-3.

---

## F11.1 — Approval Template Management

> Web console only (no mobile). Authors: HR/Placement Admin + Super Admin (`approvals.template.manage`). See [PRD](../epics/E11-approvals/prds/approval-template-management.md).

### Web · HR/Placement Admin POV

#### TC-E11-F11.1-001 · Create a valid two-line template
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** HR creates the minimum valid template (2 ordered OR-set lines) for a company with none.
- **Preconditions:** Logged in as HR admin with `approvals.template.manage`. "Plaza Senayan" (`SWP-CMP-12`) has **no** template. Rudi, Sari, Sari Hadi are active SWP staff.
- **Steps:**
  1. Open Settings/Klien → Plaza Senayan → Approval Template.
  2. Add Line 1, assign members [Rudi, Sari].
  3. Add Line 2, assign member [Sari Hadi].
  4. Save.
- **Expected result / Acceptance criteria:** Template `SWP-APT-*` created for Plaza Senayan at **version 1**; success toast; Line 1 shows OR-set [Rudi, Sari], Line 2 [Sari Hadi]. No dead-flow (toast + persisted view).
- **Traceability:** F11.1, TM-1, TM-2, TM-4, TM-8, INV-1, US Gherkin "Create a two-line template".

#### TC-E11-F11.1-002 · Add optional third line
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Configure the optional 3rd line (typically a super-admin sign-off).
- **Preconditions:** Plaza Senayan has a 2-line template (TC-001).
- **Steps:**
  1. Open the template editor.
  2. Add Line 3, assign member [Pak Super].
  3. Save.
- **Expected result:** Template now has **3 ordered lines** (`line_no` 1,2,3); version bumped to 2; audit recorded. Line 3 must be satisfied at execution (INV-2).
- **Traceability:** F11.1, TM-2, INV-2, Gherkin "Optional third line".

#### TC-E11-F11.1-003 · Reorder lines
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Reordering changes `line_no` sequence and bumps version.
- **Preconditions:** 3-line template exists.
- **Steps:**
  1. Open editor; drag Line 3 above Line 2 (reorder).
  2. Save.
- **Expected result:** Persisted order reflects new `line_no` 1..3; version bumped; audit recorded; pending instances reset to line 1 on new version (INV-6, verify in F11.2-016).
- **Traceability:** F11.1, TM-2, TM-6, INV-6.

#### TC-E11-F11.1-004 · Reject fewer than two lines
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Minimum is 2 lines; a single-line template is rejected.
- **Preconditions:** Editing a company template; only Line 1 configured.
- **Steps:**
  1. Add only Line 1 with [Rudi].
  2. Save.
- **Expected result:** Save blocked with `INVALID_REQUEST` ("minimum two lines"); inline form error; nothing persisted; no version bump.
- **Traceability:** F11.1, TM-2, Gherkin "Reject fewer than two lines".

#### TC-E11-F11.1-005 · Reject empty line (no members)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Each line requires ≥1 member.
- **Preconditions:** Editing; Line 1 = [Rudi], Line 2 = empty.
- **Steps:**
  1. Leave Line 2 with no members.
  2. Save.
- **Expected result:** `422 APPROVAL_LINE_INVALID` scoped to Line 2 (field-level error on that line); save blocked.
- **Traceability:** F11.1, TM-3, Gherkin "Reject an empty or inactive-member line".

#### TC-E11-F11.1-006 · Reject inactive / offboarded member
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Members must be active SWP staff (employment not ended, INV-J).
- **Preconditions:** Editing; Line 2 includes a user whose employment has ended.
- **Steps:**
  1. Assign an offboarded user to Line 2.
  2. Save.
- **Expected result:** `422 APPROVAL_LINE_INVALID` on Line 2 (inactive member); save blocked; member picker should ideally not surface inactive users, but server enforces.
- **Traceability:** F11.1, TM-3, INV-J, Gherkin "Reject an empty or inactive-member line".

#### TC-E11-F11.1-007 · Editing re-bases pending instances (INV-6)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Edge
- **Objective:** Saving an edit bumps version and resets all non-terminal instances to line 1 on the new chain; old decisions retained as audit but not counted.
- **Preconditions:** Plaza Senayan has pending leave + OT instances mid-chain (e.g. Budi's leave at line 2).
- **Steps:**
  1. Edit the template (change a member or reorder).
  2. Save.
- **Expected result:** Version bumped; every non-terminal Plaza Senayan instance resets to `current_line = 1` on the new `template_version`; new line-1 members notified; prior `approval_actions` rows remain in the audit/timeline but no longer count toward clearing.
- **Traceability:** F11.1, TM-6, INV-6, EX-8, Gherkin "Editing re-bases pending requests".

#### TC-E11-F11.1-008 · Same user on multiple lines allowed
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** A user may appear on lines 1 and 2 (TM-4 / C-2).
- **Preconditions:** Editing a template.
- **Steps:**
  1. Add Sari to both Line 1 and Line 2.
  2. Save.
- **Expected result:** Save succeeds; each line satisfied independently at execution; no validation error.
- **Traceability:** F11.1, TM-4, C-2.

#### TC-E11-F11.1-009 · Sole-member line equal to likely requester — soft warn
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Saving a line whose only member is likely to be a requester warns but does not block (v1: warn only).
- **Preconditions:** Editing; Line 1 has exactly one member who is a likely requester.
- **Steps:**
  1. Configure Line 1 = single member.
  2. Save.
- **Expected result:** Save **succeeds** with a soft warning ("this line is self-blockable; only super-admin bypass can clear it if the member is the requester"); persisted; at execution it self-blocks (INV-3) and clears only by bypass.
- **Traceability:** F11.1, C-1, INV-3, INV-5.

#### TC-E11-F11.1-010 · Edit during a burst of submissions (transactional reset)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Reset runs in the save transaction; instances created after save are on the new version, those before are reset.
- **Preconditions:** Multiple leave/OT requests being submitted for Plaza Senayan concurrently with an edit.
- **Steps:**
  1. Submit a flurry of requests while saving a template edit.
  2. Inspect resulting instance versions.
- **Expected result:** No instance straddles versions inconsistently; pre-save instances reset to line 1 on the new version; post-save instances created directly on the new version. No instance lost.
- **Traceability:** F11.1, TM-6, C-4, INV-6.

#### TC-E11-F11.1-011 · Field validation / loading / error states
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Editor surfaces loading skeleton, member-picker search, and server-error states with no dead flow.
- **Preconditions:** Open the template editor.
- **Steps:**
  1. Observe initial load (skeleton).
  2. Search the member picker for an active user.
  3. Force a save server error (e.g. network 500).
- **Expected result:** Loading skeleton shows; picker returns active users; on save error a retryable error toast/banner appears, form state preserved, nothing partially persisted.
- **Traceability:** F11.1, TM-8, ENGINEERING no-dead-flow.

### Web · Super Admin POV

#### TC-E11-F11.1-012 · Super admin creates/edits a template
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Happy / RBAC
- **Objective:** Super admin (also holding `approvals.template.manage`) has full template CRUD parity with HR.
- **Preconditions:** Logged in as Pak Super.
- **Steps:**
  1. Open any company's template editor.
  2. Create/edit a valid 2–3 line template.
  3. Save.
- **Expected result:** Same behavior as HR (TC-001/002); version bump + audit; super admin may also be assigned as a line member (e.g. Line 3).
- **Traceability:** F11.1, TM-5, Actors (Super Admin = author).

#### TC-E11-F11.1-013 · Delete template reverts to super-admin fallback
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Happy / Edge
- **Objective:** Deleting a template reverts the company to the no-template super-admin fallback; pending instances reset accordingly.
- **Preconditions:** Plaza Senayan has a template and pending instances mid-chain.
- **Steps:**
  1. Open the template editor; choose Delete.
  2. Confirm.
- **Expected result:** Template removed (`SWP-APT-*` deleted/soft-deleted); company now uses the implicit super-admin fallback (INV-7); pending instances reset to the single super-admin line (TM-6/TM-7); audit recorded; new fallback approvers (super admins) notified.
- **Traceability:** F11.1, TM-7, C-5, INV-7, INV-6.

#### TC-E11-F11.1-014 · Company with leader_scope = site uses company-level template
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Site-scoped companies still get exactly one company-level template in v1.
- **Preconditions:** A company with `leader_scope = site`.
- **Steps:**
  1. Open its approval template editor.
- **Expected result:** Only **one** company-level template editable (INV-1); no per-site template UI offered (out of scope v1).
- **Traceability:** F11.1, C-6, INV-1.

#### TC-E11-F11.1-015 · One-template-per-company enforced (INV-1)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** A company cannot have two templates; re-creating edits the existing one.
- **Preconditions:** Plaza Senayan already has a template.
- **Steps:**
  1. Attempt to create a second template for Plaza Senayan (e.g. via a stale create form / direct API).
- **Expected result:** Engine treats it as an **edit** of the single template, or rejects with `409 INV_1_VIOLATION` (unique `company_id`); never two rows.
- **Traceability:** F11.1, TM-1, INV-1, CONVENTIONS §7 (409 for INV).

### Web · Shift Leader POV

#### TC-E11-F11.1-016 · Shift leader cannot manage templates (403)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** A shift leader lacking `approvals.template.manage` cannot create/edit/delete a template.
- **Preconditions:** Logged in as Rudi (shift leader, no `approvals.template.manage`).
- **Steps:**
  1. Attempt to open Settings/Klien → company → Approval Template (nav should be hidden).
  2. Force the route/API directly.
- **Expected result:** Nav entry not shown (client RBAC defense-in-depth); direct access returns `403 Forbidden` and renders `comp/EmptyNoPermission`. Server is the gate.
- **Traceability:** F11.1, TM-5, Gherkin "Non-authorized user cannot manage templates", CONVENTIONS §3 (403).

### Web · Agent POV

#### TC-E11-F11.1-017 · Agent cannot access template management (403)
- [ ] **Platform:** Web · **POV:** Agent · **Priority:** P1 · **Type:** RBAC
- **Objective:** Agents have no template-management surface.
- **Preconditions:** Logged in as Budi (agent).
- **Steps:**
  1. Attempt to reach the template editor route/API.
- **Expected result:** `403 Forbidden` + `comp/EmptyNoPermission`; nav entry absent. (Agents are requesters, not template authors.)
- **Traceability:** F11.1, TM-5, Actors.

---

## F11.2 — Approval Execution Engine

> Engine lifecycle: instance creation, sequential OR-advance, terminal reject, super-admin bypass (web-only), no-template fallback, idempotency, side-effect hooks. Acting happens via Inbox (F11.3) or domain approval tab; cases here focus on engine semantics. See [PRD](../epics/E11-approvals/prds/approval-execution.md).

### Web · Shift Leader POV (current-line member acting)

#### TC-E11-F11.2-001 · Sequential OR advance to final approval
- [ ] **Platform:** Web · **POV:** Shift Leader (line-1 member) · **Priority:** P0 · **Type:** Happy
- **Objective:** A line clears on the first member's APPROVE (OR), then the chain advances line-by-line to APPROVED.
- **Preconditions:** Plaza Senayan template L1[Rudi,Sari], L2[Sari Hadi]. Budi submitted a 3-day leave → instance `SWP-APV-*` PENDING at line 1.
- **Steps:**
  1. As Rudi, open the instance and Approve line 1.
  2. As Sari Hadi, open the instance and Approve line 2.
- **Expected result:** After step 1, `current_line` → 2 (Rudi cleared L1; Sari need not act); an `APPROVE` action appended (line_no 1, template_version 1). After step 2, `status = APPROVED`; leave `OnApproved` hook fires once in the same transaction (quota committed + schedule integrated); requester Budi notified.
- **Traceability:** F11.2, EX-3, EX-7, INV-2, INV-8, Gherkin "Sequential OR advance".

#### TC-E11-F11.2-002 · OR within a line — second member need not act
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Once any one line member approves, the other members' action is unnecessary and the item leaves their queue.
- **Preconditions:** Instance at line 1 [Rudi, Sari].
- **Steps:**
  1. As Rudi, Approve line 1.
  2. As Sari, refresh.
- **Expected result:** Line 1 satisfied by Rudi alone; Sari sees the item gone from her current-line queue (now at line 2).
- **Traceability:** F11.2, EX-3, INV-2, F11.3 IB-3.

#### TC-E11-F11.2-003 · Reject is terminal
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy/Negative
- **Objective:** Any current-line member's REJECT (reason required) terminates the instance and fires OnRejected.
- **Preconditions:** Instance PENDING at line 1.
- **Steps:**
  1. As Rudi, choose Reject; submit with reason "Insufficient coverage".
- **Expected result:** `status = REJECTED` (terminal); `REJECT` action appended with reason; leave `OnRejected` hook fires; Budi notified; chain stops (line 2 never reached).
- **Traceability:** F11.2, EX-5, INV-4, Gherkin "Reject is terminal".

#### TC-E11-F11.2-004 · Reject without reason blocked
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Negative
- **Objective:** Reason is mandatory on reject.
- **Preconditions:** Instance PENDING at line 1.
- **Steps:**
  1. As Rudi, choose Reject; leave the reason empty; submit.
- **Expected result:** Blocked with `INVALID_REQUEST`/field error ("reason required"); no action appended; instance stays PENDING at line 1.
- **Traceability:** F11.2, EX-5, INV-4.

#### TC-E11-F11.2-005 · Second approver on an already-cleared line is a no-op (409)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Edge
- **Objective:** After a line is cleared/advanced, a second member's approve is a no-op conflict.
- **Preconditions:** Rudi already cleared line 1 (instance now at line 2).
- **Steps:**
  1. As Sari, attempt to Approve line 1 (e.g. from a stale tab).
- **Expected result:** `409 LINE_ALREADY_CLEARED`; no duplicate action; instance unchanged at line 2; UI surfaces "already actioned" and refreshes the queue.
- **Traceability:** F11.2, EX-11, C-4, Gherkin "Second approver on a cleared line".

#### TC-E11-F11.2-006 · Concurrent approves by two line-1 members
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Edge
- **Objective:** Race between two members on the same line: exactly one wins.
- **Preconditions:** Instance at line 1 [Rudi, Sari]; both open it simultaneously.
- **Steps:**
  1. Rudi and Sari both press Approve at the same moment.
- **Expected result:** First commit advances to line 2 and appends one `APPROVE` action; the second returns `409 LINE_ALREADY_CLEARED` (no second action). Exactly one advance.
- **Traceability:** F11.2, EX-11, C-4.

#### TC-E11-F11.2-007 · Idempotent re-post of the same approve
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Edge
- **Objective:** Re-posting the same approve with the same `Idempotency-Key` returns the same result, not a duplicate.
- **Preconditions:** Instance PENDING at line 1; Rudi approves with `Idempotency-Key: <uuid>`.
- **Steps:**
  1. Approve line 1 with key K (success).
  2. Re-send the identical approve with key K (e.g. double-tap / retry).
- **Expected result:** Second call returns the **same** cached response (same resulting state); only **one** `APPROVE` action appended; no double-advance. A different body under K → `409 IDEMPOTENCY_KEY_REUSED`.
- **Traceability:** F11.2, EX-11, CONVENTIONS §13.

#### TC-E11-F11.2-008 · Non-member cannot act on the current line (RBAC/membership)
- [ ] **Platform:** Web · **POV:** Shift Leader (not a member) · **Priority:** P0 · **Type:** RBAC
- **Objective:** Routing is by membership; a holder of `approvals.act` who is **not** on the current line cannot decide.
- **Preconditions:** A different shift leader (not on Plaza Senayan's chain) tries to act on Budi's instance.
- **Steps:**
  1. As a non-member shift leader, force the act endpoint for the instance.
- **Expected result:** `403 Forbidden` (or `409`/not-eligible per contract); no action appended; UI never offers the control (item not in their inbox, IB-1).
- **Traceability:** F11.2, EX-2, INV-3, IB-6 (acting gated by membership server-side).

#### TC-E11-F11.2-009 · Member offboarded while on current line — line still clears via other member
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P2 · **Type:** Edge
- **Objective:** If one current-line member is offboarded, another active member can still clear the line.
- **Preconditions:** Instance at line 1 [Rudi, Sari]; Rudi is offboarded (employment ended → login revoked, F2.7).
- **Steps:**
  1. As Sari (still active), Approve line 1.
- **Expected result:** Line 1 clears via Sari; advance to line 2. Rudi can no longer act (revoked). If Rudi were the **sole** member, only super-admin bypass / HR re-edit clears it.
- **Traceability:** F11.2, C-3, INV-J, INV-3/INV-5.

### Web · HR/Placement Admin POV (acting + reset interplay)

#### TC-E11-F11.2-010 · HR approves a line they sit on
- [ ] **Platform:** Web · **POV:** HR/Placement Admin (line member) · **Priority:** P1 · **Type:** Happy
- **Objective:** HR who is a line-2 member (Sari Hadi) approves the final line.
- **Preconditions:** Instance advanced to line 2 [Sari Hadi].
- **Steps:**
  1. As Sari Hadi, Approve line 2.
- **Expected result:** Last line cleared → `status = APPROVED`; `OnApproved` fires once; requester notified.
- **Traceability:** F11.2, EX-3, EX-7, INV-8.

#### TC-E11-F11.2-011 · OnApproved hook error rolls back, instance flagged (C-2)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Edge/Error
- **Objective:** A final-approval re-check failure (e.g. leave LA-5 insufficient remaining) rolls back; the instance does not become APPROVED.
- **Preconditions:** Instance at last line; the leave quota became insufficient since submission.
- **Steps:**
  1. Approve the last line, triggering `OnApproved`.
- **Expected result:** Hook signals a domain block; transaction **rolls back**; `status` stays at current line (not APPROVED); instance **flagged** (sub-state/flag + audit); no side-effects applied; HR can adjust (F6.1) or super admin can bypass; clear error surfaced (no dead flow).
- **Traceability:** F11.2, EX-9, C-2, INV-8.

#### TC-E11-F11.2-012 · Template edit re-bases a pending instance (engine side)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Edge
- **Objective:** Editing the template resets in-flight instances to line 1 on the new version; old actions retained but not counted.
- **Preconditions:** Budi's instance at line 2; HR edits the template.
- **Steps:**
  1. As HR, edit + save the Plaza Senayan template (F11.1-007).
  2. Inspect Budi's instance.
- **Expected result:** Budi's instance `current_line = 1`, `template_version` = new; prior `APPROVE` actions visible in timeline as audit (greyed/superseded) but no longer satisfy line 1; new line-1 members notified.
- **Traceability:** F11.2, EX-8, INV-6, Gherkin "Template edit re-bases a pending instance".

### Web · Super Admin POV (bypass + fallback)

#### TC-E11-F11.2-013 · Super-admin bypass force-approves from any state
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Happy/Edge
- **Objective:** A super admin bypasses a pending instance (skipping remaining lines) with a reason.
- **Preconditions:** Instance PENDING at line 1 (or any non-terminal line). Logged in as Pak Super (`approvals.bypass`). Pak Super need not be a member.
- **Steps:**
  1. Open the instance; choose Bypass; enter reason "Client escalation".
  2. Confirm.
- **Expected result:** `status = APPROVED` (remaining lines skipped); `BYPASS` action appended with reason + `template_version`; `OnApproved` hook fires once; requester notified.
- **Traceability:** F11.2, EX-6, EX-7, INV-5, INV-8, Gherkin "Super-admin bypass".

#### TC-E11-F11.2-014 · Bypass without reason blocked
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Reason is mandatory on bypass.
- **Preconditions:** Instance PENDING; Pak Super opens Bypass.
- **Steps:**
  1. Submit bypass with empty reason.
- **Expected result:** Blocked with field/`INVALID_REQUEST` error ("reason required"); no `BYPASS` action; instance unchanged.
- **Traceability:** F11.2, EX-6, INV-5.

#### TC-E11-F11.2-015 · Bypass on an already-terminal instance (409)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Cannot bypass an APPROVED/REJECTED instance.
- **Preconditions:** Instance already APPROVED (or REJECTED).
- **Steps:**
  1. Attempt Bypass.
- **Expected result:** `409 Conflict` ("already APPROVED/REJECTED"); no action; state unchanged.
- **Traceability:** F11.2, C-5, INV-9.

#### TC-E11-F11.2-016 · No-template fallback routes to super admin (never auto-approve / never block)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Edge
- **Objective:** A company with no template creates an instance on the implicit single super-admin line.
- **Preconditions:** "Baru Jaya" (`SWP-CMP-30`) has no template. An agent there submits overtime.
- **Steps:**
  1. Agent submits OT → instance created.
  2. As Pak Super, view and act.
- **Expected result:** Instance created with `template_id = null`, single implicit super-admin line, `status = PENDING`; submission is **never blocked** and the instance **never auto-approves**; only a super admin can approve/bypass it.
- **Traceability:** F11.2, EX-1, INV-7, Gherkin "No template falls back to super admin".

#### TC-E11-F11.2-017 · Bypass surface is super-admin-only (RBAC)
- [ ] **Platform:** Web · **POV:** Super Admin vs others · **Priority:** P0 · **Type:** RBAC
- **Objective:** Only `approvals.bypass` holders see/use Bypass; line members do not.
- **Preconditions:** Instance PENDING; one session as Pak Super, one as Rudi (line member, no `approvals.bypass`).
- **Steps:**
  1. As Rudi, look for a Bypass control / force the bypass endpoint.
  2. As Pak Super, confirm Bypass is available.
- **Expected result:** Rudi has **no** Bypass control and the endpoint returns `403`; Pak Super sees and can use it. Bypass is not a line-member inbox action.
- **Traceability:** F11.2, EX-6, INV-5, F11.3 non-goal (bypass separate).

#### TC-E11-F11.2-018 · Request type with no registered hook finalizes with warning
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** Edge/Error
- **Objective:** Engine still routes/finalizes a type with no hook, but logs a missing-hook warning and applies no side-effect.
- **Preconditions:** A request_type with no `OnApproved`/`OnRejected` registered (config error simulation).
- **Steps:**
  1. Finalize such an instance (approve last line / bypass).
- **Expected result:** Status reaches terminal; **no domain side-effect**; a missing-hook warning logged/audited; no crash.
- **Traceability:** F11.2, C-6, INV-8.

### Web · Agent POV (requester — self-block + view + withdraw)

#### TC-E11-F11.2-019 · No self-approval (INV-3)
- [ ] **Platform:** Web · **POV:** Agent (requester who is also a line member) · **Priority:** P0 · **Type:** Negative/RBAC
- **Objective:** A requester who is a member of the current line cannot approve their own request.
- **Preconditions:** Rudi (a line-1 member) submitted his own request; instance at line 1.
- **Steps:**
  1. As Rudi (the requester), attempt to Approve line 1.
- **Expected result:** Approve is **not offered** for his own request and the endpoint refuses (server-side); another line-1 member (Sari) must act. If Rudi were the **sole** member, the line clears only by super-admin bypass.
- **Traceability:** F11.2, EX-4, INV-3, Gherkin "No self-approval".

#### TC-E11-F11.2-020 · Requester views chain-progress timeline
- [ ] **Platform:** Web · **POV:** Agent (requester) · **Priority:** P1 · **Type:** Happy
- **Objective:** Requester sees the chain progress on their request detail.
- **Preconditions:** Budi's leave instance pending at line 2 (line 1 approved by Rudi).
- **Steps:**
  1. As Budi, open his leave request detail.
- **Expected result:** Timeline shows Line 1 (members + Rudi's APPROVE with time), Line 2 pending (members), in order; current pending line indicated; read-only (no act controls for the requester).
- **Traceability:** F11.2, Platform table (Requester views timeline), IB-4.

#### TC-E11-F11.2-021 · Withdraw underlying request while pending (C-1)
- [ ] **Platform:** Web · **POV:** Agent (requester) · **Priority:** P1 · **Type:** Edge
- **Objective:** Withdrawing the domain request (domain-owned) closes the instance without firing hooks.
- **Preconditions:** Budi's leave instance PENDING.
- **Steps:**
  1. As Budi, withdraw the leave request (domain action).
- **Expected result:** Domain cancels the request; engine marks the instance cancelled/closed; **no** `OnApproved`/`OnRejected` hook fires; item drops out of approvers' inboxes.
- **Traceability:** F11.2, C-1, INV-8.

### Mobile · Shift Leader POV (on-site approver)

#### TC-E11-F11.2-022 · Approve current line on mobile
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** On-site shift leader approves the current line from the phone with full engine parity.
- **Preconditions:** Mobile app logged in as Rudi; Budi's instance pending at line 1.
- **Steps:**
  1. Open Approvals/Inbox; tap Budi's request; Approve.
- **Expected result:** Same engine result as web (TC-001): line 1 clears, advance to line 2; `APPROVE` action appended; item leaves Rudi's queue.
- **Traceability:** F11.2, EX-3, F11.3 IB (mobile parity), Gherkin "Mobile parity".

#### TC-E11-F11.2-023 · Reject with reason on mobile
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy/Negative
- **Objective:** Reject + mandatory reason works on mobile; empty reason blocked.
- **Preconditions:** Instance pending at line 1 on Rudi's phone.
- **Steps:**
  1. Tap Reject; submit empty reason (blocked); then enter a reason and submit.
- **Expected result:** Empty reason blocked with inline error; with a reason → `REJECTED`, OnRejected fires, requester notified; item leaves queue.
- **Traceability:** F11.2, EX-5, INV-4.

#### TC-E11-F11.2-024 · Mobile no-op on stale cleared line (409)
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P2 · **Type:** Edge/Error
- **Objective:** Acting on an already-advanced line from a stale mobile view returns a clean conflict.
- **Preconditions:** Line 1 already cleared by Sari; Rudi's phone shows a stale item.
- **Steps:**
  1. Tap Approve on the stale item.
- **Expected result:** `409 LINE_ALREADY_CLEARED`; friendly "already actioned" message; list refreshes and the item disappears. No duplicate action.
- **Traceability:** F11.2, EX-11, C-4.

#### TC-E11-F11.2-025 · Bypass not available on mobile (web-only)
- [ ] **Platform:** Mobile · **POV:** Super Admin · **Priority:** P2 · **Type:** RBAC/Edge
- **Objective:** Super-admin bypass has no mobile surface (F11.2 Platform table: bypass = web console).
- **Preconditions:** Pak Super logged into mobile.
- **Steps:**
  1. Look for a Bypass action on a pending instance in mobile.
- **Expected result:** No Bypass control on mobile; bypass is performed on the web console only. (Mobile may still show member approve/reject if Pak Super is a current-line member.)
- **Traceability:** F11.2, EX-6, Platform table (Bypass = Web console).

### Mobile · HR/Placement Admin POV

#### TC-E11-F11.2-026 · HR acts on a line via mobile
- [ ] **Platform:** Mobile · **POV:** HR/Placement Admin (line member) · **Priority:** P2 · **Type:** Happy
- **Objective:** HR who is a line member can approve/reject on mobile (template management remains web-only).
- **Preconditions:** Sari Hadi (HR, line-2 member) on mobile; instance at line 2.
- **Steps:**
  1. Open Approvals; Approve line 2.
- **Expected result:** Final line clears → APPROVED; OnApproved fires. (No template-edit option exposed on mobile.)
- **Traceability:** F11.2, EX-3/EX-7; F11.1 web-only.

---

## F11.3 — Approval Inbox (web + mobile)

> Aggregated "needs my decision" queue: non-terminal instances whose **current** line includes the viewer, excluding the viewer's own requests (INV-3), scoped server-side. View over the same instances as per-domain approval tabs (IB-5). See [PRD](../epics/E11-approvals/prds/approval-inbox.md).

### Web · Shift Leader POV

#### TC-E11-F11.3-001 · See and act on a current-line item
- [ ] **Platform:** Web · **POV:** Shift Leader (line member) · **Priority:** P0 · **Type:** Happy
- **Objective:** Inbox lists current-line items the viewer can act on; approving removes the item.
- **Preconditions:** Rudi is a line-1 member for Plaza Senayan; Budi's leave pending at line 1.
- **Steps:**
  1. As Rudi, open Kotak Masuk.
  2. Confirm the item shows request type + summary, requester (Budi), company (Plaza Senayan), "Line 1 of 2", submitted-at.
  3. Approve it.
- **Expected result:** Item visible with all listed fields; on approve it advances to line 2 and **leaves Rudi's inbox**.
- **Traceability:** F11.3, IB-1, IB-2, IB-3, Gherkin "See and act on a current-line item".

#### TC-E11-F11.3-002 · Reject from the inbox
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Reject (with reason) from the inbox terminates the instance and removes the item.
- **Preconditions:** Budi's request pending at line 1; Rudi viewing inbox.
- **Steps:**
  1. Reject with a reason.
- **Expected result:** Status becomes REJECTED; item leaves Rudi's inbox; requester notified.
- **Traceability:** F11.3, IB-3, EX-5, Gherkin "Reject from the inbox".

#### TC-E11-F11.3-003 · Item advanced by another member disappears on refresh
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Edge
- **Objective:** When another member clears the current line, the item drops from the viewer's queue.
- **Preconditions:** Sari approved line 1 before Rudi; Rudi's inbox still shows the item (stale).
- **Steps:**
  1. As Rudi, refresh the inbox.
- **Expected result:** Budi's request is **gone** from Rudi's line-1 queue (now at line 2, where Rudi is not a member).
- **Traceability:** F11.3, IB-3, Gherkin "Item advanced by another member".

#### TC-E11-F11.3-004 · Viewer on a later line does not see item yet (C-1)
- [ ] **Platform:** Web · **POV:** Shift Leader (line-2 member only) · **Priority:** P1 · **Type:** Edge
- **Objective:** Only current-line membership surfaces an item.
- **Preconditions:** Viewer is a member of line 2 only; instance is at line 1.
- **Steps:**
  1. Open Kotak Masuk.
- **Expected result:** The item does **not** appear until the chain advances to line 2.
- **Traceability:** F11.3, IB-1, C-1.

#### TC-E11-F11.3-005 · Viewer on line 1 AND line 2 (C-2)
- [ ] **Platform:** Web · **POV:** Shift Leader (on both lines) · **Priority:** P2 · **Type:** Edge
- **Objective:** Item shows while line 1 current; after line 1 clears (by another member), it reappears at line 2; still cannot self-approve.
- **Preconditions:** Viewer is on both line 1 and line 2; instance at line 1; viewer is NOT the requester.
- **Steps:**
  1. View inbox (item shown for line 1).
  2. Have a different line-1 member clear line 1.
  3. Refresh inbox.
- **Expected result:** Item disappears after step 1's line clears, then **reappears** for line 2; viewer can act on line 2 (self-approval still blocked if they were the requester, INV-3).
- **Traceability:** F11.3, IB-1, C-2, INV-3.

#### TC-E11-F11.3-006 · Chain timeline on detail (web)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Request detail renders the full chain timeline including actions and bypass rows.
- **Preconditions:** Instance with at least one recorded action (and ideally a bypass on another instance).
- **Steps:**
  1. Open the request detail.
- **Expected result:** Timeline shows each line, its members, every `approval_actions` row (actor, decision approve/reject/bypass, reason, time, template_version), and the current pending line — in order. `BYPASS` rows included.
- **Traceability:** F11.3, IB-4, INV-9, Gherkin "Chain timeline on detail".

#### TC-E11-F11.3-007 · Inbox mirrors per-domain approval tab (single source)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P2 · **Type:** Edge
- **Objective:** The same instance appears identically in Kotak Masuk and in the domain tab (Cuti → Approvals / Lembur → Approvals); acting in one reflects in the other.
- **Preconditions:** Budi's leave pending at line 1; Rudi a member.
- **Steps:**
  1. Open Kotak Masuk and the Cuti → Approvals tab; confirm the same item.
  2. Approve from the domain tab; check Kotak Masuk.
- **Expected result:** Single source — the item is identical in both; approving in the domain tab removes it from Kotak Masuk too (no divergence / no double queue).
- **Traceability:** F11.3, IB-5.

#### TC-E11-F11.3-008 · Inbox reflects template reset on next load (IB-7)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Edge
- **Objective:** After a template edit (INV-6), the inbox reflects the new line-1 membership.
- **Preconditions:** Instance reset to line 1 on a new version (F11.1-007); membership changed so a different leader is now on line 1.
- **Steps:**
  1. As the new line-1 member, reload Kotak Masuk.
  2. As the former line-1 member (no longer on line 1), reload Kotak Masuk.
- **Expected result:** New line-1 member now sees the item; former member (if removed from line 1) no longer sees it.
- **Traceability:** F11.3, IB-7, INV-6, C-5.

#### TC-E11-F11.3-009 · Empty inbox state
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** No pending items renders a designed empty state, not a blank screen.
- **Preconditions:** Viewer has no current-line items.
- **Steps:**
  1. Open Kotak Masuk.
- **Expected result:** `comp/EmptyInbox`-style empty state ("Tidak ada yang perlu diputuskan" / no dead flow).
- **Traceability:** F11.3, C-3.

#### TC-E11-F11.3-010 · Inbox loading and error states
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Loading skeleton and a retryable error state.
- **Preconditions:** Open Kotak Masuk; simulate slow then failed fetch.
- **Steps:**
  1. Observe initial loading skeleton.
  2. Force a list-fetch error.
- **Expected result:** Skeleton during load; on failure a retryable error banner (no blank/dead flow); retry refetches.
- **Traceability:** F11.3, ENGINEERING no-dead-flow.

#### TC-E11-F11.3-011 · Filter / group by request type
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P2 · **Type:** Happy
- **Objective:** Inbox is groupable/filterable by request type (leave vs overtime).
- **Preconditions:** Viewer has both LEAVE and OVERTIME current-line items.
- **Steps:**
  1. Filter to OVERTIME only; then to LEAVE only.
- **Expected result:** List filters correctly per type; counts reflect the filter; aggregated across types when unfiltered.
- **Traceability:** F11.3, IB-2.

#### TC-E11-F11.3-012 · Cursor pagination of a large inbox
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P2 · **Type:** Edge
- **Objective:** Large inboxes page via opaque cursor; sort change resets pagination.
- **Preconditions:** Viewer has >50 current-line items.
- **Steps:**
  1. Load page 1; follow `next_cursor` to page 2.
  2. Change sort and reuse the old cursor.
- **Expected result:** Pages return `items` + `next_cursor` + `has_more`; last page `has_more:false`/`next_cursor:null`; reusing a cursor after a sort/filter change → `400 CURSOR_MISMATCH`.
- **Traceability:** F11.3, CONVENTIONS §9 (cursor pagination).

### Web · HR/Placement Admin POV

#### TC-E11-F11.3-013 · HR inbox shows lines they're current-line members of
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** HR sees current-line items they can act on (e.g. line-2 instances) scoped to their data scope.
- **Preconditions:** Sari Hadi (HR, line-2 member); an instance advanced to line 2.
- **Steps:**
  1. Open Kotak Masuk.
- **Expected result:** Line-2 item visible with "Line 2 of 2"; HR can approve/reject; items outside her data scope not shown (server-enforced, IB-1).
- **Traceability:** F11.3, IB-1, IB-2.

#### TC-E11-F11.3-014 · Data-scope filtering (server-enforced)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** RBAC/Edge
- **Objective:** Inbox only shows instances within the viewer's company/data scope even if membership matched.
- **Preconditions:** HR with scope limited to certain companies; instances exist for out-of-scope companies.
- **Steps:**
  1. Open Kotak Masuk; attempt to query an out-of-scope instance directly.
- **Expected result:** Only in-scope current-line items listed; direct access to out-of-scope instance denied server-side (`403`/filtered). Defense-in-depth — server is the gate.
- **Traceability:** F11.3, IB-1, IB-6.

### Web · Super Admin POV

#### TC-E11-F11.3-015 · Super admin sees only items where they are a current-line member
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Edge/RBAC
- **Objective:** In the inbox, a super admin is treated as a line member (not an omniscient approver); bypass is separate.
- **Preconditions:** Pak Super is a configured line-3 member of one instance; another instance does not include him.
- **Steps:**
  1. Open Kotak Masuk.
- **Expected result:** Only the instance where Pak Super is the **current**-line member appears for line-member approve; the other does not appear in the inbox. Bypass (force-approve) is a **separate** super-admin action (F11.2), not surfaced as the inbox line-member approve.
- **Traceability:** F11.3, C-4, IB-1, F11.2 EX-6 (bypass separate).

### Web · Agent POV

#### TC-E11-F11.3-016 · Agent has no approver inbox; own request not self-approvable
- [ ] **Platform:** Web · **POV:** Agent · **Priority:** P0 · **Type:** RBAC/Negative
- **Objective:** Agents without `approvals.act` see no Kotak Masuk approver queue; their own submitted request never appears for self-approval.
- **Preconditions:** Budi (agent, no `approvals.act`). Budi submitted a request.
- **Steps:**
  1. As Budi, attempt to open Kotak Masuk (approver queue).
  2. Check whether his own request appears as actionable.
- **Expected result:** Approver inbox nav hidden / `403` on direct access (gated by `approvals.act`, IB-6); his own request never shows for self-approval (INV-3). Budi can still track status via his request detail (read-only).
- **Traceability:** F11.3, IB-6, INV-3, Gherkin "My own request is not in my inbox".

#### TC-E11-F11.3-017 · Member-and-requester sees own request excluded (INV-3)
- [ ] **Platform:** Web · **POV:** Agent/staff who is both requester and line member · **Priority:** P0 · **Type:** Negative
- **Objective:** Even a line member's own submitted request is excluded from their inbox.
- **Preconditions:** Rudi is a line-1 member and submitted his own overtime; instance at line 1.
- **Steps:**
  1. As Rudi, open Kotak Masuk.
- **Expected result:** Rudi's **own** request is NOT listed for self-approval (excluded by INV-3); only another line-1 member sees it.
- **Traceability:** F11.3, IB-1, INV-3, Gherkin "My own request is not in my inbox".

### Mobile · Shift Leader POV

#### TC-E11-F11.3-018 · Mobile inbox parity — same current-line queue
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Phone-first inbox shows the same current-line queue and supports approve/reject.
- **Preconditions:** Rudi (shift leader) on mobile; Budi's request pending at line 1.
- **Steps:**
  1. Open Approvals/Inbox on the phone.
  2. Confirm the item (type, requester, company, "Line 1 of 2").
  3. Approve, then on another item Reject with reason.
- **Expected result:** Same queue/content as web (single source); approve advances/clears; reject (with reason) terminates; acted items leave the queue.
- **Traceability:** F11.3, IB-1, IB-3, Gherkin "Mobile parity".

#### TC-E11-F11.3-019 · Mobile chain timeline on detail
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Request detail on mobile renders the chain timeline (lines, members, actions, pending line).
- **Preconditions:** Instance with recorded actions; Rudi on mobile.
- **Steps:**
  1. Open a request detail.
- **Expected result:** Timeline parity with web (IB-4): lines + members + each action (actor/decision/reason/time) + pending line, in order, including BYPASS rows.
- **Traceability:** F11.3, IB-4.

#### TC-E11-F11.3-020 · Mobile empty / loading / error states
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Mobile inbox handles empty queue, loading, and fetch error without dead flow.
- **Preconditions:** Rudi on mobile with no current-line items; then simulate load and error.
- **Steps:**
  1. Open Approvals with an empty queue.
  2. Trigger a slow load then a fetch error.
- **Expected result:** Empty state (`comp/EmptyInbox` mobile equivalent); loading indicator; retryable error state. No blank screen.
- **Traceability:** F11.3, C-3, ENGINEERING no-dead-flow.

#### TC-E11-F11.3-021 · Mobile stale-item refresh after another member acts
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P2 · **Type:** Edge
- **Objective:** Pull-to-refresh removes items already advanced by another member.
- **Preconditions:** Sari cleared line 1; Rudi's phone still shows the item.
- **Steps:**
  1. Pull to refresh.
- **Expected result:** Advanced item disappears (now at line 2 where Rudi is not a member); acting on a stale item returns `409 LINE_ALREADY_CLEARED` with friendly handling.
- **Traceability:** F11.3, IB-3, F11.2 EX-11.

### Mobile · HR/Placement Admin POV

#### TC-E11-F11.3-022 · HR mobile inbox (line member)
- [ ] **Platform:** Mobile · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Happy
- **Objective:** HR who is a line member can use the mobile inbox to act, scoped to their data scope.
- **Preconditions:** Sari Hadi (HR, line-2 member) on mobile; instance at line 2 in her scope.
- **Steps:**
  1. Open Approvals; act on the line-2 item.
- **Expected result:** Line-2 item visible and actionable; out-of-scope items not shown (server-enforced); no template-management surface on mobile.
- **Traceability:** F11.3, IB-1, IB-6.

### Mobile · Agent POV

#### TC-E11-F11.3-023 · Agent mobile — requester status view, no approver queue
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** RBAC/Negative
- **Objective:** On mobile, an agent (no `approvals.act`) has no approver inbox; can only track their own request status/timeline.
- **Preconditions:** Budi (agent) on mobile with a pending leave request.
- **Steps:**
  1. Look for an Approvals queue.
  2. Open his own leave request detail.
- **Expected result:** No approver Approvals queue (gated by `approvals.act`); his own request detail shows the read-only chain timeline/status; no act controls (INV-3).
- **Traceability:** F11.3, IB-6, INV-3.

---

## Appendix · Notes on deferred / v1-none behaviors

- **SLA / auto-escalation:** none in v1 — a line may sit un-actioned indefinitely; **super-admin bypass is the only escape hatch**. No timer/escalation test cases are included by design (FEATURE §3 / §7 Open, F11.2 §10). If/when an SLA is added, add `TC-E11-F11.2-1xx` escalation cases.
- **Live badge counts:** inbox live counts/badges are a follow-up with E10 (F11.3 §10 Open); covered only as far as filter counts in TC-E11-F11.3-011. Add badge-count cases once E10 ratifies them.
- **Role-ref membership:** v1 is explicit-users only — no "this company's shift leader" role refs (F11.1 §10). No role-ref test cases.
- **Profile change-request approval:** removed in v1 (no `SWP-CHG`); intentionally **not** tested here.
