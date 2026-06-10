import { useCurrentUser } from '@/lib/use-auth.ts';
/**
 * /me/profile — Agent views own profile + submits phone/address change request (F2 EP-5).
 *
 * Web port of apps/mobile/app/profile.tsx. Fetches the agent's own employee record via
 * useGetEmployee(employeeId) and submits edits via useCreateChangeRequest(). Only phone
 * and address are editable; full_name is read-only (statutory field, HR-only per EP-6).
 * On submit, only changed fields are sent (mobile parity). docs/eng/AGENT-WEB-ACCESS.md §5.
 */
import { type Employee, useCreateChangeRequest, useGetEmployee } from '@swp/api-client/e2';
import { Button, FormField, Input, StateView, useToast } from '@swp/ui';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AgentPage } from './agent-page.tsx';

export function AgentProfileScreen() {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const user = useCurrentUser();
  const employeeId = user?.employeeId ?? '';

  const q = useGetEmployee(employeeId, { query: { enabled: !!employeeId } });
  const create = useCreateChangeRequest();

  const emp = q.data?.data as Employee | undefined;
  const [phone, setPhone] = useState('');
  const [address, setAddress] = useState('');

  // Seed form fields once the employee record arrives.
  useEffect(() => {
    if (emp) {
      setPhone(emp.phone ?? '');
      setAddress(emp.address ?? '');
    }
  }, [emp]);

  async function onSave() {
    const changes: { phone?: string; address?: string } = {};
    if (emp && phone !== (emp.phone ?? '')) changes.phone = phone;
    if (emp && address !== (emp.address ?? '')) changes.address = address;

    if (!changes.phone && !changes.address) {
      toast({ tone: 'info', title: t('profileNoChange') });
      return;
    }

    try {
      await create.mutateAsync({ employeeId, data: { changes } });
      toast({ tone: 'success', title: t('profileSuccess') });
    } catch {
      toast({ tone: 'error', title: t('profileError') });
    }
  }

  if (q.isLoading) {
    return (
      <AgentPage title={t('profileTitle')}>
        <StateView kind="loading" title={t('loading')} />
      </AgentPage>
    );
  }

  if (q.isError) {
    return (
      <AgentPage title={t('profileTitle')}>
        <StateView kind="error" title={t('errorGeneric')} onRetry={() => void q.refetch()} />
      </AgentPage>
    );
  }

  return (
    <AgentPage title={t('profileTitle')}>
      <div className="rounded-xl border border-border bg-surface p-5">
        <div className="flex flex-col gap-4">
          {/* Name — read-only (statutory field) */}
          <FormField label={t('profileName')} htmlFor="profile-name">
            <Input id="profile-name" value={emp?.full_name ?? '—'} disabled />
          </FormField>

          {/* Phone — agent-editable */}
          <FormField label={t('profilePhone')} htmlFor="profile-phone">
            <Input
              id="profile-phone"
              type="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
            />
          </FormField>

          {/* Address — agent-editable */}
          <FormField label={t('profileAddress')} htmlFor="profile-address" span={2}>
            <textarea
              id="profile-address"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              rows={3}
              className="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:cursor-not-allowed disabled:opacity-50"
            />
          </FormField>

          {/* "pending HR approval" note — shown once a request has been submitted */}
          {create.isSuccess && <p className="text-sm text-text-2">{t('profileChangePending')}</p>}

          <div className="flex justify-end">
            <Button variant="primary" disabled={create.isPending} onClick={() => void onSave()}>
              {t('profileSave')}
            </Button>
          </div>
        </div>
      </div>
    </AgentPage>
  );
}
