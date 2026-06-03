/**
 * global-setup.ts
 *
 * Playwright globalSetup — called once before any spec runs.
 * Boots the full backend stack: Postgres → migrate → River migrate → seed → Go API → /healthz.
 * See lib/backend.ts for the implementation.
 */
import { startBackend } from './lib/backend.js';

export default async function globalSetup(): Promise<void> {
  await startBackend();
}
