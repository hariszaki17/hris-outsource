# @swp/mobile (placeholder)

React Native app for **agents** (clock-in/out, schedule, leave/OT) and **shift leaders**
(on-site ops). Not yet scaffolded — built after the web console ships.

When built, it reuses the shared packages directly:

- `@swp/api-client` — same Orval-generated hooks + the `customFetch` mutator (RN-compatible).
- `@swp/design-tokens` — the TS token export (`color`, `type`, `space`) for native styling.
- `@swp/shared` — branded IDs, `Asia/Jakarta` date/time helpers, i18n catalogs.

Keeping it in the workspace now means the shared packages stay surface-agnostic from day one.
