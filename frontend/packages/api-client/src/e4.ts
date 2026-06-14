/**
 * E4 Shift Scheduling / Jadwal — public hook surface (`@swp/api-client/e4`).
 *
 * Hand-authored barrel over the Orval `tags-split` output (one file per tag, no root barrel).
 * Lives OUTSIDE `src/gen` so `clean: true` regeneration never wipes it (ENGINEERING.md E2).
 */
export * from './gen/e4/shift-masters/shift-masters.ts';
export * from './gen/e4/schedule/schedule.ts';
export * from './gen/e4/model/index.ts';
