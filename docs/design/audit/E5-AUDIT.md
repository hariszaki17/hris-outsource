# E5 Attendance — Design Audit

**Date:** 2026-06-02
**.pen frame:** `cjTd7` ("▦ FEATURE GROUP · E5 Kehadiran")
**Specs reviewed:** `FEATURE.md` + 5 PRDs (F5.1–F5.5), `DATA-MAPPING.md`, `EPICS.md §8`
**Method:** schema fetched once; `batch_get` traversal of `cjTd7` and `yTcDc` (interaction flows). No screenshots.

The E5 frame contains three POV "lanes" with `ScreensRow` containers:

- **POV · HR / Super Admin** (web, `HIzOC`) — 3 screens
- **POV · Shift Leader** (web, `IPlwC`) — 3 screens (scoped to "Plaza Senayan")
- **POV · Agen** (mobile, `GgvNC`) — 2 screens

---

## 1. Screen inventory

| # | Screen name | Frame ID | Platform | Purpose | Reachable from |
|---|-------------|----------|----------|---------|----------------|
| 1 | Screen 1 · Kehadiran — Dashboard (HR) | `sZCLW` | Web (HR) | Cross-company attendance table + KPI stats + tabs (Semua/Hadir/Terlambat/Eksepsi) + filters + Export | Sidebar entry; row "Detail" → screen 3 |
| 2 | Screen 2 · Verifikasi Kehadiran (HR) | `UEG2J` | Web (HR) | Exceptions queue + mini-stats (Pending / Late / Auto-clockout / Out-of-geofence) + bulk bar (Setujui Terpilih / Tolak) + per-row Setujui / Tolak / Detail | Topbar "Verifikasi (8)"; row → screen 3 |
| 3 | Screen 3 · Detail Verifikasi (HR) | `VY894` | Web (HR) | Single-record review: Ringkasan, Lokasi & Geofence (with map), Riwayat & Audit timeline, Tinjauan Verifikasi (Setujui / Tolak & Minta Koreksi), Catatan Agen | Row "Detail" / queue row click |
| 4 | E5 SL · Team Attendance — Plaza Senayan | `V2QL7` | Web (SL) | Same dashboard scoped to one company; warning ScopeBanner ("Cakupan terbatas") | SL sidebar |
| 5 | E5 SL · Verifikasi — Plaza Senayan | `MsXnm` | Web (SL) | SL-scoped exceptions queue; identical structure to #2 with ScopeBanner | SL topbar Verifikasi (4) |
| 6 | E5 SL · Detail Verifikasi — Plaza Senayan | `RZPQz` | Web (SL) | Detail with ScopeBanner | Queue row |
| 7 | Agen · Absen (Clock In/Out) | `Iek78` | Mobile (Agent) | Today's shift card, live clock, GPS status pill ("Dalam radius 32 m" — in-radius variant), big Clock-In button, info note about out-of-radius behavior, Masuk/Keluar today summary | Mobile bottom-nav "Absen" |
| 8 | Agen · Riwayat Kehadiran | `PAOwr` | Mobile (Agent) | Monthly summary stats (Hadir/Telat/Tdk lengkap) + day cards with shift times, status pills (Hadir / Terlambat / Pulang awal / Tidak lengkap), per-day verification badges (Terverifikasi otomatis / Menunggu verifikasi / Auto clock-out oleh sistem) | Mobile bottom-nav (likely a "Riwayat" sub-tab on Absen) |

**Total designed:** 8 screens (6 web, 2 mobile). All `clip:true`, viewports 1440×1024 (web) and 390×844 (mobile).

---

## 2. Dead-end findings

### 2.1 Unwired clickable components (cat a)

Clickable affordances drawn but with no designed result state for what happens next:

1. **`ClockInBtn` (`VKEmF`)** on Agen · Absen — the primary brand-green Clock-In button has no designed post-tap state (no success-modal/toast frame, no in-radius-success screen variant, no out-of-radius confirmation, no GPS-missing prompt, no "second clock-in blocked" variant, no Clock-Out variant of the same button once clocked in). The "Note" frame `xrckN` describes the out-of-radius behavior verbally but no screen shows it.
2. **`BtnExport` (`n3iXJG` HR dashboard; `sH3yW` SL dashboard)** and the secondary `BtnExp` on the verification pages (`YQ0gi`, `KvfCs`) — no export-modal, format-picker, or "export queued" toast designed in the E5 lanes. The yTcDc interaction-flows frame has a generic "Ekspor laporan" flow (`yPNyD`) but it is shared across epics and no E5-local result is present.
3. **`BtnReset` (`MZy0J`, `tIFmU`)** on the filter rows — no empty-state / reset-confirmation visual.
4. **Tab `Eksepsi` (`CD2yC`, `dfoOK`)** — no filtered-only-exceptions variant of the table is drawn; the table rows shown are mixed-status. The "Semua" tab is shown as active in both HR and SL dashboards.
5. **Segmented `Hari Ini / Minggu / Bulan` (`syPtn`, `kzUlc`)** — no week or month variant of the dashboard exists.
6. **Per-row inline actions `Setujui` (`o44lNf`) and `Tolak` (`AnhEx`)** on the verification queue — only the `Tolak` action has a designed downstream (the yTcDc "Tolak & minta koreksi" flow ends with a generic "Status → Ditolak · agen dinotifikasi" step card). The single-approve from a row has no E5-local success-toast frame; only the abstract yTcDc step-card exists.
7. **`Bersihkan` link in BulkBar (`ko9Af`, `FHWax`)** — clear-selection affordance has no visual variant.
8. **`Detail` row buttons (`MeJPe`, `T3pfde`)** wire to screens 3/6 — these are correctly wired.
9. **Pagination chevrons in BackRow (`iwqVh`, `L02lP`, `dW6Dl`, `XvynG`)** show "Eksepsi 2 dari 8" but no second-record variant of the Detail screen is drawn.
10. **`Tolak & Minta Koreksi` button (`f95dLU`)** on Detail — the modal-alasan referenced in the yTcDc flow `Q08YN` is described as a step card only; **no actual reject-reason modal frame exists** under `hoY3q` (overlays) for E5, nor inline. See §5.

### 2.2 Orphan screens (cat b)

No orphan screens detected — every designed screen is reachable through the sidebar/topbar/row-click chain.

### 2.3 Missing result states (cat c)

Per the DESIGN-SYSTEM "no dead-flow" rule, every action should land somewhere designed. Missing result-state frames specific to E5:

1. **Clock-in success state** (in-radius) — no "Anda sudah clock-in pukul HH:MM" rendered variant of the mobile Absen screen.
2. **Clock-out screen variant** — the Clock-In button never changes to a Clock-Out button in any drawn frame; the Masuk/Keluar today summary stays "—:—".
3. **Out-of-geofence clock-in warning/confirmation** — no warning sheet, no "Tetap clock-in?" confirm modal.
4. **GPS unavailable prompt** — clock-in-out PRD AC requires "if location services off, prompted to enable" — not drawn.
5. **Late-grace boundary states** — no ≤15 min "Tepat waktu (toleransi)" variant; no >15 min "Terlambat · butuh verifikasi" inline warning on the mobile clock-in card.
6. **Auto-clock-out indicator on Detail** — partially present in Riwayat audit timeline (`ShlXI` "Dialihkan ke antrean eksepsi") but no dedicated "Auto-closed by system" callout on the Detail HR/SL header.
7. **Correction-applied/re-evaluated banner** — no "Catatan ini telah dikoreksi · nilai asli tersedia" indicator on a Detail screen variant (PRD F5.4 CR-5 requires preserved snapshot + indicator; PRD F5.5 C-5 requires the indicator).
8. **Bulk-approve confirmation modal** — yTcDc flow `HSOqG` references "Konfirmasi massal — Setujui N catatan?" step but no actual modal frame in `hoY3q` overlays appears for E5 nor inline.
9. **Approve success toast** — referenced as "Toast hijau · status baris → Terverifikasi" in step cards; no E5-local Toast instance drawn on any screen.
10. **HR escalation indicator** — VF-7 / leader-own-record escalation: no badge or banner on the Verifikasi screen for "Eskalasi HR (catatan Anda sendiri)" items. The SL ScopeBanner mentions escalation but no visual treatment for the row.
11. **Empty states** — no "Belum ada eksepsi" (queue empty), no "Belum ada riwayat" (agent has no records), no "Tidak ada hasil filter".
12. **Stale Pending aging** (VF C-2) — no visual treatment / aging chip.

### 2.4 Untriggered overlays (cat d)

Sections referenced by interaction-flow step cards but not realized as overlay frames in `hoY3q`:

1. **Single approve confirmation** (yTcDc step `OKUq9` "Konfirmasi · Setujui kehadiran?")
2. **Bulk approve confirmation** (yTcDc step `y19KT` "Konfirmasi massal · Setujui N catatan?")
3. **Reject reason modal** (yTcDc step `Dqkrv` "Modal alasan · Alasan penolakan wajib diisi", `BgSen` validation state) — described in step cards but no overlay frame produced.
4. **Approve/Reject loading overlay** (yTcDc steps `qDEGw`, `jwZvA` "Memproses · Progress overlay memblokir aksi").
5. **Approve / Reject / Bulk success toast** — no Toast `PtJHa` instance is rendered on any E5 screen.

These five overlays are explicitly named by the interaction-flows section but only appear as step-card descriptions; the actual overlay artifacts a developer would screenshot are missing.

### 2.5 Dangling back/close (cat e)

1. **BackRow** (`o4n6E`, `J61P0F`) "Kembali ke Verifikasi" — correctly maps back to screen 2/5.
2. **Mobile screens** have no back arrow / close drawn for the Riwayat screen; bottom-nav is the only navigation, which is acceptable.
3. **Reject reason modal** (when designed) will need a close/cancel state — currently undesigned (see §2.4).

No dangling back/close affordances found in *existing* frames.

---

## 3. Missing screens (cat f)

The following PRD-required surfaces have **no frame** in `cjTd7`:

| Missing screen | PRD ref | Why required |
|----------------|---------|--------------|
| **Agent correction-request form / sheet** (Mobile) | F5.4 CR-1, AC "File and approve a missed clock-out correction" | Agents must be able to file a check_in / check_out / code correction with reason; no form drawn. |
| **Agent correction-tracking list** (Mobile) | F5.4 §4 ("File a correction for own record; track status") | Agent should see Pending / Approved / Rejected status; the Riwayat day cards do not include correction status pills. |
| **Leader correction-approval queue / detail** (Web) | F5.4 §4 + AC "the shift leader approves it" | Leader/HR have nowhere to review pending corrections. The Verifikasi screen is for exceptions on `Attendance`, not on `AttendanceCorrection`. |
| **Reject-correction reason modal** | F5.4 CR-3 ("reason required on reject") | Distinct from attendance-record reject; correction-reject also needs reason capture. |
| **Bulk-verify confirmation modal/overlay** | F5.3 VF-6 "Bulk approve" | Required by INV-3 routing volumes (many slightly-late records). Step card exists in yTcDc; no actual overlay. |
| **Reject-attendance reason form/modal** | F5.3 VF-5 "Reject → Rejected with required reason" | Step card exists; no overlay frame. |
| **Leader-own-record escalation surface** | F5.3 VF C-5 + EPICS §8 "Leaders' own exceptions → escalate to HR" | No visual indication / filter / state on the Verifikasi screen distinguishing escalated-from-SL items in the HR queue. |
| **Clock-out variant of mobile Absen** | F5.1 CI-6 + AC "Clock out" | Card must flip to "Anda sedang bekerja · Tap untuk Clock Out" + button label change + closing time. |
| **Out-of-geofence clock-in warning** | F5.1 CI-3 + AC "Clock in outside the geofence is allowed but flagged" | No designed warning sheet / confirm. The info-note text mentions it but no result state. |
| **Unscheduled clock-in flag badge / banner** | F5.1 CI-4 + EPICS §8 "Unscheduled clock-in = allowed + flagged" | No "Tanpa Jadwal" badge variant on Today's-shift card or in the queue row. |
| **GPS-off / GPS-permission prompt** | F5.1 AC "GPS unavailable" | Required by Gherkin. No frame. |
| **Auto-clock-out toast / push notification to agent** | F5.2 EV-3 + F5.5 AR-2 | The system auto-closes; agent should be informed. No Toast/Push variant on mobile. |
| **Absent state on mobile Riwayat** | F5.2 EV-4 | The Riwayat day cards include Hadir / Terlambat / Pulang awal / Tidak lengkap but no Absen card; PRD requires Absent status to appear. |
| **Billable rollup view / export preview** | F5.5 AR-4 | Dashboard exports `Ekspor Laporan` but the rollup view itself is not designed; PRD treats it as "first-class output". |
| **Edit clock-in/out from Detail (correction shortcut)** | F5.4 §3 ("Shift Leader / HR may also file on behalf") | No "Buat Koreksi" action button on the Detail screen. |
| **HR-only override / re-open verified record** | F5.3 C-1 | Already-verified record reopening flow is undesigned. |

---

## 4. PRD coverage matrix

| PRD | Required screens / states | Designed | Missing |
|-----|---------------------------|----------|---------|
| **F5.1 Clock In/Out (GPS geofence)** | Mobile clock-in (in-radius), out-of-geofence warning, unscheduled flag, GPS-off prompt, clock-out variant, second-clock-in blocked | Mobile in-radius clock-in card (`Iek78`) only | Out-of-geofence warning; unscheduled badge; GPS-off prompt; clock-out variant; double-tap blocked; clock-out cross-midnight visual |
| **F5.2 Attendance Evaluation & Auto-Close** | Auto-clock-out evidence in records, Late flag rendering, Absent status, re-evaluated indicator | Late status pill on table rows; auto-clockout time string on Riwayat (`UX4Wo` "auto 15:00"); audit-timeline auto-close step | Absent status pill (none in inventory); re-evaluated post-correction indicator |
| **F5.3 Shift-Leader Verification (exceptions only)** | Exceptions queue, bulk approve, single approve, reject + reason, HR escalation, scope-only-own-company | Verifikasi queue + bulk bar + per-row Setujui/Tolak/Detail + Detail with Tinjauan card; SL ScopeBanner | Bulk confirm modal; reject-reason modal; success toast; HR-escalation row treatment; "Selesai" segment populated state; auto-verify SLA aging |
| **F5.4 Attendance Corrections** | Agent file-correction form, agent correction tracker, leader correction queue/detail, reject-reason modal, applied-with-snapshot indicator | **None** | All of: agent correction form (mobile); agent correction tracker; leader correction queue + detail; correction-reject modal; "telah dikoreksi · snapshot tersedia" indicator on Detail |
| **F5.5 Attendance Records & Dashboard** | Agent self-history (mobile), leader team view, HR cross-company view, exception-only filter, billable rollup, export, audit-of-export | Mobile Riwayat (`PAOwr`); HR dashboard (`sZCLW`); SL team view (`V2QL7`); filters + Eksepsi tab; Export buttons | Empty states; Eksepsi-tab filtered variant; billable-rollup view; export-format picker / queued state; export-audit log; correction-applied indicator (AR §C-5) |

---

## 5. Business-rule state check

| State | Designed? | Evidence |
|-------|-----------|----------|
| **Outside-geofence error / flag** | **No** | Mobile Absen shows only "Dalam radius · 32 m" (`ejKAi`, $ok-bg). No out-of-radius variant of the GPS pill, no warning sheet, no flagged-row treatment in the queue beyond a generic Terlambat pill. The HR queue row `d7aBR` shows "Dalam radius · 24 m" — also in-radius. |
| **Late grace (≤15 min OK)** | **Partial** | The Detail Ringkasan card sub-header reads "Toleransi 15 mnt" (`gwsyt`); the Tinjauan ExBox (`JeVex`) reads "Clock-in 07:18 — 18 menit dari jadwal 07:00 (toleransi 15 menit)". The boundary rule is *named* but no on-time-within-grace variant of the queue row is drawn — only the >15 case appears. |
| **Late grace (>15 min late flag)** | **Yes** | Queue row `d7aBR` "Terlambat 18 mnt" + "Toleransi 15 mnt"; Detail ExBox same. |
| **Unscheduled clock-in flagged but allowed** | **No** | No "Tanpa Jadwal" pill / banner / row variant anywhere. PRD CI-4 + EPICS §8 lock-in is not represented. |
| **Bulk-verify confirmation** | **No** | BulkBar (`zTNBz`, `n5RoYy`) and yTcDc step `y19KT` describe it but no overlay/modal frame exists. |
| **Reject + correction-request form** | **Partial** | `f95dLU` "Tolak & Minta Koreksi" button drawn on Detail; yTcDc `Q08YN` flow describes 5 step-cards including "Modal alasan" + "Validasi" + "Terkirim". The **modal frame itself is not designed** (no overlay in `hoY3q`, no inline frame). Correction request form for agents is entirely absent (see §3). |
| **Leader own-record escalation to HR** | **Partial** | SL ScopeBanner (`Xm4Or`, `NcfqR`, `AJ9QM`) text says "pengecualian milik sendiri dieskalasi ke HR." No row badge / filter / state in HR's queue to identify escalated items, and no variant of the SL queue showing the agent's own record excluded. |

---

## 6. Cross-epic references found

- **E1 / Auth-scope:** SL ScopeBanner mentions "Cakupan terbatas — hanya perusahaan binaan Anda (Plaza Senayan)" — implements F3.4 SL-scope (E3). Correctly distinct from the HR cross-company dashboard.
- **E2 ClientCompany geofence:** Detail Lokasi & Geofence card shows "Radius geofence: 100 m" (`YhM6B`) — the locked-in default from EPICS §8.
- **E2 Attendance codes:** Detail timeline references "WFO" tag (`Q6nguN`, `UHuZX`) — partial mapping to legacy `is_wfo` (DATA-MAPPING G-4). The PRD says this is informational only; consider whether it should be a more clearly mapped attendance-code pill.
- **E3 Placement:** Row metadata "PT Graha Mandiri · Building Mgmt" (`l4ba9u`, `KU28Z`) — placement context surfaced. Correct.
- **E4 Schedule:** Shift cards display "Shift Pagi · 07:00–15:00" (`OfiEa`), "Pagi · 07:00–15:00" — schedule context surfaced. Correct.
- **E7 Overtime / E8 Payroll / E10 Reporting:** Export buttons exist but the rollup destination view is not designed in E5 (consistent with the F5.5 §10 note that billing lives in E10).
- **E6 Leave:** F5.2 C-6 (leave suppresses Absent) — no visual treatment of "On approved leave" status anywhere. Should appear as a status pill option once E6 surfaces are in flight.

---

## 7. Prioritized recommendation

Sorted by criticality (blocks PRD acceptance criteria first, then completeness, then polish):

**P0 — blocks Gherkin AC (must add before E5 can be called "designed"):**

1. **Mobile correction-request form** (F5.4 CR-1). Sheet/screen with type-selector (`check_in` / `check_out` / `code` / `other`), corrected-time picker, required-reason textarea, Submit/Cancel.
2. **Mobile correction tracking list + status pills** (F5.4 §4).
3. **Leader/HR correction-approval queue + detail + reject-reason modal** (F5.4 §3, CR-3).
4. **Reject-attendance reason modal** for `f95dLU` (F5.3 VF-5).
5. **Bulk-approve confirmation modal** for `zTNBz` / `n5RoYy` (F5.3 VF-6).
6. **Clock-out variant of mobile Absen** + button-state flip when clocked-in (F5.1 CI-6).
7. **Out-of-geofence clock-in confirmation/warning** (F5.1 CI-3 + Gherkin).
8. **Unscheduled clock-in flag** — badge on Today's-shift card *and* on queue row (F5.1 CI-4 + EPICS §8 decision).
9. **GPS-unavailable prompt** (F5.1 Gherkin "GPS unavailable").
10. **Approve/reject success Toast** (`PtJHa` instances) on both web and mobile — wire result states to existing Toast component.

**P1 — flow correctness / decision-log visibility:**

11. **Leader own-record escalation row treatment** in HR queue (EPICS §8 + VF C-5). Visual: escalation badge + filter chip.
12. **Correction-applied indicator** + "Lihat snapshot asli" affordance on Detail (F5.4 CR-5, F5.5 AR §C-5).
13. **Absent status pill** + filter variant + mobile Riwayat day-card for Absent (F5.2 EV-4).
14. **Auto-clock-out banner on mobile** (push/toast variant after the system closes a record) (F5.2 EV-3).
15. **Eksepsi-tab filtered table variant** in HR/SL dashboards (rows shown today are mixed).

**P2 — completeness:**

16. **Empty states**: empty queue, empty riwayat, no-filter-results, no-corrections.
17. **Loading overlay** (yTcDc steps `qDEGw`, `jwZvA`).
18. **Export format-picker / queued-state** for `n3iXJG` etc. (F5.5 AR-6 audit).
19. **HR cross-company billable-rollup view** (F5.5 AR-4) — even a "feeds E10" stub frame so the link is explicit.
20. **Stale-Pending aging chip** (F5.3 C-2).

**P3 — polish:**

21. Hari Ini / Minggu / Bulan segmented states.
22. "Selesai" segment populated variant on Verifikasi.
23. Second-record Detail variant to validate the BackRow pagination (currently the chevron has no destination).

---

## 8. Notes

- The audit was **read-only** per task constraints. No `batch_design` calls were made; the `.pen` file is unchanged.
- The interaction-flows frame `yTcDc` is well-formed and explicitly enumerates Verifikasi tunggal / Verifikasi massal / Tolak & minta koreksi as step chains, which makes the **gap between flow-step-cards and actual overlay frames** the central finding. The design-system contract ("no action ends in a blank or undefined state", per `OAHOA` recap text) is therefore violated for ~5 critical overlays (bulk confirm, single confirm, reject reason, loading, success toast).
- **F5.4 Corrections** is the largest blind spot — *no* correction-related screen exists on either platform, despite the PRD being one of five and EPICS §8 locking in the 7-day self-correction window.
- The two POV "lanes" (HR vs SL) duplicate the same dashboard / verification / detail triad. The only structural delta is the SL ScopeBanner. Consider whether the SL lane could be expressed as descendant overrides on a shared screen rather than copied frames, both to reduce drift and to make it easy to add the missing HR-escalation row treatment in one place.
- Mobile coverage is thin: 2 screens for 1 actor (Agent) vs. the PRDs that require clock-in/out, history, **correction request**, **correction tracking**, and notification surfaces. Mobile expansion is the single biggest delta to closing E5.
- All `comp/*` library components needed for the missing screens (`comp/Toast`, `comp/TextField`, `comp/BtnPrimary`/`Secondary`/`Danger`, `comp/FilterSelect`, `comp/StatusPill`) already exist as reusables — no foundations work blocks the recommendations.
