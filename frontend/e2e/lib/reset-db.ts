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
import { execSync } from 'node:child_process';
import pg from 'pg';

const { Client } = pg;

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------
const REPO_ROOT = path.resolve(import.meta.dirname, '../../..');
const BACKEND_DIR = path.join(REPO_ROOT, 'backend');
const E2E_DIR = path.join(REPO_ROOT, 'frontend', 'e2e');
const ENV_FILE = path.join(E2E_DIR, '.env.e2e');

// ---------------------------------------------------------------------------
// Parse .env.e2e (minimal — only need DATABASE_URL)
// ---------------------------------------------------------------------------
function getDbUrl(): string {
  if (!fs.existsSync(ENV_FILE)) {
    throw new Error(`[reset-db] .env.e2e not found at ${ENV_FILE}`);
  }
  const lines = fs.readFileSync(ENV_FILE, 'utf8').split('\n');
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith('DATABASE_URL=')) {
      return trimmed.slice('DATABASE_URL='.length).trim();
    }
  }
  throw new Error('[reset-db] DATABASE_URL not found in .env.e2e');
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
  const dbUrl = getDbUrl();
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

  // Re-apply the seed so personas are available again.
  const envForSeed: NodeJS.ProcessEnv = {
    ...process.env,
    DATABASE_URL: dbUrl,
    ENV: 'test',
  };
  execSync('go run ./cmd/seed', {
    cwd: BACKEND_DIR,
    env: envForSeed,
    stdio: 'inherit',
  });
}
