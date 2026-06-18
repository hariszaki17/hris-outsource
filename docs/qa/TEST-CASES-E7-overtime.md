# Test Cases · E7 — Overtime Tracking (Manual QA)

> **Epic:** E7 Overtime Tracking · **Status:** Draft v1 · **Generated:** 2026-06-17
> **Sources:** [FEATURE.md](../epics/E7-overtime/FEATURE.md), PRDs F7.1 [overtime-rules](../epics/E7-overtime/prds/overtime-rules.md), F7.2 [overtime-capture](../epics/E7-overtime/prds/overtime-capture.md), F7.3 [overtime-approval](../epics/E7-overtime/prds/overtime-approval.md), F7.4 [overtime-records](../epics/E7-overtime/prds/overtime-records.md); [api/CONVENTIONS.md](../api/CONVENTIONS.md).

---

## 1. Scope

This document contains **exhaustive manual test cases** for E7 Overtime Tracking, organized per **platform** (Web console / Mobile app) × per **point-of-view** (super admin · HR/placement admin · shift leader · agent).

E7 covers four features:

| ID | Feature | Primary surface(s) |
|----|---------|--------------------|
| **F7.1** | Overtime Rules (day-type tiers) + holiday calendar | Web (HR / super admin) |
| **F7.2** | Overtime Capture (request + auto-detect) | Mobile (agent), Web/Mobile (shift leader) |
| **F7.3** | Overtime Approval (via the E11 engine) | Web + Mobile (approval-line members), Mobile (agent confirm) |
| **F7.4** | Overtime Records & Reporting | Mobile (agent), Web/Mobile (shift leader), Web (HR / super admin) |

**Domain facts under test:**
- **Day-type tiers:** Workday / RestDay / Holiday. Global only — exactly one rule per day type (OR-1).
- **Statutory multipliers (reference only in v1, per PP 35/2021):** Workday 1.5× (first hour) then 2.0× (subsequent hours); RestDay / Holiday progressive 2.0× → 3.0× → 4.0×. v1 records **hours only** — multipliers are stored as reference and shown in reports; no money is computed (INV-2, OR-3).
- **`min_minutes` = 60** (FEATURE §7); OT below the threshold is not counted (INV-5, OC-4).
- **Rest day** = a day the agent has no scheduled shift (OR-5). **Holiday > RestDay > Workday** precedence (OA-4, FEATURE §7, C-2).
- **Cross-midnight OT** → attributed to the **start date** (FEATURE §7, F7.1 C-1).
- **Approval routes through the E11 engine** (per-company chain); OT contributes only the `OnApproved` count-by-day-type hook (INV-3, OA-1).
- **Auto-detected OT** is a candidate that the **agent confirms first**, then routes to approval (INV-4, OC-7, OA-2).
- **Internal-only system:** client companies are data, not tenants; four roles only.

> Note on multipliers: PP 35/2021 progressive percentages are encoded here as the multipliers HR seeds into `OvertimeRule`. Because v1 is hours-only, **Calc** cases verify the *day-type classification, hour accounting, and the reference multiplier displayed* — not a payroll-money result. Each Calc case states input hours, the per-hour multiplier each hour maps to, and the expected reference weighted-hours figure that reports surface.

---

## 2. Coverage matrix

Legend: ● primary surface for that role · ○ secondary/visibility · — not applicable.

| Feature | Surface | Super admin | HR / placement admin | Shift leader | Agent |
|---------|---------|:-----------:|:--------------------:|:------------:|:-----:|
| **F7.1** Rules + holiday calendar | Web | ● | ● | — | — |
| **F7.1** | Mobile | — | — | — | — |
| **F7.2** Capture (request) | Mobile | — | — | ● (on behalf) | ● |
| **F7.2** Capture (request) | Web | ○ | ○ | ● (on behalf) | — |
| **F7.2** Auto-detect + confirm | Mobile | — | — | — | ● (confirm) |
| **F7.3** Approval | Web | ● (bypass) | ● | ● | ○ (timeline) |
| **F7.3** Approval | Mobile | ○ | ○ | ● | ● (confirm/timeline) |
| **F7.4** Records & reporting | Web | ● | ● | ● (own company) | — |
| **F7.4** Records & reporting | Mobile | ○ | ○ | ● (own company) | ● (own) |

**Test-type tags:** Happy · Negative · Edge · RBAC · Empty/Loading/Error · Calc.
**Priority:** P0 (critical path / data integrity / RBAC) · P1 (important) · P2 (edge / polish).

---

## F7.1 — Overtime Rules (day-type tiers) + holiday calendar

### Web · HR / placement admin POV

#### TC-E7-F7.1-001 · Create the three statutory day-type rules
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Happy
- **Objective:** HR can define one global OT rule per day type with reference multipliers.
- **Preconditions:** Logged in as HR; no OT rules yet (fresh tenant) or rules screen reachable from Lembur → Aturan/Rules.
- **Steps:**
  1. Open OT Rules.
  2. Create Workday rule: `multiplier` 1.5, `min_minutes` 60, `requires_preapproval` off; Save.
  3. Create RestDay rule: `multiplier` 2.0, `min_minutes` 60; Save.
  4. Create Holiday rule: `multiplier` 3.0, `min_minutes` 60; Save.
- **Expected result / AC:** Three rules persist (Workday/RestDay/Holiday), each with its multiplier as **reference** and a 60-minute threshold; success toast per save; audit entry written. Multipliers are not applied to any money figure.
- **Traceability:** F7.1, OR-1, OR-3, OR-4, US (Gherkin "Define tiered rules"), INV-2.

#### TC-E7-F7.1-002 · Enforce one global rule per day type (duplicate blocked)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Negative
- **Objective:** A second Workday rule cannot be created.
- **Preconditions:** A Workday rule already exists.
- **Steps:**
  1. Attempt to create another Workday rule.
  2. Save.
- **Expected result / AC:** Save blocked with a validation/`409 Conflict`-style message ("a rule for this day type already exists"); only one Workday rule remains. The day-type select offers Edit instead of duplicate-create once a tier exists.
- **Traceability:** F7.1, OR-1, US (Gherkin "One global rule per day type").

#### TC-E7-F7.1-003 · Validation — multiplier must be > 0, min_minutes ≥ 0
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Negative
- **Objective:** Invalid rule values are rejected.
- **Preconditions:** OT Rules create form open.
- **Steps:**
  1. Enter multiplier `0` (then `-1`) and min_minutes `-5`.
  2. Save.
- **Expected result / AC:** Field-level validation errors (`multiplier > 0`, `min_minutes >= 0`); save disabled/rejected; no persist. Matches FEATURE flowchart validation gate `multiplier>0, min>=0`.
- **Traceability:** F7.1, FEATURE §F7.1 flow (S1), OR-1.

#### TC-E7-F7.1-004 · Deactivate (not delete) a referenced rule
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Negative
- **Objective:** A rule referenced by OT records cannot be deleted, only deactivated.
- **Preconditions:** At least one Approved OT record references the Workday rule.
- **Steps:**
  1. Attempt to Delete the Workday rule.
  2. Then Deactivate it instead.
- **Expected result / AC:** Delete is blocked (`409`/disabled with explanation "rule is referenced; deactivate instead"); Deactivate succeeds, status → inactive; existing records keep their reference; audited.
- **Traceability:** F7.1, OR-6, US (Gherkin "Cannot delete a referenced rule").

#### TC-E7-F7.1-005 · Edit an existing rule's reference multiplier
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Happy
- **Objective:** HR can update a rule multiplier; change is audited.
- **Preconditions:** Holiday rule exists at 3.0.
- **Steps:** Edit Holiday rule multiplier 3.0 → 3.5; Save.
- **Expected result / AC:** Persisted; audit entry; future classifications/reports show 3.5 as the Holiday reference. Already-recorded hours unchanged (hours-only model).
- **Traceability:** F7.1, OR-3, OR-6.

#### TC-E7-F7.1-006 · No rule for a day type → default + flag
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** When a day type has no rule, classification falls back to a default and flags.
- **Preconditions:** Delete/deactivate the Holiday rule so no active Holiday rule exists; an agent works OT on a holiday.
- **Steps:**
  1. Trigger OT capture on a holiday date with no active Holiday rule.
  2. View the resulting OT record / rules screen warning.
- **Expected result / AC:** OT is still captured using a fallback (e.g., default multiplier) and the record/report is **flagged** as missing-rule; HR sees a warning to define the Holiday rule.
- **Traceability:** F7.1, C-3.

#### TC-E7-F7.1-007 · Empty state — no OT rules defined
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Rules screen shows a helpful empty state.
- **Preconditions:** Fresh environment, no rules.
- **Steps:** Open OT Rules.
- **Expected result / AC:** Empty state with CTA to create the three statutory tiers; no console error.
- **Traceability:** F7.1.

### Web · HR / placement admin POV — Holiday calendar

#### TC-E7-F7.1-010 · Bootstrap holiday calendar — "Import {tahun}" prefill
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Happy
- **Objective:** HR can prefill the public-holiday calendar for a year and confirm.
- **Preconditions:** Holiday calendar empty for 2026; logged in as HR.
- **Steps:**
  1. Open Holiday Calendar.
  2. Click **Import 2026**.
  3. Review the prefilled list of Indonesian national public holidays for 2026.
  4. Confirm/Save.
- **Expected result / AC:** A draft list of 2026 national holidays is prefilled (recurring fixed dates + that year's movable dates); HR confirms; entries persist with `recurring` flags set appropriately; audited. These dates now drive the Holiday day_type (OR-5).
- **Traceability:** F7.1, OR-5, C-4, US (Gherkin "Holiday calendar drives the Holiday tier").

#### TC-E7-F7.1-011 · Add cuti bersama (collective leave) days during import confirm
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Happy
- **Objective:** HR can add cuti bersama one-off dates not in the statutory prefill.
- **Preconditions:** "Import 2026" prefill reviewed but not yet confirmed.
- **Steps:**
  1. In the prefill review, add a cuti bersama date (e.g., 2026-03-20) with name "Cuti Bersama Nyepi".
  2. Confirm/Save.
- **Expected result / AC:** Cuti bersama saved as a one-off (`recurring = false`) holiday entry alongside statutory holidays; treated as Holiday day_type for OT classification.
- **Traceability:** F7.1, OR-5, C-4.

#### TC-E7-F7.1-012 · Add a single one-off holiday manually
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Happy
- **Objective:** Manual add of a movable/one-off holiday.
- **Preconditions:** Calendar populated for 2026.
- **Steps:** Add holiday `date` 2026-08-17, name "Hari Kemerdekaan", `recurring = true`; Save.
- **Expected result / AC:** Persisted; an agent working OT on 2026-08-17 is classified Holiday. Recurring flag means it auto-applies for future years.
- **Traceability:** F7.1, OR-5, C-4, US (Gherkin "2026-08-17 in the holiday calendar").

#### TC-E7-F7.1-013 · Recurring vs one-off classification
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Recurring entries reapply annually; movable holidays must be added per year.
- **Preconditions:** A recurring entry (Aug 17) and a movable entry (Idul Fitri 2026) exist.
- **Steps:**
  1. Switch the calendar year to 2027.
  2. Inspect which entries carried over.
- **Expected result / AC:** Recurring fixed-date holidays appear for 2027 automatically; movable holidays do **not** auto-populate and prompt HR to import/add 2027 dates.
- **Traceability:** F7.1, C-4.

#### TC-E7-F7.1-014 · Duplicate holiday date prevented
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Negative
- **Objective:** Two holiday entries for the same date are prevented/merged.
- **Preconditions:** 2026-08-17 already exists.
- **Steps:** Add another entry for 2026-08-17.
- **Expected result / AC:** Blocked or de-duplicated with a clear message; one entry per date.
- **Traceability:** F7.1, OR-5.

#### TC-E7-F7.1-015 · Re-import same year does not duplicate confirmed entries
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Re-running Import 2026 after confirmation is idempotent.
- **Preconditions:** 2026 already imported and confirmed.
- **Steps:** Click Import 2026 again; review; confirm.
- **Expected result / AC:** No duplicate dates created; the diff shows only new/changed holidays; existing manual additions (cuti bersama) preserved.
- **Traceability:** F7.1, OR-5, C-4.

#### TC-E7-F7.1-016 · Import error/network failure surfaces gracefully
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Import shows loading and recovers from failure.
- **Preconditions:** Simulate backend/network error during Import 2026.
- **Steps:** Trigger Import 2026 with the backend unavailable.
- **Expected result / AC:** Loading indicator during fetch; on failure an error state with Retry; no partial/corrupt calendar persisted.
- **Traceability:** F7.1, CONVENTIONS (error handling).

### Web · Super admin POV

#### TC-E7-F7.1-020 · Super admin manages rules and calendar (same as HR + statutory seed)
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Super admin has full rule/calendar management and can seed statutory multipliers.
- **Preconditions:** Logged in as super admin; fresh environment.
- **Steps:**
  1. Seed statutory multipliers: Workday 1.5, RestDay 2.0, Holiday 3.0 (or via a "seed defaults" action if present).
  2. Run Import {current year} for the holiday calendar.
- **Expected result / AC:** Rules + calendar created; audited under super-admin identity; values available to all companies (global).
- **Traceability:** F7.1, OR-1, FEATURE §2 (HR/Super Admin manage OT rules + holiday calendar).

### Web · Shift leader POV / Agent POV (RBAC)

#### TC-E7-F7.1-030 · Shift leader cannot access OT Rules
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Rules are back-office; shift leader is denied.
- **Preconditions:** Logged in as shift leader.
- **Steps:** Navigate to the OT Rules / Holiday Calendar URL directly.
- **Expected result / AC:** `403 Forbidden`; UI shows `comp/EmptyNoPermission`; no rule data leaked. (Client RBAC is defense-in-depth; server denies regardless.)
- **Traceability:** F7.1 §4 (mobile/leader not surfaced), CONVENTIONS §403, FEATURE §6.

#### TC-E7-F7.1-031 · Agent has no rules/calendar surface (mobile)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** RBAC
- **Objective:** OT rules/calendar are not exposed on mobile to agents.
- **Preconditions:** Logged in as agent (mobile).
- **Steps:** Inspect mobile nav and attempt any rules/calendar deep link.
- **Expected result / AC:** No rules/calendar entry in agent mobile nav; any forced route is denied/not found.
- **Traceability:** F7.1 §4 ("Not surfaced — rules are back-office").

---

## F7.2 — Overtime Capture (request + auto-detect)

### Mobile · Agent POV

#### TC-E7-F7.2-001 · Agent pre-requests OT (happy path)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent submits an OT request that becomes a Pending record.
- **Preconditions:** Agent "Budi" has an active placement at "Plaza Senayan"; rules exist; logged in (mobile).
- **Steps:**
  1. Open OT → Request OT.
  2. Enter `work_date` 2026-06-10, `start_at`/`end_at` totalling 2 hours, a reason.
  3. Submit.
- **Expected result / AC:** A `Pending` OT record is created with `source = Requested`, `duration_minutes = 120`, `day_type` classified from schedule+calendar (Workday on 2026-06-10), `attendance_id = null`; confirmation/toast; an E11 approval instance is created (F7.3).
- **Traceability:** F7.2, OC-1, OC-3, OC-7, US (Gherkin "Agent pre-requests OT"), INV-1.

#### TC-E7-F7.2-002 · Request blocked with no active placement
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Negative
- **Objective:** OT cannot be requested for a date with no active placement.
- **Preconditions:** Budi has no active placement on 2026-07-01.
- **Steps:** Request OT for 2026-07-01; Submit.
- **Expected result / AC:** Blocked with a clear message ("no active placement for this date"); no record created.
- **Traceability:** F7.2, OC-6, US (Gherkin "Request without placement blocked").

#### TC-E7-F7.2-003 · Reason required / field validation
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Negative
- **Objective:** A request requires a date, valid time range, and reason.
- **Preconditions:** Request form open.
- **Steps:**
  1. Submit with end_at before start_at.
  2. Submit with empty reason.
- **Expected result / AC:** Validation errors; submit blocked; no record. Time range must be positive; reason mandatory.
- **Traceability:** F7.2, OC-1, OC-5.

#### TC-E7-F7.2-004 · Below-threshold request (< 60 min) rejected/ignored
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Requested OT under `min_minutes` is not counted.
- **Preconditions:** `min_minutes = 60`.
- **Steps:** Request OT of 40 minutes; Submit.
- **Expected result / AC:** Either blocked at submit with "below minimum 60 min" or created but flagged "not counted"; per OC-4 it is not counted toward totals.
- **Traceability:** F7.2, OC-4, INV-5.

#### TC-E7-F7.2-005 · Cross-midnight request attributed to start date
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** OT 22:00 2026-06-14 → 02:00 2026-06-15 is attributed to the start day.
- **Preconditions:** Active placement; rules exist.
- **Steps:** Request OT start 22:00 on 2026-06-14, end 02:00 on 2026-06-15; Submit.
- **Expected result / AC:** `work_date = 2026-06-14` (start day), `duration_minutes = 240`; day_type derived from 2026-06-14. Spanning midnight does not split the record.
- **Traceability:** F7.2, OC-5, C-1, F7.1 C-1, FEATURE §7 (cross-midnight → start date).

#### TC-E7-F7.2-006 · Rest-day OT classification on request
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Calc
- **Objective:** Working on a day with no scheduled shift classifies RestDay.
- **Preconditions:** Budi has no scheduled shift on 2026-06-14 (his weekly off); RestDay rule = 2.0.
- **Steps:** Request OT 3 hours on 2026-06-14; Submit.
- **Expected result / AC:** Record `day_type = RestDay`, `duration_minutes = 180`. Reference multiplier shown = 2.0 (or progressive 2/3/4 if encoded per-hour — see Calc cases TC-E7-F7.4-040+). No money computed.
- **Traceability:** F7.2, OC-3, OR-5, US (Gherkin "Rest-day OT classification").

#### TC-E7-F7.2-007 · Holiday classification on request
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Calc
- **Objective:** OT on a calendar holiday classifies Holiday.
- **Preconditions:** 2026-08-17 in calendar; Holiday rule = 3.0; agent placed.
- **Steps:** Request 2 hours OT on 2026-08-17; Submit.
- **Expected result / AC:** `day_type = Holiday`, duration 120; reference multiplier 3.0 displayed.
- **Traceability:** F7.2, OC-3, OR-5.

#### TC-E7-F7.2-008 · Holiday + rest day same date → Holiday precedence
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** When a date is both a holiday and the agent's rest day, classify Holiday.
- **Preconditions:** 2026-08-17 is in the holiday calendar AND Budi has no scheduled shift that day.
- **Steps:** Request OT on 2026-08-17.
- **Expected result / AC:** `day_type = Holiday` (Holiday > RestDay); reference multiplier = Holiday rule.
- **Traceability:** F7.2, C-5, F7.1 C-2, OA-4 (HOLIDAY > RESTDAY > WORKDAY), FEATURE §7.

#### TC-E7-F7.2-009 · Confirm an auto-detected OT candidate
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent confirms a system-raised candidate, which then enters approval.
- **Preconditions:** Verified attendance for Budi shows clock-out 16:30 vs scheduled shift end 15:00 on 2026-06-10; min_minutes 60 → 90-min candidate exists `source = AutoDetected`, linked `attendance_id`.
- **Steps:**
  1. Open OT → see the auto-detected candidate (90 min, 2026-06-10).
  2. Review and Confirm.
- **Expected result / AC:** Candidate confirmed; an E11 `ApprovalInstance` (`request_type = OVERTIME`) is created and routed; status `Pending`; until confirmed it does not enter the chain (OC-7/OA-2).
- **Traceability:** F7.2, OC-2, OC-7, OA-2, INV-4, US (Gherkin "Auto-detect OT from verified attendance" + F7.3 "Auto-detected candidate").

#### TC-E7-F7.2-010 · Agent dismisses/declines an auto-detected candidate
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Agent can decline a candidate they did not actually work.
- **Preconditions:** An unconfirmed AutoDetected candidate exists.
- **Steps:** Open candidate → Decline/Dismiss.
- **Expected result / AC:** Candidate does not create an approval instance and is not counted (INV-4); action audited; the underlying attendance record is unaffected.
- **Traceability:** F7.2, OC-7, INV-4, F7.3 OA-5.

#### TC-E7-F7.2-011 · No double request (duplicate protection)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Negative
- **Objective:** Auto-detect must not create a duplicate when OT already requested for the same shift.
- **Preconditions:** Budi already requested OT for his 2026-06-10 shift.
- **Steps:** Trigger auto-detect for the same shift (simulate the detection job).
- **Expected result / AC:** No duplicate candidate created for that shift; existing record retained.
- **Traceability:** F7.2, OC-8, US (Gherkin "No double-counting").

#### TC-E7-F7.2-012 · Empty state — no OT to show / no candidates
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Agent OT screen with nothing pending shows an empty state.
- **Preconditions:** Agent with no OT records/candidates.
- **Steps:** Open OT screen.
- **Expected result / AC:** Friendly empty state + Request OT CTA; loading skeleton while fetching; error state with retry on failure.
- **Traceability:** F7.2, F7.4 C-1.

#### TC-E7-F7.2-013 · Auto-detected OT corrected after E5 attendance correction
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** Correcting the source attendance re-derives the candidate.
- **Preconditions:** AutoDetected candidate from a clock-out later corrected in E5 (16:30 → 16:00).
- **Steps:** Apply the E5 correction; re-open the candidate.
- **Expected result / AC:** Candidate duration/day_type re-derived (now 60 min); candidate re-evaluated against min_minutes; if it drops below threshold it is ignored.
- **Traceability:** F7.2, C-2, OC-5.

### Mobile / Web · Shift leader POV

#### TC-E7-F7.2-020 · Shift leader requests OT on behalf of an agent
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** A leader can capture OT for an agent in their company.
- **Preconditions:** Leader of "Plaza Senayan"; agent Budi placed there.
- **Steps:** Open OT → Request on behalf → select Budi → date 2026-06-10, 2 h, reason; Submit.
- **Expected result / AC:** Pending OT record created for Budi `source = Requested`, attributed correctly; enters E11 chain; audited under the leader's identity.
- **Traceability:** F7.2 §4 (leader requests on behalf), OC-1, OC-7.

#### TC-E7-F7.2-021 · Leader cannot request OT for an agent outside their company
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Scope limits on-behalf requests to the leader's company.
- **Preconditions:** Leader of "Plaza Senayan"; agent placed at a different company.
- **Steps:** Attempt on-behalf request for an out-of-scope agent.
- **Expected result / AC:** `403`/blocked; agent not selectable in the picker; no record.
- **Traceability:** F7.2, F7.4 OR-1 (scope), CONVENTIONS §403.

#### TC-E7-F7.2-022 · Leader requests OT on mobile
- [ ] **Platform:** Mobile · **POV:** Shift leader · **Priority:** P2 · **Type:** Happy
- **Objective:** On-behalf request works from the leader mobile surface.
- **Preconditions:** Leader logged in (mobile).
- **Steps:** Request OT on behalf of an in-scope agent.
- **Expected result / AC:** Same as TC-E7-F7.2-020 from mobile (comp/SLMobileNav surface).
- **Traceability:** F7.2 §4 (Web/mobile leader).

### System auto-detect (verify via observable effects)

#### TC-E7-F7.2-030 · Auto-detect threshold — 90 min over → candidate created
- [ ] **Platform:** (System; verify on Mobile agent) · **POV:** Agent · **Priority:** P0 · **Type:** Calc
- **Objective:** Verified clock-out 90 min past scheduled end creates a 90-min candidate.
- **Preconditions:** Scheduled shift end 15:00; verified clock-out 16:30; min_minutes 60.
- **Steps:** Run/await the OT detection job; check Budi's OT candidates.
- **Expected result / AC:** Candidate `duration_minutes = 90`, `source = AutoDetected`, `attendance_id` linked, `status = Pending` (awaiting agent confirm). Excess = 90 ≥ 60 → created.
- **Traceability:** F7.2, OC-2, US (Gherkin auto-detect), INV-4.

#### TC-E7-F7.2-031 · Auto-detect below threshold — 20 min over → ignored
- [ ] **Platform:** (System; verify on Mobile agent) · **POV:** Agent · **Priority:** P0 · **Type:** Calc
- **Objective:** 20 min over shift end is below 60 and not created.
- **Preconditions:** Shift end 15:00; verified clock-out 15:20; min_minutes 60.
- **Steps:** Run detection; check candidates.
- **Expected result / AC:** No OT record created (20 < 60). Excess ignored.
- **Traceability:** F7.2, OC-4, INV-5, US (Gherkin "Below threshold is ignored").

#### TC-E7-F7.2-032 · Auto-detect exactly at threshold (60 min)
- [ ] **Platform:** (System; verify on Mobile agent) · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Exactly 60 min over is counted (≥ threshold).
- **Preconditions:** Shift end 15:00; clock-out 16:00; min_minutes 60.
- **Steps:** Run detection.
- **Expected result / AC:** Candidate of 60 min created (boundary inclusive per OC-2 "≥ min_minutes").
- **Traceability:** F7.2, OC-2 (≥), INV-5.

#### TC-E7-F7.2-033 · Auto-detect on a rest day (worked unscheduled)
- [ ] **Platform:** (System; verify on Mobile agent) · **POV:** Agent · **Priority:** P1 · **Type:** Calc
- **Objective:** Verified attendance on a day with no schedule → RestDay candidate.
- **Preconditions:** No scheduled shift on 2026-06-14; verified attendance shows 4 worked hours.
- **Steps:** Run detection.
- **Expected result / AC:** RestDay candidate created (the whole worked time is OT since there is no scheduled shift to net against); day_type RestDay.
- **Traceability:** F7.2, OC-2, OC-3, OR-5.

---

## F7.3 — Overtime Approval (via the E11 engine)

### Web · Shift leader POV (line member)

#### TC-E7-F7.3-001 · Line-1 leader approves OT in Kotak Masuk (inbox)
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** Happy
- **Objective:** The on-site leader (line 1) clears their line.
- **Preconditions:** Budi has a Pending 2-h OT at Plaza Senayan; template line 1 = [Sari (leader)], line 2 = [HR]; logged in as Sari.
- **Steps:**
  1. Open Kotak Masuk → OT instance for Budi.
  2. Approve.
- **Expected result / AC:** Line 1 cleared; instance advances to line 2 (HR); `OvertimeRecord.status` stays `Pending` (still routing); `approval_actions` appended; Budi/HR notified.
- **Traceability:** F7.3, OA-1, OA-6, US (Gherkin "Chain approves"), INV-3.

#### TC-E7-F7.3-002 · Leader rejects OT with reason → terminal Rejected
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** Happy
- **Objective:** A current-line reject ends the instance; nothing counts.
- **Preconditions:** Pending OT; leader on the current line.
- **Steps:** Open the OT instance → Reject → enter reason → Confirm.
- **Expected result / AC:** `OvertimeRecord.status = REJECTED`; `OnRejected` hook fires — nothing counts toward F7.4; underlying attendance unaffected; reason recorded in `approval_actions`; agent notified.
- **Traceability:** F7.3, OA-5, OA-6, INV-4, US (Gherkin "Reject ends it"), C-1.

#### TC-E7-F7.3-003 · Reject requires a reason
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P1 · **Type:** Negative
- **Objective:** Reject cannot be submitted without a reason.
- **Preconditions:** Pending OT; current line.
- **Steps:** Reject with empty reason → Confirm.
- **Expected result / AC:** Validation blocks submit; no state change until a reason is provided.
- **Traceability:** F7.3, OA-6.

#### TC-E7-F7.3-004 · Self-approval blocked (INV-3)
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** A line member cannot clear the line for their own OT.
- **Preconditions:** Leader Sari has her own OT record and sits on its chain.
- **Steps:** Open her own OT instance and attempt to Approve.
- **Expected result / AC:** Approve disabled/`403` for the self-record line; another member (or super-admin bypass) must clear it; clear message.
- **Traceability:** F7.3, OA-1 (self-block), INV-3, US (Gherkin "Cannot self-approve").

#### TC-E7-F7.3-005 · Leader cannot act on another company's OT
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Scope restricts leaders to instances where they are a current-line member.
- **Preconditions:** OT instance for an agent at a company the leader does not lead.
- **Steps:** Attempt to open/approve that OT instance (direct deep link).
- **Expected result / AC:** `403`/not in inbox; `comp/EmptyNoPermission`; no data leak.
- **Traceability:** F7.3, OA-1, F7.4 OR-1, CONVENTIONS §403.

#### TC-E7-F7.3-006 · Bulk approve multiple OT instances
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Leader bulk-clears their current line across several OT instances.
- **Preconditions:** Five pending OT instances where the leader is the current-line member.
- **Steps:** Select all five → Bulk approve.
- **Expected result / AC:** Each clears the leader's line and advances; partial-success reporting if any fails (e.g., one became self-blocked); audited per instance (`:bulk-approve`, CONVENTIONS §14).
- **Traceability:** F7.3, OA-7, US (Gherkin "Bulk approve").

#### TC-E7-F7.3-007 · Lembur → Approvals per-domain tab mirrors inbox
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P2 · **Type:** Happy
- **Objective:** The Lembur → Approvals tab is a view over the same E11 instances.
- **Preconditions:** Pending OT instances on the leader's line.
- **Steps:** Open Lembur → Approvals tab.
- **Expected result / AC:** Same instances as Kotak Masuk (IB-5 view); approve/reject/bulk available; consistent state.
- **Traceability:** F7.3 §4, OA-7.

### Web · HR / placement admin POV (line member)

#### TC-E7-F7.3-010 · HR clears line 2 → terminal approve + count hook
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Happy
- **Objective:** Final line approval fires the OnApproved count-by-day-type hook.
- **Preconditions:** Line 1 (leader) already approved; HR is line 2; Budi's 2-h Workday OT.
- **Steps:** HR opens the OT instance → Approve.
- **Expected result / AC:** Instance APPROVED; `OvertimeRecord.status = APPROVED`; `OnApproved` hook counts 2 h under day_type Workday (F7.4) within the engine transaction; `approval_actions` appended; Budi notified.
- **Traceability:** F7.3, OA-4, OA-6, INV-3, US (Gherkin "Chain approves, hours count").

#### TC-E7-F7.3-011 · No template → super-admin fallback line
- [ ] **Platform:** Web · **POV:** HR/Super admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Companies with no approval template still route (never auto-approve, never block).
- **Preconditions:** Budi's company has no E11 approval template.
- **Steps:** Capture OT for Budi; inspect routing.
- **Expected result / AC:** Instance routes to the E11 super-admin fallback line; does not auto-approve; appears in the super admin's inbox.
- **Traceability:** F7.3, OA-3, US (Gherkin "No template falls back to super admin").

### Web · Super admin POV

#### TC-E7-F7.3-020 · Super-admin bypass force-approves; hook still counts
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Bypass terminally approves regardless of remaining lines, and counting still occurs.
- **Preconditions:** Pending OT mid-chain (line 1 not yet cleared).
- **Steps:** Super admin opens the OT instance → Bypass/Force-approve → confirm reason.
- **Expected result / AC:** Instance APPROVED via bypass; `OnApproved` counts the hours by day_type; bypass recorded in `approval_actions` with actor + reason; agent notified.
- **Traceability:** F7.3, OA-4, C-5, US (Gherkin via E11 bypass), INV-5 (E11 bypass).

#### TC-E7-F7.3-021 · Approve OT for a now-ended placement
- [ ] **Platform:** Web · **POV:** HR/Super admin · **Priority:** P2 · **Type:** Edge
- **Objective:** OT for work that already happened can be approved even if the placement ended.
- **Preconditions:** Pending OT for a placement that has since ended.
- **Steps:** Approve the OT instance through the chain.
- **Expected result / AC:** Approval allowed (work already occurred); counted; audited with a note that the placement is ended.
- **Traceability:** F7.3, C-4.

### Mobile · Shift leader POV

#### TC-E7-F7.3-030 · Leader approves OT from mobile inbox
- [ ] **Platform:** Mobile · **POV:** Shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Approve/reject available on the leader mobile surface.
- **Preconditions:** Pending OT on the leader's line; logged in (mobile, SLMobileNav).
- **Steps:** Open Kotak Masuk (mobile) → OT instance → Approve.
- **Expected result / AC:** Same effect as web (TC-E7-F7.3-001); line cleared, advances; audited.
- **Traceability:** F7.3 §4 (Web/mobile inbox).

#### TC-E7-F7.3-031 · Leader rejects OT from mobile
- [ ] **Platform:** Mobile · **POV:** Shift leader · **Priority:** P2 · **Type:** Happy
- **Objective:** Reject-with-reason works on mobile.
- **Preconditions:** Pending OT on leader's line (mobile).
- **Steps:** Reject → reason → confirm.
- **Expected result / AC:** Terminal Rejected; nothing counts; agent notified.
- **Traceability:** F7.3, OA-5, OA-6.

### Mobile · Agent POV

#### TC-E7-F7.3-040 · Agent watches the chain timeline
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** The agent sees their OT instance's E11 chain timeline and status.
- **Preconditions:** Budi has an OT instance routing through line 1 → line 2.
- **Steps:** Open the OT record → view the approval timeline.
- **Expected result / AC:** Timeline shows each line, who acted, when, and the current pending line; status updates to APPROVED/REJECTED on terminal; read-only.
- **Traceability:** F7.3 §4 (agent watches chain timeline), OA-6.

#### TC-E7-F7.3-041 · Agent withdraws a pending OT request
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Agent cancels a pending request before any finalize.
- **Preconditions:** Budi has a Pending OT request not yet terminally decided.
- **Steps:** Open the pending request → Withdraw/Cancel → confirm.
- **Expected result / AC:** `OvertimeRecord.status = Cancelled`; the E11 instance is closed; nothing counts; audited.
- **Traceability:** F7.3, C-3.

#### TC-E7-F7.3-042 · Agent cannot approve OT
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Agents have no approve/reject capability.
- **Preconditions:** Agent logged in (mobile).
- **Steps:** Inspect agent OT surfaces / attempt any approve action via deep link.
- **Expected result / AC:** No approve/reject controls for agents; any forced action `403`; only confirm/withdraw/timeline are available.
- **Traceability:** F7.3 §3 (agent = requester/confirmer only), CONVENTIONS §403.

#### TC-E7-F7.3-043 · Notification on terminal decision
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Happy
- **Objective:** Agent is notified when their OT is approved or rejected.
- **Preconditions:** Pending OT for the agent.
- **Steps:** Have an approver approve, then (separate record) reject.
- **Expected result / AC:** Push/in-app notification on each terminal decision with the outcome and reason (if rejected); deep-links to the record.
- **Traceability:** F7.3, OA-6 (agent notified via E10).

### Approval — error/edge

#### TC-E7-F7.3-050 · Stale instance — already actioned by another member
- [ ] **Platform:** Web · **POV:** HR/Shift leader · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Acting on an OT line another member just cleared is handled.
- **Preconditions:** Two line members; one approves first.
- **Steps:** Second member (with a stale view) attempts to approve the same line.
- **Expected result / AC:** Conflict handled gracefully (`409`/"already actioned"); view refreshes to the current line; no double action.
- **Traceability:** F7.3, OA-1, CONVENTIONS (409).

#### TC-E7-F7.3-051 · Attendance corrected after OT approval → flagged for re-review
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Correcting source attendance after approval flags the OT.
- **Preconditions:** An Approved AutoDetected OT whose source attendance is later corrected (E5).
- **Steps:** Apply the E5 correction; view the OT record/report.
- **Expected result / AC:** OT is re-derived or flagged for re-review (per open C-2); report notes the change; audited. (Resolution is outside the engine.)
- **Traceability:** F7.3, C-2, F7.4 C-3.

---

## F7.4 — Overtime Records & Reporting

### Mobile · Agent POV

#### TC-E7-F7.4-001 · Agent views own approved OT
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent sees their approved OT hours by date and status.
- **Preconditions:** Budi has approved OT records.
- **Steps:** Open "My overtime".
- **Expected result / AC:** List of approved OT with date, duration (hours), day_type, and status; pending/rejected may show separately and are excluded from approved totals.
- **Traceability:** F7.4, OR-1, OR-4, US (Gherkin "Agent views own approved OT").

#### TC-E7-F7.4-002 · Agent with no OT — empty state
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Empty state for an agent with no OT.
- **Preconditions:** Agent with zero OT.
- **Steps:** Open "My overtime".
- **Expected result / AC:** Empty state ("no overtime yet"); loading skeleton during fetch; error+retry on failure.
- **Traceability:** F7.4, C-1.

#### TC-E7-F7.4-003 · Agent sees only their own OT (scope)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Agent cannot see other agents' OT.
- **Preconditions:** Multiple agents with OT.
- **Steps:** Inspect "My overtime"; attempt any deep link to another agent's OT.
- **Expected result / AC:** Only own records returned; cross-agent access `403`.
- **Traceability:** F7.4, OR-1, CONVENTIONS §403.

#### TC-E7-F7.4-004 · Times render in Asia/Jakarta
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** OT times display in WIB regardless of device locale.
- **Preconditions:** Approved cross-midnight OT (22:00→02:00).
- **Steps:** View the record on a device set to a non-WIB timezone.
- **Expected result / AC:** Start/end render in Asia/Jakarta; work_date = start day; no off-by-one date.
- **Traceability:** F7.4, OR-7, C-1 (cross-midnight).

### Web / Mobile · Shift leader POV

#### TC-E7-F7.4-010 · Leader views team OT for their company
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Leader sees approved OT for agents in their company.
- **Preconditions:** Leader of Plaza Senayan; several agents with OT.
- **Steps:** Open OT report scoped to their company.
- **Expected result / AC:** Approved OT grouped by agent and day_type for their company; pending/rejected excluded from approved totals.
- **Traceability:** F7.4, OR-1, OR-2, OR-4.

#### TC-E7-F7.4-011 · Leader denied a company they don't lead
- [ ] **Platform:** Web · **POV:** Shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Scope enforced on the OT report.
- **Preconditions:** Leader of company A.
- **Steps:** Attempt to open the OT report for company B (deep link / filter).
- **Expected result / AC:** Access denied (`403`); `comp/EmptyNoPermission`; no rows from company B.
- **Traceability:** F7.4, OR-1, US (Gherkin "Scope enforced"), CONVENTIONS §403.

#### TC-E7-F7.4-012 · Leader views team OT on mobile
- [ ] **Platform:** Mobile · **POV:** Shift leader · **Priority:** P2 · **Type:** Happy
- **Objective:** Team OT visible on the leader mobile surface.
- **Preconditions:** Leader logged in (mobile).
- **Steps:** Open team OT.
- **Expected result / AC:** Company-scoped approved OT shown; same scope rules as web.
- **Traceability:** F7.4 §4 (Web/mobile leader).

### Web · HR / placement admin & Super admin POV — reporting & export

#### TC-E7-F7.4-020 · HR OT report by tier for a company/period
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Happy
- **Objective:** HR runs the OT report grouped by day_type, agent, with reference multipliers.
- **Preconditions:** Plaza Senayan has approved OT across Workday/RestDay/Holiday for June 2026.
- **Steps:** Run OT report for Plaza Senayan, period 2026-06-01 to 2026-06-30.
- **Expected result / AC:** Approved OT hours grouped by day_type (Workday/RestDay/Holiday) and agent; each tier shows its **reference multiplier** (1.5/2.0/3.0); no money column in v1.
- **Traceability:** F7.4, OR-2, OR-3, US (Gherkin "HR reports OT by tier").

#### TC-E7-F7.4-021 · Approved-only totals (exclude pending/rejected)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Calc
- **Objective:** Pending and rejected OT are excluded from approved totals.
- **Preconditions:** Budi in June 2026 has: Approved 2 h Workday, Pending 3 h Workday, Rejected 1 h Holiday.
- **Steps:** Run the report; read Budi's totals.
- **Expected result / AC:** Approved Workday total = 2 h only; the 3 h pending and 1 h rejected are NOT in approved totals (may appear in a separate pending/rejected breakdown). Reference Workday multiplier 1.5 shown.
- **Traceability:** F7.4, OR-4, US (Gherkin "Only approved OT counts").

#### TC-E7-F7.4-022 · Group by position (free-text)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Happy
- **Objective:** Report can aggregate by the free-text position carried from placement.
- **Preconditions:** Agents with differing positions have approved OT.
- **Steps:** Group the OT report by position.
- **Expected result / AC:** Hours aggregated per position string; positions match the placement's free-text position axis.
- **Traceability:** F7.4, OR-2 (position free-text), data model §6.

#### TC-E7-F7.4-023 · Export reflects filters and is audited (queued/202)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Happy
- **Objective:** Export of a filtered OT report is structured for payroll/billing and audited.
- **Preconditions:** OT report filtered by company + June 2026.
- **Steps:** Export (Excel/CSV/PDF).
- **Expected result / AC:** File reflects exactly the applied filters; structured for payroll import (E8) and billing (E10); the export action is audited; large exports return `202 Accepted` with a job id and complete asynchronously.
- **Traceability:** F7.4, OR-5, C-4, US (Gherkin "Export for payroll/billing"), CONVENTIONS (202 async).

#### TC-E7-F7.4-024 · Super admin cross-company OT report
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Super admin sees OT across all companies.
- **Preconditions:** OT across multiple companies.
- **Steps:** Run the OT report with no company filter.
- **Expected result / AC:** Cross-company aggregation; full visibility; export available.
- **Traceability:** F7.4, OR-1 (HR/Super Admin see all).

#### TC-E7-F7.4-025 · Read-only report deep-links to OT record/approval
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Happy
- **Objective:** Report rows deep-link to the underlying record + approval trail.
- **Preconditions:** Approved OT in the report.
- **Steps:** Click an OT row.
- **Expected result / AC:** Opens the read-only OT record with its E11 approval timeline; no edit affordances in the report.
- **Traceability:** F7.4, OR-6.

#### TC-E7-F7.4-026 · Late approval — OT appears in the worked period, flagged
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** OT approved after the period closed lands in the worked period and is flagged.
- **Preconditions:** OT worked 2026-06-28, approved 2026-07-03 (after June close).
- **Steps:** Run the June report and the July report.
- **Expected result / AC:** The OT appears in June (worked period), flagged as late-approval; not double-counted in July.
- **Traceability:** F7.4, C-2.

#### TC-E7-F7.4-027 · Migrated historical OT flagged (day_type defaulted)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** E9-migrated OT with defaulted day_type is flagged in reports.
- **Preconditions:** Migrated OT records with day_type defaulted to Workday/unclassified.
- **Steps:** Run a report covering migrated history.
- **Expected result / AC:** Migrated rows are flagged "unclassified / Workday-defaulted history" so totals are interpreted correctly.
- **Traceability:** F7.4, C-5.

#### TC-E7-F7.4-028 · Large-org export paginated/queued
- [ ] **Platform:** Web · **POV:** Super admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** A very large export is queued rather than blocking the UI.
- **Preconditions:** Large org with many OT records.
- **Steps:** Export a wide cross-company/period range.
- **Expected result / AC:** Export is queued (`202` + job id); user notified on completion; UI remains responsive; cursor pagination for the on-screen list.
- **Traceability:** F7.4, C-4, CONVENTIONS (202, cursor pagination).

### Calc cases — day-type tiers & PP 35/2021 progressive multipliers

> These verify **day-type classification + hour accounting + the reference multiplier surfaced**. v1 computes no money; the "reference weighted hours" below is the figure reports may surface as `Σ(hours × multiplier)` for the future payroll run. Statutory mapping (PP 35/2021):
> - **Workday:** hour 1 → 1.5×; hour 2 and beyond → 2.0×.
> - **Rest day / Public holiday (for a standard schedule):** progressive **2.0× → 3.0× → 4.0×** across worked hours.

#### TC-E7-F7.4-040 · Workday OT 1 hour → 1.5×
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Calc
- **Objective:** A single workday OT hour maps to the 1.5× first-hour tier.
- **Preconditions:** Approved 1 h Workday OT; Workday rule 1.5/2.0.
- **Steps:** Open the report row for that record.
- **Expected result / AC:** day_type = Workday; counted hours = 1.0; reference weighted hours = 1 × 1.5 = **1.5**; no money.
- **Traceability:** F7.4, OR-2, OR-3.

#### TC-E7-F7.4-041 · Workday OT 3 hours → 1.5× + 2.0× + 2.0×
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Calc
- **Objective:** Workday progression: first hour 1.5×, remaining at 2.0×.
- **Preconditions:** Approved 3 h Workday OT.
- **Steps:** Open the report row.
- **Expected result / AC:** Counted hours = 3.0; per-hour multipliers = [1.5, 2.0, 2.0]; reference weighted hours = 1.5 + 2.0 + 2.0 = **5.5**.
- **Traceability:** F7.4, OR-2, OR-3.

#### TC-E7-F7.4-042 · Rest-day OT 3 hours → 2× + 2× + 3×... (progressive)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Calc
- **Objective:** Rest-day progression for a 6-day-week schedule: hours 1–7 at 2×, hour 8 at 3×, hours 9–10 at 4× (PP 35/2021). For 3 hours, all at 2×.
- **Preconditions:** Approved 3 h RestDay OT (no scheduled shift that day); RestDay rule reference 2.0.
- **Steps:** Open the report row.
- **Expected result / AC:** day_type = RestDay; counted hours = 3.0; per-hour multipliers = [2.0, 2.0, 2.0]; reference weighted hours = **6.0**. (First 7 rest-day hours are 2×.)
- **Traceability:** F7.4, OR-2, OR-3, F7.2 OR-5.

#### TC-E7-F7.4-043 · Rest-day OT 9 hours → progressive 2×/3×/4×
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Calc
- **Objective:** Verify the full progressive ladder beyond the 8th hour.
- **Preconditions:** Approved 9 h RestDay OT on a 6-day-week schedule.
- **Steps:** Open the report row.
- **Expected result / AC:** Per-hour multipliers = hours 1–7 → 2.0× (×7), hour 8 → 3.0×, hour 9 → 4.0×; counted hours = 9.0; reference weighted hours = (7×2.0) + 3.0 + 4.0 = 14 + 3 + 4 = **21.0**.
- **Traceability:** F7.4, OR-2, OR-3.

#### TC-E7-F7.4-044 · Holiday OT 3 hours → 2× + 2× + 2× (first 8 hours)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Calc
- **Objective:** Holiday OT progression: first 8 hours at 2×, hour 9 at 3×, hours 10–11 at 4×.
- **Preconditions:** Approved 3 h Holiday OT on 2026-08-17; Holiday rule reference 3.0 (display) with progressive per-hour mapping 2×/3×/4×.
- **Steps:** Open the report row.
- **Expected result / AC:** day_type = Holiday; counted hours = 3.0; per-hour multipliers = [2.0, 2.0, 2.0]; reference weighted hours = **6.0**. (Holiday first-8-hours tier is 2× per PP 35/2021.)
- **Traceability:** F7.4, OR-2, OR-3, F7.1 (Holiday tier), OA-4.

#### TC-E7-F7.4-045 · Holiday OT 10 hours → progressive 2×/3×/4×
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Calc
- **Objective:** Full holiday ladder past the 8th and 9th hours.
- **Preconditions:** Approved 10 h Holiday OT.
- **Steps:** Open the report row.
- **Expected result / AC:** Per-hour multipliers = hours 1–8 → 2.0× (×8), hour 9 → 3.0×, hour 10 → 4.0×; counted hours = 10.0; reference weighted hours = (8×2.0) + 3.0 + 4.0 = 16 + 3 + 4 = **23.0**.
- **Traceability:** F7.4, OR-2, OR-3.

#### TC-E7-F7.4-046 · Holiday-over-rest-day uses Holiday ladder (precedence)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Calc
- **Objective:** When a date is both holiday and rest day, the Holiday tier/multipliers apply.
- **Preconditions:** Approved 3 h OT on 2026-08-17 where the agent also had no scheduled shift.
- **Steps:** Open the report row.
- **Expected result / AC:** day_type = Holiday (not RestDay); per-hour multipliers per the Holiday ladder = [2.0, 2.0, 2.0]; weighted hours = **6.0**. Confirms HOLIDAY > RESTDAY > WORKDAY.
- **Traceability:** F7.4, OA-4, F7.1 C-2, F7.2 C-5, FEATURE §7.

#### TC-E7-F7.4-047 · Cross-midnight OT counted on start day's tier
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Calc
- **Objective:** OT 22:00 (Workday 2026-06-14... note: 2026-06-14 is Budi's rest day) is counted on the start-day tier.
- **Preconditions:** Approved OT 22:00 2026-06-14 → 02:00 2026-06-15; 2026-06-14 is a RestDay for the agent, 2026-06-15 is a Workday.
- **Steps:** Open the report row.
- **Expected result / AC:** Entire 4 h counted under work_date 2026-06-14 with day_type = RestDay (start day governs); reference per-hour multipliers = [2.0×4] → weighted hours = **8.0**; not split across 06-15.
- **Traceability:** F7.4, OR-7, C-1, F7.1 C-1, FEATURE §7 (cross-midnight → start date).

#### TC-E7-F7.4-048 · Below-threshold excluded from tier totals
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Calc
- **Objective:** A 40-minute record (below 60) does not add to any tier total.
- **Preconditions:** A 40-min OT exists (created-but-not-counted or blocked); other approved OT exists.
- **Steps:** Run the report.
- **Expected result / AC:** The 40-min record contributes 0 to all day-type totals; flagged "below minimum" if surfaced.
- **Traceability:** F7.4, OR-2, F7.2 OC-4, INV-5.

#### TC-E7-F7.4-049 · Correction re-derives approved OT hours in the report
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Calc
- **Objective:** When a correction changes approved hours, the report reflects re-derived hours.
- **Preconditions:** Approved 90-min OT later re-derived to 60 min after an E5 correction.
- **Steps:** Apply correction; re-run report.
- **Expected result / AC:** Report shows 60 min (1.0 h) for that record; change audited; reference weighting recomputed accordingly.
- **Traceability:** F7.4, C-3, F7.3 C-2.

---

## Appendix · Traceability summary

- **Invariants:** INV-1 (every OT classified to a day_type) → F7.2-001/006/007. INV-2 (hours-only, multiplier reference) → F7.1-001/005, F7.4-020/040+. INV-3 (E11 routing, self-block) → F7.3-001/004/010. INV-4 (auto-detected candidate doesn't count until approved) → F7.2-009/010/030, F7.3-002. INV-5 (below min_minutes not counted) → F7.2-004/031, F7.4-048.
- **F7.1:** OR-1 (001/002/020), OR-3 (001/005/F7.4-020), OR-4 (001), OR-5 (010/011/012/F7.2-006/008), OR-6 (004/005); C-1 (F7.2-005), C-2 (F7.2-008/F7.4-046), C-3 (006), C-4 (010/011/013).
- **F7.2:** OC-1 (001/020), OC-2 (009/030/031/032), OC-3 (001/006/007), OC-4 (004/031/F7.4-048), OC-5 (003/005/013), OC-6 (002), OC-7 (009/010/020), OC-8 (011); C-1 (005), C-2 (013), C-5 (008).
- **F7.3:** OA-1 (001/004/005/011), OA-2 (F7.2-009), OA-3 (011), OA-4 (010/020/F7.4-046), OA-5 (002/010-reject/F7.2-010), OA-6 (001/002/003/010/043), OA-7 (006/007); C-1 (002), C-2 (051), C-3 (041), C-4 (021), C-5 (020).
- **F7.4:** OR-1 (001/003/010/011/024), OR-2 (020/022/040+), OR-3 (020/040+), OR-4 (001/021), OR-5 (023), OR-6 (025), OR-7 (004/047); C-1 (002), C-2 (026), C-3 (049), C-4 (028), C-5 (027).
