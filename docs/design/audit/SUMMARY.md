# Design Audit — Executive Summary

**Date:** 2026-06-02
**Inputs:** 10 per-epic audit reports in `docs/design/audit/`.
**Method:** Synthesis only. No `.pen` edits. Per-epic reports are authoritative; this synthesis adds the cross-epic view + fix-sequencing plan.

---

## 1. Headline counts

Severity vocabulary differs by report (BLOCKER/HIGH/MEDIUM/LOW vs P0/P1/P2/P3). Normalized: **BLOCKER=P0, HIGH=P1, MEDIUM=P2, LOW=P3**.

| Epic | BLOCKER | HIGH | MEDIUM | LOW | Total |
|------|---------|------|--------|-----|-------|
| E1 Foundations | 8 | 7 | 7 | 4 | 26 |
| E2 Identity | 14 | 9 | 9 | 4 | 36 |
| E3 Placement | 6 | 5 | 6 | 5 | 22 |
| E4 Scheduling | 4 | 5 | 4 | 2 | 15 |
| E5 Attendance | 10 | 5 | 5 | 3 | 23 |
| E6 Leave | 3 | 6 | 9 | 5 | 23 |
| E7 Overtime | 4 | 5 | 7 | 5 | 21 |
| E8 Payroll | 3 | 3 | 4 | 2 | 12 |
| E9 Migration | 8 (gap) | 3 (gap) | 1 (gap) | 0 | 12 |
| E10 Reporting | 7 | 11 | 4 | 0 | 22 |
| **Totals** | **67** | **59** | **56** | **30** | **212** |

E9 is a pure gap analysis (no `.pen` frame at all); its "BLOCKERs" are required-but-undesigned screens, not broken designed ones.

---

## 2. Cross-epic dead-ends

### 2.1 Confirmed broken cross-epic links

| Source (epic · element) | Target expected | Verdict |
|---|---|---|
| **E1** Login post-login routing | E10 dashboards (`ETi5H`, `RiSPW`, `e8Sw1`) | Targets exist; routing **unannotated** on E1 side. |
| **E1** Pengguna table · "Plaza Senayan" link | E3 company detail (`nLN4d`) | Target exists; annotation missing on E1. |
| **E1** AuditLog row deep-links (Cuti #LR-1042, Kehadiran #ATT-10711, etc.) | E6/E5/E3/E8 detail screens | E6 SL leave detail does **NOT** exist (E6 M4) — link broken for SL actor. |
| **E2** Karyawan Detail (`JBjBb`) 4 tabs: Penempatan / Kehadiran / Cuti & Lembur / Dokumen | E3/E5/E6/E7 per-employee timelines | **None** of these epics ship per-employee timeline views; all four tabs **dead-end**. "Dokumen" tab has no PRD anywhere. |
| **E2** Daftar "Impor" button | E9 import flow | E9 has no frame; button unwired both sides. |
| **E3** Detail · view-agent-schedule | E4 by-agent matrix | E4 has no by-agent view; only week grid by company. |
| **E4** Cuti-locked cell click | E6 leave detail (leader view) | E6 has HR detail but **no SL detail variant** — SL click dead-ends. |
| **E4** Auto-publish "memicu notifikasi" | E10 notification | E10 has center but no "Jadwal baru" card-specific design. |
| **E5** Tolak → modal-alasan → notify | E10 notification + E5 agent correction-tracker | Agent correction-tracker **completely missing** in E5 (P0). Loop broken. |
| **E5** SL ScopeBanner "Eskalasi HR" | HR queue row badge / filter | **No badge or filter** in HR queue. Receiver UI missing. |
| **E6** Approve → "perlu pengganti" | E4 schedule cleared-day feedback | E4 has no cancelled-by-leave annotation. |
| **E6/E7** Approve → notify agent (LA-7/OA-1) | E10 notification result | E10 has card but **no mark-as-read result and no stale-link state**. |
| **E7** Detail referencing `ATT-10692` | E5 attendance detail | **E7 OT detail screen itself missing** (P0). |
| **E7** Export from Rekap | E10 export framework | E10 has format-picker only, **no progress/success/error** (P0). |
| **E8** Excel export (`RFJJj`) | E10 export framework + E1 audit | Same E10 gap; audit writeback unsurfaced. |
| **E8** "Perlu review" decrypt-fail row | E9 reconciliation queue | E9 has no frame; destination missing. |
| **E10** "Perlu Tindakan" Penempatan akan berakhir | E3 expiring list | E3 has no "expiring list" view; only Detail. Partial. |
| **E10** Notification "Auto clock-out" | E5 attendance detail | E5 has web detail only; **no agent-side mobile attendance detail**. Mobile link broken. |

**Validated good links:** E10 dashboard Perlu Tindakan rows → E5/E6/E7 queues; Agent Beranda Clock-In → E5; Agent Beranda pending leave → E6 mobile status.

### 2.2 Inverse — receiver expects traffic no sender produces

- **E2** Detail "Dokumen" tab — orphan; no PRD or source epic populates.
- **E5** Mobile Riwayat anticipates Absen pill, but no E6 leave→absent feedback is produced on either side.
- **E6** Detail timeline shows SL+HR stages, but no producer of the no-leader collapse state (LA-2) exists upstream.
- **E9** Review queue (when designed) expects E8 decrypt_fail flags. E8 produces; E9 has no destination yet.
- **E10** Leave notification "Cuti disetujui" → SL mobile leave detail. No SL mobile leave detail exists.

### 2.3 Net cross-epic findings

- **Largest broken cluster:** E2 employee-detail tabs (4 dead tabs because no epic ships a per-employee timeline). Fix at cross-cutting level — see D3.
- **Second:** notification → target cycle. Every approval epic (E5/E6/E7) ends with "agent notified" but E10 lacks both sender-side toast and receiver-side mark-read state.
- **Third:** SL detail variants missing across the board (E5 has them; E6/E7 don't). E10 dashboard routes SL to detail → hits HR screen. Mobile leader dashboards also missing.

---

## 3. Pattern-level findings (recur across 3+ epics)

| # | Pattern | Epics affected | Severity |
|---|---|---|---|
| **P-1** | Reject-with-reason modal | E1, E2, E5×2, E6, E7 | BLOCKER ×5 |
| **P-2** | Approve/Reject success toast (post-action + queue removal) | E1, E2, E3, E5, E6, E7, E8 | BLOCKER ×6 |
| **P-3** | Bulk approve/action confirmation modal | E5, E7 (E9 future) | BLOCKER ×2 |
| **P-4** | Empty states — filtered zero results | All 10 epics | HIGH |
| **P-5** | Empty states — fresh / no-data-yet | E1, E5, E6, E7, E8, E10 | BLOCKER (E10); HIGH others |
| **P-6** | Loading / skeleton states | All 10 epics | MEDIUM |
| **P-7** | Row-kebab popover menu (per-row actions) | E1, E2, E3, E4, E6, E7 | BLOCKER (E1/E2/E3/E4) |
| **P-8** | Destructive-action confirm dialog (E1-themed, not E5-themed) | E1, E2, E3, E4, E7 | BLOCKER (E1/E3) |
| **P-9** | No-permission / 403 state | E1, E3, E8, E9 | MEDIUM/HIGH |
| **P-10** | Session-expired re-auth banner | E1 (origin); affects all | HIGH |
| **P-11** | No-leader → HR-only approval state | E6, E7 | HIGH ×2 |
| **P-12** | Export framework end-to-end (picker → progress → success → error) | E1, E3, E5, E6, E7, E8, E10 (owner) | BLOCKER |
| **P-13** | Notification result states (toast on send + mark-read + stale + empty) | E4, E5, E6, E7, E10 (owner) | BLOCKER (E10) |
| **P-14** | Settings/Preferences IA (Pengaturan hub, mobile profile, notification prefs stub) | E1, E2, E10 | MEDIUM |
| **P-15** | Audit-trail viewer (who/what/when on a record) | E1, E3, E5, E6, E7, E8 | HIGH |
| **P-16** | Picker overlays for cross-epic FKs (Employee/Company/ServiceLine/Position) | E2, E3, E4, E9 | HIGH |
| **P-17** | Mobile leader surfaces (approval queue, dashboard, notifications) | E5, E6, E7, E10 | HIGH |
| **P-18** | Form validation error states (inline + blocked-submit + "Menyimpan…") | E1, E2, E3, E5, E6, E7 | BLOCKER/HIGH ×many |

**Fix-once impact:** P-1, P-2, P-4, P-5, P-7, P-8, P-12, P-13, P-18 together eliminate roughly **45–55 of the 67 BLOCKERs and ~30 of the 59 HIGHs**.

---

## 4. Coverage matrix

| Epic | Features designed | Features missing | BLOCKERs | HIGHs |
|---|---|---|---|---|
| E1 | F1.1/1.2/1.3/1.4 all partial | Password-reset flow; Add/Edit user; Audit detail drawer; settings IA | 8 | 7 |
| E2 | F2.1 Employee substantial | **F2.2 Agreement (entire), F2.3 Company (entire incl. geofence), F2.4 ServiceLines/Positions (entire), F2.5 OpsMasterData (entire)**, HR approval queue for agent edits | 14 | 9 |
| E3 | F3.1/3.2/3.5 partial | **F3.3 Transfer (entire), F3.4 SL Assignment (entire)**, Renew/Terminate/Resign modals, row-kebab | 6 | 5 |
| E4 | F4.1 list + Tambah; F4.2 grid visual; F4.3 agent week | Shift-picker popover (core), Edit/Deactivate Shift, Bulk apply-to-range, Validation, Reminders, HR oversight | 4 | 5 |
| E5 | F5.1 partial, F5.3 + SL, F5.5 dashboard + mobile | **F5.4 Corrections (entire)**, Clock-out variant, Out-of-geofence, Reject modal, Bulk-approve | 10 | 5 |
| E6 | F6.1 quotas + modals, F6.2 mobile, F6.3 HR detail+queues, F6.5 calendars | SL detail variant; reject modal; result toasts; cancel/shorten; pending toggle | 3 | 6 |
| E7 | F7.1 rules list, F7.2 mobile, F7.3 queues, F7.4 records | OT detail screen; Reject modal; Create/Edit Rule; Add/Edit Holiday; bulk-approve selection; toasts; flagged-no-preapproval | 4 | 5 |
| E8 | F8.1 mobile + summary, F8.2 HR archive+detail | Export flow; decrypt-fail detail; HR audit-note overlay; empty states; access-denied; PDF deferred-but-shown | 3 | 3 |
| E9 | — (no frame) | **F9.3 Review Queue (entire — 9 states), F9.5 Cutover Console (entire — 5 states)** | 8 | 3 |
| E10 | F10.1 centers, F10.2 dashboards (no Super Admin), F10.3 report, F10.4 format-picker | Export progress/success/error; mark-read result; empty states; SL mobile dashboard; Super Admin variant | 7 | 11 |

---

## 5. Fix-sequencing recommendation

CLAUDE.md guidance: **one design session per design-system section** to avoid token bloat. Total estimate **~16 sessions**.

### Wave 1 — Cross-cutting components & overlays (DO FIRST, ~3 sessions, Opus)

These eliminate 40–50% of downstream Wave 2 scope.

**Session 1.1 — Modals & overlays family**
- Reject-with-reason modal (generic) — instances: E1, E2, E5×2, E6, E7
- Bulk-approve confirmation modal (selected count, audit-note) — E5, E7, E9
- Destructive-confirm dialog template (E1-themed) — E1, E2, E3, E4, E7
- Discard-changes confirm (dirty forms) — E2, E3, E6, E9

**Session 1.2 — Result states & feedback**
- Toast variants: approve / reject / save / error / queued (wire to `PtJHa` / `tplBu`)
- Loading skeleton family (list, detail, card)
- Empty-state family: filtered-zero, fresh-no-data, no-permission/403, session-expired
- Mark-as-read notification card transition

**Session 1.3 — Export framework end-to-end**
- Multi-step modal: Format → Progress/Queued → Success-with-download → Error (E10 owns; consumed by E1/E3/E5/E6/E7/E8/E10)
- Confidentiality-marking variant (E8)
- PDF-size warning variant (E10 C-2)

### Wave 2 — Per-epic BLOCKER fixes, dependency order (10 sessions, Sonnet OK)

- **2.1 E1**: Forgot/Reset password (web+mobile), Tambah Pengguna, Edit/Ubah Peran drawer + row-kebab, Failed login + lockout, Disabled-account rejection, Audit log detail drawer, Settings IA (needs D2)
- **2.2a E2 master-data foundation**: ClientCompany list/detail/create-edit + **geofence_radius_m editor + map picker** (§8 locked), Service Lines + nested Positions CRUD
- **2.2b E2 remainder**: Employment Agreement (list/create PKWT-PKWTT/renew/close), Leave Types CRUD, Attendance Codes CRUD (color+flags), Overtime Rules CRUD, HR approval queue for agent change requests, wire row-kebabs, decide JBjBb tabs (D3)
- **2.3 E3**: Transfer modal (F3.3 entire), Renew modal, End/Terminate confirm+reason, Assign/Reassign Shift Leader picker (INV-2/3/4), Roster row→Detail wiring, Create form error variants
- **2.4 E4**: Shift-picker popover (core F4.2), Cell-edit/clear menu (F4.4), Conflict-block toasts, Bulk apply-to-range, Auto-publish toast, Edit Shift modal + deactivate, **fix min_minutes display** (S1)
- **2.5 E5**: F5.4 Corrections entirely new (agent form + tracker + leader/HR queue + detail + reject modal), Clock-out variant, Out-of-geofence, Unscheduled flag, GPS-unavailable, Reject + Bulk-approve (uses Wave 1), Leader-own-record escalation treatment
- **2.6 E6**: Reject-with-reason (Wave 1), Approve/Reject results (Wave 1), **SL Leave Detail screen (L1 variant)**, No-leader timeline variant, Quota-exceeded blocked-submit + missing-doc error, Calendar approved+pending toggle
- **2.7 E7**: **OT detail screen** (web + mobile sheet — central decision UI), Reject modal (Wave 1), Bulk-approve selection + checkboxes, Create/Edit OT Rule, Add/Edit Holiday, Worked-without-request flag, Auto-detect confirm result + dismiss, **fix min_minutes** (S1)
- **2.8 E8**: Resolve PDF contradiction (D5), Excel export flow (Wave 1.3), Decrypt-fail detail state (links E9), HR audit-note annotation overlay (§8 lock), Read-only "Final" pill near header
- **2.9 E9 (build from zero)**: 2.9a Reconciliation Review Queue (queue + 5 resolve drawers + recon report dashboard + bulk-resolve confirm); 2.9b Cutover Console (readiness dashboard + validation gates + go/no-go modal + run history)
- **2.10 E10**: Export progress/success/error (Wave 1.3 instanced), Notification empty + mark-read (Wave 1.2), Approval-inbox empty state, Agent dashboard empty state, Super Admin variant (D1), SL mobile dashboard + SL mobile notifications, Report empty + pending-records callout

### Wave 3 — HIGH-severity gaps (~4 themed sessions)

- **3.1 Cross-epic linking & navigation**: annotate post-login routing, employee-detail tabs decision (D3), history-chain navigation, report deep-links
- **3.2 Mobile leader surfaces**: SL mobile dashboard + leave approval + OT approval + unified mobile leader nav
- **3.3 Validation & error variants**: per-form copy for forms across E1/E2/E3/E5/E6/E7
- **3.4 Terminal states & lifecycle**: E3 Ended/Terminated/Resigned/Superseded; E6 cancel/shorten; E7 cancel/withdraw; E8 access-denied; expiring lists

### Wave 4 — MEDIUM/LOW polish (~2 sessions)

Settings IA, persona inconsistencies, pagination/hover, mobile parity gaps, audit-trail viewers, history navigation, picker openness.

**Model strategy:** Wave 1 = Opus (foundations propagate). Waves 2–4 = Sonnet for screen generation once design-system additions are locked.

---

## 6. Decisions / clarifications needed from the user

| # | Question | Source | Default |
|---|---|---|---|
| D1 | Super Admin dashboard — distinct variant or "same as HR"? | E10 P1 | Same as HR with role-label change; document explicitly |
| D2 | Settings IA — tabs on Pengaturan, sub-nav, or three sidebar siblings? | E1 P2 | Left sub-nav under Pengaturan |
| D3 | E2 employee-detail tabs (Penempatan/Kehadiran/Cuti&Lembur/Dokumen) — each epic adds a sub-view, or reduce strip to Profil only? Dokumen has no PRD. | E2 + cross-epic | Reduce to Profil; deep-link cards on Profil |
| D4 | **OT `min_minutes`** — §8 locks 60m but design shows 30m/0m + "<30 menit" copy. Confirm 60m. | E7 P0 (spec defect) | 60m everywhere |
| D5 | **E8 PDF download** — §8 defers PDF but design has 3 PDF/download affordances (`xAXSf`, `QYVEV`, implication on `RFJJj`). Remove or reopen? | E8 P0 | Remove the three; Excel-only export v1 |
| D6 | Calendar approved+pending toggle default | E6 P1 (F6.5 §10 open) | Approved by default; toggle to show pending |
| D7 | Mobile leader scope — mobile in v1 or web-only? | E5/E6/E7 + E10 P1 | Web-only v1; mobile leader deferred |
| D8 | HR override / force-approve UI — distinct affordance or invisible? | E6 P3, E7 P3 | Defer to v1.1; not surfaced |
| D9 | "Impor" button on E2 Karyawan — owned by E9 or separate? | E2 BLOCKER | Owned by E9; remove or relabel "Lihat status migrasi" |
| D10 | Notification preferences entrypoint — omit or stubbed-disabled? | E10 P1 | Disabled stub so future IA slot exists |
| D11 | Geofence-disabled state when ClientCompany has no lat/lng | E2/E5 | Banner on Detail + flag on Clock-In |
| D12 | E1 persona inconsistency — Topbar shows "Rudi Wijaya · Shift Leader" on HR-only screens (`kHNWT`, `rtJRB`) | E1 LOW | Change to HR persona |

---

## 7. Spec defects (PRD bugs — separate doc-reconciliation pass)

| # | Defect | Source | Resolution |
|---|---|---|---|
| **S1** | E7 overtime-rules.md / overtime-capture.md show min_minutes = 30 (and "<30 menit" copy). EPICS §8 locks **60**. | E7 audit §5 + §7.4 | Update PRD copy to match §8 |
| **S2** | E8 PDF download mentioned in PRDs and visible in design, but §8 defers. Ambiguous wording. | E8 audit §2.1 + §7 | Reconcile PRD with §8 |
| **S3** | F6.5 §10 "approved+pending toggle default" marked open but §8 doesn't resolve. | E6 audit §2.4 | Add to §8 |
| **S4** | E9 reconciliation-review.md §10 asks "which issue types are blocking" — already resolved in §8. PRD §10 needs reconciling per "§8 wins" rule. | E9 audit notes | Delete open question from PRD §10 |
| **S5** | E4 shift-master-catalog.md §10 has open items (multiple breaks; 24h shift) — need explicit resolution or "deferred" marker. | E4 audit §8 | Add §8 row or mark deferred |
| **S6** | E2 detail tab "Dokumen" has no PRD reference. | E2 audit §2.1 | Add PRD or remove tab (see D3) |
| **S7** | E1 mobile Pengaturan / "Tetap masuk" toggle is on web Pengaturan but agents are mobile-only — contradicts AU-3 intent. | E1 audit §7 | Add note to F1.4 |
| **S8** | E5 "WFO" attendance code referenced informationally; mapping to legacy `is_wfo` unclear. | E5 audit §6 | Add note to F2.5 or F5.5 |
| **S9** | E1 audit log pagination — AL-1 implies high volume but design has no pagination controls. | E1 audit §7 | Add explicit pagination requirement to F1.3 |
| **S10** | E10 "Perlu Tindakan" panel doing double duty as widget + leader's combined inbox — DB-3 leans yes; not in §8. | E10 audit §8 | Add §8 decision |
