/**
 * lib/reset-db.ts
 *
 * resetDb() — truncate ephemeral app tables and re-apply the seed so every spec
 * starts from the same deterministic state.
 *
 * Chosen mechanism: TRUNCATE app tables (preserving `users` and `id_counters`
 * which contain the seeded personas), then call the Go seed command again.
 * The seed is idempotent (skips existing users), so re-running it after
 * truncate restores any test-deleted users and resets token tables cleanly.
 *
 * Why not per-test transactions?
 *   Playwright specs run against a real HTTP server; wrapping HTTP-driven DB
 *   writes in a test transaction is impractical without test-only hooks in the
 *   API. Truncate-and-reseed is the cheapest robust option at this scale.
 *
 * Why keep `users`?
 *   The seed uses "skip if exists" logic. If a test deletes a user, the TRUNCATE
 *   removes them from `users` but the next resetDb() re-seeds them (seed is
 *   called after truncate below). So users are always restored.
 *
 * Documented in README.md §DB isolation.
 */

import * as path from 'node:path';
import * as fs from 'node:fs';
import * as os from 'node:os';
import { execFileSync, execSync } from 'node:child_process';
import pg from 'pg';

const { Client } = pg;

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------
const REPO_ROOT = path.resolve(import.meta.dirname, '../../..');
const BACKEND_DIR = path.join(REPO_ROOT, 'backend');
const E2E_DIR = path.join(REPO_ROOT, 'frontend', 'e2e');
const ENV_FILE = path.join(E2E_DIR, '.env.e2e');

// Prebuilt seed binary compiled ONCE by globalSetup (lib/backend.ts SEED_BIN).
// Reseeding via this binary avoids a `go run ./cmd/seed` toolchain recompile on every
// beforeEach (~250×/full run), which otherwise thrashes the machine. Keep in sync with
// lib/backend.ts SEED_BIN.
const SEED_BIN = path.join(
  os.tmpdir(),
  process.platform === 'win32' ? 'swp-e2e-seed.exe' : 'swp-e2e-seed',
);

// ---------------------------------------------------------------------------
// Parse .env.e2e (minimal — only need DATABASE_URL)
// ---------------------------------------------------------------------------
/** Parse .env.e2e into a flat map (comments + blank lines skipped). */
function loadEnv(): Record<string, string> {
  if (!fs.existsSync(ENV_FILE)) {
    throw new Error(`[reset-db] .env.e2e not found at ${ENV_FILE}`);
  }
  const out: Record<string, string> = {};
  for (const line of fs.readFileSync(ENV_FILE, 'utf8').split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const eq = trimmed.indexOf('=');
    if (eq === -1) continue;
    out[trimmed.slice(0, eq).trim()] = trimmed.slice(eq + 1).trim();
  }
  return out;
}

function getDbUrl(): string {
  const url = loadEnv().DATABASE_URL;
  if (!url) throw new Error('[reset-db] DATABASE_URL not found in .env.e2e');
  return url;
}

// ---------------------------------------------------------------------------
// App tables to truncate (order matters for FK constraints; most restrictive first)
// Tables NOT listed here are kept (users, id_counters).
// Update this list as new phases add tables.
// ---------------------------------------------------------------------------
const TRUNCATE_TABLES = [
  'idempotency_keys',
  'password_reset_tokens',
  'refresh_tokens',
  'audit_log',
  // Phase 3: E2 org/master-data tables (FK order: most-dependent first)
  'positions',
  'service_lines',
  'client_sites',
  'client_companies',
  'leave_types',
  'attendance_codes',
  'overtime_rules',
  // Phase 10: E8 payroll tables (FK order: most-dependent first).
  // payslip_audit_notes / payslip_components / payslip_benefits FK to payslips;
  // payslips FK to employees / placements / attendance. All MUST be truncated before
  // placements + employees below. export_jobs has NO FK to kept tables (requested_by_id
  // is a plain text id) — clearing it resets any test-created export job rows so the
  // export E2E never sees a stale DONE row leak into the next spec.
  // IMPORTANT: do NOT truncate River's own tables (river_job / river_*); leaving them
  // intact lets the running worker keep its queue. The Go seed re-applies the
  // SWP-PS-90121..90124 FINAL fixtures + the SWP-PS-90119 DECRYPT_FAIL row + its two
  // audit notes (ON CONFLICT DO NOTHING; breakdown lines only on fresh insert).
  'payslip_audit_notes',
  'payslip_benefits',
  'payslip_components',
  'payslips',
  'export_jobs',
  // Phase 11: E10 reporting. `notifications` has NO FK to kept tables (recipient_id
  // is a plain text SWP-EMP-*/SWP-USR-* id), so its position is flexible — kept here
  // with the E10/export group. TRUNCATE clears any test-dispatched rows (from the
  // capstone leave/OT approval auto-dispatch) so they never leak into the next spec;
  // the Go seed re-applies the SWP-NTF-9000x fixtures (ON CONFLICT DO NOTHING). The
  // export_jobs row above is also re-cleared so report-export E2E never sees a stale
  // DONE/CANCELLED row. River internal tables (river_*) are left intact (see note above).
  'notifications',
  // Phase 7: E5 attendance tables (FK order: most-dependent first).
  // attendance_corrections FK to attendance; attendance FK to schedule_entries /
  // placements / employees / client_companies. Both MUST be truncated before
  // schedule_entries + placements + employees below. Reseeded by the Go seed.
  'attendance_corrections',
  'attendance',
  // Phase 8: E6 leave tables (FK order: most-dependent first).
  // leave_approvals FK to leave_requests; leave_requests FK to employees /
  // leave_types / placements / client_companies; leave_quotas FK to employees /
  // leave_types. All MUST be truncated before schedule_entries + placements +
  // employees below. The INV-3 approval write-through inserts approved_leave_days
  // rows + flips schedule_entries.status — both are in this list already, so a
  // reset clears any test-approved leave so it never leaks into the next spec;
  // the Go seed re-applies the SWP-LR-8001..8007 / SWP-LQ-8001/8002 fixtures.
  'leave_approvals',
  'leave_requests',
  'leave_quotas',
  // Phase 9: E7 overtime tables (FK order: most-dependent first).
  // overtime_approvals FK to overtime; overtime FK to attendance / schedule_entries /
  // placements / employees / client_companies / holidays. Both MUST be truncated before
  // schedule_entries + placements + employees + holidays below. holidays FK is inbound
  // from overtime.holiday_id (SWP-OT-30009 → SWP-HOL-9001), so overtime is listed BEFORE
  // holidays. The Go seed re-applies the SWP-OT-3000x + SWP-HOL-900x fixtures (09-02).
  'overtime_approvals',
  'overtime',
  'holidays',
  // Phase 6: E4 scheduling tables (FK order: most-dependent first).
  // schedule_entries / approved_leave_days FK to placements/employees/shift_masters,
  // so they MUST be truncated before placements + employees below. shift_masters has
  // no inbound FK from kept tables. Reseeded by `go run ./cmd/seed` afterwards.
  'schedule_entries',
  'approved_leave_days',
  'shift_masters',
  // Phase 5: E3 placement tables (FK order: most-dependent first).
  // Truncated BEFORE people tables so placement FKs to employees/agreements drop cleanly.
  'placement_history',
  'shift_leader_assignments',
  'placements',
  // Phase 4: E2 people tables (FK order: most-dependent first)
  'change_requests',
  'agreement_attachments',
  'employment_agreements',
  'employees',
];

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * resetDb() — truncate app tables and re-seed personas. Call in beforeEach or
 * beforeAll in specs that need a clean slate.
 */
export async function resetDb(): Promise<void> {
  const env = loadEnv();
  const dbUrl = env.DATABASE_URL;
  if (!dbUrl) throw new Error('[reset-db] DATABASE_URL not found in .env.e2e');
  const client = new Client({ connectionString: dbUrl });
  await client.connect();

  try {
    // Truncate transactionally so no partial state on failure.
    await client.query('BEGIN');
    for (const table of TRUNCATE_TABLES) {
      await client.query(`TRUNCATE TABLE ${table} CASCADE`);
    }
    // Also clear users so the seed re-creates them fresh (handles test-deleted users).
    await client.query('TRUNCATE TABLE users CASCADE');
    await client.query('COMMIT');
  } catch (err) {
    await client.query('ROLLBACK');
    throw err;
  } finally {
    await client.end();
  }

  // Re-apply the seed so personas are available again. Pass the FULL .env.e2e so the
  // seed sees PAYROLL_ENCRYPTION_KEY (E8) — without it seedPayroll skips entirely and the
  // payslip fixtures never come back after the TRUNCATE above, breaking the e8 specs.
  const envForSeed: NodeJS.ProcessEnv = {
    ...process.env,
    ...env,
    DATABASE_URL: dbUrl,
    ENV: 'test',
  };
  // Prefer the prebuilt binary (globalSetup compiled it once); fall back to `go run`
  // only if it's missing (e.g. a spec run outside the full harness).
  if (fs.existsSync(SEED_BIN)) {
    execFileSync(SEED_BIN, [], {
      cwd: BACKEND_DIR,
      env: envForSeed,
      stdio: 'inherit',
    });
  } else {
    execSync('go run ./cmd/seed', {
      cwd: BACKEND_DIR,
      env: envForSeed,
      stdio: 'inherit',
    });
  }
}
