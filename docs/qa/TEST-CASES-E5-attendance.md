# Test Cases — E5 · Attendance (Manual QA)

> **Epic:** E5 Attendance · **Status:** Draft v1 · **Doc type:** Manual test-case catalog
> **Scope source:** `docs/epics/E5-attendance/FEATURE.md` + all PRDs in `docs/epics/E5-attendance/prds/` + `docs/api/CONVENTIONS.md`.
> **Date basis:** all relative dates resolved to absolute (project "today" = **2026-06-17**, TZ **Asia/Jakarta**).

---

## 1. Scope

Manual (human-driven) test cases for E5 Attendance: GPS clock in/out with a mandatory mobile clock-in photo, geofence evaluation (allowed-but-flagged when outside), shift-aware lateness/auto-close evaluation, exceptions-only shift-leader verification, attendance corrections (incl. `NEW_ENTRY` + payability) routed through the E11 approval engine, HR/SL manual (on-behalf) attendance, late-roster auto-reconcile, and the records/dashboard read surfaces.

**Features covered:**

| ID | Feature | Primary platform(s) | Primary POV(s) |
|----|---------|---------------------|----------------|
| F5.1 | Clock In/Out (GPS geofence + mobile photo) | Mobile (agent), Web (HR/SL on-behalf, internal self) | Agent, HR, SL |
| F5.2 | Attendance Evaluation & Auto-Close | System (results visible Web/Mobile) | System, all |
| F5.3 | Shift-Leader Verification (exceptions only) | Web, Mobile | Shift Leader, HR, Super Admin |
| F5.4 | Attendance Corrections (E11-routed, NEW_ENTRY, payable) | Web, Mobile | Agent, SL, HR, Super Admin |
| F5.5 | Attendance Records & Dashboard | Web, Mobile | Agent, SL, HR, Super Admin |
| F5.6 | Manual Attendance Entry | Web only | HR, Super Admin, Shift Leader |
| F5.7 | Attendance Auto-Reconcile (late-roster linking) | System (results Web/Mobile) | System; HR/SL trigger; Agent observes |

**Roles (POV):** Super Admin · HR/Placement Admin · Shift Leader (exactly 1 per company, derived scope) · Agent.

**Conventions used in expected results (CONVENTIONS.md):**
- `401 UNAUTHENTICATED` (missing/expired token) · `403 FORBIDDEN` / `OUT_OF_SCOPE` (valid token, lacks permission / cross-company) · `400 INVALID_REQUEST` (syntactic) · `409` (invariant/state conflict, e.g. `ALREADY_CLOCKED_IN`, `ATTENDANCE_ALREADY_EXISTS`) · `422` (semantic business-rule failure, e.g. `PHOTO_REQUIRED`, `NO_ACTIVE_PLACEMENT`, `OUTSIDE_CORRECTION_WINDOW`).
- Cursor pagination only; `403` no-permission renders `comp/EmptyNoPermission`; `401` mid-session renders `comp/EmptySessionExpired`.
- Idempotency-Key required on critical creates/actions (manual create, bulk).

---

## 2. Coverage matrix (feature × platform × role)

Legend: ✅ in-scope cases authored · — not applicable.

| Feature | Web · Super Admin | Web · HR | Web · Shift Leader | Web · Agent | Mobile · Shift Leader | Mobile · Agent |
|---------|:--:|:--:|:--:|:--:|:--:|:--:|
| F5.1 Clock In/Out | ✅ (self / oversight) | ✅ (on-behalf, internal self) | — | — | ✅ (own clock) | ✅ |
| F5.2 Evaluation/Auto-Close | ✅ (observe) | ✅ (observe) | ✅ (observe) | — | ✅ (observe) | ✅ (observe) |
| F5.3 Verification | ✅ (all, escalations) | ✅ (all, escalations) | ✅ (own company) | — | ✅ (own company) | — |
| F5.4 Corrections | ✅ (approve, HR-exempt) | ✅ (approve, file, HR-exempt) | ✅ (approve in Inbox) | ✅ (read-only `/corrections`) | ✅ (approve) | ✅ (file own) |
| F5.5 Records/Dashboard | ✅ (all + export) | ✅ (all + export) | ✅ (own company) | — | ✅ (own company) | ✅ (own riwayat) |
| F5.6 Manual Entry | ✅ (any company) | ✅ (any company) | ✅ (own company) | — | — | — |
| F5.7 Auto-Reconcile | ✅ (trigger via roster) | ✅ (trigger via roster) | ✅ (trigger via roster) | — | — | ✅ (observe status) |

---

## F5.1 — Clock In/Out (GPS geofence + mandatory mobile photo)

> Rules CI-1..CI-10; cases C-1..C-11. Mobile clock-in photo mandatory (`422 PHOTO_REQUIRED`); web exempt; clock-out photo optional everywhere; out-of-geofence allowed+flagged; one open record (CI-5); unscheduled flagged (CI-4); shift-window fixing (CI-9).

### Mobile · Agent POV

#### TC-E5-F5.1-001 · Clock in inside geofence with photo (happy path)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify a scheduled agent can clock in inside the site geofence with a captured photo.
- **Preconditions:** Agent "Budi" logged in on mobile; active placement at "Plaza Senayan" (site geofence 100m); has a "Parking Night" shift today (2026-06-17); device GPS on and physically within 100m of site center.
- **Steps:**
  1. Open the mobile clock-in screen.
  2. Tap **Clock In**; capture the live photo when prompted.
  3. Allow the photo to upload (`POST /attendance:photo-upload` → `SWP-FILE-*`).
  4. Confirm/submit the clock-in.
- **Expected result / Acceptance criteria:** Attendance record created with `check_in_at` (server time, Asia/Jakarta), `lat_in`/`lng_in` set, `in_geofence_in=true`, `photo_in_id` = uploaded file, `schedule_id` linked to today's shift, `placement_id` set, default attendance code applied. Success toast; clock-in button switches to **Clock Out** state.
- **Traceability:** F5.1, CI-1, CI-2, CI-7, CI-10, US (Background AC), INV-1, INV-2.

#### TC-E5-F5.1-002 · Mobile clock-in WITHOUT photo is rejected (422 PHOTO_REQUIRED)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Negative
- **Objective:** Verify mobile clock-in is rejected when no photo was captured/uploaded.
- **Preconditions:** Agent on mobile, inside geofence, scheduled shift today; no photo captured (`photo_id` absent).
- **Steps:**
  1. Attempt to invoke clock-in bypassing/skipping the photo step (e.g. via the API with `platform=MOBILE` and no `photo_id`).
- **Expected result:** Server responds `422 PHOTO_REQUIRED`; **no** attendance record is created (verify via records list). UI keeps the agent on the capture step.
- **Traceability:** F5.1, CI-10, C-7, AC "Mobile clock in without a photo is rejected".

#### TC-E5-F5.1-003 · platform defaults to MOBILE (fail-safe) — missing platform enforces photo
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify that when `platform` is unset, the server treats it as MOBILE and enforces the photo rule.
- **Preconditions:** Clock-in request submitted with no `platform` field and no `photo_id`.
- **Steps:**
  1. Submit clock-in omitting `platform` and `photo_id`.
- **Expected result:** `422 PHOTO_REQUIRED` (fails safe to MOBILE). No record created.
- **Traceability:** F5.1, CI-10 ("`platform` defaults to MOBILE (fails safe)").

#### TC-E5-F5.1-004 · Clock in OUTSIDE geofence — allowed but flagged (not blocked)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify an out-of-geofence clock-in is recorded and flagged for verification, never blocked.
- **Preconditions:** Agent ~500m from site center; photo captured; scheduled shift today.
- **Steps:**
  1. Clock in with photo from outside the 100m radius.
- **Expected result:** Record created with `in_geofence_in=false`; record is routed to `Pending` verification (flagged exception, F5.3); **no** error returned to the agent. Note: `OUT_OF_GEOFENCE` (422) is reserved for explicit geofence-enforced flows; per CI-3 the clock-in itself succeeds and is flagged.
- **Traceability:** F5.1, CI-3, INV-3, AC "Clock in outside the geofence is allowed but flagged".

#### TC-E5-F5.1-005 · Unscheduled clock-in (no shift today) — recorded with schedule_id null, flagged UNSCHEDULED
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify clock-in with no schedule is allowed and flagged unscheduled.
- **Preconditions:** Agent has an active placement but **no** schedule entry for today; photo captured.
- **Steps:**
  1. Clock in with photo.
- **Expected result:** Record created with `schedule_id=null`, flag `UNSCHEDULED`, `verification_status=Pending`; no lateness computed (F5.2 EV-6). Eligible for later auto-reconcile (F5.7).
- **Traceability:** F5.1, CI-4, INV-1, EV-6, AC "Unscheduled clock-in is flagged".

#### TC-E5-F5.1-006 · Second open clock-in blocked (one open record, CI-5)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Negative
- **Objective:** Verify a second clock-in is blocked while one record is still open.
- **Preconditions:** Agent already clocked in today and has NOT clocked out.
- **Steps:**
  1. Attempt to clock in again (capture photo, submit).
- **Expected result:** Blocked with `409 ALREADY_CLOCKED_IN`; no new record. UI shows the open record and offers **Clock Out** instead.
- **Traceability:** F5.1, CI-5, AC "Cannot clock in twice".

#### TC-E5-F5.1-007 · Clock out closes the open record (photo optional)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify clock-out captures time/location and closes the record without requiring a photo.
- **Preconditions:** Agent currently clocked in.
- **Steps:**
  1. Tap **Clock Out** (skip the optional photo).
  2. Submit.
- **Expected result:** `check_out_at`, `lat_out`/`lng_out`, `in_geofence_out` set; record closed; `photo_out_id=null` is accepted. Agent may now clock in again for a new shift.
- **Traceability:** F5.1, CI-6, CI-10, C-11, AC "Clock out".

#### TC-E5-F5.1-008 · Clock out WITH optional photo accepted (photo_out_id stored)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Happy
- **Objective:** Verify an optional clock-out photo is accepted and stored.
- **Preconditions:** Agent clocked in; captures a clock-out photo.
- **Steps:**
  1. Capture + upload a clock-out photo; submit clock-out.
- **Expected result:** Record closed with `photo_out_id=SWP-FILE-*`; never required.
- **Traceability:** F5.1, CI-6, CI-10, C-10.

#### TC-E5-F5.1-009 · Clock out without an open clock-in — blocked, prompts correction
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Negative
- **Objective:** Verify clock-out is blocked when there is no open record.
- **Preconditions:** Agent has no open (clocked-in) record today.
- **Steps:**
  1. Attempt to clock out.
- **Expected result:** Blocked / no record closed; UI prompts to file a correction (F5.4 — e.g. a `check_in`/`new_entry`).
- **Traceability:** F5.1, C-3, F5.4.

#### TC-E5-F5.1-010 · GPS unavailable / location services off
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Error
- **Objective:** Verify the GPS-off path prompts to enable location and, if still unavailable, records location-missing + flag.
- **Preconditions:** Device location services disabled; photo capture available.
- **Steps:**
  1. Tap Clock In; observe the GPS prompt.
  2. Decline/leave GPS off; capture photo; proceed.
- **Expected result:** Agent first prompted to enable GPS. If location remains unavailable, clock-in is recorded with location missing and flagged for verification (record not blocked on GPS alone). Photo rule still applies.
- **Traceability:** F5.1, CI-1, AC "GPS unavailable".

#### TC-E5-F5.1-011 · Camera permission denied / no camera — clock-in blocked client-side
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Error
- **Objective:** Verify the client blocks clock-in and prompts for camera access when no photo can be taken.
- **Preconditions:** Camera permission denied (or device has no camera).
- **Steps:**
  1. Tap Clock In; observe the permission prompt.
  2. Deny camera; attempt to proceed.
- **Expected result:** Client blocks clock-in (no `photo_id` can be obtained), prompts the agent to grant camera access. If a request is still forced to the server without `photo_id`, server returns `422 PHOTO_REQUIRED`.
- **Traceability:** F5.1, CI-10, C-7.

#### TC-E5-F5.1-012 · Photo upload fails (network) — retry, clock-in cannot proceed
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Error
- **Objective:** Verify a failed photo upload blocks clock-in until a `photo_id` is obtained.
- **Preconditions:** Connectivity drops during `POST /attendance:photo-upload`.
- **Steps:**
  1. Capture photo; let upload fail.
  2. Observe retry affordance; clock-in stays blocked.
- **Expected result:** Upload error surfaced with retry; clock-in cannot proceed without `photo_id` (online-only v1, no offline photo queue).
- **Traceability:** F5.1, CI-10, C-8.

#### TC-E5-F5.1-013 · Stale photo_id (>24h orphan) rejected at clock-in
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify a photo uploaded >24h earlier and never attached is rejected.
- **Preconditions:** A `photo_id` exists from an upload older than 24h (expired orphan).
- **Steps:**
  1. Attempt clock-in passing the stale `photo_id`.
- **Expected result:** `422 PHOTO_REQUIRED` (orphan expired); client must re-capture + re-upload.
- **Traceability:** F5.1, CI-10, C-9, §6 (orphan expires after 24h).

#### TC-E5-F5.1-014 · Photo upload rejects oversized/wrong type file
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Negative
- **Objective:** Verify the upload endpoint enforces ≤10MB and image/jpeg|png.
- **Preconditions:** Attempt to upload a >10MB file or a non-image type.
- **Steps:**
  1. `POST /attendance:photo-upload` with an 11MB file; then with a `.pdf`.
- **Expected result:** Upload rejected (`400 INVALID_REQUEST` / size or type error); no `photo_id` issued.
- **Traceability:** F5.1, §6 (multipart ≤10MB, image/jpeg|png).

#### TC-E5-F5.1-015 · Clock-in well before shift start — allowed, earliness noted
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify early clock-in is allowed and lateness deferred to F5.2.
- **Preconditions:** Shift starts 07:00; agent clocks in 05:30 with photo, in geofence.
- **Steps:**
  1. Clock in at 05:30.
- **Expected result:** Record created; not flagged late; earliness handled in evaluation (F5.2). `shift_start_at` fixed at clock-in (CI-9).
- **Traceability:** F5.1, C-1, CI-9.

#### TC-E5-F5.1-016 · Cross-midnight clock-out next day — stays tied to shift start date
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify a night-shift clock-out on the next calendar day stays attributed to the shift's start date.
- **Preconditions:** Night shift 23:00 (2026-06-17) → 07:00 (2026-06-18); agent clocked in 22:55.
- **Steps:**
  1. Clock out 07:05 on 2026-06-18.
- **Expected result:** Record remains tied to start date 2026-06-17; `check_out_at` recorded on 2026-06-18; record closes normally.
- **Traceability:** F5.1, C-2, INV-6, AR-8 (display in F5.5).

#### TC-E5-F5.1-017 · Site has no geofence set — geofence check skipped + flagged, record saved
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify a site with no lat/lng skips the geofence check but still records.
- **Preconditions:** Placement site has no `lat`/`lng` configured (mobile/cruise placement).
- **Steps:**
  1. Clock in with photo.
- **Expected result:** Geofence check skipped (`in_geofence_in=null`) and flagged; record still saved. Default radius 100m if a radius alone existed.
- **Traceability:** F5.1, CI-2, C-6, INV-2 (E2 F2.6 ST-8).

#### TC-E5-F5.1-018 · REMOTE (WFH) mode skips geofence but still requires mobile photo
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify `mode=REMOTE` clock-in skips geofence yet the mobile photo rule still applies.
- **Preconditions:** Agent allowed to clock in REMOTE for a WFH day.
- **Steps:**
  1. Clock in with `mode=REMOTE`, photo captured.
- **Expected result:** `in_geofence_in=null` (geofence skipped, recorded as remote, not blocked); `photo_in_id` still required and stored.
- **Traceability:** F5.1 FEATURE §1/§4 (mode), CI-10 (photo rule independent of geofence/mode).

#### TC-E5-F5.1-019 · CI-9 — shift_end_at follows master edits until clock-out then fixes
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify the effective shift window behavior across a master edit.
- **Preconditions:** Agent clocked in (shift 07:00–15:00); planner edits the linked schedule end to 16:00 before the agent clocks out.
- **Steps:**
  1. Observe record's effective `shift_end_at` after the master edit (still open) → should be 16:00.
  2. Clock out; observe `shift_end_at` fixes to current end at clock-out.
- **Expected result:** `shift_start_at` stays fixed at clock-in; `shift_end_at` tracks the edit (16:00) while open, then fixes at clock-out.
- **Traceability:** F5.1, CI-9, INV-6.

### Web · HR / Internal-staff self POV

#### TC-E5-F5.1-020 · Web clock-in WITHOUT photo is allowed (platform=WEB)
- [ ] **Platform:** Web · **POV:** HR (or internal staff self) · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify web clock-in does not require a photo and creates a record with `photo_in_id=null`.
- **Preconditions:** HR admin (or internal staff) on the web console; `platform=WEB`.
- **Steps:**
  1. Clock in on the web console without attaching a photo.
- **Expected result:** Attendance record created; `photo_in_id=null`; no `PHOTO_REQUIRED` error.
- **Traceability:** F5.1, CI-10, C-11, AC "Web clock in does not require a photo".

#### TC-E5-F5.1-021 · Web clock-in with optional photo attached
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Happy
- **Objective:** Verify a photo may still be optionally attached on web.
- **Preconditions:** `platform=WEB`; HR attaches a photo.
- **Steps:**
  1. Clock in on web, optionally attaching a photo file.
- **Expected result:** Record created; `photo_in_id` populated if provided; not required either way.
- **Traceability:** F5.1, CI-10, C-11.

#### TC-E5-F5.1-022 · Internal staff (SWP HQ) clock-in with no geofence
- [ ] **Platform:** Web · **POV:** Agent/Internal self · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify internal staff at HQ can clock in; attendance is presence/discipline only (not pay-gated).
- **Preconditions:** Internal-staff assignment at SWP HQ (no geofence or HQ geofence as configured).
- **Steps:**
  1. Clock in on web (`platform=WEB`).
- **Expected result:** Record created; no photo required; `is_payable` mechanic is FIELD-only — internal record is presence-only.
- **Traceability:** F5.1, FEATURE §1 (FIELD vs INTERNAL), CI-10.

### Web/Mobile · Shift Leader & RBAC

#### TC-E5-F5.1-023 · Shift leader sees near-live "who is clocked in" (own company)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P2 · **Type:** Happy
- **Objective:** Verify the leader sees a near-live view of clocked-in agents for their company.
- **Preconditions:** SL of "Plaza Senayan"; several agents clocked in.
- **Steps:**
  1. Open the live roster / records view (F5.5).
- **Expected result:** Currently clocked-in agents for the led company appear near-live; cross-company agents do not.
- **Traceability:** F5.1, CI-8, F5.5 AR-9.

#### TC-E5-F5.1-024 · Unauthenticated / expired token on clock-in
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** RBAC
- **Objective:** Verify an expired session blocks clocking and prompts re-auth.
- **Preconditions:** Token expired mid-session.
- **Steps:**
  1. Attempt to clock in.
- **Expected result:** `401 UNAUTHENTICATED`; client renders `comp/EmptySessionExpired` + re-auth flow; no record.
- **Traceability:** F5.1, CONVENTIONS §401/403.

#### TC-E5-F5.1-025 · Agent cannot clock in on behalf of another agent
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** RBAC
- **Objective:** Verify an agent can only clock themselves in.
- **Preconditions:** Agent attempts to submit a clock-in targeting another `employee_id`.
- **Steps:**
  1. Submit a clock-in for a different employee.
- **Expected result:** `403 FORBIDDEN`; no record created.
- **Traceability:** F5.1, RBAC defense-in-depth.

---

## F5.2 — Attendance Evaluation & Auto-Close

> Rules EV-1..EV-8; cases C-1..C-6. System logic; results observable on Web/Mobile. Grace = 15 min.

### System (results observed by all POVs)

#### TC-E5-F5.2-001 · On-time clean clock-in auto-approves
- [ ] **Platform:** System (Mobile/Web observe) · **POV:** Agent/SL observe · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify an on-time, in-geofence, completed record becomes Present + AutoApproved.
- **Preconditions:** Shift 07:00 (grace 15 min); agent clocks in 07:05 inside geofence and clocks out at shift end.
- **Steps:**
  1. Perform clock-in 07:05 and clock-out at 15:00.
- **Expected result:** `status=Present`, `is_late=false`, `verification_status=AutoApproved`; not in the SL queue.
- **Traceability:** F5.2, EV-1, EV-5, INV-3, AC "On-time clean clock-in auto-approves".

#### TC-E5-F5.2-002 · Late clock-in flagged (late_minutes computed)
- [ ] **Platform:** System · **POV:** Agent/SL observe · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify lateness past grace sets is_late, late_minutes, status=Late, Pending.
- **Preconditions:** Shift 07:00, grace 15 min; agent clocks in 07:30.
- **Steps:**
  1. Clock in at 07:30.
- **Expected result:** `is_late=true`, `late_minutes=30`, `status=Late`, `verification_status=Pending`.
- **Traceability:** F5.2, EV-1, EV-2, EV-5, AC "Late clock-in is flagged".

#### TC-E5-F5.2-003 · Grace boundary — late only if strictly after start+grace
- [ ] **Platform:** System · **POV:** observe · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify boundary handling at exactly start+grace.
- **Preconditions:** Shift 07:00, grace 15 min; clock-in exactly 07:15:00.
- **Steps:**
  1. Clock in at exactly 07:15:00.
  2. Repeat at 07:15:01.
- **Expected result:** 07:15:00 → not late (boundary inclusive of on-time; late only if strictly after). 07:15:01 → late (`late_minutes` ≈ 1, per server rounding).
- **Traceability:** F5.2, C-1, EV-1.

#### TC-E5-F5.2-004 · Out-of-geofence clean time still needs verification
- [ ] **Platform:** System · **POV:** observe · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify on-time but out-of-geofence routes to Pending.
- **Preconditions:** On-time clock-in with `in_geofence_in=false`.
- **Steps:**
  1. Observe the routing result.
- **Expected result:** `status=Present` but `verification_status=Pending` (flagged by geofence fact).
- **Traceability:** F5.2, EV-5, INV-3, AC "Out-of-geofence clean time still needs verification".

#### TC-E5-F5.2-005 · Auto-clock-out for forgotten clock-out (INV-4)
- [ ] **Platform:** System · **POV:** observe · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify the shift-end job auto-closes an open record.
- **Preconditions:** Agent clocked in, never clocked out; shift ends 15:00.
- **Steps:**
  1. Let the shift-end job run after 15:00.
- **Expected result:** `check_out_at=15:00`, `auto_closed=true`, `status=Incomplete`, `verification_status=Pending`.
- **Traceability:** F5.2, EV-3, INV-4, AC "Auto-clock-out for a forgotten clock-out".

#### TC-E5-F5.2-006 · No-show marked Absent (INV-5)
- [ ] **Platform:** System · **POV:** observe · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify a scheduled shift with no clock-in by shift end is Absent + Pending, check_in_at null.
- **Preconditions:** Agent had a scheduled shift today; never clocked in.
- **Steps:**
  1. Let the shift-end (+buffer) job run.
- **Expected result:** `status=Absent`, `check_in_at=null`, `verification_status=Pending`; resolvable via a `check_in` correction (F5.4).
- **Traceability:** F5.2, EV-4, INV-5, AC "No-show is marked absent".

#### TC-E5-F5.2-007 · Re-evaluation after a correction re-routes
- [ ] **Platform:** System · **POV:** observe · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify applying a correction re-runs evaluation and may auto-approve.
- **Preconditions:** A Late record; a `check_in` correction to an on-time value is approved/applied.
- **Steps:**
  1. Apply the correction.
- **Expected result:** `is_late=false` recomputed; record re-routes (may become AutoApproved if no other flags).
- **Traceability:** F5.2, EV-7, CR-9, AC "Re-evaluation after a correction".

#### TC-E5-F5.2-008 · Early clock-out flagged Incomplete (>15 min early)
- [ ] **Platform:** System · **POV:** observe · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify clocking out more than 15 min before shift end flags the record.
- **Preconditions:** Shift ends 15:00; agent clocks out 14:30.
- **Steps:**
  1. Clock out at 14:30.
- **Expected result:** Record flagged early/Incomplete → Pending (per decision: early clock-out flagged if >15 min early).
- **Traceability:** F5.2, C-4, FEATURE §7 (early clock-out >15 min flagged).

#### TC-E5-F5.2-009 · Cross-midnight auto-close uses next-day shift end
- [ ] **Platform:** System · **POV:** observe · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify auto-close for a night shift uses the end time on the following day.
- **Preconditions:** Night shift 23:00 (2026-06-17) → 07:00 (2026-06-18); agent clocked in, never out.
- **Steps:**
  1. Let the shift-end job run after 07:00 on 2026-06-18.
- **Expected result:** Auto-close at 07:00 2026-06-18; `auto_closed=true`, `status=Incomplete`, tied to 2026-06-17.
- **Traceability:** F5.2, C-2, EV-3.

#### TC-E5-F5.2-010 · Unscheduled clock-in skips lateness, always Pending
- [ ] **Platform:** System · **POV:** observe · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify EV-6 for records with no schedule_id.
- **Preconditions:** Unscheduled clock-in (schedule_id null).
- **Steps:**
  1. Observe evaluation.
- **Expected result:** No lateness computed; `verification_status=Pending` (flagged).
- **Traceability:** F5.2, EV-6, C-3.

#### TC-E5-F5.2-011 · Catch-up job evaluates all overdue shifts after downtime (EV-8)
- [ ] **Platform:** System · **POV:** observe · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify a delayed/missed job catches up by due time, not "now only".
- **Preconditions:** Job runner down across several shift-end times; multiple open/no-show records overdue.
- **Steps:**
  1. Bring the job runner back; trigger the next run.
- **Expected result:** All overdue records evaluated (auto-closed / marked Absent) by their due times; all derived changes audited as `system`.
- **Traceability:** F5.2, EV-8, C-5.

#### TC-E5-F5.2-012 · Approved leave suppresses Absent → On Leave (C-6)
- [ ] **Platform:** System · **POV:** observe · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify an approved leave for a scheduled shift prevents an Absent record.
- **Preconditions:** Agent has an approved E6 leave covering today, and a scheduled shift exists.
- **Steps:**
  1. Let the absence sweep run.
- **Expected result:** Status is `On Leave` (not Absent); leave precedence honored.
- **Traceability:** F5.2, C-6, INV-5 (leave suppresses Absent), E6 dependency.

---

## F5.3 — Shift-Leader Verification (exceptions only)

> Rules VF-1..VF-8; cases C-1..C-5. Queue holds only Pending exceptions; SL scoped to own company; HR/Super see all; reject prompts a correction.

### Web · Shift Leader POV

#### TC-E5-F5.3-001 · Only exceptions appear in the queue
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify auto-approved clean records do not appear; only Pending exceptions do.
- **Preconditions:** SL of "Plaza Senayan"; Budi has a clean AutoApproved record; Citra has a Late (Pending) record.
- **Steps:**
  1. Open the verification queue.
- **Expected result:** Citra's late record shown with its exception reason; Budi's record absent.
- **Traceability:** F5.3, VF-1, INV-3, AC "Only exceptions appear in the queue".

#### TC-E5-F5.3-002 · Queue item shows exception reason, times, GPS/geofence, schedule context
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify each item presents full context for a decision.
- **Preconditions:** A Pending out-of-geofence + late record.
- **Steps:**
  1. Open the record detail in the queue.
- **Expected result:** Shows exception reason(s) (Late, OUTSIDE_GEOFENCE), check-in/out times, GPS point + geofence result, scheduled shift; clock-in photo (if mobile) visible.
- **Traceability:** F5.3, VF-3.

#### TC-E5-F5.3-003 · Approve a late record → Verified
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify approval sets Verified + verified_by/at and notifies the agent.
- **Preconditions:** Citra's late Pending record in the SL's queue.
- **Steps:**
  1. Open and **Approve** the record.
- **Expected result:** `verification_status=Verified`, `verified_by=SL`, `verified_at` set; agent notified; record counts for OT (E7) and billing (E10).
- **Traceability:** F5.3, VF-4, VF-8, AC "Approve a late record".

#### TC-E5-F5.3-004 · Reject an out-of-geofence record requires a reason → prompts correction
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Negative
- **Objective:** Verify reject requires a reason and links to a correction.
- **Preconditions:** Pending out-of-geofence record.
- **Steps:**
  1. **Reject** without a reason → expect validation.
  2. Reject with a reason.
- **Expected result:** Reject without reason blocked (`400`/field error). With reason → `Rejected`; a correction (F5.4) is prompted/linked; agent notified.
- **Traceability:** F5.3, VF-5, VF-8, AC "Reject an out-of-geofence record".

#### TC-E5-F5.3-005 · Bulk approve multiple slightly-late records
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify bulk approve sets all selected to Verified.
- **Preconditions:** Five slightly-late Pending records in the queue.
- **Steps:**
  1. Select all five; **Bulk Approve** (with Idempotency-Key).
- **Expected result:** All five → `Verified`; bulk response returns success per item; partial-failure array rendered if any failed.
- **Traceability:** F5.3, VF-6, CONVENTIONS §bulk/idempotency.

#### TC-E5-F5.3-006 · Confirm an Absent record (leader confirms absent)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify an Absent record can be confirmed in the queue or routed to correction if the agent worked.
- **Preconditions:** An Absent Pending record.
- **Steps:**
  1. Open the Absent record; confirm absent (Verified) OR trigger a correction.
- **Expected result:** Confirming → Verified (absent stands). If the agent worked, a `check_in` correction is filed (F5.4) which re-evaluates Absent → Present/Late.
- **Traceability:** F5.3, C-3, CR-9.

#### TC-E5-F5.3-007 · Scope enforced — leader does not see other companies' records
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Verify the queue is scoped to the led company.
- **Preconditions:** A Pending record exists at a company the SL does not lead.
- **Steps:**
  1. Open the queue; attempt to access the other company's record by direct URL/ID.
- **Expected result:** Out-of-company record not in queue; direct access → `403 OUT_OF_SCOPE` (`comp/EmptyNoPermission`).
- **Traceability:** F5.3, VF-2, AC "Scope is enforced", CONVENTIONS OUT_OF_SCOPE=403.

#### TC-E5-F5.3-008 · Leader's own exception escalates to HR (no self-verify)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** Verify a leader cannot verify their own attendance exception.
- **Preconditions:** The SL has their own Pending exception record.
- **Steps:**
  1. Look for the SL's own record in their queue; attempt to approve it.
- **Expected result:** Self-verification disallowed/escalated to HR (separation of duties); the SL's own record routes to the HR queue.
- **Traceability:** F5.3, C-5, FEATURE §7 ("Leaders' own exceptions → escalate to HR").

#### TC-E5-F5.3-009 · Already-verified record cannot be reopened from the queue
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P2 · **Type:** Negative
- **Objective:** Verify a Verified record is not re-decidable from the queue.
- **Preconditions:** A previously Verified record.
- **Steps:**
  1. Attempt to re-open/re-decide it from the queue.
- **Expected result:** Not actionable from the queue; only a correction (F5.4) or HR override changes it.
- **Traceability:** F5.3, C-1.

### Mobile · Shift Leader POV

#### TC-E5-F5.3-010 · Verify exception from mobile (approve)
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify the SL can approve exceptions on mobile.
- **Preconditions:** SL on mobile; Pending exception in own company.
- **Steps:**
  1. Open the mobile verification queue; approve a late record.
- **Expected result:** `Verified` with verified_by/at; agent notified; parity with web.
- **Traceability:** F5.3, VF-4, FEATURE §6 (Web/mobile SL).

### Web · HR / Super Admin POV

#### TC-E5-F5.3-011 · HR sees cross-company queue + handles escalations (no leader)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify HR sees all companies and receives no-leader escalations.
- **Preconditions:** "Mall X" has no shift leader and has Pending records.
- **Steps:**
  1. Open the HR verification queue.
- **Expected result:** All companies visible; "Mall X" Pending items appear in the HR queue (VF-7); HR can approve/reject.
- **Traceability:** F5.3, VF-2, VF-7, AC "Escalate when no leader".

#### TC-E5-F5.3-012 · Super admin oversight across all companies
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** RBAC
- **Objective:** Verify super admin has full cross-company verification visibility.
- **Preconditions:** Super admin logged in.
- **Steps:**
  1. Open the verification queue.
- **Expected result:** All Pending records across all companies visible and actionable.
- **Traceability:** F5.3, VF-2.

#### TC-E5-F5.3-013 · Migrated historical record never floods the live queue
- [ ] **Platform:** Web · **POV:** HR/SL · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify imported records come in Verified and don't appear as Pending.
- **Preconditions:** E9-migrated attendance records present.
- **Steps:**
  1. Open the queue.
- **Expected result:** Migrated records are `Verified` (G-5); none flood the Pending queue.
- **Traceability:** F5.3, C-4.

#### TC-E5-F5.3-014 · Stale pending surfaced/aged in queue + reporting
- [ ] **Platform:** Web · **POV:** SL/HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify long-pending records are aged/surfaced.
- **Preconditions:** A Pending record older than several days.
- **Steps:**
  1. Sort/observe age in the queue.
- **Expected result:** Stale Pending items are visibly aged and surfaced in reporting (optional reminder).
- **Traceability:** F5.3, C-2.

#### TC-E5-F5.3-015 · Empty verification queue state
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P2 · **Type:** Empty
- **Objective:** Verify the empty state when no exceptions are pending.
- **Preconditions:** No Pending records for the led company.
- **Steps:**
  1. Open the queue.
- **Expected result:** Empty state shown (no dead-flow); no errors.
- **Traceability:** F5.3, VF-1.

---

## F5.4 — Attendance Corrections (E11-routed, NEW_ENTRY, payability)

> Rules CR-1..CR-14; cases C-1..C-8. Corrections route through the E11 approval engine; statuses PENDING→APPROVED→APPLIED / REJECTED / CANCELLED; NEW_ENTRY for no-record days; payability on apply.

### Mobile · Agent POV (Pengajuan → Koreksi)

#### TC-E5-F5.4-001 · File a missed clock-out correction (check_out) and it opens an E11 instance
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify filing a check_out correction creates a PENDING correction with an approval instance.
- **Preconditions:** Agent Budi; 2026-06-10 record auto-closed (forgot clock-out).
- **Steps:**
  1. On the Pengajuan → Koreksi tab, search and select the 2026-06-10 record.
  2. File a `check_out` correction, proposed time 15:10, with a required reason; submit.
- **Expected result:** Correction `status=PENDING` with `approval_instance_id`; an E11 `approval_instance` opened; tracker shows it pending.
- **Traceability:** F5.4, CR-1, CR-2, CR-3, US, AC "File and approve a missed clock-out correction".

#### TC-E5-F5.4-002 · Correction requires a reason
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Negative
- **Objective:** Verify a correction without a reason is rejected.
- **Preconditions:** Filing any correction type.
- **Steps:**
  1. Submit a correction with an empty reason.
- **Expected result:** `400 INVALID_REQUEST` with field `reason`; not filed.
- **Traceability:** F5.4, CR-1.

#### TC-E5-F5.4-003 · Cancel a pending correction before a decision (CANCELLED)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify the requester can withdraw a pending correction.
- **Preconditions:** A PENDING correction filed by the agent, no decision yet.
- **Steps:**
  1. Open the tracker; cancel the correction.
- **Expected result:** `status=CANCELLED`; the underlying attendance record is unchanged.
- **Traceability:** F5.4, CR-3.

#### TC-E5-F5.4-004 · NEW_ENTRY for a no-shift day; payable pending then flagged true
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify NEW_ENTRY creates a record on approval with is_payable=null, then HR flags payable.
- **Preconditions:** Agent worked 2026-06-10; no record and no shift that day; active placement covers the date.
- **Steps:**
  1. Search own attendance; find 2026-06-10 missing; file a `new_entry` with check-in 07:00 + check-out 15:00 + reason.
  2. Approver approves in the Inbox.
  3. HR flags the day payable via `POST /attendance/{id}:set-payable`.
- **Expected result:** New attendance row created for 2026-06-10, `schedule_id=null`, flags `UNSCHEDULED`+`CORRECTED`, `is_payable=null` (pending). After `:set-payable`, `is_payable=true` and payroll counts the day.
- **Traceability:** F5.4, CR-10, CR-12, CR-13, AC "Search and create a record for a no-shift day".

#### TC-E5-F5.4-005 · NEW_ENTRY proposed_check_in_at must fall on work_date
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Negative
- **Objective:** Verify the proposed check-in must be on work_date and at least a check-in is required.
- **Preconditions:** Filing a new_entry for work_date 2026-06-10.
- **Steps:**
  1. Submit a new_entry with no `proposed_check_in_at`.
  2. Submit a new_entry whose check-in date is 2026-06-11.
- **Expected result:** Both rejected (`400 INVALID_REQUEST`): missing check-in; off-date check-in.
- **Traceability:** F5.4, CR-10.

#### TC-E5-F5.4-006 · NEW_ENTRY for a day that already has a record → 409
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Negative
- **Objective:** Verify duplicate-day prevention.
- **Preconditions:** A record already exists for 2026-06-10.
- **Steps:**
  1. File a new_entry for 2026-06-10.
- **Expected result:** `409 ATTENDANCE_ALREADY_EXISTS`; agent steered to correct the existing record instead.
- **Traceability:** F5.4, CR-11, C-6, AC "New_entry for a day that already has a record is blocked".

#### TC-E5-F5.4-007 · NEW_ENTRY with no active placement on work_date → 422
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Negative
- **Objective:** Verify NO_ACTIVE_PLACEMENT on a new_entry.
- **Preconditions:** No active placement covering work_date.
- **Steps:**
  1. File a new_entry for that date.
- **Expected result:** `422 NO_ACTIVE_PLACEMENT`.
- **Traceability:** F5.4, CR-11.

#### TC-E5-F5.4-008 · 7-day self-correction window enforced (existing record)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Negative
- **Objective:** Verify the agent cannot self-correct records older than 7 days.
- **Preconditions:** A record with `shift_date` = 2026-06-05 (older than 7 days vs today 2026-06-17).
- **Steps:**
  1. Attempt to file a correction on the 2026-06-05 record.
- **Expected result:** `422 OUTSIDE_CORRECTION_WINDOW`; agent directed to HR (HR exempt).
- **Traceability:** F5.4, CR-14, C-3, FEATURE §7 (7-day window).

#### TC-E5-F5.4-009 · 7-day window measured vs work_date for new_entry
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify the window for new_entry is measured against work_date.
- **Preconditions:** new_entry work_date = 2026-06-02 (older than 7 days).
- **Steps:**
  1. File a new_entry for 2026-06-02.
- **Expected result:** `422 OUTSIDE_CORRECTION_WINDOW` (HR exempt).
- **Traceability:** F5.4, CR-14.

#### TC-E5-F5.4-010 · Correction on an Absent record re-evaluates to Present/Late (CR-9)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify approving a check_in correction on an Absent record re-runs evaluation.
- **Preconditions:** An Absent record (check_in_at=null) for a scheduled shift starting 07:00.
- **Steps:**
  1. File a `check_in` correction with the real arrival time (e.g. 07:05); approver approves.
- **Expected result:** Record `check_in_at` populated; status recomputes Absent → Present (07:05 within grace) or Late (if after grace); `is_late`/`late_minutes` recomputed.
- **Traceability:** F5.4, CR-9, CR-4, INV-5, AC "Correction changes lateness".

#### TC-E5-F5.4-011 · Empty/zero search results on Koreksi search
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Empty
- **Objective:** Verify the search empty state when no matching attendance exists.
- **Preconditions:** Agent searches a date range with no records.
- **Steps:**
  1. Search; observe results.
- **Expected result:** Empty state; affordance to file a `new_entry` if applicable (no dead-flow).
- **Traceability:** F5.4, CR-1, US.

### Web · HR / Shift Leader POV (E11 Inbox + read-only /corrections)

#### TC-E5-F5.4-012 · Approve a correction in the E11 Inbox → APPLIED + re-evaluate
- [ ] **Platform:** Web · **POV:** HR (or SL approver) · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify final approval applies the correction and re-evaluates the record.
- **Preconditions:** A PENDING check_out correction (proposed 15:10) on an auto-closed record; approver is on the final line.
- **Steps:**
  1. Open the E11 Inbox; approve the correction.
- **Expected result:** `check_out_at=15:10`, `auto_closed` cleared, record re-evaluated/re-routed; correction `status=APPLIED`; original auto-closed values retained as snapshot; requester notified.
- **Traceability:** F5.4, CR-2, CR-3, CR-4, CR-5, CR-8.

#### TC-E5-F5.4-013 · Reject a correction in the Inbox with reason → REJECTED, record unchanged
- [ ] **Platform:** Web · **POV:** HR/SL · **Priority:** P0 · **Type:** Negative
- **Objective:** Verify rejection leaves the record unchanged and surfaces the reason.
- **Preconditions:** A PENDING correction.
- **Steps:**
  1. Reject in the Inbox with a reason.
- **Expected result:** `status=REJECTED`; requester sees the reason; attendance record unchanged.
- **Traceability:** F5.4, CR-3, AC "Reject a correction".

#### TC-E5-F5.4-014 · No self-approval — requester excluded from their own correction line (E11 INV-3)
- [ ] **Platform:** Web · **POV:** SL/HR · **Priority:** P0 · **Type:** RBAC
- **Objective:** Verify the requester cannot approve their own correction.
- **Preconditions:** The approver who filed the correction is also a member of its approval line.
- **Steps:**
  1. Open the Inbox as the requester.
- **Expected result:** The requester's own correction is not actionable by them; another line member (or super-admin fallback) decides.
- **Traceability:** F5.4, CR-2, C-8, E11 INV-3.

#### TC-E5-F5.4-015 · No approval template → super-admin fallback line (E11 INV-7)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify corrections still route when a company has no configured template.
- **Preconditions:** Company has no CORRECTION approval template.
- **Steps:**
  1. File a correction; observe routing.
- **Expected result:** Routes to the super-admin fallback line; decidable there.
- **Traceability:** F5.4, CR-2, E11 INV-7.

#### TC-E5-F5.4-016 · Approved correction on a shift day is auto-payable; :set-payable rejected
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify a shift-backed corrected day auto-sets is_payable=true and rejects manual flag.
- **Preconditions:** 2026-06-11 record had a scheduled shift; a correction on it is approved/applied.
- **Steps:**
  1. After apply, attempt `POST /attendance/{id}:set-payable`.
- **Expected result:** `is_payable=true` automatically; `:set-payable` → `422 ATTENDANCE_HAS_SHIFT_AUTO_PAYABLE`.
- **Traceability:** F5.4, CR-12, CR-13, AC "Approved correction on a shift day is auto-payable".

#### TC-E5-F5.4-017 · Payroll treats is_payable=null as non-payable
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify a no-shift applied day stays non-payable until flagged.
- **Preconditions:** A NEW_ENTRY applied day with is_payable=null.
- **Steps:**
  1. Inspect payroll context before flagging payable.
- **Expected result:** Day not counted as payable (null treated as non-payable) until `:set-payable` sets true.
- **Traceability:** F5.4, CR-12, CR-13, C-7.

#### TC-E5-F5.4-018 · HR exempt from the 7-day window
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify HR can correct records older than 7 days.
- **Preconditions:** A record older than 7 days.
- **Steps:**
  1. As HR, file/apply a correction on the old record.
- **Expected result:** Allowed (HR exempt); no OUTSIDE_CORRECTION_WINDOW.
- **Traceability:** F5.4, CR-14, C-3.

#### TC-E5-F5.4-019 · Correcting a migrated/historical record is HR-only
- [ ] **Platform:** Web · **POV:** HR vs Agent · **Priority:** P2 · **Type:** RBAC
- **Objective:** Verify only HR may correct migrated records.
- **Preconditions:** A migrated historical record.
- **Steps:**
  1. As an agent, attempt a correction → expect denial.
  2. As HR, file the correction → allowed.
- **Expected result:** Agent denied; HR allowed.
- **Traceability:** F5.4, CR-7, C-5.

#### TC-E5-F5.4-020 · Original snapshot preserved; history queryable
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify pre-correction values remain queryable after apply.
- **Preconditions:** A correction applied to a record.
- **Steps:**
  1. View the record's correction history.
- **Expected result:** `original_snapshot` retained; original pre-correction values queryable; corrections never erase history.
- **Traceability:** F5.4, CR-5, CR-6, AC "History preserved".

#### TC-E5-F5.4-021 · Multiple corrections on one record — latest applied wins, all audited
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify a record can have several corrections, latest applied value wins.
- **Preconditions:** A record with two sequentially applied corrections.
- **Steps:**
  1. Apply correction A, then correction B; inspect.
- **Expected result:** Both tracked + audited; latest applied value wins.
- **Traceability:** F5.4, CR-6, C-2.

#### TC-E5-F5.4-022 · /corrections is read-only (decisions live in the Inbox)
- [ ] **Platform:** Web · **POV:** HR/SL · **Priority:** P1 · **Type:** RBAC
- **Objective:** Verify the /corrections screen shows history only, no decide actions.
- **Preconditions:** Open `/corrections`.
- **Steps:**
  1. Look for approve/reject controls on `/corrections`.
- **Expected result:** Read-only list/history; approve/reject only available in the E11 Inbox.
- **Traceability:** F5.4, §2 non-goals, §10 decision (2026-06-15).

#### TC-E5-F5.4-023 · Downstream propagation flagged when correcting a record that fed E7/E10
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify a correction that changes worked time flags/notifies downstream (v1).
- **Preconditions:** Record already fed E7 overtime / E10 billing; a correction is applied.
- **Steps:**
  1. Apply the correction; observe downstream signaling.
- **Expected result:** v1 flags/notifies downstream; synchronous recompute deferred.
- **Traceability:** F5.4, C-4, §10 open(C-4).

### Web · Agent POV

#### TC-E5-F5.4-024 · Agent reads own correction status (read-only)
- [ ] **Platform:** Web · **POV:** Agent · **Priority:** P2 · **Type:** Happy
- **Objective:** Verify the agent tracks correction status on the Pengajuan tab (web).
- **Preconditions:** Agent filed corrections.
- **Steps:**
  1. Open the Pengajuan → Koreksi tracker on web.
- **Expected result:** Statuses (PENDING/APPROVED/APPLIED/REJECTED/CANCELLED) visible; no approve controls for own items.
- **Traceability:** F5.4, §4 platform table.

---

## F5.5 — Attendance Records & Dashboard

> Rules AR-1..AR-11; cases C-1..C-6. Scoped reads; mobile riwayat date-range + status quick-filter; billable/payable rollups; audited export; leader scope server-pinned.

### Mobile · Agent POV (Riwayat Kehadiran)

#### TC-E5-F5.5-001 · Agent views own history (self scope only)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify the agent sees only their own records.
- **Preconditions:** Agent Budi with records; other agents have records.
- **Steps:**
  1. Open "Riwayat Kehadiran".
- **Expected result:** Own records with status + corrections shown; cannot see others' attendance.
- **Traceability:** F5.5, AR-1, AR-2, AC "Agent views own history".

#### TC-E5-F5.5-002 · Status quick-filter — single-select with Semua reset (AR-11)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify tapping a status chip filters the list and Semua resets.
- **Preconditions:** Active range "Bulan ini"; records of mixed statuses.
- **Steps:**
  1. Tap the "Terlambat" chip.
  2. Tap "Semua".
- **Expected result:** List shows only Late records, "Terlambat" chip active (filled); tapping "Semua" (or re-tapping active chip) clears the filter, all statuses shown. Sends one `status` value; omitted when Semua.
- **Traceability:** F5.5, AR-11, AC "Agent filters own history by status" (frames GJI1a, l6UYy).

#### TC-E5-F5.5-003 · Date-range presets sheet (Bulan ini / 30 hari / Bulan lalu)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify range presets map to date_from/date_to and update chip counts.
- **Preconditions:** Default range = current month (June 2026).
- **Steps:**
  1. Tap the range chip; choose "30 hari terakhir".
- **Expected result:** List + summary chip counts reflect the last 30 days; counts reflect the current range (inclusive, shift-start-date basis).
- **Traceability:** F5.5, AR-10, AR-11 (frame txgoB).

#### TC-E5-F5.5-004 · Custom calendar range picker (Terapkan)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify a custom range applies start→end and updates list + counts.
- **Preconditions:** Riwayat open.
- **Steps:**
  1. Open range chip → "Custom…"; tap 5 Mei (start), 18 Mei (end); tap "Terapkan".
- **Expected result:** List + summary-chip counts reflect 5–18 Mei 2026.
- **Traceability:** F5.5, AR-10, AC "Agent picks a custom date range" (frame x2rDk).

#### TC-E5-F5.5-005 · Filter with zero results → EmptyFilteredZero (no dead-flow)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Empty
- **Objective:** Verify a chip/range combination with no matches shows a reset/widen CTA.
- **Preconditions:** A status/range combo with no records.
- **Steps:**
  1. Apply a filter producing zero results.
- **Expected result:** `EmptyFilteredZero` state with reset-to-Semua / widen-range CTA (no dead-flow).
- **Traceability:** F5.5, C-6, AR-10, AR-11.

#### TC-E5-F5.5-006 · Agent with no records → empty state
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Empty
- **Objective:** Verify the empty state for a new agent with no attendance.
- **Preconditions:** Agent has zero records.
- **Steps:**
  1. Open Riwayat.
- **Expected result:** Empty state shown.
- **Traceability:** F5.5, C-1.

#### TC-E5-F5.5-007 · Cross-midnight record displays spanning two days
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify display of a night-shift record.
- **Preconditions:** A 2026-06-17 night-shift record clocked out 2026-06-18.
- **Steps:**
  1. View the record.
- **Expected result:** Displays spanning two days; counted once to the start date; times in Asia/Jakarta.
- **Traceability:** F5.5, AR-8, C-4.

#### TC-E5-F5.5-008 · Corrected record shows current values + correction indicator
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify a corrected record indicates a correction was applied.
- **Preconditions:** A record that had a correction applied.
- **Steps:**
  1. View the record.
- **Expected result:** Shows current (post-correction) values + an indicator that a correction was applied.
- **Traceability:** F5.5, C-5.

### Web · Shift Leader POV

#### TC-E5-F5.5-009 · Leader team view + exception-only filter (own company)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify the leader sees their company's records and can filter exceptions-only.
- **Preconditions:** SL of "Plaza Senayan"; mixed records this week.
- **Steps:**
  1. Filter by exception-only for this week.
- **Expected result:** Only late/out-of-geofence/incomplete/absent records for the led company shown.
- **Traceability:** F5.5, AR-1, AR-3, AC "Leader views team attendance with exceptions".

#### TC-E5-F5.5-010 · Leader company filter is server-pinned (AR-9)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Verify the company filter is locked to the SL's E3 assignment; cross-company values denied.
- **Preconditions:** SL of "Plaza Senayan".
- **Steps:**
  1. Inspect the company filter (should be pinned; site/position only narrow within).
  2. Attempt a request with a cross-company `company`/`site` value.
- **Expected result:** UI offers no out-of-scope options; a forced cross-company value → `403 OUT_OF_SCOPE`.
- **Traceability:** F5.5, AR-9, AC "Scope enforced for leaders".

#### TC-E5-F5.5-011 · Row action deep-links to verify (F5.3) / correct (F5.4)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P2 · **Type:** Happy
- **Objective:** Verify read-only rows deep-link to verify/correct.
- **Preconditions:** A Pending exception row in the list.
- **Steps:**
  1. Use the row action.
- **Expected result:** Navigates to the verify (F5.3) or correct (F5.4) flow; the list itself is read-only.
- **Traceability:** F5.5, AR-7.

### Web · HR / Super Admin POV

#### TC-E5-F5.5-012 · Filters: company, site, position, date, status, verification, exception
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify all documented filters work (position is free-text).
- **Preconditions:** Cross-company data.
- **Steps:**
  1. Apply combinations of company/site/position/date-range/status/verification/exception filters.
- **Expected result:** Results match filters; `company`/`site`/`position` map 1:1 to denormalized columns (no JOIN needed).
- **Traceability:** F5.5, AR-3, FEATURE §4 denormalization.

#### TC-E5-F5.5-013 · Billable rollup for a client grouped by position
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify the billable rollup sums worked records with billable codes grouped by company/position/period.
- **Preconditions:** Verified records with billable attendance codes for "Plaza Senayan", June 2026.
- **Steps:**
  1. Run the billable rollup for "Plaza Senayan" for June 2026.
- **Expected result:** Worked hours for billable codes grouped by position; feeds E10.
- **Traceability:** F5.5, AR-4, AC "Billable rollup for a client".

#### TC-E5-F5.5-014 · Payable rollup for payroll context
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Happy
- **Objective:** Verify the payable rollup sums records with payable codes.
- **Preconditions:** Records with payable codes.
- **Steps:**
  1. Run the payable rollup.
- **Expected result:** Payable hours summed for payroll context (E8).
- **Traceability:** F5.5, AR-5.

#### TC-E5-F5.5-015 · Unverified records in billable rollup (policy)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify the policy for Pending records in billing (flag/exclude until verified).
- **Preconditions:** Pending (unverified) billable-coded records exist.
- **Steps:**
  1. Run the billable rollup.
- **Expected result:** Per policy, unverified records are flagged/excluded from billing until verified (billable = verified records).
- **Traceability:** F5.5, C-3, FEATURE §7 ("Billable = verified records only").

#### TC-E5-F5.5-016 · Export reflects filters and is audited
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify the export contains only filtered rows and is audited.
- **Preconditions:** Filtered by position "Security Guard" + status "Present".
- **Steps:**
  1. Export to Excel.
- **Expected result:** File contains only matching records; export recorded in the audit log (who exported what).
- **Traceability:** F5.5, AR-6, AC "Export reflects filters and is audited".

#### TC-E5-F5.5-017 · High-volume range — cursor pagination; large export streamed/queued
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify pagination and large-export handling.
- **Preconditions:** A high-volume company + wide date range.
- **Steps:**
  1. Page through results (cursor); export a large set.
- **Expected result:** Server-side cursor pagination; large exports queued/streamed; `CURSOR_MISMATCH` (400) if sort/filter changes mid-cursor.
- **Traceability:** F5.5, C-2, CONVENTIONS §pagination.

#### TC-E5-F5.5-018 · Cross-company leader read denied (defense-in-depth)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** Verify a leader opening attendance for a company they don't lead is denied.
- **Preconditions:** SL attempts to open another company's attendance directly.
- **Steps:**
  1. Navigate to a non-led company's attendance.
- **Expected result:** Access denied (`403 OUT_OF_SCOPE`, `comp/EmptyNoPermission`).
- **Traceability:** F5.5, AR-1, AR-9, AC "Scope enforced for leaders".

#### TC-E5-F5.5-019 · Times render in Asia/Jakarta
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify all times display in Asia/Jakarta regardless of device locale.
- **Preconditions:** Records present; device in a different TZ.
- **Steps:**
  1. View records.
- **Expected result:** Times render in Asia/Jakarta.
- **Traceability:** F5.5, AR-8.

---

## F5.6 — Manual Attendance Entry (Buat Kehadiran Manual)

> Rules MR-1..MR-15; AC-1..AC-15; cases C-1..C-9. Web-only full-page form; geofence bypassed; always PENDING + MANUAL_ENTRY; created_by traced; SL scoped to own company.

### Web · HR / Super Admin POV

#### TC-E5-F5.6-001 · Happy path — HR creates manual attendance with schedule (Late)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify HR creates a manual record that evaluates lateness against the schedule.
- **Preconditions:** Budi (SWP-EMP-1042), active placement at "Plaza Senayan"; schedule today 07:00–15:00.
- **Steps:**
  1. Open the manual attendance page; search "Budi", select SWP-EMP-1042.
  2. Select date 2026-06-04; autofill returns company/site/schedule.
  3. Enter check_in_at 2026-06-04T08:00:00+07:00; submit.
- **Expected result:** Record created: status=LATE, late_minutes=60, verification_status=PENDING, flags=[MANUAL_ENTRY, LATE], schedule_id set, `geofence_in={inside:true,distance_m:0,radius_m:0}`, lat_in/lng_in null, wfo=true, created_by=HR; redirects to dashboard.
- **Traceability:** F5.6, MR-1..MR-9, MR-12, AC-1.

#### TC-E5-F5.6-002 · Happy path — manual with check-out (Present, worked_minutes)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify worked_minutes computed and on-time status.
- **Preconditions:** Active placement + schedule 07:00–15:00.
- **Steps:**
  1. Enter check_in 07:00, check_out 15:00; submit.
- **Expected result:** status=PRESENT, late_minutes=0, worked_minutes=480, flags=[MANUAL_ENTRY].
- **Traceability:** F5.6, MR-7, AC-2.

#### TC-E5-F5.6-003 · No active placement → 422 NO_ACTIVE_PLACEMENT (create)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Negative
- **Objective:** Verify create rejects when no active placement.
- **Preconditions:** Employee has no active placement.
- **Steps:**
  1. Submit a manual attendance for that employee.
- **Expected result:** `422 NO_ACTIVE_PLACEMENT`.
- **Traceability:** F5.6, MR-1, AC-3, C-1.

#### TC-E5-F5.6-004 · check_out before check_in → 400 INVALID_REQUEST (field check_out_at)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Negative
- **Objective:** Verify time-order validation.
- **Preconditions:** Active placement.
- **Steps:**
  1. Enter check_out_at before check_in_at; submit.
- **Expected result:** `400 INVALID_REQUEST` with field `check_out_at`.
- **Traceability:** F5.6, MR-2, AC-4, C-2.

#### TC-E5-F5.6-005 · No schedule → unscheduled, no lateness
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify a manual entry with no schedule is UNSCHEDULED with no lateness eval.
- **Preconditions:** Active placement but no schedule today.
- **Steps:**
  1. Enter check_in 10:00; submit.
- **Expected result:** schedule_id=null, status=PRESENT, flags=[MANUAL_ENTRY, UNSCHEDULED], late_minutes=0.
- **Traceability:** F5.6, MR-5, AC-5, C? (no schedule).

#### TC-E5-F5.6-006 · check_out_at equals check_in_at → valid (0 minutes)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify equal times are accepted.
- **Preconditions:** Active placement.
- **Steps:**
  1. Enter identical check_in and check_out; submit.
- **Expected result:** Valid; worked_minutes=0.
- **Traceability:** F5.6, MR-7, C-6.

#### TC-E5-F5.6-007 · Future check-in date allowed (no server restriction)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify a future date is accepted (no server-side future block).
- **Preconditions:** Active placement.
- **Steps:**
  1. Enter a future check_in date; submit.
- **Expected result:** Allowed (HR practice is today/past, but no server restriction).
- **Traceability:** F5.6, C-3.

#### TC-E5-F5.6-008 · Idempotency replay returns same 201 + Idempotent-Replayed header
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify safe replay with the same Idempotency-Key + body.
- **Preconditions:** A successful create with a known Idempotency-Key.
- **Steps:**
  1. Replay the same request (same key + body).
- **Expected result:** Same `201` with the same record ID and `Idempotent-Replayed: true`.
- **Traceability:** F5.6, MR-10, AC-10, CONVENTIONS §idempotency.

#### TC-E5-F5.6-009 · Note optional, stored as note
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Happy
- **Objective:** Verify the optional note is stored.
- **Preconditions:** Active placement.
- **Steps:**
  1. Create with a free-text note.
- **Expected result:** `note` stored on the record.
- **Traceability:** F5.6, MR-11.

#### TC-E5-F5.6-010 · Multiple placements — server resolves single active (INV-1)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify resolution to the single active placement.
- **Preconditions:** Employee with history of multiple placements (one active per INV-1).
- **Steps:**
  1. Create manual attendance.
- **Expected result:** Server resolves the single active placement; no ambiguity.
- **Traceability:** F5.6, MR-1, C-5, INV-1.

### Web · Autofill behavior

#### TC-E5-F5.6-011 · Autofill returns placement + schedule (200)
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify autofill returns company/site/position + schedule times.
- **Preconditions:** SWP-EMP-1002 with active placement + schedule 2026-06-04 07:00–15:00.
- **Steps:**
  1. `GET /attendance:manual-autofill?employee_id=SWP-EMP-1002&date=2026-06-04`.
- **Expected result:** Returns employee_name, company_name, schedule_id, shift_start_at, shift_end_at.
- **Traceability:** F5.6, AC-11, MR-1.

#### TC-E5-F5.6-012 · Autofill returns no schedule when absent
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify autofill nulls schedule fields when none exists and submit stays enabled.
- **Preconditions:** Active placement, no schedule 2026-06-04.
- **Steps:**
  1. Call autofill for that date.
- **Expected result:** schedule_id/shift_start_at/shift_end_at null, existing_attendance_id null; form shows placement summary + non-blocking "no schedule" notice + enabled submit.
- **Traceability:** F5.6, AC-12, MR-5.

#### TC-E5-F5.6-013 · Autofill resolves PKWTT (open-ended) and EXPIRING placements
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify open-ended and EXPIRING/EXTENDED placements resolve (not NO_ACTIVE_PLACEMENT).
- **Preconditions:** Placement with EXPIRING + end_date 2026-06-29, OR end_date NULL (PKWTT); chosen date in term.
- **Steps:**
  1. Call autofill for the employee + in-term date.
- **Expected result:** Placement resolved (company/site/position populated); no NO_ACTIVE_PLACEMENT.
- **Traceability:** F5.6, AC-13, MR-1.

#### TC-E5-F5.6-014 · Autofill surfaces existing attendance → form disables create, steers to verify/correct
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify the absence-sweep row is detected and create is disabled.
- **Preconditions:** SWP-EMP-3001, schedule 2026-06-10; absence-sweep created ABSENT/PENDING record SWP-ATT-3.
- **Steps:**
  1. Call autofill for 2026-06-10.
- **Expected result:** Returns existing_attendance_id=SWP-ATT-3, status ABSENT, verification PENDING; form disables create and shows "Kehadiran sudah ada" card with "Lihat & Koreksi Kehadiran" → opens /attendance/SWP-ATT-3.
- **Traceability:** F5.6, MR-14, AC-14, C-9.

#### TC-E5-F5.6-015 · Autofill no-placement (422) is a non-blocking warning; submit stays enabled
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Error
- **Objective:** Verify 422 NO_ACTIVE_PLACEMENT renders an amber warning, not a blocking error.
- **Preconditions:** Employee has no active placement on the chosen date.
- **Steps:**
  1. Trigger autofill (receives 422).
- **Expected result:** Amber informational "no active placement" warning (not red error); submit remains enabled; the create endpoint re-validates per MR-1.
- **Traceability:** F5.6, MR-15, AC-15.

#### TC-E5-F5.6-016 · Autofill network/5xx → blocking error with "Coba lagi" retry
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Error
- **Objective:** Verify genuine fetch failures render the blocking error state.
- **Preconditions:** Autofill returns a network/5xx error.
- **Steps:**
  1. Trigger autofill with backend unavailable.
- **Expected result:** Summary shows red blocking error + "Coba lagi" retry action.
- **Traceability:** F5.6, MR-15, AC-15.

#### TC-E5-F5.6-017 · Autofill missing date param → 400 INVALID_REQUEST
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Negative
- **Objective:** Verify required-param validation on autofill.
- **Preconditions:** N/A.
- **Steps:**
  1. `GET /attendance:manual-autofill?employee_id=SWP-EMP-1042` (no date).
- **Expected result:** `400 INVALID_REQUEST`.
- **Traceability:** F5.6, AC-9.

#### TC-E5-F5.6-018 · Loading state on autofill
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Loading
- **Objective:** Verify the summary card shows a loading state while autofill resolves.
- **Preconditions:** Slow autofill response.
- **Steps:**
  1. Select employee + date; observe the summary card.
- **Expected result:** "Ringkasan Penempatan" shows loading text until resolved; then placement rows + schedule notice.
- **Traceability:** F5.6, §10 design (loading state).

### Web · Shift Leader POV (scoped)

#### TC-E5-F5.6-019 · SL creates manual attendance within own company → 201
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify SL can create within their own company.
- **Preconditions:** SL leads "Plaza Senayan" (SWP-CMP-0021); employee's placement is there.
- **Steps:**
  1. Create manual attendance for that employee.
- **Expected result:** `201`; created_by = SL's SWP-EMP id; record PENDING.
- **Traceability:** F5.6, MR-12, MR-13, AC-6.

#### TC-E5-F5.6-020 · SL creates for employee outside own company → 422 OUT_OF_SCOPE
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Verify cross-company scope enforcement for SL.
- **Preconditions:** SL leads SWP-CMP-0021; target employee placed at SWP-CMP-0022.
- **Steps:**
  1. Attempt to create manual attendance for that employee.
- **Expected result:** `422 OUT_OF_SCOPE`.
- **Traceability:** F5.6, MR-13, AC-7.

#### TC-E5-F5.6-021 · SL creates manual attendance for themselves → allowed, PENDING
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify an SL may create their own manual attendance (still PENDING for another to verify).
- **Preconditions:** SL targets their own employee ID (within own company).
- **Steps:**
  1. Create manual attendance for self.
- **Expected result:** Allowed; record PENDING (another HR/leader verifies).
- **Traceability:** F5.6, C-7, MR-3.

#### TC-E5-F5.6-022 · Super admin creates for any company (no scope restriction)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** RBAC
- **Objective:** Verify super admin has no scope restriction on manual create.
- **Preconditions:** Super admin; any company/employee.
- **Steps:**
  1. Create manual attendance across companies.
- **Expected result:** Always allowed; created_by = super admin id.
- **Traceability:** F5.6, C-8.

#### TC-E5-F5.6-023 · Always PENDING + MANUAL_ENTRY flag + audit source=manual_entry
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P1 · **Type:** Happy
- **Objective:** Verify every manual record is PENDING, flagged, and audited.
- **Preconditions:** Any successful manual create.
- **Steps:**
  1. Inspect the created record + audit log.
- **Expected result:** verification_status=PENDING; flag MANUAL_ENTRY present; audit written with source=manual_entry, created_by=actor.
- **Traceability:** F5.6, MR-3, MR-9, MR-12.

#### TC-E5-F5.6-024 · Agent cannot access the manual attendance page
- [ ] **Platform:** Web · **POV:** Agent · **Priority:** P1 · **Type:** RBAC
- **Objective:** Verify agents have no manual-entry capability.
- **Preconditions:** Agent (no web manual-entry permission).
- **Steps:**
  1. Attempt to open / call the manual attendance create.
- **Expected result:** `403 FORBIDDEN`; page hidden in UI (defense-in-depth; agents file corrections via F5.4 instead).
- **Traceability:** F5.6, §3 actors (HR/Super/SL only).

---

## F5.7 — Attendance Auto-Reconcile (late-roster linking)

> Rules AR-1..AR-10; cases C-AR-1..C-AR-9. Server-side side-effect of schedule create / :bulk-apply. No endpoint. Machine-owned UNSCHEDULED records adopted; human-decided records get lineage only; window [shift_start−2h, shift_end+4h]; grace 15 min.

### System · triggered by HR/SL roster create (results observed Web/Mobile)

#### TC-E5-F5.7-001 · On-time unscheduled clock-in adopted by a late roster → AUTO_APPROVED
- [ ] **Platform:** System (Web trigger) · **POV:** HR/SL trigger; Agent observe · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify a covering schedule adopts an on-time machine-owned UNSCHEDULED record and auto-approves it.
- **Preconditions:** Dewi clocked in 07:03 today, no schedule (UNSCHEDULED, PENDING, machine-owned).
- **Steps:**
  1. HR creates a SCHEDULED entry for Dewi today 07:00–15:00.
- **Expected result:** Record `schedule_id` links to the entry; `UNSCHEDULED` removed; status=PRESENT, verification_status=AUTO_APPROVED, is_payable=true; `ATTENDANCE_RECONCILED` audit with before/after.
- **Traceability:** F5.7, AR-1, AR-2, AR-3, AR-6, AR-7, AR-8, AR-10, AC "On-time unscheduled clock-in adopted".

#### TC-E5-F5.7-002 · Late unscheduled clock-in stays PENDING but correctly labelled LATE
- [ ] **Platform:** System · **POV:** SL observe · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify reconcile re-derives lateness and keeps the record in the queue.
- **Preconditions:** Budi clocked in 07:40 today, no schedule (UNSCHEDULED, PENDING).
- **Steps:**
  1. HR creates a SCHEDULED entry for Budi today 07:00–15:00.
- **Expected result:** status=LATE, late_minutes=40, flags=[LATE], verification_status remains PENDING.
- **Traceability:** F5.7, AR-6, AR-7, AC "Late unscheduled clock-in stays in queue".

#### TC-E5-F5.7-003 · Out-of-geofence survives reconcile (UNSCHEDULED stripped, OUTSIDE_GEOFENCE kept)
- [ ] **Platform:** System · **POV:** SL observe · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify independent flags are not stripped.
- **Preconditions:** UNSCHEDULED + OUTSIDE_GEOFENCE on-time clock-in today.
- **Steps:**
  1. Create a covering schedule.
- **Expected result:** UNSCHEDULED removed; OUTSIDE_GEOFENCE remains; verification_status remains PENDING.
- **Traceability:** F5.7, AR-7, AC "Out-of-geofence survives reconcile".

#### TC-E5-F5.7-004 · AUTO_CLOSED/INCOMPLETE incompleteness survives reconcile
- [ ] **Platform:** System · **POV:** SL observe · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify an auto-closed open record keeps INCOMPLETE (not overwritten to PRESENT/LATE).
- **Preconditions:** UNSCHEDULED + AUTO_CLOSED + INCOMPLETE machine-owned record.
- **Steps:**
  1. Create a covering schedule.
- **Expected result:** AUTO_CLOSED kept, status stays INCOMPLETE, UNSCHEDULED removed, PENDING.
- **Traceability:** F5.7, AR-6, AR-7, §6 transitions.

#### TC-E5-F5.7-005 · Day-off entry does not adopt the record
- [ ] **Platform:** System · **POV:** SL observe · **Priority:** P1 · **Type:** Negative
- **Objective:** Verify is_day_off=true does not trigger reconcile.
- **Preconditions:** UNSCHEDULED clock-in today.
- **Steps:**
  1. HR marks today as is_day_off for that agent.
- **Expected result:** Record unchanged (still UNSCHEDULED, PENDING) — worked on a declared day off, genuinely needs SL review.
- **Traceability:** F5.7, AR-1, C-AR-2, AC "Day-off entry does not adopt".

#### TC-E5-F5.7-006 · Check-in outside the shift window is not adopted
- [ ] **Platform:** System · **POV:** SL observe · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify window-only matching (no day fallback).
- **Preconditions:** UNSCHEDULED clock-in at 03:00 today.
- **Steps:**
  1. HR creates a SCHEDULED entry for today 14:00–22:00 (window [12:00, 02:00+1d]).
- **Expected result:** No record reconciled; the 03:00 record stays UNSCHEDULED.
- **Traceability:** F5.7, AR-3, C? , AC "Check-in outside the shift window".

#### TC-E5-F5.7-007 · Human-decided record (VERIFIED) gets lineage only (RELINKED)
- [ ] **Platform:** System · **POV:** HR observe · **Priority:** P0 · **Type:** Edge
- **Objective:** Verify a human-decided record is not re-derived; only schedule_id lineage attached.
- **Preconditions:** UNSCHEDULED record already VERIFIED by the SL.
- **Steps:**
  1. Create a covering schedule.
- **Expected result:** verification_status stays VERIFIED, status unchanged, flags/is_payable untouched; schedule_id + snapshot attached; audited `ATTENDANCE_RELINKED`.
- **Traceability:** F5.7, AR-9, AR-2, AC "Human-decided record is not silently re-derived".

#### TC-E5-F5.7-008 · Bulk roster reconciles each cell in its own transaction
- [ ] **Platform:** System · **POV:** HR trigger · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify :bulk-apply reconciles each agent independently.
- **Preconditions:** UNSCHEDULED clock-ins for Dewi and Budi today.
- **Steps:**
  1. HR bulk-applies a shift to both for today.
- **Expected result:** Each record reconciled in its own tx; partial failures isolated (existing BulkApply semantics).
- **Traceability:** F5.7, AR-10, C-AR-5, AC "Bulk roster reconciles each cell".

#### TC-E5-F5.7-009 · Cross-midnight clock-in caught by window-OR
- [ ] **Platform:** System · **POV:** observe · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify a 00:30 clock-in is adopted by a 23:00→07:00 shift dated the previous day.
- **Preconditions:** Clock-in 00:30 (machine-owned UNSCHEDULED); shift 23:00→07:00 dated yesterday.
- **Steps:**
  1. Create the cross-midnight shift entry.
- **Expected result:** Local date ≠ work_date, but window `[shift_start−2h, shift_end+4h]` catches it; record adopted.
- **Traceability:** F5.7, AR-3, C-AR-4.

#### TC-E5-F5.7-010 · Re-placed agent (placement mismatch) not matched
- [ ] **Platform:** System · **POV:** observe · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify the placement_id guard prevents adoption across a re-placement.
- **Preconditions:** Agent re-placed between clock-in and roster entry; record placement_id ≠ new entry placement_id.
- **Steps:**
  1. Create the new shift entry under the new placement.
- **Expected result:** No match (placement_id guard); record stays UNSCHEDULED; logged.
- **Traceability:** F5.7, AR-2, C-AR-8.

#### TC-E5-F5.7-011 · Two unscheduled clock-ins same day — earliest-in-window adopted
- [ ] **Platform:** System · **POV:** observe · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify only the in-window (earliest if several) record is adopted; the other stays UNSCHEDULED.
- **Preconditions:** Two UNSCHEDULED clock-ins today (e.g. after an auto-close); one new shift.
- **Steps:**
  1. Create the shift entry.
- **Expected result:** The in-window record (earliest if multiple qualify) adopted; the other stays UNSCHEDULED.
- **Traceability:** F5.7, AR-3, C-AR-9.

#### TC-E5-F5.7-012 · force_replace / MODIFIED entry skips reconcile (uniqueness guard)
- [ ] **Platform:** System · **POV:** observe · **Priority:** P2 · **Type:** Negative
- **Objective:** Verify reconcile does not run on force_replace of an existing entry.
- **Preconditions:** A day that already has a linked record; a force_replace (MODIFIED) entry is created.
- **Steps:**
  1. Perform the force_replace.
- **Expected result:** Skipped (that day's record already linked; respects `attendance_schedule_uq`).
- **Traceability:** F5.7, AR-1, AR-4, C-AR-6.

#### TC-E5-F5.7-013 · CANCELLED_BY_LEAVE entry does not reconcile
- [ ] **Platform:** System · **POV:** observe · **Priority:** P2 · **Type:** Negative
- **Objective:** Verify a leave-cancelled entry is not a real shift and triggers no reconcile.
- **Preconditions:** UNSCHEDULED clock-in; a CANCELLED_BY_LEAVE entry created.
- **Steps:**
  1. Create the CANCELLED_BY_LEAVE entry.
- **Expected result:** No reconcile (not a real shift).
- **Traceability:** F5.7, AR-1, C-AR-3.

#### TC-E5-F5.7-014 · Reconcile error rolls back the schedule create (fail-closed atomicity)
- [ ] **Platform:** System · **POV:** HR trigger · **Priority:** P1 · **Type:** Error
- **Objective:** Verify an unexpected reconcile error rolls back the entire schedule create.
- **Preconditions:** Force a reconcile failure inside the schedule-create tx (e.g. lock/constraint error).
- **Steps:**
  1. Create a covering schedule that triggers the failure.
- **Expected result:** Schedule create rolled back (correctness over availability); HR can retry; nothing partially applied.
- **Traceability:** F5.7, AR-10, §10 (synchronous, same-tx, fail-closed).

#### TC-E5-F5.7-015 · is_payable explicit true/false never overridden by reconcile
- [ ] **Platform:** System · **POV:** observe · **Priority:** P2 · **Type:** Edge
- **Objective:** Verify reconcile only sets is_payable when it is NULL.
- **Preconditions:** A machine-owned UNSCHEDULED record with is_payable explicitly false.
- **Steps:**
  1. Create a covering schedule.
- **Expected result:** is_payable stays false (only NULL → true); reconcile does not override an explicit value.
- **Traceability:** F5.7, AR-8.

#### TC-E5-F5.7-016 · Agent observes corrected status on mobile after reconcile (no action)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Happy
- **Objective:** Verify the agent sees the updated status/verification on the attendance detail.
- **Preconditions:** Agent's UNSCHEDULED record reconciled to PRESENT/AUTO_APPROVED.
- **Steps:**
  1. Open the attendance detail on mobile.
- **Expected result:** Updated `status` / `verification_status` shown (server-driven); no agent action required.
- **Traceability:** F5.7, §4 (Mobile/Agent observe).

---

## Appendix — Cross-cutting RBAC & state coverage summary

- **401 / session expired:** TC-E5-F5.1-024 (extend to every authenticated surface: corrections, manual entry, verification, records).
- **403 / OUT_OF_SCOPE (leader cross-company):** TC-E5-F5.3-007, TC-E5-F5.5-010, TC-E5-F5.5-018, TC-E5-F5.6-020.
- **403 / capability (agent on admin-only):** TC-E5-F5.1-025, TC-E5-F5.6-024.
- **Empty states:** TC-E5-F5.3-015, TC-E5-F5.4-011, TC-E5-F5.5-005, TC-E5-F5.5-006.
- **Loading states:** TC-E5-F5.6-018 (and autofill/list spinners generally).
- **Error states:** TC-E5-F5.1-010/012, TC-E5-F5.6-016, TC-E5-F5.7-014.
- **Idempotency / bulk:** TC-E5-F5.3-005, TC-E5-F5.6-008.
- **Photo rule (mobile required / web exempt / clock-out optional):** TC-E5-F5.1-001/002/003/007/008/011/012/013/020/021.
- **Geofence (inside / outside-flagged / no-geo / remote):** TC-E5-F5.1-001/004/017/018, TC-E5-F5.2-004.
- **One-open-record (CI-5):** TC-E5-F5.1-006.
- **Unscheduled (CI-4) → reconcile (F5.7):** TC-E5-F5.1-005, TC-E5-F5.7-*.
