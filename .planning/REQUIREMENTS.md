# Requirements: SWP HRIS — Mobile Foundation (Expo Scaffold)

**Defined:** 2026-06-08
**Milestone:** v1.1 — Mobile Foundation
**Core Value:** A real, buildable Expo app exists in the monorepo, consuming the shared
contract/tokens packages, with all MVP-required native capabilities installed — so feature
screens can be built on a proven foundation without re-litigating tooling or forcing reinstalls.

## v1.1 Requirements

Scaffold-only. Feature screens are explicitly deferred (see Out of Scope).

### Scaffold (SCAF)

- [ ] **SCAF-01**: The `frontend/apps/mobile` placeholder is replaced by a real Expo app (managed workflow + dev-client) on the latest stable SDK that boots via Expo for iOS and Android.
- [ ] **SCAF-02**: Navigation uses Expo Router (file-based, typed routes) with at least one route group and a root layout.
- [ ] **SCAF-03**: A minimal smoke screen renders that imports and exercises all three shared packages (proves the wiring, not a feature).

### Monorepo wiring (MONO)

- [ ] **MONO-01**: The mobile package declares `@swp/api-client`, `@swp/shared`, `@swp/design-tokens` as `workspace:*` dependencies and resolves them through pnpm.
- [ ] **MONO-02**: Metro is configured for the pnpm monorepo (watchFolders to the workspace root + nodeModulesPaths / symlink resolution) so workspace packages resolve at bundle time.
- [ ] **MONO-03**: The web-only `@swp/ui` (DOM/shadcn/Tailwind) is NOT imported; a thin RN primitive layer lives inside `apps/mobile` instead, backed by the shared tokens.

### Styling (STYLE)

- [ ] **STYLE-01**: NativeWind is wired and renders Tailwind classes in RN.
- [ ] **STYLE-02**: NativeWind theme is driven by the `@swp/design-tokens` TS export (color/type/space) — no raw hex in app code (mirrors web ENGINEERING.md token rule).

### Native capabilities (NATIVE)

- [ ] **NATIVE-01**: `expo-location` installed + config-plugged (foundation for E5 F5.1 GPS geofence). Permission strings declared.
- [ ] **NATIVE-02**: `expo-notifications` installed + config-plugged (foundation for E10 F10.1 push).
- [ ] **NATIVE-03**: `expo-image-picker` installed + config-plugged (foundation for E6 F6.2 leave-doc upload).
- [ ] **NATIVE-04**: `expo-updates` installed + config-plugged for EAS Update (OTA), with a documented force-update-on-launch gate stub.

### Tooling parity (TOOL)

- [ ] **TOOL-01**: TypeScript strict; `turbo run typecheck` includes mobile and passes.
- [ ] **TOOL-02**: Biome lint includes mobile and passes (web Biome config reused/extended).
- [ ] **TOOL-03**: A documented stub/TODO records the FOLLOW-UP backend `min_supported_version` version-gate contract (no backend change this milestone).

## Future Requirements (deferred to later milestones)

### Feature screens

- **MOB-AGENT**: Clock in/out + geofence (F5.1), my schedule (F4.3), leave request (F6.2), OT request/confirm (F7.2), payslip history (F8.1), attendance correction (F5.4), profile self-service (F2.1), Beranda + notifications (F10.x).
- **MOB-LEADER**: Attendance verification queue (F5.3/F5.4), leave approval L1 (F6), OT approval L1 (F7), SL team dashboard + combined inbox (F10).

## Out of Scope

| Feature | Reason |
|---------|--------|
| Any feature screen / business flow | This milestone is foundation only; screens come once the scaffold is proven. |
| Backend `min_supported_version` endpoint | Follow-up backend contract; mobile leaves a documented stub. |
| EAS build/release pipeline + app store config | Infra milestone; scaffold only proves local boot + bundle. |
| Promoting RN primitives to `@swp/ui` | Premature; promote on 2nd domain-agnostic reuse per ENGINEERING.md. |
| Bare React Native / ejecting | Expo managed chosen; native deps are first-party Expo modules. |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SCAF-01..03 | Phase 12 | Pending |
| MONO-01..03 | Phase 12 | Pending |
| STYLE-01..02 | Phase 12 | Pending |
| NATIVE-01..04 | Phase 12 | Pending |
| TOOL-01..03 | Phase 12 | Pending |
