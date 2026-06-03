/**
 * RBAC role model — the four internal roles (EPICS.md §3, CONVENTIONS.md `x-rbac`).
 * The web console is used by the three staff roles; `agent` is the mobile surface only.
 *
 * NOTE (ENGINEERING.md A2): the authoritative *per-operation* permission map is generated
 * from the `x-rbac` OpenAPI extension into `@swp/api-client` (a post-gen step, not yet built).
 * Until that lands, coarse module/nav visibility is hand-authored in the web app
 * (`apps/web/src/app/nav.ts`) against this type, and will be superseded by the generated map.
 */
export const ROLES = ['super_admin', 'hr_admin', 'shift_leader', 'agent'] as const;
export type Role = (typeof ROLES)[number];

/** Roles that sign in to the web console (`agent` is mobile-only). */
export const WEB_ROLES = [
  'super_admin',
  'hr_admin',
  'shift_leader',
] as const satisfies readonly Role[];
export type WebRole = (typeof WEB_ROLES)[number];
