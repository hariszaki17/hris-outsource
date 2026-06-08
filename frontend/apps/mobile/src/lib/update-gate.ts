// Force-update-on-launch gate (EAS Update / OTA).
//
// SCAFFOLD STUB — wiring deferred to a later milestone. Two layers, by design:
//
// 1. JS-only updates (this file): `expo-updates` can fetch a new JS bundle over-the-air and
//    reload without a store reinstall. `checkForJsUpdate()` blocks the UI behind an updating
//    gate when a new bundle is available, then reloads — covering ~90% of changes.
//
// 2. Native/store updates (TODO, needs backend): when a native change ships (new native
//    module, SDK bump), OTA cannot deliver it. A FOLLOW-UP backend contract — e.g.
//    `GET /app/version-policy` returning `min_supported_version` — will let the app hard-block
//    below the minimum and deep-link to the store. NOT implemented this milestone.
import * as Updates from 'expo-updates';

/** Fetch + apply a pending OTA JS bundle. Returns true if an update was applied (app reloads). */
export async function checkForJsUpdate(): Promise<boolean> {
  // Disabled in dev / when updates aren't configured (no EAS Update URL yet).
  if (__DEV__ || !Updates.isEnabled) return false;
  try {
    const result = await Updates.checkForUpdateAsync();
    if (!result.isAvailable) return false;
    await Updates.fetchUpdateAsync();
    await Updates.reloadAsync();
    return true;
  } catch {
    // Never block launch on an update-check failure.
    return false;
  }
}

// TODO(backend, follow-up milestone): version-gate.
// const policy = await getVersionPolicy()  // GET /app/version-policy
// if (semverLt(appVersion, policy.min_supported_version)) showHardUpdateGate(policy.store_url)
export const MIN_SUPPORTED_VERSION_TODO =
  'backend GET /app/version-policy → { min_supported_version } not yet implemented';
