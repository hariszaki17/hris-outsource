# E6 Leave — Design Audit

**Date:** 2026-06-02
**.pen frame:** `EduUv` (▦ FEATURE GROUP · E6 Cuti)
**Specs:** `FEATURE.md` + 5 PRDs (F6.1–F6.5) + EPICS §8

## 1. Screen inventory

| # | Screen name | Frame ID | Platform | Purpose | Reachable from |
|---|-------------|----------|----------|---------|----------------|
| 1 | E6 · Persetujuan Cuti (HR L2) | `yho5i` | Web (HR) | Level-2 HR approval queue (table of 5 rows, 6 pending, search + Co/Type filters, row-level Detail / Tolak / Setujui) | sidebar nav · "Persetujuan Cuti" |
| 2 | E6 · Kuota & Hibah Cuti (HR) | `P6HZ7E` | Web (HR) | Per-employee quota table (period 2026), with "Terbitkan Kuota Tahunan" + "Ekspor" actions, footnote on LQ-6 audit | sidebar nav · "Kuota Cuti" |
| 3 | E6 · Kalender Cuti (HR) | `s5niW` | Web (HR) | Cross-company monthly leave calendar; legend (Tahunan/Sakit/Lainnya/clash); same-line clash days highlighted ("Parking · pengganti") on 23–24 Jun | sidebar nav · "Kalender Cuti" |
| 4 | E6 · Detail Pengajuan Cuti | `DJrBn` | Web (HR) | Final-approval detail: applicant header with **Tolak** / **Setujui (final)** actions, Detail Pengajuan, Dokumen, Dampak Saldo, Alur Persetujuan timeline, Delegasi & Coverage | row Detail link from #1 |
| 5 | E6 · Sesuaikan Kuota (modal) | `CGCnL` | Web (HR) | Per-employee quota adjust modal: current Total/Terpakai/Sisa, Total baru / Sisa baru fields, Alasan required, info LQ-6 note | quota row "pencil" action in #2 |
| 6 | E6 · Terbitkan Kuota Tahunan (modal) | `W2zYM` | Web (HR) | Bulk period grant trigger: period select, default entitlement per type (Cuti Tahunan / Cuti Sakit), pro-rata toggle, preview count "84 karyawan akan menerima…" | "Terbitkan Kuota Tahunan" CTA in #2 |
| 7 | E6 SL · Persetujuan Cuti (L1) | `qb0S0` | Web/Mobile (Shift Leader) | Level-1 queue scoped to leader's company (Plaza Senayan, 3 pending), same row pattern as HR queue | SL sidebar |
| 8 | E6 SL · Kalender Cuti Tim | `YvYcr` | Web/Mobile (Shift Leader) | Team leave calendar scoped to own company; "Plaza Senayan (terkunci)" filter; clash highlight on 23–24 Jun | SL sidebar |
| 9 | Agen · Ajukan Cuti | `QT92D` | Mobile (Agent) | Leave request form: Balance banner, Jenis Cuti, Dates, Duration chip, Alasan, Upload dokumen, Delegasi, warning note re over-balance block | "+" on #11 |
| 10 | Agen · Status Pengajuan Cuti | `hjCYy` | Mobile (Agent) | Per-request status detail: Summary, Timeline (approval steps), Detail, "Tarik Pengajuan" (withdraw) | history row on #11 |
| 11 | Agen · Cuti Saya (Saldo & Riwayat) | `o1BUa` | Mobile (Agent) | Balance card (8/12 days, progress bar, expiry note) + request history list (4 items, statuses) | bottom-nav · "Cuti" |

## 2. Dead-end findings

### 2.1 Unwired clickable components (cat a)

- **#1 (HR queue) and #7 (SL queue) row actions** — every row has `Detail` (eye), `Tolak`, `Setujui` buttons (`icx8b` / `JkHI2` and siblings). Only the Detail action has a designed destination (#4). The inline row-level `Setujui` and `Tolak` lead nowhere (no confirm modal, no result toast, no Rejected/Approved row state designed). [P0]
- **#4 (Detail Pengajuan) header buttons** — `Tolak` (`dY7Yw`) and `Setujui (final)` (`l7oNAS`) at the top of the header have no destination: no reject-reason modal, no success state, no toast, no redirect back to the queue. [P0]
- **#2 (Kuota) row "pencil" edit icons** on each agent row — only #5 (Sesuaikan Kuota modal) is designed; the "pencil" icon on the *default entitlement* row in modal #6 (`mWurm`, `GwHfd`) implies an inline edit interaction that isn't designed. [P2]
- **#10 (Agen Status Pengajuan)** "Tarik Pengajuan" (withdraw) button (`p89EDv`) — no confirm modal, no cancelled-state result. [P1]
- **Detail row Detail/Tolak/Setujui in tables on #1/#7** — even the Detail link route has no visible "selected from queue" affordance / breadcrumb in #4. [P3]

### 2.2 Orphan screens (cat b)

- **#4 (E6 · Detail Pengajuan Cuti)** has no upward "back to queue" affordance visible at the header level (only the topbar shows "Detail Pengajuan / Cuti"); functionally reachable from the queue eye button but there's no Back/Close control on the detail header itself. Borderline orphan-feeling. [P2]
- All other screens are reachable via sidebar (web) or bottom nav (mobile).

### 2.3 Missing result states (cat c)

- **Approve success state** for both Leader (L1 → moves to HR) and HR (final Approved). No toast, no row-flips-to-Approved, no redirect-to-queue. [P0]
- **Reject result state** with reason capture. No reject-with-reason modal designed (cat d), and no resulting Rejected row/state. [P0]
- **Quota-exceeded blocking error** on request submit. The form has a warning note ("Cuti tahunan melebihi sisa saldo (8 hari) akan diblokir." `VbRoR`) — that's a *pre-emptive* hint, not the actual blocked-submit error state with disabled button or post-submit error toast (LR-3 / INV-1). [P1]
- **Document missing error** state. Upload field has hint label "(wajib untuk cuti sakit)" but the empty + missing-after-submit error styling is not designed (LR-2 / INV-5). [P1]
- **Balance re-check at final approval (LA-5)** — no "Saldo berubah, persetujuan diblokir / ditandai" warning state on #4. [P1]
- **Sesuaikan Kuota save → toast / confirmation** — no result state. [P2]
- **Terbitkan Kuota success / per-employee count completed toast** — no result state. [P2]
- **Withdraw confirmation + Cancelled status** — no confirm modal, no after-state. [P2]
- **Backdated-leave conflict (LI-3 / F6.4 C-3)** — no "agent already worked / clocked in this day" flag UI on #4. [P2]

### 2.4 Untriggered overlays (cat d)

- **Reject-with-reason modal** — required by LA-1 and LA-7 ("decision creates a `LeaveApproval` … with reason"). Not designed. [P0]
- **Approve confirmation / final-approve modal** (especially for HR final where quota deducts + schedule integration fires) — not designed. [P1]
- **Withdraw-confirm modal** — not designed. [P1]
- **Sesuaikan Kuota modal (#5)** *is* designed, but trigger-from-row is the only entry; no overlay variant for "set a new total" vs "adjust remaining only" distinction. [P3]
- **Calendar approved+pending toggle** (LV-3 + open question in F6.5 §10) — not designed as a UI control. The calendar today shows entries without any visible toggle. The clash legend is present, but the approved/pending distinction is absent from the legend and from any toggle control. [P1]
- **Toast** component exists in DS (`PtJHa`) but no toast instances are placed for: submission success, approve/reject success, withdraw success, quota grant complete, balance-restored on cancellation. [P1]

### 2.5 Dangling back/close (cat e)

- **Modals #5 and #6** — close `x` icons (`C6djx7`, `cs7L1`) and Cancel/Simpan buttons are present and lead back to the parent screen via dismissal: OK.
- **Mobile #9 (Ajukan Cuti)** AppBar `chevron-left` Back (`vhBPc`) — implicit return to #11; OK.
- **Mobile #10 (Status Pengajuan)** AppBar Back (`M5u5y`) — implicit return to #11; OK.
- **Web #4 (Detail)** — has no explicit Back button in the page header (only browser/topbar). Minor. [P3]

## 3. Missing screens (cat f)

| # | Missing screen / state | Why required | Severity |
|---|------------------------|--------------|----------|
| M1 | **Reject reason modal** (LA-1, LA-7) | Reason is mandatory for both L1 leader and L2 HR rejects. | P0 |
| M2 | **Approve-success toast / queue-after-approve** | LA-4 fires quota deduction + schedule integration; user needs feedback. | P0 |
| M3 | **Reject-success state** (row removed, status pill Rejected, toast) | Final state for `Rejected`. | P0 |
| M4 | **Shift-Leader Leave Detail (L1)** | The SL queue (#7) opens a detail page; HR has one (#4), but no SL-scoped variant exists. The L1 approver may have different actions ("Setujui & teruskan ke HR") and a different timeline state. | P1 |
| M5 | **Quota-exceeded blocked-submit error state** (LR-3 / INV-1) — disabled Kirim button + inline error / blocking toast | Locked decision: annual over-balance is blocked. | P1 |
| M6 | **Document-required missing-on-submit error** (LR-2 / INV-5) | Submission must be blocked until file attached. | P1 |
| M7 | **Approved-leave-day → schedule auto-cancel feedback** (F6.4 / LI-1, LI-2) | Cross-epic but visible to leader: where does the leader see "shift cleared, perlu pengganti"? Calendar shows clash but no per-leave "shift cancelled" confirmation. | P1 |
| M8 | **Balance re-check insufficient at final approval** (LA-5) | Approval blocked/flagged inline on #4. | P1 |
| M9 | **No-leader → HR sole-approver state** on #4 (LA-2) | The Alur Persetujuan timeline (`CmGkI`) shows two stages "Shift Leader / HR" — there's no variant where Shift Leader stage is suppressed/marked "Tidak ada leader → langsung HR". | P1 |
| M10 | **Calendar approved/pending toggle control** + pending visual state | LV-3 + open question; pending should be visually distinct (e.g. striped / dashed). | P1 |
| M11 | **Withdraw confirm + Cancelled status** (LR-7) | Currently only the trigger button exists. | P2 |
| M12 | **Cancel/shorten an Approved leave** (LI-4 → restore schedule day) | No UI for shortening approved leave; restoration flow not visible. | P2 |
| M13 | **Backdated/clock-in conflict flag** (LI-3) | Visible to leader/HR during approval. | P2 |
| M14 | **Per-period history view** (F6.5 C-3, LV cross-period browsing) | Agent should be able to view prior periods; mobile #11 shows only current period. | P3 |
| M15 | **Empty state** for agent with no history (F6.5 C-1) | Mobile #11 always shows 4 history items; empty state not designed. | P3 |
| M16 | **Document preview / file chip post-upload** in mobile request | After upload, the field shows filename, size, remove action — currently the upload box stays in its empty visual state. | P2 |
| M17 | **HR override (force-approve) UI** (LA-8) | Allowed by spec; not surfaced. | P3 |
| M18 | **Period-end expiry job result / closed-quota row** (LQ-4) | Quota table doesn't show "Closed / hangus" state for prior periods. | P3 |
| M19 | **Quota grant success after-modal toast + count audit** | After "Terbitkan", confirmation of the 84-employee grant outcome. | P2 |

## 4. PRD coverage matrix

| PRD | Required screens / states | Designed | Missing |
|-----|---------------------------|----------|---------|
| **F6.1** Quota & Balances | HR quota table, manual adjust modal w/ reason, period-start grant (manual trigger), pro-rate toggle, balance auto-grant feedback, expired/closed quota row, audit trail surface | Screens #2, #5, #6 (incl. pro-rata toggle, preview count, LQ-6 audit note) | Save-success toast (M19), closed/expired-period row state (M18), audit trail viewer |
| **F6.2** Leave Request | Mobile request form, type/dates/duration, document upload, delegate, balance pre-check warning, blocked submit (over-balance, missing doc), withdraw, overlap-error | Screen #9 with Balance, Upload, Delegasi, warning hint; #10 with Withdraw button | Quota-exceeded blocked-submit error (M5), missing-doc submit error (M6), overlap-with-existing error (LR-5), backdated flag, document-uploaded chip (M16), withdraw-confirm (M11) |
| **F6.3** Two-Level Approval | L1 leader queue + detail, L2 HR queue + detail, approve/reject actions w/ reason, no-leader → HR-only state, self-approve blocked, balance re-check error, HR override | #1, #7 (queues), #4 (HR detail with Tolak / Setujui (final), timeline) | SL detail page (M4), reject-reason modal (M1), approve/reject success states (M2, M3), no-leader timeline variant (M9), balance re-check insufficient state (M8), self-approve blocked state, override UI (M17) |
| **F6.4** Leave–Schedule/Attendance | Schedule cancel/mark-Leave feedback, absent-suppression visual on E5, conflict flag (already-worked), restore on cancel/shorten | Cross-epic; calendar #3/#8 surfaces "Parking · pengganti" tag (clash) | Per-leave "shift cancelled / perlu pengganti" feedback (M7), clock-in conflict UI (M13), shorten-leave restore (M12). The integration is referenced via "perlu pengganti" labels but the *result* on the E4 schedule view isn't visible here (out of epic, but the leave detail should signal it) |
| **F6.5** Calendar & Balance Views | Agent balance + history, leader/HR team calendar, approved+pending toggle, coverage clash highlight (service-line aware), filters, exports, deep-links, scope enforcement, empty state | #11 (balance + history), #3/#8 (calendars w/ legend, Co filter, clash highlight, Ekspor) | Approved+pending toggle control (M10), pending visual styling, deep-link from calendar cell to request, empty agent history (M15), prior-period selector (M14), per-day click-through detail popover |

## 5. Business-rule state check

- **Quota-exceeded error on request:** **partial** — pre-emptive warning note present (`VbRoR`); the actual blocked-submit error after attempting to submit (or disabled CTA when duration > remaining) is not designed.
- **Document upload + missing-doc error:** **partial** — upload field is present with "(wajib untuk cuti sakit)" hint label; no post-submit blocking error state and no uploaded-file chip.
- **Approve / Reject result:** **no** — buttons exist on both row and detail levels but no confirm modal, no reason capture for rejects, no success toast, no transitioned row/status pill.
- **No-leader → HR sole approver:** **no** — the timeline component (`CmGkI`) on #4 hardcodes both stages "Shift Leader · Budi S." and "HR · menunggu"; no variant where the SL stage is removed/marked "escalated".
- **Calendar approved+pending toggle:** **no** — the calendar shows entries (presumably approved-only), but no toggle control and no separate pending visual treatment exist. Coverage-clash highlight (LV-4) *is* designed and is service-line-aware ("Parking · pengganti"), matching the 2026-05-31 decision.
- **Probation pro-rated quota display:** **partial** — the bulk grant modal #6 has a "Pro-rata percobaan & joiner tengah tahun" toggle with formula caption "entitlement × sisa bulan / 12"; the per-employee quota table (#2) doesn't visibly indicate which rows received a pro-rated grant vs full entitlement (no badge / Total column showing 7 vs 12, no probation chip).

## 6. Cross-epic references found

- **E2 (Karyawan / Leave types):** `is_document_required` flag drives the upload requirement (LR-2). The mobile form references "Cuti Sakit" type. The bulk-grant modal lists type-specific entitlements (Cuti Tahunan 12 hari, Cuti Sakit 12 hari). No defect — clean handoff.
- **E3 (Penempatan):** scoping rule (leader = company shift leader; SL queue is scoped to "Plaza Senayan", calendar #8 shows "Plaza Senayan (terkunci)"). LV-1 scope (agent/leader/HR) is correctly reflected.
- **E4 (Jadwal Shift):** clash labels "Parking · pengganti" on calendar cells 23–24 Jun reference the uncovered-slot concept (resolved 2026-05-31). Cross-epic feedback "shift cancelled on approval" (LI-1) isn't designed on either side here.
- **E5 (Kehadiran):** "absent suppression" on approved-leave days (LI-2) — no visible cross-reference here (would live on the E5 attendance screen).
- **E8 (Payroll):** unpaid-leave effect — out of scope per FEATURE §3.
- **E10 (Notifications):** notify-on-each-step (LA-7, LR-6) — Toast component `PtJHa` exists in DS but no instances are placed in E6 screens. Notification list/inbox not surfaced (cross-epic).
- **E1 (Audit):** LQ-6 audit reason is captured in #5 modal; #2 has footnote referencing audit. Audit log viewer itself is cross-epic.

## 7. Prioritized recommendation

**P0 (must fix before screen-build handoff)**
1. **Reject-with-reason modal (M1)** — referenced by `dY7Yw` (detail) and the row Tolak buttons; design once, reuse across HR L2, SL L1, and the row inline action.
2. **Approve & Reject result states (M2, M3)** — at minimum a Toast + queue-row removal pattern. The detail screen should redirect back to the queue with a success toast and the row should disappear or render with a status pill.
3. **Add inline action wiring** on row Detail/Tolak/Setujui (table cells `icx8b`, `JkHI2` etc.) — either remove the inline buttons (force detail-page flow) or design the inline confirm overlay; current design implies a one-click approval that has no destination.

**P1 (need before build)**
4. **SL Detail (L1) screen (M4)** — clone #4 with adjusted header CTA "Setujui & teruskan ke HR", different timeline state, and SL-scoped data.
5. **No-leader timeline variant (M9)** for #4: collapse the SL stage to a "Tidak ada Shift Leader — eskalasi langsung ke HR" badge per LA-2.
6. **Quota-exceeded blocked-submit error (M5)** and **missing-doc submit error (M6)** on mobile #9 — disabled CTA + inline error styling on each field, plus a top-of-screen error banner.
7. **Calendar approved + pending toggle (M10)** + pending visual treatment (striped/dashed fill, lighter alpha) — choose default per the F6.5 §10 open question and ship the toggle so HR can flip.
8. **Balance-re-check failure (M8)** on #4 header (warning banner + disabled Setujui).
9. **Schedule-cancel feedback (M7)** — minimum: an info card on #4 listing the shifts that will be cleared on approval ("3 shift akan dibatalkan: 15, 16, 17 Jun").

**P2 (polish, can land in v1.1)**
10. Withdraw-confirm modal + Cancelled state (M11, LR-7).
11. Cancel/shorten approved leave + schedule restore (M12, LI-4).
12. Backdated / clock-in conflict flag (M13, LI-3).
13. Uploaded-file chip (M16) and uploaded-doc preview on the HR detail (#4 "Dokumen" card is designed; verify preview behavior).
14. Grant-success toast (M19) + per-employee badge for pro-rated rows (M18 reverse).

**P3 (nice-to-have)**
15. Period selector on agent mobile (M14), empty-history state (M15), HR override UI (M17), prior-period closed-quota row (M18).

## 8. Notes

- The web HR + SL screens reuse a clean pattern (Sidebar + Topbar + Content) and the queue tables are consistent across L1/L2, which makes wiring up the missing modals cheap once the reject-reason and result-toast patterns are designed.
- The clash highlight on the team calendar correctly implements the 2026-05-31 decision (service-line-aware, not raw headcount): cells 23–24 Jun show `$warn-bg` fill + "Parking · pengganti" tag. The legend includes the swatch. Good.
- The bulk grant modal (#6) is well-considered: period selector, default-per-type, pro-rata toggle, preview count, plus warning "tidak menimpa jumlah yang sudah terpakai" — covers LQ-1 and the 2026-05-31 "manual Terbitkan trigger / repair" decision.
- The Sesuaikan Kuota modal (#5) correctly requires the reason field per LQ-6 with audit note.
- Mobile coverage is *thin*: only 3 screens. There's no mobile leader-approval queue (leader uses web), no mobile calendar for the agent (only history list), no notifications inbox. That may be intentional (the FEATURE.md platform table says leader uses Web/Mobile but the design only ships web); flag for explicit confirmation.
- The "Delegasi & Coverage" card on #4 (`J1CSI`) — content wasn't unpacked here but is present; spot-check it reflects the 2026-05-31 decision that delegate is *non-binding suggested backfill*.
- Toast component (`PtJHa`) exists in the DS but is unused in E6 — once you wire result states, every approve/reject/withdraw/grant flow should drop a toast.
- The Cuti calendar legend uses `$accent-blue` for "Tahunan" and `$bad-tx` for "Sakit". This is consistent with DS Section 2 (attendance status mapping uses teal-not-green for present; here the chosen "Tahunan = blue" / "Sakit = red" reads correctly and does not collide with the brand-green primary).
