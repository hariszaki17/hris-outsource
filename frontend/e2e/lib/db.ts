/**
 * lib/db.ts
 *
 * Test DB helpers for the E2E auth suite. Uses `pg` with the same DATABASE_URL
 * as reset-db.ts (postgres://hris:hris@localhost:5433/hris_e2e?sslmode=disable).
 *
 * Helpers:
 *   seedResetToken(email, plaintext)     — insert a fresh (1h TTL) password_reset_tokens row
 *   seedExpiredResetToken(email, plain)  — insert an already-expired row (TTL = now-1h)
 *   disableUser(email)                   — SET status='disabled' for the ACCOUNT_DISABLED test
 *   getLastLoginAt(email)               — SELECT last_login_at for the AU-3 assertion
 *   countResetTokensFor(email)           — COUNT rows in password_reset_tokens for C-2 assertion
 *
 * Reset-token mechanism (per 01-03 SUMMARY):
 *   The token_hash stored in password_reset_tokens is sha256(hex(plaintext)).
 *   The BE never emits the plaintext (no mailer in Phase 1). The E2E harness
 *   inserts a known (plaintext → hash) pair directly and presents the plaintext
 *   to the browser.  This is the "seed a known token" approach documented in
 *   01-03 SUMMARY §Reset Token Lifetime and E2E Token Access.
 *
 * All functions open a fresh client, execute the query, and close. This is safe
 * for the serial test suite (fullyParallel=false, workers=1). A shared Pool
 * could be used if connection overhead becomes measurable.
 */

import * as crypto from 'node:crypto';
import * as path from 'node:path';
import * as fs from 'node:fs';
import pg from 'pg';

const { Client } = pg;

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const E2E_DIR = path.resolve(import.meta.dirname, '..');
const ENV_FILE = path.join(E2E_DIR, '.env.e2e');

function getDbUrl(): string {
  if (!fs.existsSync(ENV_FILE)) {
    throw new Error(`[db] .env.e2e not found at ${ENV_FILE}`);
  }
  const lines = fs.readFileSync(ENV_FILE, 'utf8').split('\n');
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith('DATABASE_URL=')) {
      return trimmed.slice('DATABASE_URL='.length).trim();
    }
  }
  throw new Error('[db] DATABASE_URL not found in .env.e2e');
}

async function withClient<T>(fn: (client: pg.Client) => Promise<T>): Promise<T> {
  const client = new Client({ connectionString: getDbUrl() });
  await client.connect();
  try {
    return await fn(client);
  } finally {
    await client.end();
  }
}

// ---------------------------------------------------------------------------
// sha256 hex helper (matches the BE's hash algorithm)
// ---------------------------------------------------------------------------

function sha256Hex(plaintext: string): string {
  return crypto.createHash('sha256').update(plaintext, 'utf8').digest('hex');
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * seedResetToken — insert a valid (expires 1 hour from now) password_reset_tokens row.
 *
 * Looks up the user by email, then inserts the row with token_hash = sha256(plaintext).
 * Any existing token for this user is deleted first (simplicity; no ON CONFLICT needed
 * since token_hash is UNIQUE and we control the plaintext).
 * Returns the plaintext so the caller can pass it to the browser.
 *
 * @param email     The user's email address (case-insensitive lookup).
 * @param plaintext The known plaintext token to seed (e.g. "test-reset-token-001").
 * @returns         The same plaintext (for passing to the reset URL).
 */
export async function seedResetToken(email: string, plaintext: string): Promise<string> {
  const tokenHash = sha256Hex(plaintext);
  await withClient(async (client) => {
    // Look up the user id.
    const userRes = await client.query<{ id: string }>(
      'SELECT id FROM users WHERE lower(email) = lower($1)',
      [email],
    );
    if (userRes.rows.length === 0) {
      throw new Error(`[seedResetToken] No user found with email: ${email}`);
    }
    const userId = userRes.rows[0].id;

    // Remove any stale tokens for this user so UNIQUE on token_hash doesn't bite us.
    await client.query('DELETE FROM password_reset_tokens WHERE user_id = $1', [userId]);

    // Insert the fresh token (expires 1 hour from now).
    await client.query(
      `INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
       VALUES ($1, $2, now() + interval '1 hour')`,
      [userId, tokenHash],
    );
  });
  return plaintext;
}

/**
 * seedExpiredResetToken — same as seedResetToken but with expires_at = now() - 1 hour.
 * Used by the "AU-4 · reset with an expired token" scenario.
 *
 * @param email     The user's email address.
 * @param plaintext The known plaintext token to seed.
 * @returns         The same plaintext.
 */
export async function seedExpiredResetToken(email: string, plaintext: string): Promise<string> {
  const tokenHash = sha256Hex(plaintext);
  await withClient(async (client) => {
    const userRes = await client.query<{ id: string }>(
      'SELECT id FROM users WHERE lower(email) = lower($1)',
      [email],
    );
    if (userRes.rows.length === 0) {
      throw new Error(`[seedExpiredResetToken] No user found with email: ${email}`);
    }
    const userId = userRes.rows[0].id;

    await client.query('DELETE FROM password_reset_tokens WHERE user_id = $1', [userId]);

    // Insert an already-expired token (1 hour in the past).
    await client.query(
      `INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
       VALUES ($1, $2, now() - interval '1 hour')`,
      [userId, tokenHash],
    );
  });
  return plaintext;
}

/**
 * disableUser — set a user's status to 'disabled' for the ACCOUNT_DISABLED test.
 *
 * @param email The user's email address (case-insensitive).
 */
export async function disableUser(email: string): Promise<void> {
  await withClient(async (client) => {
    const result = await client.query(
      "UPDATE users SET status = 'disabled' WHERE lower(email) = lower($1)",
      [email],
    );
    if ((result.rowCount ?? 0) === 0) {
      throw new Error(`[disableUser] No user found with email: ${email}`);
    }
  });
}

/**
 * getLastLoginAt — return the last_login_at timestamp for a user, or null if never logged in.
 *
 * Used by the AU-3 assertion: verify last_login_at is null before login, non-null after.
 *
 * @param email The user's email address (case-insensitive).
 * @returns     ISO string timestamp, or null.
 */
export async function getLastLoginAt(email: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ last_login_at: Date | null }>(
      'SELECT last_login_at FROM users WHERE lower(email) = lower($1)',
      [email],
    );
    if (res.rows.length === 0) {
      throw new Error(`[getLastLoginAt] No user found with email: ${email}`);
    }
    const ts = res.rows[0].last_login_at;
    return ts ? ts.toISOString() : null;
  });
}

/**
 * countResetTokensFor — count how many password_reset_tokens rows exist for a given email.
 *
 * Used by the C-2 assertion: unknown email → forgot-password call → zero rows created.
 *
 * @param email The email address to count tokens for (may not exist as a user).
 * @returns     The count (0 if no matching user or no tokens).
 */
export async function countResetTokensFor(email: string): Promise<number> {
  return withClient(async (client) => {
    const res = await client.query<{ cnt: string }>(
      `SELECT COUNT(*) AS cnt
         FROM password_reset_tokens prt
         JOIN users u ON u.id = prt.user_id
        WHERE lower(u.email) = lower($1)`,
      [email],
    );
    return parseInt(res.rows[0]?.cnt ?? '0', 10);
  });
}

// ---------------------------------------------------------------------------
// E1 foundations helpers (added in Phase 02-04)
// ---------------------------------------------------------------------------

/**
 * getUserStatus — return the status ('active' | 'disabled') for a user, or null if not found.
 *
 * Used to assert both sides of deactivate → reactivate flows.
 *
 * @param email The user's email address (case-insensitive).
 * @returns     'active' | 'disabled' | null
 */
export async function getUserStatus(email: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ status: string }>(
      'SELECT status FROM users WHERE lower(email) = lower($1)',
      [email],
    );
    return res.rows[0]?.status ?? null;
  });
}

/**
 * getUserRole — return the role for a user, or null if not found.
 *
 * Used to assert DB-side after change-role operations.
 *
 * @param email The user's email address (case-insensitive).
 * @returns     'super_admin' | 'hr_admin' | 'shift_leader' | 'agent' | null
 */
export async function getUserRole(email: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ role: string }>(
      'SELECT role FROM users WHERE lower(email) = lower($1)',
      [email],
    );
    return res.rows[0]?.role ?? null;
  });
}

/**
 * countAuditRowsByEntityType — count rows in audit_log where entity_type = $1.
 *
 * Used to verify that mutations write audit records (e.g. after change-role).
 *
 * @param entityType The entity_type to count (e.g. 'user', 'placement').
 * @returns          The row count.
 */
export async function countAuditRowsByEntityType(entityType: string): Promise<number> {
  return withClient(async (client) => {
    const res = await client.query<{ cnt: string }>(
      'SELECT COUNT(*) AS cnt FROM audit_log WHERE entity_type = $1',
      [entityType],
    );
    return parseInt(res.rows[0]?.cnt ?? '0', 10);
  });
}

/**
 * getLatestAuditAction — return the most recent action value from audit_log, or null.
 *
 * Used to verify the exact action recorded after a mutation (e.g. 'user.change_role').
 *
 * @returns The action string of the most recent audit_log row, or null if table is empty.
 */
export async function getLatestAuditAction(): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ action: string }>(
      'SELECT action FROM audit_log ORDER BY created_at DESC LIMIT 1',
    );
    return res.rows[0]?.action ?? null;
  });
}

/**
 * insertAuditRows — insert N synthetic audit_log rows for pagination tests.
 *
 * Each row is a 'user.create' action targeting a synthetic entity id so it
 * fills the page without conflicting with real seeded rows.
 * Columns match migration 00004: before_state/after_state (jsonb), id via swp_next_id('AL').
 *
 * @param count How many rows to insert.
 */
export async function insertAuditRows(count: number): Promise<void> {
  await withClient(async (client) => {
    for (let i = 0; i < count; i++) {
      // Use a synthetic entity_id (no FK) — audit_log has no FK on entity_id.
      // The id column is a generated SWP-AL-<N> using the id_counters sequence.
      await client.query(
        `INSERT INTO audit_log (id, actor_user_id, actor_role, action, entity_type, entity_id, before_state, after_state, request_id)
         VALUES ('SWP-AL-' || swp_next_id('AL'), NULL, NULL, 'user.create', 'user', $1, NULL, '{}'::jsonb, 'req-synthetic-' || $2::text)`,
        [`SWP-USR-SYNTH-${i.toString().padStart(5, '0')}`, i],
      );
    }
  });
}

// ---------------------------------------------------------------------------
// E2 org/master-data verification helpers (added in Phase 03-06)
// ---------------------------------------------------------------------------

/**
 * getCompanyStatus — return the status ('active' | 'inactive') for a client company by id,
 * or null if not found.
 */
export async function getCompanyStatus(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ status: string }>(
      'SELECT status FROM client_companies WHERE id = $1',
      [id],
    );
    return res.rows[0]?.status ?? null;
  });
}

/**
 * countSitesForCompany — count client_sites rows for a given company id
 * where deleted_at IS NULL and status = 'active'.
 */
export async function countSitesForCompany(companyId: string): Promise<number> {
  return withClient(async (client) => {
    const res = await client.query<{ cnt: string }>(
      `SELECT COUNT(*) AS cnt FROM client_sites
       WHERE client_company_id = $1 AND deleted_at IS NULL AND status = 'active'`,
      [companyId],
    );
    return parseInt(res.rows[0]?.cnt ?? '0', 10);
  });
}

/**
 * getSiteGeofence — return geo_lat, geo_lng, geofence_radius_m for a site, or null if not found.
 */
export async function getSiteGeofence(
  siteId: string,
): Promise<{ lat: number | null; lng: number | null; radius: number } | null> {
  return withClient(async (client) => {
    const res = await client.query<{
      geo_lat: number | null;
      geo_lng: number | null;
      geofence_radius_m: number;
    }>(
      'SELECT geo_lat, geo_lng, geofence_radius_m FROM client_sites WHERE id = $1',
      [siteId],
    );
    if (res.rows.length === 0) return null;
    const row = res.rows[0];
    return { lat: row.geo_lat, lng: row.geo_lng, radius: row.geofence_radius_m };
  });
}

/**
 * getServiceLineStatus — return the status for a service line by id, or null if not found.
 */
export async function getServiceLineStatus(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ status: string }>(
      'SELECT status FROM service_lines WHERE id = $1',
      [id],
    );
    return res.rows[0]?.status ?? null;
  });
}

/**
 * getPositionStatus — return the status for a position by id, or null if not found.
 * Note: soft-deleted positions have deleted_at set; status column may not exist —
 * check deleted_at IS NULL as the "active" indicator.
 */
export async function getPositionStatus(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ deleted_at: Date | null }>(
      'SELECT deleted_at FROM positions WHERE id = $1',
      [id],
    );
    if (res.rows.length === 0) return null;
    return res.rows[0].deleted_at === null ? 'active' : 'inactive';
  });
}

/**
 * countActivePositionsForLine — count positions for a service line where deleted_at IS NULL.
 */
export async function countActivePositionsForLine(lineId: string): Promise<number> {
  return withClient(async (client) => {
    const res = await client.query<{ cnt: string }>(
      'SELECT COUNT(*) AS cnt FROM positions WHERE service_line_id = $1 AND deleted_at IS NULL',
      [lineId],
    );
    return parseInt(res.rows[0]?.cnt ?? '0', 10);
  });
}

/**
 * getLeaveTypeStatus — return the status for a leave type by id, or null if not found.
 * Soft-deleted rows have deleted_at set; check for that.
 */
export async function getLeaveTypeStatus(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ status: string; deleted_at: Date | null }>(
      'SELECT status, deleted_at FROM leave_types WHERE id = $1',
      [id],
    );
    if (res.rows.length === 0) return null;
    if (res.rows[0].deleted_at !== null) return 'inactive';
    return res.rows[0].status ?? null;
  });
}

/**
 * getAttendanceCodeStatus — return the status for an attendance code by id, or null if not found.
 */
export async function getAttendanceCodeStatus(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ status: string; deleted_at: Date | null }>(
      'SELECT status, deleted_at FROM attendance_codes WHERE id = $1',
      [id],
    );
    if (res.rows.length === 0) return null;
    if (res.rows[0].deleted_at !== null) return 'inactive';
    return res.rows[0].status ?? null;
  });
}

/**
 * getOvertimeRuleStatus — return the status for an overtime rule by id, or null if not found.
 */
export async function getOvertimeRuleStatus(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ status: string; deleted_at: Date | null }>(
      'SELECT status, deleted_at FROM overtime_rules WHERE id = $1',
      [id],
    );
    if (res.rows.length === 0) return null;
    if (res.rows[0].deleted_at !== null) return 'inactive';
    return res.rows[0].status ?? null;
  });
}

/**
 * getCompanyByName — return the id of a client company by name, or null if not found.
 * Useful when a test creates a company and needs the generated id.
 */
export async function getCompanyByName(name: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ id: string }>(
      'SELECT id FROM client_companies WHERE name = $1 ORDER BY created_at DESC LIMIT 1',
      [name],
    );
    return res.rows[0]?.id ?? null;
  });
}

// ---------------------------------------------------------------------------
// E2 people verification helpers (added in Phase 04-06)
// ---------------------------------------------------------------------------

/**
 * getEmployeeStatus — return the status ('active' | 'inactive') for an employee by id,
 * or null if not found.
 */
export async function getEmployeeStatus(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ status: string }>(
      'SELECT status FROM employees WHERE id = $1',
      [id],
    );
    return res.rows[0]?.status ?? null;
  });
}

/**
 * getEmployeePhone — return the phone number for an employee by id, or null if not found.
 */
export async function getEmployeePhone(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ phone: string | null }>(
      'SELECT phone FROM employees WHERE id = $1',
      [id],
    );
    return res.rows[0]?.phone ?? null;
  });
}

/**
 * getEmployeeIdByNIK — return the id of an employee with the given NIK, or null if not found.
 */
export async function getEmployeeIdByNIK(nik: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ id: string }>(
      'SELECT id FROM employees WHERE nik = $1',
      [nik],
    );
    return res.rows[0]?.id ?? null;
  });
}

/**
 * getAgreementStatus — return the status for an employment agreement by id,
 * or null if not found.
 */
export async function getAgreementStatus(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ status: string }>(
      'SELECT status FROM employment_agreements WHERE id = $1',
      [id],
    );
    return res.rows[0]?.status ?? null;
  });
}

/**
 * countAttachmentsForAgreement — count agreement_attachments rows for a given agreement id.
 */
export async function countAttachmentsForAgreement(agreementId: string): Promise<number> {
  return withClient(async (client) => {
    const res = await client.query<{ cnt: string }>(
      'SELECT COUNT(*) AS cnt FROM agreement_attachments WHERE agreement_id = $1',
      [agreementId],
    );
    return parseInt(res.rows[0]?.cnt ?? '0', 10);
  });
}

/**
 * getChangeRequestStatus — return the status ('pending' | 'approved' | 'rejected') for
 * a change request by id, or null if not found.
 */
export async function getChangeRequestStatus(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ status: string }>(
      'SELECT status FROM change_requests WHERE id = $1',
      [id],
    );
    return res.rows[0]?.status ?? null;
  });
}

/**
 * getSiteByName — return the id of a site by name within a company, or null if not found.
 */
export async function getSiteByName(
  companyId: string,
  name: string,
): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ id: string }>(
      `SELECT id FROM client_sites
       WHERE client_company_id = $1 AND name = $2 AND deleted_at IS NULL
       ORDER BY created_at DESC LIMIT 1`,
      [companyId, name],
    );
    return res.rows[0]?.id ?? null;
  });
}

/**
 * countPrimarySitesForCompany — count how many sites have is_primary=true for a company.
 * INV-5: exactly 1 primary site must exist at all times.
 */
export async function countPrimarySitesForCompany(companyId: string): Promise<number> {
  return withClient(async (client) => {
    const res = await client.query<{ cnt: string }>(
      `SELECT COUNT(*) AS cnt FROM client_sites
       WHERE client_company_id = $1 AND is_primary = true AND deleted_at IS NULL`,
      [companyId],
    );
    return parseInt(res.rows[0]?.cnt ?? '0', 10);
  });
}

// ---------------------------------------------------------------------------
// E3 placement verification helpers (added in Phase 05-04)
// ---------------------------------------------------------------------------

/**
 * getPlacementLifecycleStatus — return the PERSISTED lifecycle_status for a placement,
 * or null if not found. Note: the persisted column is the base status (ACTIVE / ENDED /
 * TRANSFERRED / SUPERSEDED / TERMINATED / RESIGNED / PENDING_START); the EXPIRING +
 * same-day-ACTIVE derivations happen at the DTO boundary, not in this column.
 */
export async function getPlacementLifecycleStatus(id: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ lifecycle_status: string }>(
      'SELECT lifecycle_status FROM placements WHERE id = $1',
      [id],
    );
    return res.rows[0]?.lifecycle_status ?? null;
  });
}

/**
 * setCompanyStatus — flip a client company's status (e.g. 'inactive') so the
 * COMPANY_INACTIVE create path can be exercised against the real BE.
 */
export async function setCompanyStatus(id: string, status: string): Promise<void> {
  await withClient(async (client) => {
    const res = await client.query('UPDATE client_companies SET status = $2 WHERE id = $1', [
      id,
      status,
    ]);
    if ((res.rowCount ?? 0) === 0) {
      throw new Error(`[setCompanyStatus] No company found with id: ${id}`);
    }
  });
}

/**
 * getActiveLeaderEmployeeForCompany — return the employee_id of the company-scope
 * (site_id IS NULL) ACTIVE shift-leader for a company, or null if vacant.
 */
export async function getActiveLeaderEmployeeForCompany(companyId: string): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ employee_id: string }>(
      `SELECT employee_id FROM shift_leader_assignments
       WHERE client_company_id = $1 AND site_id IS NULL AND unassigned_at IS NULL
       ORDER BY assigned_at DESC LIMIT 1`,
      [companyId],
    );
    return res.rows[0]?.employee_id ?? null;
  });
}

/**
 * getShiftLeaderAssignmentActive — return whether a SLA row is active
 * (unassigned_at IS NULL) plus its vacated_reason; null if not found.
 */
export async function getShiftLeaderAssignment(
  id: string,
): Promise<{ active: boolean; vacated_reason: string | null } | null> {
  return withClient(async (client) => {
    const res = await client.query<{ unassigned_at: Date | null; vacated_reason: string | null }>(
      'SELECT unassigned_at, vacated_reason FROM shift_leader_assignments WHERE id = $1',
      [id],
    );
    if (res.rows.length === 0) return null;
    return {
      active: res.rows[0].unassigned_at === null,
      vacated_reason: res.rows[0].vacated_reason,
    };
  });
}

/**
 * getPlacementIdForEmployeeAtCompany — return the id of the active/non-terminal placement
 * for an employee at a company, or null. Used to resolve a created placement's id.
 */
export async function getPlacementIdForEmployeeAtCompany(
  employeeId: string,
  companyId: string,
): Promise<string | null> {
  return withClient(async (client) => {
    const res = await client.query<{ id: string }>(
      `SELECT id FROM placements
       WHERE employee_id = $1 AND client_company_id = $2 AND deleted_at IS NULL
       ORDER BY created_at DESC LIMIT 1`,
      [employeeId, companyId],
    );
    return res.rows[0]?.id ?? null;
  });
}
