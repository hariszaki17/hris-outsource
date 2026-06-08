/**
 * E7 · Aturan OT & Kalender Libur (HR) — combined OT-rule reference + holiday calendar.
 *
 * .pen frame implemented:
 *   vd4na  "E7 · Aturan OT & Kalender Libur (HR)"
 *     Left  — "Tier Tipe-Hari" table: each OT rule (E2 master) expanded into its 3 day-type
 *             tiers (Hari Kerja / Hari Libur / Hari Besar) with ×mult (ref), MIN, pra-approval.
 *     Right — "Kalender Hari Libur" with full CRUD (E7 /holidays).
 *
 * Route: /overtime/aturan (role: hr_admin | super_admin)
 *
 * OT-rule master CRUD itself lives in E2 (`/master-data/overtime-rules`); the "Tambah Aturan"
 * and row-edit affordances link there (this page is the OT-centric read view + holiday owner).
 */
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  type ListOvertimeRules200,
  type OvertimeRule,
  useListOvertimeRules,
} from '@swp/api-client/e2';
import { type Holiday, type HolidayPage, useListHolidays } from '@swp/api-client/e7';
import { Button, DateText, EmptyState, StateView, StatusBadge } from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { CalendarPlus, Pencil, Plus, Trash2 } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { HolidayImportModal } from './holiday-import-overlay.tsx';
import { DeleteHolidayConfirm, HolidayFormModal } from './holiday-overlays.tsx';
import { holidayCategoryTone } from './overtime-shared.tsx';

// ---------------------------------------------------------------------------
// Tier-row projection
// ---------------------------------------------------------------------------

type TierKey = 'workday' | 'restday' | 'holiday';

interface TierRow {
  key: string;
  rule: OvertimeRule;
  tier: TierKey;
  rate: number;
}

function expandTiers(rules: OvertimeRule[]): TierRow[] {
  const out: TierRow[] = [];
  for (const rule of rules) {
    out.push({ key: `${rule.id}-workday`, rule, tier: 'workday', rate: rule.weekday_rate });
    out.push({ key: `${rule.id}-restday`, rule, tier: 'restday', rate: rule.restday_rate });
    out.push({ key: `${rule.id}-holiday`, rule, tier: 'holiday', rate: rule.holiday_rate });
  }
  return out;
}

const TIER_LABEL_KEY: Record<TierKey, string> = {
  workday: 'rules.tierWorkday',
  restday: 'rules.tierRestday',
  holiday: 'rules.tierHoliday',
};

/** Format a multiplier the Indonesian way: ×1,5 */
function formatMult(rate: number): string {
  return `×${rate.toLocaleString('id-ID', { minimumFractionDigits: 1 })}`;
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function OvertimeRulesScreen() {
  const { t } = useTranslation('overtime');
  const user = useCurrentUser();
  const canManage = user?.role === 'hr_admin' || user?.role === 'super_admin';

  const [holidayModalOpen, setHolidayModalOpen] = useState(false);
  const [editingHoliday, setEditingHoliday] = useState<Holiday | null>(null);
  const [deletingHoliday, setDeletingHoliday] = useState<Holiday | null>(null);
  const [importOpen, setImportOpen] = useState(false);

  const rulesQuery = useListOvertimeRules({ limit: 200 });
  const holidaysQuery = useListHolidays({ limit: 200 });

  const rulesPage = rulesQuery.data?.data as ListOvertimeRules200 | undefined;
  const rules = (rulesPage?.data ?? []) as OvertimeRule[];
  const tierRows = useMemo(() => expandTiers(rules), [rules]);

  const holidaysPage = holidaysQuery.data?.data as HolidayPage | undefined;
  const holidays = holidaysPage?.data ?? [];

  if (!canManage) {
    return (
      <div className="p-6">
        <EmptyState
          variant="no-permission"
          title={t('rules.noPermissionTitle')}
          description={t('rules.noPermissionBody')}
        />
      </div>
    );
  }

  const openAddHoliday = () => {
    setEditingHoliday(null);
    setHolidayModalOpen(true);
  };
  const openEditHoliday = (h: Holiday) => {
    setEditingHoliday(h);
    setHolidayModalOpen(true);
  };

  return (
    <div className="flex flex-col gap-[18px] bg-app-bg p-6">
      {/* Title band */}
      <div>
        <h1 className="text-[30px] font-bold text-text">{t('rules.title')}</h1>
        <p className="mt-1 max-w-[760px] text-[14px] text-text-3">{t('rules.subtitle')}</p>
      </div>

      <div className="flex gap-5">
        {/* Left — Tier Tipe-Hari table */}
        <section className="flex-1 overflow-hidden rounded-[12px] border border-border bg-surface">
          <header className="flex items-center justify-between border-b border-border-soft px-[18px] py-[14px]">
            <h2 className="text-[15px] font-bold text-text">{t('rules.cardTitle')}</h2>
            <Button asChild variant="secondary" size="sm">
              <Link to="/master-data/overtime-rules">
                <Plus className="size-[13px]" aria-hidden="true" />
                {t('rules.addRule')}
              </Link>
            </Button>
          </header>

          {rulesQuery.isError ? (
            <div className="p-5">
              <StateView
                kind="error"
                title={t('rules.errorTitle')}
                onRetry={() => rulesQuery.refetch()}
                retryLabel={t('common.retry')}
              />
            </div>
          ) : rulesQuery.isLoading ? (
            <div className="p-5">
              <StateView kind="loading" title={t('common.loading')} />
            </div>
          ) : tierRows.length === 0 ? (
            <div className="p-5">
              <EmptyState
                variant="fresh"
                title={t('rules.emptyTitle')}
                description={t('rules.emptyBody')}
              />
            </div>
          ) : (
            <div>
              {/* Column header */}
              <div className="flex bg-surface-2 px-[18px] py-[10px] text-[10px] font-semibold tracking-[0.4px] text-text-3">
                <span className="w-[170px]">{t('rules.colDayType')}</span>
                <span className="w-[130px]">{t('rules.colLine')}</span>
                <span className="w-[90px]">{t('rules.colMult')}</span>
                <span className="w-[80px]">{t('rules.colMin')}</span>
                <span className="w-[110px]">{t('rules.colPreApproval')}</span>
                <span className="w-[80px]">{t('rules.colStatus')}</span>
              </div>
              {tierRows.map((row) => (
                <div
                  key={row.key}
                  className="flex items-center border-b border-border-soft px-[18px] py-3 last:border-b-0"
                >
                  <span className="w-[170px] text-[13px] font-semibold text-text">
                    {t(TIER_LABEL_KEY[row.tier])}
                  </span>
                  <span className="w-[130px] text-[13px] text-text-3">
                    {row.rule.service_line_id ? row.rule.name : t('rules.scopeGlobal')}
                  </span>
                  <span className="w-[90px] font-mono text-[13px] font-semibold text-text">
                    {formatMult(row.rate)}
                  </span>
                  <span className="w-[80px] font-mono text-[13px] text-text-2">
                    {row.rule.min_minutes}m
                  </span>
                  <span className="w-[110px] text-[13px] text-text-3">
                    {row.rule.pre_approval_required ? t('common.yes') : t('common.no')}
                  </span>
                  <span className="w-[80px]">
                    <StatusBadge dot tone={row.rule.status === 'ACTIVE' ? 'ok' : 'neutral'}>
                      {row.rule.status === 'ACTIVE'
                        ? t('rules.statusActive')
                        : t('rules.statusInactive')}
                    </StatusBadge>
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>

        {/* Right — Kalender Hari Libur */}
        <section className="w-[380px] overflow-hidden rounded-[12px] border border-border bg-surface">
          <header className="flex items-center justify-between border-b border-border-soft px-[18px] py-[14px]">
            <h2 className="text-[15px] font-bold text-text">{t('holidays.cardTitle')}</h2>
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => setImportOpen(true)}
                className="flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-primary hover:bg-surface-2"
              >
                <CalendarPlus className="size-[15px]" aria-hidden="true" />
                {t('holidays.import.button')}
              </button>
              <button
                type="button"
                onClick={openAddHoliday}
                aria-label={t('holidays.addTitle')}
                className="flex size-7 items-center justify-center rounded-md text-primary hover:bg-surface-2"
              >
                <Plus className="size-[18px]" aria-hidden="true" />
              </button>
            </div>
          </header>

          {holidaysQuery.isError ? (
            <div className="p-5">
              <StateView
                kind="error"
                title={t('holidays.errorTitle')}
                onRetry={() => holidaysQuery.refetch()}
                retryLabel={t('common.retry')}
              />
            </div>
          ) : holidaysQuery.isLoading ? (
            <div className="p-5">
              <StateView kind="loading" title={t('common.loading')} />
            </div>
          ) : holidays.length === 0 ? (
            <div className="p-5">
              <EmptyState
                variant="fresh"
                title={t('holidays.emptyTitle')}
                description={t('holidays.emptyBody')}
              />
            </div>
          ) : (
            <ul>
              {holidays.map((h) => (
                <li
                  key={h.id}
                  className="group flex items-center gap-3 border-b border-border-soft px-[18px] py-3 last:border-b-0"
                >
                  <div className="flex h-10 w-[54px] flex-col items-center justify-center rounded-lg bg-warn-bg">
                    <DateText
                      kind="date"
                      value={h.date}
                      options={{ day: '2-digit', month: 'short' }}
                      className="text-[12px] font-bold text-warn-tx"
                    />
                  </div>
                  <div className="flex flex-1 flex-col gap-0.5">
                    <span className="text-[13px] font-semibold text-text">{h.name}</span>
                    <span className="text-[11px] text-text-3">
                      {h.recurring
                        ? t('holidays.recurringYearly')
                        : t(`holidayCategory.${h.category}`)}
                    </span>
                  </div>
                  <StatusBadge tone={holidayCategoryTone(h.category)}>
                    {t(`holidayCategory.${h.category}`)}
                  </StatusBadge>
                  <div className="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                    <button
                      type="button"
                      onClick={() => openEditHoliday(h)}
                      aria-label={t('holidays.editTitle')}
                      className="flex size-7 items-center justify-center rounded-md text-text-2 hover:bg-surface-2"
                    >
                      <Pencil className="size-[14px]" aria-hidden="true" />
                    </button>
                    <button
                      type="button"
                      onClick={() => setDeletingHoliday(h)}
                      aria-label={t('holidays.deleteTitle')}
                      className="flex size-7 items-center justify-center rounded-md text-bad-tx hover:bg-bad-bg"
                    >
                      <Trash2 className="size-[14px]" aria-hidden="true" />
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      <HolidayFormModal
        editing={editingHoliday}
        open={holidayModalOpen}
        onClose={() => setHolidayModalOpen(false)}
        onSaved={() => holidaysQuery.refetch()}
      />
      <DeleteHolidayConfirm
        holiday={deletingHoliday}
        open={deletingHoliday !== null}
        onClose={() => setDeletingHoliday(null)}
        onDeleted={() => holidaysQuery.refetch()}
      />
      <HolidayImportModal
        open={importOpen}
        onClose={() => setImportOpen(false)}
        onImported={() => holidaysQuery.refetch()}
      />
    </div>
  );
}
