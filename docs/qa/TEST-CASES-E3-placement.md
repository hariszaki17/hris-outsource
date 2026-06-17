# Test Cases — E3 Placement Management

> **Epic:** E3 Placement Management (the differentiator) · **Type:** Detailed manual test cases · **Status:** Draft v1
> **Sources:** [FEATURE.md](../epics/E3-placement/FEATURE.md), PRDs F3.1–F3.5 in [`prds/`](../epics/E3-placement/prds/), [API CONVENTIONS](../api/CONVENTIONS.md).
> **Reference date for all "today"/relative-date conversions:** **2026-06-17** (Asia/Jakarta). All dates below are absolute.

---

## 1. Scope

This document is an **exhaustive manual test plan** for the five E3 features:

| Feature | Title |
|---|---|
| **F3.1** | Agent Placement (create & activate) |
| **F3.2** | Placement Lifecycle & Status |
| **F3.3** | Re-placement & Transfer (with history) |
| **F3.4** | Shift-Leader Assignment |
| **F3.5** | Company Placement Roster |

Cases are organized **per feature → per platform (Web / Mobile) → per POV (super admin · HR/placement admin · shift leader · agent)**. They cover happy paths, **invariant enforcement (INV-1..5)**, lifecycle state transitions (each valid + invalid), transfer/replacement with history preservation, RBAC denials, empty/loading/error states, and roster views. Every `BR-#`/`LC-#`/`TR-#`/`SL-#`/`RO-#`, `C-#`, and `INV-#` is traced.

**Out of scope** (other epics, referenced only as side-effects): shift master & rostering (E4), attendance/geofence (E5), leave (E6), overtime (E7), payroll (E8), legacy migration (E9), notification rendering internals (E10), audit-log UI (E1).

### Platform / role notes
- The HRIS web console is **internal-only** (Vite SPA). HR admin, super admin, lead, and shift leader operate primarily on **Web**. Mutations (create/transfer/renew/terminate/assign-leader) are **Web-only** — there is no mobile create/edit surface.
- **Agent** is mobile-primary (React Native): the agent has **read-only** views (own active placement + own history). "Agent" is a domain term for a `FIELD` employee with no elevation (baseline `self.*`), per CONVENTIONS §17.
- **Shift leader** consumes the **roster (read)** on Web and a condensed roster on Mobile; the role/company scope is **derived per request** from the active `shift_leader_assignments` row (SL-10).
- **Lead** = company-scoped arranger (FEATURE §2); where a case applies equally to HR admin and lead within scope it is noted; lead-specific scope denials are called out.

### Conventions used in steps
- `409 INV_<N>_VIOLATION` = invariant conflict; `422 RULE_VIOLATION`/specific code = business-rule failure; `403 FORBIDDEN`/`OUT_OF_SCOPE` = RBAC; `404 NOT_FOUND` = no visibility. (CONVENTIONS §7, §11.)
- All "today" comparisons use **Asia/Jakarta** (BR-5, C-8, LC-2). Reference "today" = **2026-06-17**.

---

## 2. Coverage matrix

Legend: ✅ = cases present · — = not applicable on that platform/role.

| Feature | Web · Super Admin | Web · HR Admin | Web · Shift Leader | Web · Agent | Mobile · Shift Leader | Mobile · Agent |
|---|:--:|:--:|:--:|:--:|:--:|:--:|
| **F3.1** Create & activate | ✅ | ✅ | — (RBAC deny) | — (RBAC deny) | — | ✅ (read result) |
| **F3.2** Lifecycle & status | ✅ | ✅ | — (read) | — | — | ✅ (read status) |
| **F3.3** Transfer | ✅ | ✅ | — (RBAC deny) | — | — | ✅ (read result) |
| **F3.4** Shift-leader assignment | ✅ | ✅ | — (RBAC deny) | — | ✅ (read self-grant) | — |
| **F3.5** Company roster | ✅ | ✅ | ✅ (own only) | — | ✅ (own only) | — |

> Agent never sees other agents' placements or any company roster; agent only reads **own** placement (F3.1/F3.2 result). Shift leader reads **only the company/unit they lead** (RO-4). Mutations are HR admin / super admin (and lead, scoped) only.

---

## F3.1 — Agent Placement (create & activate)

Traces PRD F3.1: US-1..5, BR-1..10, C-1..12, INV-1/INV-5.

### Web · HR Admin

#### TC-E3-F3.1-001 · Create an immediately-active placement (happy path)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy
- **Objective:** HR admin places an active agent at a company/site/position with start date = today and it activates immediately.
- **Preconditions:** Signed in as HR admin. Active agent "Budi" (`SWP-EMP-…`) with **no** active/scheduled placement. Active company "Plaza Senayan" with active Main Site. Budi has a finalized PKWT agreement covering today.
- **Steps:**
  1. Open New Placement.
  2. Select agent "Budi".
  3. Select company "Plaza Senayan"; site defaults to "Main Site".
  4. Enter position "Parking Attendant" (free text).
  5. Set start date = **2026-06-17** (today); set a valid end date within the PKWT period (e.g. 2026-12-31).
  6. Attach the PKWT employment agreement.
  7. Submit.
- **Expected result / Acceptance criteria:** Placement created with status **Active** (BR-5). `201 Created` with `Location`. Audit-log entry recorded (BR-7). Budi can see the active placement; the placement appears in the Plaza Senayan roster. Notification dispatched to Budi + assigned leader if any (BR-7, CONVENTIONS §16.2).
- **Traceability:** F3.1, US-1, BR-1, BR-5, BR-7, INV-5.

#### TC-E3-F3.1-002 · Create a future-dated (Scheduled) placement
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy
- **Objective:** A start date in the future yields status Scheduled, not yet visible as active to the agent.
- **Preconditions:** HR admin signed in; agent "Budi" with no active/scheduled placement; active company + site.
- **Steps:**
  1. New Placement → agent Budi → company/site → position "Parking Attendant".
  2. Set start date = **2026-07-01** (14 days from today).
  3. Submit.
- **Expected result:** Status **Scheduled** (BR-5). Not shown as active to Budi. Will auto-activate on 2026-07-01 (F3.2 LC-2). Audit + notification recorded.
- **Traceability:** F3.1, US-2, BR-5.

#### TC-E3-F3.1-003 · Site defaults to primary Main Site for single-location company
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Creating without explicitly choosing a site defaults to the company's primary Main Site.
- **Preconditions:** Single-location company "Mall Kelapa Gading" with only its "Main Site"; agent Budi free.
- **Steps:**
  1. New Placement → Budi → company "Mall Kelapa Gading"; do not change the site selector.
  2. Set position + start date; submit.
- **Expected result:** Placement created with `site_id` = Main Site (BR-3b). INV-5 satisfied (site belongs to company).
- **Traceability:** F3.1, BR-3b, INV-5.

#### TC-E3-F3.1-004 · Place at a specific non-default site of a multi-site company
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** HR can target a specific site; placement records that site.
- **Preconditions:** Company "Plaza Group" with sites "Main Site", "Plaza Senayan", "Plaza Indonesia" (all active); agent Budi free.
- **Steps:**
  1. New Placement → Budi → "Plaza Group".
  2. Change site to "Plaza Senayan".
  3. Set position + start date; submit.
- **Expected result:** Placement `site_id` = "Plaza Senayan". E5 clock-in will validate against that site's geofence. INV-5 satisfied.
- **Traceability:** F3.1, BR-3b, INV-5.

#### TC-E3-F3.1-005 · Position typeahead suggests existing DISTINCT values but accepts any new string
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Position is free-text; typeahead aids entry; new strings accepted; nothing enforced.
- **Preconditions:** HR admin; existing placements with positions "Parking Attendant", "Building Technician".
- **Steps:**
  1. New Placement → agent → company/site.
  2. In position field type "Park" → observe suggestion "Parking Attendant".
  3. Instead type a brand-new value "Lobby Concierge"; submit.
- **Expected result:** Suggestions from `GET /positions:search` show DISTINCT existing values; the new "Lobby Concierge" is accepted and stored verbatim (BR-9). No FK/uniqueness/service-line enforcement.
- **Traceability:** F3.1, BR-9.

#### TC-E3-F3.1-006 · Open-ended placement for a PKWTT agent (no end date)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy/Edge
- **Objective:** A PKWTT (indefinite) agent can have an open-ended placement with no upper bound.
- **Preconditions:** Agent Budi with a PKWTT agreement (no end date); no active placement.
- **Steps:**
  1. New Placement → Budi → company/site → position.
  2. Set start date 2026-06-17; leave end date blank.
  3. Attach the PKWTT agreement; submit.
- **Expected result:** Placement created with open-ended period; status Active. No auto-cap. It will never auto-expire (LC-3). 
- **Traceability:** F3.1, BR-1, BR-1b, C-4.

#### TC-E3-F3.1-007 · Auto-cap placement end to the PKWT agreement end
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Edge
- **Objective:** When the entered end date exceeds the PKWT agreement end, the system auto-caps and notifies the creator.
- **Preconditions:** Agent Budi with a PKWT agreement ending **2026-12-31**; no active placement.
- **Steps:**
  1. New Placement → Budi → company/site → position.
  2. Set start 2026-06-17, end date **2027-03-31**.
  3. Attach the PKWT agreement; submit.
- **Expected result:** Placement created with end date **auto-capped to 2026-12-31** (BR-1b). Creator notified the end date was adjusted to the agreement end.
- **Traceability:** F3.1, BR-1b.

#### TC-E3-F3.1-008 · Create without an employment agreement (awaiting_agreement flag)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy/Edge
- **Objective:** A placement may be created before the PKWT/PKWTT is finalized; it is flagged awaiting agreement and period validation is skipped.
- **Preconditions:** Agent Budi with **no** finalized agreement; active company + site.
- **Steps:**
  1. New Placement → Budi → "Plaza Senayan" → position "Parking Attendant".
  2. Leave the employment-agreement field empty.
  3. Set start 2026-06-17; optionally leave end date open.
  4. Submit.
- **Expected result:** Placement created successfully, flagged **`awaiting_agreement = true`** (BR-1, BR-10). No period-within-agreement validation (BR-1b skipped). End date may be open-ended. `awaiting_agreement` is a compliance flag, not a lifecycle status — status still follows BR-5 (Active). It appears under the roster "awaiting agreement" filter.
- **Traceability:** F3.1, BR-1, BR-10.

#### TC-E3-F3.1-009 · Backfill the employment agreement later (re-runs period check / auto-cap)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy/Edge
- **Objective:** Attaching the agreement clears the awaiting flag and re-validates the period.
- **Preconditions:** Budi has a placement flagged awaiting_agreement running **2026-06-09** onward (open end). His finalized PKWT runs **2026-06-01 → 2026-12-31**.
- **Steps:**
  1. Open the placement; choose Attach agreement (`POST /placements/{id}/agreement`).
  2. Select Budi's PKWT agreement; confirm.
- **Expected result:** `awaiting_agreement` cleared. BR-1b re-runs: if the placement end exceeded 2026-12-31 it is auto-capped to 2026-12-31; creator notified if adjusted. Audit entry recorded.
- **Traceability:** F3.1, BR-10, BR-1b.

#### TC-E3-F3.1-010 · Backfill rejected — agreement belongs to another agent
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Backfilling an agreement not belonging to the placement's agent is rejected.
- **Preconditions:** Budi has a placement flagged awaiting_agreement. An agreement exists belonging to a different agent "Andi".
- **Steps:**
  1. Open Budi's awaiting placement → Attach agreement.
  2. Select Andi's agreement; confirm.
- **Expected result:** Rejected with message "Agreement does not belong to this agent" (`422`/specific code). Placement stays flagged awaiting_agreement.
- **Traceability:** F3.1, BR-10.

#### TC-E3-F3.1-011 · Backfill an agreement to a placement that already has one (no-op/reject)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Re-attaching an agreement when one is already present is rejected/no-op.
- **Preconditions:** Budi has a placement with an attached PKWT (not awaiting).
- **Steps:**
  1. Open the placement → attempt Attach agreement.
- **Expected result:** Action unavailable, or returns rejected/no-op — nothing is pending. To change the agreement, HR must renew/transfer, not backfill (BR-10, C-12).
- **Traceability:** F3.1, BR-10, C-12.

#### TC-E3-F3.1-012 · INVARIANT — block a second active placement (double-booking)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant
- **Objective:** Creating a second overlapping active placement for an agent is rejected (one active placement per agent).
- **Preconditions:** Budi already has an **active** placement at "Mall Kelapa Gading".
- **Steps:**
  1. New Placement → Budi → "Plaza Senayan" → position.
  2. Set an overlapping period (e.g. start 2026-06-20).
  3. Submit.
- **Expected result:** Creation **blocked** — `409 INV_1_VIOLATION`, message "Agent already has an active placement". UI offers to **end or transfer** the existing placement. INV-1 enforced at persist time (DB constraint), not only UI.
- **Traceability:** F3.1, US-3, BR-2, **INV-1**.

#### TC-E3-F3.1-013 · INVARIANT — overlap enforced at persist time on concurrent create (DB constraint)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Invariant
- **Objective:** Two HR admins creating overlapping placements for the same agent — only one commits.
- **Preconditions:** Agent Budi free. Two HR sessions (A and B).
- **Steps:**
  1. In session A, prepare a placement for Budi (start 2026-06-20) but do not submit.
  2. In session B, create and submit a placement for Budi (start 2026-06-20).
  3. In session A, submit.
- **Expected result:** Session B succeeds; session A fails the overlap check at persist time with `409 INV_1_VIOLATION` (BR-2, C-6) — last writer gets the error. Not relying on UI-only guard.
- **Traceability:** F3.1, BR-2, C-6, **INV-1**.

#### TC-E3-F3.1-014 · INVARIANT — 1-day buffer: block same-day handover
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant
- **Objective:** A new placement cannot start the same day a prior one ends; min 1-day buffer.
- **Preconditions:** Budi had a placement at "Mall Kelapa Gading" ending **2026-06-30** (now terminal/ending).
- **Steps:**
  1. New Placement → Budi → "Plaza Senayan".
  2. Set start date **2026-06-30** (same as prior end).
  3. Submit.
- **Expected result:** Blocked with "No overlap or same-day handover — start the day after the prior placement ends". Earliest allowed start = **2026-07-01** (BR-2, C-2).
- **Traceability:** F3.1, BR-2, C-2.

#### TC-E3-F3.1-015 · 1-day buffer: allow start the day after prior end
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Start exactly one day after the prior end is allowed.
- **Preconditions:** Budi's prior placement ends **2026-06-30**.
- **Steps:**
  1. New Placement → Budi → company/site → start date **2026-07-01**; submit.
- **Expected result:** Allowed — no overlap, no same-day handover (C-1).
- **Traceability:** F3.1, BR-2, C-1.

#### TC-E3-F3.1-016 · Reject end date before start date
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** end_date must be after start_date.
- **Preconditions:** HR admin; agent free.
- **Steps:**
  1. New Placement → set start **2026-07-10**, end **2026-07-01**.
  2. Submit.
- **Expected result:** Blocked with field-level validation error on `end_date` (`INVALID_REQUEST`, `fields.end_date`) (BR-4, CONVENTIONS §12).
- **Traceability:** F3.1, BR-4.

#### TC-E3-F3.1-017 · Block placement into an inactive/archived company
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Cannot place into a non-active company.
- **Preconditions:** Company "Old Tower" is archived/inactive.
- **Steps:**
  1. New Placement → agent → attempt to select "Old Tower" (or it is filtered out); force-submit if reachable.
- **Expected result:** Company not selectable, or creation blocked with "Company is not active" (BR-3).
- **Traceability:** F3.1, BR-3.

#### TC-E3-F3.1-018 · Block placement into an inactive site of an active company
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Site must belong to the company and be active.
- **Preconditions:** Active company with one active and one **inactive** site.
- **Steps:**
  1. New Placement → company → attempt to select the inactive site; submit.
- **Expected result:** Inactive site not selectable / blocked (BR-3b, E2 ST-4). INV-5 (site belongs to company) upheld.
- **Traceability:** F3.1, BR-3b, INV-5.

#### TC-E3-F3.1-019 · Block placing an inactive/resigned agent
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Only active employees can be placed.
- **Preconditions:** Agent "Citra" with employee status resigned/inactive.
- **Steps:**
  1. New Placement → search agent → Citra is excluded or blocked on submit.
- **Expected result:** Blocked — only active employees can be placed (BR-1 data model "employee status = active", C-9).
- **Traceability:** F3.1, C-9.

#### TC-E3-F3.1-020 · Warn when the company has no shift leader (creation still succeeds)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy/Edge
- **Objective:** Placement creation is not blocked by a leaderless company; a warning prompts F3.4.
- **Preconditions:** "Plaza Senayan" active, **no** shift leader assigned; agent free.
- **Steps:**
  1. New Placement → Budi → "Plaza Senayan" → position; submit.
- **Expected result:** Placement created successfully. UI surfaces a warning "assign a shift leader" linking to the company "Pemimpin Shift" tab (BR-8, C-7). Leader notification skipped (no leader).
- **Traceability:** F3.1, BR-8, C-7.

#### TC-E3-F3.1-021 · HR admin backdates start date with a reason
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Backdating is allowed for HR admin with a mandatory reason; audited.
- **Preconditions:** Agent Budi free; no overlapping prior placement.
- **Steps:**
  1. New Placement → Budi → company/site → position.
  2. Set start date **2026-06-01** (in the past).
  3. Observe the backdate-reason field becomes required; enter a reason.
  4. Submit.
- **Expected result:** Placement created (status Active per BR-5). Audit log records the backdating reason (BR-6). Submitting without the reason is blocked.
- **Traceability:** F3.1, BR-6, C-3.

#### TC-E3-F3.1-022 · Warn on far-future start date (likely data entry error)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** A start date > 1 year out is allowed but warns.
- **Preconditions:** Agent free.
- **Steps:**
  1. New Placement → set start date **2027-09-01** (> 1 year from today).
  2. Submit.
- **Expected result:** Allowed (status Scheduled) but a warning is shown about the unusually far-future date (C-10).
- **Traceability:** F3.1, C-10.

#### TC-E3-F3.1-023 · Agent currently serving as shift leader at A blocked from placement at B
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Invariant
- **Objective:** An agent who is shift leader at company A still has an active placement at A, so a new placement at B is blocked by INV-1.
- **Preconditions:** Budi is the shift leader of company A and actively placed there.
- **Steps:**
  1. New Placement → Budi → company B; submit.
- **Expected result:** Blocked by INV-1 (`409 INV_1_VIOLATION`). HR must transfer first (F3.3), which also vacates the leader role (F3.4 SL-6, C-5).
- **Traceability:** F3.1, C-5, **INV-1**.

#### TC-E3-F3.1-024 · Renew/transfer of an awaiting-agreement placement propagates the pending flag
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** A successor of a still-pending placement stays awaiting; no auto-cap.
- **Preconditions:** Budi has an Active placement flagged awaiting_agreement.
- **Steps:**
  1. Renew (F3.2) or transfer (F3.3) the placement to a new period.
  2. Submit.
- **Expected result:** Successor created **also awaiting_agreement** (null agreement propagates). BR-1b not run; no PKWT auto-cap. Successor stays flagged until its own backfill (BR-10, C-11).
- **Traceability:** F3.1, BR-10, C-11.

#### TC-E3-F3.1-025 · "Today" evaluated in Asia/Jakarta (timezone boundary)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Active-vs-Scheduled determination uses org timezone, not server UTC.
- **Preconditions:** Test executed near the UTC/WIB day boundary (e.g., 23:30 WIB = 16:30 UTC, where WIB date is one day ahead of UTC).
- **Steps:**
  1. Create a placement with start date = the **WIB** "today".
- **Expected result:** Status determined as Active because start ≤ today in Asia/Jakarta (BR-5, C-8), not flipped to Scheduled by a UTC comparison.
- **Traceability:** F3.1, BR-5, C-8.

#### TC-E3-F3.1-026 · Form field-error rendering on validation failure
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Error
- **Objective:** Server 400/422 field errors map to inline form errors.
- **Preconditions:** HR admin on the New Placement form.
- **Steps:**
  1. Submit with missing required fields (no agent, no position).
- **Expected result:** Inline field-level errors rendered from `error.fields` (CONVENTIONS §11/§12); submit blocked; no record created.
- **Traceability:** F3.1, BR-1, CONVENTIONS §11.

#### TC-E3-F3.1-027 · Loading state while submitting / picker fetch
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Pickers and submit show loading; failure shows retry.
- **Preconditions:** HR admin; simulate slow network for employee/company picker and submit.
- **Steps:**
  1. Open New Placement; observe picker loading states.
  2. Submit and observe a pending/disabled state until response.
  3. Simulate a 500 on submit.
- **Expected result:** Loading indicators while pickers/submit pending; on `500 INTERNAL` an error toast with retry; no duplicate record created (idempotency-key honored if used, CONVENTIONS §13).
- **Traceability:** F3.1, CONVENTIONS §7/§13.

### Web · Super Admin

#### TC-E3-F3.1-028 · Super admin creates with backdating (migration correction)
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P1 · **Type:** Happy/Edge
- **Objective:** Super admin can create + backdate with reason (migration corrections).
- **Preconditions:** Signed in as super admin; agent free.
- **Steps:**
  1. New Placement → agent → company/site → position.
  2. Backdate start to **2026-05-01**; enter reason "migration correction".
  3. Submit.
- **Expected result:** Created with warning; audit notes backdating (BR-6, C-3). Same invariant checks (INV-1, buffer) still apply.
- **Traceability:** F3.1, BR-6, C-3.

#### TC-E3-F3.1-029 · Super admin override on invariant still enforced (no escalation past INV-1)
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P1 · **Type:** Invariant
- **Objective:** Even super admin cannot create a second concurrent active placement violating INV-1.
- **Preconditions:** Agent Budi already has an active placement.
- **Steps:**
  1. New Placement → Budi → another company with overlapping dates; submit.
- **Expected result:** Blocked `409 INV_1_VIOLATION`. INV-1 is a hard data invariant; correction path is end/transfer, not a duplicate active record.
- **Traceability:** F3.1, **INV-1**, BR-2.

### Web · Shift Leader / Agent (RBAC)

#### TC-E3-F3.1-030 · RBAC — shift leader cannot create a placement
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Create is restricted to hr_admin/super_admin (and lead within scope); shift leader denied.
- **Preconditions:** Signed in as shift leader (derived from active assignment).
- **Steps:**
  1. Attempt to navigate to New Placement / call the create endpoint.
- **Expected result:** UI hides the create action; direct API call returns `403 FORBIDDEN`. Client renders no-permission state (`comp/EmptyNoPermission`).
- **Traceability:** F3.1, CONVENTIONS §17.

#### TC-E3-F3.1-031 · RBAC — agent (baseline) cannot create a placement
- [ ] **Platform:** Web · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Baseline employee has no placement-create capability.
- **Preconditions:** Signed in as a `FIELD` employee with no elevation.
- **Steps:**
  1. Attempt to access the create flow / call create endpoint.
- **Expected result:** `403 FORBIDDEN`; create surface not shown.
- **Traceability:** F3.1, CONVENTIONS §17.

### Mobile · Agent

#### TC-E3-F3.1-032 · Agent views own newly-created active placement (read)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** After HR creates an active placement, the agent sees it (company, site, position, period) on mobile.
- **Preconditions:** HR created an active placement for the signed-in agent (per TC-001).
- **Steps:**
  1. Open the mobile app → My Placement.
- **Expected result:** Active placement shown with company, site, position, start/end period (US-4). Read-only.
- **Traceability:** F3.1, US-4.

#### TC-E3-F3.1-033 · Agent does NOT see a future Scheduled placement as active
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** A Scheduled (future) placement is not surfaced as the current active placement.
- **Preconditions:** Agent has a Scheduled placement starting 2026-07-01 and no active placement.
- **Steps:**
  1. Open My Placement.
- **Expected result:** No active placement shown (or shown distinctly as "upcoming"); not presented as currently active (US-2/BR-5 semantics).
- **Traceability:** F3.1, US-2, BR-5.

#### TC-E3-F3.1-034 · Agent empty state — never placed
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** A never-placed agent sees an empty state, not an error.
- **Preconditions:** Agent with no placement, no history.
- **Steps:**
  1. Open My Placement.
- **Expected result:** Empty state ("not yet placed"); no error. Loading indicator while fetching; graceful retry on network error.
- **Traceability:** F3.1, US-4.

---

## F3.2 — Placement Lifecycle & Status

Traces PRD F3.2: LC-1..10, C-1..8, INV-1. State set: Draft, Scheduled, Active, Expiring, Ended, Terminated, Resigned, Superseded.

### Web · HR Admin — system-driven transitions

#### TC-E3-F3.2-001 · Scheduled → Active auto-activation on start date (Asia/Jakarta job)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant (valid transition)
- **Objective:** A Scheduled placement auto-activates on its start date via the daily job.
- **Preconditions:** Placement for Budi with status Scheduled, start date = **2026-06-17** (today).
- **Steps:**
  1. Trigger / wait for the daily activation job (Asia/Jakarta).
  2. Reload the placement.
- **Expected result:** Status becomes **Active** (LC-2). Budi + company shift leader notified. Audit entry by actor `system`.
- **Traceability:** F3.2, LC-2, transition Scheduled→Active.

#### TC-E3-F3.2-002 · Active → Expiring at 30 days before end (system)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant (valid transition)
- **Objective:** Active placement flips to Expiring exactly 30 days before end_date.
- **Preconditions:** Active placement with end_date = **2026-07-17** (30 days from today).
- **Steps:**
  1. Run the expiry job for today 2026-06-17.
  2. Reload placement.
- **Expected result:** Status becomes **Expiring** (LC-3). HR admin + shift leader receive an expiring notification (LC-10, CONVENTIONS §16.2). An Inbox Continue/End decision task is raised.
- **Traceability:** F3.2, LC-3, LC-10, transition Active→Expiring.

#### TC-E3-F3.2-003 · Open-ended placement never expires
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** A placement with no end_date is never flagged Expiring.
- **Preconditions:** Active placement with end_date = null.
- **Steps:**
  1. Run the expiry job.
- **Expected result:** Status remains **Active** (LC-3, C-4). No Expiring transition.
- **Traceability:** F3.2, LC-3, C-4.

#### TC-E3-F3.2-004 · INVARIANT/Negative — no system auto-end (grace persists past end_date)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant (no transition)
- **Objective:** Once end_date passes with no HR decision, the placement stays Expiring (grace); the system never auto-ends.
- **Preconditions:** Expiring placement whose end_date = **2026-06-16** (yesterday); no HR decision recorded.
- **Steps:**
  1. Run the daily job for 2026-06-17.
  2. Reload placement; check the agent's login.
- **Expected result:** Status **remains Expiring** (LC-4) — there is **no** Expiring/Active→Ended system transition. Agent's login stays valid (login revocation is employment-end only, F2.7 OB-6). HR Inbox still shows a pending Continue/End decision.
- **Traceability:** F3.2, LC-3, LC-4, C (grace), transition Expiring→Expiring.

#### TC-E3-F3.2-005 · Catch-up safe job after downtime
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** A missed job day does not skip due transitions; the job evaluates by date, not "today only".
- **Preconditions:** Several placements due to activate/expire on dates 2026-06-15 and 2026-06-16; the job did not run those days.
- **Steps:**
  1. Run the job on 2026-06-17 after the outage.
- **Expected result:** All due transitions (activations, expiries) for 2026-06-15/16/17 are applied (C-6). No transitions silently skipped.
- **Traceability:** F3.2, LC-2, LC-3, C-6.

### Web · HR Admin — HR-driven transitions

#### TC-E3-F3.2-006 · Active → Terminated early with reason + effective date
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant (valid transition)
- **Objective:** HR terminates an active placement early.
- **Preconditions:** Active placement for Budi.
- **Steps:**
  1. Open placement → Terminate.
  2. Enter a termination reason (required) + effective date.
  3. Confirm.
- **Expected result:** Status **Terminated** (LC-5); `termination_reason` stored; `ended_at` = effective date; audited. Notification fired. Login NOT revoked (placement-end only).
- **Traceability:** F3.2, LC-5, transition Active→Terminated.

#### TC-E3-F3.2-007 · Expiring → Terminated early
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Invariant (valid transition)
- **Objective:** HR can terminate an Expiring placement early.
- **Preconditions:** Expiring placement for Budi.
- **Steps:**
  1. Terminate with reason + effective date; confirm.
- **Expected result:** Status **Terminated** (LC-5); audited.
- **Traceability:** F3.2, LC-5, transition Expiring→Terminated.

#### TC-E3-F3.2-008 · Scheduled → Terminated (cancel before start)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Invariant (valid transition)
- **Objective:** A Scheduled placement can be cancelled (terminated) before it ever activates.
- **Preconditions:** Scheduled placement starting 2026-07-01.
- **Steps:**
  1. Terminate with reason; confirm.
- **Expected result:** Status **Terminated**; it never activates (C-3, state machine Scheduled→Terminated).
- **Traceability:** F3.2, LC-5, C-3, transition Scheduled→Terminated.

#### TC-E3-F3.2-009 · Active → Resigned (record agent resignation)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant (valid transition)
- **Objective:** Recording a resignation closes the active placement.
- **Preconditions:** Active placement for Budi.
- **Steps:**
  1. Open placement → Record resignation → set `resign_at` (e.g. 2026-07-31, may be future).
  2. Confirm.
- **Expected result:** Status **Resigned** with `resign_at` stored (LC-6). Future schedule (E4) cancelled from resign date (C-4). Employment agreement closure handled in E2.
- **Traceability:** F3.2, LC-6, C-4, transition Active→Resigned.

#### TC-E3-F3.2-010 · Active → Superseded via renewal (linked successor)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant (valid transition)
- **Objective:** Renewal creates a successor placement and supersedes the prior — history preserved, never edited in place.
- **Preconditions:** Expiring placement P1 for Budi at "Plaza Senayan" as "Parking Attendant", ending **2026-06-30**.
- **Steps:**
  1. Open P1 → Renew (same company + site + position).
  2. Set new period starting **2026-07-01** (day after P1 ends).
  3. Confirm.
- **Expected result:** New placement **P2** created with `predecessor_id` → P1; P1 set to **Superseded** effective P2's start date (LC-7). P2 satisfies the 1-day buffer (F3.1 BR-2). Both records retained; chain queryable.
- **Traceability:** F3.2, LC-7, transition Active/Expiring→Superseded, INV-1.

#### TC-E3-F3.2-011 · Renewal rejected when overlapping/same-day as predecessor (buffer)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative
- **Objective:** A renewal violating the 1-day buffer is rejected.
- **Preconditions:** P1 ends **2026-06-30**.
- **Steps:**
  1. Renew P1 with successor start **2026-06-30** (same day).
- **Expected result:** Rejected by the 1-day buffer (F3.1 BR-2, C-2). Neither P1 superseded nor P2 created.
- **Traceability:** F3.2, LC-7, C-2.

#### TC-E3-F3.2-012 · Renewal with a gap (successor starts >1 day later)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** A gap between predecessor end and successor start is allowed.
- **Preconditions:** P1 ends 2026-06-30.
- **Steps:**
  1. Renew with successor start **2026-07-10**.
- **Expected result:** Allowed; P1 ends naturally, P2 is **Scheduled** until 2026-07-10 (C-1).
- **Traceability:** F3.2, LC-7, C-1.

#### TC-E3-F3.2-013 · INVARIANT — only one non-terminal placement per agent over time
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Invariant
- **Objective:** Across an agent's history, exactly one placement is non-terminal at any moment.
- **Preconditions:** Budi has multiple historical placements (some Superseded/Ended) plus one Active.
- **Steps:**
  1. Open Budi's placement history.
- **Expected result:** Exactly one non-terminal record (INV-1); chain readable via predecessor/successor links (C-7).
- **Traceability:** F3.2, C-7, **INV-1**.

### Web · HR Admin — immutability & invalid transitions

#### TC-E3-F3.2-014 · INVARIANT — terminal placements are immutable (HR cannot edit)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant (invalid transition)
- **Objective:** Ended/Terminated/Resigned/Superseded records cannot be edited by HR admin.
- **Preconditions:** A placement with status **Ended**.
- **Steps:**
  1. HR admin attempts to edit its dates/fields.
- **Expected result:** Change rejected (LC-1); edit controls disabled. Only a Super Admin override is permitted.
- **Traceability:** F3.2, LC-1, invalid transition out of terminal.

#### TC-E3-F3.2-015 · Negative — invalid transition (e.g. Ended → Active) blocked
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative (invalid transition)
- **Objective:** Re-activating a terminal placement is not allowed.
- **Preconditions:** Placement with status Terminated.
- **Steps:**
  1. Attempt (via UI or direct API) to set status back to Active.
- **Expected result:** Rejected (not a defined transition, LC-1). Correct path is a new placement (F3.1) or renewal from a non-terminal record.
- **Traceability:** F3.2, LC-1.

#### TC-E3-F3.2-016 · Edit end_date into the past on an active placement (blocked per default)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Negative/Edge
- **Objective:** Setting end_date into the past is blocked (default decision: use Terminate instead).
- **Preconditions:** Active placement; today 2026-06-17.
- **Steps:**
  1. Attempt to set end_date = **2026-06-10** (past).
- **Expected result:** Per the §10 default, blocked with guidance to use Terminate (C-5). (If the build chose the "immediate end-of-term" alternative, document the deviation.)
- **Traceability:** F3.2, C-5.

#### TC-E3-F3.2-017 · Ending the shift leader's own placement triggers a vacancy
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant (side-effect)
- **Objective:** A terminal transition for a placement whose agent leads that company raises a leader-vacancy.
- **Preconditions:** Budi is the shift leader of "Plaza Senayan" and actively placed there.
- **Steps:**
  1. Terminate Budi's Plaza Senayan placement.
- **Expected result:** Placement Terminated; a **shift-leader vacancy** is raised for Plaza Senayan and Budi's assignment is auto-vacated with reason `PlacementEnded` (LC-8, F3.4 SL-6). His `shift_leader` role lapses next request (SL-10).
- **Traceability:** F3.2, LC-8, F3.4 SL-6/SL-10, **INV-2**.

#### TC-E3-F3.2-018 · Every transition writes an audit entry + notification
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Each lifecycle transition is audited (actor/system, before/after, reason) and notifies the right people.
- **Preconditions:** A placement subjected to terminate/renew/resign.
- **Steps:**
  1. Perform a transition; check audit log (E1) and notifications (E10).
- **Expected result:** Audit entry with before/after + reason; matching notification dispatched (LC-9, LC-10). Expiring → HR + leader; activation → agent + leader.
- **Traceability:** F3.2, LC-9, LC-10.

#### TC-E3-F3.2-019 · PKWT extended after renewal — successor may extend to new agreement end
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** If the PKWT is extended, the renewed successor can run to the new agreement end (auto-cap recomputed).
- **Preconditions:** Budi's PKWT extended to 2027-06-30; renewing an expiring placement.
- **Steps:**
  1. Renew with successor end up to 2027-06-30.
- **Expected result:** Successor allowed up to the new agreement end (F3.1 BR-1b, C-8); auto-cap reflects the new end.
- **Traceability:** F3.2, C-8, F3.1 BR-1b.

### Web · Super Admin

#### TC-E3-F3.2-020 · Super Admin override edits a terminal placement
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Only super admin can correct a terminal (immutable) record.
- **Preconditions:** Placement status Ended; signed in as super admin.
- **Steps:**
  1. Open the terminal placement → use the super-admin correction action; change a field with a reason.
- **Expected result:** Change permitted (LC-1 override); audited as a correction. HR admin attempting the same is blocked (cross-check TC-014).
- **Traceability:** F3.2, LC-1.

### Web · Shift Leader / Mobile · Agent (read)

#### TC-E3-F3.2-021 · RBAC — shift leader cannot trigger lifecycle transitions
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Terminate/renew/resign actions are hidden/denied for shift leader.
- **Preconditions:** Signed in as shift leader of a company; viewing a placement in their roster.
- **Steps:**
  1. Attempt terminate/renew via UI or direct API.
- **Expected result:** Actions hidden; direct calls return `403 FORBIDDEN`/`OUT_OF_SCOPE`.
- **Traceability:** F3.2, CONVENTIONS §17.

#### TC-E3-F3.2-022 · Mobile — agent sees status change reflected (Active → Terminated)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** When HR terminates the agent's placement, the agent's mobile view reflects the new status and login still works.
- **Preconditions:** Agent had an active placement; HR terminates it (TC-006).
- **Steps:**
  1. Agent opens My Placement after the change.
- **Expected result:** Placement shown as ended/terminated (or no current active placement); the agent can **still log in** (placement-end never revokes login, LC-4/F2.7 OB-2).
- **Traceability:** F3.2, LC-4.

---

## F3.3 — Re-placement & Transfer (with history)

Traces PRD F3.3: TR-1..9, C-1..8, INV-1/INV-5. Action: `POST /placements/{id}:transfer`.

### Web · HR Admin

#### TC-E3-F3.3-001 · Transfer an agent to a new company (happy path, history preserved)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Move Budi to a new company; old placement closed `Transferred`, linked successor created.
- **Preconditions:** Budi has an active placement at "Mall Kelapa Gading" as "Parking Attendant". "Plaza Senayan" is active.
- **Steps:**
  1. Open Budi's placement → Transfer.
  2. Choose company "Plaza Senayan", site, position "Building Technician", start **2026-06-22** (next Monday).
  3. Confirm.
- **Expected result:** Old placement closed with `ended_reason = Transferred`, `ended_at = 2026-06-21` (newStart − 1 day, TR-2). New Active/Scheduled placement at Plaza Senayan created with `predecessor_id` → old (TR-3). Budi, old leader, new leader notified (TR-8). History chain queryable (TR-9), old shows "Parking Attendant", new shows "Building Technician".
- **Traceability:** F3.3, TR-1, TR-2, TR-3, TR-8, TR-9, INV-1.

#### TC-E3-F3.3-002 · Transfer effective immediately (new start = today)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy/Edge
- **Objective:** Immediate transfer closes old yesterday, new active today.
- **Preconditions:** Budi active at Mall Kelapa Gading.
- **Steps:**
  1. Transfer to Plaza Senayan with start **2026-06-17** (today).
- **Expected result:** Old `ended_at = 2026-06-16` (yesterday); new placement **Active** today (C-1, TR-2).
- **Traceability:** F3.3, C-1, TR-2.

#### TC-E3-F3.3-003 · Transfer with future start (successor Scheduled, old stays active until buffer)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy/Edge
- **Objective:** Future-dated transfer keeps old active until newStart−1.
- **Preconditions:** Budi active at Mall Kelapa Gading.
- **Steps:**
  1. Transfer to Plaza Senayan with start **2026-07-01**.
- **Expected result:** New placement **Scheduled**; old stays Active until **2026-06-30** then closed `Transferred` (C-2, TR-2).
- **Traceability:** F3.3, C-2, TR-2.

#### TC-E3-F3.3-004 · Position-only change at same company/site is a valid transfer
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy/Edge
- **Objective:** Changing only the position (same company + site) is a valid transfer (post service-line removal).
- **Preconditions:** Budi active at Plaza Senayan as "Parking Attendant".
- **Steps:**
  1. Transfer Budi at the **same** company + site, new position "Lobby Supervisor", new period.
- **Expected result:** Accepted as a transfer (TR-1, C-3, BR-9). Old closed `Transferred`, successor with new position.
- **Traceability:** F3.3, TR-1, TR-7, C-3.

#### TC-E3-F3.3-005 · Transfer requires an actual change (reject no-op)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Same company + same site + same position is not a transfer.
- **Preconditions:** Budi active at Plaza Senayan as "Parking Attendant".
- **Steps:**
  1. Attempt a "transfer" with identical company, site, and position.
- **Expected result:** Rejected — "this is a renewal (F3.2), not a transfer" (TR-1 requires a different company OR site OR position).
- **Traceability:** F3.3, TR-1.

#### TC-E3-F3.3-006 · INVARIANT — transfer is atomic (rollback on successor failure)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant
- **Objective:** If the successor creation fails the buffer/validation, the old placement is NOT closed.
- **Preconditions:** Budi active at Mall Kelapa Gading ending 2026-06-30; attempt a transfer with start 2026-06-30 (same-day, buffer violation).
- **Steps:**
  1. Transfer with new start **2026-06-30**.
  2. Confirm.
- **Expected result:** Buffer validation error; **neither** old closed **nor** new created (TR-6, C — atomicity). Old placement remains Active.
- **Traceability:** F3.3, TR-6, F3.1 BR-2, **INV-1**.

#### TC-E3-F3.3-007 · Transfer of a shift leader vacates their leadership (old company vacancy)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant (side-effect)
- **Objective:** Transferring the leader of company A vacates A's leadership and raises a vacancy.
- **Preconditions:** Budi is the shift leader of "Mall Kelapa Gading" (and placed there).
- **Steps:**
  1. Transfer Budi to "Plaza Senayan".
- **Expected result:** Budi's Mall Kelapa Gading leadership ended (`vacated_reason = PlacementEnded`); vacancy raised for Mall Kelapa Gading (TR-4, F3.4 SL-6). His leader scope lapses (SL-10). New placement created.
- **Traceability:** F3.3, TR-4, F3.4 SL-6, **INV-2**.

#### TC-E3-F3.3-008 · Warn when the destination company has no leader (transfer still succeeds)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** A leaderless destination warns but does not block the transfer.
- **Preconditions:** "Plaza Senayan" has no shift leader.
- **Steps:**
  1. Transfer Budi to Plaza Senayan.
- **Expected result:** Transfer succeeds; warning to assign a leader for Plaza Senayan linking to its "Pemimpin Shift" tab (TR-5).
- **Traceability:** F3.3, TR-5.

#### TC-E3-F3.3-009 · Transfer an Expiring placement (treated like active)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** An Expiring placement is transferable.
- **Preconditions:** Budi's placement at Mall Kelapa Gading is Expiring.
- **Steps:**
  1. Transfer Budi to Plaza Senayan.
- **Expected result:** Allowed; treated like active (C-4, TR-1). Old closed `Transferred`, successor created.
- **Traceability:** F3.3, TR-1, C-4.

#### TC-E3-F3.3-010 · Block transfer to an inactive/archived destination
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Destination must be an active company (F3.1 BR-3).
- **Preconditions:** Destination "Old Tower" archived.
- **Steps:**
  1. Attempt transfer of Budi to "Old Tower".
- **Expected result:** Blocked "Company is not active" (C-5, F3.1 BR-3). Old placement unchanged (atomic).
- **Traceability:** F3.3, C-5, F3.1 BR-3, TR-6.

#### TC-E3-F3.3-011 · Transfer attempted on an agent with no active placement (use F3.1)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Transfer requires a current Active/Expiring placement.
- **Preconditions:** Agent "Citra" whose only placement has Ended (no active).
- **Steps:**
  1. Attempt to transfer Citra.
- **Expected result:** Blocked / no transfer action available — must create a new placement via F3.1 (C-6, TR-1).
- **Traceability:** F3.3, C-6, TR-1.

#### TC-E3-F3.3-012 · Backdated transfer by HR admin with reason
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Backdated transfer allowed for HR admin with reason; history dates adjusted; audited.
- **Preconditions:** Budi active; transfer with start in the past.
- **Steps:**
  1. Transfer Budi with start **2026-06-01**; provide a backdate reason.
- **Expected result:** Allowed; old `ended_at = 2026-05-31`; new active; audit records reason (C-7, F3.1 BR-6).
- **Traceability:** F3.3, C-7, F3.1 BR-6.

#### TC-E3-F3.3-013 · PKWT agreement ends before new placement end (successor auto-capped)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Successor end auto-capped to PKWT agreement end on transfer.
- **Preconditions:** Budi's PKWT ends 2026-12-31; transfer with new end 2027-03-31.
- **Steps:**
  1. Transfer with end **2027-03-31**.
- **Expected result:** Successor end auto-capped to **2026-12-31** (C-8, F3.1 BR-1b); creator notified.
- **Traceability:** F3.3, C-8, F3.1 BR-1b.

#### TC-E3-F3.3-014 · Transfer of an awaiting-agreement placement keeps successor pending
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Transferring a pending placement propagates the awaiting flag (no auto-cap).
- **Preconditions:** Budi's active placement is flagged awaiting_agreement.
- **Steps:**
  1. Transfer Budi to a new company.
- **Expected result:** Successor created **also awaiting_agreement**; BR-1b skipped (F3.1 C-11/BR-10).
- **Traceability:** F3.3, F3.1 C-11/BR-10.

#### TC-E3-F3.3-015 · INVARIANT — transfer never produces a second active placement
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant
- **Objective:** After transfer, the agent still has exactly one active/non-terminal placement.
- **Preconditions:** Budi active at Mall Kelapa Gading.
- **Steps:**
  1. Transfer to Plaza Senayan (immediate); inspect Budi's placements.
- **Expected result:** Old = Transferred (terminal), new = Active. Exactly one non-terminal (INV-1). INV-5 satisfied (new placement has a valid site of the new company).
- **Traceability:** F3.3, **INV-1**, **INV-5**.

### Web · Shift Leader / Agent (RBAC) & Mobile

#### TC-E3-F3.3-016 · RBAC — shift leader cannot transfer
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Transfer action restricted to HR/super admin (and lead within scope).
- **Preconditions:** Shift leader viewing a roster placement.
- **Steps:**
  1. Attempt transfer via UI/API.
- **Expected result:** Hidden; direct call `403 FORBIDDEN`.
- **Traceability:** F3.3, CONVENTIONS §17.

#### TC-E3-F3.3-017 · Mobile — agent sees the new placement after transfer; old in history
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Agent's mobile view shows the new company/site/position; old placement appears in history.
- **Preconditions:** HR transferred the signed-in agent (TC-001).
- **Steps:**
  1. Open My Placement; open My History.
- **Expected result:** Current = new placement (new company/site/position). History shows the prior placement marked Transferred. Login unaffected.
- **Traceability:** F3.3, TR-9, US-4.

---

## F3.4 — Shift-Leader Assignment

Traces PRD F3.4: SL-0..11, C-1..6, INV-2/INV-3/INV-4. Single entry point = client-company detail **"Pemimpin Shift" tab** (E2 F2.3, SL-11).

### Web · HR Admin (company-scope leadership unit)

#### TC-E3-F3.4-001 · Assign a company's first shift leader (happy path)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Designate an actively-placed agent as the company's shift leader; role scope granted.
- **Preconditions:** "Plaza Senayan" (`leader_scope = company`) active, no current leader. Budi has an **active** placement at Plaza Senayan.
- **Steps:**
  1. Open Plaza Senayan → "Pemimpin Shift" tab.
  2. Pick candidate "Budi"; confirm.
- **Expected result:** `ShiftLeaderAssignment` created; Budi gains `shift_leader` role scoped to Plaza Senayan (SL-5, SL-10) — effective next request, no re-login. No previous assignment to end (C-1). Budi + company agents notified (SL-8). Audited (SL-8).
- **Traceability:** F3.4, SL-1, SL-2, SL-5, SL-10, C-1, **INV-2**, **INV-4**.

#### TC-E3-F3.4-002 · INVARIANT — reject a candidate not placed at the company (INV-4)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant
- **Objective:** The leader must be an agent actively placed within the unit.
- **Preconditions:** "Andi" is NOT placed at Plaza Senayan.
- **Steps:**
  1. Plaza Senayan "Pemimpin Shift" tab → try to pick "Andi".
- **Expected result:** Blocked "Candidate must be placed at this company" (SL-2). Andi not selectable / `409`/`422` on force.
- **Traceability:** F3.4, SL-2, **INV-4**.

#### TC-E3-F3.4-003 · INVARIANT — candidate with only a Scheduled placement is rejected (must be active)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Invariant
- **Objective:** A not-yet-active (Scheduled) placement does not satisfy INV-4.
- **Preconditions:** "Citra" has a **Scheduled** placement at Plaza Senayan (starts 2026-07-01).
- **Steps:**
  1. Try to assign Citra as leader.
- **Expected result:** Blocked — leader must be **actively** placed (SL-2, C-2).
- **Traceability:** F3.4, SL-2, C-2, **INV-4**.

#### TC-E3-F3.4-004 · INVARIANT — strict 1:1: reject assigning someone who already leads another unit (INV-3)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant
- **Objective:** A person may lead only one unit at a time.
- **Preconditions:** Budi already leads "Mall Kelapa Gading"; he is also (hypothetically) placed at Plaza Senayan.
- **Steps:**
  1. Plaza Senayan "Pemimpin Shift" tab → try to assign Budi.
- **Expected result:** Blocked "a shift leader is strictly 1:1 with a company" (SL-3) — reassign/vacate the other first. `409 INV_3_VIOLATION`.
- **Traceability:** F3.4, SL-3, **INV-3**.

#### TC-E3-F3.4-005 · INVARIANT — reassign: assigning a new leader ends the previous one atomically (INV-2)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant
- **Objective:** A unit has at most one active leader; reassign ends the prior.
- **Preconditions:** Budi is the current leader of Plaza Senayan; Citra has an active placement there.
- **Steps:**
  1. Plaza Senayan "Pemimpin Shift" tab → assign Citra; confirm.
- **Expected result:** Budi's assignment ended (`vacated_reason = Reassigned`, `unassigned_at = now`) and his scope revoked, **atomically** (SL-4). Citra gains the scope. Both leaders + agents notified. Exactly one active assignment (INV-2).
- **Traceability:** F3.4, SL-1, SL-4, **INV-2**.

#### TC-E3-F3.4-006 · INVARIANT — concurrent assignment of two leaders to one unit (unique-active constraint)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Invariant
- **Objective:** Two admins assigning different leaders to the same unit — only one commits.
- **Preconditions:** Two active candidates at Plaza Senayan; no current leader; two HR sessions.
- **Steps:**
  1. Session A assigns candidate X; session B assigns candidate Y near-simultaneously.
- **Expected result:** Unique-active constraint (SL-1) makes the second commit fail (`409`); it can be retried as a reassignment (C-4). Never two active leaders.
- **Traceability:** F3.4, SL-1, C-4, **INV-2**.

#### TC-E3-F3.4-007 · Auto-vacate when the leader's placement ends (PlacementEnded)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant (side-effect)
- **Objective:** Ending/terminating/transferring/expiring-and-ended the leader's placement auto-vacates the role and raises a vacancy.
- **Preconditions:** Budi leads Plaza Senayan and is placed there.
- **Steps:**
  1. Terminate Budi's Plaza Senayan placement (F3.2).
- **Expected result:** Assignment auto-vacated with `vacated_reason = PlacementEnded`; vacancy raised (SL-6). His `shift_leader` role lapses next request (SL-10).
- **Traceability:** F3.4, SL-6, SL-10, F3.2 LC-8, **INV-2**.

#### TC-E3-F3.4-008 · Vacancy: approvals escalate to HR while no leader
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Edge
- **Objective:** A company may temporarily have no leader; approvals escalate to HR.
- **Preconditions:** Plaza Senayan has no active leader; an agent there submits a leave request (E6).
- **Steps:**
  1. Observe the approval routing for that request.
- **Expected result:** Approval routed to an HR admin until a leader is filled (SL-7). Stop-gap, not permanent.
- **Traceability:** F3.4, SL-7.

#### TC-E3-F3.4-009 · Assignment history retained (never hard-deleted)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Past assignments remain queryable.
- **Preconditions:** A company that has had ≥2 leaders over time.
- **Steps:**
  1. Open the "Pemimpin Shift" tab history.
- **Expected result:** All prior assignments shown with assigned/unassigned timestamps + `vacated_reason` (SL-9). None hard-deleted.
- **Traceability:** F3.4, SL-9.

#### TC-E3-F3.4-010 · Self-assignment by an HR admin who is also placed there (allowed)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** An HR admin who is themselves actively placed at the company may be assigned leader if invariants hold.
- **Preconditions:** HR admin user has an active placement at the company and does not lead elsewhere.
- **Steps:**
  1. Assign themselves as leader.
- **Expected result:** Allowed if INV-2/3/4 hold; audited (C-6).
- **Traceability:** F3.4, C-6.

#### TC-E3-F3.4-011 · Company archived while it has a leader (assignment vacated)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Archiving a company vacates its leader and stops accepting placements.
- **Preconditions:** Plaza Senayan active with leader Budi.
- **Steps:**
  1. Archive Plaza Senayan (E2).
- **Expected result:** Leader assignment vacated; company no longer accepts placements (C-5, F3.1 BR-3).
- **Traceability:** F3.4, C-5.

### Web · HR Admin (site-scope leadership unit)

#### TC-E3-F3.4-012 · Per-site leadership when leader_scope = site (two sites, two leaders)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant
- **Objective:** With `leader_scope = site`, each active site gets exactly one leader, scoped to that site.
- **Preconditions:** "Plaza Group" with `leader_scope = site`; sites "Plaza Senayan" and "Plaza Indonesia" each with active placements; Budi placed at Plaza Senayan, Sari placed at Plaza Indonesia.
- **Steps:**
  1. Assign Budi as leader of site "Plaza Senayan".
  2. Assign Sari as leader of site "Plaza Indonesia".
- **Expected result:** Each site has exactly one leader scoped to that site; `ShiftLeaderAssignment.site_id` set per site (SL-0, SL-1). Uniqueness on `(client_company_id, site_id)` (INV-2 per unit).
- **Traceability:** F3.4, SL-0, SL-1, SL-2, **INV-2/INV-3**.

#### TC-E3-F3.4-013 · INVARIANT — site-scope strict 1:1: Budi cannot also lead a second site
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Invariant
- **Objective:** A person leads only one unit even under site-scope.
- **Preconditions:** Budi already leads site "Plaza Senayan".
- **Steps:**
  1. Try to also assign Budi to lead site "Plaza Indonesia".
- **Expected result:** Blocked — strict 1:1 per unit (SL-3, INV-3). `409 INV_3_VIOLATION`.
- **Traceability:** F3.4, SL-3, **INV-3**.

#### TC-E3-F3.4-014 · INVARIANT — site-scope candidate must be placed at THAT site (INV-4)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Invariant
- **Objective:** Under site-scope, the leader must be actively placed at that specific site, not just the company.
- **Preconditions:** "Plaza Group" site-scope; candidate placed at "Plaza Indonesia" but you attempt to assign them to lead "Plaza Senayan".
- **Steps:**
  1. Assign that candidate as leader of site "Plaza Senayan".
- **Expected result:** Blocked — must be placed at that site (SL-2, INV-4).
- **Traceability:** F3.4, SL-2, **INV-4**.

#### TC-E3-F3.4-015 · leader_scope switch company→site flags existing company-level assignment for re-designation
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Switching scope flags the prior company-level leader for per-site re-designation.
- **Preconditions:** Company with a company-level leader; switch `leader_scope` to `site` (E2 F2.6 C-3).
- **Steps:**
  1. Change leader_scope to site.
  2. Open the "Pemimpin Shift" tab.
- **Expected result:** The existing company-level assignment is flagged for re-designation per site (open item — confirm transition UX). No silent loss of leadership data.
- **Traceability:** F3.4, SL-0, §10 open item, E2 F2.6 C-3.

### Web · Super Admin & RBAC

#### TC-E3-F3.4-016 · Super admin can assign/reassign/vacate leaders (parity with HR)
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Super admin has the same assign powers.
- **Preconditions:** Signed in as super admin; valid candidate.
- **Steps:**
  1. Assign a leader from the "Pemimpin Shift" tab.
- **Expected result:** Succeeds with the same invariant checks as HR admin.
- **Traceability:** F3.4, SL-1..5.

#### TC-E3-F3.4-017 · RBAC — lead cannot assign shift leaders (FEATURE §2)
- [ ] **Platform:** Web · **POV:** HR admin (lead persona) · **Priority:** P0 · **Type:** RBAC
- **Objective:** Lead arranges placements but cannot assign shift leaders.
- **Preconditions:** Signed in as a `lead` scoped to a company.
- **Steps:**
  1. Open the company "Pemimpin Shift" tab / call the assign endpoint.
- **Expected result:** Assign action hidden/denied for lead; `403 FORBIDDEN` on direct call (FEATURE §2 — "Cannot … assign shift leaders").
- **Traceability:** F3.4, FEATURE §2, CONVENTIONS §17.

#### TC-E3-F3.4-018 · RBAC — shift leader cannot assign/replace leaders
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** A shift leader has no leader-management capability.
- **Preconditions:** Signed in as a shift leader.
- **Steps:**
  1. Attempt to open the assign action / call endpoint.
- **Expected result:** `403 FORBIDDEN`; action hidden.
- **Traceability:** F3.4, CONVENTIONS §17.

#### TC-E3-F3.4-019 · Single entry point — placement-detail leader card is read-only and links to the tab
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Leader management happens only via the company "Pemimpin Shift" tab; the placement-detail card is read-only.
- **Preconditions:** A placement at a company with a leader.
- **Steps:**
  1. Open the placement detail → shift-leader card.
- **Expected result:** Card is **read-only**, shows the current leader, and links to the company "Pemimpin Shift" tab (SL-11). No inline assign here.
- **Traceability:** F3.4, SL-11.

### Mobile · Shift Leader (read self-grant)

#### TC-E3-F3.4-020 · Mobile — newly-assigned leader gains roster/approval access on next request (no re-login)
- [ ] **Platform:** Mobile · **POV:** Shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Derived role takes effect on the next request without re-login.
- **Preconditions:** Budi is an agent on mobile; HR assigns him leader of Plaza Senayan while he is logged in.
- **Steps:**
  1. After assignment, Budi performs his next action (e.g. pull-to-refresh / open roster).
- **Expected result:** Budi's effective role becomes `shift_leader` scoped to Plaza Senayan on the next request (SL-10); he gains roster/approval access without re-authenticating.
- **Traceability:** F3.4, SL-10, SL-5.

#### TC-E3-F3.4-021 · Mobile — revoked leader loses access on next request
- [ ] **Platform:** Mobile · **POV:** Shift leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** Reassign/vacate strips the derived scope on the next request.
- **Preconditions:** Budi is leader of Plaza Senayan on mobile; HR reassigns leadership to Citra.
- **Steps:**
  1. After reassignment, Budi attempts a leader action (verify attendance / open roster).
- **Expected result:** Budi falls back to baseline `self.*` (no elevation); leader actions return `403`/`OUT_OF_SCOPE` on the next request (SL-10). No re-login required to drop the scope.
- **Traceability:** F3.4, SL-10.

---

## F3.5 — Company Placement Roster

Traces PRD F3.5: RO-1..8, C-1..6, INV-2. Read-only projection over Placement + Employee + Site + ShiftLeaderAssignment.

### Web · HR Admin

#### TC-E3-F3.5-001 · HR admin views a company roster (active + scheduled default)
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Roster lists active+scheduled placements with agent, position, site, period, status, and the current leader, plus summary counts.
- **Preconditions:** "Plaza Senayan" has 12 active placements across "Parking Attendant" and "Building Technician"; Budi is its shift leader.
- **Steps:**
  1. Open the Plaza Senayan roster.
- **Expected result:** All active + scheduled placements shown with agent, position, site, period, status (RO-1, RO-2). Budi listed as shift leader (RO-1). Summary counts by position and by status (RO-5). Sorted active-first then agent name; paginated (RO-8).
- **Traceability:** F3.5, RO-1, RO-2, RO-5, RO-8.

#### TC-E3-F3.5-002 · Filter by position (free-text match) updates counts
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Position filter restricts rows and recomputes counts.
- **Preconditions:** Roster as above.
- **Steps:**
  1. Filter by position "Parking Attendant".
- **Expected result:** Only "Parking Attendant" placements shown; counts update (RO-3, RO-5).
- **Traceability:** F3.5, RO-3, RO-5.

#### TC-E3-F3.5-003 · Filter by status and by period (date-range overlap), combinable
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy/Edge
- **Objective:** Status + period filters combine; period uses overlap semantics.
- **Preconditions:** Roster with placements spanning various periods; one placement spans the filter boundary.
- **Steps:**
  1. Filter status = Active AND period = **2026-06-01 → 2026-06-30**.
- **Expected result:** Rows whose period **overlaps** the range are included (a boundary-spanning placement is included, C-3); filters combine (RO-3).
- **Traceability:** F3.5, RO-3, C-3.

#### TC-E3-F3.5-004 · Include history toggle surfaces terminal placements
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Enabling history shows ended/terminated/resigned/transferred/superseded placements.
- **Preconditions:** Company with terminal placements in history.
- **Steps:**
  1. Toggle "include history".
- **Expected result:** Terminal placements appear in addition to active+scheduled (RO-2). With history off, only active+scheduled.
- **Traceability:** F3.5, RO-2.

#### TC-E3-F3.5-005 · awaiting_agreement filter surfaces the pending backlog
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy/Edge
- **Objective:** The roster exposes an awaiting_agreement filter (BR-10).
- **Preconditions:** Company with some placements flagged awaiting_agreement.
- **Steps:**
  1. Apply the "awaiting agreement" filter.
- **Expected result:** Only placements with `awaiting_agreement = true` shown (F3.1 BR-10). It is a compliance flag, orthogonal to status.
- **Traceability:** F3.5, F3.1 BR-10.

#### TC-E3-F3.5-006 · Agent with superseded + active placement (post-renewal) display
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** After renewal, the active placement shows by default; the superseded one only with history.
- **Preconditions:** Budi has an Active P2 (predecessor P1 Superseded) at the company.
- **Steps:**
  1. View roster (history off), then toggle history on.
- **Expected result:** Default shows only P2 (Active); with history, P1 (Superseded) also appears (C-6, RO-2).
- **Traceability:** F3.5, C-6.

#### TC-E3-F3.5-007 · Export reflects active filters and is audited
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Export to Excel/PDF respects current filters and writes an audit entry.
- **Preconditions:** Roster filtered by position "Building Technician".
- **Steps:**
  1. Export to Excel.
- **Expected result:** File contains only "Building Technician" placements (RO-6). An audit entry records who exported what, when (RO-6). Large result sets stream/queue (`202 Accepted`, C-2) — point-in-time snapshot (C-5).
- **Traceability:** F3.5, RO-6, C-2, C-5.

#### TC-E3-F3.5-008 · Empty company roster shows empty state + create prompt
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** A company with no placements shows an empty state prompting first placement (F3.1).
- **Preconditions:** "New Tower" has no placements.
- **Steps:**
  1. Open New Tower roster.
- **Expected result:** Empty state with a prompt to create the first placement (F3.1) — not an error.
- **Traceability:** F3.5, AC (empty company).

#### TC-E3-F3.5-009 · Company with no shift leader — roster shows assign prompt linking to tab
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Empty/Edge
- **Objective:** A leaderless company shows "No shift leader — assign one" linking to the "Pemimpin Shift" tab.
- **Preconditions:** Company with placements but no leader.
- **Steps:**
  1. Open the roster.
- **Expected result:** "No shift leader — assign one" prompt links to the company "Pemimpin Shift" tab (C-1, RO-7, F3.4 SL-11). The "Ganti" action also deep-links there.
- **Traceability:** F3.5, C-1, RO-7, F3.4 SL-11.

#### TC-E3-F3.5-010 · Read-only — row actions deep-link to F3.1–F3.4, no inline mutation
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P1 · **Type:** Happy
- **Objective:** The roster does not mutate; row actions navigate to the relevant edit flows.
- **Preconditions:** Roster open.
- **Steps:**
  1. Use a row action (e.g. open placement, transfer, "Ganti" leader).
- **Expected result:** Deep-links to F3.1–F3.4 surfaces; no mutation happens from the roster itself (RO-7).
- **Traceability:** F3.5, RO-7.

#### TC-E3-F3.5-011 · Large company — server-side pagination + cursor
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Edge
- **Objective:** 1000+ placements paginate server-side via cursor; export queues.
- **Preconditions:** Company with 1000+ placements.
- **Steps:**
  1. Open the roster; page through; export.
- **Expected result:** Cursor pagination (`limit`/`cursor`, CONVENTIONS §8); export streams/queues (C-2). No offset pagination.
- **Traceability:** F3.5, C-2, CONVENTIONS §8.

#### TC-E3-F3.5-012 · Loading + error states on roster fetch
- [ ] **Platform:** Web · **POV:** HR admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Roster shows loading skeleton; handles fetch error with retry.
- **Preconditions:** Simulate slow/failed roster fetch.
- **Steps:**
  1. Open roster under slow network, then under a forced 500.
- **Expected result:** Loading skeleton while pending; on `500` an error state with retry; no broken table.
- **Traceability:** F3.5, CONVENTIONS §7.

### Web · Super Admin

#### TC-E3-F3.5-013 · Super admin opens any company roster
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Global scope — any company's roster.
- **Preconditions:** Signed in as super admin.
- **Steps:**
  1. Open rosters for multiple different companies.
- **Expected result:** All companies accessible (RO-4, global scope).
- **Traceability:** F3.5, RO-4.

### Web · Shift Leader

#### TC-E3-F3.5-014 · Shift leader opens the roster of the company they lead
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** Happy
- **Objective:** A shift leader sees their own company's roster.
- **Preconditions:** Budi is the shift leader of Plaza Senayan.
- **Steps:**
  1. Budi opens rosters → Plaza Senayan.
- **Expected result:** Plaza Senayan roster loads (RO-4) with the same columns/counts; read-only.
- **Traceability:** F3.5, RO-4.

#### TC-E3-F3.5-015 · RBAC — shift leader cannot open another company's roster (scope)
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** A shift leader is restricted to the unit they lead; others hidden/403.
- **Preconditions:** Budi leads Plaza Senayan only.
- **Steps:**
  1. Budi lists companies (should only show Plaza Senayan or hide others).
  2. Budi deep-links to another company's roster URL (e.g. Mall Kelapa Gading).
- **Expected result:** Other companies hidden in the list; direct deep-link returns `403`/`OUT_OF_SCOPE` (RO-4, C-4). Client renders no-permission state.
- **Traceability:** F3.5, RO-4, C-4, CONVENTIONS §17.

#### TC-E3-F3.5-016 · RBAC — site-scope leader sees only their site's roster slice
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P1 · **Type:** RBAC/Edge
- **Objective:** Under site-scope, a leader's roster is bounded to their site.
- **Preconditions:** "Plaza Group" site-scope; Budi leads site "Plaza Senayan".
- **Steps:**
  1. Budi opens the roster.
- **Expected result:** Budi sees the Plaza Senayan **site** roster only, not Plaza Indonesia (RO-4 per leadership unit, SL-0).
- **Traceability:** F3.5, RO-4, F3.4 SL-0.

### Web · Agent (RBAC)

#### TC-E3-F3.5-017 · RBAC — agent cannot open any company roster
- [ ] **Platform:** Web · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Baseline employee has no roster access.
- **Preconditions:** Signed in as a `FIELD` employee with no elevation.
- **Steps:**
  1. Attempt to access a roster URL / endpoint.
- **Expected result:** `403 FORBIDDEN` (or `404` to avoid leaking); no roster surface available to baseline employees.
- **Traceability:** F3.5, RO-4, CONVENTIONS §17.

### Mobile · Shift Leader

#### TC-E3-F3.5-018 · Mobile — shift leader views own company roster
- [ ] **Platform:** Mobile · **POV:** Shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Condensed roster of the led company on mobile.
- **Preconditions:** Budi leads Plaza Senayan; logged in on mobile.
- **Steps:**
  1. Open the roster on mobile.
- **Expected result:** Plaza Senayan roster (agent, position, site, status) read-only, scoped to the led unit (RO-4).
- **Traceability:** F3.5, RO-4.

#### TC-E3-F3.5-019 · Mobile — RBAC: leader cannot reach a non-led company roster
- [ ] **Platform:** Mobile · **POV:** Shift leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** Scope enforced on mobile too.
- **Preconditions:** Budi leads only Plaza Senayan.
- **Steps:**
  1. Attempt to navigate to another company's roster (deep link / forced request).
- **Expected result:** Blocked `403`/`OUT_OF_SCOPE` (RO-4, C-4).
- **Traceability:** F3.5, RO-4, C-4.

---

## Appendix — Invariant traceability summary

| Invariant | Statement | Covered by |
|---|---|---|
| **INV-1** | At most one *active* placement per agent | F3.1-012/013/023/029, F3.2-013, F3.3-006/015 |
| **INV-2** | A leadership unit with active placements has exactly one shift leader | F3.2-017, F3.3-007, F3.4-001/005/006/007/012 |
| **INV-3** | A shift leader leads exactly one unit (strict 1:1) | F3.4-004/012/013 |
| **INV-4** | Designated leader must be actively placed within that unit | F3.4-001/002/003/014 |
| **INV-5** | A placement is located at exactly one Site belonging to its company | F3.1-003/004/018, F3.3-015 |
