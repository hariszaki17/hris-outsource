/**
 * E2 · Perjanjian Kerja — Daftar (HR/Admin list, all agreements)
 *
 * .pen frame: mS8rP  "E2 · Perjanjian Kerja — Daftar"
 *
 * Design: TitleBand → 4× StatCards → TableCard (Tabs, FilterRow, DataTable, Pagination).
 * Columns: NOMOR | KARYAWAN | TIPE | PERIODE | DURASI | STATUS | PENGGANTI | kebab.
 * Tabs: Semua | Aktif | Berakhir <90 hari | Superseded | Closed.
 * Filters: search (nomor/nama agen), Tipe, Status, Lini Layanan.
 *
 * F2.2 EA-1/EA-2/EA-3/EA-5 · ENGINEERING.md D1 cursor pagination.
 */

import { classifyError } from '@/lib/api-error.ts';
import {
  type Agreement,
  AgreementStatus,
  AgreementType,
  type ListAgreements200,
  type ListAgreementsParams,
  useListAgreements,
} from '@swp/api-client/e2';
import type { StatusTone } from '@swp/design-tokens';
import {
  type Column,
  CursorPagination,
  DataTable,
  EmptyState,
  FilterSelect,
  StatCard,
  StateView,
  StatusBadge,
} from '@swp/ui';
import { Link, useNavigate, useSearch } from '@tanstack/react-router';
import { AlarmClock, Archive, FileSignature, MoreVertical, Plus } from 'lucide-react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

export type AgreementsSearch = {
  type?: AgreementType;
  status?: AgreementStatus;
  /** Tab shortcut: 'all' | 'active' | 'expiring' | 'superseded' | 'closed' */
  tab?: 'all' | 'active' | 'expiring' | 'superseded' | 'closed';
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function initials(name: string): string {
  return name
    .split(' ')
    .slice(0, 2)
    .map((p) => p[0] ?? '')
    .join('')
    .toUpperCase();
}

function durationLabel(startDate: string, endDate?: string | null): string {
  if (!endDate) return '∞';
  const start = new Date(startDate);
  const end = new Date(endDate);
  const diffMs = end.getTime() - start.getTime();
  if (diffMs <= 0) return '—';
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
  const months = Math.floor(diffDays / 30);
  const days = diffDays % 30;
  if (months === 0) return `${days} hr`;
  if (days === 0) return `${months} bln`;
  return `${months} bln ${days} hr`;
}

// ---------------------------------------------------------------------------
// Inline tab component (matches Tabs design in mS8rP)
// ---------------------------------------------------------------------------

interface TabItem {
  id: string;
  label: string;
  count?: number;
  active: boolean;
  onClick: () => void;
}

function StatusTabs({ tabs }: { tabs: TabItem[] }) {
  return (
    <div className="flex items-center gap-[26px] px-[18px] pt-[6px] border-b border-border">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          onClick={tab.onClick}
          className={[
            'flex items-center gap-2 pb-[14px] pt-[14px] text-[14px] font-medium',
            tab.active
              ? 'border-b-2 border-primary text-primary font-semibold'
              : 'border-b-2 border-transparent text-text-2',
          ].join(' ')}
        >
          <span>{tab.label}</span>
          {tab.count !== undefined && (
            <span
              className={[
                'rounded-full px-[8px] py-[2px] text-[11px] font-semibold font-mono',
                tab.active ? 'bg-primary-soft text-primary' : 'bg-surface-2 text-text-2',
              ].join(' ')}
            >
              {tab.count}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// AgreementsScreen
// ---------------------------------------------------------------------------

export function AgreementsScreen() {
  const { t } = useTranslation('agreements');
  const navigate = useNavigate();
  const search = useSearch({ from: '/authed/agreements' as const });

  const [prevCursors, setPrevCursors] = useState<string[]>([]);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const kebabRefs = useRef<Map<string, HTMLButtonElement>>(new Map());

  // ---------------------------------------------------------------------------
  // Search params → API params
  // ---------------------------------------------------------------------------

  const activeTab = search.tab ?? 'all';

  const tabStatus: AgreementStatus | undefined =
    activeTab === 'active'
      ? AgreementStatus.ACTIVE
      : activeTab === 'expiring'
        ? AgreementStatus.EXPIRING
        : activeTab === 'superseded'
          ? AgreementStatus.SUPERSEDED
          : activeTab === 'closed'
            ? AgreementStatus.CLOSED
            : undefined;

  const params: ListAgreementsParams = {
    limit: PAGE_SIZE,
    status: tabStatus ?? search.status,
    type: search.type,
    cursor: search.cursor,
  };

  const query = useListAgreements(params);

  const hasFilters = Boolean(search.status || search.type);

  const page = query.data?.data as ListAgreements200 | undefined;
  const rows = (page?.data ?? []) as Agreement[];

  // ---------------------------------------------------------------------------
  // Derived counts for stat cards
  // ---------------------------------------------------------------------------

  const countActive = rows.filter((a) => a.status === AgreementStatus.ACTIVE).length;
  const countExpiring = rows.filter((a) => a.status === AgreementStatus.EXPIRING).length;
  const countArchived = rows.filter(
    (a) => a.status === AgreementStatus.SUPERSEDED || a.status === AgreementStatus.CLOSED,
  ).length;

  // ---------------------------------------------------------------------------
  // Navigation helpers
  // ---------------------------------------------------------------------------

  function setSearch(patch: AgreementsSearch) {
    const next: AgreementsSearch = { ...search, cursor: undefined, ...patch };
    void navigate({ to: '/agreements' as const, search: next });
    setPrevCursors([]);
  }

  // ---------------------------------------------------------------------------
  // Status tone map
  // ---------------------------------------------------------------------------

  const statusTone: Record<AgreementStatus, StatusTone> = {
    ACTIVE: 'ok',
    EXPIRING: 'warn',
    SUPERSEDED: 'neutral',
    CLOSED: 'bad',
  };

  // ---------------------------------------------------------------------------
  // Columns
  // ---------------------------------------------------------------------------

  const columns: Column<Agreement>[] = [
    {
      id: 'nomor',
      header: t('colNomor'),
      width: 160,
      cell: (a) => (
        <Link
          to="/agreements/$agreementId"
          params={{ agreementId: a.id }}
          className="font-mono text-[12px] font-semibold text-primary hover:underline"
        >
          {a.agreement_no ?? a.id}
        </Link>
      ),
    },
    {
      id: 'karyawan',
      header: t('colKaryawan'),
      width: 250,
      cell: (a) => (
        <div className="flex items-center gap-[10px]">
          <div className="size-[30px] rounded-full bg-surface-2 flex items-center justify-center text-[11px] font-semibold text-text-3 shrink-0">
            {initials(a.employee_id ?? '?')}
          </div>
          <span className="font-mono text-[11px] text-text-2">{a.employee_id}</span>
        </div>
      ),
    },
    {
      id: 'tipe',
      header: t('colTipe'),
      width: 90,
      cell: (a) => (
        <span
          className={[
            'inline-flex items-center rounded-[6px] border px-[8px] py-[3px] font-mono text-[10px] font-bold tracking-[0.5px]',
            a.type === AgreementType.PKWT
              ? 'border-info-bd bg-info-bg text-info-tx'
              : 'border-ok-bd bg-ok-bg text-ok-tx',
          ].join(' ')}
        >
          {a.type}
        </span>
      ),
    },
    {
      id: 'periode',
      header: t('colPeriode'),
      width: 220,
      cell: (a) => (
        <span className="font-mono text-[12px] text-text">
          {a.start_date}
          {a.end_date ? ` → ${a.end_date}` : ' → ∞'}
        </span>
      ),
    },
    {
      id: 'durasi',
      header: t('colDurasi'),
      width: 110,
      cell: (a) => (
        <span className="text-[13px] text-text">{durationLabel(a.start_date, a.end_date)}</span>
      ),
    },
    {
      id: 'status',
      header: t('colStatus'),
      width: 130,
      cell: (a) => (
        <StatusBadge dot tone={statusTone[a.status]}>
          {t(`status.${a.status}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'pengganti',
      header: t('colPengganti'),
      width: 100,
      cell: (a) =>
        a.successor_id ? (
          <Link
            to="/agreements/$agreementId"
            params={{ agreementId: String(a.successor_id) }}
            className="font-mono text-[11px] text-primary hover:underline"
          >
            {t('viewSuccessor')}
          </Link>
        ) : (
          <span className="font-mono text-[11px] text-text-3">—</span>
        ),
    },
    {
      id: 'actions',
      header: '',
      width: 52,
      cell: (a) => {
        const isOpen = openMenuId === a.id;
        const setRef = (el: HTMLButtonElement | null) => {
          if (el) kebabRefs.current.set(a.id, el);
          else kebabRefs.current.delete(a.id);
        };
        return (
          <div className="relative flex justify-center">
            <button
              ref={setRef}
              type="button"
              aria-label={t('rowActions')}
              aria-expanded={isOpen}
              aria-haspopup="menu"
              className="flex size-[30px] items-center justify-center rounded-[7px] text-text-3 hover:bg-surface-2"
              onClick={() => setOpenMenuId(isOpen ? null : a.id)}
            >
              <MoreVertical className="size-4" aria-hidden />
            </button>
            {isOpen && (
              <div
                className="absolute right-0 top-full z-10 mt-1 w-[160px] rounded-lg border border-border bg-surface py-1 shadow-md"
                role="menu"
              >
                <button
                  type="button"
                  role="menuitem"
                  className="flex w-full items-center gap-2 px-3 py-[8px] text-[13px] text-text hover:bg-surface-2"
                  onClick={() => {
                    setOpenMenuId(null);
                    void navigate({
                      to: '/agreements/$agreementId' as const,
                      params: { agreementId: a.id },
                    });
                  }}
                >
                  {t('menuView')}
                </button>
              </div>
            )}
          </div>
        );
      },
    },
  ];

  // ---------------------------------------------------------------------------
  // Error state
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    return (
      <div className="flex flex-col gap-[18px]">
        <div className="flex items-start justify-between">
          <h1 className="text-2xl font-bold text-text">{t('title')}</h1>
        </div>
        {kind === 'forbidden' || kind === 'unauthenticated' ? (
          <EmptyState
            variant="no-permission"
            title={t('noPermissionTitle')}
            description={t('noPermissionBody')}
          />
        ) : (
          <StateView
            kind="error"
            title={t('errorTitle')}
            description={t('errorBody')}
            onRetry={() => query.refetch()}
            retryLabel={t('common.retry', { ns: 'translation' })}
          />
        )}
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Tab definitions (mS8rP E3S9XP)
  // ---------------------------------------------------------------------------

  const tabs: TabItem[] = [
    {
      id: 'all',
      label: t('tabAll'),
      count: query.isLoading ? undefined : rows.length,
      active: activeTab === 'all',
      onClick: () => setSearch({ tab: 'all' }),
    },
    {
      id: 'active',
      label: t('tabActive'),
      count: query.isLoading ? undefined : countActive,
      active: activeTab === 'active',
      onClick: () => setSearch({ tab: 'active' }),
    },
    {
      id: 'expiring',
      label: t('tabExpiring'),
      count: query.isLoading ? undefined : countExpiring,
      active: activeTab === 'expiring',
      onClick: () => setSearch({ tab: 'expiring' }),
    },
    {
      id: 'superseded',
      label: t('tabSuperseded'),
      active: activeTab === 'superseded',
      onClick: () => setSearch({ tab: 'superseded' }),
    },
    {
      id: 'closed',
      label: t('tabClosed'),
      active: activeTab === 'closed',
      onClick: () => setSearch({ tab: 'closed' }),
    },
  ];

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band — mS8rP yW76t + DAZxF */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-[20px] font-semibold text-text">{t('title')}</h1>
          <p className="text-[13px] text-text-2">{t('subtitle')}</p>
        </div>
        <div className="flex items-center gap-[10px]">
          <button
            type="button"
            className="flex items-center gap-2 rounded-lg border border-border bg-surface px-[14px] py-[9px] text-[13px] font-medium text-text-2 hover:bg-surface-2"
            onClick={() => void navigate({ to: '/agreements' as const })}
          >
            {t('export')}
          </button>
          <button
            type="button"
            className="flex items-center gap-2 rounded-lg bg-primary px-[14px] py-[10px] text-[14px] font-semibold text-white hover:bg-primary/90"
            onClick={() => void navigate({ to: '/agreements/new' as const })}
          >
            <Plus className="size-4" aria-hidden />
            {t('add')}
          </button>
        </div>
      </div>

      {/* Stat cards — mS8rP fjfuH */}
      <div className="grid grid-cols-4 gap-4">
        <StatCard
          label={t('statTotal')}
          value={query.isLoading ? '—' : String(rows.length)}
          sub={t('statTotalSub')}
          icon={FileSignature}
          tone="brand"
        />
        <StatCard
          label={t('statActive')}
          value={query.isLoading ? '—' : String(countActive)}
          sub={t('statActiveSub')}
          icon={FileSignature}
          tone="ok"
        />
        <StatCard
          label={t('statExpiring')}
          value={query.isLoading ? '—' : String(countExpiring)}
          sub={t('statExpiringSub')}
          icon={AlarmClock}
          tone="warn"
        />
        <StatCard
          label={t('statArchived')}
          value={query.isLoading ? '—' : String(countArchived)}
          sub={t('statArchivedSub')}
          icon={Archive}
          tone="neutral"
        />
      </div>

      {/* Table card — mS8rP D2pjKG */}
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        {/* Tabs — mS8rP E3S9XP */}
        <StatusTabs tabs={tabs} />

        {/* Filter row — mS8rP EEKyM */}
        <div className="flex items-center gap-[10px] border-b border-border-soft px-[18px] py-[14px]">
          <FilterSelect
            aria-label={t('filterType')}
            value={search.type ?? ''}
            onChange={(e) => setSearch({ type: (e.target.value as AgreementType) || undefined })}
          >
            <option value="">{t('filterTypeAll')}</option>
            <option value={AgreementType.PKWT}>PKWT</option>
            <option value={AgreementType.PKWTT}>PKWTT</option>
          </FilterSelect>
          <FilterSelect
            aria-label={t('filterStatus')}
            value={search.status ?? ''}
            onChange={(e) =>
              setSearch({ status: (e.target.value as AgreementStatus) || undefined })
            }
          >
            <option value="">{t('filterStatusAll')}</option>
            <option value={AgreementStatus.ACTIVE}>{t('status.ACTIVE')}</option>
            <option value={AgreementStatus.EXPIRING}>{t('status.EXPIRING')}</option>
            <option value={AgreementStatus.SUPERSEDED}>{t('status.SUPERSEDED')}</option>
            <option value={AgreementStatus.CLOSED}>{t('status.CLOSED')}</option>
          </FilterSelect>
          <div className="flex-1" />
          <button
            type="button"
            className="flex items-center gap-2 rounded-lg border border-border bg-surface px-[14px] py-[9px] text-[13px] font-medium text-text-2 hover:bg-surface-2"
            onClick={() => setSearch({ type: undefined, status: undefined })}
          >
            {t('resetFilters')}
          </button>
        </div>

        {/* Data table */}
        <DataTable
          aria-label={t('title')}
          columns={columns}
          data={rows}
          getRowId={(a) => a.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          onRowClick={(a) =>
            void navigate({
              to: '/agreements/$agreementId' as const,
              params: { agreementId: a.id },
            })
          }
          empty={
            hasFilters ? (
              <EmptyState
                variant="filtered"
                title={t('filteredTitle')}
                description={t('filteredBody')}
              />
            ) : (
              <EmptyState variant="fresh" title={t('emptyTitle')} description={t('emptyBody')} />
            )
          }
          footer={
            rows.length > 0 ? (
              <CursorPagination
                rangeLabel={t('resultRange', { count: rows.length })}
                hasPrev={prevCursors.length > 0}
                hasNext={Boolean(page?.has_more)}
                prevLabel={t('common.prev', { ns: 'translation' })}
                nextLabel={t('common.next', { ns: 'translation' })}
                onPrev={() => {
                  const next = [...prevCursors];
                  const cursor = next.pop();
                  setPrevCursors(next);
                  void navigate({
                    to: '/agreements' as const,
                    search: { ...search, cursor: cursor || undefined },
                  });
                }}
                onNext={() => {
                  const nextCursor = page?.next_cursor;
                  if (!nextCursor) return;
                  setPrevCursors((s) => [...s, search.cursor ?? '']);
                  void navigate({
                    to: '/agreements' as const,
                    search: { ...search, cursor: nextCursor },
                  });
                }}
              />
            ) : undefined
          }
        />
      </div>
    </div>
  );
}
