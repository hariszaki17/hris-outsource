/**
 * E10 · Kotak Masuk (Approval Inbox) — the full-page aggregated "needs my decision" queue.
 *
 * The cross-cutting workflow surface from the nav IA (docs/eng/NAVIGATION-AND-RBAC.md): it
 * unifies leave + overtime + attendance + change-request approvals into one list. It is a
 * **view**, not a second queue — it reads the SAME `pending_approvals_panel` rows the
 * dashboard inbox panel shows (single source of truth); each row deep-links into the owning
 * domain's approval screen, where the per-domain tab approves against the same data.
 *
 * Visibility is gated in nav.ts (any `*.approve` permission); this screen is defense-in-depth.
 */

import { AgreementExpiryPanel } from '@/features/e2-identity/agreement-expiry-panel.tsx';
import { classifyError } from '@/lib/api-error.ts';
import { type Dashboard, useGetMyDashboard } from '@swp/api-client/e10';
import { StateView } from '@swp/ui';
import { useTranslation } from 'react-i18next';
import { ApprovalInboxPanel } from './approval-inbox-panel.tsx';

export function InboxScreen() {
  const { t } = useTranslation();
  const { t: td } = useTranslation('dashboard');
  const query = useGetMyDashboard();

  if (query.isLoading) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="font-bold text-3xl text-text">{t('nav.inbox')}</h1>
        <ApprovalInboxPanel rows={[]} isLoading />
      </div>
    );
  }

  if (query.isError) {
    const err = classifyError(query.error);
    const kind =
      err.kind === 'forbidden' || err.kind === 'unauthenticated' ? 'no-permission' : 'error';
    return (
      <StateView
        kind={kind}
        title={kind === 'no-permission' ? td('errors.noPermission') : td('errors.loadError')}
        description={
          kind === 'no-permission' ? td('errors.noPermissionBody') : td('errors.network')
        }
        onRetry={kind === 'error' ? () => void query.refetch() : undefined}
        retryLabel={kind === 'error' ? td('errors.retry') : undefined}
      />
    );
  }

  // Both HR and Leader dashboards carry `pending_approvals_panel`; the agent shape does not.
  const body = (query.data as { data: Dashboard } | undefined)?.data;
  const rows = body && 'pending_approvals_panel' in body ? body.pending_approvals_panel : [];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="font-bold text-3xl text-text">{t('nav.inbox')}</h1>
        <p className="text-[14px] text-text-3">{td('inbox.emptyBody')}</p>
      </div>
      <div className="flex flex-col gap-5 max-w-[640px]">
        <AgreementExpiryPanel />
        <ApprovalInboxPanel rows={rows} />
      </div>
    </div>
  );
}
