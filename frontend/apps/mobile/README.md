# @swp/mobile

React Native app (Expo) for **agents** (clock-in/out, schedule, leave/OT, payslip) and
**shift leaders** (on-site approvals). Scaffolded in milestone v1.1; feature screens land in
later milestones.

## Stack

- **Expo** (managed + dev-client, SDK 56) · **Expo Router** (file-based, typed routes).
- **NativeWind** for styling, theme generated from `@swp/design-tokens` (`src/theme/tokens.ts`) —
  tokens only, never raw hex.
- Reused shared packages (surface-agnostic, no changes): `@swp/api-client` (Orval hooks +
  `customFetch`), `@swp/shared` (branded IDs, `Asia/Jakarta` datetime, i18n),
  `@swp/design-tokens`. The web `@swp/ui` is DOM/shadcn and is **not** reused — `src/ui/*`
  holds the parallel RN primitive layer (promote to a shared package on 2nd reuse).
- Native modules locked up front (minimize future forced store reinstalls): `expo-location`
  (E5 F5.1 geofence), `expo-notifications` (E10 F10.1 push), `expo-image-picker` (E6 F6.2
  leave docs), `expo-updates` (OTA + force-update gate — see `src/lib/update-gate.ts`).

## Monorepo notes

- Metro is configured for the pnpm workspace (`metro.config.js`: `watchFolders` +
  `nodeModulesPaths`). The frontend `.npmrc` sets `node-linker=hoisted` — Expo/RN require a
  hoisted layout; pnpm's default symlinked isolation breaks Metro + native autolinking.

## Follow-ups (not in v1.1 scaffold)

- EAS Update URL + EAS build/release pipeline.
- Backend `min_supported_version` version-gate contract (drives the hard store-update gate;
  stubbed in `src/lib/update-gate.ts`).

## Run

```bash
pnpm --filter @swp/mobile start      # Expo dev server
pnpm --filter @swp/mobile typecheck  # tsc --noEmit (also via turbo run typecheck)
pnpm --filter @swp/mobile doctor     # expo-doctor
```
