# Test Cases · E4 — Shift Configuration & Scheduling

> **Epic:** E4 · **Type:** Manual test cases (QA execution checklist)
> **Source specs:** [FEATURE.md](../epics/E4-shift-scheduling/FEATURE.md) · PRDs: [shift-master-catalog](../epics/E4-shift-scheduling/prds/shift-master-catalog.md) (F4.1) · [daily-schedule-assignment](../epics/E4-shift-scheduling/prds/daily-schedule-assignment.md) (F4.2) · [schedule-views](../epics/E4-shift-scheduling/prds/schedule-views.md) (F4.3) · [schedule-changes-swaps](../epics/E4-shift-scheduling/prds/schedule-changes-swaps.md) (F4.4)
> **Shared contract:** [API CONVENTIONS](../api/CONVENTIONS.md)
> **Authored:** 2026-06-17 · **Status:** Draft v1

---

## 1. Scope & how to read this doc

This document is an **exhaustive manual-testing checklist** for E4 (shift master catalog + day-by-day scheduling + schedule views + leader-driven changes). It is organized **per feature → per platform (Web / Mobile) → per POV (super admin · HR/placement admin · shift leader · agent)**.

**Roles under test** (per [CONVENTIONS §17](../api/CONVENTIONS.md#17-rbac-matrix)):
- **super_admin** — global; manages shift master; can schedule/oversee any company.
- **hr_admin** — global oversight; manages shift master; schedules/oversees any company.
- **shift_leader** — on-site supervisor for **exactly one** client company (derived per request from the E3 assignment); builds & edits that company's roster on web + mobile; approves changes.
- **agent** — a `self.*`-baseline employee (no elevation); views own schedule on **mobile**; receives change/reminder notifications. *(v1: agent-initiated swap/day-off requests are deferred — see §F4.4.)*

**Scope notes & spec-fidelity flags (read before executing):**
- **Auto-publish, no draft/approval gate.** Saving any cell is immediately live to the agent + fires a notification (INV-4, SA-6).
- **No coverage minimums, no rotation engine** — pure day-by-day individual assignment.
- **F4.4 agent swap/day-off requests are DEFERRED to post-v1** (FEATURE §7; F4.4 §10, dated 2026-05-29). v1 = **leader-driven edits/clears only**. Cases that exercise the agent request→approve flow are tagged **[POST-V1]**: in a v1 build the expected result is that the request UI is **absent/disabled**; the documented request→approve behavior is the post-v1 acceptance target. Execute the v1 assertion now; keep the post-v1 assertion for when the feature lands.
- **Roster-compliance indicators** (holiday-shift badge, missing-weekly-rest flag, >6-consecutive-workday cap warning) requested for QA coverage are **NOT defined in any E4 PRD** as of 2026-06-17 (E4 has no coverage/rest rules; holidays live in E7, leave in E6). They are captured in **§F4.2 · Roster-compliance indicators** as tests that **must currently FAIL/be N-A** and double as a spec-gap finding — see that section's preamble. Do not pass them against a spec-faithful v1 build.
- **Times** are `Asia/Jakarta` (WIB, UTC+7); local-time fields render `HH:MM` 24h ([CONVENTIONS §10](../api/CONVENTIONS.md#10-timestamps)). Cross-midnight shift = `end_at <= start_at` (SM-2), attributed to its **start date** (SA-8).
- **Error codes** (CONVENTIONS §11): `DOUBLE_SHIFT` 409, `SHIFT_OVER_LEAVE` 409, `OUTSIDE_PLACEMENT_PERIOD` 422, `OUT_OF_SCOPE` 403, `CONFLICT` 409, `RULE_VIOLATION` 422.
- All relative dates in source Gherkin are kept as the spec's absolute dates (e.g. 2026-06-10); "today" anchors to the execution date — preconditions state the needed relationship explicitly.

**Status legend:** `[ ]` not run · `[x]` pass · `[~]` blocked/deferred · `[!]` fail. Tick the checkbox on the first line of each case.

---

## 2. Coverage matrix (features × platform × role)

Legend: ● primary surface under test · ○ secondary/read-only · — out of scope · ✎ v1 leader-only (agent side deferred).

| Feature | Web · super_admin | Web · hr_admin | Web · shift_leader | Web · agent | Mobile · shift_leader | Mobile · agent |
|---|---|---|---|---|---|---|
| **F4.1** Shift Master Catalog | ● CRUD | ● CRUD | ○ read (picker) | — (no console) | ○ read (picker) | ○ read (on schedule) |
| **F4.2** Daily Schedule Assignment | ● any company | ● any company | ● own company | — RBAC denied | ● today's roster | ○ receives assignment |
| **F4.3** Schedule Views | ● any company calendar | ● any company calendar | ● own company calendar | — | ○ today's roster view | ● "My schedule" |
| **F4.4** Schedule Changes (leader edits) | ● any | ● any | ● own company | — RBAC denied | ● own company | ✎ request [POST-V1] |
| **F4.4** Swap/Day-off requests | ○ approve (esc.) | ○ approve (escalation) | ✎ approve [POST-V1] | — | ✎ approve [POST-V1] | ✎ request [POST-V1] |

**Cross-cutting coverage checklist** (every one appears as ≥1 case below):
- Shift master CRUD: hours/breaks validation, cross-midnight detect, unique title, deactivate-not-delete.
- Roster builder happy paths (single cell, bulk date-range, mark OFF).
- Publish/notify flow — agent mobile receives the notification + sees the shift.
- Schedule changes (edit / clear) re-publish + notify.
- Swap/day-off request→approve→reject→withdraw [POST-V1].
- Schedule views per role (day/week/by-agent; agent self-list; cross-midnight display).
- RBAC denials (agent cannot edit roster; leader out-of-company scope).
- Empty / loading / error states (CONVENTIONS error envelope + design empty patterns).

---

# F4.1 — Work-Shift Master Catalog

> BR: SM-1, SM-2, SM-4, SM-5, SM-6 · Cases C-1..C-4. Console-only authoring (super_admin / hr_admin). Leaders & agents only ever read template title/times.

## F4.1 · Web · Super Admin POV

#### TC-E4-F4.1-001 · Create a day shift with a valid break
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P0 · **Type:** Happy
- **Objective:** A standard day template with an in-window break saves as active.
- **Preconditions:** Logged in as super_admin; no template titled "Morning" exists.
- **Steps:**
  1. Open Shift Master Catalog → New template.
  2. Title = "Morning"; start_at = 07:00; end_at = 15:00.
  3. Break: start_break = 12:00, end_break = 13:00.
  4. Save.
- **Expected result / AC:** Template persists with `status = ACTIVE`, `spans_midnight = false`, ID `SWP-SHF-…`; 201 + `Location`; appears in catalog list; audit entry written.
- **Traceability:** F4.1, SM-1, SM-6.

#### TC-E4-F4.1-002 · Create a template with no break (break optional)
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Break fields are optional.
- **Preconditions:** Logged in as super_admin.
- **Steps:** Create "Day Open" 08:00–17:00 leaving both break fields empty; Save.
- **Expected result / AC:** Saves active with `start_break`/`end_break` null; no break validation fires.
- **Traceability:** F4.1, SM-1.

#### TC-E4-F4.1-003 · Create a cross-midnight night shift (spans_midnight auto-set)
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P0 · **Type:** Edge
- **Objective:** When `end_at <= start_at`, the system auto-sets `spans_midnight` and computes duration across the boundary.
- **Preconditions:** Logged in as super_admin.
- **Steps:** Create "Night" start_at = 23:00, end_at = 07:00; Save.
- **Expected result / AC:** `spans_midnight = true`; computed duration = 8h (across the day boundary); UI labels it as crossing midnight.
- **Traceability:** F4.1, SM-2, Gherkin "cross-midnight night shift".

#### TC-E4-F4.1-004 · Reject a break outside the shift window
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P0 · **Type:** Negative
- **Objective:** A break not fully inside the working window is blocked.
- **Preconditions:** Logged in as super_admin.
- **Steps:** Create "Morning2" 07:00–15:00 with break 16:00–17:00; Save.
- **Expected result / AC:** Blocked with field-level error on break ("break is outside the working window"); 400/422 with `error.fields` populated; nothing persisted.
- **Traceability:** F4.1, SM-1, Gherkin "Reject a break outside the shift window".

#### TC-E4-F4.1-005 · Reject break where end_break <= start_break / partially outside
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Inverted or boundary-straddling break is rejected.
- **Preconditions:** super_admin; creating "Day" 07:00–15:00.
- **Steps:** (a) break 13:00–12:00; (b) break 14:30–15:30 (end past shift end). Save each.
- **Expected result / AC:** Both blocked with field error; break must be a forward interval fully within [start_at, end_at].
- **Traceability:** F4.1, SM-1.

#### TC-E4-F4.1-006 · Unique title enforced
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Title must be unique within the catalog.
- **Preconditions:** "Morning" already exists (TC-001).
- **Steps:** Create another template titled "Morning"; Save.
- **Expected result / AC:** Blocked with uniqueness error (409 `CONFLICT` or 422 with field error); no second row created. Verify case/trailing-space variants per server normalization.
- **Traceability:** F4.1, SM-4, Gherkin "Unique title".

#### TC-E4-F4.1-007 · Missing required field (title / start / end)
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P1 · **Type:** Negative
- **Objective:** title, start_at, end_at are required.
- **Preconditions:** super_admin.
- **Steps:** Attempt save with (a) empty title, (b) empty start_at, (c) empty end_at.
- **Expected result / AC:** Each blocked client-side and server-side returns 400 `INVALID_REQUEST` with the offending field in `error.fields`.
- **Traceability:** F4.1, SM-1, CONVENTIONS §12.

#### TC-E4-F4.1-008 · Edit template times — propagation to unrealized schedules (forward link to F4.2 SA-10)
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P0 · **Type:** Edge
- **Objective:** Editing a master's start/end updates matching future schedule entries (verified in detail under F4.2 SA-10/INV-5 cases).
- **Preconditions:** "Morning" referenced by ≥1 schedule entry with `work_date >= today`, status `Scheduled`, agent not yet checked in.
- **Steps:** Edit "Morning" end_at 15:00 → 16:00; Save; open that future schedule entry / agent view.
- **Expected result / AC:** Schedule entry's displayed end time follows the new master (16:00) for not-yet-realized entries; OFF entries and past entries unaffected; audit written. (Full realization matrix: TC-E4-F4.2-040..043.)
- **Traceability:** F4.1, F4.2 SA-10, INV-5.

#### TC-E4-F4.1-009 · Deactivate a template (not delete)
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Templates are deactivated, not hard-deleted.
- **Preconditions:** "Night" exists, status ACTIVE.
- **Steps:** Open "Night" → Deactivate; confirm.
- **Expected result / AC:** `status = INACTIVE`; row remains; removed from the active-only scheduling picker (verify in F4.2); audit entry written.
- **Traceability:** F4.1, SM-5, C-3.

#### TC-E4-F4.1-010 · Cannot hard-delete a referenced template
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P0 · **Type:** Negative
- **Objective:** A template referenced by any schedule cannot be deleted; only deactivated.
- **Preconditions:** "Morning" referenced by ≥1 schedule entry.
- **Steps:** Attempt Delete on "Morning".
- **Expected result / AC:** Deletion blocked (409 `CONFLICT`); UI offers Deactivate instead; row intact.
- **Traceability:** F4.1, SM-5, Gherkin "Cannot delete a referenced template".

#### TC-E4-F4.1-011 · Deactivated template keeps existing future schedules
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Deactivating disables new selection but existing future schedules retain the template (or are flagged).
- **Preconditions:** "Night" used by a future schedule entry.
- **Steps:** Deactivate "Night"; open the future schedule entry.
- **Expected result / AC:** Existing entries still show "Night" with its times (or display a "template deactivated" flag); the picker no longer offers "Night" for new cells.
- **Traceability:** F4.1, SM-5, C-3.

#### TC-E4-F4.1-012 · 24-hour shift (start_at == end_at) — confirm allowed or blocked
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Open decision (PRD §10): a 24h shift where start==end.
- **Preconditions:** super_admin.
- **Steps:** Create "Full Day" 08:00–08:00; Save.
- **Expected result / AC:** Behaves per the build's resolution of the open item — EITHER blocked with a clear message OR saved as a 24h full-day shift. Record actual behavior; flag if undefined. (Spec-open as of 2026-06-17.)
- **Traceability:** F4.1, C-1, PRD §10 open item.

#### TC-E4-F4.1-013 · Multiple breaks not supported (single break window only)
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P2 · **Type:** Edge
- **Objective:** v1 supports exactly one break window.
- **Preconditions:** super_admin.
- **Steps:** Inspect the template form for any "add another break" affordance.
- **Expected result / AC:** Only one break window can be entered; no add-break control. (If a second break is somehow accepted, flag as spec violation.)
- **Traceability:** F4.1, C-2.

#### TC-E4-F4.1-014 · Catalog empty state
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Empty catalog renders a designed empty state, not a blank table.
- **Preconditions:** Fresh environment / no templates.
- **Steps:** Open Shift Master Catalog.
- **Expected result / AC:** Empty-state pattern with a "New template" CTA; no dead flow.
- **Traceability:** F4.1, DESIGN-SYSTEM no-dead-flow rule.

#### TC-E4-F4.1-015 · Catalog loading & server-error states
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Loading skeleton on fetch; error pattern on 5xx.
- **Preconditions:** Throttle/network-fail the catalog list call.
- **Steps:** 1. Slow network → open catalog (observe skeleton). 2. Force 500 → reopen.
- **Expected result / AC:** Skeleton/loading while pending; on 500 an error state with retry; error envelope `code: INTERNAL` surfaced as friendly copy.
- **Traceability:** F4.1, CONVENTIONS §11.

## F4.1 · Web · HR/placement Admin POV

#### TC-E4-F4.1-016 · HR admin can CRUD shift master (same as super_admin)
- [ ] **Platform:** Web · **POV:** hr_admin · **Priority:** P0 · **Type:** RBAC/Happy
- **Objective:** hr_admin has full master-data authoring rights.
- **Preconditions:** Logged in as hr_admin.
- **Steps:** Create, edit, deactivate a template (repeat TC-001/008/009 abbreviated).
- **Expected result / AC:** All succeed; audit attributes actor = hr_admin. (Re-run the validation negatives TC-004/006 to confirm same server enforcement.)
- **Traceability:** F4.1, CONVENTIONS §17 (hr_admin master data).

## F4.1 · Web · Shift Leader POV

#### TC-E4-F4.1-017 · Shift leader cannot author shift master (RBAC denial)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Master-catalog authoring is gated to super_admin/hr_admin; leaders read-only.
- **Preconditions:** Logged in as shift_leader (active E3 assignment to one company).
- **Steps:** 1. Attempt to navigate to Shift Master Catalog authoring. 2. Direct-call `POST`/`PATCH` on a template via API.
- **Expected result / AC:** No create/edit UI exposed (defense-in-depth); API returns 403 `FORBIDDEN`; no template mutated.
- **Traceability:** F4.1, CONVENTIONS §17, ENGINEERING client-RBAC-is-not-the-gate.

## F4.1 · Mobile · Shift Leader / Agent POV (read-only)

#### TC-E4-F4.1-018 · Shift/agent see template title+times on schedule (read-only)
- [ ] **Platform:** Mobile · **POV:** shift_leader / agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Mobile surfaces the template's title/start/end (no catalog CRUD).
- **Preconditions:** Agent scheduled with "Morning" 07:00–15:00 break 12:00–13:00.
- **Steps:** Open the schedule entry on mobile (agent's My Schedule; leader's roster cell).
- **Expected result / AC:** Shows title "Morning", times 07:00–15:00, break shown per SV-3; no edit/delete controls on the template itself.
- **Traceability:** F4.1 §4 (mobile read-only), F4.3 SV-3.

---

# F4.2 — Daily Schedule Assignment

> BR: SA-1, SA-2, SA-3, SA-5, SA-6, SA-7(OFF), SA-8, SA-9, SA-10 · INV-1..5 · Cases C-1..C-8. Leader builds own-company roster (web + mobile); HR/super_admin any company; agent receives auto-published assignment.

## F4.2 · Web · Shift Leader POV (own company)

#### TC-E4-F4.2-001 · Assign a shift and auto-publish (happy path)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Assigning a shift creates a Scheduled entry, links the active placement, and auto-publishes + notifies.
- **Preconditions:** Leader of "Plaza Senayan"; "Budi" has an active placement there covering 2026-06-10.
- **Steps:**
  1. Open the company schedule grid.
  2. Select Budi's cell for 2026-06-10.
  3. Choose "Parking Night" from the picker; Save.
- **Expected result / AC:** Entry `SWP-SCH-…` created with `status = Scheduled`, `placement_id` = Budi's active placement, `shift_master_id` = Parking Night; 201; cell renders the shift immediately (no publish step); Budi notified on mobile (verify TC-E4-F4.2-030). Audit written.
- **Traceability:** F4.2, SA-1, SA-6, INV-2, INV-4, Gherkin "Assign a shift and auto-publish".

#### TC-E4-F4.2-002 · Shift picker shows only ACTIVE master templates
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Picker offers all active templates and excludes deactivated ones.
- **Preconditions:** "Night" deactivated (F4.1 TC-009); "Morning"/"Parking Night" active.
- **Steps:** Open the shift picker for Budi's cell.
- **Expected result / AC:** Active templates listed; "Night" absent. Global catalog — same list regardless of company.
- **Traceability:** F4.2, FEATURE §6b (global catalog), SM-5, Gherkin "Shift picker shows all active shifts".

#### TC-E4-F4.2-003 · Replace an existing same-day shift (warn + replace)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** Edge
- **Objective:** One shift per agent per day (INV-1); assigning a second warns and replaces.
- **Preconditions:** Budi already has "Morning" on 2026-06-10.
- **Steps:** Assign Budi "Night" on 2026-06-10; confirm the warning.
- **Expected result / AC:** A warning is shown; on confirm the entry is replaced with "Night" (still one row for `(employee, date)`); Budi notified of the change. Server upholds unique `(employee_id, work_date)`.
- **Traceability:** F4.2, SA-2, INV-1, Gherkin "Replacing an existing shift".

#### TC-E4-F4.2-004 · Block scheduling an agent not placed that day
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** Negative
- **Objective:** Cannot schedule an agent with no active placement at the company on that date.
- **Preconditions:** "Andi" has no active placement at Plaza Senayan on 2026-06-10.
- **Steps:** Try to assign Andi a shift on 2026-06-10.
- **Expected result / AC:** Blocked with "Agent is not placed here on this date"; 422 `OUTSIDE_PLACEMENT_PERIOD` (or agent not even listed in the grid). No entry created.
- **Traceability:** F4.2, SA-1, INV-2, Gherkin "Block scheduling an agent not placed that day".

#### TC-E4-F4.2-005 · Cannot schedule after placement end
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** Negative
- **Objective:** No scheduling beyond placement end date.
- **Preconditions:** Budi's placement ends 2026-06-30.
- **Steps:** Try to schedule Budi on 2026-07-05.
- **Expected result / AC:** Blocked; 422 `OUTSIDE_PLACEMENT_PERIOD`; no entry.
- **Traceability:** F4.2, SA-5, Gherkin "Cannot schedule beyond placement end".

#### TC-E4-F4.2-006 · Cannot schedule before placement start (future/Scheduled placement)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Negative
- **Objective:** A placement with a future start cannot be scheduled before its start date.
- **Preconditions:** "Eka" has a placement at Plaza Senayan with start 2026-06-20 (status Scheduled/future).
- **Steps:** Try to schedule Eka on 2026-06-15.
- **Expected result / AC:** Blocked; 422 `OUTSIDE_PLACEMENT_PERIOD`; no entry.
- **Traceability:** F4.2, SA-5, C-3.

#### TC-E4-F4.2-007 · Leader scope enforced — cannot schedule another company's agent
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** A leader may only schedule agents at their own company.
- **Preconditions:** Leader of Plaza Senayan; "Citra" placed at a different company.
- **Steps:** 1. Confirm Citra is not in the grid. 2. Direct API `POST` a schedule for Citra.
- **Expected result / AC:** Citra not selectable in UI; API returns 403 `OUT_OF_SCOPE`; no entry created.
- **Traceability:** F4.2, SA-3, INV-3, Gherkin "Leader scope is enforced", CONVENTIONS §17 (derived company scope).

#### TC-E4-F4.2-008 · Mark a day OFF (distinct from no entry)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** Happy
- **Objective:** A day can be explicitly OFF, which is different from an empty cell.
- **Preconditions:** Budi placed; 2026-06-12 currently empty.
- **Steps:** Mark 2026-06-12 OFF for Budi; Save.
- **Expected result / AC:** Entry created with `status = Off`; cell shows OFF marker; agent's view shows OFF for that day; notified; audit written. An empty cell remains visibly "no entry".
- **Traceability:** F4.2, SA-7, Gherkin "Mark a day off".

#### TC-E4-F4.2-009 · Clear an OFF back to empty
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P2 · **Type:** Edge
- **Objective:** Removing an OFF returns the cell to "no entry" state.
- **Preconditions:** Budi has OFF on 2026-06-12 (TC-008).
- **Steps:** Clear the OFF cell.
- **Expected result / AC:** Entry removed; cell reverts to empty (distinct from OFF); agent notified per change rules. (See F4.4 clear semantics.)
- **Traceability:** F4.2, SA-7, F4.4 CH-1.

#### TC-E4-F4.2-010 · Bulk apply a shift across a date range (per-day validation)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Bulk "apply to date range" creates one validated entry per day.
- **Preconditions:** Budi placed 2026-06-01..2026-06-30; "Morning" active.
- **Steps:** Bulk-assign "Morning" to Budi for 2026-06-10..2026-06-16; Save.
- **Expected result / AC:** 7 Scheduled entries created (one per day); each validated for active placement + one/day; single notification or per-day per design; audit per entry.
- **Traceability:** F4.2, C-1, Gherkin/§10 bulk helper.

#### TC-E4-F4.2-011 · Bulk range partially blocked (some days outside placement / on leave)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Edge
- **Objective:** Bulk apply validates each day; valid days commit, invalid days are reported.
- **Preconditions:** Budi placed until 2026-06-30; range includes 2026-06-28..2026-07-02 (last 2 days past placement end). Bulk endpoint uses `:bulk-*` partial-success shape.
- **Steps:** Bulk-assign "Morning" 2026-06-28..2026-07-02.
- **Expected result / AC:** 2026-06-28/29/30 succeed; 2026-07-01/02 fail with `OUTSIDE_PLACEMENT_PERIOD`; response has `succeeded` + `failed` arrays (CONVENTIONS §14); UI shows partial-failure summary. No all-or-nothing rollback of valid days.
- **Traceability:** F4.2, C-1, SA-5, CONVENTIONS §14.

#### TC-E4-F4.2-012 · Scheduling over approved leave is BLOCKED
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** Negative
- **Objective:** A day protected by approved leave (E6) cannot be scheduled.
- **Preconditions:** Budi has approved leave covering 2026-06-14.
- **Steps:** Try to assign Budi a shift on 2026-06-14.
- **Expected result / AC:** Blocked; 409 `SHIFT_OVER_LEAVE`; message explains the day is protected by approved leave; no entry.
- **Traceability:** F4.2, C-2, decision 2026-05-29 (blocked), CONVENTIONS §11 (`SHIFT_OVER_LEAVE`).

#### TC-E4-F4.2-013 · Bulk range that overlaps approved leave skips the leave day
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Edge
- **Objective:** A leave day inside a bulk range fails just that day.
- **Preconditions:** Budi has approved leave on 2026-06-14; bulk range 2026-06-13..2026-06-15.
- **Steps:** Bulk-assign "Morning" 2026-06-13..2026-06-15.
- **Expected result / AC:** 06-13 & 06-15 succeed; 06-14 in `failed` with `SHIFT_OVER_LEAVE`.
- **Traceability:** F4.2, C-1, C-2.

#### TC-E4-F4.2-014 · Cross-midnight shift attributed to start date
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Edge
- **Objective:** A 23:00–07:00 shift is recorded on its start date and renders across two days.
- **Preconditions:** "Night" (23:00–07:00, spans_midnight) active; Budi placed.
- **Steps:** Assign Budi "Night" on 2026-06-10.
- **Expected result / AC:** Entry `work_date = 2026-06-10`; grid/agent view shows it spanning 06-10→06-11; counts as the 06-10 assignment for one/day.
- **Traceability:** F4.2, SA-8, FEATURE §6b, F4.3 SV-6.

#### TC-E4-F4.2-015 · Cross-midnight shift on the last placement day allowed
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P2 · **Type:** Edge
- **Objective:** Night shift starting on the placement's final day is allowed; overnight portion handled by E5.
- **Preconditions:** Budi's placement ends 2026-06-30; "Night" active.
- **Steps:** Assign Budi "Night" on 2026-06-30.
- **Expected result / AC:** Allowed (start date within placement); entry created; no `OUTSIDE_PLACEMENT_PERIOD` despite the 07:00 portion falling on 07-01.
- **Traceability:** F4.2, SA-8, C-4.

#### TC-E4-F4.2-016 · Concurrent edits to the same cell — last write wins
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Edge
- **Objective:** Two writers on the same `(employee, date)` resolve to last-write-wins on the unique key; both audited.
- **Preconditions:** Two sessions (leader + HR) editing Budi's 2026-06-10 cell.
- **Steps:** Session A sets "Morning", Session B sets "Night" near-simultaneously; both Save.
- **Expected result / AC:** Final entry = the last committed value; exactly one row for the key; two audit entries recorded; loser may see a stale-then-refresh; no duplicate-row 500.
- **Traceability:** F4.2, C-6, INV-1.

#### TC-E4-F4.2-017 · Empty roster grid (no placed agents)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Leader of a company with no placed agents sees an empty state, not a broken grid.
- **Preconditions:** Leader of a company with zero active placements.
- **Steps:** Open the schedule grid.
- **Expected result / AC:** Empty-state with a prompt to place agents (E3); no editable rows.
- **Traceability:** F4.2, F4.3 C-3.

#### TC-E4-F4.2-018 · Save error surfaces error envelope (network/500)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** A failed save shows an error without losing the grid state.
- **Preconditions:** Force a 500 / network drop on the assign call.
- **Steps:** Assign a shift while the API fails.
- **Expected result / AC:** Inline error/toast from the envelope `message`; cell reverts to prior state (optimistic rollback); retry available; no false "published" state.
- **Traceability:** F4.2, CONVENTIONS §11, no-dead-flow.

## F4.2 · Web · HR/placement Admin POV (any company)

#### TC-E4-F4.2-019 · HR admin schedules any company
- [ ] **Platform:** Web · **POV:** hr_admin · **Priority:** P0 · **Type:** RBAC/Happy
- **Objective:** HR/super_admin scope = global; can schedule across companies.
- **Preconditions:** hr_admin; agents placed at companies X and Y.
- **Steps:** Open company X grid, assign a shift; switch to company Y, assign a shift.
- **Expected result / AC:** Both succeed; no scope block; entries link the correct placement_id per company; agents notified.
- **Traceability:** F4.2, SA-3, INV-3, Gherkin (HR any company).

#### TC-E4-F4.2-020 · HR admin schedules a company with no shift leader
- [ ] **Platform:** Web · **POV:** hr_admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Scheduling is allowed even when the company has no assigned leader.
- **Preconditions:** Company Z has placed agents but no shift_leader assignment.
- **Steps:** HR assigns a shift to a Z agent.
- **Expected result / AC:** Allowed (HR scope); agent still notified; entry created.
- **Traceability:** F4.2, C-5.

#### TC-E4-F4.2-021 · super_admin parity with hr_admin for scheduling
- [ ] **Platform:** Web · **POV:** super_admin · **Priority:** P1 · **Type:** RBAC
- **Objective:** super_admin can schedule any company identically.
- **Preconditions:** super_admin.
- **Steps:** Repeat TC-019 abbreviated as super_admin.
- **Expected result / AC:** Succeeds globally; audit actor = super_admin.
- **Traceability:** F4.2, SA-3.

## F4.2 · Web · Agent POV (RBAC denial — no console)

#### TC-E4-F4.2-022 · Agent cannot access the roster builder / edit any cell
- [ ] **Platform:** Web · **POV:** agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** A baseline (self.*) employee cannot edit rosters; web console is not an agent surface.
- **Preconditions:** Agent (no elevation) attempting to reach web scheduling.
- **Steps:** 1. Attempt to open the schedule grid URL on web. 2. Direct-call `POST /schedules` for self and for another agent.
- **Expected result / AC:** No builder UI for the agent (route guarded / `comp/EmptyNoPermission`); both API writes return 403 `FORBIDDEN`; no entry created. Client RBAC is defense-in-depth, server is the gate.
- **Traceability:** F4.2, CONVENTIONS §17 (self.* baseline has no roster write), ENGINEERING RBAC.

## F4.2 · Mobile · Shift Leader POV (quick assign today's roster)

#### TC-E4-F4.2-023 · Leader quick-assigns today's roster on mobile
- [ ] **Platform:** Mobile · **POV:** shift_leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Leader can assign/adjust today's shifts from the mobile roster (comp/SLMobileNav surface).
- **Preconditions:** Leader of Plaza Senayan on mobile; Budi placed today.
- **Steps:** Open today's roster → Budi → pick "Morning" → Save.
- **Expected result / AC:** Entry created Scheduled for today; same validations as web (placement/one-day/scope); Budi notified; cell updates live.
- **Traceability:** F4.2 §4 (mobile leader), SA-1, SA-6.

#### TC-E4-F4.2-024 · Leader mobile respects company scope
- [ ] **Platform:** Mobile · **POV:** shift_leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** Mobile roster only shows/edits the leader's own company agents.
- **Preconditions:** Leader of Plaza Senayan.
- **Steps:** Confirm only Plaza Senayan agents appear; attempt no cross-company action.
- **Expected result / AC:** Only own-company agents listed/editable; server rejects any out-of-scope write with 403 `OUT_OF_SCOPE`.
- **Traceability:** F4.2, SA-3, INV-3.

#### TC-E4-F4.2-025 · Leader mobile mark OFF
- [ ] **Platform:** Mobile · **POV:** shift_leader · **Priority:** P2 · **Type:** Happy
- **Objective:** OFF can be set from mobile.
- **Preconditions:** Leader; Budi placed today.
- **Steps:** Mark Budi OFF for today on mobile.
- **Expected result / AC:** `status = Off`; agent notified; cell shows OFF.
- **Traceability:** F4.2, SA-7.

## F4.2 · Mobile · Agent POV (receives assignment)

#### TC-E4-F4.2-030 · Agent receives the auto-publish notification + sees the shift
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify the publish/notify flow end-to-end: a leader save reaches the agent.
- **Preconditions:** Budi logged into mobile with notifications enabled; leader assigns Budi a shift (TC-001) while Budi is online.
- **Steps:** 1. Leader assigns "Parking Night" 2026-06-10. 2. Observe Budi's device.
- **Expected result / AC:** Budi receives a push notification ("new/updated shift"); opening My Schedule shows the new shift with title/times/site without a manual refresh (SV-4); no edit affordances for Budi.
- **Traceability:** F4.2, SA-6, INV-4, CONVENTIONS §16.2 (Schedule published → affected agents), F4.3 SV-4.

#### TC-E4-F4.2-031 · Agent notified on replace/change
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Happy
- **Objective:** A replaced/changed shift re-notifies the agent.
- **Preconditions:** Budi had "Morning"; leader replaces with "Night" (TC-003).
- **Steps:** Observe Budi's device after the replace.
- **Expected result / AC:** New change notification; My Schedule reflects "Night".
- **Traceability:** F4.2, SA-2, INV-4, F4.4 CH-2.

## F4.2 · Shift-master propagation (INV-5 / SA-10 / C-7 / C-8) — realization matrix

> Cross-references F4.1 TC-008. These verify the "track master until realized by attendance" rule. Requires E5 attendance hooks to be present; if E5 not yet wired, mark [~] and record.

#### TC-E4-F4.2-040 · Master edit propagates to a not-yet-checked-in future entry
- [ ] **Platform:** Web · **POV:** hr_admin · **Priority:** P0 · **Type:** Edge
- **Objective:** Editing master start/end updates entries with `work_date >= today`, status != Off, not leave-cancelled, where the agent has not checked in.
- **Preconditions:** Budi scheduled "Morning" 07:00–15:00 on a future date; no check-in.
- **Steps:** Edit "Morning" → 08:00–16:00; reopen the entry / agent view.
- **Expected result / AC:** Entry's effective start=08:00, end=16:00 (follows master live); agent's My Schedule reflects new times; OFF/past entries unaffected; break NOT stored/propagated to the entry.
- **Traceability:** F4.2, SA-10, INV-5.

#### TC-E4-F4.2-041 · Past-date and OFF entries are NOT propagated
- [ ] **Platform:** Web · **POV:** hr_admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Propagation scope excludes `work_date < today` and `status = Off`.
- **Preconditions:** "Morning" used on a past date and as an OFF cell.
- **Steps:** Edit "Morning" times; inspect the past entry and the OFF entry.
- **Expected result / AC:** Past entry keeps its historical times; OFF entry unchanged (no times); only future Scheduled entries move.
- **Traceability:** F4.2, SA-10, INV-5.

#### TC-E4-F4.2-042 · Master end edited after check-in, before check-out (C-7)
- [ ] **Platform:** Web · **POV:** hr_admin · **Priority:** P0 · **Type:** Edge
- **Objective:** start_time frozen at check-in; end_time/cross_midnight still tracks master; open attendance shift-end window updates.
- **Preconditions:** Budi checked in (E5) on "Morning" but not checked out.
- **Steps:** Edit "Morning" end_at 15:00 → 16:00.
- **Expected result / AC:** Entry start_time frozen (= actual check-in handling); end_time updates to 16:00; the open attendance record's shift-end window = 16:00 so lateness/early/auto-close use the new end.
- **Traceability:** F4.2, SA-10, INV-5, C-7.

#### TC-E4-F4.2-043 · Master edited after check-out — fully frozen (C-8)
- [ ] **Platform:** Web · **POV:** hr_admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Once checked out, both start_time and end_time are frozen; master edits don't touch the entry.
- **Preconditions:** Budi checked out on "Morning".
- **Steps:** Edit "Morning" times.
- **Expected result / AC:** The realized entry's stored start/end unchanged; attendance evaluation unaffected.
- **Traceability:** F4.2, SA-10, INV-5, C-8.

## F4.2 · Roster-compliance indicators (REQUESTED COVERAGE — SPEC GAP)

> **Spec-fidelity warning (2026-06-17):** The holiday-shift badge, missing-weekly-rest flag, and >6-consecutive-workday cap warning are **not defined anywhere in E4** (no coverage/rest/holiday rules exist in F4.1–F4.4; E4 explicitly has *no coverage minimums*, holidays live in E7, leave in E6). These cases are recorded so QA can verify them **if/when the indicators are specced**, and to flag the gap. Against a spec-faithful v1 build the **expected result is that no such indicator appears** — execute the "v1 expected" assertion; the "target" assertion is the future acceptance criterion pending a spec decision.

#### TC-E4-F4.2-050 · Holiday-shift badge on a public-holiday assignment
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P2 · **Type:** Edge (spec gap)
- **Objective:** A shift assigned on an E7 public holiday is visually badged.
- **Preconditions:** 2026-06-17 (example) flagged as a public holiday (E7 `SWP-HOL`); Budi scheduled that day.
- **Steps:** View the grid cell / agent entry for the holiday date.
- **Expected result / AC:** **v1 expected:** no holiday badge (not specced in E4) — record as N-A. **Target (post-spec):** cell shows a holiday badge sourced from E7 holidays.
- **Traceability:** F4.2 (requested QA coverage); E7 holidays — **no E4 BR exists**; FLAG as spec gap.

#### TC-E4-F4.2-051 · Missing-weekly-rest flag
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P2 · **Type:** Edge (spec gap)
- **Objective:** Flag when an agent has 7 consecutive scheduled days with no rest day in the week.
- **Preconditions:** Budi scheduled all 7 days of a calendar week with no OFF.
- **Steps:** View the week calendar.
- **Expected result / AC:** **v1 expected:** no rest-flag (not specced) — N-A. **Target:** a "no weekly rest" warning indicator on that agent's week. (Indonesian labor law context, but no E4 rule encodes it.)
- **Traceability:** F4.2 (requested QA coverage); **no E4 BR exists**; FLAG as spec gap.

#### TC-E4-F4.2-052 · >6-consecutive-workday cap warning
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P2 · **Type:** Edge (spec gap)
- **Objective:** Warn when assigning a shift would create a 7th consecutive workday.
- **Preconditions:** Budi scheduled 6 consecutive days; assigning a 7th.
- **Steps:** Assign Budi a shift on the 7th consecutive day.
- **Expected result / AC:** **v1 expected:** assignment succeeds with no warning (no cap rule in E4) — N-A. **Target:** a non-blocking warning ("exceeds 6 consecutive workdays"); confirm whether it blocks or only warns once specced.
- **Traceability:** F4.2 (requested QA coverage); **no E4 BR exists**; FLAG as spec gap.

---

# F4.3 — Schedule Calendar & Agent View

> BR: SV-1..SV-6 · Cases C-1..C-4. Read surfaces: leader/HR company calendar (web), agent My Schedule (mobile). Live via auto-publish; no editing here.

## F4.3 · Web · Shift Leader POV (own company calendar)

#### TC-E4-F4.3-001 · Leader weekly company calendar
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Week view shows each placed agent's shift per day for the company.
- **Preconditions:** Leader of Plaza Senayan; several agents scheduled across the week of 2026-06-08..2026-06-14.
- **Steps:** Open the schedule → Week view for that week.
- **Expected result / AC:** A matrix of agents × days showing each shift title/time, OFF markers, and empty cells distinctly; only Plaza Senayan agents appear.
- **Traceability:** F4.3, SV-2, SV-1, Gherkin "Leader views the weekly company calendar".

#### TC-E4-F4.3-002 · Day view and by-agent matrix
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Day, week, and by-agent views all render and switch correctly.
- **Preconditions:** As TC-001.
- **Steps:** Toggle Day → Week → By-agent.
- **Expected result / AC:** Each view renders the same underlying data scoped to the company; by-agent groups rows per agent; times in Asia/Jakarta.
- **Traceability:** F4.3, SV-2, SV-6.

#### TC-E4-F4.3-003 · Entry detail shows title, times, break, OFF, site + geo
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Each entry surfaces the full required detail.
- **Preconditions:** An entry with "Morning" 07:00–15:00 break 12:00–13:00 at a site with geo.
- **Steps:** Open/hover an entry.
- **Expected result / AC:** Shows shift title, start/end, break, OFF marker (where OFF), and the client company site + location/geo.
- **Traceability:** F4.3, SV-3.

#### TC-E4-F4.3-004 · Cross-midnight display spans two days
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Edge
- **Objective:** A 23:00–07:00 shift on 2026-06-10 displays spanning into 06-11.
- **Preconditions:** Budi "Night" 23:00–07:00 on 2026-06-10.
- **Steps:** View the week containing 06-10/06-11.
- **Expected result / AC:** Entry visually spans 06-10→06-11 (attributed to 06-10); times in WIB.
- **Traceability:** F4.3, SV-6, SA-8, Gherkin "Cross-midnight display".

#### TC-E4-F4.3-005 · Live update without manual refresh
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Edge
- **Objective:** Auto-published changes appear in the calendar without a refresh step (live/near-live via short-poll).
- **Preconditions:** Leader viewing the week; a second admin changes an entry (CONVENTIONS §14: no WebSocket; short-poll).
- **Steps:** Have the other session change a cell; wait the poll interval.
- **Expected result / AC:** The calendar reflects the change within the poll window without a manual reload.
- **Traceability:** F4.3, SV-4, CONVENTIONS §1 (short-poll).

#### TC-E4-F4.3-006 · Leader cannot see another company's calendar (scope)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** SV-1 scope — leader sees only their company.
- **Preconditions:** Leader of Plaza Senayan.
- **Steps:** 1. Confirm no company switcher to other companies. 2. Direct API `GET /schedules?company_id=<other>`.
- **Expected result / AC:** UI offers only own company; API returns 403 `OUT_OF_SCOPE` (or 404 to avoid leaking) for other companies.
- **Traceability:** F4.3, SV-1, INV-3, CONVENTIONS §7.

#### TC-E4-F4.3-007 · Empty company calendar (no placed agents)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** A company with no placed agents shows an empty state + place-agents prompt.
- **Preconditions:** Leader of a company with zero placements.
- **Steps:** Open the calendar.
- **Expected result / AC:** Empty-state with a prompt to place agents (E3); no broken matrix.
- **Traceability:** F4.3, C-3.

#### TC-E4-F4.3-008 · Large company calendar virtualizes/paginates by agent
- [ ] **Platform:** Web · **POV:** hr_admin · **Priority:** P2 · **Type:** Edge
- **Objective:** A big roster stays performant; week scoped by default; cursor pagination on the schedule list.
- **Preconditions:** Company with 200+ placed agents.
- **Steps:** Open the week view; scroll/page through agents.
- **Expected result / AC:** Rows virtualized/paginated; default scope = week; schedule list uses cursor pagination (CONVENTIONS §8, schedule is a >100k table), not offset.
- **Traceability:** F4.3, C-4, CONVENTIONS §8.

#### TC-E4-F4.3-009 · Calendar loading & error states
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Skeleton during fetch; error pattern on failure with retry.
- **Preconditions:** Slow/failing schedule query.
- **Steps:** Open calendar under throttle, then force 500.
- **Expected result / AC:** Loading skeleton then error pattern; envelope message surfaced; retry available.
- **Traceability:** F4.3, CONVENTIONS §11.

## F4.3 · Web · HR/Super Admin POV (any company)

#### TC-E4-F4.3-010 · HR/super_admin can view any company calendar
- [ ] **Platform:** Web · **POV:** hr_admin / super_admin · **Priority:** P0 · **Type:** RBAC/Happy
- **Objective:** Global scope lets HR/super_admin open any company's calendar.
- **Preconditions:** hr_admin; multiple companies with schedules.
- **Steps:** Switch between companies and view week/day/by-agent.
- **Expected result / AC:** All companies viewable; no scope block; data correct per company.
- **Traceability:** F4.3, SV-1.

## F4.3 · Mobile · Agent POV ("My schedule")

#### TC-E4-F4.3-020 · Agent views own upcoming shifts with times + site location
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P0 · **Type:** Happy
- **Objective:** My Schedule shows a forward-looking list with times and site, today highlighted.
- **Preconditions:** Budi has upcoming shifts incl. one today and future days.
- **Steps:** Open My Schedule.
- **Expected result / AC:** Forward-looking list of upcoming shifts with title, start/end, site (client company) + location/map; today's shift highlighted; OFF days marked.
- **Traceability:** F4.3, SV-2, SV-3, Gherkin "Agent views own upcoming shifts".

#### TC-E4-F4.3-021 · Agent sees ONLY their own shifts (scope)
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** SV-1 self scope — an agent never sees other agents' schedules.
- **Preconditions:** Budi and Citra both placed/scheduled at Plaza Senayan.
- **Steps:** 1. Budi opens My Schedule. 2. Attempt API `GET /schedules?employee_id=<Citra>`.
- **Expected result / AC:** Only Budi's shifts shown; API call for Citra returns 403/404; no leakage of coworkers.
- **Traceability:** F4.3, SV-1, Gherkin "Agent cannot see other agents' schedules".

#### TC-E4-F4.3-022 · Live update on agent device after a leader change
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Budi's view reflects a leader's change without manual refresh + a change notification.
- **Preconditions:** Budi viewing My Schedule; leader changes his shift.
- **Steps:** Leader edits Budi's shift; observe Budi's screen.
- **Expected result / AC:** View updates to the new shift (live/near-live); change notification received.
- **Traceability:** F4.3, SV-4, Gherkin "Live update after a change", INV-4.

#### TC-E4-F4.3-023 · Shift reminder notification ahead of a shift
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Agent gets a reminder ahead of each shift (default: evening-before + ~1h prior).
- **Preconditions:** Budi has a shift tomorrow 07:00; reminders enabled.
- **Steps:** Wait for the evening-before window and the ~1h-prior window (or trigger the reminder job).
- **Expected result / AC:** Reminder notification(s) delivered at the configured lead time(s); content includes shift time + site. (Lead time is an open default — record actual.)
- **Traceability:** F4.3, SV-5, Gherkin "Shift reminder", FEATURE §7 (reminder = evening-before + ~1h prior).

#### TC-E4-F4.3-024 · Cross-midnight shift displays spanning two days on mobile
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Edge
- **Objective:** Night shift renders across 06-10→06-11 on the agent device.
- **Preconditions:** Budi "Night" 23:00–07:00 on 2026-06-10.
- **Steps:** Open My Schedule covering 06-10.
- **Expected result / AC:** Entry shown spanning two days; times WIB; attributed to 06-10.
- **Traceability:** F4.3, SV-6, SA-8.

#### TC-E4-F4.3-025 · Agent with no upcoming shifts — empty state
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** No upcoming shifts shows an empty state, not a blank list.
- **Preconditions:** Agent placed but unscheduled going forward.
- **Steps:** Open My Schedule.
- **Expected result / AC:** "No shifts scheduled" empty state; no error.
- **Traceability:** F4.3, C-1.

#### TC-E4-F4.3-026 · Site with no geo — address only, map disabled
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Edge
- **Objective:** When the site has no geofence/coords, show address text and disable the map.
- **Preconditions:** Budi placed at a site with no geo configured.
- **Steps:** Open a shift entry's location.
- **Expected result / AC:** Address shown; map control disabled/hidden; no crash.
- **Traceability:** F4.3, C-2.

#### TC-E4-F4.3-027 · Agent My Schedule loading & offline/error
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Loading and offline/error states are designed.
- **Preconditions:** Throttle/disable network.
- **Steps:** Open My Schedule offline, then with a 500.
- **Expected result / AC:** Loading indicator, then offline/error state with retry; cached last-known shifts may show with a stale indicator if designed.
- **Traceability:** F4.3, SV-4, CONVENTIONS §11.

## F4.3 · Mobile · Shift Leader POV (today's roster view)

#### TC-E4-F4.3-028 · Leader views today's roster on mobile
- [ ] **Platform:** Mobile · **POV:** shift_leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Leader's mobile shows today's company roster (read/quick-edit surface).
- **Preconditions:** Leader of Plaza Senayan; agents scheduled today.
- **Steps:** Open the leader mobile roster.
- **Expected result / AC:** Today's agents and shifts listed for the company; scoped to own company; OFF/empty distinct.
- **Traceability:** F4.3, SV-2, FEATURE §6 (mobile leader quick view).

---

# F4.4 — Schedule Changes & Shift Swaps

> BR: CH-1..CH-8 · Cases C-1..C-6. **v1 = leader-driven edits/clears only** (always available). **Agent swap/day-off requests are DEFERRED to post-v1** — those cases are tagged **[POST-V1]**.

## F4.4 · Web · Shift Leader POV (leader edits/clears — v1)

#### TC-E4-F4.4-001 · Leader edits a scheduled shift (status → Changed, notify)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Editing a cell re-runs assignment rules, sets status Changed, auto-publishes, notifies.
- **Preconditions:** Budi has "Morning" on 2026-06-10 at Plaza Senayan.
- **Steps:** Change Budi's 2026-06-10 shift "Morning" → "Night"; Save.
- **Expected result / AC:** Entry updates to "Night", `status = Changed`; Budi notified immediately; audit written; placement/one-day rules re-validated.
- **Traceability:** F4.4, CH-1, CH-2, Gherkin "Leader edits a scheduled shift".

#### TC-E4-F4.4-002 · Leader clears a shift (removed, NOT OFF)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Clearing removes the entry entirely — distinct from marking OFF.
- **Preconditions:** Budi has "Morning" on 2026-06-10.
- **Steps:** Clear Budi's 2026-06-10 shift.
- **Expected result / AC:** Entry removed (cell back to "no entry", not OFF); Budi notified; audit written.
- **Traceability:** F4.4, CH-1, Gherkin "Leader clears a shift".

#### TC-E4-F4.4-003 · Edit re-validates placement active (block if not)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Negative
- **Objective:** An edit that would land outside the placement period is blocked like an assignment.
- **Preconditions:** Budi's placement ended; an existing entry remains pre-end.
- **Steps:** Try to move/extend the entry to a post-placement-end date.
- **Expected result / AC:** Blocked; 422 `OUTSIDE_PLACEMENT_PERIOD`.
- **Traceability:** F4.4, CH-1, SA-5.

#### TC-E4-F4.4-004 · Edit over approved leave blocked
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Negative
- **Objective:** Re-assigning a cell onto an approved-leave day is blocked (same rule as assignment).
- **Preconditions:** Budi approved-leave on 2026-06-14; an entry exists elsewhere.
- **Steps:** Try to move Budi's shift onto 2026-06-14.
- **Expected result / AC:** Blocked; 409 `SHIFT_OVER_LEAVE`.
- **Traceability:** F4.4, CH-1, F4.2 C-2.

#### TC-E4-F4.4-005 · Change to a past date blocked (HR-only correction)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P1 · **Type:** Negative
- **Objective:** Leaders cannot change past-dated schedules (they tie to attendance E5); only HR correction with reason.
- **Preconditions:** Budi had a shift yesterday (past relative to execution date).
- **Steps:** As leader, try to edit/clear the past entry.
- **Expected result / AC:** Blocked/limited for the leader; only HR can correct with a reason (verify HR path TC-016). Server enforces.
- **Traceability:** F4.4, C-5.

#### TC-E4-F4.4-006 · Leader cannot edit another company's cell (scope)
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Edit/clear are scope-gated like assignment.
- **Preconditions:** Leader of Plaza Senayan; a Citra entry at another company.
- **Steps:** Direct API `PATCH`/clear on Citra's entry.
- **Expected result / AC:** 403 `OUT_OF_SCOPE`; no mutation.
- **Traceability:** F4.4, CH-1, SA-3, INV-3.

#### TC-E4-F4.4-007 · Change save error / concurrent change
- [ ] **Platform:** Web · **POV:** shift_leader · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** A failed change shows the error envelope and doesn't leave a phantom "Changed" state.
- **Preconditions:** Force 500 on the change call; or two sessions changing the same cell.
- **Steps:** Edit a cell while API fails; separately, race two edits.
- **Expected result / AC:** Error surfaced; cell reverts on failure; for the race, last-write-wins with both audited (mirrors F4.2 C-6); single row preserved.
- **Traceability:** F4.4, CH-2, F4.2 C-6, CONVENTIONS §11.

## F4.4 · Mobile · Shift Leader POV (leader edits — v1)

#### TC-E4-F4.4-008 · Leader edits/clears today's cell on mobile
- [ ] **Platform:** Mobile · **POV:** shift_leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Leader-driven change works from mobile with the same rules + notify.
- **Preconditions:** Leader of Plaza Senayan; Budi scheduled today.
- **Steps:** Change Budi's shift, then clear another agent's shift, from mobile.
- **Expected result / AC:** Status Changed / entry removed respectively; agents notified; scope + placement + one-day re-validated; audit.
- **Traceability:** F4.4, CH-1, CH-2.

## F4.4 · Web · HR/Super Admin POV (any company + past correction)

#### TC-E4-F4.4-016 · HR corrects a past-dated schedule with a reason
- [ ] **Platform:** Web · **POV:** hr_admin · **Priority:** P1 · **Type:** Edge
- **Objective:** HR (not leader) may correct a past schedule, requiring a reason.
- **Preconditions:** A past-dated entry needing correction.
- **Steps:** As hr_admin, edit the past entry; provide the required reason.
- **Expected result / AC:** Correction applied with reason captured in audit; leader could not do this (TC-005).
- **Traceability:** F4.4, C-5.

#### TC-E4-F4.4-017 · HR/super_admin edit any company's cell
- [ ] **Platform:** Web · **POV:** hr_admin / super_admin · **Priority:** P1 · **Type:** RBAC
- **Objective:** Global scope for edits/clears.
- **Preconditions:** Entries at multiple companies.
- **Steps:** Edit/clear cells at two different companies.
- **Expected result / AC:** Both succeed; agents notified; audit per change.
- **Traceability:** F4.4, CH-1, SA-3.

## F4.4 · Web · Agent POV (RBAC denial — v1)

#### TC-E4-F4.4-018 · Agent cannot edit/clear any schedule cell
- [ ] **Platform:** Web · **POV:** agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Baseline employee has no schedule-write capability.
- **Preconditions:** Agent (no elevation).
- **Steps:** Direct API `PATCH`/clear on own and others' entries.
- **Expected result / AC:** 403 `FORBIDDEN`; no mutation; no edit UI on web.
- **Traceability:** F4.4, CONVENTIONS §17.

## F4.4 · Mobile · Agent POV — Swap / Day-off requests **[POST-V1]**

> v1 assertion for every case below: **the request UI is absent or disabled** (feature deferred 2026-05-29). The detailed flow is the post-v1 acceptance target. Execute "v1 expected"; hold "target".

#### TC-E4-F4.4-030 · Request entry point hidden/disabled in v1
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P0 · **Type:** RBAC/Edge [POST-V1]
- **Objective:** Confirm the swap/day-off request feature is not exposed in v1.
- **Preconditions:** Agent Budi on mobile, v1 build.
- **Steps:** Look for a "request swap / day-off" affordance on a shift entry; attempt the API `POST /schedule-change-requests` if it exists.
- **Expected result / AC:** **v1 expected:** no request UI; API route absent or returns 404/403. **Target (post-v1):** request flow available per TC-031..036.
- **Traceability:** F4.4 §10 (deferred 2026-05-29), FEATURE §7.

#### TC-E4-F4.4-031 · Agent requests a day-off; leader approves [POST-V1]
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Happy [POST-V1]
- **Objective:** Day-off request → leader approval clears/OFFs the shift; requester notified.
- **Preconditions:** Budi scheduled 2026-06-10; a leader exists for the company.
- **Steps:** Budi requests day-off for 2026-06-10 with a reason → leader approves.
- **Expected result / AC:** Request `Pending → Approved`; Budi's 2026-06-10 shift cleared/marked OFF (CH-4); Budi notified of approval; audited.
- **Traceability:** F4.4, CH-4, CH-6, CH-8, Gherkin "Agent requests a day-off".

#### TC-E4-F4.4-032 · Agent requests a swap with a counterpart; leader approves (atomic) [POST-V1]
- [ ] **Platform:** Mobile · **POV:** agent → shift_leader · **Priority:** P1 · **Type:** Happy [POST-V1]
- **Objective:** Swap with a same-company counterpart exchanges both shifts atomically on approval.
- **Preconditions:** Budi (Morning) and Citra (Night) both placed at Plaza Senayan, scheduled 2026-06-10.
- **Steps:** Budi submits a swap naming Citra for 2026-06-10 → leader approves.
- **Expected result / AC:** Budi gets "Night", Citra gets "Morning" on 2026-06-10 atomically (both or neither); both notified; one-day invariant still holds for both; audited.
- **Traceability:** F4.4, CH-3, CH-5, Gherkin "Agent requests a swap, approved".

#### TC-E4-F4.4-033 · Leader rejects a request with a required reason [POST-V1]
- [ ] **Platform:** Mobile · **POV:** shift_leader · **Priority:** P1 · **Type:** Negative [POST-V1]
- **Objective:** Rejection requires a reason; requester sees it.
- **Preconditions:** A pending request from Budi.
- **Steps:** Leader rejects with a reason; Budi opens the request.
- **Expected result / AC:** Status `Rejected`; reason mandatory (blocked if empty); Budi sees the reason; schedule unchanged; audited.
- **Traceability:** F4.4, CH-6, Gherkin "Reject a request with a reason".

#### TC-E4-F4.4-034 · Cross-company swap blocked [POST-V1]
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Negative [POST-V1]
- **Objective:** Both swap parties must be at the same (leader's) company.
- **Preconditions:** Counterpart placed at a different company.
- **Steps:** Budi submits a swap with that counterpart.
- **Expected result / AC:** Blocked ("both must be at the same company"); no exchange.
- **Traceability:** F4.4, CH-5, Gherkin "Cross-company swap is blocked".

#### TC-E4-F4.4-035 · Request escalates to HR when company has no leader [POST-V1]
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Edge [POST-V1]
- **Objective:** With no shift leader, requests route to HR for approval.
- **Preconditions:** Plaza Senayan has no shift leader; Budi placed there.
- **Steps:** Budi submits a day-off request.
- **Expected result / AC:** Request routes to an HR admin (CH-7 / F3.4 SL-7); HR can approve/reject.
- **Traceability:** F4.4, CH-7, Gherkin "Request escalates when there is no shift leader".

#### TC-E4-F4.4-036 · Agent withdraws a pending request [POST-V1]
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Edge [POST-V1]
- **Objective:** A pending request can be withdrawn before a decision.
- **Preconditions:** Budi has a Pending request.
- **Steps:** Budi withdraws it.
- **Expected result / AC:** Status `Withdrawn`; no schedule change; cannot withdraw after decision.
- **Traceability:** F4.4, CH-6, C-3.

#### TC-E4-F4.4-037 · Swap violating one-shift-per-day blocked [POST-V1]
- [ ] **Platform:** Mobile · **POV:** shift_leader · **Priority:** P2 · **Type:** Negative [POST-V1]
- **Objective:** A swap that would double-book either agent is blocked by conflict rules.
- **Preconditions:** A swap that would leave one agent with two shifts on the date.
- **Steps:** Attempt to approve such a swap.
- **Expected result / AC:** Blocked (CH-1 / INV-1); 409 `DOUBLE_SHIFT`.
- **Traceability:** F4.4, C-1, INV-1.

#### TC-E4-F4.4-038 · Counterpart not scheduled on swap date (give-away) [POST-V1]
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Edge [POST-V1]
- **Objective:** Confirm behavior when the counterpart has no shift on the date (one-way give-away vs require mutual).
- **Preconditions:** Counterpart unscheduled on the swap date.
- **Steps:** Budi requests a swap; leader approves.
- **Expected result / AC:** Behaves per the post-v1 resolution of the open item (C-2): either becomes a give-away (counterpart gains the shift) or is required-mutual and blocked. Record actual; spec-open.
- **Traceability:** F4.4, C-2, §10 deferred open item.

#### TC-E4-F4.4-039 · Counterpart placement ends before swap date [POST-V1]
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Negative [POST-V1]
- **Objective:** Swap blocked if the counterpart's placement is inactive on the date.
- **Preconditions:** Counterpart's placement ends before the swap date.
- **Steps:** Submit/approve the swap.
- **Expected result / AC:** Blocked (placement inactive); 422 `OUTSIDE_PLACEMENT_PERIOD`.
- **Traceability:** F4.4, C-6, SA-5.

#### TC-E4-F4.4-040 · Day-off vs formal leave boundary [POST-V1]
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Edge [POST-V1]
- **Objective:** A one-off day-off is operational; a multi-day/entitled absence should route to E6 Leave.
- **Preconditions:** Agent attempting a multi-day "day-off".
- **Steps:** Attempt a multi-day day-off request.
- **Expected result / AC:** Per post-v1 design: a one-off single-day swap stays in F4.4; multi-day/entitled absence is directed to E6 leave. Record actual; spec-open boundary.
- **Traceability:** F4.4, C-4, §10 deferred.

---

## 3. Cross-cutting / regression checklist (run after any E4 change)

- [ ] Every schedule write produced an **audit entry** (actor, before/after) — SA-9, SM-6, CH-8, CONVENTIONS §16.1.
- [ ] Every publish/change fired a **notification** to affected agent(s) — INV-4, CONVENTIONS §16.2.
- [ ] `(employee_id, work_date)` uniqueness holds under all paths (assign/replace/swap/concurrent) — INV-1.
- [ ] All schedule list endpoints use **cursor pagination**, never offset — CONVENTIONS §8.
- [ ] All times rendered in **Asia/Jakarta**; cross-midnight spans two days everywhere — SV-6, SA-8.
- [ ] All copy via i18n (Bahasa default); error messages from the envelope `message`/`fields`.
- [ ] Shift-leader role + company scope **derived per request** from the E3 assignment (no stored trust) — CONVENTIONS §17.
- [ ] No dead-flow states: every action has a designed result (toast/modal/empty/loading/error).
- [ ] **Spec gaps to track:** roster-compliance indicators (TC-050..052) undefined in E4; F4.4 agent-request flow deferred (TC-030..040); 24h shift (TC-012) & swap give-away (TC-038)/day-off-leave boundary (TC-040) are open decisions.
