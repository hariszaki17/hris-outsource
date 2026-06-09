import { classifyError } from '@/lib/api-error.ts';
import {
  type PlatformSetting,
  type PlatformSettings,
  useGetPlatformSettings,
} from '@swp/api-client/e1';
import { EmptyState, Skeleton, StateView } from '@swp/ui';
import { Lock } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * E1 · Pengaturan Umum — read-only platform conventions screen (/settings/general).
 * Built from .pen frame `tch6k`. API: GET /platform/settings (useGetPlatformSettings).
 * All settings are read-only in v1 — no PUT endpoint; rows are non-interactive (no chevron).
 * Static sections (security, role navigation, about labels) are platform conventions (PC-1..6).
 * Refs: F1.4 platform-conventions.md (PC-1..6), EPICS.md §8 E1.
 */

// ---------------------------------------------------------------------------
// SettingRow — local read-only label/value/lock-chip row (not exported)
// ---------------------------------------------------------------------------

interface SettingRowProps {
  /** Left label — human-readable name. */
  label: string;
  /** Right value — the current setting value. */
  value: string;
  /** If true, show a lock chip. */
  locked?: boolean;
  /** Localized text for the lock chip. */
  lockedLabel: string;
  /** If true, render value in monospace. */
  mono?: boolean;
}

function SettingRow({ label, value, locked = false, lockedLabel, mono = false }: SettingRowProps) {
  return (
    <div className="flex items-center justify-between gap-4 py-[12px]">
      <div className="flex min-w-0 flex-1 flex-col gap-[2px]">
        <span className="text-[13px] font-semibold leading-snug text-text-2">{label}</span>
        <span
          className={['text-[13px] leading-snug text-text', mono ? 'font-mono' : '']
            .filter(Boolean)
            .join(' ')}
        >
          {value}
        </span>
      </div>
      {/* All settings are read-only in v1 (no PUT endpoint) — rows are never navigable, so no
          chevron affordance. `locked` rows additionally surface a lock chip. */}
      {locked ? (
        <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-border bg-surface-2 px-[9px] py-[3px] text-[11px] font-medium text-text-3">
          <Lock className="size-[10px]" aria-hidden />
          <span>{lockedLabel}</span>
        </span>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// SettingCard — section card shell
// ---------------------------------------------------------------------------

interface SettingCardProps {
  title: string;
  children: React.ReactNode;
}

function SettingCard({ title, children }: SettingCardProps) {
  return (
    <div className="rounded-xl border border-border bg-surface">
      {/* Card header */}
      <div className="border-b border-border px-[18px] py-[14px]">
        <p className="text-[13px] font-semibold uppercase tracking-wide text-text-3">{title}</p>
      </div>
      {/* Card body — rows are separated by soft dividers */}
      <div className="divide-y divide-border-soft px-[18px]">{children}</div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SkeletonSettingRows — loading shimmer for API-fed card rows
// ---------------------------------------------------------------------------

function SkeletonSettingRows({ count = 4 }: { count?: number }) {
  return (
    <>
      {Array.from({ length: count }, (_, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: static decorative placeholder rows, never reordered
        <div key={i} className="flex items-center justify-between gap-4 py-[12px]">
          <div className="flex flex-1 flex-col gap-[6px]">
            <Skeleton className="h-[11px] w-28" />
            <Skeleton className="h-[13px] w-44" />
          </div>
          <Skeleton className="h-[18px] w-16 rounded-full" />
        </div>
      ))}
    </>
  );
}

// ---------------------------------------------------------------------------
// SettingsGeneralScreen
// ---------------------------------------------------------------------------

export function SettingsGeneralScreen() {
  const { t } = useTranslation();
  const query = useGetPlatformSettings();

  // Error / permission gate — if the platform query fails with forbidden/unauthenticated,
  // show no-permission for the whole page (static cards below the API gate still hidden).
  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <div className="flex flex-col gap-[18px]">
          <div className="flex flex-col gap-1">
            <h1 className="font-bold text-3xl text-text">{t('settingsGeneral.title')}</h1>
            <p className="text-sm text-text-3">{t('settingsGeneral.subtitle')}</p>
          </div>
          <EmptyState
            variant="no-permission"
            title={t('errors.forbidden')}
            description={t('settingsGeneral.noPermissionBody')}
          />
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="font-bold text-3xl text-text">{t('settingsGeneral.title')}</h1>
          <p className="text-sm text-text-3">{t('settingsGeneral.subtitle')}</p>
        </div>
        <StateView
          kind="error"
          title={t('settingsGeneral.errorTitle')}
          description={t('errors.network')}
          onRetry={() => query.refetch()}
          retryLabel={t('common.retry')}
        />
      </div>
    );
  }

  // Orval wraps the response — body is under `.data`.
  const settings = query.data?.data as PlatformSettings | undefined;

  const lockedLabel = t('settingsGeneral.locked');

  // Helper: render a SettingRow from a PlatformSetting field (API-fed).
  const apiRow = (setting: PlatformSetting | undefined, mono = false) => {
    if (!setting) return null;
    return (
      <SettingRow
        key={setting.value}
        label={setting.label}
        value={setting.value}
        locked={setting.locked}
        lockedLabel={lockedLabel}
        mono={mono}
      />
    );
  };

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band */}
      <div className="flex flex-col gap-1">
        <h1 className="font-bold text-3xl text-text">{t('settingsGeneral.title')}</h1>
        <p className="text-sm text-text-3">{t('settingsGeneral.subtitle')}</p>
      </div>

      {/* Two-column layout */}
      <div className="flex gap-5">
        {/* Left column */}
        <div className="flex min-w-0 flex-1 flex-col gap-[18px]">
          {/* Lokalisasi card — API-fed */}
          <SettingCard title={t('settingsGeneral.section.localization')}>
            {query.isLoading ? (
              <SkeletonSettingRows count={4} />
            ) : (
              <>
                {apiRow(settings?.locale)}
                {apiRow(settings?.timezone)}
                {apiRow(settings?.date_format)}
                {apiRow(settings?.currency)}
              </>
            )}
          </SettingCard>

          {/* Keamanan card — static platform conventions (PC-4) */}
          <SettingCard title={t('settingsGeneral.section.security')}>
            <SettingRow
              label={t('settingsGeneral.security.passwordPolicy')}
              value={t('settingsGeneral.security.passwordPolicyValue')}
              lockedLabel={lockedLabel}
            />
            <SettingRow
              label={t('settingsGeneral.security.accountLockout')}
              value={t('settingsGeneral.security.accountLockoutValue')}
              lockedLabel={lockedLabel}
            />
            <SettingRow
              label={t('settingsGeneral.security.session')}
              value={t('settingsGeneral.security.sessionValue')}
              lockedLabel={lockedLabel}
            />
          </SettingCard>
        </div>

        {/* Right column — fixed width 420 */}
        <div className="flex w-[420px] shrink-0 flex-col gap-[18px]">
          {/* Navigasi berbasis peran — static (EPICS §8 E1 role scope) */}
          <SettingCard title={t('settingsGeneral.section.roleNav')}>
            <SettingRow
              label={t('role.super_admin')}
              value={t('settingsGeneral.roleNav.superAdmin')}
              lockedLabel={lockedLabel}
            />
            <SettingRow
              label={t('role.hr_admin')}
              value={t('settingsGeneral.roleNav.hrAdmin')}
              lockedLabel={lockedLabel}
            />
            <SettingRow
              label={t('role.shift_leader')}
              value={t('settingsGeneral.roleNav.shiftLeader')}
              lockedLabel={lockedLabel}
            />
            <SettingRow
              label={t('role.agent')}
              value={t('settingsGeneral.roleNav.agent')}
              lockedLabel={lockedLabel}
            />
          </SettingCard>

          {/* Tentang — API-fed */}
          <SettingCard title={t('settingsGeneral.section.about')}>
            {query.isLoading ? (
              <SkeletonSettingRows count={3} />
            ) : (
              <>
                {apiRow(settings?.version, true)}
                {apiRow(settings?.stack, true)}
                {apiRow(settings?.legacy_data_source, true)}
              </>
            )}
          </SettingCard>
        </div>
      </div>
    </div>
  );
}
