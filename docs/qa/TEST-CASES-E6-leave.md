# Test Cases — E6 Leave Management (Manual QA)

> **Epic:** E6 Leave Management · **Status:** Draft v1 · **Generated:** 2026-06-17
> **Scope:** Exhaustive **manual** test cases for the SWP HRIS leave domain — per-type quota ledger, the 18-code `Fitur Ijin` catalog, HR-assigned per-employee entitlements, agent leave requests (mobile), approval via the E11 engine (web + mobile inbox), leave↔schedule/attendance integration, and calendar/balance views.
>
> **Source of truth:** [`E6 FEATURE.md`](../epics/E6-leave/FEATURE.md) and the five F6.# PRDs + [`leave-entitlement-assignment.md`](../epics/E6-leave/prds/leave-entitlement-assignment.md). Error semantics per [`api/CONVENTIONS.md`](../api/CONVENTIONS.md) (403 RBAC, 404 not-visible, 409 INV-conflict, 422 quota/business-rule).

---

## 0. Model reminders that govern the cases

These are the **current, shipped** rules (post 2026-06-15). Older gate language in the PRDs is historical and **must not** be tested as enforced.

1. **Assignment-driven entitlement (ELE-1).** A leave type is requestable **only if HR has assigned it** (an active `employee_leave_entitlement` for that employee+type). The agent picker and balance grid list **only assigned types**.
2. **Eligibility gates are DROPPED (ELE-8 / INV-7 retired).** Gender, advance-notice, min-service, lifetime-once do **NOT** block a request. The Gherkin "Gender gate" / "Notice gate" scenarios in F6.1/F6.2 are **superseded** — see TC-E6-F6.1-GATE-RETIRED below for the regression assertion. The gate **columns** survive as unenforced metadata.
3. **Retained request-time checks:** `INVALID_DATE_RANGE`, `OVERLAPPING_LEAVE` (LR-5), `BACKDATED_LEAVE` (flagged, allowed), `QUOTA_EXCEEDED` (422). `MISSING_REQUIRED_DOCUMENT` is **deferred** until upload lands — `requires_document` is metadata only today; document tests below are marked **(deferred — verify metadata not enforced)**.
4. **`cap_basis` = reset cadence + metering window (kept).** `ANNUAL_POOL` (year, expires 31 Dec, no carryover, entitled from the agreement/entitlement) · `PER_MONTH` (resets monthly) · `PER_YEAR_COUNT` (yearly occurrence count) · `LIFETIME_ONCE` / `SERVICE_UNPAID` (once per employment, never reset) · `PER_EVENT` (no standing row; `duration ≤ cap_value` per occurrence) · `UNCAPPED` (no standing row; doc-bounded).
5. **Reserve / commit / release lifecycle (LQ-12 / LQ-2 / LQ-6).** Submit reserves `pending_days`; **balance only decrements `used_days` on terminal approval**; reject/withdraw/cancel releases the reservation. `remaining = entitled − used − pending`, **never negative** (INV-6).
6. **Approval routing is E11**, not a fixed two-level chain. "Shift leader" approves only when on the company's E11 line. Super admin may bypass. `OnApproved` re-checks remaining, commits, then fires F6.4 integration.
7. **18-code `Fitur Ijin` catalog** (E2 master) used in cases: `CT`/`CTHO` (ANNUAL_POOL), `CH`/`KGD` (PER_MONTH), `STSD` (PER_YEAR_COUNT), `CM`/`CIH`/`CIU`/`CPR` (LIFETIME_ONCE), `CLTP` (SERVICE_UNPAID, unpaid), `CIM`/`CKA`/`CMA`/`CKM`/`CRM` (PER_EVENT), `SDSKD`/`CTN`/`CAP` (UNCAPPED).

---

## 1. Coverage matrix (features × platform × role)

Legend: ✅ has cases · — not applicable on this surface for this role.

| Feature | Surface | Super Admin | HR/Placement Admin | Shift Leader | Agent |
|---|---|---|---|---|---|
| **F6.1** Entitlement ledger / quota | Web | ✅ adjust, bulk grant, bypass | ✅ adjust, bulk grant, view | — (no quota admin) | — |
| **F6.1** Balance line per type | Mobile | — | — | — | ✅ view own |
| **Entitlement assignment (ELE)** | Web | ✅ catalog + assign | ✅ assign per employee | — | — |
| **Entitlement (effect)** | Mobile | — | — | — | ✅ only assigned types appear |
| **F6.2** Request leave | Mobile | — | — | — | ✅ create/withdraw |
| **F6.2** File on behalf | Web | ✅ | ✅ | ✅ (own company) | — |
| **F6.3** Approve/reject (E11 inbox) | Web | ✅ + bypass | ✅ if on line | ✅ if on line | — |
| **F6.3** Approve/reject (E11 inbox) | Mobile | — | — | ✅ if on line | — |
| **F6.3** Watch chain timeline | Mobile | — | — | — | ✅ |
| **F6.4** Schedule/attendance effect | System/Web | ✅ observe | ✅ observe | ✅ sees cleared shifts + uncovered | ✅ sees Leave on schedule |
| **F6.5** Team leave calendar | Web | ✅ all companies | ✅ all companies | ✅ own company | — |
| **F6.5** Team leave calendar | Mobile | — | — | ✅ own company | — |
| **F6.5** Own balance + history | Mobile | — | — | — | ✅ |
| **F6.5** Export | Web | ✅ | ✅ | — | — |

---

# F6.1 — Leave Entitlement Ledger & HR Assignment

## F6.1 · Web · HR/Placement Admin POV

#### TC-E6-F6.1-001 · Assign a new leave type to an employee with a quota
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** HR enrols an employee into a leave type via the "Hak Cuti" section.
- **Preconditions:** Logged in as HR. Employee `SWP-EMP-1042` (Budi) exists, active. `CT` (ANNUAL_POOL) is in the catalog and **not yet** assigned to Budi.
- **Steps:**
  1. Open Budi's employee detail → "Hak Cuti" section.
  2. Click **Tambah Jenis Cuti**.
  3. Select `CT — Cuti Tahunan` from the catalog picker; set `entitled_days = 12`; add note "Sesuai PKWT".
  4. Save.
- **Expected result / AC:** `POST /employees/1042/leave-entitlements` returns 201. A row appears in the grid: type `CT`, entitled 12, active. The assignment is audited with `assigned_by` = current HR user. The type now becomes requestable by Budi (ELE-1) and will appear in his mobile picker.
- **Traceability:** ELE-1, ELE-2, ELE-6, §8 API, INV (audit).

#### TC-E6-F6.1-002 · Edit base entitled_days affects future windows
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Editing the base entitlement changes future-period windows, not the live `used`/`pending`.
- **Preconditions:** Budi has `CT` assigned, `entitled_days = 12`. Current 2026 window already open with used=2, pending=0.
- **Steps:**
  1. In "Hak Cuti", change Budi's `CT` base entitled_days from 12 to 14.
  2. Save (PATCH).
- **Expected result / AC:** `PATCH /employees/1042/leave-entitlements/CT` returns 200. The **base** is now 14; the 2027 window (when it opens) will be 14. The **current 2026** window `entitled` is unchanged by a base edit (a current-window one-off uses `:adjust-entitled`, TC-005). Audited (ELE-4).
- **Traceability:** ELE-4, ELE-5.

#### TC-E6-F6.1-003 · Remove a type from an employee (deactivate)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Removing a type sets `active=false`; history retained.
- **Preconditions:** Budi has `CIM` (PER_EVENT) assigned and active; one prior approved `CIM` request exists.
- **Steps:**
  1. In "Hak Cuti", click remove on the `CIM` row; confirm.
- **Expected result / AC:** `DELETE /employees/1042/leave-entitlements/CIM` returns 204. Row shows as inactive/removed; `active=false` (soft). `CIM` disappears from Budi's mobile picker and balance grid. The prior request and any existing windows are **retained**. Audited.
- **Traceability:** ELE-6.

#### TC-E6-F6.1-004 · Re-add a previously removed type reactivates the row
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Re-adding the same type reactivates rather than duplicating (unique on employee+type where not deleted).
- **Preconditions:** Budi's `CIM` entitlement is `active=false` (from TC-003).
- **Steps:**
  1. Click **Tambah Jenis Cuti**, pick `CIM` again, set entitled (or toggle on), save.
- **Expected result / AC:** The existing entitlement row is **reactivated** (`active=true`), not a duplicate. No unique-constraint error. Audited.
- **Traceability:** ELE-6, §4.1 unique constraint.

#### TC-E6-F6.1-005 · Adjust the current window one-off (LQ-6)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** HR adjusts the live 2026 quota with a required remark; audited.
- **Preconditions:** Budi's `CT` 2026 window: entitled 12, used 2, pending 0.
- **Steps:**
  1. Open the `CT` 2026 quota row → **Sesuaikan Kuota** modal.
  2. Set entitled to 15; enter remark "Bonus loyalitas 2026".
  3. Save.
- **Expected result / AC:** `POST /leave-quotas:adjust-entitled` returns 200. Current-window `entitled = 15`; remaining = 15 − 2 − 0 = 13. Audited with the remark. **Base** entitlement unchanged (still drives future windows).
- **Traceability:** LQ-6, ELE-5.

#### TC-E6-F6.1-006 · Adjustment below used+pending is blocked (no-negative)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Cannot drop `entitled` below `used + pending` (INV-6).
- **Preconditions:** Budi's `CT` 2026 window: entitled 12, used 9, pending 0.
- **Steps:**
  1. Open the adjust modal; set entitled to 8; remark "test"; save.
- **Expected result / AC:** Request blocked with **422** and message indicating 8 < used+pending (9). Quota unchanged (still 12). Per Gherkin "HR adjusts a quota (no negative)".
- **Traceability:** LQ-6, INV-6, ELE-9, F6.1 Gherkin.

#### TC-E6-F6.1-007 · Adjustment requires a remark
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Remark is mandatory on a quota adjustment.
- **Preconditions:** Any open quota.
- **Steps:**
  1. Open the adjust modal; change entitled; leave remark empty; attempt save.
- **Expected result / AC:** Client validation blocks save (remark required); if forced, server returns **400/422**. No change persisted.
- **Traceability:** LQ-6.

#### TC-E6-F6.1-008 · Bulk annual grant — "Terbitkan Kuota Tahunan" with preview
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** HR triggers the annual ANNUAL_POOL grant for a period; preview shows affected count before commit.
- **Preconditions:** Several agents have `annual_leave_entitlement_days` set on their agreements and `CT` assigned.
- **Steps:**
  1. Open **Terbitkan Kuota Tahunan**; select period 2026; default entitlement per type; enable pro-rata.
  2. Review the **preview count** (N employees affected).
  3. Confirm.
- **Expected result / AC:** One `ANNUAL_POOL` `LeaveQuota` per eligible employee for `period_key = 2026`, `entitled = annual_leave_entitlement_days`, `expires_at = 2026-12-31`, `source = AUTO`. Re-running is idempotent/repair (no duplicate rows — unique on emp+type+period). Audited.
- **Traceability:** LQ-1, LQ-7, FEATURE §7 quota grant UX.

#### TC-E6-F6.1-009 · Pro-rated annual grant for a mid-year joiner
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** ANNUAL_POOL pro-rates for a joiner; statutory caps unaffected.
- **Preconditions:** Agent joined 2026-07-01 (6 months remaining), annual entitlement 12.
- **Steps:**
  1. Run the annual grant for 2026 with pro-rata on (or auto-grant on hire).
- **Expected result / AC:** `entitled ≈ 12 × 6/12 = 6` (half-up rounding). PER_MONTH/PER_EVENT/etc. windows are **not** pro-rated. Audited.
- **Traceability:** LQ-8, C-1.

#### TC-E6-F6.1-010 · View the per-type ledger for one employee
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** HR sees a line per assigned type with entitled/used/pending/remaining + window label.
- **Preconditions:** Budi assigned `CT` (ANNUAL_POOL), `CKM` (PER_EVENT), `SDSKD` (UNCAPPED).
- **Steps:**
  1. Open `GET /leave-balances/by-employee/1042/types`.
- **Expected result / AC:** Only **assigned** types returned. `CT` shows remaining + "Tahunan · hangus 31 Des 2026". `CKM` shows per-occurrence cap (no remaining count). `SDSKD` shows "Sesuai ketentuan" (no quota). Removed/inactive types excluded.
- **Traceability:** ELE-1, §4 Balance, §7.1 table.

## F6.1 · Web · Super Admin POV

#### TC-E6-F6.1-011 · Super admin manages the leave-type catalog (CRUD)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Super admin can add/edit/soft-delete a leave type in the catalog.
- **Preconditions:** Logged in as super admin.
- **Steps:**
  1. Add a new type `XTEST` with `cap_basis=PER_EVENT`, `cap_value=2`.
  2. Edit its name.
  3. Soft-delete it.
- **Expected result / AC:** Catalog reflects each change. Soft-deleted type is hidden everywhere; existing entitlements referencing it are treated as inactive (ELE-7). Audited.
- **Traceability:** ELE-7.

#### TC-E6-F6.1-012 · Soft-deleting a catalog type hides it from all pickers
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** A soft-deleted type vanishes from HR assign UI and agent picker even if entitlements still reference it.
- **Preconditions:** `CRM` assigned to ≥1 agent; soft-delete `CRM`.
- **Steps:**
  1. Soft-delete `CRM`. Reload an agent's picker.
- **Expected result / AC:** `CRM` not selectable in HR assign UI nor agent picker. Existing `CRM` entitlements treated as inactive.
- **Traceability:** ELE-7.

#### TC-E6-F6.1-GATE-RETIRED · Eligibility gates are NOT enforced (regression)
- [ ] **Platform:** Web/Mobile · **POV:** Agent/HR · **Priority:** P0 · **Type:** Negative (regression)
- **Objective:** Confirm the retired gates do not block: a male agent assigned `CH` (gender FEMALE metadata) can still request it; a `notice_days=30` type can be requested 3 days out; a 0-tenure agent can request a `min_service_years` type; no `GENDER_MISMATCH`/`INSUFFICIENT_NOTICE`/`INSUFFICIENT_SERVICE`/`ALREADY_USED_LIFETIME` error appears.
- **Preconditions:** Male agent has `CH` assigned by HR. `CIU` (notice_days=30) assigned, start in 3 days.
- **Steps:**
  1. As the male agent, request `CH` 1 day; submit.
  2. Request `CIU` starting in 3 days; submit.
- **Expected result / AC:** Both submit successfully (subject only to quota/date/overlap). **No** gender/notice/service block. The old F6.1/F6.2 "Gender gate" and "Notice gate" Gherkin scenarios are superseded.
- **Traceability:** ELE-8, LQ-15, LR-3b, INV-7 retired.

## F6.1 · Mobile · Agent POV

#### TC-E6-F6.1-013 · Agent balance grid lists only assigned types
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** `Pengajuan › Cuti` table shows only HR-assigned types with the right columns.
- **Preconditions:** Budi assigned `CT`, `CKM`, `SDSKD`. Catalog has 18 types total.
- **Steps:**
  1. Open the leave/`Cuti` tab.
- **Expected result / AC:** Table columns **Jenis Cuti · Sisa · Terpakai · Pending · Kuota · Reset/Kedaluwarsa**. Exactly 3 rows (assigned types). `CT`: Sisa = remaining, "Tahunan · hangus 31 Des 2026". `CKM`: "Per kejadian". `SDSKD`: Sisa = "Sesuai ketentuan". No unassigned types shown.
- **Traceability:** ELE-1, §7.1.

#### TC-E6-F6.1-014 · Pending reservation is reflected in Sisa
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** A pending request reduces displayed remaining (reserve counts against remaining).
- **Preconditions:** Budi `CT` entitled 12, used 0, pending 0 (Sisa 12). Submit a 3-day CT request (TC-F6.2-001).
- **Steps:**
  1. After submitting, return to the balance table.
- **Expected result / AC:** `CT` row: Pending = 3, Sisa = 9 (12 − 0 − 3). Terpakai still 0 (only commits on approval).
- **Traceability:** LQ-12, §0.5.

#### TC-E6-F6.1-015 · Balance loading state
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Loading skeleton then data.
- **Preconditions:** Throttle network.
- **Steps:** 1. Open the `Cuti` tab on a slow connection.
- **Expected result / AC:** Skeleton rows shown while fetching; resolves to the table; no flash of empty state.
- **Traceability:** §7.1, no-dead-flow.

#### TC-E6-F6.1-016 · Agent with no assigned types — empty state
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** Agent HR hasn't enrolled sees a clear empty state, not an error.
- **Preconditions:** New agent, no entitlements assigned.
- **Steps:** 1. Open the `Cuti` tab.
- **Expected result / AC:** Empty state ("Belum ada hak cuti / hubungi HR"); the "Ajukan Cuti" CTA either hidden or disabled with the picker empty (no requestable types). No crash.
- **Traceability:** ELE-1, no-dead-flow.

---

# F6.2 — Leave Request (documents, delegate)

## F6.2 · Mobile · Agent POV — Happy paths per cap_basis

#### TC-E6-F6.2-001 · Submit ANNUAL_POOL (CT) request within cap
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent submits a 3-day annual leave within remaining; reservation created.
- **Preconditions:** Budi `CT` 2026 remaining 9 (entitled 12, used 3). `CT` assigned.
- **Steps:**
  1. Tap **Ajukan Cuti**; pick `CT`; range 2026-07-06 → 2026-07-08 (3 working days, no public holiday); reason "Liburan keluarga".
  2. Skip delegate; submit.
- **Expected result / AC:** `duration_days = 3` computed; request created `Pending`; **3 reserved** as `pending_days` (remaining → 6); status timeline shows "Menunggu persetujuan"; shift leader / E11 line-1 notified. `used_days` unchanged. 201.
- **Traceability:** LR-1, LR-3, LR-6, LQ-12, BR §0.5, F6.2 Gherkin "Submit an annual leave".

#### TC-E6-F6.2-002 · Submit PER_EVENT (CKM bereavement) within occurrence cap
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** PER_EVENT request ≤ cap_value submits with no standing quota row.
- **Preconditions:** `CKM` PER_EVENT cap 2, assigned to Budi.
- **Steps:**
  1. Request `CKM` 2026-06-20 → 2026-06-21 (2 days); submit.
- **Expected result / AC:** `duration_days = 2 ≤ cap 2`; created Pending; **no** `LeaveQuota` row created/reserved (PER_EVENT holds none). Annual `CT` remaining untouched.
- **Traceability:** LR-3, LQ-13, LQ-7, INV-1, F6.2 Gherkin "Statutory leave does not consume annual quota".

#### TC-E6-F6.2-003 · Submit PER_MONTH (CH) within the month window
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** PER_MONTH window auto-opens at cap_value on first use, reserves.
- **Preconditions:** Female agent Sari assigned `CH` PER_MONTH cap 2; no June window yet.
- **Steps:**
  1. Request `CH` 2026-06-09 → 2026-06-09 (1 day); submit.
- **Expected result / AC:** A `CH` 2026-06 window auto-opens at entitled=2 (`source=AUTO`); 1 day reserved (remaining 1). Pending.
- **Traceability:** LQ-14, LQ-13, ELE-2, F6.1 Gherkin "PER_MONTH cap resets".

#### TC-E6-F6.2-004 · Submit PER_YEAR_COUNT (STSD) — one occurrence charged
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** PER_YEAR_COUNT charges 1 occurrence regardless of multi-day duration.
- **Preconditions:** `STSD` PER_YEAR_COUNT cap 5 (occurrences) assigned; 2026 window open, used 0.
- **Steps:**
  1. Request `STSD` 2026-08-01 → 2026-08-02 (2 days); submit.
- **Expected result / AC:** 1 occurrence reserved (not 2); `duration_days=2` recorded on the request; remaining occurrences → 4 pending. 
- **Traceability:** LQ-13, F6.1 Gherkin "PER_YEAR_COUNT charges occurrences".

#### TC-E6-F6.2-005 · Submit LIFETIME_ONCE (CM own marriage)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** LIFETIME_ONCE opens a one-time EMP-window and reserves.
- **Preconditions:** `CM` LIFETIME_ONCE cap 3 assigned; no prior CM request.
- **Steps:** 1. Request `CM` 3 days; submit.
- **Expected result / AC:** EMP-window opens at 3, reserves 3 (remaining 0 while pending). `period_key = EMP`. Pending.
- **Traceability:** LQ-13, LQ-14.

#### TC-E6-F6.2-006 · Submit UNCAPPED (SDSKD sick-with-letter) — no day cap
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy / (deferred doc)
- **Objective:** UNCAPPED submits with no quota and no day cap; document gate is metadata-only today.
- **Preconditions:** `SDSKD` UNCAPPED, `requires_document=true` assigned.
- **Steps:** 1. Request `SDSKD` 4 days **without** attaching a document; submit.
- **Expected result / AC:** Submission **succeeds** (document enforcement deferred); no quota row; `duration_days=4` recorded for reporting. **(When upload lands, this must flip to `MISSING_REQUIRED_DOCUMENT` block — track as a separate future case.)**
- **Traceability:** LQ-13, C-3, §0.3, ELE-8 retained checks (doc deferred), F6.2 Gherkin "Document required" (currently unenforced).

#### TC-E6-F6.2-007 · Submit SERVICE_UNPAID (CLTP) — unpaid flag carried
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Happy
- **Objective:** Unpaid leave type submits; `paid=false` recorded for payroll.
- **Preconditions:** `CLTP` SERVICE_UNPAID, `paid=false`, assigned.
- **Steps:** 1. Request `CLTP` 5 days; submit.
- **Expected result / AC:** EMP-window opens once; reserves 5; the approved days will be flagged **unpaid** to payroll (E8); metering unchanged.
- **Traceability:** LQ-16, LQ-13.

#### TC-E6-F6.2-008 · Name a delegate
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Happy
- **Objective:** Optional delegate recorded; informational only.
- **Preconditions:** Budi requesting `CT`; agent Citra exists.
- **Steps:** 1. In the request, pick Citra as delegate; submit.
- **Expected result / AC:** `delegate_id = Citra`. No coverage enforced. Delegate surfaces later as a non-binding suggestion to the leader (F6.5 LV-8).
- **Traceability:** LR-4, F6.2 Gherkin "Name a delegate".

#### TC-E6-F6.2-009 · Withdraw a pending request releases the reservation
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent withdraws before final approval; `Cancelled`; pending released.
- **Preconditions:** Budi has a Pending `CT` request reserving 3 days (remaining was 9 → 6).
- **Steps:** 1. Open the request; tap **Tarik Pengajuan**; confirm.
- **Expected result / AC:** Status → `Cancelled`; 3 `pending_days` released; remaining back to 9; E11 instance closed. Audited.
- **Traceability:** LR-7, LA-6, LQ-12, F6.2 Gherkin "Withdraw".

#### TC-E6-F6.2-010 · Single-day request
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** start = end → duration 1.
- **Steps:** 1. Request `CT` 2026-06-25 → 2026-06-25; submit.
- **Expected result / AC:** `duration_days = 1`; reserves 1.
- **Traceability:** C-2, LR-1.

#### TC-E6-F6.2-011 · Backdated sick request is allowed and flagged
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** A start_date before today is permitted (per type) and flagged `BACKDATED_LEAVE`.
- **Preconditions:** Today 2026-06-17. `SDSKD` assigned.
- **Steps:** 1. Request `SDSKD` 2026-06-15 → 2026-06-16; submit.
- **Expected result / AC:** Submission succeeds; request flagged backdated; no notice block. (If the day was already worked, F6.4 conflict-flag applies on approval — see TC-F6.4-005.)
- **Traceability:** LR-8, C-1, §0.3.

#### TC-E6-F6.2-012 · Duration excludes public holidays
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Working-day duration excludes a public holiday inside the range.
- **Preconditions:** A public holiday on 2026-06-26 (global). `CT` assigned.
- **Steps:** 1. Request `CT` 2026-06-25 → 2026-06-29 (range spans a holiday); submit.
- **Expected result / AC:** `duration_days` excludes the holiday (per EPICS §8 "excluding public holidays"). Reserved days = working days only.
- **Traceability:** FEATURE §7 "Duration = working days excluding public holidays", LR-1, open Q1.

## F6.2 · Mobile · Agent POV — Negative / blocks

#### TC-E6-F6.2-013 · Over-cap ANNUAL_POOL is blocked (QUOTA_EXCEEDED), nothing reserved
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Negative
- **Objective:** Requesting more than remaining is blocked with 422; no reservation made.
- **Preconditions:** Budi `CT` remaining 9.
- **Steps:** 1. Request `CT` 12 days; submit.
- **Expected result / AC:** **422 `QUOTA_EXCEEDED`**, message "Sisa cuti tidak cukup"; **nothing reserved** (pending unchanged, remaining stays 9). Agent told HR may adjust the quota.
- **Traceability:** LR-3, INV-1, INV-6, LQ-5, F6.2 Gherkin "Block an annual request over the cap".

#### TC-E6-F6.2-014 · PER_EVENT over occurrence cap blocked
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Negative
- **Objective:** CKM 3 days (cap 2) blocked; no quota row created.
- **Preconditions:** `CKM` PER_EVENT cap 2 assigned.
- **Steps:** 1. Request `CKM` 3 days; submit.
- **Expected result / AC:** **422** over per-occurrence cap; no quota row; nothing reserved.
- **Traceability:** LR-3, LQ-13, C-4, F6.2 Gherkin "Block a per-event request over its cap".

#### TC-E6-F6.2-015 · PER_YEAR_COUNT exhausted blocks the next occurrence
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Negative
- **Objective:** 6th STSD in a year (cap 5) blocked.
- **Preconditions:** 5 approved STSD occurrences exist for 2026 (count = 5/5).
- **Steps:** 1. Request a 6th STSD in 2026; submit.
- **Expected result / AC:** **422** count exhausted; nothing reserved.
- **Traceability:** LQ-13, F6.1 Gherkin "PER_YEAR_COUNT charges occurrences".

#### TC-E6-F6.2-016 · LIFETIME_ONCE already used blocks resubmission
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Negative
- **Objective:** A second CM request after the once-window exhausted is blocked.
- **Preconditions:** Budi has an approved `CM` (3/3 used); EMP-window exhausted.
- **Steps:** 1. Request `CM` again; submit.
- **Expected result / AC:** **422** — already used / no remaining (emerges from exhausted LIFETIME_ONCE window, **not** an explicit gate). HR override only via quota adjust + reason.
- **Traceability:** LQ-13, INV-7 (emergent), C-4, F6.1 Gherkin "LIFETIME_ONCE exhausts".

#### TC-E6-F6.2-017 · Overlapping request blocked (LR-5)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Negative
- **Objective:** Cannot overlap an existing non-rejected leave.
- **Preconditions:** Budi has a Pending leave 2026-06-10 → 2026-06-12.
- **Steps:** 1. Request leave 2026-06-11 → 2026-06-13; submit.
- **Expected result / AC:** **409/422 `OVERLAPPING_LEAVE`**; blocked; nothing reserved for the new request.
- **Traceability:** LR-5, F6.2 Gherkin "Prevent overlapping requests".

#### TC-E6-F6.2-018 · Invalid date range (end before start) blocked
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Negative
- **Objective:** end_date < start_date rejected.
- **Steps:** 1. Request `CT` 2026-07-10 → 2026-07-08; submit.
- **Expected result / AC:** **400/422 `INVALID_DATE_RANGE`**; client-side guard ideally prevents picking it.
- **Traceability:** ELE-8 retained checks, LR-1.

#### TC-E6-F6.2-019 · Unassigned type is not requestable (ELE-1)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC/Negative
- **Objective:** A type HR has not assigned does not appear in the picker and cannot be requested.
- **Preconditions:** `CIH` (Hajj) **not** assigned to Budi.
- **Steps:** 1. Open the request picker.
- **Expected result / AC:** `CIH` absent from the picker. A crafted API submit for `CIH` returns **422/403** (no active entitlement). 
- **Traceability:** ELE-1, LR-3b.

#### TC-E6-F6.2-020 · PER_MONTH request spanning a month boundary blocked (v1)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** A PER_MONTH request crossing month-end is blocked in v1.
- **Preconditions:** `CH` PER_MONTH assigned.
- **Steps:** 1. Request `CH` 2026-06-30 → 2026-07-01; submit.
- **Expected result / AC:** **422** — must fall within one window (no cross-month split in v1).
- **Traceability:** F6.1 C-2, F6.2 C-3.

#### TC-E6-F6.2-021 · Submit failure network error
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** A network failure on submit shows an error toast and does not leave a phantom reservation.
- **Steps:** 1. Disable network mid-submit.
- **Expected result / AC:** Error surfaced; form retained; no Pending request created; no reservation on the quota.
- **Traceability:** no-dead-flow, LQ-12 atomicity.

## F6.2 · Web · HR / Shift Leader POV — file on behalf

#### TC-E6-F6.2-022 · HR files a leave on behalf of an agent
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** HR creates a request for an agent (same rules apply).
- **Preconditions:** HR logged in; target agent has `CT` assigned, remaining 9.
- **Steps:** 1. From the agent context, file a 2-day CT request; submit.
- **Expected result / AC:** Pending request created in the agent's name; 2 days reserved; E11 instance created for the agent's company. Audited as filed-on-behalf.
- **Traceability:** LR §3 actors, LR-6.

#### TC-E6-F6.2-023 · Shift leader files on behalf only for own company
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** Leader may file on behalf for an agent in their company; denied for another company.
- **Preconditions:** Leader of Plaza Senayan. Agent A in Plaza Senayan; agent B in another company.
- **Steps:** 1. File on behalf for A (allowed). 2. Attempt to file for B.
- **Expected result / AC:** A succeeds. B returns **403/404** (out of scope). 
- **Traceability:** LR §3, LV-1 scope, CONVENTIONS 403/404.

---

# F6.3 — Leave Approval (via the E11 engine)

## F6.3 · Web · Shift Leader POV (E11 inbox)

#### TC-E6-F6.3-001 · Line member approves; chain advances
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** First-line approval advances to the next line; balance NOT yet committed.
- **Preconditions:** Budi's Pending 3-day `CT` request. Plaza Senayan template: line 1 [Sari=shift leader], line 2 [HR]. Sari logged in. 3 days reserved.
- **Steps:**
  1. Open **Kotak Masuk** → the leave instance.
  2. Approve line 1 with no/optional comment.
- **Expected result / AC:** Line 1 cleared; instance advances to line 2; `LeaveRequest.status` still `Pending`. **`used_days` unchanged** (commit only at terminal). `approval_actions` appends Sari's approve. Agent notified of progress.
- **Traceability:** LA-1, LA-7, F6.3 Gherkin "Chain approves", §0.5 (commit only on terminal).

#### TC-E6-F6.3-002 · Terminal approval commits the quota and fires integration
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Last line approves → OnApproved re-checks, commits pending→used, triggers F6.4.
- **Preconditions:** From TC-001, instance at line 2 [HR]; HR logged in; `CT` remaining was 9 with 3 pending.
- **Steps:** 1. Approve line 2 (terminal).
- **Expected result / AC:** Status → `APPROVED`. **Quota commits: `pending_days −3`, `used_days +3`** (remaining = 12 − 3 − 0 = stays as before reservation, i.e. 6 if used was already 3 → now used 6). F6.4 integration fires (shifts → Leave, absent suppressed). Agent notified. `approval_actions` appends.
- **Traceability:** LA-4, LQ-2, INV-2, INV-3, F6.3 Gherkin "Chain approves, side-effects fire".

#### TC-E6-F6.3-003 · Reject releases the reservation
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy/Negative
- **Objective:** A current-line reject terminates the instance and releases pending.
- **Preconditions:** Budi's Pending `CT` request, 3 days reserved, remaining 6. Sari on the current line.
- **Steps:** 1. Open the instance; **Reject** with reason "Coverage tidak ada".
- **Expected result / AC:** Status → `REJECTED`; **3 pending_days released** (remaining back to 9); **no quota consumed**; no schedule change; agent notified with the reason. `approval_actions` records the reject + reason.
- **Traceability:** LA-6, LQ-12, INV-4, F6.3 Gherkin "Reject releases the reservation".

#### TC-E6-F6.3-004 · Reject requires a reason
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Negative
- **Objective:** Reason mandatory on reject.
- **Steps:** 1. Reject without a reason.
- **Expected result / AC:** Blocked (reason required); no state change until a reason is given.
- **Traceability:** LA-7 (reason recorded).

#### TC-E6-F6.3-005 · Self-approval blocked (E11 INV-3)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Negative/RBAC
- **Objective:** A line member who filed their own leave cannot clear their own line.
- **Preconditions:** Sari (shift leader, on line 1) files her own leave; she's on its chain.
- **Steps:** 1. Sari opens her own leave instance and attempts to approve line 1.
- **Expected result / AC:** Approve action disabled/blocked for Sari on that line; another member must clear it (or super-admin bypass). 
- **Traceability:** LA-1 (E11 INV-3), F6.3 Gherkin "Cannot self-approve".

#### TC-E6-F6.3-006 · Balance re-check fails at finalize → instance stays, flagged
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** If remaining dropped below the request since submit, the OnApproved hook fails the transaction.
- **Preconditions:** Budi's `CT` 3-day request at the final line; meanwhile his window was reduced (or another approval consumed it) so remaining < 3.
- **Steps:** 1. Approve the final line.
- **Expected result / AC:** OnApproved **fails the transaction (EX-9)**; status stays at its line, **flagged insufficient**; no commit, no integration. HR must adjust the quota (LQ-6) or a super admin bypasses. Agent not notified as approved.
- **Traceability:** LA-5, F6.1 C-5, F6.3 Gherkin "Balance re-check fails at finalize".

#### TC-E6-F6.3-007 · Concurrent finalize race — second blocks
- [ ] **Platform:** Web · **POV:** HR/Super Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Two requests racing the same window; reserve/commit atomic; the second re-check blocks if exhausted.
- **Preconditions:** Window remaining = 2; two pending requests each for 2 days at finalize.
- **Steps:** 1. Approve both nearly simultaneously.
- **Expected result / AC:** One commits (used += 2); the other's re-check finds 0 remaining → fails the transaction (flagged). No negative balance.
- **Traceability:** F6.1 C-5, LQ-5, INV-6.

#### TC-E6-F6.3-008 · PER_EVENT request approval — nothing to commit
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Approving a PER_EVENT/UNCAPPED leave runs the chain and integration but commits no quota.
- **Preconditions:** Budi's Pending `CKM` (PER_EVENT, 2 days) at the final line.
- **Steps:** 1. Approve terminally.
- **Expected result / AC:** Status APPROVED; **no quota row to commit**; F6.4 integration still fires (shifts on those 2 days → Leave). 
- **Traceability:** LA-4, C-2, LQ-13.

#### TC-E6-F6.3-009 · Inbox empty state
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** No pending instances → empty state.
- **Steps:** 1. Open Kotak Masuk with no pending leave.
- **Expected result / AC:** Empty pattern ("Tidak ada pengajuan"), not an error.
- **Traceability:** no-dead-flow.

## F6.3 · Mobile · Shift Leader POV

#### TC-E6-F6.3-010 · Leader approves from the mobile inbox
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** Approve/reject the current line from mobile Kotak Masuk.
- **Preconditions:** Sari is on the current line of a pending leave; logged in on mobile.
- **Steps:** 1. Open the mobile inbox; open the leave; approve (or reject with reason).
- **Expected result / AC:** Same effect as web: chain advances / commits at terminal / releases on reject. Parity with TC-001/002/003.
- **Traceability:** F6.3 §4 (web/mobile inbox), LA-1.

## F6.3 · Web · Super Admin POV

#### TC-E6-F6.3-011 · Super-admin bypass force-approves; hook still runs
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Happy/Edge
- **Objective:** Super admin bypasses the chain; OnApproved still commits + integrates.
- **Preconditions:** A pending leave at any line; super admin logged in; 3 days reserved.
- **Steps:** 1. Use the bypass/force-approve action with a reason.
- **Expected result / AC:** Status APPROVED via bypass; **OnApproved runs**: commit pending→used, F6.4 integration. `approval_actions` records the bypass actor + reason. 
- **Traceability:** LA-4, C-4 (E11 INV-5), F6.3 §4.

#### TC-E6-F6.3-012 · No template → super-admin fallback (never auto-approve)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Company with no E11 template routes to the super-admin fallback line.
- **Preconditions:** Budi's company has no approval template.
- **Steps:** 1. Budi submits leave (mobile). 2. Super admin checks the inbox.
- **Expected result / AC:** Request routes to the super-admin fallback line; **never auto-approved**, never blocked. Reservation held pending.
- **Traceability:** LA-3, F6.3 Gherkin "No template falls back".

#### TC-E6-F6.3-013 · Template edited mid-chain resets to line 1
- [ ] **Platform:** Web · **POV:** Super Admin/HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Editing the company template while a request is mid-chain resets the instance to line 1; reservation untouched.
- **Preconditions:** A leave instance at line 2; the template is then edited.
- **Steps:** 1. Edit the company's approval template. 2. Re-open the instance.
- **Expected result / AC:** Instance resets to line 1 on the new chain (E11 INV-6); the reservation (pending_days) is untouched / still pending. 
- **Traceability:** C-5, LA-1.

## F6.3 · Mobile · Agent POV

#### TC-E6-F6.3-014 · Agent watches the chain-progress timeline
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Agent sees each line's status/decision on the request timeline.
- **Preconditions:** Budi has a request mid-chain (line 1 approved, line 2 pending).
- **Steps:** 1. Open the request detail.
- **Expected result / AC:** Timeline shows line 1 approved (actor/time), line 2 pending; status `Pending`. On terminal it shows APPROVED/REJECTED with reason. Notifications match each step.
- **Traceability:** F6.3 §4, LA-7.

#### TC-E6-F6.3-015 · Agent withdraws mid-chain
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Withdraw before finalize cancels and releases.
- **Preconditions:** Budi's request at line 2.
- **Steps:** 1. Tap **Tarik Pengajuan**; confirm.
- **Expected result / AC:** Status → Cancelled; reservation released; E11 instance closed (E11 C-1); leader notified.
- **Traceability:** C-1, LA-6, LR-7.

---

# F6.4 — Leave–Schedule/Attendance Integration

## F6.4 · System / Web · Shift Leader & Agent POV

#### TC-E6-F6.4-001 · Approval clears overlapping shifts (→ Leave)
- [ ] **Platform:** Web/System · **POV:** Shift Leader (observer) · **Priority:** P0 · **Type:** Happy
- **Objective:** On terminal approval, overlapping E4 schedule entries are marked Leave.
- **Preconditions:** Budi has shifts 2026-06-10, 11, 12. A `CT` leave 2026-06-10→12 reaches terminal approval.
- **Steps:** 1. Approve terminally. 2. Open Budi's E4 schedule for those days (leader view).
- **Expected result / AC:** Those 3 schedule entries show status **Leave** (or cleared). The dates are tagged so E5 won't mark Absent. Audited; agent + leader notified.
- **Traceability:** LI-1, LI-2, LI-5, INV-3, F6.4 Gherkin "Approval clears overlapping shifts".

#### TC-E6-F6.4-002 · Leave day produces no Absent record
- [ ] **Platform:** System · **POV:** Agent (effect) · **Priority:** P0 · **Type:** Happy
- **Objective:** The attendance end-of-day job does not create Absent on a leave-covered date.
- **Preconditions:** 2026-06-10 covered by approved leave; agent does not clock in.
- **Steps:** 1. Run/await the E5 end-of-day evaluation for 2026-06-10.
- **Expected result / AC:** **No Absent record** for Budi on 2026-06-10; the day reads as Leave in attendance.
- **Traceability:** LI-2, F6.4 Gherkin "Leave day is not an absence", INV-3.

#### TC-E6-F6.4-003 · Uncovered slot surfaced ("perlu pengganti")
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Cleared shifts become open/uncovered slots flagged for backfill; delegate shown as a non-binding suggestion.
- **Preconditions:** Approved leave cleared Budi's shift on 2026-06-10; Budi named delegate Citra.
- **Steps:** 1. Open the E4 schedule and the F6.5 team calendar for 2026-06-10.
- **Expected result / AC:** The slot shows **"perlu pengganti"** (uncovered). Citra is shown as a **suggested** (non-binding) backfill. No auto-substitution; no cross-company borrowing. Leader can re-roster a same-company agent.
- **Traceability:** LV-8, FEATURE §7 coverage model, LI-5.

#### TC-E6-F6.4-004 · Shortening an approved leave restores the schedule day
- [ ] **Platform:** Web · **POV:** Shift Leader (observer) · **Priority:** P1 · **Type:** Happy
- **Objective:** Shortening end_date restores the dropped day to unassigned and removes absent-suppression; quota commit reversed.
- **Preconditions:** Approved leave 2026-06-10→12 (3 used). Shorten to end 2026-06-11.
- **Steps:** 1. Shorten the leave (HR/leader action). 2. Inspect 2026-06-12.
- **Expected result / AC:** 2026-06-12 restored to **unassigned** for re-scheduling; absent-suppression removed; the quota commit reversed by 1 (`used_days −1`) on the **same** window (LQ-3); PER_EVENT/UNCAPPED have nothing to reverse. Audited.
- **Traceability:** LI-4, LQ-3, F6.4 Gherkin "Shortening leave restores the schedule".

#### TC-E6-F6.4-005 · Backdated leave over an already-worked day → conflict flagged
- [ ] **Platform:** Web · **POV:** Shift Leader / HR · **Priority:** P1 · **Type:** Edge
- **Objective:** If the agent already clocked in on a day later covered by approved leave, flag for review — no silent overwrite.
- **Preconditions:** Budi clocked in 2026-06-15 (worked). A backdated leave covering 2026-06-15 is approved.
- **Steps:** 1. Approve the backdated leave. 2. Open the conflict review surface.
- **Expected result / AC:** The day's attendance is **flagged for leader/HR review** (not auto-overwritten); both records visible; resolution is manual.
- **Traceability:** LI-3, C-3, F6.4 Gherkin "Conflict when the day was already worked".

#### TC-E6-F6.4-006 · Leave over a day with no schedule yet
- [ ] **Platform:** System · **POV:** Agent (effect) · **Priority:** P2 · **Type:** Edge
- **Objective:** Leave on a not-yet-scheduled day still blocks future scheduling/absent on that date.
- **Preconditions:** No schedule exists for 2026-07-20; approved leave covers it.
- **Steps:** 1. Approve. 2. Attempt to schedule the agent on 2026-07-20 later.
- **Expected result / AC:** The date is marked leave; future scheduling/absent logic respects it (warn/block); no Absent generated.
- **Traceability:** C-1.

#### TC-E6-F6.4-007 · Cancel an approved leave after dates passed → reconciled, not silent
- [ ] **Platform:** Web · **POV:** HR · **Priority:** P2 · **Type:** Edge
- **Objective:** Cancelling a leave whose dates already passed reconciles/flags historical attendance.
- **Preconditions:** An approved leave for past dates is cancelled.
- **Steps:** 1. Cancel it.
- **Expected result / AC:** Historical attendance for those days is reconciled or flagged (no silent change); quota commit reversed; audited.
- **Traceability:** C-3, LI-4, LQ-3.

#### TC-E6-F6.4-008 · Agent sees Leave on their mobile schedule
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Happy
- **Objective:** Approved leave days display as Leave on the agent's schedule calendar.
- **Preconditions:** Budi's `CT` leave 2026-06-10→12 approved.
- **Steps:** 1. Open the mobile schedule for June.
- **Expected result / AC:** 10–12 June render as **Leave** (not blank/absent); also on the F6.5 leave calendar.
- **Traceability:** LI-5, F6.4 Gherkin "Leave shows on calendars".

---

# F6.5 — Leave Calendar & Balance Views

## F6.5 · Mobile · Agent POV

#### TC-E6-F6.5-001 · Agent views balance + request history
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** "My leave" shows per-type balance and the request history with statuses.
- **Preconditions:** Budi has past/approved/pending requests and assigned types.
- **Steps:** 1. Open "Cuti / My leave".
- **Expected result / AC:** Balance lines per assigned type (remaining/used/pending + window/expiry); history list with each request's status (Pending/Approved/Rejected/Cancelled) and dates; deep-link into a request detail.
- **Traceability:** LV-2, LV-7, F6.5 Gherkin "Agent views balance and history".

#### TC-E6-F6.5-002 · Agent with no history — empty state + current balance
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** No requests yet shows empty history but still the balance.
- **Preconditions:** Assigned types, zero requests.
- **Steps:** 1. Open My leave.
- **Expected result / AC:** History empty state; balance still rendered.
- **Traceability:** C-1, no-dead-flow.

#### TC-E6-F6.5-003 · Agent views a prior period's balance
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Edge
- **Objective:** Cross-period — current shown by default, prior viewable.
- **Preconditions:** A 2025 `CT` window (expired) and a 2026 window.
- **Steps:** 1. Switch period to 2025.
- **Expected result / AC:** Shows the 2025 window (expired, no carryover into 2026); current is the default.
- **Traceability:** C-3, INV-4 (no carryover).

## F6.5 · Web · Shift Leader POV

#### TC-E6-F6.5-004 · Leader views team leave calendar (own company)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** Happy
- **Objective:** The team calendar shows who's off when (approved) + pending indicated.
- **Preconditions:** Sari leads Plaza Senayan; several approved + pending leaves in June.
- **Steps:** 1. Open the team leave calendar for June.
- **Expected result / AC:** Each day lists agents on **approved** leave; **pending** requests are indicated (per the open default/toggle); uncovered slots flagged ("perlu pengganti"). Read-only; deep-links to request/approval.
- **Traceability:** LV-3, LV-7, LV-8, F6.5 Gherkin "Leader views team leave calendar".

#### TC-E6-F6.5-005 · Filters on the team calendar
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** Happy
- **Objective:** Filter by date range, leave type, status.
- **Steps:** 1. Filter to `CT` only, status Approved, June.
- **Expected result / AC:** Calendar reflects only matching entries.
- **Traceability:** LV-5.

#### TC-E6-F6.5-006 · Leader scope enforced — denied for another company
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** A leader cannot view leave for a company they don't lead.
- **Preconditions:** Sari leads Plaza Senayan only.
- **Steps:** 1. Attempt to open the leave calendar/data for another company (deep-link or company switch).
- **Expected result / AC:** **403/404** denied; UI surfaces `EmptyNoPermission`. No data leak.
- **Traceability:** LV-1, F6.5 Gherkin "Scope enforced", CONVENTIONS 403/404.

#### TC-E6-F6.5-007 · Leader mobile team calendar parity
- [ ] **Platform:** Mobile · **POV:** Shift Leader · **Priority:** P2 · **Type:** Happy
- **Objective:** Leader sees the own-company team calendar on mobile.
- **Steps:** 1. Open the team leave calendar on mobile.
- **Expected result / AC:** Same scoped data as web (own company); read-only.
- **Traceability:** LV-1, LV-3, F6.5 §4.

## F6.5 · Web · HR / Super Admin POV

#### TC-E6-F6.5-008 · HR cross-company leave calendar
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** HR/super admin see leave across all companies with a company filter.
- **Preconditions:** Leaves across ≥2 companies.
- **Steps:** 1. Open the leave calendar; filter by company.
- **Expected result / AC:** Cross-company picture; company filter narrows it; balances reachable.
- **Traceability:** LV-1, LV-5.

#### TC-E6-F6.5-009 · HR exports leave for a period (audited)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Export reflects the active filters and is audited.
- **Preconditions:** Filter = company X + June.
- **Steps:** 1. Click Export (Excel/PDF).
- **Expected result / AC:** File contents match the filters; export action audited (LV-6); large orgs return a **202** queued/paginated export job (C-4). 
- **Traceability:** LV-6, C-4, F6.5 Gherkin "HR exports leave for a period".

#### TC-E6-F6.5-010 · Super admin sees all companies (scope)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** RBAC
- **Objective:** Super admin scope = all; no company restriction.
- **Steps:** 1. Open the calendar with no company filter.
- **Expected result / AC:** All companies visible; no scope denial.
- **Traceability:** LV-1.

#### TC-E6-F6.5-011 · Calendar loading / error states
- [ ] **Platform:** Web · **POV:** HR/Shift Leader · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Loading skeleton; 500/503 shows an error retry; empty month shows empty state.
- **Steps:** 1. Open a month with no leave / simulate a server error.
- **Expected result / AC:** Skeleton → data/empty; server error renders a retryable error state (no blank screen).
- **Traceability:** no-dead-flow, CONVENTIONS 500/503.

---

# Appendix A — RBAC denial matrix (cross-feature)

#### TC-E6-RBAC-001 · Agent cannot access quota admin / assignment
- [ ] **Platform:** Web/Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Agents have no entitlement-assignment, quota-adjust, bulk-grant, or catalog endpoints.
- **Steps:** 1. As an agent, attempt `POST /employees/{id}/leave-entitlements`, `:adjust-entitled`, bulk grant.
- **Expected result / AC:** **403** on each; client never renders these controls.
- **Traceability:** ELE §3 actors, CONVENTIONS 403.

#### TC-E6-RBAC-002 · Agent cannot approve/reject
- [ ] **Platform:** Mobile/Web · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Agents are not E11 line members for their own leave; cannot approve.
- **Steps:** 1. Agent attempts to approve their own/any leave instance.
- **Expected result / AC:** **403**; no approve UI exposed.
- **Traceability:** F6.3 §3, INV-3.

#### TC-E6-RBAC-003 · Shift leader cannot adjust quotas or manage the catalog
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** Leaders approve (if on a line) but have no quota/catalog admin.
- **Steps:** 1. Leader attempts `:adjust-entitled`, catalog CRUD, bulk grant.
- **Expected result / AC:** **403**; controls hidden.
- **Traceability:** ELE §3, F6.1 §3 actors.

#### TC-E6-RBAC-004 · Shift leader cannot view another company's data anywhere
- [ ] **Platform:** Web/Mobile · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Cross-company scope denial on calendar, balances, inbox.
- **Steps:** 1. Cross-company deep-links for calendar, employee balance, leave instance.
- **Expected result / AC:** **403/404** uniformly (404 to avoid leaking existence).
- **Traceability:** LV-1, CONVENTIONS 404/403.

#### TC-E6-RBAC-005 · 401 mid-session → re-auth UX
- [ ] **Platform:** Web/Mobile · **POV:** Any · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Expired token mid-action triggers the session-expired pattern.
- **Steps:** 1. Expire the token; perform any leave action.
- **Expected result / AC:** **401**; client renders `EmptySessionExpired` + re-auth flow; the action is not silently lost.
- **Traceability:** CONVENTIONS §401 vs 403.

---

# Appendix B — cap_basis taxonomy coverage map

| cap_basis | Example code | Happy submit | Over-cap block | Reset/window behaviour | Approval commit |
|---|---|---|---|---|---|
| ANNUAL_POOL | CT/CTHO | TC-F6.2-001 | TC-F6.2-013 | TC-F6.1-008/009 (grant, prorate), TC-F6.5-003 (year-end no carryover) | TC-F6.3-002 |
| PER_MONTH | CH/KGD | TC-F6.2-003 | (boundary) TC-F6.2-020 | resets monthly (F6.1 Gherkin) | TC-F6.3-002 pattern |
| PER_YEAR_COUNT | STSD | TC-F6.2-004 | TC-F6.2-015 | yearly count reset | occurrence commit |
| LIFETIME_ONCE | CM/CIH/CIU/CPR | TC-F6.2-005 | TC-F6.2-016 | never resets (EMP) | commit + exhaust |
| SERVICE_UNPAID | CLTP | TC-F6.2-007 | (quota block pattern) | never resets; paid=false | TC-F6.3 + LQ-16 |
| PER_EVENT | CKM/CIM/CKA/CMA/CRM | TC-F6.2-002 | TC-F6.2-014 | no standing row | TC-F6.3-008 (nothing to commit) |
| UNCAPPED | SDSKD/CTN/CAP | TC-F6.2-006 | n/a (doc gate only, deferred) | no standing row | nothing to commit |

---

# Appendix C — Notes on superseded / deferred behaviour (do NOT fail on these)

- **Eligibility gates** (gender/notice/min-service/lifetime-once) — RETIRED. See TC-E6-F6.1-GATE-RETIRED. The F6.1 "Gender gate"/"Notice gate" and F6.2 "Eligibility gate" Gherkin scenarios are historical.
- **Document enforcement** (`requires_document` / `MISSING_REQUIRED_DOCUMENT`) — DEFERRED until the upload flow ships. Today the flag is metadata only (TC-F6.2-006). Add the enforcing case when upload lands.
- **Grant-lot / FIFO / earmark model** (2026-06-08) — superseded by the per-type ledger; not tested.
- **Half-day leave** — not in v1 (full days only); no half-day cases.
- **Coverage-clash highlight** — dropped 2026-06-12; the calendar only shows who's off (no clash colouring).
