import { auth } from '@/lib/auth.ts';
import { Button, EmptyState } from '@swp/ui';
import { Link, useNavigate } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

/**
 * Global re-auth / permission states (E1 · F1.1/F1.2). These are the canonical full-page
 * surfaces for the two cross-cutting auth states (ENGINEERING.md B2 — no dead-flow):
 *   comp/EmptySessionExpired `iwcgE` → SessionExpiredScreen
 *   comp/EmptyNoPermission   `MRbzz` → NoPermissionScreen
 *
 * Per-screen 403/401 handling stays inline via `classifyError` (C1, defense-in-depth); these
 * routed screens are the standalone destinations (e.g. a 401 interceptor can route here, or a
 * user can land on `/forbidden` directly).
 */

/** P-10 — session expired; the in-memory access token is gone, prompt re-auth. */
export function SessionExpiredScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const reauth = () => {
    // Clear any stale in-memory session before sending the user back to /login.
    auth.clear();
    void navigate({ to: '/login' });
  };

  return (
    <div className="flex min-h-dvh items-center justify-center bg-app-bg p-10">
      <EmptyState
        variant="session-expired"
        title={t('sessionExpired.title')}
        description={t('sessionExpired.body')}
        hint={t('sessionExpired.hint')}
        action={
          <Button type="button" onClick={reauth}>
            {t('sessionExpired.action')}
          </Button>
        }
      />
    </div>
  );
}

/** 403 — authenticated but out of scope for this surface. */
export function NoPermissionScreen() {
  const { t } = useTranslation();
  return (
    <div className="flex min-h-dvh items-center justify-center bg-app-bg p-10">
      <EmptyState
        variant="no-permission"
        title={t('errors.forbidden')}
        description={t('noPermission.body')}
        hint={t('noPermission.hint')}
        action={
          <Button type="button" variant="secondary" asChild>
            <Link to="/">{t('noPermission.action')}</Link>
          </Button>
        }
      />
    </div>
  );
}
