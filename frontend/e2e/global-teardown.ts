/**
 * global-teardown.ts
 *
 * Playwright globalTeardown — called once after all specs complete.
 * Stops the Go API and removes the ephemeral Postgres container.
 * See lib/backend.ts for the implementation.
 */
import { stopBackend } from './lib/backend.js';

export default async function globalTeardown(): Promise<void> {
  stopBackend();
}
