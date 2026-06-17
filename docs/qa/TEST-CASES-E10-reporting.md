# Test Cases · E10 — Reporting, Exports & Notifications

> **Epic:** E10 (cross-cutting) · **Type:** Manual test cases · **Status:** v1
> **Source:** [FEATURE.md](../epics/E10-reporting/FEATURE.md) · PRDs: [notifications](../epics/E10-reporting/prds/notifications.md) · [dashboards](../epics/E10-reporting/prds/dashboards.md) · [attendance-billable-report](../epics/E10-reporting/prds/attendance-billable-report.md) · [export-framework](../epics/E10-reporting/prds/export-framework.md) · [API CONVENTIONS](../api/CONVENTIONS.md)
> **Reference date for all "today/this month" cases:** Wednesday **2026-06-17** (Asia/Jakarta, WIB UTC+7). "June" = 2026-06-01 → 2026-06-30. "Within 30 days" windows are measured from 2026-06-17.

---

## 1. Scope

This document specifies **exhaustive manual test cases** for E10, organized **per platform (Web / Mobile) × per POV (super admin · HR/placement admin · shift leader · agent)**. It covers:

- **F10.1 Notifications & Notification Center** — push (mobile) + in-app center (web + mobile), read/unread, scope, preferences/mute, durability, deep-link, every event in the NT-3 catalog and the §16.2 dispatch table.
- **F10.2 Role-Based Dashboards** — each role sees ONLY its own widgets correctly scoped (agent own / shift leader own company / HR all / super admin = HR superset + admin block), dual-surface leader dashboard (web + mobile Beranda), deep-links, empty/loading/error.
- **F10.3 Attendance & Billable-Hours Report** — verified+billable accuracy, group-by axes, scope, exports, edge cases (transfer, cross-midnight, pending, corrections).
- **F10.4 Export Framework** — xlsx/pdf/csv content correctness, scope honoring, inline vs queued (large), audit, point-in-time, confidentiality, failure.

**Out of scope (do not test here):** email/SMS/WhatsApp channels (INV-1 — not built), client-facing access/portal (INV-2 — clients don't log in), scheduled/emailed reports + external BI (not in v1). The **events themselves** are owned by E3–E8; here we test only that the **notification fires to the correct recipient on the correct channel** and that dashboards/reports **read** that data correctly.

**Key platform fact (FEATURE §6, per-PRD §4):** the **billable report (F10.3)** and the **export framework (F10.4)** are **Web-console only** — no mobile surface. **Notifications (F10.1)** reach all roles; **push is mobile-only**, **in-app center is web + mobile**. **Dashboards (F10.2):** agent = mobile only; shift leader = mobile Beranda + web team dashboard; HR/super admin = web only.

**Conventions used in expected results:** 401 = re-auth (`comp/EmptySessionExpired`); 403 = no-permission state (`comp/EmptyNoPermission`); 404 used in place of 403 where visibility would leak; copy defaults to Bahasa Indonesia (`Accept-Language` can switch to en-US). All displayed local times are Asia/Jakarta.

---

## 2. Coverage matrix (features × platform × role)

Legend: ✅ = primary surface (test fully) · ⛔ = role/platform has NO access (test the denial/absence) · — = not a platform for this feature.

| Feature | Surface | Super Admin | HR/Placement Admin | Shift Leader | Agent |
|---|---|---|---|---|---|
| **F10.1 Notifications — push** | Mobile | ⛔ (no mobile app)¹ | ⛔ (no mobile app)¹ | ✅ | ✅ |
| **F10.1 Notifications — in-app center** | Web | ✅ | ✅ | ✅ | ⛔ (agent has no web)² |
| **F10.1 Notifications — in-app center** | Mobile | ⛔¹ | ⛔¹ | ✅ | ✅ |
| **F10.2 Dashboard** | Web | ✅ HR superset + admin block | ✅ HR cockpit (no admin block) | ✅ team dashboard (own company) | ⛔² |
| **F10.2 Dashboard** | Mobile | ⛔¹ | ⛔¹ | ✅ Beranda (own company) | ✅ personal |
| **F10.3 Billable report** | Web | ✅ all companies | ✅ all companies | ✅ own company only | ⛔ (no report access) |
| **F10.3 Billable report** | Mobile | — | — | — | — |
| **F10.4 Export** | Web | ✅ all | ✅ all | ✅ own-company scope | ⛔ (no export access) |
| **F10.4 Export** | Mobile | — | — | — | — |

¹ Super admin / HR are **web-console** staff (FEATURE §6, all PRD §4 tables list mobile recipients as Agent/Leader only). Mobile cases for these roles verify the **absence** of a mobile app surface and that web in-app center carries their notifications.
² "Agent" = a `FIELD` employee with no elevation; the agent surface is the **mobile app**. Web console is staff-only. Agent web cases verify denial / no console access.

---

## 3. Test cases

---

## F10.1 — Notifications & Notification Center

> Traces: NT-1..NT-6, INV-1, INV-2, FEATURE §16.2 dispatch table, C-1..C-4.

### Web · Super Admin POV

#### TC-E10-F10.1-001 · Super admin sees HR-targeted notifications in web in-app center
- [ ] **Platform:** Web · **POV:** super admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Super admin receives the HR-admin cron notifications (agreement/placement expiring) in the in-app center.
- **Preconditions:** Logged in as super admin on web. At least one employment agreement expiring on or before 2026-07-17 (within 30 days) and one placement expiring on or before 2026-07-17 exist.
- **Steps:**
  1. Trigger (or wait for) the daily expiring-soon cron for 2026-06-17.
  2. Open the in-app notification center (bell).
- **Expected result / Acceptance criteria:** Center lists an "Agreement expiring within 30 days" notification and a "Placement expiring within 30 days" notification. Each shows type, body, and a working deep-link to the agreement/placement detail. No push is expected on web (push is mobile-only).
- **Traceability:** F10.1, NT-3, §16.2 (Agreement/Placement expiring rows), INV-1.

#### TC-E10-F10.1-002 · Mark notification read updates unread badge
- [ ] **Platform:** Web · **POV:** super admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Opening a notification marks it read and decrements the unread count.
- **Preconditions:** Super admin has ≥1 unread notification; unread badge shows a count.
- **Steps:**
  1. Note the unread badge count.
  2. Open the notification center and click an unread notification.
  3. Re-open the center.
- **Expected result / Acceptance criteria:** The clicked item is rendered as read (no unread styling); `read_at` is set server-side; unread badge count decreases by 1. Re-loading the page keeps it read (durable).
- **Traceability:** F10.1, NT-4, AC "Mark read".

#### TC-E10-F10.1-003 · Scope — super admin never sees another user's private notifications
- [ ] **Platform:** Web · **POV:** super admin · **Priority:** P0 · **Type:** RBAC
- **Objective:** Even a super admin's center contains only notifications addressed to them (no cross-user leakage).
- **Preconditions:** Another user (e.g., agent Budi) has notifications addressed to them.
- **Steps:**
  1. Open super admin's notification center.
- **Expected result / Acceptance criteria:** Only notifications with `recipient_id` = super admin appear. Budi's leave-decided / shift notifications are absent. (Super admin may inspect others' notifications only via E1 audit, not the center.)
- **Traceability:** F10.1, NT-2, INV-2, AC "Scope".

### Web · HR/Placement Admin POV

#### TC-E10-F10.1-004 · HR receives agreement-expiring notification (cron)
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P0 · **Type:** Happy
- **Objective:** HR admin is notified when an employment agreement expires within 30 days.
- **Preconditions:** HR admin logged in on web. An agreement with `end_date` = 2026-07-10 (within 30 days of 2026-06-17) exists; cron has run for 2026-06-17.
- **Steps:**
  1. Open in-app center.
- **Expected result / Acceptance criteria:** An "Agreement expiring within 30 days" notification appears, naming the employee and end date, deep-linking to the agreement. Delivered to HR admins (cron-driven).
- **Traceability:** F10.1, §16.2 (Agreement expiring → HR admins), NT-3.

#### TC-E10-F10.1-005 · HR receives placement-expiring notification (cron) — co-recipient with leader
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Placement expiring within 30 days notifies HR admins AND the assigned leader.
- **Preconditions:** A placement with `end_date` = 2026-07-01 (within 30 days); an assigned leader exists. Cron run for 2026-06-17.
- **Steps:**
  1. Open HR's in-app center; confirm the placement-expiring notification.
  2. (Cross-check) Confirm the assigned leader also received it (see TC-E10-F10.1-014).
- **Expected result / Acceptance criteria:** HR sees the placement-expiring notification with deep-link to the placement. The same notification is independently delivered to the assigned leader (two recipients, two notification rows).
- **Traceability:** F10.1, §16.2 (Placement expiring → HR admins + assigned leader).

#### TC-E10-F10.1-006 · HR receives attendance-correction-escalation notification (>7 days)
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Edge
- **Objective:** An attendance correction submitted for a record older than 7 days notifies HR (in addition to the shift leader).
- **Preconditions:** An agent submits a correction for an attendance record dated 2026-06-05 (>7 days before 2026-06-17). HR admin logged in.
- **Steps:**
  1. After the correction is submitted, open HR's in-app center.
- **Expected result / Acceptance criteria:** HR sees an "attendance correction submitted" notification (escalated because >7 days), deep-linking to the correction. For a ≤7-day correction, HR would NOT be notified (only the leader).
- **Traceability:** F10.1, §16.2 (Attendance correction submitted → Shift leader, and HR if >7 days), NT-3.

#### TC-E10-F10.1-007 · Empty state — notification center with no notifications
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P2 · **Type:** Empty
- **Objective:** A center with zero notifications shows a friendly empty state, not an error.
- **Preconditions:** Brand-new HR account with no notifications.
- **Steps:**
  1. Open the notification center.
- **Expected result / Acceptance criteria:** Empty-state copy (e.g., "Belum ada notifikasi") renders; no spinner stuck, no error.
- **Traceability:** F10.1, NT (center is durable record), DESIGN no-dead-flow.

#### TC-E10-F10.1-008 · Loading + error states of the center
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P2 · **Type:** Loading/Error
- **Objective:** The center shows a loading skeleton, and an error+retry if the fetch fails.
- **Preconditions:** Ability to throttle/fail the notifications list request.
- **Steps:**
  1. Open the center under slow network → observe loading.
  2. Force the list request to 500 → observe error.
- **Expected result / Acceptance criteria:** Loading skeleton shown while pending; on failure an error state with a retry action (not a blank panel). Retry refetches successfully.
- **Traceability:** F10.1, NT-6, error envelope §11.

#### TC-E10-F10.1-009 · Stale deep-link — target deleted
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Clicking a notification whose target was deleted shows a graceful "no longer available", not a crash.
- **Preconditions:** A notification deep-links to a placement that has since been soft-deleted (404 on fetch).
- **Steps:**
  1. Click the notification.
- **Expected result / Acceptance criteria:** A graceful "no longer available" message renders (mapped from 404/NOT_FOUND); the notification is still markable read; no unhandled error.
- **Traceability:** F10.1, C-4, CONVENTIONS §7 (404).

### Web · Shift Leader POV

#### TC-E10-F10.1-010 · Leader notified: attendance verification needed
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P0 · **Type:** Happy
- **Objective:** When agents in the leader's company need attendance verification, the leader gets an in-app notification.
- **Preconditions:** Leader of "Plaza Senayan" on web. An agent at Plaza Senayan has an attendance record awaiting verification (E5).
- **Steps:**
  1. Open the in-app center.
  2. Click the "attendance verification needed" notification.
- **Expected result / Acceptance criteria:** Notification present, scoped to Plaza Senayan; deep-link opens the attendance verification screen filtered to the relevant record(s). Recipient = shift leader of the company.
- **Traceability:** F10.1, §16.2 (Attendance verification needed → Shift leader of the company), NT-3, DB-5.

#### TC-E10-F10.1-011 · Leader notified: leave approval pending (routing)
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P0 · **Type:** Happy
- **Objective:** When an agent at the leader's company submits a leave request, the leader is notified there is an approval pending.
- **Preconditions:** Agent Budi (Plaza Senayan) submits a leave request; leader of Plaza Senayan logged in on web.
- **Steps:**
  1. Open in-app center.
- **Expected result / Acceptance criteria:** An "approval pending / requested" notification appears for the leader, deep-linking to the leave approval inbox/detail. (Mirrors AC "Approval routing notification".)
- **Traceability:** F10.1, NT-3 (approval requested), AC "Approval routing notification", §16.2.

#### TC-E10-F10.1-012 · Leader notified: approval advanced to a new line (E11)
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** When an approval instance advances to a line whose members include this leader, they are notified.
- **Preconditions:** A multi-line approval template; a request advances to a line where the leader is a current-line member.
- **Steps:**
  1. Cause the approval to advance to the leader's line.
  2. Open in-app center.
- **Expected result / Acceptance criteria:** An "approval advanced — your action needed" notification appears, deep-linking to the approval. Only NEW current-line members are notified (not earlier lines).
- **Traceability:** F10.1, §16.2 (Approval advanced → new current-line members).

#### TC-E10-F10.1-013 · Leader's center scoped to own company — no other company's events
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** The leader sees only notifications addressed to them (their company's events), never another company's.
- **Preconditions:** Leader of Plaza Senayan. A separate company "Grand Indonesia" has its own leader and pending approvals.
- **Steps:**
  1. Open Plaza Senayan leader's in-app center.
- **Expected result / Acceptance criteria:** Only Plaza Senayan-scoped notifications appear; no Grand Indonesia verification/approval notifications leak in.
- **Traceability:** F10.1, NT-2, INV-2/INV-3, scope `company`.

#### TC-E10-F10.1-014 · Leader receives placement-expiring (assigned leader recipient)
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P2 · **Type:** Happy
- **Objective:** The assigned leader is a recipient of the placement-expiring notification.
- **Preconditions:** A placement at the leader's company expiring 2026-07-01; cron run.
- **Steps:**
  1. Open the leader's in-app center.
- **Expected result / Acceptance criteria:** "Placement expiring within 30 days" notification present, deep-linking to the placement.
- **Traceability:** F10.1, §16.2 (Placement expiring → HR admins + assigned leader).

### Mobile · Shift Leader POV

#### TC-E10-F10.1-015 · Leader gets push for attendance verification needed
- [ ] **Platform:** Mobile · **POV:** shift leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Push notification is delivered to the leader's device when verification is needed.
- **Preconditions:** Leader logged into mobile app with a valid push token; OS push permission granted. An agent at their company needs verification.
- **Steps:**
  1. Trigger the verification-needed event.
  2. Observe the device (foreground + background).
- **Expected result / Acceptance criteria:** A push notification arrives; tapping it deep-links into the verification screen. The same item also appears in the in-app center (durable). Recipient correct (company leader).
- **Traceability:** F10.1, NT-1 (push), §16.2, NT-6.

#### TC-E10-F10.1-016 · Leader push + in-app for leave/OT approval pending
- [ ] **Platform:** Mobile · **POV:** shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Approval-pending events reach the leader via push and the in-app center on mobile.
- **Preconditions:** Agent submits leave and OT requests at the leader's company.
- **Steps:**
  1. Submit a leave request and an OT request.
  2. Observe leader's device + in-app center.
- **Expected result / Acceptance criteria:** A push and in-app notification appear for each (or batched if a burst — see TC-...-024); deep-links open the respective approval screens.
- **Traceability:** F10.1, NT-3, §16.2.

#### TC-E10-F10.1-017 · Push fails to deliver → in-app center still has it (durability)
- [ ] **Platform:** Mobile · **POV:** shift leader · **Priority:** P0 · **Type:** Edge
- **Objective:** If push delivery fails, the notification is still durable in the in-app center.
- **Preconditions:** Simulate push provider failure (e.g., invalid/expired token at FCM/APNs) while the leader is offline at push time.
- **Steps:**
  1. Trigger an event while push delivery is forced to fail.
  2. Open the app and view the in-app center.
- **Expected result / Acceptance criteria:** The notification is present in the in-app center despite the failed push (best-effort + retried; center is the durable record).
- **Traceability:** F10.1, NT-6, AC "In-app center is durable".

#### TC-E10-F10.1-018 · No push token → in-app only + prompt to enable push
- [ ] **Platform:** Mobile · **POV:** shift leader · **Priority:** P2 · **Type:** Edge
- **Objective:** A device without a push token receives in-app notifications and is prompted to enable push.
- **Preconditions:** Mobile app installed, OS push permission denied/not granted (no token registered).
- **Steps:**
  1. Trigger a notification event.
  2. Open the app.
- **Expected result / Acceptance criteria:** Notification appears in the in-app center; a prompt/banner invites the user to enable push. No push arrives (expected).
- **Traceability:** F10.1, C-1.

### Mobile · Agent POV

#### TC-E10-F10.1-019 · Agent gets push + in-app for schedule published/changed
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P0 · **Type:** Happy
- **Objective:** When the leader publishes/changes the agent's schedule, the agent receives push + in-app with a deep-link.
- **Preconditions:** Agent Budi on mobile with valid push token. Leader publishes a schedule that includes Budi (or changes Budi's shift).
- **Steps:**
  1. Leader publishes the schedule.
  2. Observe Budi's device and in-app center.
- **Expected result / Acceptance criteria:** Budi receives a push AND an in-app notification; tapping deep-links to his schedule for the affected date. Recipient = affected agent only.
- **Traceability:** F10.1, NT-3 (schedule published/changed), §16.2 (Schedule published → affected agents), AC "Schedule change notifies the agent".

#### TC-E10-F10.1-020 · Agent gets notification when leave decided (approved/rejected)
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P0 · **Type:** Happy
- **Objective:** The submitter (agent) is notified when their leave request is approved or rejected.
- **Preconditions:** Budi has a leave request; leader/HR decides it.
- **Steps:**
  1. Approve the request → observe Budi's device.
  2. Repeat with a different request that is rejected.
- **Expected result / Acceptance criteria:** Budi receives push + in-app "leave approved" (and separately "leave rejected"), each deep-linking to the leave request detail showing the decision. Recipient = request submitter.
- **Traceability:** F10.1, §16.2 (Leave approved/rejected → submitter; Approval decided → submitter), NT-3.

#### TC-E10-F10.1-021 · Agent gets OT decided + OT auto-detected notifications
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Agent is notified on OT approved/rejected and on OT auto-detected (E7).
- **Preconditions:** Budi has an OT request decided; separately the system auto-detects OT for Budi.
- **Steps:**
  1. Decide Budi's OT request → observe.
  2. Trigger an auto-detected OT for Budi → observe.
- **Expected result / Acceptance criteria:** Budi receives notifications for "OT approved/rejected" and for "OT auto-detected", each deep-linking to the OT record. Recipient = submitter.
- **Traceability:** F10.1, §16.2 (OT approved/rejected/auto-detected → submitter), NT-3.

#### TC-E10-F10.1-022 · Agent gets shift-reminder notification (lead time)
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Happy
- **Objective:** The agent receives a shift reminder ahead of an upcoming shift.
- **Preconditions:** Budi has a shift scheduled for 2026-06-17 at 14:00 WIB; reminder lead time configured.
- **Steps:**
  1. Wait until the reminder lead time before 14:00.
- **Expected result / Acceptance criteria:** Budi receives a push (and in-app) shift reminder naming the site and start time, deep-linking to the shift. (Lead-time value is a build-phase decision; verify a reminder fires at all.)
- **Traceability:** F10.1, NT-3 (shift reminder), deferred §7.1 (lead times).

#### TC-E10-F10.1-023 · Agent notified: placement/leader change (E3)
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Happy
- **Objective:** Agent is notified when their placement or shift leader changes.
- **Preconditions:** HR transfers Budi to a new placement / a new leader is assigned to Budi's company.
- **Steps:**
  1. Perform the placement/leader change.
  2. Observe Budi's device/center.
- **Expected result / Acceptance criteria:** Budi receives a "placement/leader change" notification deep-linking to his placement.
- **Traceability:** F10.1, NT-3 (placement/leader changes), §16.2.

#### TC-E10-F10.1-024 · Burst of schedule events is batched/grouped
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Edge
- **Objective:** A bulk schedule publish affecting many shifts for one agent groups into a sensible batched notification (not dozens of pushes).
- **Preconditions:** Leader bulk-publishes a month of schedule including 20+ of Budi's shifts.
- **Steps:**
  1. Bulk-publish the schedule.
  2. Observe Budi's device + in-app center.
- **Expected result / Acceptance criteria:** Budi receives a grouped/batched notification (e.g., "Jadwal bulan Juni dipublikasikan") rather than one push per shift; deep-links to the schedule.
- **Traceability:** F10.1, C-2.

#### TC-E10-F10.1-025 · Critical category overrides mute; non-critical mute respected
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Muting a non-critical category suppresses those notifications, but critical categories (approvals, schedule changes) still arrive even when muted.
- **Preconditions:** Preferences default all-on. Budi mutes a non-critical category (e.g., shift reminders) and attempts to mute a critical one.
- **Steps:**
  1. Mute "shift reminders" → trigger a shift reminder → observe none arrives.
  2. Attempt to mute "schedule changes"/"approvals"; trigger a schedule change → observe it still arrives.
- **Expected result / Acceptance criteria:** Non-critical muted category produces no push/in-app entry. Critical category cannot be effectively silenced — the schedule-change notification is still delivered (critical overrides mute).
- **Traceability:** F10.1, NT-5, C-3.

#### TC-E10-F10.1-026 · Agent scope — only own notifications
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Agent's center contains only notifications addressed to them.
- **Preconditions:** Two agents Budi and Siti at the same company, each with their own notifications.
- **Steps:**
  1. Log in as Budi; open the center.
- **Expected result / Acceptance criteria:** Only Budi's notifications appear; none of Siti's. No cross-user leakage.
- **Traceability:** F10.1, NT-2, AC "Scope".

#### TC-E10-F10.1-027 · Agent has no web console (negative surface)
- [ ] **Platform:** Web · **POV:** agent · **Priority:** P1 · **Type:** RBAC
- **Objective:** An agent (no elevation) cannot reach the web in-app center / console.
- **Preconditions:** Agent credentials; attempt to load the web console.
- **Steps:**
  1. Attempt to log into / open the web console as an agent.
- **Expected result / Acceptance criteria:** Web console is staff-only; the agent is blocked from the console (no-permission or login disallowed for non-elevated employees on web). Agent's surface is the mobile app only.
- **Traceability:** F10.1, FEATURE §6 (web = leader/HR), CONVENTIONS §17.

---

## F10.2 — Role-Based Dashboards

> Traces: DB-1..DB-8, INV-3, C-1..C-7. Reference "today" = 2026-06-17.

### Web · Super Admin POV

#### TC-E10-F10.2-001 · Super admin dashboard = HR cockpit superset + admin block
- [ ] **Platform:** Web · **POV:** super admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Super admin sees ALL HR cross-company KPIs PLUS the admin-only widget block.
- **Preconditions:** Super admin logged in; populated data across companies; users pending provisioning; recent audit entries; multiple positions; ≥1 pending role-change grant.
- **Steps:**
  1. Open the dashboard (`GET /dashboards/me`).
- **Expected result / Acceptance criteria:** HR KPIs render (attendance rate, billable-hours trend, OT totals, leave usage, active placements/headcount). Plus an `admin` block with FOUR widgets: (a) users & access (active users, accounts pending provisioning, offboarded/disabled ≤30d), (b) recent audit feed, (c) org rollups by free-text position (headcount + active placements), (d) pending grants. Response payload contains an `admin` block.
- **Traceability:** F10.2, DB-7, DB-4, AC "Super Admin sees admin widgets".

#### TC-E10-F10.2-002 · Admin widgets deep-link correctly
- [ ] **Platform:** Web · **POV:** super admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Each admin widget deep-links to its underlying feature.
- **Preconditions:** As above, with data in each widget.
- **Steps:**
  1. Click users & access → expect users/access screen (E2/F2.7).
  2. Click an audit entry / "Lihat semua" → expect the full audit log (E1).
  3. Click an org-rollup row → expect filtered headcount/placements by that position (E3).
  4. Click a pending grant → expect the role-change approval action.
- **Expected result / Acceptance criteria:** Each click navigates to the correct underlying screen with appropriate filter context.
- **Traceability:** F10.2, DB-5, DB-7, C-7.

#### TC-E10-F10.2-003 · Audit feed is capped (~8) with "Lihat semua" link
- [ ] **Platform:** Web · **POV:** super admin · **Priority:** P2 · **Type:** Edge
- **Objective:** The recent audit feed widget caps at ~8 entries and links to the full log.
- **Preconditions:** >50 recent sensitive audit actions exist.
- **Steps:**
  1. Open the dashboard; inspect the audit feed widget.
- **Expected result / Acceptance criteria:** Only ~8 most-recent entries render; a "Lihat semua" link deep-links to the full audit log.
- **Traceability:** F10.2, C-7, DB-5.

#### TC-E10-F10.2-004 · Admin widgets empty state on a fresh tenant
- [ ] **Platform:** Web · **POV:** super admin · **Priority:** P2 · **Type:** Empty
- **Objective:** With zero data, each admin widget renders its own empty state without errors.
- **Preconditions:** Fresh tenant: no pending provisioning, no audit entries, no placements, no pending grants.
- **Steps:**
  1. Open the super admin dashboard.
- **Expected result / Acceptance criteria:** Each admin widget shows an empty/getting-started state; no crashes; HR KPI widgets also show empty/zeroed states.
- **Traceability:** F10.2, C-5, C-1, DB-7.

#### TC-E10-F10.2-005 · Loading + error states of the dashboard
- [ ] **Platform:** Web · **POV:** super admin · **Priority:** P2 · **Type:** Loading/Error
- **Objective:** Dashboard shows skeletons while loading and an error+retry on failure.
- **Preconditions:** Ability to throttle/fail `GET /dashboards/me`.
- **Steps:**
  1. Open under slow network → observe skeletons.
  2. Force a 500 → observe error.
- **Expected result / Acceptance criteria:** Per-widget skeletons during load; on failure an error state with retry (not a blank page). Retry succeeds.
- **Traceability:** F10.2, DB-6, CONVENTIONS §11.

### Web · HR/Placement Admin POV

#### TC-E10-F10.2-006 · HR dashboard shows cross-company KPIs, NO admin block
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P0 · **Type:** RBAC
- **Objective:** HR admin sees the cross-company KPIs but the `admin` block is absent (not null-filled), and no admin widgets render.
- **Preconditions:** HR admin logged in; same populated data as super admin case.
- **Steps:**
  1. Open the dashboard.
  2. Inspect the response payload and rendered widgets.
- **Expected result / Acceptance criteria:** Attendance rate, billable-hours trend, OT totals, leave usage, active placements/headcount all render. The payload has NO `admin` key (absent, not `null`); FE renders zero admin widgets (no users & access / audit feed / org rollups / pending grants).
- **Traceability:** F10.2, DB-7, C-6, AC "HR Admin does not see admin widgets".

#### TC-E10-F10.2-007 · HR KPI aggregation performant with many companies + drill-down
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Edge
- **Objective:** With many companies, KPIs aggregate within acceptable time and support drill-down by company.
- **Preconditions:** HR account spanning 50+ companies with substantial attendance/OT/leave data.
- **Steps:**
  1. Open the dashboard; note load time.
  2. Drill into a KPI by company.
- **Expected result / Acceptance criteria:** Dashboard loads within an acceptable delay (near-live/cached); drill-down filters the KPI to the selected company.
- **Traceability:** F10.2, C-2, C-4, DB-4, DB-6.

#### TC-E10-F10.2-008 · KPI widget deep-links to underlying feature
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Clicking a KPI navigates to the feature (e.g., billable-hours trend → report; headcount → placements).
- **Preconditions:** Populated HR dashboard.
- **Steps:**
  1. Click billable-hours trend → expect the F10.3 report.
  2. Click active placements/headcount → expect placement list (E3).
- **Expected result / Acceptance criteria:** Each KPI deep-links to the correct feature with relevant filters.
- **Traceability:** F10.2, DB-5.

#### TC-E10-F10.2-009 · New HR account — friendly empty/getting-started
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P2 · **Type:** Empty
- **Objective:** A new HR user with no data sees a friendly empty/getting-started state.
- **Preconditions:** Fresh HR account, no underlying data.
- **Steps:**
  1. Open the dashboard.
- **Expected result / Acceptance criteria:** Friendly empty state across KPI widgets; no errors.
- **Traceability:** F10.2, C-1.

### Web · Shift Leader POV

#### TC-E10-F10.2-010 · Leader team dashboard scoped to own company
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Leader's web team dashboard shows today's roster, who's clocked in, pending approvals, open exceptions, and coverage gaps — for their company only.
- **Preconditions:** Leader of Plaza Senayan logged in; roster, clock-ins, pending leave/OT/attendance approvals, exceptions, and a coverage gap exist for 2026-06-17.
- **Steps:**
  1. Open the team dashboard (`GET /dashboards/me` returns `LeaderDashboard` for `shift_leader`).
- **Expected result / Acceptance criteria:** Widgets show today's roster, who's clocked in, pending approvals (attendance/leave/OT), open exceptions, and coverage gaps — all limited to Plaza Senayan.
- **Traceability:** F10.2, DB-3, DB-8, INV-3, AC "Shift-leader dashboard scoped".

#### TC-E10-F10.2-011 · Leader dashboard shows ONLY own company (scope enforced)
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** No other company's roster/approvals appear; no HR cross-company KPIs; no admin block.
- **Preconditions:** Plaza Senayan leader; Grand Indonesia also has data.
- **Steps:**
  1. Open the leader dashboard; inspect widgets and payload.
- **Expected result / Acceptance criteria:** Only Plaza Senayan data renders. No Grand Indonesia data, no cross-company KPIs, no `admin` block.
- **Traceability:** F10.2, DB-1, INV-3, AC "Scope enforced".

#### TC-E10-F10.2-012 · Leader pending-approval widget deep-links to approval screen
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Tapping a pending approval opens the approval screen.
- **Preconditions:** ≥1 pending approval on the leader dashboard.
- **Steps:**
  1. Click a pending approval.
- **Expected result / Acceptance criteria:** Navigates to the approval screen for that request.
- **Traceability:** F10.2, DB-5, AC "Deep link to action".

#### TC-E10-F10.2-013 · Leader of a company with no agents — prompt to place agents
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P2 · **Type:** Empty
- **Objective:** A leader whose company has no placed agents sees a prompt to place agents.
- **Preconditions:** Leader assigned to a company with zero active placements.
- **Steps:**
  1. Open the dashboard.
- **Expected result / Acceptance criteria:** Empty state prompting to place agents (links to E3); no errors.
- **Traceability:** F10.2, C-3.

### Mobile · Shift Leader POV (Beranda — DB-8)

#### TC-E10-F10.2-014 · Leader mobile Beranda = same payload as web team dashboard
- [ ] **Platform:** Mobile · **POV:** shift leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Mobile Beranda shows today's roster status, pending approvals, and schedule alerts for the leader's company — identical data to the web team dashboard.
- **Preconditions:** Same leader logged in on mobile; `GET /dashboards/me` returns `LeaderDashboard` (no separate endpoint).
- **Steps:**
  1. Open Beranda on mobile.
  2. Compare with the web team dashboard (TC-E10-F10.2-010) for the same moment.
- **Expected result / Acceptance criteria:** Beranda shows roster status, pending approvals, and schedule alerts for the company. Data matches the web dashboard (one payload, dual-surface). Mobile frame `.pen` `UMzuO`.
- **Traceability:** F10.2, DB-8, AC "Shift-leader mobile Beranda".

#### TC-E10-F10.2-015 · Beranda scope + deep-links
- [ ] **Platform:** Mobile · **POV:** shift leader · **Priority:** P1 · **Type:** RBAC/Happy
- **Objective:** Beranda is scoped to the leader's company and its items deep-link into the mobile flows.
- **Preconditions:** Leader of Plaza Senayan; other companies have data.
- **Steps:**
  1. Open Beranda; confirm only Plaza Senayan data.
  2. Tap a pending approval → expect the mobile approval screen.
- **Expected result / Acceptance criteria:** Only own-company data; deep-links navigate correctly.
- **Traceability:** F10.2, DB-1, DB-5, DB-8, INV-3.

### Mobile · Agent POV

#### TC-E10-F10.2-016 · Agent personal dashboard content
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent sees next/upcoming shift, today's clock status, leave balance, pending requests (leave/OT), and recent notifications — for themselves only.
- **Preconditions:** Agent Budi on mobile; has an upcoming shift on 2026-06-17, a clock status, a leave balance, ≥1 pending leave/OT request, and recent notifications.
- **Steps:**
  1. Open the personal dashboard.
- **Expected result / Acceptance criteria:** Widgets show next/upcoming shift, today's clock status, leave balance, pending leave/OT requests, and recent notifications. All scoped to Budi.
- **Traceability:** F10.2, DB-2, DB-1, AC "Agent dashboard".

#### TC-E10-F10.2-017 · Agent dashboard deep-link to action
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Tapping a pending request/shift navigates to the underlying screen.
- **Preconditions:** Agent has a pending leave request on the dashboard.
- **Steps:**
  1. Tap the pending request widget.
- **Expected result / Acceptance criteria:** Navigates to the leave request detail/status screen.
- **Traceability:** F10.2, DB-5.

#### TC-E10-F10.2-018 · Agent scope — own data only
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Agent dashboard never shows another agent's shifts/balances.
- **Preconditions:** Two agents at the same company.
- **Steps:**
  1. Log in as Budi; open dashboard.
- **Expected result / Acceptance criteria:** Only Budi's data appears; no other agent's shift/clock/leave/OT data.
- **Traceability:** F10.2, DB-1, INV-3.

#### TC-E10-F10.2-019 · New agent with no data — getting-started state
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Empty
- **Objective:** A new agent with no shift/leave/OT data sees a friendly empty/getting-started dashboard.
- **Preconditions:** New agent, not yet placed/scheduled.
- **Steps:**
  1. Open the dashboard.
- **Expected result / Acceptance criteria:** Friendly empty state (e.g., "Belum ada jadwal"); no errors.
- **Traceability:** F10.2, C-1.

#### TC-E10-F10.2-020 · Near-live freshness reflects auto-published/near-live state
- [ ] **Platform:** Mobile · **POV:** agent · **Priority:** P2 · **Type:** Edge
- **Objective:** Dashboard reflects recent changes (E4 publish, E5 clock) within an acceptable small delay.
- **Preconditions:** Agent's schedule just published; agent just clocked in.
- **Steps:**
  1. After the change, refresh/open the dashboard.
- **Expected result / Acceptance criteria:** New shift and updated clock status appear after a small acceptable delay (caching tolerated).
- **Traceability:** F10.2, DB-6, C-4.

---

## F10.3 — Attendance & Billable-Hours Report (Web only)

> Traces: BR-1..BR-7, INV-4, C-1..C-4. Period under test: **June 2026** (2026-06-01 → 2026-06-30).

### Web · HR/Placement Admin POV

#### TC-E10-F10.3-001 · Billable hours per client company for June, grouped by position
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Running the report for one company over June grouped by position returns verified billable hours per agent and position totals.
- **Preconditions:** HR logged in. "Plaza Senayan" has verified attendance on billable codes across several agents and positions in June.
- **Steps:**
  1. Open the report; filter company = Plaza Senayan, period = June 2026, group by = position.
  2. Run.
- **Expected result / Acceptance criteria:** Report renders verified billable hours per agent, subtotaled by position, with grand totals. Only verified records on `is_billable` codes are counted.
- **Traceability:** F10.3, BR-1, BR-2, INV-4, AC "Billable hours per client for a month".

#### TC-E10-F10.3-002 · Only verified + billable records count; unverified/non-billable excluded
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Records that are unverified OR on non-billable codes are excluded from billable totals.
- **Preconditions:** In June for one agent: (a) verified billable record 8h, (b) unverified billable record 8h, (c) verified non-billable record 8h.
- **Steps:**
  1. Run the report for that agent/company/June.
- **Expected result / Acceptance criteria:** Billable total = 8h (only the verified billable record). The unverified record and the non-billable record are NOT in the billable total.
- **Traceability:** F10.3, BR-1, BR-6, INV-4, C-1, AC "Only verified billable codes count".

#### TC-E10-F10.3-003 · Pending (unverified) shown separately, not in billable total
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Unverified records may be surfaced as "pending" but are excluded from billable totals.
- **Preconditions:** Mix of verified and unverified billable records in June.
- **Steps:**
  1. Run the report; inspect the pending column/section.
- **Expected result / Acceptance criteria:** Billable total counts verified only; a "pending" figure (if shown) reflects unverified hours separately and does not roll into billable.
- **Traceability:** F10.3, C-1, BR-6.

#### TC-E10-F10.3-004 · Billable vs payable vs total worked hours distinguished
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Where relevant, the report distinguishes billable, payable, and total worked hours.
- **Preconditions:** Data where billable ≠ payable ≠ total worked (e.g., a payable-but-non-billable code present).
- **Steps:**
  1. Run the report; inspect the columns.
- **Expected result / Acceptance criteria:** Distinct billable, payable, and total-worked figures are shown and reconcile with the underlying records.
- **Traceability:** F10.3, BR-3.

#### TC-E10-F10.3-005 · Group-by axes: agent / company / position / period
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Happy
- **Objective:** The report aggregates correctly by each axis: agent, client company, position (free-text), and period (day/week/month).
- **Preconditions:** Multi-company, multi-position, multi-agent verified billable data in June.
- **Steps:**
  1. Run grouped by agent → verify per-agent totals.
  2. By company → per-company totals.
  3. By position → per-position totals.
  4. By period day/week/month → time-bucketed totals.
- **Expected result / Acceptance criteria:** Each grouping produces correct subtotals/totals consistent with the raw verified-billable data; switching grouping re-aggregates without losing the period/company filter.
- **Traceability:** F10.3, BR-2.

#### TC-E10-F10.3-006 · Hours only — no rates / invoice amounts
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Negative
- **Objective:** The report shows hours, never monetary amounts/rates.
- **Preconditions:** Any populated report.
- **Steps:**
  1. Inspect all columns and totals.
- **Expected result / Acceptance criteria:** No currency/rate/amount columns anywhere; only hours (billing math is outside the system).
- **Traceability:** F10.3, BR-5, AC "Hours only (no amounts)".

#### TC-E10-F10.3-007 · Cross-midnight shift counted once to the start date
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Edge
- **Objective:** A shift spanning midnight is counted once, attributed to the start date.
- **Preconditions:** Verified billable shift 2026-06-17 22:00 → 2026-06-18 06:00 (8h).
- **Steps:**
  1. Run grouped by period=day for 2026-06-17 and 2026-06-18.
- **Expected result / Acceptance criteria:** The full 8h counts under 2026-06-17 only; 2026-06-18 shows 0h from this shift (counted once, to start date — no double counting).
- **Traceability:** F10.3, BR-7, C-4.

#### TC-E10-F10.3-008 · Agent transferred mid-period — hours split per company/placement window
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Edge
- **Objective:** An agent placed at company A then transferred to company B within June has hours split per placement window.
- **Preconditions:** Agent Budi at Plaza Senayan 2026-06-01 → 2026-06-15, transferred to Grand Indonesia 2026-06-16 → 2026-06-30; verified billable records throughout.
- **Steps:**
  1. Run by company for June (all companies).
- **Expected result / Acceptance criteria:** Budi's June hours appear split — pre-2026-06-16 hours under Plaza Senayan, on/after 2026-06-16 under Grand Indonesia — matching the placement windows; no hours dropped or double-counted.
- **Traceability:** F10.3, C-3.

#### TC-E10-F10.3-009 · Re-running after a correction reflects corrected hours; prior export point-in-time
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Edge
- **Objective:** After an E5 correction, re-running reflects corrected hours; previously exported files are unchanged (point-in-time).
- **Preconditions:** A June record corrected after a first report run + export.
- **Steps:**
  1. Run and export the report (snapshot 1).
  2. Apply an E5 correction (F5.4) that changes the hours.
  3. Re-run the report.
- **Expected result / Acceptance criteria:** Re-run shows the corrected hours; snapshot 1's exported file still reflects pre-correction figures (point-in-time).
- **Traceability:** F10.3, C-2, F10.4 EX-6.

#### TC-E10-F10.3-010 · Empty result for a period with no verified billable data
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P2 · **Type:** Empty
- **Objective:** A filter combo with no matching verified billable records shows a friendly empty result, not an error.
- **Preconditions:** Pick a company/period with no verified billable attendance (e.g., a future month or a company with only unverified records).
- **Steps:**
  1. Run the report.
- **Expected result / Acceptance criteria:** Empty-state message (e.g., "Tidak ada data jam tertagih terverifikasi"); totals = 0; export still available but would produce an empty/headers-only file.
- **Traceability:** F10.3, C-1, F10.4 (empty export).

#### TC-E10-F10.3-011 · Large dataset renders/paginates performantly
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P2 · **Type:** Edge
- **Objective:** A report across all companies for a full month with thousands of rows loads and paginates (cursor) acceptably.
- **Preconditions:** 10k+ verified billable records across companies in June.
- **Steps:**
  1. Run with no company filter, June, grouped by agent.
- **Expected result / Acceptance criteria:** Report loads within acceptable time; pagination is cursor-based (no offset on attendance); totals are correct across pages.
- **Traceability:** F10.3, CONVENTIONS §8 (cursor; attendance >100k).

#### TC-E10-F10.3-012 · Loading + error states
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P2 · **Type:** Loading/Error
- **Objective:** Report shows loading state during aggregation and an error+retry on failure.
- **Preconditions:** Ability to throttle/fail the report query.
- **Steps:**
  1. Run under slow network → loading.
  2. Force 500 → error.
- **Expected result / Acceptance criteria:** Loading indicator while aggregating; error state with retry on failure.
- **Traceability:** F10.3, CONVENTIONS §11.

### Web · Super Admin POV

#### TC-E10-F10.3-013 · Super admin runs the report across all companies
- [ ] **Platform:** Web · **POV:** super admin · **Priority:** P1 · **Type:** Happy/RBAC
- **Objective:** Super admin has the same all-company report access as HR.
- **Preconditions:** Super admin logged in; multi-company verified billable data.
- **Steps:**
  1. Run the report with no company filter (all), June.
- **Expected result / Acceptance criteria:** All companies' verified billable hours aggregate correctly; no company restriction.
- **Traceability:** F10.3, BR-4 (HR/Super Admin see all), INV-3.

### Web · Shift Leader POV

#### TC-E10-F10.3-014 · Leader can run the report for own company only
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P0 · **Type:** Happy
- **Objective:** A shift leader can report on their company and sees only that company.
- **Preconditions:** Leader of Plaza Senayan; verified billable data exists for Plaza Senayan and Grand Indonesia.
- **Steps:**
  1. Open the report; observe the company filter.
  2. Run.
- **Expected result / Acceptance criteria:** The company filter is locked/limited to Plaza Senayan; the result contains only Plaza Senayan agents' verified billable hours.
- **Traceability:** F10.3, BR-4, INV-3, AC "Leader sees only their company".

#### TC-E10-F10.3-015 · Leader cannot report on another company (scope denial)
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Attempting to run/request the report for a non-own company is denied.
- **Preconditions:** Plaza Senayan leader; attempt company = Grand Indonesia (via crafted request / URL param).
- **Steps:**
  1. Force the report request with `company_id` = Grand Indonesia.
- **Expected result / Acceptance criteria:** Server rejects with `403` / `OUT_OF_SCOPE` (or returns only own-company data); no Grand Indonesia rows ever appear. No scope escalation.
- **Traceability:** F10.3, BR-4, INV-3, CONVENTIONS §7/§11 (OUT_OF_SCOPE).

### Web · Agent POV

#### TC-E10-F10.3-016 · Agent has no access to the billable report
- [ ] **Platform:** Web · **POV:** agent · **Priority:** P1 · **Type:** RBAC
- **Objective:** Agents cannot access the billable report (no console + no report capability).
- **Preconditions:** Agent credentials.
- **Steps:**
  1. Attempt to reach the report route / call the report endpoint as an agent.
- **Expected result / Acceptance criteria:** Blocked — agent has no web console access and the endpoint denies (403/no-permission). No report data returned.
- **Traceability:** F10.3, §3 Actors (HR/SA/Leader only), CONVENTIONS §17.

---

## F10.4 — Export Framework (Web only)

> Traces: EX-1..EX-6, INV-5, C-1..C-4. Exports honor the calling report's filters/scope; every export is audited.

### Web · HR/Placement Admin POV

#### TC-E10-F10.4-001 · Export billable report to Excel (inline) — content correct
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P0 · **Type:** Happy
- **Objective:** A small filtered billable report exports to xlsx immediately and the file content matches the on-screen report.
- **Preconditions:** Report filtered to Plaza Senayan, June, grouped by position, small result set.
- **Steps:**
  1. Run the report.
  2. Export → format = xlsx.
  3. Download and open the file.
- **Expected result / Acceptance criteria:** xlsx downloads immediately (inline; `200`/`201`). File opens in Excel; columns = agent, company, position, hours (billable/payable/total as on screen), period; rows + totals exactly match the on-screen report. No rate/amount columns (BR-5).
- **Traceability:** F10.4, EX-1, EX-2, AC "Export a report inline".

#### TC-E10-F10.4-002 · Export to PDF — content + layout correct
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Happy
- **Objective:** The same report exports to a readable PDF with correct figures.
- **Preconditions:** Same filtered report.
- **Steps:**
  1. Export → format = pdf; open the file.
- **Expected result / Acceptance criteria:** PDF opens; header shows report name, filters (company/period/grouping), and run timestamp (Asia/Jakarta); table figures and totals match the on-screen report.
- **Traceability:** F10.4, EX-1, EX-6.

#### TC-E10-F10.4-003 · Export to CSV — parseable, correct values
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Happy
- **Objective:** CSV export produces a valid, parseable file with correct values.
- **Preconditions:** Same filtered report.
- **Steps:**
  1. Export → format = csv; open in a spreadsheet / parse.
- **Expected result / Acceptance criteria:** CSV has a header row + data rows; values match; UTF-8, correctly escaped (commas/quotes); numeric hours parse as numbers.
- **Traceability:** F10.4, EX-1.

#### TC-E10-F10.4-004 · Export honors the same filters as the on-screen report
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P0 · **Type:** Happy
- **Objective:** The exported file contains exactly the filtered/grouped data shown on screen — no more, no less.
- **Preconditions:** Report filtered to one company + one position + June.
- **Steps:**
  1. Note the on-screen rows.
  2. Export (any format) and compare.
- **Expected result / Acceptance criteria:** Exported rows == on-screen rows (same filter/scope); no rows from other companies/positions/periods leak in.
- **Traceability:** F10.4, EX-2.

#### TC-E10-F10.4-005 · Every export writes an audit entry
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Each export creates an `ExportJob` and an audit entry (who, report_type, filters, format, time).
- **Preconditions:** Access to E1 audit log.
- **Steps:**
  1. Export the report.
  2. Open the audit log (E1).
- **Expected result / Acceptance criteria:** An audit entry records actor = the HR user, report_type, the exact filters, format, and timestamp; an `ExportJob` (`SWP-EXP-…`) exists with matching metadata.
- **Traceability:** F10.4, EX-4, INV-5, AC "audit entry records who/what/when", CONVENTIONS §16.1.

#### TC-E10-F10.4-006 · Large export is queued + notified when ready (202)
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P0 · **Type:** Edge
- **Objective:** A large result set is queued, returns 202 with a job id, and the requester is notified when the file is ready.
- **Preconditions:** All-company June report with a result set above the queuing threshold.
- **Steps:**
  1. Export the large report.
  2. Observe the immediate response and job status.
  3. Wait for completion.
- **Expected result / Acceptance criteria:** Request returns `202 Accepted` with an export job id; UI shows "queued/processing". When done, the requester receives a notification (F10.1) that the file is ready, with a download link.
- **Traceability:** F10.4, EX-3, CONVENTIONS §7 (202), C (notify via F10.1).

#### TC-E10-F10.4-007 · Export point-in-time — data changes after export do not alter the file
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Edge
- **Objective:** The exported file reflects the data at export time even if the data changes afterward.
- **Preconditions:** Export a report, then change underlying attendance (verify a new record / apply a correction).
- **Steps:**
  1. Export (snapshot).
  2. Change the data.
  3. Re-open the previously downloaded file.
- **Expected result / Acceptance criteria:** The downloaded file is unchanged (point-in-time). A fresh export reflects the new data.
- **Traceability:** F10.4, EX-6, AC "Point-in-time", F10.3 C-2.

#### TC-E10-F10.4-008 · Export fails mid-generation — job failed, requester notified, no partial file
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Error
- **Objective:** If generation fails, the job is marked failed, the requester is notified, and no partial file is served.
- **Preconditions:** Ability to force a generation failure (e.g., storage error) on a queued export.
- **Steps:**
  1. Start an export that fails mid-generation.
  2. Inspect the job status and notifications.
  3. Attempt to download.
- **Expected result / Acceptance criteria:** Job status = failed; requester receives a failure notification; no partial/corrupt file is downloadable.
- **Traceability:** F10.4, C-1.

#### TC-E10-F10.4-009 · Very large PDF — warn / steer to xlsx/csv
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P2 · **Type:** Edge
- **Objective:** For a huge tabular dataset, PDF is warned against and xlsx/csv preferred.
- **Preconditions:** All-company June, thousands of rows; choose PDF.
- **Steps:**
  1. Attempt PDF export of the large dataset.
- **Expected result / Acceptance criteria:** A warning about PDF size appears (suggesting xlsx/csv); the user can proceed or switch format. No silent failure.
- **Traceability:** F10.4, C-2.

#### TC-E10-F10.4-010 · Concurrent exports by one user — each a separate job
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P2 · **Type:** Edge
- **Objective:** A user can launch multiple exports concurrently; each is tracked as a separate job.
- **Preconditions:** HR user; ability to launch 2+ exports quickly.
- **Steps:**
  1. Launch export A (xlsx) and export B (csv) in quick succession.
- **Expected result / Acceptance criteria:** Two distinct `ExportJob` ids; both complete independently; both audited separately.
- **Traceability:** F10.4, C-3, EX-4.

#### TC-E10-F10.4-011 · File retention/expiry — expired file no longer downloadable
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Files expire after the retention window; downloading an expired file fails gracefully.
- **Preconditions:** An export job whose file has passed the retention window (window value is an open §10 decision — verify expiry behavior, not the exact duration).
- **Steps:**
  1. Attempt to download an expired export.
- **Expected result / Acceptance criteria:** Download is refused gracefully (e.g., "file kedaluwarsa / no longer available"); a re-export is offered. No 500.
- **Traceability:** F10.4, C-4, EX-5.

#### TC-E10-F10.4-012 · Download requires auth (files not public)
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** RBAC
- **Objective:** Export file URLs require the caller's Authorization; they are not publicly accessible.
- **Preconditions:** A completed export with a `file_url`.
- **Steps:**
  1. Open the `file_url` without an Authorization header / in a logged-out session.
- **Expected result / Acceptance criteria:** Download is denied (401) without auth; succeeds with the valid token. Files have access control + expiry.
- **Traceability:** F10.4, EX-5, CONVENTIONS §15 (download requires auth).

#### TC-E10-F10.4-013 · Sensitive (payroll) export carries confidentiality marking
- [ ] **Platform:** Web · **POV:** HR/placement admin · **Priority:** P1 · **Type:** Edge
- **Objective:** A payroll-archive (E8) export carries a confidentiality marking and access control.
- **Preconditions:** HR with payroll-archive export rights; an E8 payroll export available.
- **Steps:**
  1. Export a payroll archive.
  2. Open the file.
- **Expected result / Acceptance criteria:** The file shows a confidentiality marking (e.g., "RAHASIA / CONFIDENTIAL"); access-controlled + expiring; audited like any export.
- **Traceability:** F10.4, EX-5, AC "Sensitive export marked".

### Web · Super Admin POV

#### TC-E10-F10.4-014 · Super admin export at global scope
- [ ] **Platform:** Web · **POV:** super admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Super admin can export at all-company scope and the export is audited.
- **Preconditions:** Super admin; all-company report.
- **Steps:**
  1. Export the all-company report (xlsx).
- **Expected result / Acceptance criteria:** File contains all companies' data (global scope); audit entry records actor=super admin + filters/format/time.
- **Traceability:** F10.4, EX-2, EX-4, INV-3/INV-5.

### Web · Shift Leader POV

#### TC-E10-F10.4-015 · Leader export contains only their company's data (no scope escalation)
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** An export by a company-scoped leader contains only their company's rows — exporting cannot escalate scope.
- **Preconditions:** Plaza Senayan leader runs the billable report (own company); chooses export.
- **Steps:**
  1. Export the report (xlsx).
  2. Open the file; inspect all rows.
- **Expected result / Acceptance criteria:** Every row is Plaza Senayan; no Grand Indonesia data. Even a crafted export request with a different `company_id` is rejected (403/`OUT_OF_SCOPE`) — no scope escalation via export.
- **Traceability:** F10.4, EX-2, AC "Export respects scope", INV-3.

#### TC-E10-F10.4-016 · Leader export is audited with their identity + scope
- [ ] **Platform:** Web · **POV:** shift leader · **Priority:** P1 · **Type:** Happy
- **Objective:** A leader's export is audited (who/report_type/filters/format/time) with the company-scope recorded.
- **Preconditions:** Leader exports; HR/super admin can view audit (leader may not).
- **Steps:**
  1. Leader exports the report.
  2. A super admin checks the audit log.
- **Expected result / Acceptance criteria:** Audit entry shows actor=leader, report_type, filters (incl. their company), format, time; an `ExportJob` exists.
- **Traceability:** F10.4, EX-4, INV-5, CONVENTIONS §16.1.

### Web · Agent POV

#### TC-E10-F10.4-017 · Agent cannot request exports
- [ ] **Platform:** Web · **POV:** agent · **Priority:** P1 · **Type:** RBAC
- **Objective:** Agents have no export capability.
- **Preconditions:** Agent credentials.
- **Steps:**
  1. Attempt to call an export endpoint / reach an export UI as an agent.
- **Expected result / Acceptance criteria:** Denied (403/no-permission); no `ExportJob` created. Agents have no web console and no export rights.
- **Traceability:** F10.4, §3 Actors (HR/Leader only), CONVENTIONS §17.

### Mobile · all roles

#### TC-E10-F10.4-018 · No export surface on mobile (report + export are web-only)
- [ ] **Platform:** Mobile · **POV:** agent / shift leader · **Priority:** P2 · **Type:** Negative
- **Objective:** Neither the billable report nor export is offered on mobile.
- **Preconditions:** Mobile app as agent and as shift leader.
- **Steps:**
  1. Inspect the mobile app for any billable-report or export entry point.
- **Expected result / Acceptance criteria:** No report/export UI exists on mobile (heavy tabular report is web-only per PRD §4). Notifications + dashboards are the only E10 mobile surfaces.
- **Traceability:** F10.3 §4, F10.4 §4, FEATURE §6.

---

## 4. Traceability summary

| Item | Covered by |
|---|---|
| **NT-1** push+in-app only | TC-...F10.1-001, -015, -019; INV-1 |
| **NT-2** scope/no leakage | F10.1-003, -013, -026 |
| **NT-3** event catalog | F10.1-004..006, -010..012, -014, -019..024 |
| **NT-4** type/payload/read-state | F10.1-002 |
| **NT-5** preferences/critical override | F10.1-025 |
| **NT-6** best-effort+durable | F10.1-008, -015, -017 |
| **F10.1 C-1..C-4** | -018 (C-1), -024 (C-2), -025 (C-3), -009 (C-4) |
| **§16.2 dispatch rows** | F10.1-004, -005/-014, -006, -010, -011, -012, -019, -020, -021, -022, -023 |
| **DB-1 scope** | F10.2-011, -015, -018 |
| **DB-2 agent dash** | F10.2-016 |
| **DB-3 leader dash** | F10.2-010 |
| **DB-4 HR KPIs** | F10.2-001, -006, -007 |
| **DB-5 deep-link** | F10.2-002, -008, -012, -017 |
| **DB-6 freshness** | F10.2-007, -020 |
| **DB-7 super admin superset** | F10.2-001..004, -006 |
| **DB-8 leader dual-surface** | F10.2-014, -015 |
| **F10.2 C-1..C-7** | -009/-019 (C-1), -007 (C-2), -013 (C-3), -007/-020 (C-4), -004 (C-5), -006 (C-6), -003 (C-7) |
| **BR-1/INV-4 verified+billable** | F10.3-001, -002 |
| **BR-2 group-by** | F10.3-005 |
| **BR-3 billable/payable/total** | F10.3-004 |
| **BR-4 scope** | F10.3-013, -014, -015 |
| **BR-5 hours only** | F10.3-006 |
| **BR-6 exclude unverified** | F10.3-002, -003 |
| **BR-7 audited + cross-midnight** | F10.3-007; F10.4-005 |
| **F10.3 C-1..C-4** | -002/-003/-010 (C-1), -009 (C-2), -008 (C-3), -007 (C-4) |
| **EX-1 formats** | F10.4-001, -002, -003 |
| **EX-2 scope/filters honored** | F10.4-004, -015 |
| **EX-3 queue+notify** | F10.4-006 |
| **EX-4 audit** | F10.4-005, -010, -014, -016 |
| **EX-5 access control/expiry/confidential** | F10.4-011, -012, -013 |
| **EX-6 point-in-time** | F10.4-007 |
| **F10.4 C-1..C-4** | -008 (C-1), -009 (C-2), -010 (C-3), -011 (C-4) |
| **INV-2 internal-only** | F10.1-003; F10.3-016; F10.4-017 (no client surfaces tested) |
| **INV-3 role-scoped** | F10.2-011/-018; F10.3-015; F10.4-015 |
| **INV-5 audited exports** | F10.4-005, -016 |
| **RBAC denials** | F10.1-027; F10.3-015, -016; F10.4-015, -017, -018 |
