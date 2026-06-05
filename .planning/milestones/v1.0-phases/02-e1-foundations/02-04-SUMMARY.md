---
phase: 02-e1-foundations
plan: "04"
subsystem: frontend-e2e
tags: [playwright, e2e, e1-foundations, users, audit-log, platform-settings, session-restore]

# Dependency graph
requires:
  - phase: 02-e1-foundations
    plan: "02"
    provides: "E1 Go API handlers (GET /users, /audit-log, /platform/settings) + seed extension"
  - phase: 01-test-harness-auth
    provides: "E2E harness (globalSetup, fixtures, resetDb, personas)"
provides:
  - "user-management.spec.ts — 8 tests: FND-01/RB-2/RB-6/AL-7 (list, create, edit, change-role, deactivate, reactivate, send-reset, RBAC neg)"
  - "audit-log.spec.ts — 5 active tests: FND-02/AL-5/AL-7 (list, filter, paginate, drawer, RBAC neg)"
  - "platform-settings.spec.ts — 1 test: FND-03/PC-1/PC-2/PC-5 (locale/timezone/currency from real BE)"
  - "tryRestoreSession() in auth.ts: httpOnly refresh cookie hydration on page reload"
  - "db.ts helpers: getUserStatus, getUserRole, countAuditRowsByEntityType, getLatestAuditAction, insertAuditRows"
affects: [all future e2e specs that use page.goto() on authed routes]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "tryRestoreSession(): POST /auth/refresh (cookie) + GET /auth/me before React mounts; enables page.goto() on authed routes in E2E"
    - "DataTable uses <div> rows not <tr>; use div.border-b.filter({ hasText }) for row-specific actions in Playwright"
    - "StatusBadge text not matchable by getByText in Playwright; use footer 'Menampilkan N entri' regex instead"
    - "playwright.config.ts timeout: 90_000 for cold Vite dev server startup on first test"
    - "insertAuditRows(n): direct INSERT with swp_next_id('AL') for pagination seeding; columns are before_state/after_state not before/after"

key-files:
  created:
    - frontend/e2e/tests/e1/user-management.spec.ts
    - frontend/e2e/tests/e1/audit-log.spec.ts
    - frontend/e2e/tests/e1/platform-settings.spec.ts
  modified:
    - frontend/apps/web/src/lib/auth.ts (tryRestoreSession added)
    - frontend/apps/web/src/main.tsx (tryRestoreSession called before createRoot)
    - frontend/e2e/lib/db.ts (4 new helpers + insertAuditRows)
    - frontend/e2e/playwright.config.ts (timeout: 90_000)

key-decisions:
  - "tryRestoreSession hydrates in-memory accessToken from httpOnly refresh cookie before React mounts; allows page.goto() on authed routes without 302-to-login"
  - "DataTable renders div rows not tr; tr/tbody locators silently return 0 matches — use div.border-b filter pattern"
  - "StatusBadge with dot=true: text content split across sibling spans; getByText('CREATE') fails in Playwright — use footer row-count regex instead"
  - "playwright.config.ts timeout raised to 90s for cold Vite compilation; first-test audit-log assertions need 50s headroom"
  - "insertAuditRows: correct column names are before_state/after_state (not before/after per migration 00004)"
  - "audit-log filter test: after entity_type=user filter, UI shows 4 rows (4 user-entity rows in seed) → assert Menampilkan 4 entri"
  - "PERUBAHAN selector: use exact full text 'PERUBAHAN (before → after)' to avoid strict mode violation with DataTable column header 'PERUBAHAN'"

# Metrics
duration: 107min
completed: "2026-06-04"
---

# Phase 02 Plan 04: E1 Foundations FE Wiring + Exhaustive E2E Summary

**E1 foundations screens wired to real BE (MSW off) + full Playwright E2E suite green (23 passed, 2 intentionally skipped)**

## Performance

- **Duration:** ~107 min (includes 10 iterative test runs debugging selectors/timing)
- **Started:** 2026-06-04T00:50:39Z
- **Completed:** 2026-06-04T02:37:00Z
- **Tasks:** 2
- **Files modified:** 7 (3 created, 4 extended)

## Accomplishments

### Task 1: Session restore + db helpers + user-management spec

**auth.ts `tryRestoreSession()`**: Bootstraps the in-memory access token from the httpOnly refresh cookie before React mounts. Without this, any `page.goto()` on an authed route in Playwright redirected to `/login` because `auth.isAuthenticated()` returned `false` (the in-memory token is cleared on navigation). Now: `POST /auth/refresh` (cookie auto-sent) → `GET /auth/me` → `auth.login(token, user)` before `createRoot().render()`.

**db.ts new helpers:**
- `getUserStatus(email)`: SELECT status FROM users WHERE lower(email)=lower($1)
- `getUserRole(email)`: SELECT role FROM users WHERE lower(email)=lower($1)
- `countAuditRowsByEntityType(entityType)`: COUNT FROM audit_log WHERE entity_type=$1
- `getLatestAuditAction()`: SELECT action FROM audit_log ORDER BY created_at DESC LIMIT 1
- `insertAuditRows(count)`: INSERT INTO audit_log with correct before_state/after_state columns and swp_next_id('AL') for IDs

**user-management.spec.ts (8 tests):**
- `FND-01 · users list renders seeded users from the real BE` — asserts Sari Hadi, Dewi Lestari, Bambang Sutrisno visible
- `FND-01 · create user opens modal, submits, and user row appears` — creates new.testuser@swp.test, asserts toast + DB getUserRole='agent'
- `FND-01 · edit user email updates the row in the list` — edits Dewi's email, asserts toast + updated email
- `FND-01/RB-6 · change role updates DB role and writes an audit entry` — changes Dewi's role, asserts toast + countAuditRowsByEntityType increased + getLatestAuditAction matches change_role
- `FND-01 · deactivate user sets status disabled, reactivate restores active` — full cycle via Dewi's row; asserts deactivate/reactivate badges
- `FND-01 · deactivate specific user: DB status = disabled` — deactivates Dewi, asserts getUserStatus='disabled', then reactivates to 'active'
- `FND-01 · send password reset inserts a reset token and shows success toast` — sends reset for Dewi, asserts toast + countResetTokensFor >= 1
- `RB-2/AL-7 · agent is denied the users management screen` — agent gets no-permission EmptyState

**playwright.config.ts**: raised timeout to 90s for cold Vite compilation.

### Task 2: Audit-log + platform-settings specs

**audit-log.spec.ts (5 active tests + 1 skipped):**
- `FND-02 · audit log lists seeded entries from the real BE` — asserts heading + footer "Menampilkan 5 entri"
- `FND-02/AL-5 · filter by entity type user shows only user-entity rows` — selects "Pengguna" filter, asserts count matches DB countAuditRowsByEntityType('user')=4
- `FND-02 · cursor pagination advances to the next page` — inserts 50 extra rows (55 total), clicks Berikutnya, asserts count label changes from 50 to 5
- `FND-02 · clicking an audit row opens the detail drawer with entry content` — clicks first data row, asserts drawer opens with "Entri append-only" footer and "PERUBAHAN (before → after)" diff title
- `AL-7 · agent is denied the audit log screen` — agent gets no-permission state
- `AL-2/C-2 · bulk delete` — skipped (N/A for Phase 2)

**platform-settings.spec.ts (1 test):**
- `FND-03/PC-1/PC-2/PC-5 · platform settings load locale, timezone, and currency from the real BE` — asserts "Bahasa Indonesia", "Asia/Jakarta", "Rupiah" visible; "Terkunci" chip present

## Route URLs Used

| Screen | Route URL |
|--------|-----------|
| Users management | `/settings/users` |
| Audit log | `/settings/audit-log` |
| Platform settings | `/settings/general` |

## Selectors / i18n Keys Used

| Assertion | Selector | i18n Key |
|-----------|----------|----------|
| Users list heading | `getByRole('heading', { name: 'Pengguna & Peran' })` | `users.title` |
| Add user button | `getByRole('button', { name: 'Tambah Pengguna' })` | `users.add` |
| Create modal title | `getByText('Tambah Pengguna').first()` | `userOverlays.createTitle` |
| Create success toast | `getByText('Pengguna berhasil dibuat.')` | `userOverlays.createSuccess` |
| Row action button | `getByRole('button', { name: 'Aksi baris' })` | `users.rowActions` |
| Edit drawer | `getByText('Edit Pengguna')` | `userOverlays.editTitle` |
| Change-role modal | `getByText('Ubah peran pengguna')` | `userOverlays.changeRoleTitle` |
| Deactivate confirm | `getByText('Nonaktifkan pengguna?')` | `userOverlays.deactivateTitle` |
| Deactivate button | `getByRole('button', { name: 'Nonaktifkan' })` | `userOverlays.deactivateConfirm` |
| Reactivate button | `getByRole('button', { name: 'Aktifkan' })` | `userOverlays.reactivateConfirm` |
| Send-reset confirm | `getByText('Kirim email reset kata sandi?')` | `userOverlays.sendResetTitle` |
| Audit log heading | `getByRole('heading', { name: 'Audit Log' })` | `auditLog.title` |
| Audit row count | `getByText(/Menampilkan \d+ entri/).first()` | `auditLog.rowCount` |
| Audit entity filter | `getByRole('combobox', { name: 'Filter entitas' })` | `auditLog.filterEntityLabel` |
| Drawer diff title | `getByText('PERUBAHAN (before → after)')` | `auditLog.diffTitle` |
| Drawer footer | `getByText(/Entri append-only/i)` | `auditLog.appendOnlyNote` |
| Settings heading | `getByRole('heading', { name: 'Pengaturan' })` | `settingsGeneral.title` |
| Locale value | `getByText('Bahasa Indonesia').first()` | (from BE platform_settings) |
| Timezone value | `getByText('Asia/Jakarta').first()` | (from BE platform_settings) |
| Currency value | `getByText(/Rupiah\|IDR/).first()` | (from BE platform_settings) |

## FE Screen Fix: tryRestoreSession

**No FE screen code was changed.** The only FE fix was adding `tryRestoreSession()` to `auth.ts` and calling it in `main.tsx`. This is critical infrastructure: without it, `page.goto()` on any authenticated route in Playwright would redirect to `/login` because the in-memory access token is cleared on page navigation.

## Audit Pagination Seeding Approach

The default seed only has 5 audit rows (below the 50-row page size). The pagination test calls `insertAuditRows(50)` in `beforeEach`-adjacent code (before `loginAs`) to create 55 total rows, ensuring `has_more: true` on the first page. The `insertAuditRows` function uses `swp_next_id('AL')` for SWP-AL- prefixed IDs and correctly uses `before_state`/`after_state` column names from migration 00004.

## Green Test Count Per Spec File

| File | Tests | Pass | Skip | Fail |
|------|-------|------|------|------|
| authentication.spec.ts | 10 | 9 | 1 | 0 |
| audit-log.spec.ts | 6 | 5 | 1 | 0 |
| platform-settings.spec.ts | 1 | 1 | 0 | 0 |
| user-management.spec.ts | 8 | 8 | 0 | 0 |
| **Total** | **25** | **23** | **2** | **0** |

## Task Commits

1. **Task 1: E1 FE wiring + session restore + user-management spec** - `de69a7a` (feat)
2. **Task 2: audit-log + platform-settings E2E specs** - `0d4e357` (feat)

## Files Created/Modified

**Created:**
- `frontend/e2e/tests/e1/user-management.spec.ts` — 8 user-management E2E tests (FND-01/RB-2/RB-6/AL-7)
- `frontend/e2e/tests/e1/audit-log.spec.ts` — 6 audit-log tests (FND-02/AL-5/AL-7 + 1 skipped)
- `frontend/e2e/tests/e1/platform-settings.spec.ts` — 1 platform settings test (FND-03/PC-1/PC-2/PC-5)

**Modified:**
- `frontend/apps/web/src/lib/auth.ts` — added `tryRestoreSession()` (50 lines) for httpOnly cookie session hydration
- `frontend/apps/web/src/main.tsx` — call `tryRestoreSession()` in init chain before `createRoot().render()`
- `frontend/e2e/lib/db.ts` — 5 new helpers: getUserStatus, getUserRole, countAuditRowsByEntityType, getLatestAuditAction, insertAuditRows
- `frontend/e2e/playwright.config.ts` — `timeout: 90_000` for cold Vite dev server tolerance

## Decisions Made

### tryRestoreSession() architecture
The in-memory `accessToken` in `auth.ts` is cleared on any page reload (designed for XSS safety, per WEB-STACK §6). E2E tests use `page.goto()` which causes full page reloads. Without session hydration, every `page.goto('/authed-route')` triggers `beforeLoad → auth.isAuthenticated() === false → redirect to /login`. The fix: call `POST /auth/refresh` (cookie-based, no body needed per `readRefreshToken` in handler.go which reads cookie first) + `GET /auth/me` before React mounts. This restores the in-memory token while keeping the design XSS-safe (the cookie is httpOnly).

### DataTable div-based rows
The `DataTable` component renders rows as `<div class="flex border-b border-border-soft ...">` elements, NOT `<tr>`. Playwright locators using `tr`, `tbody tr`, or `td` return 0 matches. Correct pattern: `page.locator('div.border-b').filter({ hasText: 'Dewi Lestari' }).filter({ has: page.getByRole('button', { name: 'Aksi baris' }) })`.

### StatusBadge text not matchable by getByText
The `StatusBadge` with `dot=true` renders a sibling `<span aria-hidden>` dot before the text child. Playwright's `getByText('CREATE')` fails in this context (possibly due to how text content is split or adjacent node matching). Workaround: use the footer row-count label `getByText(/Menampilkan \d+ entri/)` which is a plain text node and reliably matchable.

### audit_log column names
Migration 00004 uses `before_state` and `after_state` (JSONB), not `before`/`after`. The `insertAuditRows` helper in the initial draft used wrong column names, causing a DB error. Fixed to match the actual schema.

### playwright.config.ts timeout
Default Playwright test timeout is 30s. The first audit-log tests (alphabetically first to run) hit a cold Vite dev server that takes 30-50s to compile. Individual assertion timeouts up to 50s + 90s overall test timeout ensures green runs even on cold starts.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] tryRestoreSession for page.goto() on authed routes**
- **Found during:** Task 1 (first test run attempt)
- **Issue:** All E1 screen tests redirected to /login because `auth.isAuthenticated()` returns false on page reload (in-memory token cleared) — screens could not be tested with `page.goto()`
- **Fix:** Added `tryRestoreSession()` to `auth.ts` and wired in `main.tsx`; calls `/auth/refresh` + `/auth/me` before React mounts to restore the session from the httpOnly cookie
- **Files modified:** `frontend/apps/web/src/lib/auth.ts`, `frontend/apps/web/src/main.tsx`
- **Commits:** `de69a7a`

**2. [Rule 1 - Bug] insertAuditRows used wrong column names**
- **Found during:** Task 2 (pagination test attempt)
- **Issue:** `INSERT INTO audit_log (... before, after ...)` → column "before" does not exist; migration 00004 uses `before_state`/`after_state`
- **Fix:** Corrected column names to `before_state`/`after_state`; also added proper `id` generation via `swp_next_id('AL')`
- **Files modified:** `frontend/e2e/lib/db.ts`
- **Commit:** `de69a7a`

**3. [Rule 1 - Bug] getByText('CREATE') fails for StatusBadge text**
- **Found during:** Task 2 (audit-log test attempts #5-8)
- **Issue:** `page.getByText('CREATE')` consistently times out even when the text IS visible on screen (screenshots confirmed). StatusBadge text is not reliably matchable via `getByText`.
- **Fix:** Replaced all `getByText('CREATE')` with `getByText(/Menampilkan \d+ entri/)` (footer row-count label) as the "data loaded" sentinel
- **Files modified:** `frontend/e2e/tests/e1/audit-log.spec.ts`
- **Commit:** `0d4e357`

**4. [Rule 1 - Bug] DataTable uses div rows not tr — row locators wrong**
- **Found during:** Task 1 (first test run attempt)
- **Issue:** Locators using `page.locator('tr', { has: ... })` and `page.locator('tbody tr')` returned 0 matches; DataTable uses `<div>` not `<table>`/`<tr>`
- **Fix:** Updated all row locators to `page.locator('div.border-b').filter({ hasText: '...' }).filter({ has: ... })`
- **Files modified:** `frontend/e2e/tests/e1/user-management.spec.ts`, `frontend/e2e/tests/e1/audit-log.spec.ts`
- **Commit:** `de69a7a`, `0d4e357`

**5. [Rule 1 - Bug] PERUBAHAN strict mode violation**
- **Found during:** Task 2 (drawer test)
- **Issue:** `getByText(/PERUBAHAN/i)` matches both the DataTable column header "PERUBAHAN" and the drawer diff title "PERUBAHAN (before → after)" — strict mode violation
- **Fix:** Use exact full string `page.getByText('PERUBAHAN (before → after)')` to match only the drawer element
- **Files modified:** `frontend/e2e/tests/e1/audit-log.spec.ts`
- **Commit:** `0d4e357`

**6. [Rule 1 - Bug] Asia/Jakarta and Sari Hadi strict mode violations**
- **Found during:** Task 2 / Task 1 (platform-settings and users-list tests)
- **Issue:** "Asia/Jakarta" appears twice in the platform settings row (label and value), "Sari Hadi" appears twice (table row + topbar UserMenu)
- **Fix:** Added `.first()` to all potentially-duplicated text assertions
- **Files modified:** `frontend/e2e/tests/e1/platform-settings.spec.ts`, `frontend/e2e/tests/e1/user-management.spec.ts`
- **Commit:** `0d4e357`

**7. [Rule 3 - Blocking] playwright.config.ts timeout too low for cold Vite start**
- **Found during:** Tasks 1-2 (repeated test failures at exactly 30s)
- **Issue:** Default 30s test timeout + cold Vite compilation = first-batch tests always fail; screenshots proved the page DID load, just after the 30s cutoff
- **Fix:** Raised `timeout` to `90_000` in playwright.config.ts; raised first-assertion timeouts to 50s in audit-log spec
- **Files modified:** `frontend/e2e/playwright.config.ts`, `frontend/e2e/tests/e1/audit-log.spec.ts`
- **Commit:** `de69a7a`

## Self-Check

---
## Self-Check: PASSED

Files exist:
- `frontend/e2e/tests/e1/user-management.spec.ts` — FOUND
- `frontend/e2e/tests/e1/audit-log.spec.ts` — FOUND
- `frontend/e2e/tests/e1/platform-settings.spec.ts` — FOUND

Commits:
- `de69a7a` — feat(02-04): verify E1 screens on real BE + user-management E2E spec — FOUND
- `0d4e357` — feat(02-04): audit-log + platform-settings E2E specs — E1 suite green — FOUND

Test counts:
- user-management.spec.ts: 8 test() ≥ 7 required — PASSED
- audit-log.spec.ts: 5 active ≥ 4 required — PASSED
- platform-settings.spec.ts: 1 test ≥ 1 required — PASSED

Final run: 23 passed, 2 skipped (intentional), 0 failed — PASSED
