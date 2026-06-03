/**
 * E2 · Perusahaan Klien — Detail
 *
 * .pen frames:
 *   OmuQT — E2 · Perusahaan Klien — Detail (Plaza Senayan)  [geofence_active = true]
 *   oYgYe — E2 · Perusahaan Klien — Detail (Geofence Disabled) [geo = null → geofence_active = false]
 *
 * Layout: BackRow → HeaderCard (name+status+actions) → Tabs (Profil | Lokasi & Geofence |
 * Penempatan Aktif | Pemimpin Shift | Riwayat) → two-col content.
 *
 * Active tab = "Profil" (left col: profile card) + "Lokasi & Geofence" (right col, 520px).
 * Geofence disabled banner (D11) shown in RightCol when geofence_active === false.
 * Map area = styled placeholder box (no real map lib per task spec).
 *
 * F2.3 — CC-2 (NPWP unique), CC-5 (active-placement guard).
 * INV: company is referenced by placements — only deactivate, never delete.
 */

import { classifyError } from '@/lib/api-error.ts';
import {
  type ClientCompany,
  ClientCompanyStatus,
  useGetClientCompany,
  useUpdateClientCompany,
} from '@swp/api-client/e2';
import { Banner, DateText, StateView, StatusBadge, useToast } from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { ArrowLeft, Building2, Edit2, Info, Map as MapIcon, MapPin, MapPinOff } from 'lucide-react';
import type * as React from 'react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { EditClientCompanyDrawer } from './client-company-form.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type DetailTab = 'profil' | 'lokasi' | 'penempatan' | 'pemimpin' | 'riwayat';

interface ClientCompanyDetailScreenProps {
  clientCompanyId: string;
}

// ---------------------------------------------------------------------------
// Field row for profile card
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Map placeholder box (matches .pen MapPreview - styled placeholder)
// ---------------------------------------------------------------------------

function MapPlaceholder({ geofenceActive }: { geofenceActive: boolean }) {
  const { t } = useTranslation('clientCompanies');
  return (
    <div className="relative h-[280px] bg-surface-2 border-y border-border-soft overflow-hidden">
      {/* Grid background pattern */}
      <div
        className="absolute inset-0"
        style={{
          backgroundImage:
            'linear-gradient(to right, #E5E7EB 1px, transparent 1px), linear-gradient(to bottom, #E5E7EB 1px, transparent 1px)',
          backgroundSize: '32px 32px',
        }}
        aria-hidden
      />
      {/* Geofence radius circle (when active) */}
      {geofenceActive && (
        <div
          className="absolute"
          style={{
            left: '50%',
            top: '50%',
            width: 160,
            height: 160,
            transform: 'translate(-50%, -50%)',
            borderRadius: '50%',
            background: 'rgba(24, 142, 77, 0.15)',
            border: '2px solid #188E4D',
          }}
          aria-hidden
        />
      )}
      {/* Pin icon */}
      <div
        className="absolute"
        style={{ left: '50%', top: '50%', transform: 'translate(-50%, -50%)' }}
      >
        <MapPin size={32} className="text-primary" aria-hidden />
      </div>
      {/* Hint text */}
      <p className="absolute bottom-2 left-0 right-0 text-center text-[11px] italic text-text-2">
        {t('detail.mapHint')}
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Geofence radius inline editor
// ---------------------------------------------------------------------------

function GeofenceRadiusEditor({ company }: { company: ClientCompany }) {
  const { t } = useTranslation('clientCompanies');
  const { toast } = useToast();
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(String(company.geofence_radius_m ?? 100));
  const updateMut = useUpdateClientCompany();

  function handleSave() {
    const num = Number(value);
    if (Number.isNaN(num) || num < 25 || num > 1000) {
      toast({ tone: 'warn', title: t('detail.geofenceRadiusRangeError') });
      return;
    }
    updateMut.mutate(
      { clientCompanyId: company.id, data: { geofence_radius_m: num } },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('toast.updated') });
          setEditing(false);
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t('toast.updateFailed'), description: message });
        },
      },
    );
  }

  return (
    <div className="flex flex-col gap-[6px]">
      <span className="text-[12px] font-medium text-text-2">{t('detail.geofenceRadius')}</span>
      <div className="flex items-center justify-between rounded-lg bg-surface border border-border px-3 py-[10px] gap-2">
        {editing ? (
          <>
            <input
              type="number"
              min={25}
              max={1000}
              className="flex-1 text-[13px] text-text bg-transparent outline-none font-mono"
              value={value}
              onChange={(e) => setValue(e.target.value)}
              aria-label={t('detail.geofenceRadius')}
            />
            <span className="text-[12px] text-text-2 mr-1">m</span>
            <button
              type="button"
              onClick={handleSave}
              disabled={updateMut.isPending}
              className="px-3 py-1 rounded-md bg-primary text-white text-[12px] font-medium hover:bg-primary-strong disabled:opacity-60"
            >
              {updateMut.isPending ? t('detail.saving') : t('detail.save')}
            </button>
            <button
              type="button"
              onClick={() => {
                setEditing(false);
                setValue(String(company.geofence_radius_m ?? 100));
              }}
              className="px-2 py-1 text-[12px] text-text-2 hover:text-text"
            >
              {t('detail.cancel')}
            </button>
          </>
        ) : (
          <>
            <span className="text-[13px] font-mono text-text flex-1">
              {company.geofence_radius_m ?? 100} m
            </span>
            <button
              type="button"
              aria-label={t('detail.editRadius')}
              onClick={() => setEditing(true)}
              className="text-text-2 hover:text-primary"
            >
              <Edit2 size={14} aria-hidden />
            </button>
          </>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main screen
// ---------------------------------------------------------------------------

export function ClientCompanyDetailScreen({ clientCompanyId }: ClientCompanyDetailScreenProps) {
  const { t } = useTranslation('clientCompanies');
  const [activeTab, setActiveTab] = useState<DetailTab>('profil');
  const [editOpen, setEditOpen] = useState(false);

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

  const geofenceActive = company.geofence_active ?? false;

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
          <button
            type="button"
            onClick={() => setEditOpen(true)}
            className="flex items-center gap-2 px-3 py-2 rounded-lg border border-border text-[13px] font-medium text-text hover:bg-surface-2"
          >
            <Edit2 size={14} aria-hidden />
            {t('actions.edit')}
          </button>
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

      {/* Content — Profil tab */}
      {activeTab === 'profil' && (
        <div className="flex gap-4">
          {/* Left col — profile card */}
          <div className="flex-1 flex flex-col gap-4">
            <div className="rounded-xl bg-surface border border-border overflow-hidden">
              <div className="px-5 pt-4 pb-3 flex items-center justify-between">
                <span className="text-[14px] font-semibold text-text">
                  {t('detail.profileCard.title')}
                </span>
              </div>
              <FieldRow label={t('fields.name')} value={company.name} />
              <FieldRow label={t('fields.npwp')} value={company.npwp} mono />
              <FieldRow label={t('fields.address')} value={company.address} />
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
          </div>

          {/* Right col — geofence card */}
          <div className="w-[520px] flex flex-col gap-4">
            <div className="rounded-xl bg-surface border border-border overflow-hidden">
              <div className="px-5 pt-4 pb-3 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <MapPin size={16} className="text-primary" aria-hidden />
                  <span className="text-[14px] font-semibold text-text">
                    {t('detail.geofenceCard.title')}
                  </span>
                </div>
                <StatusBadge tone={geofenceActive ? 'ok' : 'warn'} dot>
                  {geofenceActive
                    ? t('detail.geofenceCard.active')
                    : t('detail.geofenceCard.inactive')}
                </StatusBadge>
              </div>

              {/* Geofence disabled banner (D11) */}
              {!geofenceActive && (
                <div className="mx-5 mb-4">
                  <Banner
                    tone="warn"
                    icon={MapPinOff}
                    title={t('detail.geofenceBanner.title')}
                    description={t('detail.geofenceBanner.description')}
                  />
                </div>
              )}

              {/* Map placeholder */}
              <MapPlaceholder geofenceActive={geofenceActive} />

              {/* Geofence form fields */}
              <div className="flex flex-col gap-[14px] p-5">
                <div className="flex gap-3">
                  <div className="flex-1 flex flex-col gap-[6px]">
                    <span className="text-[12px] font-medium text-text-2">
                      {t('fields.latitude')}
                    </span>
                    <div className="rounded-lg border border-border px-3 py-[10px] text-[13px] font-mono text-text bg-surface">
                      {company.geo?.lat ?? <span className="text-text-3">—</span>}
                    </div>
                  </div>
                  <div className="flex-1 flex flex-col gap-[6px]">
                    <span className="text-[12px] font-medium text-text-2">
                      {t('fields.longitude')}
                    </span>
                    <div className="rounded-lg border border-border px-3 py-[10px] text-[13px] font-mono text-text bg-surface">
                      {company.geo?.lng ?? <span className="text-text-3">—</span>}
                    </div>
                  </div>
                </div>

                <GeofenceRadiusEditor company={company} />

                {/* Info note */}
                <div className="flex items-start gap-2 rounded-lg bg-info-bg border border-info-bd px-3 py-[10px]">
                  <Info size={14} className="text-info-tx shrink-0 mt-0.5" aria-hidden />
                  <p className="text-[11px] text-info-tx leading-relaxed">
                    {t('detail.geofenceNote')}
                  </p>
                </div>

                {/* Edit location button */}
                <button
                  type="button"
                  onClick={() => setEditOpen(true)}
                  className="flex items-center justify-center gap-2 rounded-lg border border-border bg-surface px-3 py-[10px] text-[13px] font-medium text-text hover:bg-surface-2"
                >
                  <MapIcon size={14} aria-hidden />
                  {t('detail.setLocation')}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Lokasi & Geofence tab — shows same geofence card full-width */}
      {activeTab === 'lokasi' && (
        <div className="flex gap-4">
          <div className="flex-1" />
          <div className="w-[520px] flex flex-col gap-4">
            <div className="rounded-xl bg-surface border border-border overflow-hidden">
              <div className="px-5 pt-4 pb-3 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <MapPin size={16} className="text-primary" aria-hidden />
                  <span className="text-[14px] font-semibold text-text">
                    {t('detail.geofenceCard.title')}
                  </span>
                </div>
                <StatusBadge tone={geofenceActive ? 'ok' : 'warn'} dot>
                  {geofenceActive
                    ? t('detail.geofenceCard.active')
                    : t('detail.geofenceCard.inactive')}
                </StatusBadge>
              </div>

              {!geofenceActive && (
                <div className="mx-5 mb-4">
                  <Banner
                    tone="warn"
                    icon={MapPinOff}
                    title={t('detail.geofenceBanner.title')}
                    description={t('detail.geofenceBanner.description')}
                  />
                </div>
              )}

              <MapPlaceholder geofenceActive={geofenceActive} />

              <div className="flex flex-col gap-[14px] p-5">
                <div className="flex gap-3">
                  <div className="flex-1 flex flex-col gap-[6px]">
                    <span className="text-[12px] font-medium text-text-2">
                      {t('fields.latitude')}
                    </span>
                    <div className="rounded-lg border border-border px-3 py-[10px] text-[13px] font-mono text-text bg-surface">
                      {company.geo?.lat ?? <span className="text-text-3">—</span>}
                    </div>
                  </div>
                  <div className="flex-1 flex flex-col gap-[6px]">
                    <span className="text-[12px] font-medium text-text-2">
                      {t('fields.longitude')}
                    </span>
                    <div className="rounded-lg border border-border px-3 py-[10px] text-[13px] font-mono text-text bg-surface">
                      {company.geo?.lng ?? <span className="text-text-3">—</span>}
                    </div>
                  </div>
                </div>

                <GeofenceRadiusEditor company={company} />

                <div className="flex items-start gap-2 rounded-lg bg-info-bg border border-info-bd px-3 py-[10px]">
                  <Info size={14} className="text-info-tx shrink-0 mt-0.5" aria-hidden />
                  <p className="text-[11px] text-info-tx leading-relaxed">
                    {t('detail.geofenceNote')}
                  </p>
                </div>

                <button
                  type="button"
                  onClick={() => setEditOpen(true)}
                  className="flex items-center justify-center gap-2 rounded-lg border border-border bg-surface px-3 py-[10px] text-[13px] font-medium text-text hover:bg-surface-2"
                >
                  <MapIcon size={14} aria-hidden />
                  {t('detail.setLocation')}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Placeholder tabs */}
      {(activeTab === 'penempatan' || activeTab === 'pemimpin' || activeTab === 'riwayat') && (
        <div className="flex items-center justify-center py-16 text-text-3 text-[14px]">
          {t('detail.tabComingSoon')}
        </div>
      )}

      {/* Role note */}
      <div className="flex items-center gap-[9px] rounded-lg bg-surface border border-border border-l-[3px] border-l-border px-[14px] py-[10px]">
        <Info size={15} className="text-text-3 shrink-0" aria-hidden />
        <p className="text-[12px] text-text-2">{t('detail.roleNote')}</p>
      </div>

      {/* Edit drawer */}
      {editOpen && (
        <EditClientCompanyDrawer
          clientCompanyId={clientCompanyId}
          open
          onClose={() => setEditOpen(false)}
          onSaved={() => {
            setEditOpen(false);
            query.refetch();
          }}
        />
      )}
    </div>
  );
}
