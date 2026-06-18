# Test Cases · E9 — Data Migration (MySQL `lumen_swp` → Postgres)

> **Epic:** E9 Data Migration · **Status:** Draft v1 · **Parent:** [FEATURE.md](../epics/E9-migration/FEATURE.md)
> **POV:** Data-migration engineer / QA validator. **No UI** for the pipeline itself — validation is performed by running **SQL/CLI checks** against the staging area, the legacy source (read replica / dump), and the target Postgres database. The only human-facing surface is the reconciliation review queue (F9.3), validated via its backing tables.

---

## 1. Scope & how to read this document

E9 is a **one-shot, big-bang transform-and-load** script — extract a frozen snapshot of `lumen_swp`, decrypt `DBEncryption` comp/payroll fields with the legacy key, transform under the new schema (per each epic's `DATA-MAPPING.md`), reconcile unmappable rows through a review queue resolved **before** go-live, load in dependency order (idempotent via crosswalks), validate, then cut over. The three **blocking** review-item types — `decrypt_fail`, `orphan_identity`, `unmatched_placement` — are either pre-resolved in code or logged-and-skipped; `ambiguous_chain` is non-blocking. `unclassified_service_line` was removed 2026-06-12 (service line dropped project-wide; `position` is free-text copied verbatim and never queued).

These are **data-validation** test cases, not UI flows. Each case gives the exact SQL/CLI checks to run, comparing source (MySQL) counts/values against target (Postgres). Conventions used below:

- **`src`** = a MySQL connection to the frozen `lumen_swp` snapshot (read replica or restored dump).
- **`tgt`** = a `psql` connection to the target Postgres DB (or the dry-run scratch clone).
- **`crosswalk`** = the `crosswalk(legacy_table, legacy_id, new_table, new_id, run_id)` table.
- **`review_item`** = `review_item(entity_type, issue_type, payload, status, resolved_by, resolution)`.
- **`recon_report`** = `recon_report(run_id, entity_type, source_count, loaded_count, review_count)`.
- **`migration_run`** = `migration_run(id, started_at, finished_at, status, source_snapshot, stats)`.
- Money tolerance for decrypted payroll sums: **exact** (read-only verbatim carry, no recompute) unless a documented rounding note applies — default tolerance **0**.
- Dates are absolute (project clock: today is **2026-06-17**; decision dates **2026-05-29**, **2026-06-12**, **2026-06-15**).

Run all checks against a **staging rehearsal** first; the **final** run repeats them at cutover as the F9.5 validation gates.

---

## 2. Coverage matrix

Rows = features. Columns = the phase(s) each feature owns + the check-types exercised. Cell = test-case IDs (`TC-E9-F9.x-NNN`).

| Feature | Phase | CountParity | ValueSpotCheck | Idempotency | ErrorHandling | IdentityRemap | Reconciliation |
|---|---|---|---|---|---|---|---|
| **F9.1** Extraction & Staging | Extraction/Staging | 001, 002 | 004, 005 | 003 | 006 (decrypt_fail), 007, 008 | — | — |
| **F9.2** Transform & Crosswalks | Transform/Crosswalks | 003 | 002, 005, 006 | 004 | 007 (unmatched), 008 (orphan), 009 (ambiguous) | 001 | — |
| **F9.3** Reconciliation & Review | Reconciliation | 005 | 002 | — | 003, 006 | — | 001, 004, 007 |
| **F9.4** Load & Idempotent Re-runs | Load | 001 | 005 | 002, 003 | 006, 007 | 004 | 008 |
| **F9.5** Cutover/Validation/Rollback | Cutover validation | 001 | 003 (payslip), 004 (leave bal) | 007 | 005 (rollback), 008 (freeze) | — | 002 (gate) |

Check-type legend: **CountParity** = source vs target row counts equal. **ValueSpotCheck** = field-level value equality on sampled rows. **Idempotency** = re-run yields identical state, no dupes. **ErrorHandling** = unmappable/decrypt/constraint paths produce review items, never silent drop/null. **IdentityRemap** = `users.id`/`employees.id` split correctly resolved. **Reconciliation** = review-queue + recon-report correctness and the go-live gate.

---

## 3. F9.1 — Extraction & Staging

> BR refs EX-1..EX-6; cases C-1..C-4. Invariant INV-5 (decrypt-then-re-encrypt; failures → review item, never null).

### Extraction/Staging · CountParity

#### TC-E9-F9.1-001 · Per-table row count: source snapshot vs staging
- [ ] **Phase:** Extraction/Staging · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** CountParity
- **Objective:** Every source table is staged with zero row loss (extraction completeness).
- **Preconditions:** Frozen `lumen_swp` snapshot available as `src`; extraction has run; staging tables populated; a `migration_run` row exists with `source_snapshot` set.
- **Steps:** For each migrated source table (`users`, `employees`, `employee_contracts`, `companies`, `recruitment_roles`, `recruitment_role_types`, `leave_types`, `attendance_codes`, `schedules`, `shifts`, `attendance_users`, `leaves`, `employee_leave_quotas`, `overtimes`, `employee_payslips`, `employee_salaries`, `employee_salary_columns`, `employee_benefits`), compare counts:
  - `src`: `SELECT COUNT(*) FROM employee_contracts;` (repeat per table)
  - `tgt` staging: `SELECT COUNT(*) FROM stg_employee_contracts;`
  - Driver query the harness may emit: `SELECT entity_type, source_count FROM recon_report WHERE run_id = :run;`
- **Expected result / Acceptance criteria:** `staging_count = source_count` for every table (soft-deleted rows included — they are carried, not filtered, at extraction). Any delta is a defect, not a review item.
- **Traceability:** F9.1, EX-2, EX-5, INV-2; FEATURE §6 (F9.1).

#### TC-E9-F9.1-002 · Large-table streamed extract is complete (attendance)
- [ ] **Phase:** Extraction/Staging · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** CountParity
- **Objective:** The highest-volume table (`attendance_users`) is streamed/batched without dropping the head/tail of any batch.
- **Preconditions:** Extraction run with batching enabled; staging populated.
- **Steps:**
  - `src`: `SELECT COUNT(*), MIN(id), MAX(id), MIN(check_in), MAX(check_in) FROM attendance_users;`
  - `tgt`: `SELECT COUNT(*), MIN(legacy_id), MAX(legacy_id) FROM stg_attendance_users;`
  - Gap check: `SELECT s.id FROM attendance_users s LEFT JOIN stg_attendance_users t ON t.legacy_id=s.id WHERE t.legacy_id IS NULL LIMIT 50;` (cross-DB via FDW or export+diff).
- **Expected result / Acceptance criteria:** Counts equal; min/max `legacy_id` match; the missing-id query returns 0 rows. Batch boundaries leave no gaps.
- **Traceability:** F9.1, EX-5, C-3 (very large tables → stream/batch).

### Extraction/Staging · Idempotency

#### TC-E9-F9.1-003 · Re-running extraction rebuilds staging cleanly (no dupes, no drift)
- [ ] **Phase:** Extraction/Staging · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** Idempotency
- **Objective:** Extraction is idempotent — a second run from a fresh snapshot produces identical staging, not duplicated rows.
- **Preconditions:** A completed staging build; the same (or freshly-restored identical) snapshot.
- **Steps:**
  - Record baseline: `SELECT 'employee_contracts' tbl, COUNT(*) c, MD5(STRING_AGG(legacy_id::text, ',' ORDER BY legacy_id)) sig FROM stg_employee_contracts;` (repeat per table).
  - Re-run extraction.
  - Re-run the same query; diff `c` and `sig`.
- **Expected result / Acceptance criteria:** Row counts and the ordered-id signature are byte-identical before/after; no duplicate `legacy_id` in any staging table (`GROUP BY legacy_id HAVING COUNT(*)>1` returns 0).
- **Traceability:** F9.1, EX-5; FEATURE AC "Re-run extraction".

### Extraction/Staging · ValueSpotCheck (decryption)

#### TC-E9-F9.1-004 · DBEncryption fields decrypt to plausible values (comp/payroll)
- [ ] **Phase:** Extraction/Staging · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ValueSpotCheck
- **Objective:** Encrypted legacy fields (`gaji_pokok`, `bpjs_*`, `pph21`, payslip totals, salary `value`) are decrypted with the legacy key into staging — not left as ciphertext or nulled.
- **Preconditions:** Legacy encryption key configured; staging processed `employee_contracts`, `employee_payslips`, `employee_salaries`, `employee_benefits`.
- **Steps:** Pick 10 known agents with non-null comp. Cross-check the decrypted staging value against a value decrypted independently (e.g., via a one-off script that reuses `app/Casts/DBEncryption.php` logic on the same ciphertext):
  - `tgt`: `SELECT legacy_id, gaji_pokok_dec, take_home_pay_dec FROM stg_employee_payslips WHERE legacy_id IN (...);`
  - Compare against the independent decryption of the same ciphertext.
  - Sanity: `SELECT COUNT(*) FROM stg_employee_payslips WHERE take_home_pay_dec ~ '^[0-9]+(\.[0-9]+)?$' = false;` (non-numeric = failed/leaked ciphertext).
- **Expected result / Acceptance criteria:** All 10 decrypted values equal the independent decryption; no staged decrypted comp value is ciphertext, blank, or null where the source had a value; non-numeric count = 0.
- **Traceability:** F9.1, EX-3, INV-5; E8 DATA-MAPPING G-1; E2 DATA-MAPPING §5.3; E3 DATA-MAPPING G-3.

#### TC-E9-F9.1-005 · Non-encrypted columns carried verbatim
- [ ] **Phase:** Extraction/Staging · **POV:** Migration engineer/QA · **Priority:** P1 · **Type:** ValueSpotCheck
- **Objective:** Plain columns (names, dates, NIK/NPWP, flags) land in staging without transformation (transform happens in F9.2, not extraction).
- **Preconditions:** Staging populated.
- **Steps:** Sample 20 `employees`: compare `name, nik, nip, join_at, gender, npwp` between `src.employees` and `stg_employees` by `legacy_id`.
- **Expected result / Acceptance criteria:** All sampled non-encrypted fields are identical to source (no truncation, encoding loss, or date-zone shift at the extraction layer).
- **Traceability:** F9.1, EX-2.

### Extraction/Staging · ErrorHandling

#### TC-E9-F9.1-006 · Decrypt failure raises `decrypt_fail` review item, never nulls
- [ ] **Phase:** Extraction/Staging · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ErrorHandling
- **Objective:** A row that fails to decrypt produces a `review_item(issue_type='decrypt_fail')` and the field is **not** silently nulled (INV-5).
- **Preconditions:** Inject/locate at least one row with corrupt or unparseable ciphertext (e.g., a payslip whose `take_home_pay` ciphertext was truncated). Staging run executed.
- **Steps:**
  - `tgt`: `SELECT entity_type, issue_type, payload->>'legacy_id' lid, payload->>'field' fld FROM review_item WHERE issue_type='decrypt_fail';`
  - Confirm the staged row for that `legacy_id` did **not** get a null/blank in the failed field: `SELECT take_home_pay_dec FROM stg_employee_payslips WHERE legacy_id=:lid;` should be a sentinel/untouched, not silently nulled.
- **Expected result / Acceptance criteria:** One `decrypt_fail` review item per failed field with `entity_type`, legacy id, and field name in payload; status `Open`; the field is not nulled. `decrypt_fail` is **blocking** (must resolve before cutover — re-key/re-source).
- **Traceability:** F9.1, EX-4, INV-5; F9.3 RC-3, C-1; EPICS §8 (blocking set).

#### TC-E9-F9.1-007 · Wrong/rotated key fails fast before mass decrypt
- [ ] **Phase:** Extraction/Staging · **POV:** Migration engineer/QA · **Priority:** P1 · **Type:** ErrorHandling
- **Objective:** An incorrect/rotated legacy key aborts with a clear error before bulk-decrypting (avoid silently producing thousands of garbage values).
- **Preconditions:** Run the staging decrypt step with a deliberately wrong key in a scratch environment.
- **Steps:** Run extraction with the bad key; observe exit code and log; check `migration_run.status`.
- **Expected result / Acceptance criteria:** Process aborts early (non-zero exit, explicit "decrypt key validation failed" message); `migration_run.status` = `Failed`/`Aborted`; no mass `decrypt_fail` flood; staging comp fields untouched from the prior good run (or empty).
- **Traceability:** F9.1, C-4 (key rotated/incorrect → fail fast).

#### TC-E9-F9.1-008 · Schema-drift detection (unexpected columns/tables)
- [ ] **Phase:** Extraction/Staging · **POV:** Migration engineer/QA · **Priority:** P2 · **Type:** ErrorHandling
- **Objective:** Columns/tables present in the snapshot but absent from the mapping are detected and flagged (not silently ignored).
- **Preconditions:** Snapshot whose schema may have drifted since the mapping was authored.
- **Steps:** Compare `information_schema.columns` of `src` against the mapping manifest; the extraction harness should emit a drift report.
- **Expected result / Acceptance criteria:** Any unmapped column/table is listed in a drift warning (logged + surfaced for mapping review); inconsistent-dump detection (C-1) aborts rather than staging a torn snapshot.
- **Traceability:** F9.1, C-1, C-2.

---

## 4. F9.2 — Transform & Crosswalks

> BR refs TR-1..TR-8; cases C-1..C-4. INV-1 (crosswalk-keyed), INV-2 (nothing dropped), INV-3 (dependency order computed identity-first).

### Transform/Crosswalks · IdentityRemap

#### TC-E9-F9.2-001 · Identity split resolves to 1:1 User↔Employee with crosswalks both ways
- [ ] **Phase:** Transform/Crosswalks · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** IdentityRemap
- **Objective:** The legacy identity duality (`users.id` for login/attendance vs `employees.id` for HR, bridged by `employees.user_id`) is resolved to a single new identity with a 1:1 User↔Employee link, and both legacy ids are kept in the crosswalk.
- **Preconditions:** Transform run after staging; identity computed first (TR-3).
- **Steps:**
  - User crosswalk completeness: `SELECT COUNT(*) FROM stg_users s LEFT JOIN crosswalk c ON c.legacy_table='users' AND c.legacy_id=s.legacy_id WHERE c.new_id IS NULL;`
  - Employee crosswalk completeness: same against `stg_employees` / `legacy_table='employees'`.
  - 1:1 link: `SELECT user_id, COUNT(*) FROM employee GROUP BY user_id HAVING COUNT(*)>1;` and `SELECT COUNT(*) FROM employee WHERE user_id IS NULL;`
  - Bridge preserved: for 15 sampled `employees` with non-null `user_id`, confirm the new `employee.user_id` resolves to the User crosswalked from the same legacy `users.id`.
- **Expected result / Acceptance criteria:** Every staged user and employee has a crosswalk row; **no Employee has a null `user_id`** (G-5 invariant); no User maps to >1 Employee; the legacy bridge is faithfully reproduced; backfilled Users (legacy `employees.user_id` null) appear keyed on phone with a forced-reset flag.
- **Traceability:** F9.2, TR-3; E2 DATA-MAPPING G-5; E3 DATA-MAPPING G-8; FEATURE AC "Remap identity first".

### Transform/Crosswalks · ValueSpotCheck

#### TC-E9-F9.2-002 · `employee_contracts` split → exactly one EmploymentAgreement + one Placement
- [ ] **Phase:** Transform/Crosswalks · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ValueSpotCheck
- **Objective:** Each staged contract row decomposes into exactly one EmploymentAgreement and one Placement, with crosswalks linking the legacy contract id to **both** new ids.
- **Preconditions:** Identity transform done; placement-string reconciliation resolved (or queued); transform run.
- **Steps:**
  - `SELECT COUNT(*) FROM stg_employee_contracts;` (= N, excluding rows queued unmatched).
  - `SELECT legacy_id, COUNT(*) FILTER (WHERE new_table='employment_agreement') ea, COUNT(*) FILTER (WHERE new_table='placement') pl FROM crosswalk WHERE legacy_table='employee_contracts' GROUP BY legacy_id;`
  - Field carry on a sample contract: `agreement_no=pkwt_reference`, EA `start_date=contract_start_at`, EA `end_date=contract_end_at` (null→PKWTT), Placement `position` = the `recruitment_roles.role` label verbatim, Placement `site_id` = the company's auto "Main Site".
- **Expected result / Acceptance criteria:** For every non-queued contract, `ea=1 AND pl=1`; `agreement_no`, dates, derived PKWT/PKWTT type, free-text `position`, and `site_id`→Main Site all match the mapping; `annual_leave` lands on the EmploymentAgreement entitlement, not the placement.
- **Traceability:** F9.2, TR-4; E3 DATA-MAPPING §3, G-1, G-4, G-5; E2 DATA-MAPPING (employee_contracts→EmploymentAgreement); FEATURE AC "Split a legacy contract".

#### TC-E9-F9.2-003 · Crosswalk count parity — one crosswalk per produced row
- [ ] **Phase:** Transform/Crosswalks · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** CountParity
- **Objective:** Every transformed row has exactly one crosswalk entry (TR-2); no transformed row is crosswalk-less and no crosswalk is dangling.
- **Preconditions:** Transform run complete.
- **Steps:**
  - Dangling crosswalk (points to nothing): for each `new_table`, `SELECT c.new_id FROM crosswalk c LEFT JOIN <new_table> t ON t.id=c.new_id WHERE c.legacy_table=... AND t.id IS NULL;` (after load) — at transform time, validate against the transformed-record staging.
  - Duplicate crosswalk: `SELECT legacy_table, legacy_id, new_table, COUNT(*) FROM crosswalk GROUP BY 1,2,3 HAVING COUNT(*)>1;`
- **Expected result / Acceptance criteria:** No duplicate `(legacy_table, legacy_id, new_table)` triples; no transformed row without a crosswalk; multi-target legacy ids (contracts) legitimately have multiple crosswalk rows differing by `new_table`.
- **Traceability:** F9.2, TR-2, INV-1.

#### TC-E9-F9.2-005 · Role/company remap — `companies.role=2`=client; role enum remap (`agent`→NULL)
- [ ] **Phase:** Transform/Crosswalks · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ValueSpotCheck
- **Objective:** Only `companies.role=2` becomes a `ClientCompany (type=CLIENT)`; the legacy `users.role` tinyint remaps to the elevation enum `{super_admin, hr_admin, lead, NULL}` with `agent`→NULL and `shift_leader`→NULL; the SWP `type=INTERNAL` company + HQ Site are seeded; `employee_type` is derived.
- **Preconditions:** Identity + company transform done; the explicit legacy-role value map (G-1) loaded.
- **Steps:**
  - Client filter: `SELECT COUNT(*) FROM company WHERE type='CLIENT';` vs `src`: `SELECT COUNT(*) FROM companies WHERE role=2 AND <not soft-deleted-policy>;`. Confirm no role 1/3/4 leaked into CLIENT.
  - INTERNAL seed: `SELECT COUNT(*) FROM company WHERE type='INTERNAL';` = 1, and it has ≥1 Site.
  - Role remap: `SELECT role, COUNT(*) FROM users GROUP BY role;` (post). Confirm distinct values ⊆ {super_admin, hr_admin, lead, NULL}; legacy-agent and legacy-shift_leader users have `role IS NULL`.
  - `employee_type`: agents with a CLIENT placement → `FIELD`; SWP internal staff → `INTERNAL` assigned to HQ Site; ambiguous → review item.
- **Expected result / Acceptance criteria:** CLIENT count = source `role=2`; exactly one INTERNAL company with an HQ Site; no `role` value outside the elevation enum or NULL; legacy `agent`/`shift_leader` → NULL; `employee_type` populated per rule with ambiguous cases queued.
- **Traceability:** F9.2, TR-1; E2 DATA-MAPPING G-1, G-9, G-8; E3 DATA-MAPPING §4, G-6; EPICS §8 (role/identity remap 2026-06-15); FEATURE §7.

#### TC-E9-F9.2-006 · Derived links — schedule→placement, attendance→schedule, OT day_type
- [ ] **Phase:** Transform/Crosswalks · **POV:** Migration engineer/QA · **Priority:** P1 · **Type:** ValueSpotCheck
- **Objective:** Net-new links absent in legacy are derived correctly: `Schedule.placement_id` (active placement on `work_date`), `Attendance.schedule_id`/`placement_id` (by employee+date), historical attendance `verification_status=Verified`, OT `day_type` default `Workday`+flag for history.
- **Preconditions:** E3 placement crosswalks exist; transform run for E4/E5/E7.
- **Steps:**
  - Schedule link: sample 20 schedules — confirm `placement_id` is the placement whose period contains `work_date`; schedules with no active placement appear as review items (not silently linked).
  - Attendance: `SELECT COUNT(*) FROM attendance WHERE verification_status<>'Verified';` (historical) → expect 0; geofence flags `in_geofence_in/out` null for history (no retro-flag).
  - OT: `SELECT day_type, COUNT(*) FROM overtime GROUP BY day_type;` — historical rows default `Workday` and carry a "historical day_type unverified" flag.
- **Expected result / Acceptance criteria:** Derived FKs match the active-on-date placement; unmatched schedule/attendance dates → review items, not dropped (INV-2); all historical attendance is `Verified` with null geofence flags; historical OT `day_type` defaulted+flagged.
- **Traceability:** F9.2, TR-5, TR-6; E4 DATA-MAPPING G-3/G-4; E5 DATA-MAPPING G-1/G-3/G-5; E7 DATA-MAPPING G-3/G-7; FEATURE AC "Derive schedule placement link".

### Transform/Crosswalks · Idempotency

#### TC-E9-F9.2-004 · Re-run transform reuses crosswalks, produces no duplicates
- [ ] **Phase:** Transform/Crosswalks · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** Idempotency
- **Objective:** Transform is deterministic and re-runnable against the same snapshot (TR-8); a second run reuses existing crosswalks rather than minting new new_ids.
- **Preconditions:** One completed transform; same staging snapshot.
- **Steps:**
  - Snapshot crosswalk: `SELECT COUNT(*) c, MD5(STRING_AGG(legacy_table||legacy_id||new_table||new_id, ',' ORDER BY legacy_table,legacy_id,new_table)) sig FROM crosswalk;`
  - Re-run transform; re-query.
- **Expected result / Acceptance criteria:** `c` and `sig` unchanged; no new crosswalk rows for already-mapped legacy ids; no duplicate new entities created.
- **Traceability:** F9.2, TR-8, INV-1; FEATURE AC "Idempotent re-run".

### Transform/Crosswalks · ErrorHandling

#### TC-E9-F9.2-007 · Unmatched free-text placement → `unmatched_placement` review item (blocking)
- [ ] **Phase:** Transform/Crosswalks · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ErrorHandling
- **Objective:** A `placement` free-text string that matches no `ClientCompany` (exact → alias → fuzzy-with-confirm all fail) becomes a `review_item(issue_type='unmatched_placement')`; the contract is **not** loaded with a dangling/null company.
- **Preconditions:** Transform run; matching strategy = exact + alias list + fuzzy-with-manual-confirm.
- **Steps:**
  - `SELECT payload->>'placement_string' s, COUNT(*) FROM review_item WHERE issue_type='unmatched_placement' GROUP BY 1 ORDER BY 2 DESC;`
  - Confirm no Placement was produced for those contracts: `SELECT COUNT(*) FROM crosswalk WHERE legacy_table='employee_contracts' AND legacy_id IN (<queued ids>) AND new_table='placement';` → 0.
  - Ambiguous fuzzy multi-match (C-1): payload should carry candidate companies for HR to pick.
- **Expected result / Acceptance criteria:** Every unmatched string is queued with context; no placement loaded for queued contracts; multi-match candidates included; `unmatched_placement` is **blocking**.
- **Traceability:** F9.2, TR-7, C-1; E3 DATA-MAPPING G-2; F9.3 RC-3; EPICS §8 (blocking set, placement-string matching).

#### TC-E9-F9.2-008 · Orphan identity → `orphan_identity` review item (blocking)
- [ ] **Phase:** Transform/Crosswalks · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ErrorHandling
- **Objective:** A legacy `user_id` with no `employees` row (or an `employees` row with no resolvable user and no backfillable phone/email) raises `orphan_identity`, never a half-linked record.
- **Preconditions:** Transform run; G-5 backfill applied (employees with null `user_id` get a User keyed on phone).
- **Steps:**
  - `SELECT entity_type, payload->>'legacy_user_id', payload->>'legacy_employee_id' FROM review_item WHERE issue_type='orphan_identity';`
  - Confirm employees backfilled (had null user_id but a phone) did **not** become orphans: they should be linked, not queued.
  - Confirm users with no employee and no backfill path are queued (reviewed separately).
- **Expected result / Acceptance criteria:** True orphans (no bridge, no backfill key) are queued; backfillable employees are linked; no Employee left with null `user_id`; `orphan_identity` is **blocking**.
- **Traceability:** F9.2, TR-7, C-2; E2 DATA-MAPPING G-5; E3 G-8; EPICS §8 (blocking set).

#### TC-E9-F9.2-009 · Ambiguous renewal/transfer chain → `ambiguous_chain` (non-blocking) + best-effort predecessor
- [ ] **Phase:** Transform/Crosswalks · **POV:** Migration engineer/QA · **Priority:** P1 · **Type:** ErrorHandling
- **Objective:** When an employee's contracts can't be unambiguously ordered into a renewal chain, a best-effort `predecessor_id` link is set and an `ambiguous_chain` review item is raised — and this type is **non-blocking** (cutover may proceed).
- **Preconditions:** Employee with overlapping/gapped contract dates; transform run.
- **Steps:**
  - `SELECT COUNT(*) FROM review_item WHERE issue_type='ambiguous_chain';`
  - For a flagged employee, confirm `placement.predecessor_id` is set best-effort (by date order) rather than null/erroring.
- **Expected result / Acceptance criteria:** Best-effort predecessor link present; `ambiguous_chain` items raised but classified **non-blocking** (tracked for post-launch, do not gate cutover).
- **Traceability:** F9.2, TR-7, C-3; E3 DATA-MAPPING G-7; F9.3 RC-3; FEATURE §7 (ambiguous_chain non-blocking).

---

## 5. F9.3 — Reconciliation & Review Queue

> BR refs RC-1..RC-6; cases C-1, C-3, C-4. The go-live gate.

### Reconciliation · Reconciliation (queue + report correctness)

#### TC-E9-F9.3-001 · Review items carry issue_type, entity_type, payload context
- [ ] **Phase:** Reconciliation · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** Reconciliation
- **Objective:** Every review item is well-formed: typed, attributed to an entity, and carries enough payload for HR to resolve it.
- **Preconditions:** A transform run that produced review items of each type.
- **Steps:**
  - `SELECT issue_type, entity_type, COUNT(*), COUNT(*) FILTER (WHERE payload IS NULL OR payload='{}') empty_payload FROM review_item GROUP BY 1,2;`
  - Confirm `issue_type` ∈ {`unmatched_placement`, `orphan_identity`, `decrypt_fail`, `ambiguous_chain`}; **no** `unclassified_service_line` exists (removed 2026-06-12).
- **Expected result / Acceptance criteria:** No null/empty payloads; all issue_types are in the valid set; `unclassified_service_line` count = 0; position is never queued (free-text copied verbatim).
- **Traceability:** F9.3, RC-1; FEATURE §4 (REVIEW_ITEM), §7; EPICS §8.

#### TC-E9-F9.3-002 · Resolution is recorded and feeds the next run
- [ ] **Phase:** Reconciliation · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ValueSpotCheck
- **Objective:** Resolving an item stores the chosen mapping/correction + `resolved_by` + timestamp, and the resolution is applied on the next transform/load run (not lost).
- **Preconditions:** An open `unmatched_placement` for "PLZ SNYN"; HR maps it to "Plaza Senayan"; a subsequent run executed.
- **Steps:**
  - `SELECT status, resolution, resolved_by, resolved_at FROM review_item WHERE payload->>'placement_string'='PLZ SNYN';`
  - After re-run: confirm the previously-queued contracts now produced a Placement linked to the "Plaza Senayan" `ClientCompany` (crosswalk + FK).
- **Expected result / Acceptance criteria:** Item `status='Resolved'` with non-null resolution/resolved_by/timestamp; the next run loads the formerly-queued rows mapped to the chosen company; review_count drops accordingly.
- **Traceability:** F9.3, RC-2; F9.4 C-1; FEATURE AC "Resolve an unmatched placement".

#### TC-E9-F9.3-004 · Reconciliation report balances: source = loaded + in-review (per entity)
- [ ] **Phase:** Reconciliation · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** Reconciliation
- **Objective:** Each `recon_report` row balances — every source row is accounted for as either loaded or in-review; nothing vanishes (INV-2).
- **Preconditions:** A completed load run with `recon_report` emitted per entity.
- **Steps:**
  - `SELECT entity_type, source_count, loaded_count, review_count, source_count - (loaded_count + review_count) AS gap FROM recon_report WHERE run_id=:run;`
  - Cross-validate `source_count` against the actual staging count and `loaded_count` against the actual target count for 3 entities.
- **Expected result / Acceptance criteria:** `gap = 0` for every entity; reported `source_count`/`loaded_count` match independent `COUNT(*)` queries; deferred rows are counted in review/queued, not lost.
- **Traceability:** F9.3, RC-4; F9.4 LD-6; INV-2; FEATURE AC "Reconciliation report balances".

#### TC-E9-F9.3-007 · New items surfaced on re-run; counts updated
- [ ] **Phase:** Reconciliation · **POV:** Migration engineer/QA · **Priority:** P2 · **Type:** Reconciliation
- **Objective:** Items appearing only on a later run (e.g., after a partial fix exposes a new edge) are surfaced and counts updated (C-4).
- **Preconditions:** Two runs where the second introduces a new unmatched case.
- **Steps:** Diff `review_item` and `recon_report.review_count` between run N and N+1.
- **Expected result / Acceptance criteria:** New items appear with the later `run` context; counts reflect the new totals; no double-counting of already-resolved items.
- **Traceability:** F9.3, RC-4, C-4.

### Reconciliation · ErrorHandling / gate

#### TC-E9-F9.3-003 · Go-live gate = NOT READY while blocking items open
- [ ] **Phase:** Reconciliation · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ErrorHandling
- **Objective:** The cutover-readiness check reports NOT READY whenever any **blocking** item (`decrypt_fail`, `orphan_identity`, `unmatched_placement`) is `Open`.
- **Preconditions:** At least one open blocking item.
- **Steps:**
  - `SELECT issue_type, COUNT(*) FROM review_item WHERE status='Open' AND issue_type IN ('decrypt_fail','orphan_identity','unmatched_placement') GROUP BY 1;`
  - Run the readiness CLI (`migrate readiness` or equivalent); observe verdict.
- **Expected result / Acceptance criteria:** With ≥1 open blocking item, readiness = NOT READY and lists the blocking counts; the gate refuses cutover.
- **Traceability:** F9.3, RC-3; F9.5 CV-2; FEATURE AC "Go-live gate blocks"; EPICS §8 (blocking set).

#### TC-E9-F9.3-006 · Non-blocking items alone do NOT block cutover
- [ ] **Phase:** Reconciliation · **POV:** Migration engineer/QA · **Priority:** P1 · **Type:** ErrorHandling
- **Objective:** With only `ambiguous_chain` (non-blocking) items remaining, readiness = READY (items tracked for post-launch).
- **Preconditions:** All blocking items resolved; only `ambiguous_chain` open.
- **Steps:** `SELECT DISTINCT issue_type FROM review_item WHERE status='Open';` (expect only `ambiguous_chain`); run readiness CLI.
- **Expected result / Acceptance criteria:** Readiness = READY; the remaining non-blocking items are reported as "tracked, non-gating".
- **Traceability:** F9.3, RC-3; FEATURE §7; FEATURE AC "Non-blocking items don't block cutover".

### Reconciliation · CountParity (bulk)

#### TC-E9-F9.3-005 · Bulk resolution maps many identical strings at once
- [ ] **Phase:** Reconciliation · **POV:** Migration engineer/QA · **Priority:** P1 · **Type:** CountParity
- **Objective:** Bulk-mapping N identical placement strings (e.g., 40 × "PLZ SNYN") resolves all N items in one action (RC-5).
- **Preconditions:** 40 `unmatched_placement` items with identical `placement_string`.
- **Steps:**
  - Before: `SELECT COUNT(*) FROM review_item WHERE status='Open' AND payload->>'placement_string'='PLZ SNYN';` → 40.
  - Perform the bulk-map to "Plaza Senayan".
  - After: same query → 0; `SELECT COUNT(*) FROM review_item WHERE status='Resolved' AND resolution->>'company_name'='Plaza Senayan';` → 40.
- **Expected result / Acceptance criteria:** All 40 flip to `Resolved` with the same resolution; subsequent run loads all 40 placements to the one company.
- **Traceability:** F9.3, RC-5; FEATURE AC "Bulk-map identical strings".

---

## 6. F9.4 — Load & Idempotent Re-runs

> BR refs LD-1..LD-7 (+LD-1b); cases C-1..C-4. INV-1 (upsert by crosswalk), INV-3 (dependency order).

### Load · CountParity

#### TC-E9-F9.4-001 · Per-entity load parity: loaded = source − in-review
- [ ] **Phase:** Load · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** CountParity
- **Objective:** After load, each target table's row count equals source minus rows still in review (and minus deferred), per `recon_report`.
- **Preconditions:** Load run complete against `tgt`.
- **Steps:** For each entity (employee, employment_agreement, placement, company, schedule, attendance, leave, overtime, payslip, salary_component, benefit):
  - `tgt`: `SELECT COUNT(*) FROM placement;`
  - Compare to `recon_report.loaded_count`; compare `recon_report.source_count - review_count` to the same.
- **Expected result / Acceptance criteria:** `loaded_count = source_count - review_count` for every entity; target `COUNT(*)` equals `loaded_count`; payslips ≈ employees × months (sanity).
- **Traceability:** F9.4, LD-6; F9.3 RC-4; E8 G-4.

### Load · Idempotency

#### TC-E9-F9.4-002 · Re-run load upserts by crosswalk — no duplicates
- [ ] **Phase:** Load · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** Idempotency
- **Objective:** Running the load twice updates existing rows (crosswalk hit) and never inserts duplicates (INV-1, LD-2).
- **Preconditions:** One completed load; same transformed dataset.
- **Steps:**
  - Baseline: `SELECT 'placement' t, COUNT(*) c FROM placement;` (per table) and total crosswalk count.
  - Re-run load.
  - Re-count; also check no duplicate business keys, e.g. `SELECT employee_id, start_date, client_company_id, COUNT(*) FROM placement GROUP BY 1,2,3 HAVING COUNT(*)>1;`
- **Expected result / Acceptance criteria:** Counts identical before/after the re-run; zero duplicate business keys; crosswalk count unchanged; updated-not-inserted confirmed (e.g., `xmax`/updated_at moved, id stable).
- **Traceability:** F9.4, LD-2, INV-1; C-2 (partial prior run continues cleanly); FEATURE AC "Idempotent upsert".

#### TC-E9-F9.4-003 · Re-run after resolving review items loads the formerly-queued rows
- [ ] **Phase:** Load · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** Idempotency
- **Objective:** Rows previously queued (e.g., unmatched placement) load on the post-resolution re-run while already-loaded rows upsert unchanged (C-1).
- **Preconditions:** A resolved `unmatched_placement`; prior load done.
- **Steps:** Re-run load. `SELECT COUNT(*) FROM placement WHERE client_company_id=:resolved_company;` increases by the resolved count; spot-check 3 previously-loaded placements are byte-identical (no churn).
- **Expected result / Acceptance criteria:** Newly-resolved rows now present; previously-loaded rows unchanged (no spurious updates, ids stable); recon review_count for that entity drops to match.
- **Traceability:** F9.4, LD-2, C-1; F9.3 RC-2.

### Load · IdentityRemap / dependency order

#### TC-E9-F9.4-004 · Load respects dependency order; no orphan FKs
- [ ] **Phase:** Load · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** IdentityRemap
- **Objective:** Entities load in order (identity/master → placement → schedule → attendance → leave → overtime → payroll) and **referential integrity holds** — no dangling FKs (INV-3, LD-1, LD-3).
- **Preconditions:** Load run complete.
- **Steps:** Run orphan-FK checks on `tgt`:
  - `SELECT COUNT(*) FROM employee e LEFT JOIN "user" u ON u.id=e.user_id WHERE u.id IS NULL;`
  - `SELECT COUNT(*) FROM placement p LEFT JOIN employee e ON e.id=p.employee_id WHERE e.id IS NULL;`
  - `SELECT COUNT(*) FROM placement p LEFT JOIN company c ON c.id=p.client_company_id WHERE c.id IS NULL;`
  - `SELECT COUNT(*) FROM placement p LEFT JOIN site s ON s.id=p.site_id WHERE s.id IS NULL;` (LD-1b Main Site)
  - `schedule.placement_id`, `attendance.schedule_id`/`placement_id`, `leave.employee_id`, `overtime.employee_id`, `payslip.employee_id`, `salary_component.payslip_id` — each LEFT JOIN parent, expect 0 nulls where FK required.
- **Expected result / Acceptance criteria:** Every orphan-FK query returns 0; `Placement.site_id` always resolves to the company's auto-created Main Site; load order is reflected in per-entity `recon_report` timestamps.
- **Traceability:** F9.4, LD-1, LD-1b, LD-3, INV-3; E2 DATA-MAPPING G-8; FEATURE AC "Load in dependency order".

### Load · ValueSpotCheck

#### TC-E9-F9.4-005 · Historical-state rules honored on load
- [ ] **Phase:** Load · **POV:** Migration engineer/QA · **Priority:** P1 · **Type:** ValueSpotCheck
- **Objective:** Imported history carries the correct closed-state flags per per-epic mappings (LD-7): attendance `Verified`, leaves at final status, payslips `source=Migrated`/`is_posted=true`/`payment_status=Paid` with `payroll_run_id` NULL.
- **Preconditions:** Load complete.
- **Steps:**
  - `SELECT COUNT(*) FROM attendance WHERE verification_status<>'Verified';` → 0.
  - `SELECT source, is_posted, payment_status, COUNT(*) FROM payslip GROUP BY 1,2,3;` → all `Migrated/true/Paid`; `SELECT COUNT(*) FROM payslip WHERE payroll_run_id IS NOT NULL;` → 0.
  - No net-new `PayrollRun`/`PayrollPayment`/`PayrollAdjustment` rows created by migration.
  - Leave final status carried; one `ANNUAL_POOL` `MIGRATION` LeaveQuota per employee (annual type CTHO/CT); statutory types have none from legacy.
- **Expected result / Acceptance criteria:** All flags match; migration creates no payroll-run/payment/adjustment rows; leave quota backfill exactly one ANNUAL_POOL MIGRATION lot per employee.
- **Traceability:** F9.4, LD-7; E5 G-5; E8 G-7; E6 G-5; FEATURE §6.

### Load · ErrorHandling

#### TC-E9-F9.4-006 · Failed batch rolls back transactionally; others unaffected
- [ ] **Phase:** Load · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ErrorHandling
- **Objective:** A batch hitting a constraint error rolls back wholly (no partial corruption) and is reported; sibling batches still commit (LD-4).
- **Preconditions:** Inject a constraint-violating row into one batch (e.g., a duplicate unique key) in a scratch run.
- **Steps:** Run load; inspect logs + `migration_run.stats`. `SELECT COUNT(*) FROM <entity>;` for the failed batch's entity; confirm no half-inserted batch rows; confirm other entities loaded.
- **Expected result / Acceptance criteria:** The failing batch leaves zero rows (full rollback); the failure is recorded in stats/log with the offending key; other batches/entities are fully loaded.
- **Traceability:** F9.4, LD-4; FEATURE AC "Batch failure rolls back".

#### TC-E9-F9.4-007 · Stricter-new-schema constraint → review item, not force-load; deferred missing-dep
- [ ] **Phase:** Load · **POV:** Migration engineer/QA · **Priority:** P1 · **Type:** ErrorHandling
- **Objective:** A row violating a constraint the legacy lacked becomes a review item (not force-inserted); a row whose dependency isn't loaded yet is deferred, not inserted with a dangling ref (LD-3, C-4).
- **Preconditions:** A row that violates a new not-null/check; a schedule whose placement is queued.
- **Steps:** Run load; check the violating row produced a review item and is absent from target; check the dependent schedule is deferred (counted as deferred/queued in recon), not present with null `placement_id` where required.
- **Expected result / Acceptance criteria:** Constraint-violating row → review item, absent from target; dependency-missing row deferred and reported, never force-inserted with dangling FK.
- **Traceability:** F9.4, LD-3, C-4; FEATURE AC "Defer when a dependency is missing".

### Load · Reconciliation (dry-run isolation)

#### TC-E9-F9.4-008 · Dry-run loads to scratch clone; production target untouched
- [ ] **Phase:** Load · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** Reconciliation
- **Objective:** Dry-run mode writes only to a scratch/clone DB; the real target is not mutated (LD-5).
- **Preconditions:** A populated (or empty baseline) production target; dry-run configured to a scratch DB.
- **Steps:** Snapshot target row counts/`MD5` signature; run a dry-run load; re-snapshot target; confirm scratch clone received the data.
- **Expected result / Acceptance criteria:** Production target signature unchanged after dry-run; scratch clone holds the loaded rows and emits its own recon report.
- **Traceability:** F9.4, LD-5; FEATURE AC "Dry-run does not touch production target".

---

## 7. F9.5 — Cutover, Validation & Rollback

> BR refs CV-1..CV-8; cases C-1..C-4. The big-bang switch; these are the final-run validation gates.

### Cutover validation · CountParity

#### TC-E9-F9.5-001 · Final-run per-entity record-count gate (source vs loaded)
- [ ] **Phase:** Cutover validation · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** CountParity
- **Objective:** On the frozen final snapshot, every entity's source count reconciles against loaded (the count gate of CV-3).
- **Preconditions:** Freeze in place; final F9.1–F9.4 pass complete; all blocking items resolved.
- **Steps:** For each entity compare frozen-source `COUNT(*)` to target `COUNT(*)` and to `recon_report`; assert `source = loaded + review` with review = 0 for blocking types.
- **Expected result / Acceptance criteria:** All entity counts reconcile; no blocking review items remain; the count gate passes. A mismatch is an automatic NO-GO.
- **Traceability:** F9.5, CV-2, CV-3; F9.3 RC-4; FEATURE AC "Validation gates pass → GO".

### Cutover validation · Reconciliation (gate)

#### TC-E9-F9.5-002 · Final run blocked while blocking review items open
- [ ] **Phase:** Cutover validation · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** Reconciliation
- **Objective:** Cutover is refused if any blocking item is open (the F9.3 gate enforced at F9.5 — CV-2).
- **Preconditions:** Leave one `decrypt_fail` open.
- **Steps:** Attempt the cutover/final-run command; observe it aborts with the open-blocking list.
- **Expected result / Acceptance criteria:** Cutover blocked; the open blocking items are listed; no traffic switch occurs.
- **Traceability:** F9.5, CV-2; F9.3 RC-3; FEATURE AC "Final run requires a clean review gate".

### Cutover validation · ValueSpotCheck

#### TC-E9-F9.5-003 · Payslip totals reconcile (decrypted sums, exact)
- [ ] **Phase:** Cutover validation · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ValueSpotCheck
- **Objective:** Aggregate payslip monetary totals match between decrypted source and target (read-only verbatim carry; no recompute) — the payslip-totals gate (CV-3).
- **Preconditions:** Final load done; decrypt successful (no open `decrypt_fail`).
- **Steps:**
  - Per period: `tgt`: `SELECT year, month, SUM(take_home_pay) thp, SUM(gross_earnings) ge, SUM(gross_deductions) gd FROM payslip GROUP BY 1,2 ORDER BY 1,2;`
  - `src` (decrypted): same aggregation over decrypted `employee_payslips`.
  - Per-agent spot-check 10 payslips: THP/gross fields equal to the decrypted source.
- **Expected result / Acceptance criteria:** Per-period sums match **exactly** (tolerance 0); sampled per-agent values equal; no double-counting of `employee_salaries` line items into payslip-summary totals (E8 G-3).
- **Traceability:** F9.5, CV-3; E8 DATA-MAPPING G-3, G-6 (read-only, no recompute); FEATURE AC "payslip totals reconcile".

#### TC-E9-F9.5-004 · Leave-balance reconciliation
- [ ] **Phase:** Cutover validation · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ValueSpotCheck
- **Objective:** Migrated annual leave balances reconcile to the legacy remaining quota (the leave-balance gate, CV-3).
- **Preconditions:** Final load; E6 quota backfill done.
- **Steps:**
  - `src`: `SELECT employee_id, leave_remaining FROM employee_leave_quotas WHERE <current period>;`
  - `tgt`: `SELECT employee_id, SUM(remaining) FROM leave_quota WHERE source='MIGRATION' AND cap_basis='ANNUAL_POOL' GROUP BY 1;`
  - Diff per employee.
- **Expected result / Acceptance criteria:** Each employee's migrated ANNUAL_POOL remaining equals legacy `leave_remaining`; exactly one MIGRATION lot per employee; statutory types carry no legacy balance (meter forward).
- **Traceability:** F9.5, CV-3; E6 DATA-MAPPING G-5, §4.

### Cutover validation · ErrorHandling (rollback / freeze)

#### TC-E9-F9.5-005 · Validation failure → NO-GO → rollback to ims-system; target reset
- [ ] **Phase:** Cutover validation · **POV:** Migration engineer/QA · **Priority:** P0 · **Type:** ErrorHandling
- **Objective:** A failed gate records NO-GO, keeps SWP on ims-system (unfreeze), and the partially-loaded target is discarded/reset so a crosswalk-keyed retry is clean (CV-5, C-4).
- **Preconditions:** Force a gate failure (e.g., a deliberate count mismatch) on a rehearsal.
- **Steps:** Run validation; observe go/no-go record; confirm legacy unfrozen; confirm target reset; re-run from scratch and confirm clean (no dupes via crosswalk).
- **Expected result / Acceptance criteria:** `go_no_go='NO-GO'` recorded with reason; ims-system unfrozen and serving; target discarded/reset; subsequent retry loads cleanly with no duplicates.
- **Traceability:** F9.5, CV-5, C-4; FEATURE AC "Validation fails → rollback".

#### TC-E9-F9.5-007 · Final run is idempotent on the frozen snapshot (rehearsal == final)
- [ ] **Phase:** Cutover validation · **POV:** Migration engineer/QA · **Priority:** P1 · **Type:** Idempotency
- **Objective:** Re-executing the final run on the frozen snapshot yields the same target state (rehearsal dry-runs predict the final exactly).
- **Preconditions:** Frozen snapshot; one completed final-equivalent run.
- **Steps:** Capture target count + crosswalk signature; re-run; compare.
- **Expected result / Acceptance criteria:** Identical counts and crosswalk signature; archived `migration_run` (final) recorded with validation results (CV-8).
- **Traceability:** F9.5, CV-8; F9.4 INV-1; C-3 (dry-runs size the window).

#### TC-E9-F9.5-008 · Freeze prevents legacy writes during the final run; caught delta re-migrated
- [ ] **Phase:** Cutover validation · **POV:** Migration engineer/QA · **Priority:** P1 · **Type:** ErrorHandling
- **Objective:** During the final run ims-system is read-only; any write that slips in is detected and re-migrated (CV-1, C-1).
- **Preconditions:** Freeze applied; attempt a legacy write in a rehearsal.
- **Steps:** Attempt an insert/update on `lumen_swp` while frozen → expect rejection. If a delta is detected (max id/updated_at advanced past snapshot), confirm it is captured and re-migrated, not lost.
- **Expected result / Acceptance criteria:** Legacy writes rejected under freeze; any detected post-snapshot delta is re-migrated; no silent drift between source-of-truth and target at switch time.
- **Traceability:** F9.5, CV-1, C-1; FEATURE AC "Freeze prevents legacy writes"; INV-4 (big-bang, no two-way sync).

---

## 8. Traceability summary

- **Invariants:** INV-1 (TC-F9.2-003/004, F9.4-002/003), INV-2 (TC-F9.1-001, F9.2-006/007/008, F9.3-004), INV-3 (TC-F9.4-004), INV-4 (TC-F9.5-008), INV-5 (TC-F9.1-004/006).
- **Blocking review types** (`decrypt_fail`, `orphan_identity`, `unmatched_placement`): TC-F9.1-006, F9.2-007/008, F9.3-003. **Non-blocking** (`ambiguous_chain`): TC-F9.2-009, F9.3-006. **Removed** (`unclassified_service_line`, 2026-06-12): TC-F9.3-001 asserts absence.
- **Identity split** (`users.id` vs `employees.id`): TC-F9.2-001, F9.4-004.
- **DBEncryption decrypt** (success + fail): TC-F9.1-004/006/007.
- **Placement-string crosswalk** (matched/unmatched/bulk): TC-F9.2-002/007, F9.3-002/005.
- **Role/company remap** (`role=2`=client, `agent`→NULL, INTERNAL seed, `employee_type`): TC-F9.2-005.
- **Idempotent re-run:** TC-F9.1-003, F9.2-004, F9.4-002/003, F9.5-007.
- **Reconciliation report balance + go-live gate:** TC-F9.3-003/004/006, F9.5-001/002.
- **Cutover runbook** (freeze, validation gates, rollback): TC-F9.5-001/002/003/004/005/008.
