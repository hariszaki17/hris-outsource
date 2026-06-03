import { useSyncExternalStore } from 'react';
import { type SessionUser, auth } from './auth.ts';

/**
 * Subscribe to the current signed-in user. `auth.getUser` returns a stable reference that
 * only changes when the session changes, so it is a safe `useSyncExternalStore` snapshot.
 */
export function useCurrentUser(): SessionUser | null {
  return useSyncExternalStore(auth.subscribe, auth.getUser, auth.getUser);
}
