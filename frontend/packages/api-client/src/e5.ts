/**
 * E5 Attendance / Kehadiran — public hook surface (`@swp/api-client/e5`).
 *
 * Hand-authored barrel over the Orval `tags-split` output (one file per tag, no root barrel).
 * Lives OUTSIDE `src/gen` so `clean: true` regeneration never wipes it (ENGINEERING.md E2).
 */
export * from './gen/e5/clock-in-out/clock-in-out.ts';
export * from './gen/e5/attendance-records/attendance-records.ts';
export * from './gen/e5/attendance-verification/attendance-verification.ts';
export * from './gen/e5/corrections/corrections.ts';
export * from './gen/e5/model/index.ts';
