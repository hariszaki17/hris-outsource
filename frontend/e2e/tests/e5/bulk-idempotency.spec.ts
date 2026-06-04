/**
 * tests/e5/bulk-idempotency.spec.ts
 *
 * E5 · bulk verify/reject partial success + idempotency replay (F5.3 / CONVENTIONS §14)
 * against the REAL stack + the REAL Postgres-backed Idempotency store (07-03 covered the
 * in-memory stub; here the production middleware is exercised end-to-end).
 *
 * Coverage:
 *   BULK-partial-verify   verify 9002 first → bulk-verify [9002(now terminal), 9003] → 200
 *                         succeeded=[9003], failed=[{id:9002, error.code CONFLICT}].
 *   BULK-idempotency      fixed key K → bulk-verify [9004] → 200 succeeded=[9004]; SAME key+body
 *                         → SAME 200 body (replay, not double-processed); SAME key + DIFFERENT
 *                         body → 409 IDEMPOTENCY_KEY_REUSED.
 *   BULK-reject-partial   reject 9003 first → bulk-reject [9003(terminal), 9002] → 200 partial.
 *   BULK-all-fail         bulk-verify two already-terminal ids → 422, failed non-empty, succeeded empty.
 *
 * Bulk envelope (openapi BulkActionResponse): { succeeded: string[], failed: [{id, error}] };
 * 200 if >=1 succeeded, 422 if all failed. Terminal verify/reject = 409 CONFLICT.
 * Seed (07-02): 9002/9003/9004 PENDING @ CMP-0021.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { ATT, apiAs, apiAsWithKey, bulk, waitForToken } from '../../lib/e5-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// BULK-partial-verify — one terminal + one PENDING → 200 partial success
// ---------------------------------------------------------------------------

test('BULK-partial-verify · bulk-verify [terminal 9002, pending 9003] → 200 succeeded=[9003], failed=[9002 CONFLICT]', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  // Make 9002 terminal first (single verify).
  const v = await apiAs(page, 'POST', `/attendance/${ATT.lateCmp21}:verify`, {});
  expect(v.status).toBe(200);

  // Bulk-verify the now-terminal 9002 + the still-PENDING 9003.
  const res = await apiAs(page, 'POST', '/attendance:bulk-verify', {
    ids: [ATT.lateCmp21, ATT.geoCmp21],
  });
  expect(res.status).toBe(200);
  const body = bulk(res.body);
  expect(body.succeeded).toContain(ATT.geoCmp21);
  expect(body.succeeded).not.toContain(ATT.lateCmp21);
  const failed = body.failed ?? [];
  expect(failed.some((f) => f.id === ATT.lateCmp21 && f.error?.code === 'CONFLICT')).toBe(true);
});

// ---------------------------------------------------------------------------
// BULK-idempotency — fixed key replay + key reuse with different body → 409
// ---------------------------------------------------------------------------

test('BULK-idempotency · same key+body replays the same 200; same key+different body → 409 IDEMPOTENCY_KEY_REUSED', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  const key = `e2e-fixed-${Date.now()}`;
  const body = { ids: [ATT.autoClosedCmp21] }; // 9004 PENDING

  // First call → 200, 9004 verified.
  const first = await apiAsWithKey(page, 'POST', '/attendance:bulk-verify', body, key);
  expect(first.status).toBe(200);
  expect(bulk(first.body).succeeded).toContain(ATT.autoClosedCmp21);

  // Replay: same key + same body → identical 200 body (NOT re-processed as a terminal CONFLICT).
  const replay = await apiAsWithKey(page, 'POST', '/attendance:bulk-verify', body, key);
  expect(replay.status).toBe(200);
  expect(bulk(replay.body).succeeded).toContain(ATT.autoClosedCmp21);
  expect(JSON.stringify(replay.body)).toBe(JSON.stringify(first.body));

  // Same key + DIFFERENT body → 409 IDEMPOTENCY_KEY_REUSED.
  const reuse = await apiAsWithKey(
    page,
    'POST',
    '/attendance:bulk-verify',
    { ids: [ATT.geoCmp21] },
    key,
  );
  expect(reuse.status).toBe(409);
  const code = (reuse.body as { error?: { code?: string } }).error?.code;
  expect(code).toBe('IDEMPOTENCY_KEY_REUSED');
});

// ---------------------------------------------------------------------------
// BULK-reject-partial — one terminal + one PENDING → 200 partial
// ---------------------------------------------------------------------------

test('BULK-reject-partial · bulk-reject [terminal 9003, pending 9002] → 200 partial (succeeded + failed)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  // Make 9003 terminal first (single reject with a valid reason).
  const r = await apiAs(page, 'POST', `/attendance/${ATT.geoCmp21}:reject`, {
    reason: 'Lokasi di luar geofence dan tidak ada bukti.',
  });
  expect(r.status).toBe(200);

  const res = await apiAs(page, 'POST', '/attendance:bulk-reject', {
    ids: [ATT.geoCmp21, ATT.lateCmp21],
    reason: 'Bukti clock-in tidak sesuai foto.',
  });
  expect(res.status).toBe(200);
  const body = bulk(res.body);
  expect((body.succeeded ?? []).length).toBeGreaterThan(0);
  expect((body.failed ?? []).length).toBeGreaterThan(0);
  expect((body.failed ?? []).some((f) => f.id === ATT.geoCmp21 && f.error?.code === 'CONFLICT')).toBe(
    true,
  );
});

// ---------------------------------------------------------------------------
// BULK-all-fail — two already-terminal ids → 422
// ---------------------------------------------------------------------------

test('BULK-all-fail · bulk-verify two already-terminal records → 422, failed non-empty, succeeded empty', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  // Verify both 9002 + 9004 first so they are terminal.
  expect((await apiAs(page, 'POST', `/attendance/${ATT.lateCmp21}:verify`, {})).status).toBe(200);
  expect((await apiAs(page, 'POST', `/attendance/${ATT.autoClosedCmp21}:verify`, {})).status).toBe(
    200,
  );

  const res = await apiAs(page, 'POST', '/attendance:bulk-verify', {
    ids: [ATT.lateCmp21, ATT.autoClosedCmp21],
  });
  expect(res.status).toBe(422);
  const body = bulk(res.body);
  expect((body.succeeded ?? []).length).toBe(0);
  expect((body.failed ?? []).length).toBe(2);
});
