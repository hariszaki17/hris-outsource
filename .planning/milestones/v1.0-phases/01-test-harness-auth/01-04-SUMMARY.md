---
phase: 01-test-harness-auth
plan: 04
subsystem: auth
tags: [react, tanstack-router, tanstack-query, zod, react-hook-form, api-client, msw]

requires:
  - phase: 01-test-harness-auth/01-02
    provides: "@swp/api-client with generated E1 hooks (useAuthLogin, useAuthLogout, useAuthForgotPassword, useAuthResetPassword)"

provides:
  - "Real login flow: useAuthLogin() + buildSessionUser() + error-code banner mapping"
  - "Real logout flow: useAuthLogout() called from shell before auth.clear()"
  - "Real forgot-password flow: useAuthForgotPassword() always advances to 'sent'"
  - "Real reset-password flow: useAuthResetPassword() reads token from typed URL param"
  - "credentials:'include' on all API fetches for cross-origin refresh cookie transport"
  - ".env.development with VITE_ENABLE_MSW=true / VITE_API_BASE_URL defaults"

affects:
  - 01-test-harness-auth/01-05
  - all-subsequent-phases

tech-stack:
  added: []
  patterns:
    - "buildSessionUser(MeResponse) mapper in auth.ts — single place for BE user → SessionUser conversion"
    - "Error code switch on ApiError.code for banner routing (INVALID_CREDENTIALS/ACCOUNT_DISABLED/ACCOUNT_LOCKED)"
    - "logout in shell.tsx calls hook then auth.clear() + navigate (hook failure doesn't block clear)"
    - "credentials:'include' in mutator.ts customFetch — applies to every generated hook automatically"
    - "validateSearch on /reset-password route types the token?: string param (TanStack Router D1)"

key-files:
  created:
    - frontend/apps/web/.env.development
  modified:
    - frontend/packages/api-client/src/mutator.ts
    - frontend/apps/web/src/lib/auth.ts
    - frontend/apps/web/src/features/auth/login-screen.tsx
    - frontend/apps/web/src/features/auth/forgot-password-screen.tsx
    - frontend/apps/web/src/features/auth/reset-password-screen.tsx
    - frontend/apps/web/src/app/shell.tsx
    - frontend/apps/web/src/app/user-menu.tsx
    - frontend/apps/web/src/app/router.tsx

key-decisions:
  - "buildSessionUser sets companyName = scope.company_id literal for shift_leader (no company-name endpoint in Phase 1); TODO(Phase-3) to resolve via companies endpoint"
  - "credentials:'include' added to mutator.ts customFetch so ALL generated hooks send the refresh cookie cross-origin; BE sets CORS allow-origin for :4173/:5173"
  - "logout handler lives in shell.tsx (useAuthLogout) and is passed to UserMenu as onLogout prop; UserMenu stays stateless re: auth — cleaner architecture"
  - "forgot-password always advances to 'sent' even on network error (anti-enumeration, authentication.md C-2)"
  - "reset-password minLength raised from 8 to 10 to match BE platform password policy (AU-4)"
  - ".env.example already documented both vars; .env.development created with MSW=true defaults; e2e harness overrides both via playwright webServer env"

patterns-established:
  - "ApiError.code switch: read error code from caught ApiError, map to UI state (banner/field error/navigate)"
  - "buildSessionUser: canonical MeResponse → SessionUser mapper, importable from lib/auth.ts"

requirements-completed: [AUTH-01, AUTH-02, AUTH-04]

duration: 38min
completed: 2026-06-04
---

# Phase 1 Plan 04: Wire Auth Screens to Real API Hooks

**Login, forgot-password, and reset-password screens wired to generated `@swp/api-client` E1 hooks with error-code mapping, cross-origin cookie credentials, and SessionUser built from the MeResponse.**

## Performance

- **Duration:** ~38 min
- **Started:** 2026-06-04T00:00:00Z
- **Completed:** 2026-06-04T00:38:00Z
- **Tasks:** 2 of 2
- **Files modified:** 8 + 1 created

## Accomplishments

- Replaced the `auth.login('dev-token', {...})` stub in `login-screen.tsx` with the real `useAuthLogin()` mutation; error codes `INVALID_CREDENTIALS`, `ACCOUNT_DISABLED`, and `ACCOUNT_LOCKED`/429 each navigate to the existing banner state via `?error=` search param.
- Added `buildSessionUser(MeResponse): SessionUser` to `auth.ts` — derives name, role, permissions (via `permissionsForRole`), initials (first letters of up to two name words), and `companyName` (scope.company_id for shift_leader scope; undefined for global scope).
- Added `credentials: 'include'` to `customFetch` in `mutator.ts` so the httpOnly refresh cookie is sent/received cross-origin between FE (:4173/:5173) and BE (:8081) on every generated hook.
- Wired logout in `shell.tsx` using `useAuthLogout()`; logout failure (e.g. already-expired session) is swallowed — `auth.clear()` and navigate always run.
- Wired `forgot-password-screen.tsx` to `useAuthForgotPassword()`; always advances to 'sent' even on network error (anti-enumeration per authentication.md C-2).
- Wired `reset-password-screen.tsx` to `useAuthResetPassword()` reading `token` from the typed `/reset-password?token=…` search param; handles `RESET_TOKEN_EXPIRED` (banner) and `WEAK_PASSWORD` (field error); minLength raised from 8→10 to match BE password policy.
- Added `validateSearch` on the `/reset-password` route in `router.tsx` to type `token?: string`.
- Created `.env.development` with `VITE_ENABLE_MSW=true` and `VITE_API_BASE_URL=/api/v1` defaults; Playwright e2e harness (01-05) overrides both.

## Task Commits

1. **Task 1: Wire login+logout to real hooks; build SessionUser; cookie credentials** — `869df86` (feat)
2. **Task 2: Wire forgot-password+reset-password; type reset token param; env files** — `6b6c501` (feat)

## Files Created/Modified

- `frontend/packages/api-client/src/mutator.ts` — added `credentials: 'include'` to `customFetch`
- `frontend/apps/web/src/lib/auth.ts` — added `buildSessionUser(MeResponse)` + `MeResponse` import
- `frontend/apps/web/src/features/auth/login-screen.tsx` — replaced dev-token stub; wired `useAuthLogin()` with error-code banner mapping
- `frontend/apps/web/src/features/auth/forgot-password-screen.tsx` — wired `useAuthForgotPassword()`; always-advance-to-sent pattern
- `frontend/apps/web/src/features/auth/reset-password-screen.tsx` — wired `useAuthResetPassword()` with token from URL; error handling; minLength 10
- `frontend/apps/web/src/app/shell.tsx` — added `useAuthLogout()` + `handleLogout` passed to `UserMenu`
- `frontend/apps/web/src/app/user-menu.tsx` — removed inline `auth.clear()` stub; accepts `onLogout` prop
- `frontend/apps/web/src/app/router.tsx` — added `validateSearch` on `/reset-password` route
- `frontend/apps/web/.env.development` — created with dev defaults (MSW on, API at /api/v1)

## Decisions Made

- **companyName for shift_leader:** Phase 1 has no company-name lookup endpoint; `buildSessionUser` sets `companyName = scope.company_id` (e.g. `"SWP-CMP-0021"`). TODO(Phase-3): resolve via companies endpoint once available.
- **logout architecture:** `useAuthLogout()` lives in `shell.tsx`, not `user-menu.tsx`, to keep `UserMenu` auth-agnostic. `UserMenu` receives `onLogout: () => void` prop. This is cleaner than the original stub which had `auth.clear()` inline in the menu.
- **forgot-password error swallowing:** On network error or 429, still advance to 'sent' — surfacing an error would reveal the difference between a valid email request (202) and a failed request, which defeats the anti-enumeration design.
- **reset minLength 8→10:** The plan specified 10 to match BE policy; the original screen had 8. Updated both the Zod schema and the live `reqMin` flag.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `Checkbox` component uses native `onChange`, not `onCheckedChange`**
- **Found during:** Task 1 (login-screen.tsx wiring)
- **Issue:** Initial draft used `onCheckedChange` (shadcn Checkbox API) but `@swp/ui Checkbox` is a native input wrapper using standard `onChange: React.ChangeEventHandler<HTMLInputElement>`
- **Fix:** Changed to `onChange={(e) => setRememberMe(e.target.checked)}`
- **Files modified:** `login-screen.tsx`
- **Verification:** TypeScript typecheck passed
- **Committed in:** `869df86` (Task 1 commit)

**2. [Rule 1 - Architecture] Logout handler moved to `shell.tsx`, `UserMenu` accepts `onLogout` prop**
- **Found during:** Task 1 (shell.tsx + user-menu.tsx wiring)
- **Issue:** The plan said "shell.tsx: wire logout to useAuthLogout()" but the logout action was inline in `user-menu.tsx`. Keeping `useAuthLogout` in `user-menu.tsx` would leave the hook buried in the dropdown component rather than in the shell that owns the session.
- **Fix:** Added `useAuthLogout()` to `AppShell`, defined `handleLogout()` there, passed it as `onLogout` prop to `UserMenu`. `UserMenu` now just calls `onLogout()` without knowing about the auth hook.
- **Files modified:** `shell.tsx`, `user-menu.tsx`
- **Verification:** TypeScript typecheck passed; `grep useAuthLogout shell.tsx` satisfied the plan's acceptance criteria
- **Committed in:** `869df86` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 minor architectural refinement)
**Impact on plan:** No scope creep. Both fixes improved correctness and design quality.

## Issues Encountered

None — typecheck and build passed cleanly on first attempt after fixes.

## User Setup Required

None — env files provide defaults; no external service configuration required for this plan.

## Next Phase Readiness

- AUTH-01/02/04 FE half complete: real login/forgot/reset/logout against the generated API client.
- The E2E harness (01-05) can now exercise the full login flow against the real BE.
- companyName for shift_leader will show the raw `SWP-CMP-0021` company_id until Phase 3 resolves it — acceptable for the e2e test personas.

---
*Phase: 01-test-harness-auth*
*Completed: 2026-06-04*
