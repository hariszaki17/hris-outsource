import { useCurrentUser } from '@/lib/use-auth.ts';
import { EmptyState } from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { ArrowRight, CalendarOff, Clock3, Info, Timer } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * E2 · Data Master — Hub (F2.x operational master data).
 * Built from .pen frame `f8mBr`.
 * Three cards: Jenis Cuti, Kode Kehadiran, Aturan Lembur.
 * Static nav — no async data.
 * Refs: MD-1, MD-2, LT-1..3, AC-1..3, OR-1..3.
 */

// ---------------------------------------------------------------------------
// MasterDataCard
// ---------------------------------------------------------------------------

interface MasterDataCardProps {
  href:
    | '/master-data/leave-types'
    | '/master-data/attendance-codes'
    | '/master-data/overtime-rules';
  icon: React.ComponentType<{ className?: string; 'aria-hidden'?: boolean }>;
  iconBg: string;
  iconColor: string;
  heading: string;
  description: string;
  linkLabel: string;
}

function MasterDataCard({
  href,
  icon: Icon,
  iconBg,
  iconColor,
  heading,
  description,
  linkLabel,
}: MasterDataCardProps) {
  return (
    <Link
      to={href}
      className="group flex flex-col gap-[14px] rounded-xl border border-border bg-surface p-5 transition-shadow hover:shadow-card focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
    >
      {/* Icon badge */}
      <div
        className={`flex size-[46px] shrink-0 items-center justify-center rounded-[10px] ${iconBg}`}
      >
        <Icon className={`size-6 ${iconColor}`} aria-hidden />
      </div>

      {/* Heading + description */}
      <div className="flex flex-col gap-1.5">
        <p className="text-[18px] font-semibold leading-snug text-text">{heading}</p>
        <p className="text-[13px] leading-[1.5] text-text-2">{description}</p>
      </div>

      {/* Footer: Kelola link */}
      <div className="flex items-center justify-end border-t border-border-soft pt-3">
        <div className="flex items-center gap-[6px] text-[13px] font-semibold text-primary">
          {linkLabel}
          <ArrowRight className="size-[14px]" aria-hidden />
        </div>
      </div>
    </Link>
  );
}

// ---------------------------------------------------------------------------
// MasterDataHubScreen
// ---------------------------------------------------------------------------

export function MasterDataHubScreen() {
  const { t } = useTranslation();
  const user = useCurrentUser();

  // Defense-in-depth (ENGINEERING.md C1): this hub has no API call to surface a 403, so the
  // capability check is done client-side here. Master data is Super Admin/HR only
  // (rbac.ts `masterdata.manage`); SL/agent get the no-permission state, not a live hub.
  const canManage = user?.permissions.includes('masterdata.manage') ?? false;
  if (!canManage) {
    return (
      <EmptyState
        variant="no-permission"
        title={t('errors.forbidden')}
        description={t('masterData.noPermission')}
      />
    );
  }

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-[20px] font-semibold text-text">{t('masterData.title')}</h1>
          <p className="text-[13px] text-text-2">{t('masterData.subtitle')}</p>
        </div>
      </div>

      {/* Role note banner */}
      <div className="flex items-center gap-[9px] rounded-lg border border-l-[3px] border-border bg-surface px-[14px] py-[10px]">
        <Info className="size-[15px] shrink-0 text-text-3" aria-hidden />
        <p className="text-[12px] text-text-2">{t('masterData.roleNote')}</p>
      </div>

      {/* Three cards grid */}
      <div className="grid grid-cols-3 gap-4">
        <MasterDataCard
          href="/master-data/leave-types"
          icon={CalendarOff}
          iconBg="bg-info-bg"
          iconColor="text-info-tx"
          heading={t('masterData.hub.leaveTypes.heading')}
          description={t('masterData.hub.leaveTypes.description')}
          linkLabel={t('masterData.hub.manage')}
        />
        <MasterDataCard
          href="/master-data/attendance-codes"
          icon={Clock3}
          iconBg="bg-primary-soft"
          iconColor="text-primary"
          heading={t('masterData.hub.attendanceCodes.heading')}
          description={t('masterData.hub.attendanceCodes.description')}
          linkLabel={t('masterData.hub.manage')}
        />
        <MasterDataCard
          href="/master-data/overtime-rules"
          icon={Timer}
          iconBg="bg-warn-bg"
          iconColor="text-warn-tx"
          heading={t('masterData.hub.overtimeRules.heading')}
          description={t('masterData.hub.overtimeRules.description')}
          linkLabel={t('masterData.hub.manage')}
        />
      </div>
    </div>
  );
}
