# Roadmap: SWP HRIS backend + full-stack E2E

## Milestones

- ✅ **v1.0 — Backend + Full-Stack E2E** — Phases 1–11 (shipped 2026-06-05)
- 🔨 **v1.1 — Mobile Foundation (Expo Scaffold)** — Phase 12 (started 2026-06-08)

## Phases

### 🔨 v1.1 — Mobile Foundation (Expo Scaffold)

Scaffold-only milestone: a real, buildable Expo app in `frontend/apps/mobile` consuming the
shared contract/tokens packages, with all MVP-required native capabilities installed. No
feature screens. Built in worktree `feat/mobile-scaffold`.

- [ ] **Phase 12: Expo Scaffold** (0/1) — Replace the `apps/mobile` placeholder with a real
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
