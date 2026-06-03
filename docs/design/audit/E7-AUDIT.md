# E7 Overtime — Design Audit

**Date:** 2026-06-02
**.pen frame:** SGrJK (`▦ FEATURE GROUP · E7 Lembur`)
**Specs:** `docs/epics/E7-overtime/FEATURE.md` + 4 PRDs (`overtime-rules.md`, `overtime-capture.md`, `overtime-approval.md`, `overtime-records.md`) + DATA-MAPPING.md + `EPICS.md` §8

---

## 1. Screen inventory

| # | Screen name | Frame ID | Platform | Purpose | Reachable from |
|---|---|---|---|---|---|
| 1 | E7 · Persetujuan Lembur (HR L2) | `H1eBN` | Web 1440×1024 | HR Level-2 approval queue (5 pending: 3 auto-detect + 2 request, with Workday/RestDay/Holiday tier badges) | Sidebar (component instance) |
| 2 | E7 · Aturan OT & Kalender Libur (HR) | `vd4na` | Web 1440×1024 | HR OT-rule CRUD (`vsQW2` left: tier table) + public-holiday calendar editor (`tpIxx` right: 5 holidays incl. recurring + movable) | Sidebar |
| 3 | E7 · Rekap Lembur (HR) | `JEmCk` | Web 1440×1024 | HR aggregate report — 4 stat cards (Total / Workday / RestDay / Holiday with ref multipliers) + filters + 5 rows + Export button | Sidebar |
| 4 | E7 SL · Persetujuan Lembur (L1) | `Vh2P9` | Web 1440×1024 | Shift Leader Level-1 approval queue (Plaza Senayan, 3 pending) — same row shape as HR but `c3` column shows the *clock-out time* (not approver name) | Sidebar |
| 5 | Agen · Ajukan Lembur | `wDLQu` | Mobile 390×844 | Agent pre-approval request form (context chip, date, start/end time, duration chip, tier chip, reason, info note, Submit footer) | Mobile bottom-nav "Lembur" → Riwayat → "+" AppBar icon (`Dh8sN`) |
| 6 | Agen · OT Terdeteksi (Konfirmasi) | `mzCUA` | Mobile 390×844 | Agent post-shift confirmation of an auto-detected OT candidate (big +3j 32m display, link back to attendance ATT-10692, optional note, dual footer: "Konfirmasi & Ajukan" + "Bukan Lembur") | Notification → AppBar back chevron returns |
| 7 | Agen · Lembur Saya (Riwayat) | `nd3KT` | Mobile 390×844 | Agent's own approved/pending/rejected OT history — period summary by tier (12j / 6j / 4j), 5 history cards w/ status pills | Bottom-nav "Lembur" (active) |

**Total: 7 screens — 4 web, 3 mobile.** Three POVs (HR Admin, Shift Leader, Agent) all represented.

---

## 2. Dead-end findings

### 2.1 Unwired clickable components (cat a)

| # | Sev | Item | Issue |
|---|---|---|---|
| a1 | **High** | "Setujui" primary button on every approval-table row (HR `XQ4FN`,`oVWvX`,`B0Fswt`,`vhcY2`,`oCjVn`; SL `D00eQ9` etc.) | Visually a CTA; no Approve-confirmation modal or success Toast designed. Each Approve action is a state transition (`Pending → LeaderApproved`, then `→ Approved`) per OA-1; spec says "agent notified" — no toast/notification result state exists. |
| a2 | **High** | "Tolak" buttons on every row (HR `gfWho`,`oGvus`,`VI5Fc`,`fzoVS`,`L1hRR`; SL `MMgi0`,`I6iTW`,`exuBF`,`TGW50`,`ieYy7`) | OA-1 / OA-7 require a **reason** on reject; no Reject-with-reason modal designed in either HR or SL flow. |
| a3 | **High** | "Setujui Massal" / Bulk button (HR `EHYUn`, SL `fQllv`) | OA-9 says bulk approve is allowed; no row-selection state (no checkboxes visible on rows), no selection counter, no confirmation modal, no result toast. The button is decorative. |
| a4 | **Med** | "Detail" buttons on every approval row | No OT-detail modal/screen exists. Approvers (esp. HR L2) need to inspect notes, day-type derivation source, the linked attendance record (auto-detect) before approving — this is the core decision UI and it leads nowhere. |
| a5 | **Med** | "Tambah Aturan" button (`dTkrw`) on rules table | No "Create OT Rule" form/modal designed; OR-1 fields (day_type, multiplier, min_minutes, requires_preapproval, optional service_line) have nowhere to be entered. |
| a6 | **Med** | "+" icon (`oYHbH`) on Kalender Hari Libur header | No "Add Holiday" modal/form designed; `HolidayCalendar` fields (date, name, recurring) cannot be created from a clickable element. |
| a7 | **Med** | "Edit" affordance on existing rule rows and holiday rows | Rule rows (`Sm3Ze`,`jGuL0`,`AeA3C`,`P7GfZ`) and holiday rows (`YZOLM`,`ELK9y`,`PvxTf`,`N9UbA`,`LLx4V`) have no edit icon/menu — OR-6 "deactivate not delete" plus daily holiday edits have no UI. |
| a8 | **Med** | "Ekspor" button on Rekap (`NwG6V`) | OR-5 says exports are filtered, audited, structured for E8/E10; no format-picker/loading/success state for export (Excel/CSV/PDF choice). |
| a9 | **Low** | Date / Start / End / Reason input boxes on Ajukan form (`GXMQR`,`zCuXR`,`N3WlQ`,`zAYbE`) | No date picker overlay, no time picker overlay, no validation error state (e.g. end before start, overlapping existing OT — OC-3 C-3). |
| a10 | **Low** | "Kirim Pengajuan" submit (`oJJGX`) on Ajukan | No success toast/screen, no "request without active placement" blocked state (OC-6), no duplicate-detected warning (OC-8). |
| a11 | **Low** | "Konfirmasi & Ajukan" / "Bukan Lembur" on Terdeteksi (`n6rAeQ`,`CKyPk`) | No result state for either path — what happens after confirm? after dismiss? Per OC-7 "agent confirms then leader approves" — needs a confirmed/dismissed acknowledgement. |
| a12 | **Low** | History cards on Lembur Saya (`CL7W3`,`Otdjl`,`U9YiH`,`Uw7W0`,`h2Iih`) | Card-shaped with status pill; no detail drawer or screen to see approval timeline (level-1 / level-2 decisions, reasons) — important when status is "Ditolak" so the agent can see the reason (`h2Iih`). |
| a13 | **Low** | Filters on all 3 web screens (`cBxIA`,`K95sV`,`Ur6DK` etc.) | FilterSelect / SearchField components instanced but no opened-dropdown state or empty-result state. |

### 2.2 Orphan screens (cat b)

None — all 7 screens are reachable in principle:
- Web screens have a Sidebar instance, implying nav entry.
- Mobile Riwayat is the bottom-nav landing (active "Lembur" tab `E3KZRl`).
- Mobile Ajukan is reachable from Riwayat's `Dh8sN` "+" icon (back chevron returns).
- Mobile Terdeteksi is implicitly entered from a notification (no in-app deep-link designed but a back chevron returns).

### 2.3 Missing result states (cat c)

| # | Sev | Missing state |
|---|---|---|
| c1 | **High** | Approve success — no toast/empty-queue state after HR/SL approves the last pending OT. |
| c2 | **High** | Reject success — no toast, no return to queue with row removed, no audit visible. |
| c3 | **High** | Reject-reason modal — required by OA-1/OA-7 ("reason required"). |
| c4 | **High** | Bulk-approve result — selected count, "X disetujui" toast, partial-failure (some couldn't approve due to OA-5 self-approval block) state. |
| c5 | **High** | OT detail view (modal or screen) — for HR/SL to inspect before decision. Should show: source (Requested/Auto-detected), linked attendance, day-type derivation rationale, notes, current approval level, history. |
| c6 | **Med** | Submit OT request success — confirmation toast / Lembur Saya updated. |
| c7 | **Med** | Submit blocked — "No active placement on this date" (OC-6); "Overlapping OT" (OC C-3); "End before start" validation. |
| c8 | **Med** | Auto-detected confirm success — "Diajukan ke Shift Leader" feedback. |
| c9 | **Med** | "Bukan Lembur" dismiss — clarify what happens (record marked dismissed? E5 attendance unchanged per C-1?). |
| c10 | **Med** | Empty state — "Belum ada pengajuan lembur" on Lembur Saya for new agents (C-1 of F7.4). |
| c11 | **Med** | Empty approval queue — "Tidak ada lembur menunggu persetujuan" for HR/SL after working through queue. |
| c12 | **Med** | Create-OT-rule form + success + validation (OR-1 fields, multiplier > 0 check from F7.1 flow `S1`). |
| c13 | **Med** | Add-Holiday form + success (date, name, recurring toggle, movable per C-4 of F7.1). |
| c14 | **Med** | Export-as-X picker + generating/done state. |
| c15 | **Low** | Late-approval flag display (C-2 of F7.4) — period closed but appears anyway. |
| c16 | **Low** | Re-derivation flag (G-3 migration + F7.3 C-2) — "day_type defaulted/flagged" indicator on historical or attendance-corrected records. |
| c17 | **Low** | Withdrawal/Cancel from agent side (F7.3 C-3) — agent action and result on a Pending row. |

### 2.4 Untriggered overlays (cat d)

No E7-specific overlays exist in `hoY3q` (DS · C · Overlays) — confirmed via name-pattern search. Generic Toast component (`PtJHa`) is in DS but no E7 toast instances are wired into any screen. There is **zero** overlay/modal coverage for the E7 epic.

### 2.5 Dangling back/close (cat e)

| # | Sev | Item |
|---|---|---|
| e1 | **Low** | Mobile Ajukan back chevron (`AcbfD`) — implied to return to Riwayat (`nd3KT`); not explicitly wired but the assumption is safe. |
| e2 | **Low** | Mobile Terdeteksi back chevron (`lZWvb`) — same: implied return to Riwayat or notification source; entry path is itself undesigned. |

---

## 3. Missing screens (cat f)

| # | Sev | Missing screen | Required by |
|---|---|---|---|
| f1 | **High** | **OT detail / inspection view** (web modal or full screen + mobile sheet) | F7.3 OA-1 (decide); F7.2 OC-2 (attendance link); F7.4 OR-6 (deep-link to record/approval). Without this, no informed approval is possible. |
| f2 | **High** | **Reject-with-reason modal** (HR + SL) | OA-1 / OA-7 (reason required). |
| f3 | **High** | **Create / Edit OT Rule form** | F7.1 OR-1 — currently no UI to set day_type, multiplier, min_minutes, requires_preapproval, service_line scope. |
| f4 | **High** | **Add / Edit Public Holiday entry** | F7.1 OR-5 + C-4 (recurring vs movable). The right-side calendar is read-only. |
| f5 | **Med** | **Approve-bulk selection state** (rows with checkboxes, action bar, count, confirm modal) | OA-9. |
| f6 | **Med** | **Approval result toast / empty-queue state** | F7.3 flow `S3` "Notify agent / Persist + audit". |
| f7 | **Med** | **Agent OT submit success + blocked variants** | OC-6, OC-8, OC C-3. |
| f8 | **Med** | **Worked-without-request "Flagged" indicator/badge** on rows | EPICS §8: "Pre-approval = worked-without-request OT still approvable after the fact (flagged)." No flag in any row is visible. |
| f9 | **Med** | **Export options modal / progress + success** | OR-5 (E8/E10 export). |
| f10 | **Med** | **Override flow for HR** | OA-8 (HR may override with a reason). No UI distinguishes override from regular decision. |
| f11 | **Low** | **Service-line precedence indicator** | OR-2 — Rules table shows global+Parking rows but no visual cue that Parking overrides global for Workday. |
| f12 | **Low** | **Cancel/withdraw** action on Lembur Saya for Pending rows | F7.3 C-3. |
| f13 | **Low** | **Empty / loading / error states** for all 3 web tables and 3 mobile screens. |
| f14 | **Low** | **<60 min skipped indicator** in UI (a captured-then-skipped or "ignored" toast for the agent) | OC-4 / INV-5. Currently invisible to the user — the rule applies silently. |
| f15 | **Low** | **Notification examples** triggering Agen · OT Terdeteksi | F7.2 flow `C5` "Notify approver / agent to confirm". |

---

## 4. PRD coverage matrix

| PRD | Required screens/states | Designed | Missing |
|---|---|---|---|
| **F7.1 Overtime Rules** | Rules list, Create/Edit Rule form, Holiday calendar list, Add/Edit Holiday form, deactivate (not delete), service-line scope | Rules list (`vsQW2`), Holiday calendar list (`tpIxx`) | Create/Edit Rule, Add/Edit Holiday, deactivate confirmation, service-line precedence cue (OR-2), `min_minutes` value mismatch (table shows 30m / 0m — spec locks 60m) |
| **F7.2 Overtime Capture** | Agent request form, Auto-detect confirm prompt, Sub-threshold ignored feedback, day-type classification visible, no-placement-blocked, duplicate-protected | Ajukan form (`wDLQu`), Terdeteksi confirm (`mzCUA`), tier chip on form + record | Sub-threshold message in agent UI (`a5jjhX` in form mentions "< 30 menit" — wrong number), blocked/duplicate error states, validation states, submit-success toast, "Bukan Lembur" result, link to attendance is shown (good — `UgbM0`) |
| **F7.3 Two-Level Approval** | SL approval queue, HR approval queue, Approve action, Reject-with-reason, Bulk approve, Self-approval blocked, Escalation when no leader, Override (HR), OT detail | SL queue (`Vh2P9`), HR queue (`H1eBN`), inline Approve/Reject row buttons, Bulk button (decorative) | Reject-reason modal, Bulk selection state + result, OT detail, Approve/Reject result toasts, Self-approval blocked state, No-leader escalation indicator, HR override flow, Audit/timeline view |
| **F7.4 OT Records & Reporting** | Agent My-OT view, HR report w/ tier aggregation + ref multipliers, Filters, Export, Empty state, Late-approval flag, Migrated/defaulted-day_type flag | Lembur Saya (`nd3KT`) w/ tier summary + history cards w/ status pills; HR Rekap (`JEmCk`) w/ 4 StatCards (Total/Workday/RestDay/Holiday + ref multipliers ×1,5 / ×2,0 / ×3,0) + filters + table; Export button | Export options modal/result, Empty states (no OT), Late-approval flag, Migrated/defaulted-day-type flag (G-3/G-5), Detail drill-down from history card |

---

## 5. Business-rule state check

- **Auto-detected OT confirm prompt:** **yes** — `mzCUA` "Agen · OT Terdeteksi (Konfirmasi)" with attendance link `UgbM0` (ATT-10692), explicit confirmation copy, dual footer. Result state after confirm/dismiss: **no**.
- **Pre-approval request form:** **yes** — `wDLQu` "Agen · Ajukan Lembur" with date/start/end/reason and duration+tier preview chips. Picker overlays / validation / submit-success: **no**.
- **Worked-without-request flagged:** **no** — `c2` column shows "Request" vs "Auto-deteksi" badges but no third badge for "worked-without-request (after-the-fact)" per EPICS §8 lock. Per F7.1 OR-1 `requires_preapproval=true` rules, any row that bypasses pre-approval should be visibly flagged on the approval queue; currently indistinguishable.
- **<60 min skipped indicator:** **no — and worse, contradicted**. Spec locks `min_minutes = 60`; the rules table in `vsQW2` shows `30m` for Workday/Holiday and `0m` for Hari Besar. The Ajukan form's info note `a5jjhX` says "OT < 30 menit tidak dihitung" — wrong number. No "skipped because below threshold" feedback in either auto-detect or request flow.
- **Tier indicator (Holiday/RestDay/Workday):** **yes — partially**. Three tiers shown in row column `c1` ("Hari Kerja · ref" / "Hari Libur · ref" / "Hari Besar · ref") and as 3 StatCards on Rekap and 3 summary boxes on Lembur Saya. **Missing:** explicit indicator that Holiday tier *wins* over RestDay when both apply (EPICS §8 lock + C-2 of F7.1) — there is no row that demonstrates this collision or its resolution.
- **Approve/Reject result:** **no** — no toast, no row-removed state, no empty-queue state, no reason-modal for Reject.
- **Public holiday calendar editor:** **partial** — read-only list of 5 holidays present (`tpIxx`), `+` icon present but no Add/Edit modal designed. Recurring vs movable shown via subtitle copy ("Berulang tiap tahun" / "Tanggal bergerak").

---

## 6. Cross-epic references found

- **E5 Attendance:** Agen · Terdeteksi explicitly cites `ATT-10692` (`uhwmJ`) — correctly models F7.2 OC-2 ("verified attendance beyond shift end") with the attendance link in `attendance_id`. Good integration cue.
- **E4 Schedule:** Agen · Ajukan context chip `USqgL` "Plaza Senayan · Shift Pagi 07:00–15:00" + Terdeteksi "shift berakhir 15:00" (`wg7F9`) — establishes the shift-end baseline that OT runs against.
- **E2/E3 Placement:** company name (Plaza Senayan, Mall Kelapa Gading, Grand Indonesia, Senayan City) appears in every row sub-text — placement is the contextual scope.
- **E6 Leave (shared calendar):** the Holiday calendar on `tpIxx` is shared with E6 per EPICS §8 lock — design treats it as E7-local; need to verify the same component renders in E6 or note the cross-epic master.
- **E1 Audit:** no audit-trail UI visible on OT records (no "edited by X at Y" footers, no approval timeline).
- **E8 Payroll / E10 Reporting:** Export button on Rekap (`NwG6V`) signals the handoff; no destination/format picker designed.
- **F3.4 (Shift Leader scope):** SL screen header copy `ckCPK` "Tingkat-1 (Shift Leader) · Plaza Senayan" correctly scopes to one company per F3.4 SL-7.

---

## 7. Prioritized recommendation

**P0 (block any usable click-through):**
1. **OT detail modal/screen** (f1) — without it neither HR nor SL can decide responsibly; this is the central screen of the approval feature.
2. **Reject-with-reason modal** (f2) — OA-1/OA-7 mandate; current Reject buttons would commit without input.
3. **Approve / Reject / Bulk result states** (c1, c2, c4) — every action button is a dead-end without a toast and queue update.
4. **Fix the `min_minutes` mismatch** — rules table (`SheV2`,`KEMS2`,`Odq14`) and Ajukan note (`a5jjhX`) display 30m / 0m, contradicting the §8 lock of **60m**. Decide whether to (a) update the design to 60m everywhere, or (b) re-open the decision; do not ship divergent UI.

**P1 (PRD compliance, common paths):**
5. **Create/Edit OT Rule form + Add/Edit Holiday form** (f3, f4) — HR cannot configure anything from current screens.
6. **Bulk-approve selection state** (f5) + checkboxes on rows — wire up OA-9.
7. **Worked-without-request flag** (f8) on rows — required by EPICS §8 lock; introduce a third `c2` badge (e.g. "Tanpa pra-persetujuan" / warning tier color).
8. **Agent submit success/error variants** (c6, c7) — `Belum punya penempatan aktif`, `Overlap dengan pengajuan lain`, validation.
9. **Auto-detect confirm result + dismiss path** (c8, c9) — finish the Terdeteksi flow.

**P2 (reporting / migration completeness):**
10. **Export options + progress** (f9, c14) — pick format, audit row.
11. **Empty / loading states** for all tables and mobile lists (c10, c11, f13).
12. **Late-approval and migrated/defaulted-day-type flags** (c15, c16) — per F7.4 C-2/C-5 and DATA-MAPPING G-3.

**P3 (polish):**
13. **Override (HR) flow** (f10) — distinct affordance/reason capture.
14. **Service-line precedence cue** (f11) on rules table — small "overrides Global" pill on the Parking row.
15. **Cancel/withdraw** (f12), notification entry to Terdeteksi (f15).
16. **Holiday-beats-RestDay collision example/tooltip** — currently no visual covers EPICS §8 lock; could be a single explainer chip or a worked example row in the table header.

---

## 8. Notes

- **Strengths:** All 7 screens follow the design system tokens consistently (`$primary` for SL Approve CTA + active nav, info-blue for auto-detect badge, warn-amber for holiday day-cell, ok-green for approved status pills). The auto-detect → attendance link (`UgbM0`) is a particularly clean cross-epic cue. Tier classification is consistent across HR queue, SL queue, agent records, and the Rekap StatCards. The Ajukan form's live duration+tier preview chips (`S4oii7`) is a nice touch.
- **Sidebar/topbar reused** via component instances (`iCqTB`, `caFkE`) on every web screen — good consistency.
- **Mobile bottom-nav** active state correctly on "Lembur" (`E3KZRl`) on Riwayat; the Ajukan and Terdeteksi screens correctly drop the bottom-nav for a Footer CTA pattern.
- **Coverage gap is overwhelmingly in result/feedback/modal/form states**, not in primary list/index screens. The "shell" of E7 exists; the **interactive depth** (approve/reject/create) does not. This is the same pattern observed in audits of analogous approval epics — addressing P0 + P1 above closes ~80% of the gap.
- **Numerical inconsistency on `min_minutes`** is the most concrete spec-vs-design defect: 30m / 0m / "< 30 menit" in the design vs the **60m** locked in `EPICS.md` §8 (and re-asserted in FEATURE.md §7 "Resolved 2026-05-29"). Worth fixing before any screen-gen pass propagates the wrong number.
- No `.pen` modifications were made — read-only audit per instructions.
