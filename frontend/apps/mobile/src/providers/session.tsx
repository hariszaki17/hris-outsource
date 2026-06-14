// Session state for the mobile app. Mirrors the web auth flow but RN-native:
// restore on cold start via a BODY-based POST /auth/refresh (refresh token from SecureStore,
// not a cookie), then GET /auth/me to hydrate the user.
import { customFetch } from '@swp/api-client';
import { type MeResponse, useAuthLogout } from '@swp/api-client/e1';
import { type ReactNode, createContext, useCallback, useContext, useEffect, useState } from 'react';
import { tokenStore } from '../lib/auth';

type Status = 'restoring' | 'authed' | 'unauthed';

type LoginResult = { access_token: string; refresh_token: string; user: MeResponse };

type SessionValue = {
  status: Status;
  user: MeResponse | null;
  signIn: (result: LoginResult) => Promise<void>;
  signOut: () => Promise<void>;
};

const SessionContext = createContext<SessionValue | null>(null);

export function useSession(): SessionValue {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error('useSession must be used within SessionProvider');
  return ctx;
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<Status>('restoring');
  const [user, setUser] = useState<MeResponse | null>(null);
  const logout = useAuthLogout();

  const signIn = useCallback(async (result: LoginResult) => {
    await tokenStore.setSession(result.access_token, result.refresh_token);
    setUser(result.user);
    setStatus('authed');
  }, []);

  const signOut = useCallback(async () => {
    try {
      await logout.mutateAsync();
    } catch {
      // Best-effort server logout; clear locally regardless.
    }
    await tokenStore.clear();
    setUser(null);
    setStatus('unauthed');
  }, [logout]);

  // Let a 401 (onUnauthenticated) drop the session.
  useEffect(() => {
    tokenStore.registerForceLogout(() => {
      void signOut();
    });
  }, [signOut]);

  // Cold-start restore.
  useEffect(() => {
    let active = true;
    void (async () => {
      const refresh = await tokenStore.getRefresh();
      if (!refresh) {
        if (active) setStatus('unauthed');
        return;
      }
      try {
        // customFetch<T> casts the { data, status, headers } envelope to T, so T must be the
        // envelope shape — the response body lives under `.data`. Non-2xx throws (mutator).
        const refreshed = await customFetch<{
          data: { access_token: string; refresh_token?: string };
        }>('/auth/refresh', {
          method: 'POST',
          body: JSON.stringify({ refresh_token: refresh }),
        });
        tokenStore.setAccess(refreshed.data.access_token);
        if (refreshed.data.refresh_token) {
          await tokenStore.persistRefresh(refreshed.data.refresh_token);
        }
        const me = await customFetch<{ data: MeResponse }>('/auth/me', { method: 'GET' });
        if (active) {
          setUser(me.data);
          setStatus('authed');
        }
      } catch {
        await tokenStore.clear();
        if (active) setStatus('unauthed');
      }
    })();
    return () => {
      active = false;
    };
  }, []);

  return (
    <SessionContext.Provider value={{ status, user, signIn, signOut }}>
      {children}
    </SessionContext.Provider>
  );
}
