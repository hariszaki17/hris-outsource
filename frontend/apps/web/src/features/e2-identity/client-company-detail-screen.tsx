/**
 * E2 · Perusahaan Klien — Detail
 *
 * Layout: BackRow → HeaderCard (name+status+actions) → Tabs (Profil | Lokasi & Site |
 * Penempatan Aktif | Pemimpin Shift | Riwayat) → content.
 *
 * Geofence config moved off the company onto its Sites (F2.6, 2026-06-03): the "Lokasi & Site"
 * tab renders `SitesPanel` (list + add/edit with the map geofence editor). The Profil tab shows
 * only the company profile card (statutory/billing fields + `leader_scope`).
 *
 * F2.3 — CC-2 (NPWP unique), CC-5 (active-placement guard). INV: company is referenced by
 * placements — only deactivate, never delete.
 */

import { classifyError } from '@/lib/api-error.ts';
import { useCurrentUser } from '@/lib/use-auth.ts';
import { type ClientCompany, ClientCompanyStatus, useGetClientCompany } from '@swp/api-client/e2';
import { DateText, EmptyState, StateView, StatusBadge } from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { ArrowLeft, Building2, Edit2, Info } from 'lucide-react';
import type * as React from 'react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  PemimpinShiftPanel,
  PenempatanAktifPanel,
  RiwayatPanel,
} from './client-company-tab-panels.tsx';
import { SitesPanel } from './site-form.tsx';

type DetailTab = 'profil' | 'lokasi' | 'penempatan' | 'pemimpin' | 'riwayat';

interface ClientCompanyDetailScreenProps {
  clientCompanyId: string;
}

function FieldRow({
  label,
  value,
  mono = false,
}: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between px-5 py-[10px] border-t border-border-soft">
      <span className="text-[12px] font-medium text-text-2">{label}</span>
      <span className={`text-[13px] text-text ${mono ? 'font-mono font-medium' : ''}`}>
        {value ?? '—'}
      </span>
    </div>
  );
}

export function ClientCompanyDetailScreen({ clientCompanyId }: ClientCompanyDetailScreenProps) {
  const { t } = useTranslation('clientCompanies');
  const [activeTab, setActiveTab] = useState<DetailTab>('profil');
  const currentUser = useCurrentUser();
  // Edit/deactivate are clients.write (hr_admin/super_admin). A company-scoped shift_leader can
  // legitimately open this read-only detail (getClientCompany is company_or_global), so gate the
  // write affordance rather than the whole screen — defense-in-depth, no dead-flow (A2).
  const canWrite = currentUser?.role === 'hr_admin' || currentUser?.role === 'super_admin';

  const query = useGetClientCompany(clientCompanyId);
  const company = query.data?.data as ClientCompany | undefined;

  const tabs: { id: DetailTab; label: string }[] = [
    { id: 'profil', label: t('detail.tabs.profil') },
    { id: 'lokasi', label: t('detail.tabs.lokasi') },
    { id: 'penempatan', label: t('detail.tabs.penempatan') },
    { id: 'pemimpin', label: t('detail.tabs.pemimpin') },
    { id: 'riwayat', label: t('detail.tabs.riwayat') },
  ];

  if (query.isPending) {
    return (
      <div className="p-6 bg-app-bg h-full">
        <StateView kind="loading" title={t('state.loading')} />
      </div>
    );
  }

  if (query.isError || !company) {
    const { kind, message } = classifyError(query.error);
    // A 403 is reachable WITH valid capability: a shift_leader hitting a company outside their
    // server-side scope (or a role without clients.read deep-linking). Show a no-permission
    // state with no Retry, mirroring the list screens.
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <div className="p-6 bg-app-bg h-full">
          <EmptyState
            variant="no-permission"
            title={t('state.noPermissionTitle')}
            description={t('state.noPermissionBody')}
          />
        </div>
      );
    }
    return (
      <div className="p-6 bg-app-bg h-full">
        <StateView
          kind={kind === 'not-found' ? 'empty' : 'error'}
          title={kind === 'not-found' ? t('state.notFound') : t('state.errorTitle')}
          description={message}
          onRetry={kind !== 'not-found' ? () => void query.refetch() : undefined}
        />
      </div>
    );
  }

  const leaderScopeLabel =
    company.leader_scope === 'site' ? t('fields.leaderScopeSite') : t('fields.leaderScopeCompany');

  return (
    <div className="flex flex-col gap-4 p-6 bg-app-bg h-full overflow-y-auto">
      {/* Back row */}
      <div className="flex items-center gap-[7px]">
        <Link
          to={'/client-companies' as never}
          className="flex items-center gap-[7px] text-text-2 hover:text-text"
        >
          <ArrowLeft size={16} aria-hidden />
          <span className="text-[13px] font-medium">{t('detail.backLink')}</span>
        </Link>
      </div>

      {/* Header card */}
      <div className="flex items-center justify-between rounded-xl bg-surface border border-border p-5">
        <div className="flex items-center gap-4">
          <div className="flex items-center justify-center w-11 h-11 rounded-xl bg-surface-2 shrink-0">
            <Building2 size={22} className="text-text-2" aria-hidden />
          </div>
          <div className="flex flex-col gap-1">
            <h1 className="text-[18px] font-semibold text-text">{company.name}</h1>
            <div className="flex items-center gap-2">
              <span className="text-[12px] text-text-2">{company.address}</span>
              {company.npwp && (
                <span className="text-[11px] font-mono text-text-3">{company.npwp}</span>
              )}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-[10px]">
          <StatusBadge tone={company.status === ClientCompanyStatus.ACTIVE ? 'ok' : 'bad'} dot>
            {company.status === ClientCompanyStatus.ACTIVE
              ? t('status.active')
              : t('status.inactive')}
          </StatusBadge>
          {canWrite && (
            <Link
              to={'/client-companies/$clientCompanyId/edit' as never}
              params={{ clientCompanyId } as never}
              className="flex items-center gap-2 px-3 py-2 rounded-lg border border-border text-[13px] font-medium text-text hover:bg-surface-2"
            >
              <Edit2 size={14} aria-hidden />
              {t('actions.edit')}
            </Link>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-7 border-b border-border">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            onClick={() => setActiveTab(tab.id)}
            className={[
              'py-3 text-[14px] font-medium border-b-2 -mb-px transition-colors',
              activeTab === tab.id
                ? 'border-primary text-primary font-semibold'
                : 'border-transparent text-text-2 hover:text-text',
            ].join(' ')}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Profil tab — company profile card (full width); Sites live on the Lokasi tab */}
      {activeTab === 'profil' && (
        <div className="rounded-xl bg-surface border border-border overflow-hidden">
          <div className="px-5 pt-4 pb-3 flex items-center justify-between">
            <span className="text-[14px] font-semibold text-text">
              {t('detail.profileCard.title')}
            </span>
          </div>
          <FieldRow label={t('fields.name')} value={company.name} />
          <FieldRow label={t('fields.npwp')} value={company.npwp} mono />
          <FieldRow label={t('fields.address')} value={company.address} />
          <FieldRow label={t('fields.leaderScope')} value={leaderScopeLabel} />
          <FieldRow label={t('fields.pic')} value={company.pic_name} />
          <FieldRow label={t('fields.phone')} value={company.phone} mono />
          <FieldRow label={t('fields.email')} value={company.email} />
          {company.created_at && (
            <FieldRow
              label={t('fields.createdAt')}
              value={<DateText value={company.created_at} kind="instant" />}
            />
          )}
        </div>
      )}

      {/* Lokasi & Site tab — full sites panel */}
      {activeTab === 'lokasi' && <SitesPanel clientCompanyId={clientCompanyId} />}

      {/* Penempatan Aktif — active agent roster at this company */}
      {activeTab === 'penempatan' && <PenempatanAktifPanel clientCompanyId={clientCompanyId} />}

      {/* Pemimpin Shift — current leader + assign/replace/revoke (single entry point) */}
      {activeTab === 'pemimpin' && (
        <PemimpinShiftPanel clientCompanyId={clientCompanyId} companyName={company.name} />
      )}

      {/* Riwayat — historical placements at this company */}
      {activeTab === 'riwayat' && <RiwayatPanel clientCompanyId={clientCompanyId} />}

      {/* Role note */}
      <div className="flex items-center gap-[9px] rounded-lg bg-surface border border-border border-l-[3px] border-l-border px-[14px] py-[10px]">
        <Info size={15} className="text-text-3 shrink-0" aria-hidden />
        <p className="text-[12px] text-text-2">{t('detail.roleNote')}</p>
      </div>
    </div>
  );
}
