import { Link } from '@tanstack/react-router';
import { ArrowRight, Info, ScrollText, Settings, UsersRound } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * E1 · Pengaturan Ringkasan — static navigation overview for the /settings index route.
 * Built from .pen frame `E7WOwh`. No async data — pure static nav.
 * Refs: F1.2 (users/RBAC), F1.3 (audit log), F1.4 (platform conventions).
 */

// ---------------------------------------------------------------------------
// SettingCard — one overview card linking to a settings sub-section
// ---------------------------------------------------------------------------

interface SettingCardProps {
  to: string;
  icon: React.ComponentType<{ className?: string; 'aria-hidden'?: boolean }>;
  chipLabel: string;
  heading: string;
  description: string;
  linkLabel: string;
}

function SettingCard({
  to,
  icon: Icon,
  chipLabel,
  heading,
  description,
  linkLabel,
}: SettingCardProps) {
  return (
    <Link
      to={to}
      className="group flex flex-col gap-[14px] rounded-xl border border-border bg-surface p-[22px] transition-shadow hover:shadow-card focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
    >
      {/* Card header row: icon wrap + chip */}
      <div className="flex items-start justify-between">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-[10px] bg-primary-soft">
          <Icon className="size-5 text-primary" aria-hidden />
        </div>
        <span className="inline-flex items-center rounded-full border border-border bg-surface-2 px-[9px] py-[3px] text-[12px] font-medium text-text-2">
          {chipLabel}
        </span>
      </div>

      {/* Heading + description */}
      <div className="flex flex-col gap-[6px]">
        <p className="text-[18px] font-bold leading-snug text-text">{heading}</p>
        <p className="text-[13px] leading-[1.5] text-text-3">{description}</p>
      </div>

      {/* "Buka" link row */}
      <div className="flex items-center gap-1 text-[13px] font-semibold text-primary">
        {linkLabel}
        <ArrowRight className="size-[13px]" aria-hidden />
      </div>
    </Link>
  );
}

// ---------------------------------------------------------------------------
// SettingsOverviewScreen
// ---------------------------------------------------------------------------

export function SettingsOverviewScreen() {
  const { t } = useTranslation();

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band */}
      <div className="flex flex-col gap-1">
        <h1 className="font-bold text-3xl text-text">{t('settingsOverview.title')}</h1>
        <p className="text-sm text-text-3">{t('settingsOverview.subtitle')}</p>
      </div>

      {/* Three equal cards */}
      <div className="grid grid-cols-3 gap-[18px]">
        <SettingCard
          to="/settings/users"
          icon={UsersRound}
          chipLabel={t('settingsOverview.card.users.chip')}
          heading={t('settingsOverview.card.users.heading')}
          description={t('settingsOverview.card.users.description')}
          linkLabel={t('settingsOverview.card.open')}
        />
        <SettingCard
          to="/settings/audit-log"
          icon={ScrollText}
          chipLabel={t('settingsOverview.card.auditLog.chip')}
          heading={t('settingsOverview.card.auditLog.heading')}
          description={t('settingsOverview.card.auditLog.description')}
          linkLabel={t('settingsOverview.card.open')}
        />
        <SettingCard
          to="/settings/general"
          icon={Settings}
          chipLabel={t('settingsOverview.card.general.chip')}
          heading={t('settingsOverview.card.general.heading')}
          description={t('settingsOverview.card.general.description')}
          linkLabel={t('settingsOverview.card.open')}
        />
      </div>

      {/* Footer info banner */}
      <div className="flex items-start gap-3 rounded-[10px] border border-info-bd bg-info-bg px-4 py-[14px]">
        <Info className="mt-px size-4 shrink-0 text-info-tx" aria-hidden />
        <p className="text-[13px] leading-[1.5] text-info-tx">{t('settingsOverview.lockedNote')}</p>
      </div>
    </div>
  );
}
