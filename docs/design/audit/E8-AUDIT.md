# E8 Payroll — Design Audit

**Date:** 2026-06-02
**.pen frame:** KLvfV (▦ FEATURE GROUP · E8 Payroll)
**Specs:** FEATURE.md + 2 PRDs (payslip-history.md, payroll-archive.md) + DATA-MAPPING.md
**Decisions baseline:** EPICS.md §8 — E8 is read-only history; PDF download deferred; immutable with HR audit-note annotation.

---

## 1. Screen inventory

| # | Screen name | Frame ID | Platform | Purpose | Reachable from |
|---|-------------|----------|----------|---------|----------------|
| 1 | E8 · Arsip Payroll (HR) | `jBgLn` | Web Console | HR/Super Admin browse the migrated payroll archive (list view: employee × period rows; filters by year/month/search; one "Perlu review" row indicating decrypt anomaly). | Sidebar nav · "Payroll" (per Topbar crumb "Payroll / Arsip"). Row chevrons (`ckssi`, `kBqQh`, `QcGT8`, `Kc2zw`, `c6QgI`) imply drill into detail. |
| 2 | E8 · Detail Slip Gaji (HR) | `JaScP` | Web Console | HR/Super Admin full payslip detail: Pendapatan (earnings line items), Potongan (deductions), Take-Home highlight, Benefit (employer-borne BPJS), Informasi (period/working days/status/source), confidentiality note. | From list row click on `jBgLn` (implicit). |
| 3 | Agen · Slip Gaji (Daftar) | `v8uQX2` | Mobile (Agent) | Agent's own historical payslip summaries by period — take-home, gross/deduction breakdown, chevron into detail. 2026 list shown (Mei → Februari). | BottomNav · "Gaji" tab (`CqRc5`). |
| 4 | Agen · Detail Slip (Ringkasan) | `ocsq4` | Mobile (Agent) | Per-payslip summary detail (take-home hero, Ringkasan totals, Hari Kerja/Periode, "Hanya ringkasan" disclosure). Top bar has download icon (`QYVEV`). | From list card click on `v8uQX2`. |

**Total designed:** 4 screens (2 web, 2 mobile). **Total in-frame overlays/states:** 0.

---

## 2. Dead-end findings

### 2.1 Unwired clickable components (cat a)

| Sev | Where | What | Why dead |
|---|---|---|---|
| **High** | `jBgLn` → `RFJJj` ("Ekspor (RAHASIA)" button) | Primary export-to-Excel/Audit CTA. | No export modal, no progress state, no completion toast designed anywhere in E8 — clicking has no designed result. PA-5 / PA-7 require this to be audited and produce a file. |
| **High** | `JaScP` → `xAXSf` ("Ekspor PDF" button on detail header) | Detail-page export. | EPICS.md §8 explicitly defers PDF, yet the button is rendered with `download` icon and "Ekspor PDF" label. Either contradicts the deferred-PDF decision **or** is unwired (no PDF preview/modal/toast designed). |
| **High** | `ocsq4` → `QYVEV` (download icon in mobile AppBar) | Mobile download affordance. | Same conflict as above — PDF download was deferred in v1. Icon present, no destination state designed. |
| **Medium** | `jBgLn` row chevrons (`ckssi`, `kBqQh`, `QcGT8`, `Kc2zw`, `c6QgI`) | 5 list rows imply drill into detail. | Only one detail screen exists (`JaScP` for "Budi Santoso"). Acceptable as representative, but no visual cue (e.g. selected-row indicator) confirms which row maps to which detail. |
| **Medium** | `jBgLn` row `b5tEou` ("Rudi Hartono · Perlu review") | "Perlu review" warn-chip on a decrypt-anomaly row. | Tapping is implied (chevron present `c6QgI`) but the **decrypt-fail review/error detail screen is missing** (C-1 in payslip-history.md, C-1 in payroll-archive.md, E9 decrypt_fail blocking item). |
| **Medium** | `jBgLn` filters (`jN6Se` year, `B3K1fA` month, `adEoE` search) | Filter changes have no designed empty/zero-result state. | If filter narrows to 0 results, no state shown. |
| **Low** | Mobile BottomNav · `b1BD0A` "Profil" + `gIPMk` "Jadwal" + `xPcj5` "Beranda" | Cross-tab nav from `v8uQX2` / `ocsq4`. | Out-of-epic by design (other epics own these); not a true dead-end for E8 audit. |

### 2.2 Orphan screens (cat b)

None. All 4 screens are entry-pointable: HR via Sidebar → Payroll; Agent via BottomNav → Gaji; details via list row click.

### 2.3 Missing result states (cat c)

| Sev | Action | Missing result |
|---|---|---|
| **High** | HR clicks "Ekspor (RAHASIA)" on list | No export-config modal (period / employee / what columns), no in-progress state, no success toast referencing "audit log entry created with confidentiality marking" (PA-5, PA-7). |
| **High** | HR clicks list row for decrypt-anomaly record (`b5tEou`) | No "decryption failure / flagged for migration review" state. C-2 (payslip-history) and C-1 (payroll-archive) require: surface unavailability + flag for migration review without crashing. |
| **High** | HR opens a payslip — no audit-note input | The §8 decision "HR may annotate via an audited note (no edits)" has **no UI surface** in any designed screen. Detail `JaScP` has a confidentiality note (`iqEjt`) but no annotation/audit-note input affordance. |
| **Medium** | Agent or HR with no payroll history (C-1 payslip-history) | No empty state on `v8uQX2` (mobile list) or `jBgLn` (web list). |
| **Medium** | Agent attempts the AppBar "download" icon (`QYVEV`) on `ocsq4` | No state — either toast "Belum tersedia" (PDF deferred) or remove the icon. Currently misleads users. |
| **Low** | C-3 payroll-archive: component totals vs payslip mismatch | No visual flag/discrepancy state in `JaScP`. |
| **Low** | C-4 payroll-archive: large export queued/streamed | No queued/streaming variant of the export interaction. |

### 2.4 Untriggered overlays (cat d)

No overlays exist in E8 at all (export modal, decrypt-fail modal, annotate-note dialog, confirmation toasts). This is the dominant gap.

### 2.5 Dangling back/close (cat e)

| Sev | Where | What |
|---|---|---|
| **Low** | `JaScP` (web detail) | No explicit back/breadcrumb back-link visible inside the content area (Topbar reads "Arsip / Payroll" but there is no in-content "← Kembali ke Arsip" — acceptable if Topbar crumb is clickable, but unverified). |
| **Low** | `ocsq4` AppBar back (`BctK7`, chevron-left) | Wired in pattern (returns to `v8uQX2`); fine. |

---

## 3. Missing screens (cat f)

Listed in priority order:

1. **HR Export modal** — period range / scope / format options + confidentiality reminder (PA-5).
2. **Export in-progress state** — toast or inline progress (C-4: large export queued).
3. **Export success toast** — confirms audit-log entry written (PA-7).
4. **Decrypt-fail / unavailable record state** — for both list-row flag-only (already designed via warn chip) **and** the detail page when opened from `b5tEou`-style row (currently nothing) — must surface "record under migration review" without crashing (C-2 payslip-history, C-1 payroll-archive, ties to E9 blocking item).
5. **HR audit-note annotation overlay** — input field + audit-log preview, fulfilling §8 "HR may annotate via an audited note (no edits)". Decision exists; surface does not.
6. **Empty state — agent no payroll history** (mobile `v8uQX2` variant, C-1 payslip-history). Probable copy: "Belum ada slip gaji yang tercatat".
7. **Empty state — HR archive zero results** (web `jBgLn` variant when filters yield none).
8. **Access-denied state** — agent tries to access another agent's payslip (Gherkin scenario "Agent cannot see others' payslips"), and shift-leader/agent tries to open the archive (Gherkin scenario in payroll-archive.md). Both are role-gate denials with no designed UI.
9. **Loading skeleton** for list + detail (decrypt-on-read latency makes a loading state non-trivial).
10. **PDF-deferred toast/disabled affordance** — OR remove the `download` icon (`QYVEV`) and the "Ekspor PDF" button (`xAXSf`) until PDF is in scope.

---

## 4. PRD coverage matrix

| PRD | Required screens/states | Designed | Missing |
|---|---|---|---|
| **F8.1 Payslip History (mobile + web list)** | (a) Agent mobile list of own payslips; (b) Agent mobile detail summary; (c) HR view of any agent's summaries; (d) Empty state (C-1); (e) Decrypt-fail (C-2); (f) Access-denied (Gherkin); (g) Read-only indicator | (a) `v8uQX2`; (b) `ocsq4`; (c) partially via HR archive `jBgLn`+`JaScP` (HR sees same data + more); (g) "Hanya ringkasan" disclosure `MtNDm` is informational, not a strict read-only indicator | (d) empty, (e) decrypt-fail detail, (f) access-denied. PDF download conflict on `QYVEV`. |
| **F8.2 Payroll Archive (HR)** | (a) HR archive list with search/period filter; (b) Detail with payslip + components + benefits; (c) Export with confidentiality marking + audit log; (d) Search-by-period; (e) Decrypt-fail flag (C-1); (f) Mismatch discrepancy flag (C-3); (g) Large-export queued (C-4); (h) Annotated audit-note input (§10 + §8 decision); (i) Confidentiality marking on screen | (a) `jBgLn` (incl. filters `jN6Se`/`B3K1fA`/`adEoE`); (b) `JaScP` (Pendapatan + Potongan + Benefit + Info); (e) row-level warn chip `h5w7t` "Perlu review"; (i) `glGDo` banner + `iqEjt` note + Topbar lock | (c) export modal/progress/done toast; (g) queued state; (h) audit-note input; (f) discrepancy detail UI; access-denied state for non-HR roles. |
| **DATA-MAPPING.md** | Decrypt-fail surfacing per G-1 ("failures → review queue, never null") | Row warn chip on `b5tEou` ("Perlu review") | Detail page when opened: no "under migration review" panel. |

---

## 5. Business-rule state check

- **Read-only / no-edit indicator on payslip view:** **partial.** `JaScP` shows a confidentiality lock note (`iqEjt`) — communicates RAHASIA/audited, but does **not** explicitly say "read-only / cannot be edited". Mobile `ocsq4` has an informational note ("Hanya ringkasan...") that mentions HR holds the breakdown, not the immutability of the record itself. Invariant INV-1 / PH-4 / PA-4 not visually reinforced on the detail surface (no "Final · tidak dapat diedit" pill near the header). The "Status: Final (migrasi)" row in `Dadnb` is the closest signal but is buried in the Informasi card.
- **HR audit-note input (annotate):** **no.** The §8 decision "HR may annotate via an audited note (no edits)" has zero UI representation in E8.
- **Export-to-Excel flow (modal/progress/done):** **no.** Button exists (`RFJJj`); no modal, no progress, no success toast, no audit-confirmation feedback.
- **Decrypt-fail error state:** **partial.** Row-level warn chip exists on the list (`b5tEou` → `h5w7t`); the **detail-page state when opening such a record is missing**. No "under migration review" panel inside `JaScP`-equivalent.
- **Empty state (no payroll history):** **no.** Neither mobile `v8uQX2` nor web `jBgLn` has a "Belum ada riwayat" variant.

---

## 6. Cross-epic references found

- **E2 (Employees):** rows in `jBgLn` use `comp/Avatar` (`YVANc`) keyed by initials (BS, SA, AP, DL, RH) — implies employee lookup; no link/affordance to jump to E2 employee profile from a payslip (HR-only convenience gap, not a spec violation).
- **E9 (Migration):** the "Perlu review" warn chip on row `b5tEou` is the visible touch-point with E9's `decrypt_fail` blocking item (DATA-MAPPING.md G-1, "failures → review queue"). No link to the migration review queue UI (which lives in E10/E9).
- **E10 (Reporting/Exports):** PA-5 references "exports include a confidentiality marking" and PA-6/PA-7 reference retention + audit log. The export tooling is owned by E10 per the PRD's Dependencies list — so the missing export modal might legitimately live in E10's design frame. **Recommendation:** if E10 owns the export, leave a stub reference in E8 (e.g. a comment node) rather than relying on an unwired button.
- **E1 (RBAC/Audit):** access-denied scenarios (Gherkin: agent → another's payslip; shift-leader → archive) have no E8-side denial screen; this may also be owned by E1 cross-cuttingly, but at minimum the agent-mobile path needs a friendly empty/denied variant.

---

## 7. Prioritized recommendation

**P0 — block before build:**
1. Resolve the PDF contradiction. EPICS.md §8 defers PDF; the design has three PDF/download affordances (`RFJJj` is OK as Excel/audit export, but `xAXSf` "Ekspor PDF" and `QYVEV` download icon directly contradict the decision). Either (a) remove these three affordances from v1, or (b) reopen the §8 decision. Recommend (a).
2. Design the **Excel/audit export flow** (`RFJJj` target): modal → in-progress → success toast referencing audit-log entry + RAHASIA marking (PA-5, PA-7). Confirm ownership split with E10.
3. Design the **decrypt-fail detail state** so that clicking a "Perlu review" row leads to a designed "under migration review" panel (links to G-1 review queue). This closes the loop on the E9 blocker.

**P1 — required by PRD acceptance criteria:**
4. Add **HR audit-note annotation overlay** to fulfill §8 immutable-with-audited-note decision (currently has zero surface).
5. Add **empty states** on mobile `v8uQX2` and web `jBgLn`.
6. Add an explicit **read-only / final** pill near the detail header on both `JaScP` and `ocsq4` so INV-1 is visible at a glance (lift the "Final (migrasi)" status out of the Informasi card).

**P2 — polish & robustness:**
7. Loading skeletons for list + detail (decrypt-on-read latency).
8. Access-denied screen variant (or rely on E1 — confirm ownership).
9. Discrepancy flag inline on detail when component totals ≠ payslip summary (C-3 payroll-archive).
10. Queued/streaming export variant for large exports (C-4).

---

## 8. Notes

- Frame `KLvfV` is at `y=35081`, width 6000; both POV rows are populated (HR/Super Admin · web with 2 screens; Agen · mobile with 2 screens).
- Strong points: the HR detail screen `JaScP` is comprehensive — Pendapatan + Potongan + Take-Home highlight + Benefit + Informasi + confidentiality note all present; the list `jBgLn` already models the decrypt-anomaly row with a warn chip (good — needs only a destination state).
- The agent-mobile design correctly limits visibility to summary level per INV-3 (no component breakdown surface), and the "Hanya ringkasan. Rincian komponen gaji tersedia di HR." disclosure (`MtNDm`) is on-spec.
- The "Status: Final (migrasi)" + "Sumber: lumen_swp" rows in the HR Informasi card are a nice migration-traceability detail aligned with DATA-MAPPING.md.
- Audit findings concentrated in two themes: **(1) wiring of the Export action is entirely missing** (no result states downstream of the button), and **(2) the §8 audit-note annotation decision has no surface at all** — both worth resolving in the next design session along with the PDF-deferral cleanup.
