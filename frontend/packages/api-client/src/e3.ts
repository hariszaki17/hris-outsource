/**
 * E3 Placement / Penempatan — public hook surface (`@swp/api-client/e3`).
 *
 * Hand-authored barrel over the Orval `tags-split` output (one file per tag, no root barrel).
 * Lives OUTSIDE `src/gen` so `clean: true` regeneration never wipes it (ENGINEERING.md E2).
 */
export * from './gen/e3/placements/placements.ts';
export * from './gen/e3/shift-leader-assignments/shift-leader-assignments.ts';
export * from './gen/e3/client-companies/client-companies.ts';
export * from './gen/e3/model/index.ts';
