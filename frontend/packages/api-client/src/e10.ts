/**
 * E10 Reporting & Notifications — public hook surface (`@swp/api-client/e10`). Hand-authored
 * barrel over the Orval `tags-split` output (no root barrel emitted), outside `src/gen` so regen
 * does not wipe it. Typed react-query hooks + MSW mocks; Zod deferred (WEB-STACK §4).
 */
export * from './gen/e10/dashboards/dashboards.ts';
export * from './gen/e10/notifications/notifications.ts';
export * from './gen/e10/reports/reports.ts';
export * from './gen/e10/exports/exports.ts';
export * from './gen/e10/model/index.ts';
