# Phase 10: E8 Payroll - Context

**Gathered:** 2026-06-05 (autonomous — recommended decisions auto-accepted per user's overnight directive)
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the FE-used E8 "payroll" endpoints against the real BE and wire the screens off MSW,
proven with exhaustive full-stack Playwright E2E. E8 is **historical, read-only** payroll
(no active runs, no calculations, no editing — INV-1). The web surface is the **HR/Super Admin
payroll archive**: list payslips, open a payslip detail (full component breakdown + benefits),
list + create audit notes, and trigger an **async Excel export** (202 + job id, completed by the
River worker). Monetary fields are **encrypted at rest** (INV-2) and **role-gated** (INV-3/4):
agent self-service summaries are mobile-only (out of web scope). Since there is no payroll-run or
migration epic in this milestone, payslips + components + benefits are **seeded** (read-only).
</domain>

<decisions>
## Implementation Decisions

### Scope = the 5 FE-used hooks ONLY (fe-endpoint-inventory.md E8)
- `GET /payslips` (list, cursor + year/period/status filters), `GET /payslips/{id}` (detail —
  full component breakdown + benefits), `POST /payslips:export` (async Excel → 202 + job id),
  `GET /payslips/{id}/audit-notes` (list), `POST /payslips/{id}/audit-notes` (create).
- **OUT of scope:** active payroll runs/calculations, editing payslips, forward payroll-input
  export (INV-5 — E5/E7 exports cover external payroll inputs), PDF download (deferred v1.1 — Excel
  only), agent mobile self-service summaries.

### Read-only + encryption + RBAC (INV-1..4)
- **Read-only (INV-1):** no in-app create/edit of payslips; they are seeded. The only writes in
  this phase are audit notes (append-only) and export-job rows.
- **Encryption at rest (INV-2):** monetary fields (gross_earnings, gross_deductions,
  take_home_pay, component values, benefit values) are stored **encrypted**. Add a small platform
  crypto helper (`internal/platform/crypto`, AES-256-GCM, key from config/env) — there is no
  existing helper. Decrypt at the service boundary for authorized roles.
- **DECRYPT_FAIL is NOT an error — 200 OK with a row status (per the E8 contract):** a payslip
  whose ciphertext cannot be decrypted is returned in the list/detail with
  `status: "DECRYPT_FAIL"` and its monetary fields omitted/nulled (never a 4xx). Seed one such row
  (bad/legacy ciphertext) to exercise this path honestly. `status` filter: FINAL = normally
  decryptable.
- **RBAC (INV-3/4, §17):** the web archive is **hr_admin/super_admin only** (full breakdown +
  benefits + export + audit notes). `agent`/`shift_leader` → 403 `FORBIDDEN`/`OUT_OF_SCOPE` on the
  web archive endpoints. (Agent own-summary is mobile, not built here.)

### Async export via River (success criterion 2)
- `POST /payslips:export` enqueues a **River job** (the River client + worker already exist:
  `internal/platform/jobs/jobs.go` `EnqueueTx`/`registerWorkers`, `cmd/worker`) and returns
  **202 Accepted + { job_id, status }** (no synchronous file build on the request path).
- An **`export_jobs`** table tracks status (QUEUED → RUNNING → COMPLETED/FAILED) + the result
  (e.g. file ref / row count / `confidential: true`). A new `PayslipExportWorker` builds the Excel
  (or a faithful stand-in artifact) and marks the job COMPLETED. `EXPORT_TOO_LARGE` (per contract)
  when the filter matches too many rows.
- **E2E proves completion via the worker:** POST returns 202 + job_id; the worker (running in the
  harness, like the notification worker) processes the job; the test asserts the `export_jobs` row
  transitions to COMPLETED (poll via the DB helper, since E8 has no FE job-status hook — the E10
  export framework, Phase-11, adds the generalized status surface). Do NOT fake completion.

### Audit notes
- `GET /payslips/{id}/audit-notes` (list, chronological), `POST` (create — note text + author).
  Append-only; each create writes an audit_log row + notify stub (TODO Phase-11). RBAC hr/super.

### Build approach (mirror Phase-7/8/9 slice EXACTLY, minus the workflow state machine)
- migration → sqlc (`make gen`) → repository → service (apperr codes, RBAC, crypto decrypt at
  boundary, River enqueue) → hand-written chi handlers → routes in server.go under RequireRole →
  Go contract tests → FE wiring (MSW off) + live Playwright E2E. Match
  `docs/api/E8-payroll/openapi.yaml` byte-for-byte. Cursor pagination + filters (§11). New
  migrations: `payslips`, `payslip_components`, `payslip_benefits` (or a jsonb breakdown — match
  the contract), `payslip_audit_notes`, `export_jobs`. FKs to employees/placements. SWP IDs:
  check ids.go for PAY/PS/note/job prefixes; add only if missing. New query dir
  `backend/db/queries/payroll/`. New `PayslipExportWorker` registered in jobs.go.

### Seed (in 10-02)
- Several payslips for the seeded employees (Phase-4/5 SWP-EMP-1042/1108/2891/3001) across a few
  periods (e.g. 2025-11, 2025-12), with encrypted monetary fields + component breakdown + benefits;
  at least one `DECRYPT_FAIL` row (undecryptable ciphertext); a couple of audit notes on one
  payslip. Enough rows that the export produces a non-trivial file (but under EXPORT_TOO_LARGE).
- **TZ note:** clearly-in-range Asia/Jakarta dates.

### Plan split (4 plans, mirrors ROADMAP)
- **10-01** Migrations + sqlc + domain (`payslips`, components/benefits, `payslip_audit_notes`,
  `export_jobs`) + the platform crypto helper.
- **10-02** Services + handlers: read payslips (list/detail, decrypt-at-boundary + DECRYPT_FAIL
  row status), audit notes list/create, async export (River enqueue → 202 + job_id;
  PayslipExportWorker → export_jobs COMPLETED; EXPORT_TOO_LARGE), RBAC, audit, notify stub, seed.
  Edits server.go/main.go/seed.go + jobs.go (register worker).
- **10-03** Go contract tests vs E8 openapi (list/detail shapes incl. DECRYPT_FAIL status row,
  audit notes, export 202 + job_id, EXPORT_TOO_LARGE, RBAC 403, cursor shapes).
- **10-04** Full-stack Playwright E2E under NEW frontend/e2e/tests/e8/ (per Gherkin AC: archive
  list + filters, detail breakdown, DECRYPT_FAIL render, audit notes list/create, export 202 +
  worker-completes-job, RBAC 403). Selectors derived from the REAL e8-payroll components.

### Claude's Discretion
- Component/benefit storage (separate tables vs jsonb) — match the contract response shape.
- The Excel artifact: a real .xlsx (via a Go lib) vs a faithful stand-in file the worker writes +
  marks COMPLETED — pick the simplest that honestly proves "job completes via the worker"; if a
  full xlsx lib is heavy, a CSV/placeholder file is acceptable as long as the job lifecycle is real.
- Crypto key management (config/env constant for the milestone) — document; not production KMS.
- How the E2E observes job completion (DB poll helper) given no FE job-status hook in E8.
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Platform kernel (httpx cursor, rbac roles, audit, apperr + error.details, ids, db.TxManager,
  i18n, TZ). **River client + worker already exist** (`internal/platform/jobs/jobs.go`:
  `Client.EnqueueTx`/`Enqueue`/`registerWorkers`; `NotificationWorker` is the pattern to mirror for
  `PayslipExportWorker`; `cmd/worker/main.go` boots the worker; the E2E harness already runs the
  worker for notifications — confirm/extend).
- **Reference slices = Phase-2 foundations (read + list + simple writes, audit) and Phase-8/9
  (seed + RBAC + handlers).** No two-level state machine needed here (simpler than E6/E7).
- E2E harness (hardened detached boot): real stack + worker + resetDb + loginAs PERSONAS.* +
  window.__swp_get_token__ + db helpers (for polling export_jobs). Layout `frontend/e2e/tests/
  {e1..e7,smoke}/` → add `e8/`.

### Established Patterns
- Cursor list + filters; {data} detail envelope (unwrap on FE — recurring finding); apperr struct
  literals for non-default status; RBAC via RequireRole; audit-in-tx; notify stub (TODO Phase-11).
  FE errors via classifyError/error.details. DataTable rows div.border-b; .js E2E imports; PERSONAS.*.
- River enqueue inside a tx via EnqueueTx (transactional outbox) — enqueue the export job in the
  same tx that inserts the export_jobs QUEUED row.

### Integration Points
- New `backend/db/queries/payroll/` (sqlc glob). New `internal/platform/crypto` helper. New
  `PayslipExportWorker` registered in `jobs.go` registerWorkers + constructed with its deps.
  Routes in server.go authenticated group under RequireRole(hr_admin, super_admin). Seed extension.
  FE screens exist (e8-payroll/*, built from .pen) calling `@swp/api-client` e8 hooks via MSW —
  wire to real BE. E2E under new frontend/e2e/tests/e8/. resetDb must TRUNCATE payroll + export_jobs
  tables (and not break River's own tables).
</code_context>

<specifics>
## Specific Ideas
- The archive screen (payslip-archive-screen.tsx) + detail + audit-note drawer + export are the
  primary surfaces — E2E drives REAL selectors/overlays, not invented ones.
- DECRYPT_FAIL E2E: a seeded undecryptable payslip renders with the DECRYPT_FAIL state (the FE has
  payroll-states.tsx for this) — 200, not an error.
- Export E2E (the headline): trigger export → assert 202 + a job id in the response → wait for the
  River worker to process → assert the export_jobs row is COMPLETED (DB poll helper). Real worker,
  not mocked.
- RBAC E2E: agent/shift_leader cannot reach the web payroll archive (403).
- Audit-note E2E: create a note on a payslip → it appears in the list.

</specifics>

<deferred>
## Deferred Ideas
- Active payroll runs/calculations, payslip editing (read-only v1).
- Forward payroll-input export (INV-5), PDF download (v1.1).
- Agent mobile self-service summaries.
- The generalized E10 export framework + FE job-status surface — Phase-11 (this phase builds the
  payslip-export job mechanism that E10 generalizes).
- Notification dispatch implementation (stubbed; Phase-11).
</deferred>

---

*Phase: 10-e8-payroll*
*Context gathered: 2026-06-05 (autonomous)*
