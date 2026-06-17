# Manual Test Cases · E2 — Identity, Org & Master Data

> Exhaustive manual QA test cases for E2 (Employee profile + login provisioning, Employment Agreement, Client Company Directory, Client Sites & Geofence, Operational Master Data, Offboarding & Session Revocation), organized per platform (Web console / Mobile) × per POV (super admin · HR/placement admin · shift leader · agent). E2 is predominantly **Web admin CRUD**; mobile surfaces are agent self-service + read-only lookups. Dates are absolute (today = **2026-06-17**, TZ = `Asia/Jakarta`).

**Source specs:** `docs/epics/E2-identity/FEATURE.md`, PRDs F2.1/F2.2/F2.3/F2.5/F2.6/F2.7, `docs/api/CONVENTIONS.md`.

**Conventions used in expected results:**
- Error envelope per CONVENTIONS §11 (`error.code`, `error.message` in Bahasa, `error.fields`).
- `403 FORBIDDEN` for RBAC-role denial; `404` when a resource exists but caller lacks visibility (no-leak); `409` for invariant/conflict; `422` for business-rule/semantic violation; `400` for syntactic/validation.
- Every create/update/deactivate/offboard writes an audit entry (CONVENTIONS §16.1) — verified via `GET /audit-log` (E1).
- Master data is **soft-deactivated, never hard-deleted**.

---

## Coverage matrix

| Feature | Web | Mobile | Super Admin | HR/Placement Admin | Shift Leader | Agent |
|---------|:---:|:------:|:-----------:|:------------------:|:------------:|:-----:|
| **F2.1** Employee & Agent Profile (+ login provisioning) | ✓ | ✓ | ✓ | ✓ | — | ✓ (own profile, self-edit) |
| **F2.2** Employment Agreement (PKWT/PKWTT + comp) | ✓ | ✓ | ✓ | ✓ | — | ✓ (own summary, read, mobile) |
| **F2.3** Client Company Directory | ✓ | — | ✓ | ✓ | ✓ (own company, read, scoped list) | — |
| **F2.6** Client Sites & Geofence | ✓ | — | ✓ | ✓ | ✓ (own site, read via placement) | ✓ (own site, read via placement) |
| **F2.5** Operational Master Data | ✓ | ✓ | ✓ | ✓ | ✓ (read-only labels) | ✓ (read-only labels) |
| **F2.7** Offboarding & Session Revocation | ✓ | ✓ | ✓ | ✓ | — | ✓ (subject; signed out on revoke) |

Legend: ✓ = in scope for that platform/role · — = not served (do not test as a feature surface; RBAC-denial cases for unserved roles are explicitly enumerated where relevant).

---

## F2.1 — Employee & Agent Profile (+ login provisioning)

### Web console · HR/Placement Admin POV

#### TC-E2-F2.1-001 · Create employee auto-provisions a self-service login (happy path)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Verify creating an employee always creates the linked User (role baseline / `agent`) and returns a show-once temp password.
- **Preconditions:** Logged in as HR admin. No employee with NIK `3271010101900001` or phone `+628123456001` exists.
- **Steps:**
  1. Open Tambah Karyawan.
  2. Enter full name "Budi Santoso", NIK `3271010101900001`, join date `2026-06-17`, phone `+628123456001`.
  3. (Optional) leave personal email empty.
  4. Submit.
- **Expected result / Acceptance criteria:** Employee record created with a generated `SWP-EMP-####` id; a linked `User` (`SWP-USR-####`) auto-created at the same time with the baseline (self-service) role; a one-time temp password is shown **once** (show-once panel, copyable, not retrievable after dismissal); audit entry written for both employee + user create.
- **Traceability:** F2.1, EP-1, EP-2, EP-3, EP-4, EP-8, INV-1, C-1(removed→always provisions).

#### TC-E2-F2.1-002 · Reject create with missing required field (full name / NIK / join date)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Minimum-field validation (EP-1).
- **Preconditions:** Logged in as HR admin.
- **Steps:**
  1. Open Tambah Karyawan.
  2. Leave NIK and join date blank; enter only full name.
  3. Submit.
- **Expected result / Acceptance criteria:** Submit blocked; `400 INVALID_REQUEST` with `error.fields.nik` and `error.fields.join_at` populated (Bahasa messages); no employee/user created.
- **Traceability:** F2.1, EP-1.

#### TC-E2-F2.1-003 · Reject duplicate NIK
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** NIK uniqueness (EP-2).
- **Preconditions:** Employee exists with NIK `3271010101900001`.
- **Steps:**
  1. Open Tambah Karyawan.
  2. Enter a new name, the duplicate NIK `3271010101900001`, a fresh phone, join date.
  3. Submit.
- **Expected result / Acceptance criteria:** Creation blocked with a uniqueness error mapped to `error.fields.nik`; no record created; no temp password issued.
- **Traceability:** F2.1, EP-2.

#### TC-E2-F2.1-004 · Reject create with duplicate / missing required phone (login identifier)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Phone required + unique (EP-2, EP-3).
- **Preconditions:** A User already uses phone `+628123456001`.
- **Steps:**
  1. Create employee A with no phone at all → Submit.
  2. Create employee B with phone `+628123456001` (duplicate) → Submit.
- **Expected result / Acceptance criteria:** (1) Blocked — `error.fields.phone` "wajib diisi" (phone is the mandatory login identifier, provisioning not deferrable). (2) Blocked — uniqueness error on `phone`. No record created in either case.
- **Traceability:** F2.1, EP-2, EP-3.

#### TC-E2-F2.1-005 · Reject create when supplied email already used (C-2)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Email unique when present (EP-2).
- **Preconditions:** A User exists with email `sari@example.com`.
- **Steps:**
  1. Create a new employee with a fresh NIK + phone but personal email `sari@example.com`.
  2. Submit.
- **Expected result / Acceptance criteria:** Blocked with uniqueness error on `email`; no record created.
- **Traceability:** F2.1, EP-2, C-2.

#### TC-E2-F2.1-006 · Phone normalization to E.164 on create
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Phone normalized to `+62` / E.164 before storing as login identifier.
- **Preconditions:** Logged in as HR admin; no user on `+628123456777`.
- **Steps:**
  1. Create employee with phone entered as `08123456777` (local format).
  2. Submit; then open the created record.
- **Expected result / Acceptance criteria:** Phone stored normalized as `+628123456777`; uniqueness check applies against the normalized value; login identifier matches normalized form.
- **Traceability:** F2.1, EP-2, EP-5b.

#### TC-E2-F2.1-007 · Edit statutory fields as HR (allowed)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** HR can edit statutory/identity fields (NIK, NIP, name, birth date, NPWP, BPJS, bank) that are read-only to the agent.
- **Preconditions:** Employee `SWP-EMP-1042` exists.
- **Steps:**
  1. Open the employee, Edit.
  2. Update NPWP, BPJS Kesehatan, NIP, birth place.
  3. Save.
- **Expected result / Acceptance criteria:** Changes saved; audit entry with before/after written; no approval flow involved (admin authority direct).
- **Traceability:** F2.1, EP-6, EP-8.

#### TC-E2-F2.1-008 · Reactivate a previously deactivated employee (C-3)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Reactivation re-enables login (with new agreement required — cross to OB-8).
- **Preconditions:** Employee `SWP-EMP-1043` is inactive (previously offboarded).
- **Steps:**
  1. Open the inactive employee.
  2. Choose Aktifkan / Reactivate.
  3. Observe the requirement for a new active agreement before reactivation completes (OB-8).
- **Expected result / Acceptance criteria:** Employee status → active; linked User re-enabled (or re-invited); a **new** active agreement is required (cannot reactivate into the closed one); old sessions are NOT restored; audit entry written.
- **Traceability:** F2.1, EP-7, C-3, F2.7 OB-8.

#### TC-E2-F2.1-009 · Employee list — empty state
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Empty directory renders the designed empty state, not a broken table.
- **Preconditions:** Filter the employee list to a query with zero matches.
- **Steps:**
  1. Open Karyawan list.
  2. Search `q=zzzznonexistent`.
- **Expected result / Acceptance criteria:** Designed empty-state (illustration + "tidak ada karyawan" copy + CTA Tambah Karyawan); no pagination controls; no error.
- **Traceability:** F2.1, CONVENTIONS §8.

#### TC-E2-F2.1-010 · Employee list — loading + cursor pagination
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Skeleton/loading on first paint; cursor pagination works (no offset).
- **Preconditions:** >50 employees exist.
- **Steps:**
  1. Open Karyawan list (observe loading skeleton).
  2. Scroll / click next page.
  3. Inspect the request: it carries `cursor` and `limit`, not `offset`.
- **Expected result / Acceptance criteria:** Loading skeleton shown then replaced; next page uses `next_cursor`; `has_more` drives the "more" affordance; changing sort resets pagination (`CURSOR_MISMATCH` not surfaced to user, refetched).
- **Traceability:** F2.1, CONVENTIONS §8.

#### TC-E2-F2.1-011 · Server error on create surfaces error envelope
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** 500 from create renders a recoverable error, form data retained.
- **Preconditions:** Simulate backend 500 on `POST /employees`.
- **Steps:**
  1. Fill a valid Tambah Karyawan form.
  2. Submit while backend returns `500 INTERNAL`.
- **Expected result / Acceptance criteria:** Error toast/inline with the envelope `message`; entered form values preserved; no partial employee/user created (transactional).
- **Traceability:** F2.1, CONVENTIONS §11, EP-3.

### Web console · Super Admin POV

#### TC-E2-F2.1-012 · Super admin full CRUD parity with HR
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** RBAC
- **Objective:** Super admin can do everything HR can on employees.
- **Preconditions:** Logged in as super admin.
- **Steps:**
  1. Create an employee.
  2. Edit statutory fields.
  3. Deactivate then reactivate.
- **Expected result / Acceptance criteria:** All actions succeed (super admin = global scope); audit entries attribute the super-admin actor.
- **Traceability:** F2.1, EP-1..EP-8, CONVENTIONS §17.

### Web console · Shift Leader POV (RBAC denial)

#### TC-E2-F2.1-013 · Shift leader cannot create or edit employee records
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Shift leader has no employee-authoring capability.
- **Preconditions:** Logged in as a shift leader (derived from an active E3 assignment).
- **Steps:**
  1. Attempt to open Tambah Karyawan (UI affordance should be hidden).
  2. Force the create request directly (`POST /employees`).
- **Expected result / Acceptance criteria:** UI control hidden client-side; direct API call returns `403 FORBIDDEN`; no record created. (Client RBAC is defense-in-depth, server is the gate.)
- **Traceability:** F2.1, CONVENTIONS §17 (roles gate).

### Web console · Agent POV (agent web self-service)

#### TC-E2-F2.1-014 · Agent self-edits the editable set on web console (instant)
- [ ] **Platform:** Web · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Agent web console mirrors mobile self-edit (instant, no approval).
- **Preconditions:** Logged in as an agent (baseline, own record only).
- **Steps:**
  1. Open own profile → Ubah Profil.
  2. Update address + app language.
  3. Save.
- **Expected result / Acceptance criteria:** Changes apply immediately; no "menunggu persetujuan" state exists; each change audited; statutory fields not editable.
- **Traceability:** F2.1, EP-5, EP-6, EP-8, C-6.

### Mobile · Agent POV

#### TC-E2-F2.1-015 · Agent views own profile (read) on mobile
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent sees their own profile.
- **Preconditions:** Agent "Budi" logged in on mobile.
- **Steps:**
  1. Open Profil.
- **Expected result / Acceptance criteria:** Identity, contact, statutory IDs displayed; statutory fields rendered read-only; editable fields show an edit affordance.
- **Traceability:** F2.1 §4, EP-6.

#### TC-E2-F2.1-016 · Agent self-edit several editable fields in one save (instant, each audited) (C-7)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Multi-field instant self-edit applies immediately; each field audited.
- **Preconditions:** Agent "Budi" logged in; no user uses target phone.
- **Steps:**
  1. Open Ubah Profil.
  2. Change photo, address, app language, phone (to a free number), emergency contact (name + phone), and bank account.
  3. Save.
- **Expected result / Acceptance criteria:** All changes apply immediately (no approval queue); each change written as an audit entry; UI confirms success; no pending state shown.
- **Traceability:** F2.1, EP-5, EP-8, C-6, C-7.

#### TC-E2-F2.1-017 · Agent self-edited phone collision rejected inline (EP-5b, C-7)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Negative
- **Objective:** Phone collision rejects only that field; other edits in the same save still apply.
- **Preconditions:** Another user already uses phone `+628120000099`. Agent "Budi" logged in.
- **Steps:**
  1. Open Ubah Profil.
  2. Change address (valid) + phone to `+628120000099` (taken).
  3. Save.
- **Expected result / Acceptance criteria:** Phone field rejected inline with a uniqueness error (`error.fields.phone`); address either applies (if field-independent commit) or the save is blocked surfacing only the phone field error per UI design — confirm against spec behavior; on next sign-in the login identifier is unchanged if phone was rejected.
- **Traceability:** F2.1, EP-2, EP-5b, C-7.

#### TC-E2-F2.1-018 · Changing phone updates login identifier for next sign-in
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Successful phone change re-keys the login.
- **Preconditions:** Agent "Budi" with phone `+628123456001`; target `+628123456002` free.
- **Steps:**
  1. Change phone to `+628123456002`; Save.
  2. Sign out.
  3. Sign in using the new phone.
- **Expected result / Acceptance criteria:** New phone accepted at sign-in; old phone no longer authenticates; change audited.
- **Traceability:** F2.1, EP-5b.

#### TC-E2-F2.1-019 · Agent cannot edit statutory fields (C-4)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Statutory/terms fields are not offered/blocked for agent.
- **Preconditions:** Agent "Budi" logged in.
- **Steps:**
  1. Open Ubah Profil; confirm NIK, name, NPWP, BPJS, placement, contract, compensation are not editable.
  2. Attempt to PATCH a statutory field directly (`PATCH /employees/SWP-EMP-1042` with `nik`).
- **Expected result / Acceptance criteria:** Fields rendered read-only in UI; direct PATCH of a statutory field rejected (`403` or `400` ignoring the field per spec); statutory value unchanged.
- **Traceability:** F2.1, EP-6, C-4.

#### TC-E2-F2.1-020 · Agent self-edit while offline / network error
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Failed save surfaces an error, no silent loss.
- **Preconditions:** Agent logged in; toggle device offline.
- **Steps:**
  1. Open Ubah Profil; change address.
  2. Save while offline.
- **Expected result / Acceptance criteria:** Inline/toast error "tidak ada koneksi"; entered value retained for retry; no audit entry written; profile unchanged server-side.
- **Traceability:** F2.1, CONVENTIONS §11.

#### TC-E2-F2.1-021 · Agent cannot view another agent's profile
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** `self` scope enforced.
- **Preconditions:** Agent "Budi" (`SWP-EMP-1042`) logged in; another employee `SWP-EMP-2000` exists.
- **Steps:**
  1. Attempt `GET /employees/SWP-EMP-2000`.
- **Expected result / Acceptance criteria:** `404 NOT_FOUND` (no-leak) or `403` per `self` scope; no other-employee data returned.
- **Traceability:** F2.1, CONVENTIONS §17 (`scope: self`).

---

## F2.2 — Employment Agreement (PKWT/PKWTT + comp)

### Web console · HR/Placement Admin POV

#### TC-E2-F2.2-001 · Create a PKWT agreement (happy path)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** PKWT created directly active with start+end+agreement no.+comp, comp stored encrypted.
- **Preconditions:** Employee "Budi" has no active agreement. Logged in as HR.
- **Steps:**
  1. Open New Agreement; select employee "Budi"; type = PKWT.
  2. Set start `2026-06-17`, end `2027-06-16`, agreement no. `PKWT/2026/0042`.
  3. Set base salary, BPJS terms, tax profile.
  4. Set annual-leave entitlement = 12.
  5. Click "Activate Agreement".
- **Expected result / Acceptance criteria:** Agreement created with `status = active` (no DRAFT step; create UI offers only Cancel + Activate); compensation stored encrypted (not shown in audit log in cleartext); `annual_leave_entitlement_days = 12`; audit entry (comp masked).
- **Traceability:** F2.2, EA-1, EA-4, EA-10, EA-11, INV-2.

#### TC-E2-F2.2-002 · Create an open-ended PKWTT (no end date)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** PKWTT created with start only.
- **Preconditions:** Employee "Siti" has no active agreement.
- **Steps:**
  1. New Agreement; type = PKWTT; start `2026-06-17`; no end date field shown/required.
  2. Set comp; Activate.
- **Expected result / Acceptance criteria:** Agreement active with `end_date = null`; never enters `expiring`; audit entry written.
- **Traceability:** F2.2, EA-1, OB-4 (PKWTT never expiring).

#### TC-E2-F2.2-003 · Reject a PKWT without end date
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** PKWT requires end date (EA-1).
- **Preconditions:** Employee with no active agreement.
- **Steps:**
  1. New Agreement; type = PKWT; set start only; leave end empty.
  2. Activate.
- **Expected result / Acceptance criteria:** Blocked; `error.fields.end_date` required; no agreement created.
- **Traceability:** F2.2, EA-1.

#### TC-E2-F2.2-004 · Reject end_date before start_date
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Date-range validation (CONVENTIONS §12).
- **Preconditions:** Employee with no active agreement.
- **Steps:**
  1. New PKWT; start `2027-06-16`, end `2026-06-17` (end < start).
  2. Activate.
- **Expected result / Acceptance criteria:** `400 INVALID_REQUEST` with `error.fields.end_date`; no agreement created.
- **Traceability:** F2.2, EA-1, CONVENTIONS §12.

#### TC-E2-F2.2-005 · Reject PKWT period exceeding statutory max (≤5 years)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Cross-field business rule (CONVENTIONS §12, `PKWT_PERIOD_EXCEEDS_MAX`).
- **Preconditions:** Employee with no active agreement.
- **Steps:**
  1. New PKWT; start `2026-06-17`, end `2032-06-17` (>5 years).
  2. Activate.
- **Expected result / Acceptance criteria:** `422` with code `PKWT_PERIOD_EXCEEDS_MAX`; no agreement created.
- **Traceability:** F2.2, EA-1, CONVENTIONS §12.

#### TC-E2-F2.2-006 · Renewal creates a linked successor and supersedes prior
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Renewal links via predecessor_id; prior → superseded (EA-3).
- **Preconditions:** "Budi" has an active PKWT ending `2026-12-31`.
- **Steps:**
  1. Open the agreement; Renew for 2027 (start `2027-01-01`, end `2027-12-31`).
  2. Confirm.
- **Expected result / Acceptance criteria:** New agreement created with `predecessor_id` = old; old agreement `status = superseded`; `annual_leave_entitlement_days` copied onto successor (HR may adjust); audit entries for both.
- **Traceability:** F2.2, EA-3, EA-10, INV-2.

#### TC-E2-F2.2-007 · Renew PKWT as PKWTT (convert to permanent) (C-2)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Successor may be a different type.
- **Preconditions:** "Budi" has an active PKWT.
- **Steps:**
  1. Renew, choosing successor type PKWTT; start only.
  2. Confirm.
- **Expected result / Acceptance criteria:** Successor created as PKWTT open-ended; predecessor superseded; audit entries written.
- **Traceability:** F2.2, EA-3, C-2.

#### TC-E2-F2.2-008 · Block second active agreement without renewal (INV-2)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** At most one active agreement (EA-2 / INV-2).
- **Preconditions:** "Budi" already has an active agreement.
- **Steps:**
  1. Attempt to create a second active agreement for "Budi" outside the renewal flow.
  2. Activate.
- **Expected result / Acceptance criteria:** Blocked — either `409 INV_2_VIOLATION` (or guided into the renewal-supersede flow); never two active agreements coexisting.
- **Traceability:** F2.2, EA-2, INV-2.

#### TC-E2-F2.2-009 · Mid-agreement compensation update is effective-dated & historized (C-3)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Comp updates create a new CompensationRecord, not overwrite.
- **Preconditions:** "Budi" has an active agreement with comp effective `2026-06-17`.
- **Steps:**
  1. Open comp; update base salary effective `2026-07-01`.
  2. Save.
- **Expected result / Acceptance criteria:** A new `CompensationRecord` created with `effective_date = 2026-07-01`; prior record retained; "current comp" = latest as of today; audit entry written with values masked.
- **Traceability:** F2.2, EA-4, EA-7, C-3, data model §6.

#### TC-E2-F2.2-010 · Set annual-leave entitlement on agreement (E6 source)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** `annual_leave_entitlement_days` stored and consumed by E6.
- **Preconditions:** Creating an agreement for "Budi".
- **Steps:**
  1. Set annual-leave entitlement to 12.
  2. Activate.
- **Expected result / Acceptance criteria:** Stored `annual_leave_entitlement_days = 12`; null leaves org default; value `>= 0` enforced (negative rejected).
- **Traceability:** F2.2, EA-10.

#### TC-E2-F2.2-011 · Reject negative annual-leave entitlement
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Negative
- **Objective:** `annual_leave_entitlement_days >= 0`.
- **Preconditions:** Creating an agreement.
- **Steps:**
  1. Set annual-leave entitlement = -1; Activate.
- **Expected result / Acceptance criteria:** `400` with `error.fields.annual_leave_entitlement_days`; not created.
- **Traceability:** F2.2, EA-10, data model §6.

#### TC-E2-F2.2-012 · No "Save as Draft" and no file-upload in create form (MVP scope)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Confirm MVP scope cuts: no DRAFT, no "Berkas Perjanjian" upload.
- **Preconditions:** Open the agreement-create form.
- **Steps:**
  1. Inspect available actions.
  2. Inspect for any file-upload control.
- **Expected result / Acceptance criteria:** Only Cancel + "Activate Agreement" present (no draft save); no document/attachment upload field; agreement created without an attached PDF.
- **Traceability:** F2.2, EA-11, EA non-goals §2.

#### TC-E2-F2.2-013 · Agreements list — minimal columns, search, filters
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** List shows joined employee name + id; searchable by name/id/agreement number; only type + status filters.
- **Preconditions:** Several agreements exist.
- **Steps:**
  1. Open Agreements list.
  2. Search `q=Budi`, then `q=SWP-EMP-1042`, then `q=PKWT/2026/0042`.
  3. Apply type=PKWT and status=active filters.
  4. Inspect columns.
- **Expected result / Acceptance criteria:** Each `q` returns the right agreement(s); type + status filters work; no "Pengganti"/successor column, no per-row kebab, no filter "Reset" button.
- **Traceability:** F2.2, EA-12.

#### TC-E2-F2.2-014 · Compensation hidden/masked in audit log
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Comp changes audited with values masked (EA-7).
- **Preconditions:** A comp update was made (TC-E2-F2.2-009).
- **Steps:**
  1. Open Audit Log; find the comp-update entry.
- **Expected result / Acceptance criteria:** Entry shows the change occurred (old/new flagged) but **salary/BPJS values are masked**, not cleartext.
- **Traceability:** F2.2, EA-4, EA-7, CONVENTIONS §16.1.

#### TC-E2-F2.2-015 · Agreements list — empty + loading + error
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Non-happy list states.
- **Preconditions:** Zero-match filter; then simulate a backend 500.
- **Steps:**
  1. Filter to zero results.
  2. Reload with backend forced to `500`.
- **Expected result / Acceptance criteria:** (1) Designed empty state. (2) Loading skeleton then error state with retry, envelope message; no crash.
- **Traceability:** F2.2, CONVENTIONS §11/§8.

### Web console · Super Admin POV

#### TC-E2-F2.2-016 · Super admin can view encrypted compensation (role-gated)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** RBAC
- **Objective:** Comp visible only to authorized roles (EA-4).
- **Preconditions:** Agreement with comp exists.
- **Steps:**
  1. As super admin open the agreement comp section.
- **Expected result / Acceptance criteria:** Comp decrypted and shown to authorized role; access itself audited.
- **Traceability:** F2.2, EA-4, CONVENTIONS §17.

### Mobile · Agent POV

#### TC-E2-F2.2-017 · Agent views own agreement summary, comp hidden
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent sees type/period/status, never salary/BPJS amounts.
- **Preconditions:** Agent "Budi" has an active PKWT.
- **Steps:**
  1. Open Kontrak/Agreement summary on mobile.
- **Expected result / Acceptance criteria:** Type (PKWT), period, status shown; base salary and BPJS amounts NOT shown.
- **Traceability:** F2.2, EA-4, §4, AC "Agent cannot see compensation".

#### TC-E2-F2.2-018 · Agent cannot fetch comp via API
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** RBAC
- **Objective:** Comp endpoint denied for agent baseline.
- **Preconditions:** Agent "Budi" logged in.
- **Steps:**
  1. Attempt to fetch the comp/CompensationRecord for own agreement directly.
- **Expected result / Acceptance criteria:** `403 FORBIDDEN`; no comp values returned even for own record.
- **Traceability:** F2.2, EA-4, CONVENTIONS §17.

---

## F2.3 — Client Company Directory

### Web console · HR/Placement Admin POV

#### TC-E2-F2.3-001 · Create a client company auto-creates a primary Main Site
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Create company → active + auto primary "Main Site" → placeable (CC-1c / ST-3).
- **Preconditions:** No company named "Plaza Group". Logged in as HR.
- **Steps:**
  1. Create company "Plaza Group" with registered address; optional NPWP/PIC/phone; leave leader_scope default.
  2. Save.
  3. Open the company → "Lokasi & Site" tab.
- **Expected result / Acceptance criteria:** Company saved `status = active` (`SWP-CMP-####`); `leader_scope = company` by default; a primary "Main Site" (`SWP-SITE-####`, `is_primary = true`, geofence empty) auto-created and visible under Lokasi & Site; company immediately placeable via that site; audit entries for company + site.
- **Traceability:** F2.3, CC-1, CC-1b, CC-1c, INV-5, F2.6 ST-3.

#### TC-E2-F2.3-002 · Reject create with missing required fields (name / registered address)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Required fields (CC-1).
- **Preconditions:** Logged in as HR.
- **Steps:**
  1. Create company leaving name and address empty; Save.
- **Expected result / Acceptance criteria:** Blocked; `error.fields.name` + `error.fields.address`; not created.
- **Traceability:** F2.3, CC-1.

#### TC-E2-F2.3-003 · Reject duplicate company name
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Name unique (CC-2).
- **Preconditions:** "Plaza Senayan" company exists.
- **Steps:**
  1. Create another company named "Plaza Senayan"; Save.
- **Expected result / Acceptance criteria:** Blocked with uniqueness error on `name`; not created.
- **Traceability:** F2.3, CC-2.

#### TC-E2-F2.3-004 · Reject duplicate NPWP when provided
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** NPWP unique when present (CC-2).
- **Preconditions:** A company already uses NPWP `01.234.567.8-901.000`.
- **Steps:**
  1. Create a differently-named company with the duplicate NPWP; Save.
- **Expected result / Acceptance criteria:** Blocked; uniqueness on `npwp`; not created. (A null/blank NPWP does not trip uniqueness.)
- **Traceability:** F2.3, CC-2.

#### TC-E2-F2.3-005 · Set leader_scope = site on create
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** leader_scope drives per-site leadership (CC-1b / ST-9).
- **Preconditions:** Logged in as HR.
- **Steps:**
  1. Create a multi-site client; set leader_scope = site.
  2. Save.
- **Expected result / Acceptance criteria:** `leader_scope = site` stored; consumed by E3 F3.4 for per-site leader designation.
- **Traceability:** F2.3, CC-1b, F2.6 ST-9.

#### TC-E2-F2.3-006 · Edit company via full-page screen from detail (not drawer)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Edit lives at `/client-companies/$id/edit` (full page), no drawer.
- **Preconditions:** Company "Plaza Group" exists.
- **Steps:**
  1. Open company detail → Profil tab → Edit.
  2. Confirm a full-page edit screen (route `/client-companies/$id/edit`), not a drawer.
  3. Change PIC + phone; Save.
- **Expected result / Acceptance criteria:** Edit opens as a full page; changes saved; audit entry; no `EditClientCompanyDrawer`.
- **Traceability:** F2.3, §4 UI/flow, Decisions 2026-06-07.

#### TC-E2-F2.3-007 · Detail "Profil" tab shows statutory/billing + leader_scope only (no Sites)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Profil tab must not duplicate Sites/geofence.
- **Preconditions:** Company with ≥1 site.
- **Steps:**
  1. Open company detail → Profil tab.
- **Expected result / Acceptance criteria:** Profil shows name, registered address, NPWP, PIC, phone, leader_scope only; Sites & geofence appear only on the "Lokasi & Site" tab.
- **Traceability:** F2.3, §4 UI/flow, F2.6.

#### TC-E2-F2.3-008 · List's only row action is Aktifkan/Nonaktifkan (no kebab)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** List row action constraint.
- **Preconditions:** Company list has rows.
- **Steps:**
  1. Open the directory list.
  2. Inspect row actions.
- **Expected result / Acceptance criteria:** Each row offers only Aktifkan/Nonaktifkan; no per-row kebab; create/edit live elsewhere (detail/full-page).
- **Traceability:** F2.3, §4 UI/flow.

#### TC-E2-F2.3-009 · Deactivate company with no active placements (happy)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Soft-deactivate a clean company.
- **Preconditions:** "Old Tower" company has no active placements.
- **Steps:**
  1. From list, Nonaktifkan "Old Tower"; confirm.
- **Expected result / Acceptance criteria:** Status → Inactive (soft, not hard-deleted); company no longer offered as a new-placement target (E3 BR-3); audit entry.
- **Traceability:** F2.3, CC-3, CC-4, CC-6.

#### TC-E2-F2.3-010 · Deactivate company with active placements is blocked/warned (CC-5)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Guard against orphaning active placements.
- **Preconditions:** "Plaza Senayan" company has active placements.
- **Steps:**
  1. Attempt Nonaktifkan "Plaza Senayan".
- **Expected result / Acceptance criteria:** Action warns/blocks with guidance to end/transfer those placements first (warn-and-guide / block-until-resolved); company stays active until resolved.
- **Traceability:** F2.3, CC-5.

#### TC-E2-F2.3-011 · Reactivate an inactive company (C-2)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Reactivation makes it a valid placement target again.
- **Preconditions:** "Old Tower" is Inactive.
- **Steps:**
  1. Aktifkan "Old Tower"; confirm.
- **Expected result / Acceptance criteria:** Status → Active; selectable as a placement target again; audit entry.
- **Traceability:** F2.3, CC-3, C-2.

#### TC-E2-F2.3-012 · Hard-delete forbidden when referenced (CC-4)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** No hard delete of a referenced company.
- **Preconditions:** Company referenced by ≥1 placement (active or historical).
- **Steps:**
  1. Attempt a hard `DELETE /client-companies/SWP-CMP-12`.
- **Expected result / Acceptance criteria:** Hard-delete unavailable in UI; direct DELETE soft-deactivates or is rejected — never removes the row; references stay intact.
- **Traceability:** F2.3, CC-4, CONVENTIONS §6.

#### TC-E2-F2.3-013 · Detail page E3-backed tabs render (Penempatan Aktif / Pemimpin Shift / Riwayat)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Detail page surfaces the three E3-backed tabs reading the company roster.
- **Preconditions:** Company with active placements + a current shift leader.
- **Steps:**
  1. Open company detail.
  2. Visit Penempatan Aktif, Pemimpin Shift, Riwayat tabs.
- **Expected result / Acceptance criteria:** Penempatan Aktif shows active roster; Pemimpin Shift shows the current leader with assign/replace/revoke (single entry point to E3 F3.4); Riwayat shows historical placements (`include_history`). (Leader mutations hit E3 endpoints — out of E2 scope to assert beyond presence.)
- **Traceability:** F2.3, §4 detail tabs, Decisions 2026-06-08.

#### TC-E2-F2.3-014 · Company without geo allowed; geofencing disabled until geo added (C-1)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Geo is on Site and optional.
- **Preconditions:** New company just created (Main Site geo empty).
- **Steps:**
  1. View the auto Main Site's geofence state.
- **Expected result / Acceptance criteria:** Site allowed without geo; E5 geofencing for that site flagged disabled until geo added.
- **Traceability:** F2.3, C-1, F2.6 ST-8.

#### TC-E2-F2.3-015 · Directory list empty / loading / error states
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Non-happy list states.
- **Preconditions:** Zero-match filter; then simulated 500.
- **Steps:**
  1. Search a query with no matches.
  2. Reload with backend `500`.
- **Expected result / Acceptance criteria:** Empty state for (1); loading skeleton then error+retry for (2); no crash.
- **Traceability:** F2.3, CONVENTIONS §11/§8.

### Web console · Super Admin POV

#### TC-E2-F2.3-016 · Super admin sees all companies (global scope)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** RBAC
- **Objective:** Global scope for list (CC-7).
- **Preconditions:** Multiple companies exist.
- **Steps:**
  1. Open the directory list as super admin.
- **Expected result / Acceptance criteria:** All companies visible regardless of any leader assignment.
- **Traceability:** F2.3, CC-7, CONVENTIONS §17.

### Web console · Shift Leader POV (scoped read)

#### TC-E2-F2.3-017 · Shift leader sees only their own company in the directory list (CC-7)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** List is role-scoped server-side; leader sees one company.
- **Preconditions:** Logged in as a shift leader whose active E3 assignment is company "Plaza Senayan"; other companies exist.
- **Steps:**
  1. Open `GET /client-companies` (directory list).
- **Expected result / Acceptance criteria:** Only "Plaza Senayan" returned; other companies absent; scope derived from the active assignment (server-side), not stored columns.
- **Traceability:** F2.3, CC-7, F3.4 SL-10, CONVENTIONS §17 (derived scope).

#### TC-E2-F2.3-018 · Shift leader cannot create / edit / deactivate companies
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** No authoring capability for shift leader.
- **Preconditions:** Logged in as shift leader of "Plaza Senayan".
- **Steps:**
  1. Attempt `POST /client-companies`.
  2. Attempt `PATCH` on own company.
  3. Attempt to deactivate own company.
- **Expected result / Acceptance criteria:** All return `403 FORBIDDEN` (or `OUT_OF_SCOPE` where applicable); read-only access only.
- **Traceability:** F2.3, CC-7, CONVENTIONS §17.

#### TC-E2-F2.3-019 · Shift leader cannot access a company they don't lead (out-of-scope)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Company scope enforced on detail too.
- **Preconditions:** Shift leader of "Plaza Senayan"; company "Grand Indonesia" exists.
- **Steps:**
  1. Attempt `GET /client-companies/SWP-CMP-99` (Grand Indonesia).
- **Expected result / Acceptance criteria:** `404 NOT_FOUND` (no-leak) or `403 OUT_OF_SCOPE`; no data returned.
- **Traceability:** F2.3, CC-7, CONVENTIONS §17.

---

## F2.6 — Client Sites & Geofence

### Web console · HR/Placement Admin POV

#### TC-E2-F2.6-001 · Add a second site with a valid geofence (happy path)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Add a non-primary site, set geofence center + radius, becomes placeable.
- **Preconditions:** Company "Plaza Group" exists (has Main Site).
- **Steps:**
  1. Company detail → Lokasi & Site → Tambah Site.
  2. Name "Plaza Senayan", address; drop map pin (lat `-6.225`, lng `106.799`); radius 100m.
  3. Save.
- **Expected result / Acceptance criteria:** Site saved Active with an active geofence (`SWP-SITE-####`); selectable as a placement target; audit entry.
- **Traceability:** F2.6, ST-1, ST-4, ST-7, US-1, US-2.

#### TC-E2-F2.6-002 · Reject duplicate site name within the same company (ST-2 / C-6)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** `(client_company_id, name)` unique.
- **Preconditions:** "Plaza Group" already has site "Plaza Senayan".
- **Steps:**
  1. Add another site named "Plaza Senayan" to "Plaza Group"; Save.
- **Expected result / Acceptance criteria:** Blocked with uniqueness error on `name`; not created. (Concurrent duplicate adds: the second commit fails on the unique constraint — C-6.)
- **Traceability:** F2.6, ST-2, INV-5, C-6.

#### TC-E2-F2.6-003 · Same site name allowed across different companies
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Name uniqueness is per-company, not global.
- **Preconditions:** "Plaza Group" has "Plaza Senayan"; company "Other Group" exists.
- **Steps:**
  1. Add "Plaza Senayan" to "Other Group"; Save.
- **Expected result / Acceptance criteria:** Saved successfully (no cross-company collision).
- **Traceability:** F2.6, ST-2.

#### TC-E2-F2.6-004 · Reject site create missing required parent/name/address (ST-1)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Required fields.
- **Preconditions:** On Tambah Site form.
- **Steps:**
  1. Leave name + address empty; Save.
- **Expected result / Acceptance criteria:** Blocked with `error.fields.name` + `error.fields.address`; not created. Parent company is implied by context.
- **Traceability:** F2.6, ST-1.

#### TC-E2-F2.6-005 · Site without geo allowed; flagged disabled for E5 (ST-8 / C-1)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Geo optional (mobile/remote placement).
- **Preconditions:** "Plaza Group" exists.
- **Steps:**
  1. Add site "Annex" with name + address; do NOT set lat/lng.
  2. Save.
- **Expected result / Acceptance criteria:** Site saved; geofence shown as disabled/flagged ("geofence belum diatur"); E5 will skip the location check until geo added.
- **Traceability:** F2.6, ST-8, C-1, INV-5 (geofence optional).

#### TC-E2-F2.6-006 · Geofence radius default = 100m when unset
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** `geofence_radius_m` defaults to 100.
- **Preconditions:** On Tambah Site form.
- **Steps:**
  1. Set lat/lng; leave radius field at default / blank.
  2. Save; reopen.
- **Expected result / Acceptance criteria:** Radius persisted as 100m.
- **Traceability:** F2.6, ST-1, ST-7.

#### TC-E2-F2.6-007 · Reject invalid geofence coordinates (lat/lng out of range)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Coordinate validation — lat ∈ [-90,90], lng ∈ [-180,180].
- **Preconditions:** On Tambah/Edit Site.
- **Steps:**
  1. Enter lat `100.5`, lng `200.0` (both out of range); Save.
  2. Then try lat set but lng blank (only one coordinate).
- **Expected result / Acceptance criteria:** (1) `400 INVALID_REQUEST` with field errors on lat/lng; not saved. (2) Reject partial coordinate pair — both lat and lng must be set together to enable a geofence.
- **Traceability:** F2.6, ST-7, CONVENTIONS §12.

#### TC-E2-F2.6-008 · Reject non-positive / absurd geofence radius
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Radius must be a sensible positive integer (meters).
- **Preconditions:** On Edit Site with geo set.
- **Steps:**
  1. Set radius = 0; Save.
  2. Set radius = -50; Save.
  3. Set radius = 5000000 (absurd); Save.
- **Expected result / Acceptance criteria:** 0 and negative rejected (`error.fields.geofence_radius_m`); an absurd-large value rejected or capped per spec max; valid range enforced.
- **Traceability:** F2.6, ST-7, CONVENTIONS §12.

#### TC-E2-F2.6-009 · Move the primary flag, never empty (ST-3)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Exactly one primary; flag moves, never empties.
- **Preconditions:** "Plaza Group" has "Main Site" (primary) + "Plaza Senayan".
- **Steps:**
  1. Mark "Plaza Senayan" as primary; confirm.
- **Expected result / Acceptance criteria:** "Plaza Senayan" becomes primary; "Main Site" loses primary; the company still has exactly one primary site (partial-unique constraint holds); audit entry.
- **Traceability:** F2.6, ST-3, INV-5, US-3.

#### TC-E2-F2.6-010 · Cannot clear primary leaving none
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** Primary can never be left empty.
- **Preconditions:** "Plaza Group" with one primary site.
- **Steps:**
  1. Attempt to un-set primary without choosing another site.
- **Expected result / Acceptance criteria:** Blocked — must move the primary to another site; never zero primaries.
- **Traceability:** F2.6, ST-3, INV-5.

#### TC-E2-F2.6-011 · Deactivate a site with active placements blocked/warned (ST-6)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Guard active placements.
- **Preconditions:** "Plaza Senayan" site has active placements.
- **Steps:**
  1. Attempt to deactivate "Plaza Senayan".
- **Expected result / Acceptance criteria:** Warned/blocked to end/transfer the placements first; site stays Active until resolved.
- **Traceability:** F2.6, ST-6.

#### TC-E2-F2.6-012 · Cannot deactivate the company's last active site (ST-6 / C-2)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Company must keep ≥1 active site while active.
- **Preconditions:** Company "Mall Kelapa Gading" has only one active site (its primary Main Site), no active placements.
- **Steps:**
  1. Attempt to deactivate that only/primary site.
- **Expected result / Acceptance criteria:** Blocked while the company is active; guidance: deactivate the company instead (F2.3).
- **Traceability:** F2.6, ST-3, ST-6, C-2, INV-5.

#### TC-E2-F2.6-013 · Reassign primary then deactivate the old site (C-5)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Old primary can be deactivated once non-primary + no active placements.
- **Preconditions:** "Plaza Group" has "Main Site" (primary, no active placements) + "Plaza Senayan".
- **Steps:**
  1. Make "Plaza Senayan" primary.
  2. Deactivate "Main Site".
- **Expected result / Acceptance criteria:** Allowed — Main Site deactivated since primary already moved and it has no active placements; company keeps ≥1 active site.
- **Traceability:** F2.6, ST-3, ST-6, C-5.

#### TC-E2-F2.6-014 · Site hard-delete forbidden when referenced (ST-5)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** No hard delete of a referenced site.
- **Preconditions:** Site referenced by a placement (active or historical).
- **Steps:**
  1. Attempt hard `DELETE` of the site.
- **Expected result / Acceptance criteria:** Hard-delete unavailable; only deactivation; reference preserved.
- **Traceability:** F2.6, ST-5, CONVENTIONS §6.

#### TC-E2-F2.6-015 · Switch leader_scope company→site flags existing leader for re-designation (C-3)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Changing the company's leadership unit flags the lone company-level leader.
- **Preconditions:** Company has `leader_scope = company` and one company-level shift leader.
- **Steps:**
  1. Edit company; change leader_scope to `site`; Save.
- **Expected result / Acceptance criteria:** Existing company-level assignment flagged for re-designation; HR prompted to name a leader per active site (handed to E3 F3.4); audit entry.
- **Traceability:** F2.6, ST-9, C-3, F2.3 CC-1b.

#### TC-E2-F2.6-016 · Edit geofence audited (ST-10)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Geofence/primary-change actions audited.
- **Preconditions:** Site with geo set.
- **Steps:**
  1. Change radius from 100 to 150; Save.
  2. Open audit log.
- **Expected result / Acceptance criteria:** Audit entry with before/after radius; actor recorded.
- **Traceability:** F2.6, ST-10, CONVENTIONS §16.1.

#### TC-E2-F2.6-017 · Sites tab empty / loading / error
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Non-happy site-list states (note: a company always has ≥1 site, so "empty" applies to filtered views/errors).
- **Preconditions:** Simulate `500` on the sites fetch.
- **Steps:**
  1. Open Lokasi & Site with backend forced to `500`.
- **Expected result / Acceptance criteria:** Loading skeleton then error+retry; no crash; the guaranteed primary site is never silently dropped.
- **Traceability:** F2.6, CONVENTIONS §11.

### Web console · Super Admin POV

#### TC-E2-F2.6-018 · Super admin full site CRUD parity
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** RBAC
- **Objective:** Super admin can author sites/geofence everywhere.
- **Preconditions:** Logged in as super admin.
- **Steps:**
  1. Add a site, set geofence, move primary, deactivate a non-primary site.
- **Expected result / Acceptance criteria:** All succeed (global scope); audited.
- **Traceability:** F2.6, ST-1..ST-10, CONVENTIONS §17.

### Web console · Shift Leader POV (RBAC denial)

#### TC-E2-F2.6-019 · Shift leader cannot author sites/geofence
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** No site-authoring for shift leader.
- **Preconditions:** Shift leader of "Plaza Senayan".
- **Steps:**
  1. Attempt `POST /client-companies/SWP-CMP-12/sites` and a geofence `PATCH`.
- **Expected result / Acceptance criteria:** `403 FORBIDDEN`; no site created/changed.
- **Traceability:** F2.6, §4, CONVENTIONS §17.

### Mobile · Agent / Shift Leader POV (read via placement)

#### TC-E2-F2.6-020 · Agent sees the site (name + address) they're placed at
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Read-only site surfaced via placement/attendance, not a directory (US-5).
- **Preconditions:** Agent "Budi" placed at site "Plaza Senayan".
- **Steps:**
  1. Open the placement/attendance context on mobile.
- **Expected result / Acceptance criteria:** Site name + address shown so the agent knows where to clock in; no site directory/CRUD exposed.
- **Traceability:** F2.6, §4, US-5.

#### TC-E2-F2.6-021 · Agent does not see other sites / no site directory
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** RBAC
- **Objective:** No directory browsing on mobile.
- **Preconditions:** Agent "Budi" placed at one site.
- **Steps:**
  1. Confirm there is no "all sites" listing accessible.
- **Expected result / Acceptance criteria:** Only the placed-at site is visible; no enumeration of other companies' sites.
- **Traceability:** F2.6, §4.

---

## F2.5 — Operational Master Data (leave / attendance / overtime)

### Web console · Super Admin POV

#### TC-E2-F2.5-001 · Create a PER_EVENT leave type requiring a document (CKA)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Create a per-occurrence statutory leave type with cap + doc gate.
- **Preconditions:** Logged in as super admin; code `CKA` not yet present (or test in a fresh env).
- **Steps:**
  1. Master Data → Leave Types → Tambah.
  2. code `CKA`, name "Khitanan / Baptisan anak", category LIFE_EVENT, cap_basis `PER_EVENT`, cap_value 2, cap_unit DAYS, paid=true, gender ANY, requires_document=true.
  3. Save.
- **Expected result / Acceptance criteria:** Leave type created; E6 will cap requests at 2 days per occurrence and force a document upload; audit entry.
- **Traceability:** F2.5, LT-1, LT-2, LT-3, AC "per-occurrence statutory".

#### TC-E2-F2.5-002 · Reject duplicate leave-type code
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** `code` unique (LT-1 / MD-2).
- **Preconditions:** Leave type `CT` exists.
- **Steps:**
  1. Create another leave type with code `CT`; Save.
- **Expected result / Acceptance criteria:** Blocked with uniqueness error on `code`; not created.
- **Traceability:** F2.5, LT-1, MD-2.

#### TC-E2-F2.5-003 · Reject duplicate leave-type name
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** `name` unique within the list (MD-2).
- **Preconditions:** Leave type named "Cuti Haid" exists.
- **Steps:**
  1. Create another leave type with name "Cuti Haid" (different code); Save.
- **Expected result / Acceptance criteria:** Blocked with uniqueness error on `name`.
- **Traceability:** F2.5, MD-2, data model §6.

#### TC-E2-F2.5-004 · Create each cap_basis variant correctly stores mechanics
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** All 7 cap_basis values accepted with consistent cap_value/cap_unit semantics.
- **Preconditions:** Logged in as super admin.
- **Steps:**
  1. Create a type each for `ANNUAL_POOL` (CT, 12 DAYS), `PER_EVENT` (CKM, 2 DAYS), `PER_MONTH` (CH, 2 DAYS), `PER_YEAR_COUNT` (STSD, 5 COUNT), `UNCAPPED` (SDSKD, blank cap), `LIFETIME_ONCE` (CM, 3 DAYS), `SERVICE_UNPAID` (CLTP, 365 DAYS, paid=false, min_service_years=5).
- **Expected result / Acceptance criteria:** Each persists its `cap_basis`, `cap_value`, `cap_unit`; UNCAPPED accepts a blank cap_value; SERVICE_UNPAID stores paid=false + min_service_years=5; ANNUAL_POOL annual entitlement is sourced from E2 agreement (not cap_value at request time, per LT-4).
- **Traceability:** F2.5, LT-1, LT-2, LT-4, §5a.

#### TC-E2-F2.5-005 · Gender-restricted leave type stored (CH female-only, CIM male-only)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** `gender` gate stored; enforced at request-time by E6.
- **Preconditions:** Logged in as super admin.
- **Steps:**
  1. Create "Cuti Haid" (CH) gender=FEMALE.
  2. Create "Istri melahirkan" (CIM) gender=MALE.
- **Expected result / Acceptance criteria:** Both stored with the respective gender gate; E6 will enforce eligibility at request time.
- **Traceability:** F2.5, LT-1, LT-3.

#### TC-E2-F2.5-006 · Religious leave with notice_days + lead/trail days (CIH)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** notice_days, lead_days, trail_days stored.
- **Preconditions:** Logged in as super admin.
- **Steps:**
  1. Create "Cuti Ibadah Haji" (CIH) LIFETIME_ONCE, notice_days=30, lead_days=5, trail_days=5, requires_document=true.
- **Expected result / Acceptance criteria:** All fields persisted; E6 enforces ≥30d notice; +5 paid days before and after the event window.
- **Traceability:** F2.5, LT-1, §5a.

#### TC-E2-F2.5-007 · Create a billable attendance code needing verification
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Attendance code with is_billable + needs_verification.
- **Preconditions:** Logged in as super admin.
- **Steps:**
  1. Master Data → Attendance Codes → Tambah.
  2. name "Overtime Present", is_workday=true, is_payable=true, is_billable=true, needs_verification=true, pick a color.
  3. Save.
- **Expected result / Acceptance criteria:** Code created; E5 will flag attendance under it as billable and require shift-leader verification; audit entry.
- **Traceability:** F2.5, AC-1, AC-2, AC-3.

#### TC-E2-F2.5-008 · Reject duplicate attendance-code name
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Attendance code `name` unique (MD-2).
- **Preconditions:** Code "Present" exists.
- **Steps:**
  1. Create another code "Present"; Save.
- **Expected result / Acceptance criteria:** Blocked with uniqueness error on `name`.
- **Traceability:** F2.5, MD-2, AC AC "Unique names".

#### TC-E2-F2.5-009 · Color clash between attendance codes allowed (C-5)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Same color is allowed (cosmetic), optional warning.
- **Preconditions:** Code "Present" uses teal.
- **Steps:**
  1. Create code "Half Day" with the same teal color; Save.
- **Expected result / Acceptance criteria:** Saved successfully (color is not unique); an optional non-blocking warning may appear.
- **Traceability:** F2.5, C-5.

#### TC-E2-F2.5-010 · Create a global overtime rule
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** OT rule (multiplier, min_minutes, requires_preapproval), global only.
- **Preconditions:** Logged in as super admin.
- **Steps:**
  1. Master Data → Overtime Rules → Tambah.
  2. name "Night OT", multiplier 2.0, min_minutes 60, requires_preapproval=true.
  3. Save.
- **Expected result / Acceptance criteria:** Rule created and applies globally (no service-line / scope axis offered anywhere in the form); audit entry.
- **Traceability:** F2.5, OR-1, OR-2.

#### TC-E2-F2.5-011 · Reject duplicate overtime-rule name
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** OT rule `name` unique (MD-2).
- **Preconditions:** Rule "Night OT" exists.
- **Steps:**
  1. Create another rule "Night OT"; Save.
- **Expected result / Acceptance criteria:** Blocked with uniqueness on `name`.
- **Traceability:** F2.5, MD-2.

#### TC-E2-F2.5-012 · Reject invalid OT multiplier / min_minutes
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** Negative
- **Objective:** Sensible numeric validation.
- **Preconditions:** On Tambah Overtime Rule.
- **Steps:**
  1. multiplier = 0 or negative; Save.
  2. min_minutes = -10; Save.
- **Expected result / Acceptance criteria:** Rejected with field errors; not created. (Exact bounds confirmed against E7; multiplier must be positive, min_minutes ≥ 0.)
- **Traceability:** F2.5, OR-1, OR-3, CONVENTIONS §12.

#### TC-E2-F2.5-013 · Cannot delete a referenced leave type — only deactivate (MD-1)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Soft-deactivate referenced master data.
- **Preconditions:** Leave requests reference "Cuti Tahunan" (CT).
- **Steps:**
  1. Attempt to delete CT.
- **Expected result / Acceptance criteria:** Hard delete blocked; only Nonaktifkan offered; type deactivated (soft), references intact; audit entry.
- **Traceability:** F2.5, MD-1, AC "Cannot delete referenced".

#### TC-E2-F2.5-014 · Deactivate a leave type with open requests (C-1)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** New requests can't use it; in-flight complete.
- **Preconditions:** Leave type "CKA" has in-flight requests (E6).
- **Steps:**
  1. Deactivate "CKA".
- **Expected result / Acceptance criteria:** Type → Inactive; new leave requests cannot select it; existing in-flight requests still complete; audit entry.
- **Traceability:** F2.5, C-1, MD-1.

#### TC-E2-F2.5-015 · Master-data lists empty / loading / error states
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P2 · **Type:** Empty/Loading/Error
- **Objective:** Non-happy states for each of the three lists.
- **Preconditions:** Fresh/empty list; then simulate `500`.
- **Steps:**
  1. Open an empty master list.
  2. Reload with backend `500`.
- **Expected result / Acceptance criteria:** Designed empty state with Tambah CTA; loading skeleton then error+retry; no crash.
- **Traceability:** F2.5, CONVENTIONS §11/§8.

### Web console · HR/Placement Admin POV

#### TC-E2-F2.5-016 · HR can add/edit master data
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** RBAC/Happy
- **Objective:** HR has master-data authoring (per §4 "Super Admin / HR").
- **Preconditions:** Logged in as HR.
- **Steps:**
  1. Add a leave type, an attendance code, and an overtime rule.
  2. Edit one of each.
- **Expected result / Acceptance criteria:** All succeed; audit entries attribute the HR actor.
- **Traceability:** F2.5, §4, LT/AC/OR rules.

### Web console · Shift Leader POV (RBAC denial)

#### TC-E2-F2.5-017 · Shift leader cannot author master data
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P1 · **Type:** RBAC
- **Objective:** Master-data authoring denied (consume read-only only).
- **Preconditions:** Logged in as shift leader.
- **Steps:**
  1. Attempt `POST /master-data/leave-types` (and similarly for attendance codes / overtime rules).
- **Expected result / Acceptance criteria:** `403 FORBIDDEN`; no record created; read endpoints (labels) remain accessible.
- **Traceability:** F2.5, §4, CONVENTIONS §17.

### Mobile · Agent / Shift Leader POV (read-only labels)

#### TC-E2-F2.5-018 · Leave types appear as selectable options on mobile leave request
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Happy
- **Objective:** Active leave types surface as labels/options (read-only) when requesting leave.
- **Preconditions:** Agent on mobile; active leave types seeded; an inactive type also exists.
- **Steps:**
  1. Start a leave request; open the type picker.
- **Expected result / Acceptance criteria:** Only Active leave types selectable; each shows name + color; gender-ineligible / service-ineligible types are filtered or gated per E6; deactivated types absent.
- **Traceability:** F2.5, §4, MD-1, LT-3.

#### TC-E2-F2.5-019 · Attendance status colors/labels render read-only on mobile
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P2 · **Type:** Happy
- **Objective:** Attendance codes render as status labels (read-only).
- **Preconditions:** Agent on mobile with attendance history.
- **Steps:**
  1. View attendance history.
- **Expected result / Acceptance criteria:** Status labels render using the code name + color (via StatusBadge); no edit affordance on the master data.
- **Traceability:** F2.5, §4, AC-1.

---

## F2.7 — Employee Offboarding & Session Revocation

### Web console · HR/Placement Admin POV

#### TC-E2-F2.7-001 · Terminate for cause revokes the session immediately (happy/atomic)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Offboard atomically closes agreement, deactivates employee, ends placement, disables user, revokes sessions; next request 401.
- **Preconditions:** Agent "Budi" has an active agreement, an open placement, and a working login/session.
- **Steps:**
  1. Open Budi → Berhentikan / Offboard.
  2. Reason = TERMINATED + documented cause (free text), effective date = today (`2026-06-17`).
  3. Confirm.
  4. From Budi's authenticated mobile session, make any request after submit.
- **Expected result / Acceptance criteria:** In one transaction: agreement `status = closed`, `closed_reason = TERMINATED`, `closed_at = 2026-06-17`; `Employee.status = inactive`; non-terminal placement → terminal (TERMINATED); linked User disabled, `tokens_valid_after` bumped, refresh tokens revoked; Budi's next authenticated request returns `401`. Audit: parent `employee.offboard` entry lists cascaded agreement id + placement ids; cascaded closes tagged `caused_by = employee_offboard` + `source_employee_id`.
- **Traceability:** F2.7, OB-1, OB-3, OB-9, OB-12, INV-6, EP-7, EA-5, AC "Terminate for cause".

#### TC-E2-F2.7-002 · Resignation effective today closes agreement & revokes
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** RESIGNED reason path.
- **Preconditions:** "Eka" active agreement + working login.
- **Steps:**
  1. Offboard "Eka", reason RESIGNED, effective today.
  2. Confirm.
- **Expected result / Acceptance criteria:** Agreement closed RESIGNED; employee inactive; placement ended (mapped RESIGNED→RESIGNED terminal); login disabled + sessions revoked; audit written.
- **Traceability:** F2.7, OB-1, OB-3, OB-5, AC "Every offboard revokes the login".

#### TC-E2-F2.7-003 · TERMINATED / ABSCONDED requires a free-text reason; OTHER requires a note
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Reason note validation (OB-3).
- **Preconditions:** Active employee "Budi".
- **Steps:**
  1. Offboard with reason TERMINATED but empty cause; Confirm.
  2. Offboard with reason OTHER but empty note; Confirm.
- **Expected result / Acceptance criteria:** Both blocked with a field error requiring the free-text reason/note; no offboard occurs. (MVP CHECK-supported reason set = END_OF_TERM / RESIGNED / TERMINATED / OTHER.)
- **Traceability:** F2.7, OB-3.

#### TC-E2-F2.7-004 · MVP reason set only — RETIRED/DECEASED/ABSCONDED deferred
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Confirm the offboard reason dropdown exposes only the 4 CHECK-supported values in MVP.
- **Preconditions:** Open the Offboard form.
- **Steps:**
  1. Inspect the reason dropdown.
- **Expected result / Acceptance criteria:** Only END_OF_TERM, RESIGNED, TERMINATED, OTHER selectable (RETIRED/DECEASED/ABSCONDED deferred pending a CHECK-constraint migration); a forced API call with `closed_reason = DECEASED` is rejected.
- **Traceability:** F2.7, OB-3 (reconciled 2026-06-07), §10.

#### TC-E2-F2.7-005 · Reason→placement-terminal mapping
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Verify the placement terminal state matches the offboard reason.
- **Preconditions:** Three active employees each with one open placement.
- **Steps:**
  1. Offboard one RESIGNED, one TERMINATED, one END_OF_TERM (effective today).
  2. Inspect each ended placement's terminal state.
- **Expected result / Acceptance criteria:** RESIGNED→placement RESIGNED; TERMINATED→placement TERMINATED; END_OF_TERM (and OTHER)→placement ENDED.
- **Traceability:** F2.7, OB-1, implementation note (reason→terminal mapping).

#### TC-E2-F2.7-006 · Future-dated resignation creates pending offboarding, login stays valid (OB-7)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Future effective date schedules revocation; access retained until then.
- **Preconditions:** "Budi" active + working login. Today = `2026-06-17`.
- **Steps:**
  1. Offboard "Budi" RESIGNED, effective `2026-07-31` (last working day).
  2. Confirm.
  3. Have Budi make an authenticated request now.
- **Expected result / Acceptance criteria:** An `Offboarding` record created with `status = pending`, `effective_date = 2026-07-31`; agreement still active; Budi's request now still succeeds; the pending offboard is visible and cancellable.
- **Traceability:** F2.7, OB-7, Offboarding data model §6.

#### TC-E2-F2.7-007 · Pending future-dated offboard fires on its date (revokes)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Edge
- **Objective:** Scheduled job applies the offboard at effective_date.
- **Preconditions:** Pending offboarding for "Budi" effective `2026-07-31` exists; simulate the job running on `2026-07-31` Asia/Jakarta.
- **Steps:**
  1. Advance/trigger the daily job for `2026-07-31`.
  2. Have Budi make a request after the job runs.
- **Expected result / Acceptance criteria:** Offboarding `status = applied`, `applied_at` set; agreement closed RESIGNED; employee inactive; sessions revoked; Budi's next request returns `401`; audit entry by `system` actor.
- **Traceability:** F2.7, OB-7, OB-9, OB-12.

#### TC-E2-F2.7-008 · Cancel a pending future-dated offboard before it fires (C-3)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Agent reconsiders; pending offboard cancellable.
- **Preconditions:** Pending offboarding for "Budi" effective `2026-07-31`.
- **Steps:**
  1. Before `2026-07-31`, open the pending offboard and Cancel.
- **Expected result / Acceptance criteria:** Offboarding `status = cancelled`; agreement stays active; no revocation fired; login keeps working; audit entry.
- **Traceability:** F2.7, OB-7, C-3.

#### TC-E2-F2.7-009 · Offboard an already-inactive employee returns 409 (OB-11)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Negative
- **Objective:** Idempotency/conflict.
- **Preconditions:** "Budi" already inactive (no active agreement).
- **Steps:**
  1. Attempt to offboard "Budi" again.
- **Expected result / Acceptance criteria:** `409 CONFLICT` (nothing to close); no state change.
- **Traceability:** F2.7, OB-11, AC "Cannot offboard already-inactive".

#### TC-E2-F2.7-010 · Offboard an employee with no active agreement returns 409
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Negative
- **Objective:** No active agreement ⇒ nothing to offboard.
- **Preconditions:** Active employee whose agreement is somehow already closed.
- **Steps:**
  1. Attempt to offboard.
- **Expected result / Acceptance criteria:** `409 CONFLICT`.
- **Traceability:** F2.7, OB-11.

#### TC-E2-F2.7-011 · PKWT within 30 days flagged expiring + Inbox decision task (OB-4)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Expiry job flags but never terminates.
- **Preconditions:** "Budi" PKWT ending `2026-07-15` (28 days out from today `2026-06-17`); simulate the daily job.
- **Steps:**
  1. Run the expiry job (Asia/Jakarta).
  2. Open the HR Inbox.
- **Expected result / Acceptance criteria:** Agreement derived flag `expiring`; a decision task appears in HR Inbox (payload: employee, agreement, end_date, days-remaining; actions Continue/End); Budi's login keeps working; HR-admins notified.
- **Traceability:** F2.7, OB-4, EA-8, CONVENTIONS §16.2 (agreement-expiring notification).

#### TC-E2-F2.7-012 · PKWTT never enters expiring (OB-4)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Open-ended agreements never flagged.
- **Preconditions:** "Siti" active PKWTT (no end date).
- **Steps:**
  1. Run the expiry job.
- **Expected result / Acceptance criteria:** Siti's agreement stays `active`; no decision task raised.
- **Traceability:** F2.7, OB-4, AC "PKWTT never auto-expires".

#### TC-E2-F2.7-013 · HR chooses "Continue" on expiry task → renew, no revoke
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** Continue path renews agreement+placement, no offboard.
- **Preconditions:** "Budi" expiring PKWT with an open decision task.
- **Steps:**
  1. In Inbox, open the task; choose Continue.
  2. Complete the renewal (new agreement + placement) per EA-3 / F3.2.
- **Expected result / Acceptance criteria:** Successor agreement created (predecessor superseded); login NOT revoked; placement continued; decision task closed; audit written.
- **Traceability:** F2.7, OB-4, EA-3, INV-6.

#### TC-E2-F2.7-014 · HR chooses "End" on expiry task → offboard END_OF_TERM
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Happy
- **Objective:** End path offboards and revokes.
- **Preconditions:** "Budi" expiring PKWT with open decision task.
- **Steps:**
  1. In Inbox, choose End; reason END_OF_TERM; confirm.
- **Expected result / Acceptance criteria:** Offboard fires (agreement closed END_OF_TERM, employee inactive, placement ENDED, sessions revoked); decision task closed; audit written.
- **Traceability:** F2.7, OB-4, OB-1, AC "HR chooses to end".

#### TC-E2-F2.7-015 · Grace — lapsed PKWT keeps access until HR decides (OB-6)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Edge
- **Objective:** No auto-offboard; task escalates.
- **Preconditions:** "Budi" expiring PKWT whose end_date `2026-06-16` passed yesterday with no decision; simulate today's job.
- **Steps:**
  1. Run the daily job for `2026-06-17`.
  2. Check Budi's agreement + login + the Inbox task.
- **Expected result / Acceptance criteria:** Agreement remains `expiring`; login still works; no agreement closed/revoked on a timer; decision task escalated (re-notify).
- **Traceability:** F2.7, OB-6, EA-9, AC "Grace".

#### TC-E2-F2.7-016 · Expiry job missed a day catches up safely (C-7)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Downtime catch-up only delays the flag, never terminates.
- **Preconditions:** Job didn't run on `2026-06-16`; several PKWTs became due in the interim.
- **Steps:**
  1. Run the job on `2026-06-17` (catch-up).
- **Expected result / Acceptance criteria:** All PKWTs due by date are evaluated and flagged `expiring`; no unintended offboards/revocations (no auto-offboard exists).
- **Traceability:** F2.7, OB-6, C-7.

#### TC-E2-F2.7-017 · Concurrent decision on the same expiry task — first wins (C-10)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Acting on an already-decided task returns 409.
- **Preconditions:** Two HR admins open the same expiry task; first decides End.
- **Steps:**
  1. Admin A submits End.
  2. Admin B submits any decision on the same task.
- **Expected result / Acceptance criteria:** Admin A succeeds; Admin B gets `409 CONFLICT`; task already resolved.
- **Traceability:** F2.7, OB-11, C-10.

#### TC-E2-F2.7-018 · Placement transfer does NOT revoke login (OB-2)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Edge
- **Objective:** Placement-only transitions never offboard.
- **Preconditions:** "Budi" actively placed at "Plaza Senayan", working login.
- **Steps:**
  1. Transfer Budi to "Grand Indonesia" (E3 action).
  2. Have Budi make an authenticated request.
- **Expected result / Acceptance criteria:** Old placement → Transferred, new one opens; agreement stays active; login keeps working (no revocation); request succeeds.
- **Traceability:** F2.7, OB-2, INV-6, AC "Placement transfer does not revoke".

#### TC-E2-F2.7-019 · Reactivate offboarded employee requires a new agreement (OB-8 / C-6)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Reactivation bounded; old sessions stay dead.
- **Preconditions:** "Budi" offboarded (agreement closed, login disabled).
- **Steps:**
  1. Reactivate Budi.
  2. Attempt to reactivate into the closed agreement (should be impossible).
  3. Create a new active agreement; reactivation completes.
- **Expected result / Acceptance criteria:** Cannot reactivate into the closed agreement; a new active agreement is required; login re-enabled/re-invited; old sessions are NOT restored (re-auth fresh).
- **Traceability:** F2.7, OB-8, C-6, F2.1 C-3.

#### TC-E2-F2.7-020 · Access token issued seconds before revocation is rejected (C-5)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P0 · **Type:** Edge
- **Objective:** Epoch invalidates pre-revocation tokens (`token.iat < tokens_valid_after`).
- **Preconditions:** Budi mints a fresh access token at T0; HR offboards Budi at T0+5s.
- **Steps:**
  1. Capture Budi's token issued at T0.
  2. Offboard Budi at T0+5s.
  3. Use the T0 token after revocation.
- **Expected result / Acceptance criteria:** `401` — token rejected because `iat < tokens_valid_after`; the short access-token window is closed by the epoch, not left to expiry.
- **Traceability:** F2.7, OB-9, C-5, CONVENTIONS §3 (not purely stateless).

#### TC-E2-F2.7-021 · Super Admin corrects a wrong closed_reason post-offboard (C-9)
- [ ] **Platform:** Web · **POV:** Super Admin · **Priority:** P1 · **Type:** Edge
- **Objective:** Terminal-record correction by super admin only; no session re-issue.
- **Preconditions:** "Budi" offboarded with reason TERMINATED (wrong); should be RESIGNED.
- **Steps:**
  1. As super admin, correct `closed_reason` to RESIGNED.
- **Expected result / Acceptance criteria:** Correction allowed (override); re-audited; sessions are NOT re-issued; HR-admin (non-super) attempting the same correction is denied (terminal records immutable except super-admin override).
- **Traceability:** F2.7, OB-10, C-9.

#### TC-E2-F2.7-022 · Drift: inactive employee with active agreement reconciled (C-8)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P2 · **Type:** Edge
- **Objective:** Offboarding treats agreement as source of truth.
- **Preconditions:** Migration drift — employee inactive but agreement still `active`.
- **Steps:**
  1. Run offboard reconciliation on the record.
- **Expected result / Acceptance criteria:** The dangling active agreement is closed; surfaced in the E9 review queue; consistent terminal state achieved.
- **Traceability:** F2.7, C-8.

#### TC-E2-F2.7-023 · Offboard confirmation modal previews the cascade (no dead-flow)
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Happy
- **Objective:** Destructive action confirms and shows what will be closed.
- **Preconditions:** Active employee with agreement + placement.
- **Steps:**
  1. Click Berhentikan; observe the confirmation modal.
- **Expected result / Acceptance criteria:** Modal warns it will close the agreement, end the placement, disable the login, and revoke all sessions; requires reason + effective date; Cancel aborts with no change.
- **Traceability:** F2.7, OB-1, OB-3, ENGINEERING (no dead-flow states).

#### TC-E2-F2.7-024 · Offboard transaction failure leaves no partial state
- [ ] **Platform:** Web · **POV:** HR/Placement Admin · **Priority:** P1 · **Type:** Empty/Loading/Error
- **Objective:** Atomicity — partial cascade must not persist.
- **Preconditions:** Simulate a mid-transaction failure (e.g., revocation hook errors).
- **Steps:**
  1. Submit a valid offboard; force a failure during the cascade.
- **Expected result / Acceptance criteria:** Whole transaction rolls back — agreement still active, employee active, placement open, login still valid; error envelope surfaced; retry possible. Fail-safe never leaves a half-revoked user.
- **Traceability:** F2.7, OB-1, CONVENTIONS §11.

### Web console · Shift Leader POV (RBAC denial)

#### TC-E2-F2.7-025 · Shift leader cannot offboard or act on the expiry task (OB-10)
- [ ] **Platform:** Web · **POV:** Shift Leader · **Priority:** P0 · **Type:** RBAC
- **Objective:** Only HR / Super Admin may offboard or decide expiry.
- **Preconditions:** Logged in as a shift leader of a company; an active employee in that company.
- **Steps:**
  1. Attempt `POST /employees/{id}:deactivate`.
  2. Attempt to act on an expiry decision task.
- **Expected result / Acceptance criteria:** Both `403 FORBIDDEN`; no offboard; task unchanged.
- **Traceability:** F2.7, OB-10, CONVENTIONS §17.

### Mobile · Agent POV (subject of revocation)

#### TC-E2-F2.7-026 · Revoked agent is signed out on next request with re-login screen (C-4)
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P0 · **Type:** Edge
- **Objective:** No dead-flow; revoked session routes to a clear re-login state.
- **Preconditions:** Budi mid-session on mobile; HR offboards him effective today.
- **Steps:**
  1. With Budi active in the app, HR offboards him.
  2. Budi performs any action triggering an authenticated request.
- **Expected result / Acceptance criteria:** Request returns `401`; app routes to a re-login screen showing a "tidak ada hubungan kerja aktif" / no-active-employment message (per CONVENTIONS `comp/EmptySessionExpired`); Budi cannot re-authenticate (login disabled).
- **Traceability:** F2.7, OB-9, C-4, CONVENTIONS §3.

#### TC-E2-F2.7-027 · Agent cannot self-offboard or self-revoke
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** RBAC
- **Objective:** Offboarding is HR/Super-admin only (OB-10).
- **Preconditions:** Agent "Budi" logged in.
- **Steps:**
  1. Attempt `POST /employees/SWP-EMP-1042:deactivate` as the agent.
- **Expected result / Acceptance criteria:** `403 FORBIDDEN`; no offboard.
- **Traceability:** F2.7, OB-10, CONVENTIONS §17.

#### TC-E2-F2.7-028 · Future-dated offboard — agent retains mobile access until the date
- [ ] **Platform:** Mobile · **POV:** Agent · **Priority:** P1 · **Type:** Edge
- **Objective:** Pending offboard does not revoke early.
- **Preconditions:** Pending offboarding for Budi effective `2026-07-31`; today `2026-06-17`.
- **Steps:**
  1. Budi clocks in / uses the app before `2026-07-31`.
- **Expected result / Acceptance criteria:** All authenticated actions succeed normally; access only ends when the job fires on `2026-07-31`.
- **Traceability:** F2.7, OB-7, OB-9.

---

## Notes on traceability & gaps

- **Notifications (CONVENTIONS §16.2):** agreement-expiring (E2) notifies HR admins (cron-driven); covered in TC-E2-F2.7-011. Notification *delivery* mechanics are E10; E2 cases assert the dispatch trigger only.
- **Audit (CONVENTIONS §16.1):** every write writes an audit entry; representative audit assertions are embedded (TC-E2-F2.1-001, F2.2-014, F2.6-016, F2.7-001) rather than repeated on every case.
- **Cross-epic boundaries:** E3 (placement targets/transfer/roster), E5 (geofence evaluation, attendance), E6 (leave eligibility/quota), E7 (OT calc) are *consumed by* E2 — these cases assert E2 stores/exposes the data correctly and stop at the epic boundary.
- **Migration cases** (F2.1 C-5, F2.2 C-4/C-5, F2.3 C-3/C-4, F2.6 C-4, F2.5 C-2) are E9 territory (one-shot script, no UI) and are intentionally not expanded as interactive manual cases here beyond noting the expected reconciliation outcome.
