/**
 * E2 Identity / Karyawan & Master Data — public hook surface (`@swp/api-client/e2`).
 *
 * Hand-authored barrel over the Orval `tags-split` output (one file per tag, no root barrel).
 * Lives OUTSIDE `src/gen` so `clean: true` regeneration never wipes it. Generated files stay
 * untouched (ENGINEERING.md E2).
 */
export * from './gen/e2/employees/employees.ts';
export * from './gen/e2/agreements/agreements.ts';
export * from './gen/e2/client-companies/client-companies.ts';
export * from './gen/e2/sites/sites.ts';
export * from './gen/e2/people/people.ts';
export * from './gen/e2/master-data/master-data.ts';
export * from './gen/e2/model/index.ts';

// NOTE — profile change-requests removed 2026-06-14 (EPICS §8 E11): agent self-edits
// (phone / emergency contact / bank account) are now INSTANT via PATCH /me/profile.
// The `change-requests` tag, paths, and hooks were deleted from the E2 spec and regen,
// so this barrel no longer re-exports any change-request surface.
