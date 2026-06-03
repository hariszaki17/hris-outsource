# E9 Migration — Gap Analysis

**Date:** 2026-06-02
**Status:** No design frame exists in `brainstorm.pen` for E9.
**Question:** How much of E9 needs UI? (Not "what's wrong with the E9 design" — there is no design to critique.)
**Verdict:** **E9 is ~80% backend, but the reconciliation review queue + cutover/validation console are first-class web UI** that HR/super-admin and ops actually operate. A small admin-console frame is required; it cannot live in the runbook alone.

---

## 1. PRD-by-PRD analysis

### F9.1 — extraction-staging
- **Actors:** Migration engineer / ops; System (snapshot, decrypt). No HR / Super Admin interaction.
- **Surface declared in PRD §4:** "Migration tooling (CLI/job) reading a dump/replica; staging database. **No mobile/web user surface** (reports surface in F9.3)."
- **UI needed?:** **No** (backend-only).
- **Rationale:** All actions — snapshot, stage, decrypt — are CLI/job operations. The only human-visible signal is the `decrypt_fail` review item, which surfaces inside F9.3's queue. EX-4 ("decrypt failure produces a REVIEW_ITEM") is the bridge.
- **If yes, screens:** —

### F9.2 — transform-crosswalks
- **Actors:** Migration engineer; System (transform, crosswalk).
- **Surface declared in PRD §4:** "Migration tooling (CLI/job) over staging → transformed dataset. **No end-user surface.**"
- **UI needed?:** **No** for the transform itself. **Optional/minimal** for crosswalk inspection during dry-runs (engineer-only diagnostic — could be a CLI report).
- **Rationale:** TR-1..TR-8 are deterministic rules. Anything ambiguous (TR-7) becomes a REVIEW_ITEM and flows to F9.3's queue. No human decisions are made in F9.2.
- **If yes, screens:** Optionally a **Crosswalk inspector** (read-only table: `legacy_table`, `legacy_id`, `new_table`, `new_id`, `run_id`) — but this is a "nice-to-have" engineer tool, not user-facing. **Out of scope for v1 UI.**

### F9.3 — reconciliation-review ⭐ **PRIMARY UI**
- **Actors:** HR / Super Admin (resolve), Migration engineer (monitor), System (queue, report, gate).
- **Surface declared in PRD §4:** **Web console** for HR/Super Admin to work the queue + **Web console** for engineer to view recon reports.
- **UI needed?:** **Yes — substantial.**
- **Rationale:** This is the entire human-in-the-loop step of E9. Five `issue_type` flows, three of them **blocking** (decrypt_fail, orphan_identity, unmatched_placement — per EPICS.md §8), two non-blocking (unclassified_service_line, ambiguous_chain). Placement-string matching is fuzzy + manual confirm. Bulk resolution (RC-5: "bulk-map identical placement strings") requires UI affordance. Resolutions are audited (RC-6).
- **Screens needed:**
  1. **Review queue — list view.** Filters by `issue_type`, `entity_type`, `status (Open|Resolved)`, `run_id`. Counts badge per issue type. Bulk-select checkboxes (RC-5). Severity indicator (blocking vs non-blocking). Sorted by blocking-first.
  2. **Review item — detail / resolve drawer (unmatched_placement).** Shows legacy free-text string + occurrence count + payload (employee, dates). Search-and-pick of ClientCompany; fuzzy suggestions ranked; **alias list** management; "Apply to all N identical strings" checkbox (bulk). Confirm + audit note.
  3. **Review item — detail / resolve drawer (orphan_identity).** Shows orphan `user_id` or `employee_id`, surrounding context. Actions: link to existing User/Employee, create new, mark as discardable (with reason).
  4. **Review item — detail / resolve drawer (decrypt_fail).** Shows entity_type + payload (column, row pointer — never the encrypted value). Actions: retry with re-keyed value, mark as data-quality issue. Comp/payroll integrity warning.
  5. **Review item — detail / resolve drawer (unclassified_service_line).** Non-blocking. Pick from {Facility Services, Building Management, Parking} or defer. Bulk-apply across same position/placement string.
  6. **Review item — detail / resolve drawer (ambiguous_chain).** Visualizes the candidate predecessor contracts; HR picks best-effort link or flags as truly novel.
  7. **Reconciliation report — per-run dashboard.** Per-entity rows: `source_count` / `loaded_count` / `review_count` (must balance — RC-4). Trend across runs. Drill into entity → list of review items.
  8. **Bulk-resolve confirmation modal.** "You are about to map 40 records of 'PLZ SNYN' → Plaza Senayan. Continue?" with audit note input.
  9. **Empty / loading / error states** for the queue (no open items → "Ready for cutover" affordance; loading; failed to load run).
- **Audit trail:** Reuse E1 audit-log component for "who resolved what, when, with what resolution."

### F9.4 — load-idempotent
- **Actors:** Migration engineer; System (load, upsert, report).
- **Surface declared in PRD §4:** "Migration tooling (CLI/job) → Postgres. Reports surface in F9.3."
- **UI needed?:** **No** for the load mechanic. **Optional** for a dry-run results view (currently piggybacks on F9.3's recon-report screen via LD-6).
- **Rationale:** LD-1..LD-7 are all backend invariants (dependency order, transactional batches, dry-run flag). Output (`source_count` vs `loaded_count` + deferred) flows back into F9.3's recon report.
- **If yes, screens:** **Dry-run results** view is already covered by the F9.3 reconciliation-report dashboard with a `run_id` filter and a "Dry-run" badge — no separate screen needed.

### F9.5 — cutover-validation ⭐ **SECONDARY UI**
- **Actors:** Migration engineer/ops, HR/Super Admin (sign-off + review gate), Leadership (go/no-go), System (validation).
- **Surface declared in PRD §4:** **Runbook + tooling** for engineer/ops; **Web console** for HR/Super Admin (validation reports + sign-off).
- **UI needed?:** **Yes — focused.** Not a full runbook UI (the runbook stays as a doc), but a **validation-gate console** with go/no-go capture.
- **Rationale:** CV-2..CV-4 require recording the go/no-go decision against a final `MigrationRun` with validation evidence. CV-3 enumerates four validation gates: record counts, leave-balance reconciliation, payslip totals, sample spot-checks. HR/leadership sign-off needs a UI surface; it cannot be a CLI flag (auditability + accountability).
- **Screens needed:**
  1. **Cutover readiness dashboard.** Single status card: "READY / NOT READY for cutover." Shows: open blocking review items count (must be 0), last dry-run timestamp + result, validation-gate pass/fail per category. Action: "Initiate final run" (disabled if not ready).
  2. **Validation gates — results view.** Four panels: (a) Record counts (per-entity source vs loaded diff table), (b) Leave-balance reconciliation, (c) Payslip totals, (d) Sample spot-checks (configurable agent/placement pick-list). Each panel: pass/fail + threshold + drill-in.
  3. **Go/no-go decision modal.** Required: signer identity (HR + Leadership), decision (GO / NO-GO), notes. On GO → switch traffic confirmation. On NO-GO → rollback runbook link.
  4. **Migration run history.** Read-only list of all `MigrationRun` rows: id, started/finished, status, source_snapshot, stats, recon report link. Final run flagged.
  5. **Post-cutover monitoring panel.** Key flows status (errors, login counts, attendance check-in volume). Tied to E10 reporting.
- **Note:** The freeze/rollback mechanics themselves are tooling, not UI. The console only **records the decision** and **surfaces the state**.

---

## 2. Required screens summary

| # | Screen | PRD | Platform | Severity | Notes |
|---|---|---|---|---|---|
| 1 | Review queue — list view | F9.3 | Web | **P0 (blocking)** | Core HR workflow. Filters, bulk-select, severity ordering. |
| 2 | Resolve drawer — `unmatched_placement` | F9.3 | Web | **P0 (blocking)** | Most frequent issue; bulk-apply critical. |
| 3 | Resolve drawer — `orphan_identity` | F9.3 | Web | **P0 (blocking)** | Blocking. Link/create/discard with reason. |
| 4 | Resolve drawer — `decrypt_fail` | F9.3 | Web | **P0 (blocking)** | Blocking. Comp/payroll integrity. |
| 5 | Resolve drawer — `unclassified_service_line` | F9.3 | Web | **P1 (non-blocking)** | 3-way pick. Bulk-apply. |
| 6 | Resolve drawer — `ambiguous_chain` | F9.3 | Web | **P1 (non-blocking)** | Candidate-predecessor picker. |
| 7 | Reconciliation report — per-run dashboard | F9.3 + F9.4 | Web | **P0** | Counts must balance (RC-4). Trend across runs. |
| 8 | Bulk-resolve confirmation modal | F9.3 | Web | **P0** | Required for RC-5. Audit-note capture. |
| 9 | Cutover readiness dashboard | F9.5 | Web | **P0** | Single go-live status surface; reuses F9.3 gate. |
| 10 | Validation gates — results view | F9.5 | Web | **P0** | Four panels (counts / leave / payslip / spot-check). |
| 11 | Go/no-go decision modal | F9.5 | Web | **P0** | Captures sign-off identity + decision + notes. |
| 12 | Migration run history | F9.5 | Web | **P1** | Read-only audit-style list. |
| 13 | Post-cutover monitoring panel | F9.5 | Web | **P2** | Could be deferred to E10 reporting reuse. |
| 14 | Empty / loading / error states for queue | F9.3 | Web | **P0** | "No open items → Ready for cutover" affordance. |

**Totals:** ~14 screen states across **2 features** (F9.3, F9.5). F9.1, F9.2, F9.4 are backend-only.

---

## 3. Recommended design scope

**Recommendation:** **Create an E9 admin-console frame in `.pen`** with the screens above, scoped tightly to two clusters:

- **Cluster A — Reconciliation Review Queue (F9.3):** screens 1–8, 14. ~9 states. This is the bulk of the work and reuses heavily from existing approval/queue patterns (E5/E6/E7 approvals).
- **Cluster B — Cutover Console (F9.5):** screens 9–13. ~5 states. Lower-volume, higher-stakes; closer to a status dashboard than a workbench.

**Do NOT design:**
- Extraction/staging UI (F9.1) — CLI only.
- Transform UI (F9.2) — CLI only; crosswalk inspector can be deferred to ops tooling outside the design system.
- Load mechanics UI (F9.4) — CLI/job; dry-run results piggyback on F9.3's recon-report dashboard.

**Audience:** Super Admin + HR Admin only (per FEATURE.md §2 actors table). Hide entirely from shift-leader / agent roles. Place behind a `super_admin`-only "Migration" nav section that disappears post-cutover (or becomes read-only).

**Mobile:** N/A. FEATURE.md §6 explicitly excludes mobile from E9.

---

## 4. Dependencies on other epics (reuse opportunities)

| Pattern | Source epic | Reuse for E9 |
|---|---|---|
| **Audit log entry list** (who/what/when, with diff) | E1 Foundations | Review item resolution history; go/no-go decision record. RC-6 + CV-8 both audited. |
| **Approval queue list + drawer** (filters, bulk-select, status pills) | E6 Leave / E7 Overtime approvals | Direct template for F9.3 review queue. Same interaction shape (open items → drawer → resolve → audit). |
| **ClientCompany search-and-pick** | E2 master data / E3 placement | `unmatched_placement` resolution uses the same picker component. Fuzzy + alias UI lives here. |
| **Employee/User pickers** | E2 identity | `orphan_identity` resolution drawer. |
| **Service-line classifier (3-way)** | E3 placement | `unclassified_service_line` resolution; identical to placement service-line field. |
| **Status pills, severity badges, count cards** | E1 + DESIGN-SYSTEM.md tokens | Throughout F9.3 + F9.5. Use teal for "Resolved" (per design-system rule: green = brand only; positive status = teal). |
| **Reports / dashboard tiles** | E10 Reporting | Reconciliation reports and validation-gate panels. Post-cutover monitoring can defer entirely to E10. |
| **Confirmation modal with audit-note input** | E1 Foundations | Bulk-resolve modal; go/no-go modal. |

**Net effect:** Cluster A is mostly *composition* of existing `comp/*` patterns (approval queue + pickers + audit log), so design effort is much lower than the screen count suggests. Cluster B is more bespoke (validation dashboard, go/no-go) but small.

---

## 5. Notes

- **Decision log alignment:** EPICS.md §8 confirms blocking vs non-blocking issue types (decrypt_fail / orphan_identity / unmatched_placement = **blocking**; unclassified_service_line / ambiguous_chain = non-blocking). The per-PRD "Still open" question in `reconciliation-review.md` §10 ("which issue types are blocking") is **already resolved** in §8 — per CLAUDE.md "§8 wins" rule. PRD §10 should be reconciled in a separate pass (out of scope for this audit).
- **Placement-string matching:** EPICS.md §8 confirms exact + alias list + fuzzy-with-manual-confirm. The `unmatched_placement` drawer (screen #2) must surface: (a) exact match attempts, (b) alias-list editor, (c) ranked fuzzy candidates. This is the single most important screen in E9.
- **Post-cutover read-only period:** EPICS.md §8 says `lumen_swp` is retained read-only ~6–12 months. The E9 console itself should persist (in read-only mode) for the same window so audit/sign-off records remain accessible.
- **Bulk operations:** RC-5 explicitly requires bulk resolution. This is not optional UI polish — without it, HR cannot realistically clear thousands of identical placement strings before go-live. Screen #8 (bulk-confirm modal) is a P0.
- **What the .pen currently lacks:** Zero E9 frames. The audit recommendation is to add an "E9 — Migration Console" frame at the end of the screens section, structured as two pages (Reconciliation Queue + Cutover Console), reusing existing `comp/*` library components per DESIGN-SYSTEM.md.
- **Sequencing:** F9.3 screens must exist before any dry-run rehearsal can include HR in the loop. F9.5 screens are needed before the rehearsed cutover. Design F9.3 first.
- **Open spec items that don't block design:** validation-gate exact thresholds (CV-3, sized by dry-runs), maintenance-window length, post-cutover rollback-window duration. These are ops parameters, not UI structure decisions.
