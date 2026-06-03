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

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------
const REPO_ROOT = path.resolve(import.meta.dirname, '../../..');
const BACKEND_DIR = path.join(REPO_ROOT, 'backend');
const E2E_DIR = path.join(REPO_ROOT, 'frontend', 'e2e');
const COMPOSE_FILE = path.join(E2E_DIR, 'docker-compose.e2e.yml');
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
  const dbUrl = testEnv.DATABASE_URL as string;
  try {
    // Prefer the `river` CLI installed by `make tools`.
    execSync(`river migrate-up --database-url "${dbUrl}"`, {
      cwd: BACKEND_DIR,
      env: { ...process.env, ...testEnv },
      stdio: 'inherit',
    });
    console.log('[e2e] River migrations applied.');
  } catch {
    console.warn(
      '[e2e] WARNING: `river` CLI not found — River queue tables not created. ' +
      'Async worker features (notifications, exports) will not be exercised in Phase 1.',
    );
  }
}

// ---------------------------------------------------------------------------
// Step 5: Seed personas
// ---------------------------------------------------------------------------
function runSeed(testEnv: NodeJS.ProcessEnv): void {
  console.log('[e2e] Seeding test personas (go run ./cmd/seed) …');
  runSync('go', ['run', './cmd/seed'], { cwd: BACKEND_DIR, env: testEnv });
}

// ---------------------------------------------------------------------------
// Step 6: Generate Ed25519 keypair
// ---------------------------------------------------------------------------
function generateKeypair(): { privateKey: string; publicKey: string } {
  console.log('[e2e] Generating Ed25519 keypair (go run ./cmd/seed -genkeys) …');
  const output = execSync('go run ./cmd/seed -genkeys', {
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
function startApiProcess(testEnv: NodeJS.ProcessEnv): void {
  console.log('[e2e] Starting Go API on :8081 (go run ./cmd/api) …');
  apiProcess = spawn('go', ['run', './cmd/api'], {
    cwd: BACKEND_DIR,
    env: { ...process.env, ...testEnv },
    stdio: ['ignore', 'inherit', 'inherit'],
    detached: false,
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

  // 8. Wait for /healthz
  console.log('[e2e] Waiting for Go API /healthz on :8081 …');
  await waitForHttp('http://localhost:8081/healthz', 60_000);

  console.log('[e2e] ── backend stack is UP ─────────────────────────────────');
}

export function stopBackend(): void {
  console.log('[e2e] ── stopBackend ─────────────────────────────────────────');

  // Terminate the Go API process
  if (apiProcess && !apiProcess.killed) {
    apiProcess.kill('SIGTERM');
    apiProcess = null;
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
