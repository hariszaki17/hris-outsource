---
gsd_state_version: 1.0
milestone: v1.2
milestone_name: Mobile MVP (Agent App)
status: in_progress
stopped_at: Phase 14 complete (FE+BE); Phase 15 (attendance correction) next
last_updated: "2026-06-08T00:00:00.000Z"
last_activity: "2026-06-08 — Phase 14 (clock in/out + geofence + my-attendance) COMPLETE, full-stack. FIRST rebased the mobile branch onto the parallel backend's new E5 commit (feat/backend-impl 3816b3a: attendance filters + true-absence + absence-sweep) so clock-in builds on current attendance code. BE (new agent clock path, no migration): clock.sql (4 queries) + ClockService (Haversine geofence + late/early eval) + ClockRepo + ClockHandler; POST /attendance:clock-in|:clock-out (RequireRole agent, Idempotency); opened GET /attendance + /{id} to agent self-scope (List forces caller employee_id; Get 404s others). Contract-exact: GPS_UNAVAILABLE/OUT_OF_GEOFENCE(force)/ALREADY|NOT_CLOCKED_IN, verification flag→PENDING else AUTO_APPROVED/VERIFIED. go build+vet+45 tests green, make gen idempotent. FE: expo-location GPS, clock card with geofence force-confirm flow + all variants, my-attendance history + StatusBadge + Attendance tab, i18n. tsc/biome/expo export 1780 green. CLOCK-03 (clock photo) DEFERRED. NOTE: BE edits attendance_service.go/server.go/cmd/api — conflicts with parallel backend work; coordinate merge. human_needed: live clock needs device GPS + running BE. Next: Phase 15 — attendance correction (backend NEW)."
progress:
  total_phases: 8
  completed_phases: 2
  total_plans: 2
  completed_plans: 2
  percent: 25
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-08)

**Core value:** An agent runs their full daily work loop from the phone, against the real Go backend.
**Current focus:** Milestone v1.2 — Mobile MVP (Agent App). Phases 13–14 done; Phase 15 (attendance correction) next.

## Current Position

Phase: 15 of 20 (Attendance correction) — next
Plan: Phase 14 COMPLETE (FE ad6d5c4 + BE 5f1f968)
Status: Phase 14 clock in/out + geofence shipped full-stack (build/tests green; live needs device GPS). Branch rebased onto current backend.
Last activity: 2026-06-08 — Phase 14 complete; clock-in/out endpoints + RN screen, geofence + my-attendance.

Progress: [██        ] 25% (2/8 phases)

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
| Phase 07-e5-attendance P04 | 75 | 3 tasks | 7 files |
| Phase 08-e6-leave P01 | 4 | 2 tasks | 8 files |
| Phase 08-e6-leave P02 | 38 | 3 tasks | 22 files |
| Phase 08-e6-leave P03 | 6 | 2 tasks | 4 files |
| Phase 08-e6-leave P04 | 64 | 3 tasks | 8 files |
| Phase 09-e7-overtime P01 | 5 | 3 tasks | 5 files |
| Phase 09-e7-overtime P02 | 11 | 3 tasks | 14 files |
| Phase 09-e7-overtime P03 | 6 | 2 tasks | 3 files |
| Phase 09 P04 | 51 | 3 tasks | 8 files |
| Phase 10-e8-payroll P01 | 20 | 3 tasks | 10 files |
| Phase 10-e8-payroll P02 | 30 | 3 tasks | 16 files |
| Phase 10-e8-payroll P03 | 15 | 2 tasks | 3 files |
| Phase 10-e8-payroll P04 | 75 | 3 tasks | 11 files |
| Phase 11-e10-reporting P01 | 7 | 3 tasks | 12 files |
| Phase 11-e10-reporting P02 | 11 | 3 tasks | 15 files |
| Phase 11-e10-reporting P02b | 9 | 2 tasks | 18 files |
| Phase 11-e10-reporting P03 | 6 | 2 tasks | 5 files |
| Phase 11 P04 | 53 | 3 tasks | 11 files |

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
- [Phase 07-e5-attendance]: [07-04]: harness orphan-API fix — go run ./cmd/api does not forward SIGTERM to its exe/api child; freePort(8081) before boot + detached process-group kill on teardown (stale binary was serving old routes → 404 on new E5 endpoints)
- [Phase 07-e5-attendance]: [07-04]: E5 E2E = 5 specs/18 tests green vs real FE+Go+ephemeral PG; bulk partial-success + Postgres idempotency replay driven via apiAs for determinism; OUTSIDE_CORRECTION_WINDOW stays contract-only (mobile create path out of web scope); no conflict_details bug in E5 (FE audit no-op)
- [Phase 08-e6-leave]: [08-01]: leave_approvals is a separate bigserial decision-trail table (not denormalized columns) — mirrors placement_history; feeds the FE timeline[] + the FEATURE ER decision log
- [Phase 08-e6-leave]: [08-01]: leave_quotas remaining = total-used-pending is a DERIVED domain method (LeaveQuota.Remaining()), never stored; pending recompute is computed-on-read via CountPendingLeaveDaysForQuota (no trigger)
- [Phase 08-e6-leave]: [08-01]: SWP-LR display id reconciled — new leave_requests.id shares swp_next_id('LR') with Phase-6's literal SWP-LR-44210 fixture (no collision); 08-02 INV-3 write-through inserts the REAL id into approved_leave_days, replacing the fixture mechanism
- [Phase 08-e6-leave]: [08-01]: sqlc quirks for 08-02 repo — dates→pgtype.Date, jsonb(last_adjustment/last_override)→[]byte, ints→int32, CountPendingLeaveDaysForQuota→int64, leave_approvals.ID→int64; INV-3 loop-closer queries (InsertApprovedLeaveDay ON CONFLICT + CancelScheduleEntriesForLeave→CANCELLED_BY_LEAVE) live in scheduling/ dir, added by 08-02
- [Phase 08-e6-leave]: [08-03]: E6 drift gate mirrors the Phase-7 attendance harness EXACTLY — fakeTx + in-memory fake leave/quota repos + a recording fakeScheduleRepo (svc.SchedulePort) over the REAL services+handler via newHarness(role,company,employee) on chi with a mutable-principal closure middleware + stubIdempotency at the server.go router position
- [Phase 08-e6-leave]: [08-03]: INV-3 loop-closer asserted at the service-contract level — fakeScheduleRepo records cancelCalls + insertedDays and returns a configurable schedule_impact[] so the over-balance approve-final blocks BEFORE the tx (no deduct/cancel) while override deducts into negative remaining + sets last_override + fires INV-3; the CANCELLED_BY_LEAVE → LEAVE DTO mapping is pinned on the wire
- [Phase 08-e6-leave]: [08-03]: decodeBody snapshots rr.Body.Bytes() so one response is re-decodable (errCode + errFields) — the one deliberate divergence from the 07-03 attendance harness which only decoded once
- [Phase 08-e6-leave]: [08-04]: E6 FE detail unwraps the BE {data:<LeaveRequest>} envelope (E6 openapi declares bare LeaveRequest so Orval narrows query.data.data to it; handler wraps in dataResponse like every epic) — fixed toward what the BE returns
- [Phase 08-e6-leave]: [08-04]: override modal opens off ApiError.code BALANCE_RECHECK_FAILED (the 422's error.fields make classifyError 'validation' not 'rule'; Bahasa msg lacks 'BALANCE'); detail GET never pre-flags requires_override (BE re-checks only at approve-final) so the 422 error path is the real override trigger
- [Phase 08-e6-leave]: [08-04]: schedule_impact[].new_status='LEAVE' asserted on the approve-final ACTION RESPONSE (LeaveService.Get re-derives only the timeline, not schedule_impact); INV-3 pre-condition probes monday+2 → DOUBLE_SHIFT (engine: SHIFT_OVER_LEAVE precedes DOUBLE_SHIFT); post-approval → SHIFT_OVER_LEAVE from the real approved_leave_days row + GET /schedule status=CANCELLED_BY_LEAVE
- [Phase 08-e6-leave]: [08-04]: E6 full-stack Playwright suite (21 tests/5 specs) green headless vs real FE+Go+ephemeral PG; full e1-e6 run 184 passed/6 skipped/0 failed — no regressions; seeded remaining is total-used-PENDING (Dewi 5, Budi -3)
- [Phase 09-e7-overtime]: [09-01]: overtime.holiday_id is a plain text column in 00031; the FK to holidays(id) is added via ALTER TABLE in 00032 (goose runs 00031 before 00032) — deferred cross-migration FK pattern; Down drops the constraint before the table
- [Phase 09-e7-overtime]: [09-01]: overtime_approvals is a separate bigserial decision-trail table (mirrors leave_approvals) — level int CHECK(1,2), decision CHECK APPROVED/REJECTED/OVERRIDE_APPROVED; avoids touching ids.go
- [Phase 09-e7-overtime]: [09-01]: reference_multiplier numeric(4,2) STORED only — pgtype.Numeric -> *float64, NO monetary method on domain.Overtime (INV-2); TierPrecedence resolves HOLIDAY>RESTDAY>WORKDAY; holiday_date column avoids reserved-word date; overtime_rules CRUD reused from E2/Phase-3
- [Phase 09-e7-overtime]: [09-01]: sqlc quirks for 09-02 repo — work_date/holiday_date->pgtype.Date, minutes/level->int32, reference_multiplier->pgtype.Numeric(nullable)<->*float64, applicable_service_lines->[]string, employee_name/company_name->*string(LEFT JOIN), CountOvertimeUsingHoliday->int64; GetOvertimeForUpdate FOR-UPDATE lock + UpdateOvertimeStatus RETURNING-or-409
- [Phase 09-e7-overtime]: [09-02]: OvertimeRepo is dual-port (OvertimeRepository+RuleRepository); FindOvertimeRule reuses E2 overtime_rules (line-scoped wins over NULL-line global default) — no rule CRUD; SchedulePort typed on schedulingsvc.LiveEntry so the existing scheduleRepo satisfies it verbatim for WORKDAY/RESTDAY classification
- [Phase 09-e7-overtime]: [09-02]: SELF_APPROVAL_FORBIDDEN via apperr.Error{HTTPStatus:403} struct literal; calculation tier_breakdown single-tier (supersedes null), multiplier = rule per-tier rate REFERENCE only (INV-2); bulk dispatches HR->ApproveFinal/leader->ApproveL1 each in own tx, SELF/OUT_OF_SCOPE/409 land in failed[]; :confirm guardConfirmActor enforces agent-self, staff pass for web seam
- [Phase 09-e7-overtime]: [09-03]: E7 contract tests are the drift gate — fakeOvertimeRepo dual-port (OvertimeRepository+RuleRepository) + fakeHolidayRepo (configurable in-use) + fakeScheduleRepo over the REAL services+handler via newHarness on chi; OT_BELOW_MIN + ClassifyDayType asserted through the REAL exported seams (h.otSvc) because their only production trigger is the out-of-web-scope create/auto-detect path
- [Phase 09]: [09-04] overtime-detail-screen unwraps the {data:<Overtime>} GET envelope with a bare fallback (was rendering blank); web confirm/withdraw driven via apiAs (agent-self UI is out of web scope); OT_BELOW_MIN asserted honestly via the seeded below-min calc (no web HTTP trigger); openRules retries the deep-route auth-restore redirect race. 25 e7 E2E green; full e1–e7 = 209 passed/6 skipped/0 failed.
- [Phase 10-01]: monetary fields stored as *_enc bytea AES-256-GCM ciphertext (INV-2) — NO plaintext money column; decrypt at the 10-02 service boundary
- [Phase 10-01]: export_jobs terminal-success status is DONE (openapi), not COMPLETED (CONTEXT prose); crypto.ErrDecrypt is the typed DECRYPT_FAIL source with DecryptPtr null/valid/garbage three-case seam
- [Phase 10-01]: payslip_audit_notes.id is service-assigned composite '{payslip_id}-NOTE-{seq}' (not swp_next_id); seq via CountPayslipAuditNotes+1
- [Phase 10-02]: Repo returns RAW *_enc ciphertext; service owns the single decryptMoney seam (DecryptPtr garbage→DECRYPT_FAIL). Whole-payslip DECRYPT_FAIL = 200 with money nulled + breakdown [].
- [Phase 10-02]: Async export = transactional outbox: InsertExportJob(QUEUED) + jobs.EnqueueTx(PayslipExportArgs) in one tx; pool-backed PayslipExportWorker flips export_jobs RUNNING→DONE (CSV/row-count stand-in). svc.Jobs interface seam lets 10-03 fake River.
- [Phase 10-e8-payroll]: [10-03]: E8 contract tests = drift gate; DECRYPT_FAIL asserted via seedDecryptFail random-garbage bytea through the REAL crypto.Decrypt (200 row status, not a stub flag) in BOTH list+detail; export 202 + transactional-outbox enqueue asserted via recording fakeJobs (one PayslipExportArgs, matching JobID); harness copies the Phase-9 overtime testkit under RequireRole(super_admin, hr_admin).
- [Phase 10-e8-payroll]: [10-04]: payslip-detail unwraps the BE {data:Payslip} envelope with a bare fallback (openapi declares bare Payslip, handler wraps it; Phase-8 [08-04] precedent)
- [Phase 10-e8-payroll]: [10-04]: harness boots the River worker (cmd/worker, detached, PAYROLL_ENCRYPTION_KEY) so the export job completes; River queue migrations applied programmatically via cmd/migrate river-up (no river CLI)
- [Phase 10-e8-payroll]: [10-04]: export E2E proves worker completion via pollExportJob (export_jobs.status DONE, row_count>0) since E8 has no FE job-status hook; 16 e8 specs green, full e1-e8 225 passed/6 skipped/0 failed
- [Phase 11-e10-reporting]: [11-01]: export_jobs GENERALIZED via ALTER 00036 (not recreate) — +CANCELLED status/+EXCEL format/+report_type/filters(jsonb)/audit_log_entry_id/progress_percent/expires_at, all nullable/defaulted; Phase-10 PAYSLIP path unchanged. DB status RUNNING/DONE maps to wire PROCESSING/COMPLETED at the 11-02b DTO boundary
- [Phase 11-e10-reporting]: [11-01]: reporting aggregation SQL aligned to REAL E5..E9 schema (verification_status PENDING/VERIFIED not PENDING_VERIFY; PENDING_L1/HR not bare PENDING; placements client_company_id; check_in_at::date as shift date; is_billable codes) — plan prose used placeholder names. GetExportJob renamed GetExportJobGeneric (shared sqlcgen pkg); mapExportJob made generic (00036 ALTER split Insert/Get Row types); min(text)::text → string not interface{}
- [Phase 11-e10-reporting]: [11-02]: notify seam injected via SetNotifier(jobs.Dispatcher) not the constructor — prior services' constructor signatures + drift-gate test harnesses unchanged; nil-safe notify.Dispatch no-ops when unwired
- [Phase 11-e10-reporting]: [11-02]: NotificationWorker un-stubbed to INSERT via sqlcgen directly (cycle-free, mirrors PayslipExportWorker pool-backed write); NotificationArgs.NotifKind avoids the River Kind() method collision; List scope=self = recipient_id IN (principal user id, employee id)
- [Phase 11-e10-reporting]: [11-02]: documented dispatch stubs left unwired (no clean single recipient): leave approve-l1, OT confirm/approve-l1/withdraw; mandatory leave/OT/attendance approve-final+reject+verify ARE wired. 11-04 must TRUNCATE notifications in reset-db
- [Phase 11-e10-reporting]: [11-02b]: generic export framework adds a SECOND River worker (report.export) over the ALTER-generalized export_jobs — coexists with PayslipExportWorker; DB RUNNING/DONE mapped to wire PROCESSING/COMPLETED at the DTO so the built FE drives it unchanged
- [Phase 11-e10-reporting]: [11-02b]: dashboard fields without an 11-01 rollup query (attendance_rate_pct/billable_mtd/ot_mtd/trend/leave_balance/today_shift/schedule_alerts) emitted present-but-0/empty/null per openapi REQUIRED (never a fake constant); live counts that DO have queries are real. audit.RecordReturningID added to capture export_jobs.audit_log_entry_id
- [Phase 11-e10-reporting]: [11-03]: E10 drift gate — Go contract tests over the REAL reporting services+handlers via newHarness(role,company,employee) + fake repos + recording fakeJobs + stubIdempotency (copied from the Phase-10 payroll testkit). fakeTx.QueryRow returns a fakeRow scanning a SWP-AL id so audit.RecordReturningID runs honestly in the export tx. Asserts all 8 FE ops + export codes (FORMAT_UNSUPPORTED/TOO_LARGE/RATE_LIMITED) + DB→wire PROCESSING/COMPLETED + outbox (one ReportExportArgs, matching JobID) + RBAC (agent POST /exports 403) + cursor envelopes. go test ./... exits 0, no regressions.
- [Phase 11]: [11-04]: E10 FE needed a DOUBLE {data}-unwrap (query.data.data.data) in dashboard/report/export-flow — Orval wraps the body in {data} AND the BE handler wraps the payload in {data:<T>} though the openapi declares bare; with one unwrap the dashboard showed the agent fallback + report/export rendered empty against the real BE. notifications LIST is single-wrapped (cursor envelope IS the body) so its screen only needed the marked_count fix. 14 e10 Playwright specs green; full e1..e10 = 239 passed / 6 skipped / 0 failed — the v1.0 milestone is closed.

### Pending Todos

None.

### Blockers/Concerns

- Phase 1 depends on Docker being available for the ephemeral Postgres in the E2E harness.

## Session Continuity

Last session: 2026-06-05T09:23:15.352Z
Stopped at: Completed 11-e10-reporting/11-04-PLAN.md
Resume file: None
