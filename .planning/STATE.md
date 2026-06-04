---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: executing
stopped_at: Completed 07-e5-attendance/07-03-PLAN.md
last_updated: "2026-06-04T18:36:46.536Z"
last_activity: "2026-06-04 — Plan 07-03 complete: E5 contract tests = the drift gate. 31 table-driven Go tests over the REAL attendance + correction services/handlers (chi router + mutable principal, in-memory fake repos): list/cursor envelopes {data,next_cursor,has_more}, leader-scope + OUT_OF_SCOPE 403, cross-scope 404, verify/reject 200, VERIFY_OWN_RECORD 403, terminal CONFLICT 409 (fields.verification_status/status), missing-reason 400 INVALID_REQUEST, bulk partial-success {succeeded,failed} 200/422, idempotency replay + IDEMPOTENCY_KEY_REUSED 409 (in-memory stub middleware mirroring the Postgres contract — real store covered by 07-04 E2E), correction get-with-diff (check_out_at row), approve→APPLIED (+ attendance CORRECTED), OUTSIDE_CORRECTION_WINDOW 422 (fields.attendance_date + window_days="7", HR exempt), and the CORRECTION_ALREADY_PENDING seam. All byte-for-shape vs docs/api/E5-attendance/openapi.yaml. go test ./... / go build / go vet / gofmt clean; no regressions. Ready for 07-04 FE wiring + E2E."
progress:
  total_phases: 11
  completed_phases: 6
  total_plans: 33
  completed_plans: 32
  percent: 97
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-03)

**Core value:** Every screen the web app shows today works end-to-end against the real backend.
**Current focus:** Phase 7 — E5 Attendance (data layer landed; services/handlers next)

## Current Position

Phase: 7 of 11 (E5 Attendance) — IN PROGRESS
Plan: 3 of 4 in current phase — Plan 07-03 COMPLETE
Status: In progress
Last activity: 2026-06-04 — Plan 07-03 complete: E5 contract tests = the drift gate. 31 table-driven Go tests over the REAL attendance + correction services/handlers (chi router + mutable principal, in-memory fake repos): list/cursor envelopes {data,next_cursor,has_more}, leader-scope + OUT_OF_SCOPE 403, cross-scope 404, verify/reject 200, VERIFY_OWN_RECORD 403, terminal CONFLICT 409 (fields.verification_status/status), missing-reason 400 INVALID_REQUEST, bulk partial-success {succeeded,failed} 200/422, idempotency replay + IDEMPOTENCY_KEY_REUSED 409 (in-memory stub middleware mirroring the Postgres contract — real store covered by 07-04 E2E), correction get-with-diff (check_out_at row), approve→APPLIED (+ attendance CORRECTED), OUTSIDE_CORRECTION_WINDOW 422 (fields.attendance_date + window_days="7", HR exempt), and the CORRECTION_ALREADY_PENDING seam. All byte-for-shape vs docs/api/E5-attendance/openapi.yaml. go test ./... / go build / go vet / gofmt clean; no regressions. Ready for 07-04 FE wiring + E2E.

Progress: [██████████] 97%

## Performance Metrics

**Velocity:**
- Total plans completed: 3
- Average duration: ~35min
- Total execution time: ~1.75 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-test-harness-auth | 3 done / 5 total | ~105min | ~35min |
| Phase 01 P03 | 690 | 3 tasks | 16 files |
| Phase 01-test-harness-auth P05 | 2413 | 2 tasks | 4 files |
| Phase 02-e1-foundations P01 | 2 | 2 tasks | 7 files |
| Phase 02-e1-foundations P02 | 25 | 3 tasks | 9 files |
| Phase 02-e1-foundations P03 | 15 | 2 tasks | 1 files |
| Phase 02-e1-foundations P04 | 107 | 2 tasks | 7 files |
| Phase 03-e2-org-master-data P01 | 25 | 3 tasks | 22 files |
| Phase 03-e2-org-master-data P02 | 452 | 3 tasks | 8 files |
| Phase 03-e2-org-master-data P03 | 6 | 3 tasks | 8 files |
| Phase 03-e2-org-master-data P04 | 12 | 3 tasks | 8 files |
| Phase 03-e2-org-master-data P05 | 20 | 3 tasks | 4 files |
| Phase 03-e2-org-master-data P06 | 75 | 3 tasks | 9 files |
| Phase 04-e2-people P01 | 271 | 2 tasks | 13 files |
| Phase 04-e2-people P02 | 447 | 3 tasks | 9 files |
| Phase 04-e2-people P03 | 385 | 3 tasks | 9 files |
| Phase 04-e2-people P04 | 329 | 3 tasks | 8 files |
| Phase 04-e2-people P05 | 25 | 3 tasks | 3 files |
| Phase 04-e2-people P06 | 5400 | 4 tasks | 9 files |
| Phase 05-e3-placement P01 | 4 | 3 tasks | 9 files |
| Phase 05-e3-placement P02 | 18 | 3 tasks | 16 files |
| Phase 05-e3-placement P03 | 7 | 2 tasks | 3 files |
| Phase 05-e3-placement P04 | 75 | 3 tasks | 11 files |
| Phase 06-e4-schedule-shifts P01 | 5 | 3 tasks | 7 files |
| Phase 06-e4-schedule-shifts P02 | 11 | 3 tasks | 13 files |
| Phase 06-e4-schedule-shifts P03 | 4 | 2 tasks | 3 files |
| Phase 06-e4-schedule-shifts P04 | 69 | 2 tasks | 8 files |
| Phase 07-e5-attendance P01 | 5 | 2 tasks | 8 files |
| Phase 07-e5-attendance P02 | 12 | 3 tasks | 16 files |
| Phase 07-e5-attendance P03 | 5 | 2 tasks | 3 files |

## Accumulated Context

| Phase 01-test-harness-auth P02 | 2 | 2 tasks | 3 files |
| Phase 01-test-harness-auth P04 | 2 | 2 tasks | 8 files |

### Decisions

Full log in PROJECT.md Key Decisions. Recent:
- Scope = FE-used endpoints only (`.planning/reference/fe-endpoint-inventory.md` is the contract).
- No server-side OpenAPI codegen (oapi-codegen lacks 3.1 support) — hand-written handlers + Go contract tests.
- Full-stack Playwright E2E (real BE + ephemeral Postgres); exhaustive per Gherkin AC.
- One phase per epic, dependency-ordered, auth first.
- [Phase 01-test-harness-auth]: shift_leader company_id = SWP-CMP-0021 literal (FK not enforced until Phase 3 companies migration)
- [Phase 01-test-harness-auth]: cmd/seed exported password constants live in seed.go co-located with hashing logic; sequential inserts (no tx) for idempotent skip-if-exists
- [01-01]: webServer uses `vite dev` not `vite preview` — avoids build step; dev server reads VITE_* env vars at startup
- [01-01]: DB isolation = TRUNCATE app tables + reseed (not per-worker transactions — incompatible with real HTTP server)
- [01-01]: Ed25519 keypair generated fresh per run via `go run ./cmd/seed -genkeys` stdout (line1=privkey, line2=pubkey)
- [01-04]: buildSessionUser sets companyName = scope.company_id literal for shift_leader (no company-name endpoint in Phase 1); TODO(Phase-3) to resolve via companies endpoint
- [01-04]: credentials:'include' added to mutator.ts customFetch so ALL generated hooks send the refresh cookie cross-origin; BE sets CORS allow-origin for :4173/:5173
- [01-04]: logout handler lives in shell.tsx (useAuthLogout) and is passed to UserMenu as onLogout prop; UserMenu stays stateless re: auth
- [01-04]: forgot-password always advances to 'sent' even on network error (anti-enumeration, authentication.md C-2)
- [01-04]: reset-password minLength raised from 8 to 10 to match BE platform password policy (AU-4)
- [Phase 01-test-harness-auth]: TxRunner extracted as interface in service package to allow fake-based unit tests without testcontainers
- [Phase 01-test-harness-auth]: Reset token plaintext not emailed in Phase 1; E2E harness obtains token by querying password_reset_tokens directly (no mailer wired)
- [Phase 01-test-harness-auth]: Reset-token E2E acquisition: seedResetToken(email, plaintext) inserts sha256(plaintext) directly into password_reset_tokens — no mailer needed; E2E controls the plaintext presented to the browser
- [Phase 01-test-harness-auth]: Docker Scout CLI hook (config.json 'scout.hooks: pull') intercepts docker pull and hangs; workaround is to remove 'pull' from hooks and pull via curl --unix-socket docker.sock POST /images/create
- [Phase 02-e1-foundations]: ids.go unchanged — platform_settings keys are plain text (not SWP-prefixed); USR and AL prefixes already existed
- [Phase 02-e1-foundations]: foundations/ query package at db/queries/foundations/ — sqlc glob db/queries/* picks up new subdirectories automatically
- [Phase 02-e1-foundations]: platform_settings stored as flat key/value table matching openapi PlatformSettings shape; wave-2 maps rows to response object
- [Phase 02-e1-foundations]: chi ':' action suffix routes match natively: '/users/{user_id}:change-role' works without sub-router
- [Phase 02-e1-foundations]: status mapping: DB lowercase 'active'/'disabled' uppercased to ACTIVE/DISABLED only at DTO boundary
- [Phase 02-e1-foundations]: ip field always null in audit responses — audit_log table has no ip column (migration 00004 omission)
- [Phase 02-e1-foundations]: send-password-reset reuses auth.NewRefreshToken()+InsertResetToken (sha256, 1h TTL); no mailer; E2E reads from DB
- [Phase 02-e1-foundations]: Session revocation on deactivate: out of scope in Phase 2; only status set; auth-side revocation deferred
- [Phase 02-e1-foundations]: fakeTx instead of nil pgx.Tx: foundations service calls audit.Record inside InTx closures (unlike identity service); nil tx panics; fakeTx implements pgx.Tx with only Exec as no-op
- [Phase 02-e1-foundations]: Dynamic principal injection: harness.principal is a mutable field read by a closure middleware, so tests can swap roles without re-building the chi router
- [Phase 02-e1-foundations]: tryRestoreSession hydrates in-memory accessToken from httpOnly cookie before React mounts — enables page.goto() on authed routes in E2E
- [Phase 02-e1-foundations]: DataTable rows are div.border-b not tr — all E2E row locators must use div.border-b.filter() pattern
- [Phase 02-e1-foundations]: playwright.config.ts timeout: 90s to accommodate cold Vite compilation on first test run
- [Phase 03-e2-org-master-data]: geo_lat/geo_lng stored as nullable double precision; geofence_active derived at DTO boundary (not stored)
- [Phase 03-e2-org-master-data]: ListClientCompanies service_line+has_leader narg params accepted but (IS NULL OR TRUE) — no placements table in Phase 3
- [Phase 03-e2-org-master-data]: ids.go NOT modified — CMP/SITE/SVC/POS/LT/AC/OTR prefixes already existed
- [Phase 03-e2-org-master-data]: OrgCompanies Deps field in server.go; siblings 03-03/03-04 append their own r.Group{} after the ORG slice coordination point
- [Phase 03-e2-org-master-data]: GEOFENCE_RADIUS_INVALID uses apperr.Error{HTTPStatus:400} struct literal — bypasses statusForCode (which defaults to 422)
- [Phase 03-e2-org-master-data]: Seed uses explicit IDs SWP-CMP-0021/0022 + SWP-SITE-0001/0002 via direct INSERT with ON CONFLICT (id) DO NOTHING for deterministic E2E targets
- [Phase 03-e2-org-master-data]: ServiceLineService is a separate struct from Service in the same org package; ServiceLineHandler in same orghttp package — OrgServiceLines Deps field type = *orghttp.ServiceLineHandler
- [Phase 03-e2-org-master-data]: SoftDeletePosition uses repo.SoftDeletePosition (sets deleted_at) not SetPositionStatus — hard soft-delete matching 03-01 decision; SERVICE_LINE_IN_USE when CountActivePositionsForLine > 0; POSITION_IN_USE on unique (line,name) violation; seed uses explicit IDs SWP-SVC-001/002/003 + SWP-POS-014/015
- [Phase 03-e2-org-master-data]: MasterDataService is a separate struct from Service and ServiceLineService in org package; MasterDataHandler in same orghttp package; OrgMasterData Deps field type = *orghttp.MasterDataHandler
- [Phase 03-e2-org-master-data]: min_minutes<30 validation uses apperr.Rule('RULE_VIOLATION') before tx; OvertimeRule uses float64 in domain+DTO (float32 in sqlc); service_line_id is *string (nullable JSON null, never omitempty)
- [Phase 03-e2-org-master-data]: 3 master-data route groups: LT+AC reads all 4 roles; OTR reads excl agent (spec x-rbac); writes super_admin+hr_admin; seed explicit IDs SWP-LT-001/002 + SWP-AC-001/002 + SWP-OTR-001
- [Phase 03-e2-org-master-data]: Conflict toast text: t('errors.conflict')='Terjadi konflik dengan kondisi saat ini.' — regex /konflik/i not /conflict/i
- [Phase 03-e2-org-master-data]: noValidate required on RHF+Zod modal forms with type=number inputs to prevent browser native validation blocking submission
- [Phase 03-e2-org-master-data]: Toggle role=switch (not checkbox/button) per toggle.tsx — Playwright must use getByRole('switch')
- [Phase 04-e2-people]: Bytea blob for agreement_attachments: simplest approach that passes E2E and survives container teardown via reseed; no external storage dependency
- [Phase 04-e2-people]: EA-2 enforced at DB level via partial unique index on employment_agreements(employee_id) WHERE status='active' AND deleted_at IS NULL
- [Phase 04-e2-people]: File prefix FILE added to ids.go for SWP-FILE attachment IDs
- [Phase 04-e2-people]: GET /employees/{id} RBAC: web roles only (super_admin, hr_admin, shift_leader) — agent excluded; agent self-service is mobile-only in Phase 4
- [Phase 04-e2-people]: EP-3 login provisioning stub: provision_login/login_email accepted but UserID stays NULL; no E1 user created in Phase 4 employees milestone
- [Phase 04-e2-people]: seedEmployees() called before persona user loop in Seed() — ordering contract for /auth/me employee resolution
- [Phase 04-e2-people]: EXPIRING virtual status computed at DTO boundary (toAgreementResponse): status=active+PKWT+end_date<now+30d → emit EXPIRING; persisted DB status stays active
- [Phase 04-e2-people]: FILE_TOO_LARGE uses apperr.Error{HTTPStatus:413} struct literal — bypasses statusForCode (no 413 mapping); same technique as GEOFENCE_RADIUS_INVALID in Phase 3
- [Phase 04-e2-people]: ACTIVE_AGREEMENT_EXISTS uses apperr.Conflict() (409) not apperr.Rule() (422 default) — state-of-record constraint, not a semantic rule
- [Phase 04-e2-people]: Approve applies whitelisted fields only (phone/address/bank_account): buildApproveParams overlays CR.Changes onto a full copy of current employee; statutory fields never touched
- [Phase 04-e2-people]: Notification dispatch on CR resolution deferred: stub comment in ApproveChangeRequest + RejectChangeRequest marks Phase N (notifications epic) integration point
- [Phase 04-e2-people]: Old->new diff computed live in GetChangeRequestDetail (not stored snapshot): old = current employee values at query time; audit_log captures exact before/after at approve time
- [Phase 04-e2-people]: FILE_TOO_LARGE 413 and PKWT_PERIOD_EXCEEDS_MAX 422 confirmed by contract tests: apperr.Error struct literals bypass statusForCode
- [Phase 04-e2-people]: BankAccount json tags: added snake_case json tags to domain.BankAccount so diff serialization uses keys FE formatDiffValue expects; also fixes jsonb unmarshal from DB seed
- [Phase 04-e2-people]: RenewAgreement: supersede predecessor before insert — releases partial unique index on active employee; prevents ACTIVE_AGREEMENT_EXISTS on renew
- [Phase 04-e2-people]: window.__swp_get_token__ E2E helper: exposes in-memory access token on window in VITE_ENABLE_MSW=false mode; allows page.evaluate() to make authenticated API requests
- [Phase 05-e3-placement]: Placement id allocated via column DEFAULT ('SWP-PL-'||swp_next_id('PL')) — CreatePlacement/CreateShiftLeaderAssignment omit id from the INSERT column list (DEFAULT fires); diverges from Phase-4 inline-INSERT allocation site but behaviour-identical
- [Phase 05-e3-placement]: INV-1 enforced by placements_active_employee_uq partial unique index WHERE lifecycle_status IN ('ACTIVE','EXPIRING','PENDING_START','SCHEDULED') AND deleted_at IS NULL ('SCHEDULED' inert forward-compat); INV-2 via sla_active_company_uq + sla_active_site_uq; INV-3 via sla_active_employee_uq
- [Phase 05-e3-placement]: placement_history uses bigserial PK (no SWP id, avoids touching ids.go); GetPlacementChain via recursive CTE walks predecessor+successor links
- [Phase 05-e3-placement]: error.details envelope added additively (apperr.Details + ConflictWithDetails + httpx omitempty) so INV_1..4 409s carry INVViolationDetails; Phase 1-4 errors stay byte-identical
- [Phase 05-e3-placement]: INV-1 = service pre-check + FOR UPDATE re-check + 23505 partial-unique backstop; INV-2/3/4 enforced in one InTx with the 05-01 ...ForUpdate locks; PENDING_START does not satisfy INV-4
- [Phase 05-e3-placement]: lifecycle_status derived at the DTO boundary (Asia/Jakarta): persisted ACTIVE+end<=today+30d -> EXPIRING; PENDING_START+start<=today -> ACTIVE (mirrors Phase-4 toAgreementResponse)
- [Phase 05-e3-placement]: PlacementService + ShiftLeaderService mutually referential via SetLeaderService (current-leader joins + SL-6 auto-vacate); transfer reuses source site_id (no site in FE request / no primary-site query in 05-01)
- [Phase 05-e3-placement]: [05-03] E3 contract tests = drift gate: fakePlacementRepo+fakeShiftLeaderRepo (shared placement state) + fakeTx httptest harness mirroring Phase-4 agreements; INV-4 join + current-leader resolve off the same fixtures
- [Phase 05-e3-placement]: [05-03] Site-scope leadership is contract-tested here (distinct per-site unit); 05-04 FE E2E targets company-scope only per CONTEXT.md deferred decision
- [Phase 05-e3-placement]: [05-03] Asia/Jakarta-midnight vs UTC-midnight boundary: a same-day start derives PENDING_START under the fixed clock; ACTIVE-on-create asserted via backdated start — 05-04 E2E must respect this when picking dates
- [Phase 05-e3-placement]: [05-04] ApiError now captures error.details (was dropped) so the INV-1 conflict Banner renders current_placement; Phase 1-4 errors unaffected (details optional)
- [Phase 05-e3-placement]: [05-04] roster filters/pagination navigate to /client-companies/{id}/roster (was the company detail route) + route gains validateSearch; fixed all roster filter/toggle interactions
- [Phase 05-e3-placement]: [05-04] INV-3 unreachable in company-scope FE (INV-4 precedence + INV-1); E2E asserts reachable 409, pure INV_3 envelope is contract-tested in 05-03; negative invariants asserted via apiAs real-409 + INV-1 also via the create-form Banner
- [Phase 05-e3-placement]: [05-04] e3-helpers (apiAs token-fetch + pickCombobox + comboFieldById) is the reusable E2E pattern for token API calls + FK-picker driving; status filters match PERSISTED lifecycle_status (EXPIRING is DTO-derived, not server-filterable)
- [Phase 06-e4-schedule-shifts]: [06-01]: INV-1 DOUBLE_SHIFT backstop = partial unique index schedule_entries_active_agent_date_uq on (employee_id, work_date) WHERE deleted_at IS NULL (mirrors Phase-5 placements_active_employee_uq); service pre-checks via FindLiveEntryForAgentDate then catches 23505
- [Phase 06-e4-schedule-shifts]: [06-01]: over-leave = minimal E4-owned approved_leave_days table (bigserial PK, employee_id+leave_date unique, denormalized leave_request_id with NO FK) — exercises SHIFT_OVER_LEAVE now without colliding with E6's leave_requests / SWP-LR namespace; E6 (Phase 8) later populates/supersedes
- [Phase 06-e4-schedule-shifts]: [06-01]: shift/schedule time columns are text HH:MM (not SQL time) matching openapi/FE snapshot render; sqlc returns date cols as pgtype.Date (06-02 repo converts <-> time.Time like Phase-5); in_use_count is int64; ids.go untouched (SHF/SCH prefixes already present)
- [Phase 06-e4-schedule-shifts]: [06-02]: shared ordered 6-check conflict engine (Evaluate) reused by create/update/:check/:bulk-apply; resolves placement first (scope source), emits OUTSIDE_PLACEMENT_PERIOD when no placement, OUT_OF_SCOPE before any 422 when placement found; each code carries explicit apperr HTTPStatus (403/422/409)
- [Phase 06-e4-schedule-shifts]: [06-02]: bulk-apply = per-cell own-tx atomicity (CreateEntry loop); one failing cell never rolls back successes; handler 200 if >=1 succeeded else 422; :check runs the same expansion engine-only (no writes/audit/notify)
- [Phase 06-e4-schedule-shifts]: [06-02]: over-leave delivered honestly via real approved_leave_days read; seed plants SWP-LR-44210 for EMP-3001 (monday+3) so SHIFT_OVER_LEAVE is exercisable now; PATCH /schedule re-runs engine with ForceReplace=true (self-edit not a double-shift); leader past-date DELETE → 403 (C-5)
- [Phase 06-e4-schedule-shifts]: [06-03]: E4 contract tests = drift gate; fakeShiftMasterRepo (nameIndex DUPLICATE_NAME sentinel) + fakeScheduleRepo (placements/approvedLeave/liveEntry maps) over the REAL services+handler; all six conflict codes asserted with exact status+code+details, bulk-apply 200/422/weekdays_mask, :check no-write, force_replace MODIFIED+replaced_entry_id, leader past-date DELETE 403
- [Phase 06-e4-schedule-shifts]: [06-03]: OUTSIDE_PLACEMENT_PERIOD asserted on code+422 only (engine emits it precisely when NO placement covers the date, so there is no placement to populate the detail — honest, not weakened); other detail-bearing codes assert full details
- [Phase 06-e4-schedule-shifts]: [06-04]: FE conflict-details fix — ShiftPickerPopover reads :check failed[].error.details (was conflict_details, always undefined) so DOUBLE_SHIFT/over-leave block messages render against the real BE (mirrors Phase-5 error.details precedent)
- [Phase 06-e4-schedule-shifts]: [06-04]: reset-db truncates E4 tables (schedule_entries/approved_leave_days/shift_masters before placements) so test-created schedule entries reset (seed is ON CONFLICT DO NOTHING); waitForToken() added to dodge the post-goto 401 race (in-memory access token re-hydrated async by tryRestoreSession)
- [Phase 06-e4-schedule-shifts]: [06-04]: SHIFT_OVER_LEAVE delivered honestly via the seeded approved_leave_days row (SWP-LR-44210) — asserted via real 409 details.leave_request_id AND the real popover :check block toast; E6 (Phase 8) wires the production leave source. CH-1 creates a future cell first (C-5 leader past-date DELETE guard)
- [Phase 07-e5-attendance]: [07-01]: geofence/lateness/auto-close are plain STORED columns on attendance (in_geofence/in_distance_m/out_geofence/out_distance_m/geofence_radius_m + is_late/late_minutes/auto_closed) — no runtime Haversine, no clock pipeline; 07-02 seeds them directly for honest PENDING exceptions
- [Phase 07-e5-attendance]: [07-01]: ApproveCorrection sets status='APPLIED' directly (no APPROVED intermediate) and 07-02 calls ApplyCorrectionToAttendance in the same tx (COALESCE whitelist of check_in/out + attendance_code; appends de-duped CORRECTED flag; sets last_correction_id)
- [Phase 07-e5-attendance]: [07-01]: company_id + attendance_shift_date denormalized onto attendance_corrections so leader-scope queue + OUTSIDE_CORRECTION_WINDOW 7-day check need no JOIN; CORRECTION_ALREADY_PENDING backstopped by partial unique index (attendance_id) WHERE status='PENDING'
- [Phase 07-e5-attendance]: [07-01]: sqlc quirks for 07-02 repo — original_snapshot jsonb→[]byte (json marshal/unmarshal map[string]any), attendance_shift_date date→pgtype.Date, integer cols→int32, wfo→Wfo, flags text[]→[]string; new internal/domain/attendance/ SUBPACKAGE (not flat package domain) per plan
- [Phase 07-e5-attendance]: [07-02]: VERIFY_OWN_RECORD=403 + terminal verify/reject=409 CONFLICT(fields.verification_status) + terminal correction=409 CONFLICT(fields.status) — matched openapi over CONTEXT; bulk verify/reject = loop-single + apperr.As → {succeeded,failed} 200/422, idempotency owned by the router middleware (not the service)
- [Phase 07-e5-attendance]: [07-02]: CorrectionService.Approve applies the proposed change to the target attendance in the SAME tx (attRepo.ApplyCorrectionToAttendance COALESCE whitelist + CORRECTED flag + last_correction_id) then ApproveCorrection→APPLIED; OUTSIDE_CORRECTION_WINDOW exposed as exported CheckCorrectionWindow(shiftDate,isHR,now) seam for 07-03 (correction-CREATE is out of web scope)
- [Phase 07-e5-attendance]: [07-02]: DTO required-nullable openapi fields (check_out_at/schedule_id/geofence_out/verified_by/lat_out...) are pointers WITHOUT omitempty → serialize as JSON null; denormalized display names use omitempty; cross-scope reads return 404 (hide existence), write-path scope returns 403 OUT_OF_SCOPE
- [Phase 07-e5-attendance]: [07-03]: E5 contract tests mirror the Phase-6 scheduling harness EXACTLY — fakeTx (Exec no-op for audit-in-tx) + fakeTxRunner + in-memory fake repos over the REAL svc.AttendanceRepository/CorrectionRepository ports + newHarness(role,company,employee) mounting the real services+handler on chi with a mutable-principal closure middleware; this is the drift gate replacing server codegen
- [Phase 07-e5-attendance]: [07-03]: idempotency replay asserted via an in-memory stubIdempotency middleware (scoped by principal UserID) mirroring the Postgres contract at the same router position as server.go — same key+body replays status/body (+ Idempotent-Replayed); same key+different body → 409 IDEMPOTENCY_KEY_REUSED. Real Postgres-backed store (needs *db.Pool) is exercised by 07-04 E2E (documented seam)
- [Phase 07-e5-attendance]: [07-03]: CORRECTION_ALREADY_PENDING asserted as a SEAM (create endpoint is mobile/agent-only, OUT of web scope; backstopped by the 07-01 partial-unique index) — fake countPending detects two PENDING corrections + the 409 + fields.pending_correction_id wire shape via apperr.ConflictWithDetails; OUTSIDE_CORRECTION_WINDOW 422 driven through the REAL CorrectionService.Approve for both leader-422 and HR-exempt branches

### Pending Todos

None.

### Blockers/Concerns

- Phase 1 depends on Docker being available for the ephemeral Postgres in the E2E harness.

## Session Continuity

Last session: 2026-06-04T18:35:57.495Z
Stopped at: Completed 07-e5-attendance/07-03-PLAN.md
Resume file: None
