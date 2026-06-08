# Roadmap: SWP HRIS backend + full-stack E2E

## Milestones

- ✅ **v1.0 — Backend + Full-Stack E2E** — Phases 1–11 (shipped 2026-06-05)
- ✅ **v1.1 — Mobile Foundation (Expo Scaffold)** — Phase 12 (shipped 2026-06-08)
- 📋 **v1.2 — Mobile MVP (Agent App)** — Phases 13–20 (planned 2026-06-08)

## Phases

### 📋 v1.2 — Mobile MVP (Agent App)

Full-stack vertical slices: each phase ships the Go endpoint(s) + the RN screen(s) together,
end-to-end against the real backend. Agent persona only — the shift-leader app is a later
milestone (its endpoints already exist). Builds on the v1.1 Expo scaffold.

Backend effort legend: **READY** (agent-callable today) · **OPEN-ROUTE** (exists, just
excludes agent — open route + self-filter) · **NEW** (build the handler).

- [ ] **Phase 13: App shell + auth + Beranda** (SHELL-01..04) — backend READY.
  Secure-storage session, tab nav + force-update gate hook, role-shaped `dashboards/me`,
  notifications list/mark-read. Foundation every other screen sits on.
  - Success: agent logs in on a device, session survives restart, Beranda + notifications render off the real BE.
- [ ] **Phase 14: Clock in/out + geofence + my-attendance** (CLOCK-01..03, ATTEND-01..02) — backend NEW + OPEN-ROUTE. The killer loop.
  BE: `POST /attendance:clock-in|:clock-out|:photo` + server geofence; open `GET /attendance` to agent self.
  FE: GPS clock screen (all variants) + my-attendance history/detail.
  - Success: agent clocks in inside/outside the site geofence (flagged, not blocked) and sees the record in history.
- [ ] **Phase 15: Attendance correction** (ATTEND-03..04) — backend NEW.
  BE: agent-scoped `POST /corrections` (7-day window, re-eval). FE: file-correction form + status.
  - Success: agent files a check-in correction on an Absent/wrong record; it appears pending for the SL.
- [ ] **Phase 16: My schedule** (SCHED-01..02) — backend OPEN-ROUTE.
  BE: open `GET /schedule` to agent self. FE: week view + shift detail + reminder hook.
  - Success: agent sees their own upcoming shifts for the week.
- [ ] **Phase 17: Leave request** (LEAVE-01..02) — backend NEW.
  BE: agent `POST /leave-requests` + own list + attachment, quota/schedule validation, SL→HR routing.
  FE: leave form (type/dates/delegate/doc upload) + own-requests status.
  - Success: agent submits a leave request with a document; it enters the SL approval queue.
- [ ] **Phase 18: Overtime request/confirm** (OT-01..02) — backend NEW.
  BE: agent `POST /overtime` + `:confirm`. FE: request + confirm-auto-detected + detail.
  - Success: agent requests OT (or confirms auto-detected); it routes to approval.
- [ ] **Phase 19: Payslip history** (PAY-01..02) — backend OPEN-ROUTE.
  BE: open `GET /payslips` to agent self. FE: payslip history + summary (read-only).
  - Success: agent views their own payslip summaries (take-home/gross/paid date).
- [ ] **Phase 20: Profile self-service** (PROFILE-01..02) — read OPEN-ROUTE + change-request NEW.
  BE: agent self-read `GET /employees/{id}` + `POST /change-requests`. FE: profile view/limited-edit → change-request + Pengaturan.
  - Success: agent edits phone/bank → a change-request lands for HR; password change + logout work.

### ✅ v1.1 — Mobile Foundation (Expo Scaffold)

Scaffold-only milestone: a real, buildable Expo app in `frontend/apps/mobile` consuming the
shared contract/tokens packages, with all MVP-required native capabilities installed. No
feature screens. Built in worktree `feat/mobile-scaffold`.

- [x] **Phase 12: Expo Scaffold** (1/1) — Replace the `apps/mobile` placeholder with a real
  Expo app (latest SDK, managed + dev-client, Expo Router) wired into the pnpm/Turborepo
  monorepo (Metro workspace resolution), consuming `@swp/api-client` + `@swp/shared` +
  `@swp/design-tokens`; NativeWind driven by the shared tokens; `expo-location` /
  `expo-notifications` / `expo-image-picker` / `expo-updates` installed + config-plugged;
  TS strict + Biome + turbo typecheck/lint green for mobile.
  - Requirements: SCAF-01..03, MONO-01..03, STYLE-01..02, NATIVE-01..04, TOOL-01..03
  - Success criteria:
    1. `pnpm install` resolves; `frontend/apps/mobile` builds a Metro bundle (no missing-module errors for workspace packages).
    2. Smoke screen renders using a value from each of the three shared packages.
    3. `turbo run typecheck` (incl. mobile) and Biome lint are green.
    4. The four native modules are present in `app.json` plugins and install cleanly.
    5. `expo-doctor` reports no blocking issues.



<details>
<summary>✅ v1.0 — Backend + Full-Stack E2E (Phases 1–11) — SHIPPED 2026-06-05</summary>

Every FE-used endpoint across all 11 epics implemented behind the locked OpenAPI contracts,
proven by exhaustive full-stack Playwright E2E (real FE ↔ real Go API ↔ ephemeral Postgres;
final suite 239 passed / 6 skipped / 0 failed).

- [x] Phase 1: Test Harness + Auth (5/5) — completed 2026-06-04
- [x] Phase 2: E1 Foundations (4/4) — completed 2026-06-04
- [x] Phase 3: E2 Org & Master Data (6/6) — completed 2026-06-04
- [x] Phase 4: E2 People (6/6) — completed 2026-06-04
- [x] Phase 5: E3 Placement (4/4) — completed 2026-06-04
- [x] Phase 6: E4 Schedule & Shifts (4/4) — completed 2026-06-04
- [x] Phase 7: E5 Attendance (4/4) — completed 2026-06-05
- [x] Phase 8: E6 Leave (4/4) — completed 2026-06-05
- [x] Phase 9: E7 Overtime (4/4) — completed 2026-06-05
- [x] Phase 10: E8 Payroll (4/4) — completed 2026-06-05
- [x] Phase 11: E10 Reporting & Notifications (5/5) — completed 2026-06-05

Full detail: [`milestones/v1.0-ROADMAP.md`](milestones/v1.0-ROADMAP.md) ·
Audit: [`milestones/v1.0-MILESTONE-AUDIT.md`](milestones/v1.0-MILESTONE-AUDIT.md)

</details>

### 📋 v1.1 (next milestone — planned)

Start with `/gsd:new-milestone` (questioning → research → requirements → roadmap). Candidate
themes carried forward from the v1.0 audit tech debt:
- Notification dispatch coverage beyond leave/OT/attendance (placement, payroll, change-requests, quotas).
- PDF export (currently `EXPORT_FORMAT_UNSUPPORTED`).
- E9 migration (legacy MySQL `lumen_swp` → Postgres), mobile (React Native) surface, production infra/CI/CD.

## Progress

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Test Harness + Auth | v1.0 | 5/5 | Complete | 2026-06-04 |
| 2. E1 Foundations | v1.0 | 4/4 | Complete | 2026-06-04 |
| 3. E2 Org & Master Data | v1.0 | 6/6 | Complete | 2026-06-04 |
| 4. E2 People | v1.0 | 6/6 | Complete | 2026-06-04 |
| 5. E3 Placement | v1.0 | 4/4 | Complete | 2026-06-04 |
| 6. E4 Schedule & Shifts | v1.0 | 4/4 | Complete | 2026-06-04 |
| 7. E5 Attendance | v1.0 | 4/4 | Complete | 2026-06-05 |
| 8. E6 Leave | v1.0 | 4/4 | Complete | 2026-06-05 |
| 9. E7 Overtime | v1.0 | 4/4 | Complete | 2026-06-05 |
| 10. E8 Payroll | v1.0 | 4/4 | Complete | 2026-06-05 |
| 11. E10 Reporting & Notifications | v1.0 | 5/5 | Complete | 2026-06-05 |
