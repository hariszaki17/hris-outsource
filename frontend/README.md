# hris-outsource — frontend

Monorepo for SWP's HRIS Outsource frontend. Stack and rules are **authoritative** in
[`docs/eng/WEB-STACK.md`](../docs/eng/WEB-STACK.md) and [`docs/eng/ENGINEERING.md`](../docs/eng/ENGINEERING.md).

## Layout

```
apps/
  web/                 Vite + React + TS SPA (admin / HR / shift leader console)
  mobile/              React Native (agents + leaders) — placeholder
packages/
  api-client/          Orval-generated TanStack Query hooks + Zod + MSW (from docs/api/*/openapi.yaml)
  ui/                  shadcn primitives + molecules, themed to design tokens
  design-tokens/       DESIGN-SYSTEM.md tokens → Tailwind theme + TS export
  shared/              branded SWP-* ID types, Asia/Jakarta tz helpers, i18n catalogs
```

## Commands

```bash
pnpm install          # install workspace deps
pnpm gen              # regenerate the typed API client from docs/api/*/openapi.yaml (Orval)
pnpm dev              # run apps/web dev server
pnpm typecheck        # type-check all packages
pnpm test             # vitest across packages
pnpm lint             # biome lint + format check
pnpm --filter @swp/web e2e   # playwright E2E
```

## Golden rules (see ENGINEERING.md)

- **Never hand-edit** `packages/api-client/src/gen/**` — change the spec, run `pnpm gen`.
- **Tokens over literals** — no raw hex; use the design-token theme.
- **Reuse before building** — compose from `packages/ui`; promote a domain-agnostic component on its 2nd use.
- **Cite spec IDs** (`F#`/`BR-#`/`INV-#`/`C-#`) in commits.
