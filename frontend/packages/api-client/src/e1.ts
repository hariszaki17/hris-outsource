/**
 * E1 Foundations — public hook surface (`@swp/api-client/e1`).
 *
 * Hand-authored barrel over the Orval `tags-split` output: that mode emits one file per tag
 * (`gen/e1/<tag>/<tag>.ts`) but NO root barrel, so this re-export is the stable import point.
 * It lives OUTSIDE `src/gen` so `clean: true` regeneration never wipes it. Generated files
 * themselves stay untouched (ENGINEERING.md E2).
 */
export * from './gen/e1/authentication/authentication.ts';
export * from './gen/e1/users/users.ts';
export * from './gen/e1/audit-log/audit-log.ts';
export * from './gen/e1/platform/platform.ts';
export * from './gen/e1/model/index.ts';
