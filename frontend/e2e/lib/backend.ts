/**
 * lib/backend.ts
 *
 * startBackend() / stopBackend() — used by global-setup.ts and global-teardown.ts.
 *
 * Boot sequence (matches e2e-harness-spec.md globalSetup):
 *   1. Start ephemeral Postgres on :5433 via docker-compose.e2e.yml
 *   2. Wait for pg_isready (healthcheck)
 *   3. Run goose migrations: go run ./cmd/migrate up
 *   4. Run River queue migrations (river migrate-up), with fallback if CLI absent
 *   5. Run go run ./cmd/seed to upsert the four test personas
 *   6. Generate a fresh Ed25519 keypair via go run ./cmd/seed -genkeys
 *   7. Spawn go run ./cmd/api on :8081 with the test env
 *   8. Poll GET /healthz until 200 (timeout 60 s)
 *
 * stopBackend():
 *   - Sends SIGTERM to the API process
 *   - Runs docker compose … down -v to remove the container + volumes
 */
import * as path from 'node:path';
import { ChildProcess, execSync, spawn } from 'node:child_process';
import * as fs from 'node:fs';
import * as os from 'node:os';

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------
const REPO_ROOT = path.resolve(import.meta.dirname, '../../..');
const BACKEND_DIR = path.join(REPO_ROOT, 'backend');
const E2E_DIR = path.join(REPO_ROOT, 'frontend', 'e2e');
const COMPOSE_FILE = path.join(E2E_DIR, 'docker-compose.e2e.yml');

// Prebuilt seed binary. resetDb() reseeds in EVERY beforeEach (~250×/full run);
// `go run ./cmd/seed` recompiles+spawns the toolchain each time, which thrashes the
// machine on the full suite. We compile ONCE here and both globalSetup + reset-db.ts
// exec this binary by the same fixed path. Keep this in sync with lib/reset-db.ts SEED_BIN.
export const SEED_BIN = path.join(
  os.tmpdir(),
  process.platform === 'win32' ? 'swp-e2e-seed.exe' : 'swp-e2e-seed',
);
const ENV_FILE = path.join(E2E_DIR, '.env.e2e');

// ---------------------------------------------------------------------------
// Parse .env.e2e
// ---------------------------------------------------------------------------
function loadEnvFile(filePath: string): Record<string, string> {
  const out: Record<string, string> = {};
  if (!fs.existsSync(filePath)) return out;
  const lines = fs.readFileSync(filePath, 'utf8').split('\n');
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const eqIdx = trimmed.indexOf('=');
    if (eqIdx === -1) continue;
    const key = trimmed.slice(0, eqIdx).trim();
    const value = trimmed.slice(eqIdx + 1).trim();
    out[key] = value;
  }
  return out;
}

// ---------------------------------------------------------------------------
// Module-level state (kept alive across the Playwright worker lifecycle)
// ---------------------------------------------------------------------------
let apiProcess: ChildProcess | null = null;
// The River worker (go run ./cmd/worker). Spawned alongside the API (10-04) so async
// jobs — notifications AND the E8 payslip-export job (PayslipExportArgs) — are actually
// processed: POST /payslips:export inserts a QUEUED export_jobs row + EnqueueTx's the job
// in one tx (transactional outbox), and ONLY a running worker flips it RUNNING → DONE.
// Without this process the export_jobs row would stay QUEUED forever (the export E2E poll
// would time out). Detached into its own process group + reaped on teardown, mirroring the
// API spawn (go run does not forward SIGTERM to its exe child — decision [07-04]).
let workerProcess: ChildProcess | null = null;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Run a command synchronously, streaming stdout/stderr to the console. */
function runSync(
  cmd: string,
  args: string[],
  opts: { cwd?: string; env?: NodeJS.ProcessEnv } = {},
): void {
  const fullCmd = [cmd, ...args].join(' ');
  console.log(`[e2e] $ ${fullCmd}`);
  execSync(fullCmd, {
    cwd: opts.cwd ?? BACKEND_DIR,
    env: { ...process.env, ...(opts.env ?? {}) },
    stdio: 'inherit',
  });
}

/** Poll a predicate (sync) at `intervalMs` until it returns true or timeout. */
async function pollUntil(
  predicate: () => boolean,
  { timeoutMs = 60_000, intervalMs = 1_000, label = 'condition' } = {},
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (predicate()) return;
    await new Promise((r) => setTimeout(r, intervalMs));
  }
  throw new Error(`[e2e] Timed out waiting for: ${label}`);
}

/** Poll GET url until it returns 200. */
async function waitForHttp(url: string, timeoutMs = 60_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url);
      if (res.ok) {
        console.log(`[e2e] ${url} is healthy`);
        return;
      }
    } catch {
      // connection refused — keep polling
    }
    await new Promise((r) => setTimeout(r, 1_000));
  }
  throw new Error(`[e2e] Timed out waiting for ${url} to return 200`);
}

// ---------------------------------------------------------------------------
// Step 1-2: Postgres container
// ---------------------------------------------------------------------------

function composeUp(): void {
  runSync('docker', ['compose', '-f', COMPOSE_FILE, 'up', '-d'], { cwd: E2E_DIR });
}

async function waitForPostgres(): Promise<void> {
  console.log('[e2e] Waiting for Postgres :5433 to be ready …');
  await pollUntil(
    () => {
      try {
        execSync(
          `docker compose -f ${COMPOSE_FILE} exec -T e2e-postgres pg_isready -U hris -d hris_e2e`,
          { cwd: E2E_DIR, stdio: 'pipe' },
        );
        return true;
      } catch {
        return false;
      }
    },
    { timeoutMs: 60_000, intervalMs: 2_000, label: 'Postgres :5433 pg_isready' },
  );
}

// ---------------------------------------------------------------------------
// Step 3: goose migrations
// ---------------------------------------------------------------------------
function runMigrations(testEnv: NodeJS.ProcessEnv): void {
  console.log('[e2e] Running goose migrations (go run ./cmd/migrate up) …');
  runSync('go', ['run', './cmd/migrate', 'up'], { cwd: BACKEND_DIR, env: testEnv });
}

// ---------------------------------------------------------------------------
// Step 4: River queue migrations (best-effort, falls back gracefully)
// ---------------------------------------------------------------------------
function runRiverMigrations(testEnv: NodeJS.ProcessEnv): void {
  // Apply River's queue migrations PROGRAMMATICALLY via `go run ./cmd/migrate river-up`
  // (rivermigrate over a pgx pool) — no external `river` CLI install required. Without
  // these tables the worker crashes on boot ("relation river_queue does not exist") and
  // the async E8 export job never completes (the export E2E poll would time out). This is
  // the robust path; the legacy `river` CLI is no longer required.
  try {
    runSync('go', ['run', './cmd/migrate', 'river-up'], { cwd: BACKEND_DIR, env: testEnv });
    console.log('[e2e] River migrations applied (programmatic rivermigrate).');
    return;
  } catch (err) {
    console.warn('[e2e] programmatic river-up failed, falling back to `river` CLI:', err);
  }
  const dbUrl = testEnv.DATABASE_URL as string;
  try {
    execSync(`river migrate-up --database-url "${dbUrl}"`, {
      cwd: BACKEND_DIR,
      env: { ...process.env, ...testEnv },
      stdio: 'inherit',
    });
    console.log('[e2e] River migrations applied (river CLI).');
  } catch {
    console.warn(
      '[e2e] WARNING: River queue tables not created — async worker features ' +
        '(notifications, exports) will NOT be exercised.',
    );
  }
}

// ---------------------------------------------------------------------------
// Step 4b: Compile the seed binary ONCE (reused by every resetDb reseed).
// ---------------------------------------------------------------------------
function buildSeedBinary(testEnv: NodeJS.ProcessEnv): void {
  console.log(`[e2e] Building seed binary once → ${SEED_BIN} …`);
  runSync('go', ['build', '-o', SEED_BIN, './cmd/seed'], { cwd: BACKEND_DIR, env: testEnv });
}

// ---------------------------------------------------------------------------
// Step 5: Seed personas (prebuilt binary — no per-call `go run` recompile)
// ---------------------------------------------------------------------------
function runSeed(testEnv: NodeJS.ProcessEnv): void {
  console.log('[e2e] Seeding test personas (prebuilt seed binary) …');
  runSync(SEED_BIN, [], { cwd: BACKEND_DIR, env: testEnv });
}

// ---------------------------------------------------------------------------
// Step 6: Generate Ed25519 keypair
// ---------------------------------------------------------------------------
function generateKeypair(): { privateKey: string; publicKey: string } {
  console.log('[e2e] Generating Ed25519 keypair (prebuilt seed binary -genkeys) …');
  const output = execSync(`"${SEED_BIN}" -genkeys`, {
    cwd: BACKEND_DIR,
    env: process.env,
    encoding: 'utf8',
  });
  // Contract (from cmd/seed/main.go): exactly two lines on stdout.
  // Line 1: AUTH_JWT_PRIVATE_KEY (base64 std, 64-byte Ed25519 private key)
  // Line 2: AUTH_JWT_PUBLIC_KEY  (base64 std, 32-byte Ed25519 public key)
  const lines = output.trim().split('\n').map((l) => l.trim());
  if (lines.length < 2) {
    throw new Error(`[e2e] go run ./cmd/seed -genkeys returned unexpected output: ${output}`);
  }
  return { privateKey: lines[0], publicKey: lines[1] };
}

// ---------------------------------------------------------------------------
// Step 7: Start Go API
// ---------------------------------------------------------------------------
/**
 * freePort — kill any process still bound to a TCP port. `go run ./cmd/api` execs the
 * compiled binary as a CHILD; a SIGTERM to the `go run` parent is NOT forwarded to that
 * child, so a previous run can leave an ORPHAN `exe/api` holding :8081. The next run's
 * fresh API then fails to bind and exits, while the orphan keeps serving the STALE binary
 * (→ 404 on newly-added routes). Reaping the port-holder before boot guarantees the new
 * binary actually serves. Best-effort: ignore "no such process" / empty results.
 */
function freePort(port: number): void {
  try {
    const pids = execSync(`lsof -ti tcp:${port} || true`, { encoding: 'utf8' })
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean);
    for (const pid of pids) {
      console.log(`[e2e] freeing port ${port}: killing orphan pid ${pid}`);
      try {
        process.kill(Number(pid), 'SIGKILL');
      } catch {
        /* already gone */
      }
    }
  } catch {
    /* lsof unavailable or nothing bound — fine */
  }
}

function startApiProcess(testEnv: NodeJS.ProcessEnv): void {
  // Reap any orphaned API from a prior run that still holds :8081 (go run doesn't
  // forward SIGTERM to its child binary — see freePort docstring).
  freePort(8081);
  console.log('[e2e] Starting Go API on :8081 (go run ./cmd/api) …');
  apiProcess = spawn('go', ['run', './cmd/api'], {
    cwd: BACKEND_DIR,
    env: { ...process.env, ...testEnv },
    stdio: ['ignore', 'inherit', 'inherit'],
    // Own process group so stopBackend can SIGTERM the WHOLE tree (go run + exe/api),
    // not just the `go run` parent (which would orphan the bound child).
    detached: true,
  });

  apiProcess.on('error', (err) => {
    console.error('[e2e] API process error:', err);
  });

  apiProcess.on('exit', (code, signal) => {
    if (code !== 0 && signal !== 'SIGTERM' && signal !== 'SIGKILL') {
      console.error(`[e2e] API process exited unexpectedly (code=${code}, signal=${signal})`);
    }
  });
}

// ---------------------------------------------------------------------------
// Step 7b: Start the River worker (go run ./cmd/worker)
// ---------------------------------------------------------------------------
/**
 * startWorkerProcess — boot the River job processor so async jobs complete. The
 * E8 export E2E asserts the PayslipExportWorker flips export_jobs → DONE; that ONLY
 * happens if a worker is running (the API merely enqueues the job inside the export
 * tx). The worker reads DATABASE_URL + PAYROLL_ENCRYPTION_KEY from the SAME testEnv as
 * the API/seed (so its export job uses the same AEAD key as the seed's ciphertext).
 * Spawned detached (own process group) so stopBackend can SIGTERM the whole tree
 * (go run + exe/worker) — go run does not forward SIGTERM to its child binary.
 */
function startWorkerProcess(testEnv: NodeJS.ProcessEnv): void {
  console.log('[e2e] Starting River worker (go run ./cmd/worker) …');
  workerProcess = spawn('go', ['run', './cmd/worker'], {
    cwd: BACKEND_DIR,
    env: { ...process.env, ...testEnv },
    stdio: ['ignore', 'inherit', 'inherit'],
    detached: true,
  });

  workerProcess.on('error', (err) => {
    console.error('[e2e] worker process error:', err);
  });

  workerProcess.on('exit', (code, signal) => {
    if (code !== 0 && signal !== 'SIGTERM' && signal !== 'SIGKILL') {
      console.error(`[e2e] worker process exited unexpectedly (code=${code}, signal=${signal})`);
    }
  });
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

export async function startBackend(): Promise<void> {
  console.log('[e2e] ── startBackend ─────────────────────────────────────────');

  // Load the base env (excludes generated JWT keys)
  const baseEnv = loadEnvFile(ENV_FILE);

  // 1–2. Postgres
  composeUp();
  await waitForPostgres();

  // Build the full test env (reused for all Go processes)
  const testEnv: NodeJS.ProcessEnv = {
    ...process.env,
    ...baseEnv,
  };

  // 3. Goose migrations
  runMigrations(testEnv);

  // 4. River migrations (best-effort)
  runRiverMigrations(testEnv);

  // 4b. Compile the seed binary once (reused by every resetDb reseed).
  buildSeedBinary(testEnv);

  // 5. Seed personas
  runSeed(testEnv);

  // 6. Generate JWT keypair for the Go API
  const { privateKey, publicKey } = generateKeypair();
  const apiEnv: NodeJS.ProcessEnv = {
    ...testEnv,
    AUTH_JWT_PRIVATE_KEY: privateKey,
    AUTH_JWT_PUBLIC_KEY: publicKey,
  };

  // 7. Start Go API
  startApiProcess(apiEnv);

  // 7b. Start the River worker so async jobs (notifications + the E8 payslip export)
  //     actually complete. Uses apiEnv so it shares DATABASE_URL + PAYROLL_ENCRYPTION_KEY
  //     (the export job's AEAD key MUST match the seed's ciphertext).
  startWorkerProcess(apiEnv);

  // 8. Wait for /healthz
  console.log('[e2e] Waiting for Go API /healthz on :8081 …');
  await waitForHttp('http://localhost:8081/healthz', 60_000);

  console.log('[e2e] ── backend stack is UP ─────────────────────────────────');
}

export function stopBackend(): void {
  console.log('[e2e] ── stopBackend ─────────────────────────────────────────');

  // Terminate the Go API. The API was spawned detached (its own process group), so
  // signal the WHOLE group (negative pid) to also reap the `exe/api` child that `go run`
  // execs — a plain apiProcess.kill() would only hit the `go run` parent and orphan the
  // bound child. Fall back to freeing the port directly if anything slips through.
  if (apiProcess && apiProcess.pid && !apiProcess.killed) {
    try {
      process.kill(-apiProcess.pid, 'SIGTERM');
    } catch {
      apiProcess.kill('SIGTERM');
    }
    apiProcess = null;
  }
  freePort(8081);

  // Terminate the River worker. Spawned detached (own process group), so signal the
  // WHOLE group (negative pid) to reap the `exe/worker` child that `go run` execs —
  // a plain workerProcess.kill() would only hit the `go run` parent and orphan the child.
  if (workerProcess && workerProcess.pid && !workerProcess.killed) {
    try {
      process.kill(-workerProcess.pid, 'SIGTERM');
    } catch {
      workerProcess.kill('SIGTERM');
    }
    workerProcess = null;
  }

  // Tear down the Postgres container (including volumes)
  try {
    execSync(`docker compose -f ${COMPOSE_FILE} down -v`, {
      cwd: E2E_DIR,
      env: process.env,
      stdio: 'inherit',
    });
  } catch (err) {
    console.warn('[e2e] docker compose down -v failed (may already be stopped):', err);
  }

  console.log('[e2e] ── backend stack is DOWN ───────────────────────────────');
}
