/**
 * E2 · Antrian Persetujuan Perubahan Data — HR approval queue for agent-submitted
 * profile-change requests (EP-5).
 *
 * Built from `.pen` frame `Ckteo` (E2 · Antrian Persetujuan — HR).
 *
 * Features:
 *  - Stat cards: pending / approved-7d / rejected-7d / stale (>3 days)
 *  - Tab filter: Semua / Profil (Telepon|Alamat) / Bank
 *  - Search + request_type filter
 *  - DataTable with cursor pagination
 *  - Detail drawer (ChangeRequestDetailDrawer) on row click
 *  - Inline approve from row action button
 *
 * Hooks: useListPendingChangeRequests · useApproveChangeRequest (inline row)
 * Full review surface lives in ChangeRequestDetailDrawer (change-request-overlays.tsx).
 *
 * F2.x · EP-5 · INV-1
 */

import { classifyError } from '@/lib/api-error.ts';
import {
  type ChangeRequest,
  ChangeRequestRequestType,
  ChangeRequestStatus,
  type ListPendingChangeRequests200,
  type ListPendingChangeRequestsParams,
  useApproveChangeRequest,
  useListPendingChangeRequests,
} from '@swp/api-client/e2';
import {
  Avatar,
  Button,
  type Column,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  FilterSelect,
  SearchField,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { AlarmClock, Check, CheckCheck, Clock } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChangeRequestDetailDrawer, requestTypeLabel } from './change-request-overlays.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** Typed URL search params for `/change-requests`. */
export type ChangeRequestsSearch = {
  q?: string;
  request_type?: ChangeRequestRequestType;
  tab?: 'all' | 'profile' | 'bank';
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

function daysSince(isoDate: string): number {
  const diff = Date.now() - new Date(isoDate).getTime();
  return Math.floor(diff / (1000 * 60 * 60 * 24));
}

/**
 * Avatar initials for an employee id. The list projection (ChangeRequest) carries
 * only `employee_id` (no name — the joined name lives on the detail response), so
 * derive a deterministic 2-char token from the id's trailing digits rather than
 * faking name-initials from an opaque id.
 */
function idInitials(employeeId: string): string {
  const tail = employeeId.replace(/[^a-zA-Z0-9]/g, '').slice(-2);
  return (tail || '?').toUpperCase();
}

// ---------------------------------------------------------------------------
// StatCard — local display component (mirrors comp/StatCard in .pen)
// ---------------------------------------------------------------------------

interface StatCardProps {
  icon: React.ReactNode;
  iconBgClass: string;
  label: string;
  value: string | number;
  sub: string;
}

function StatCard({ icon, iconBgClass, label, value, sub }: StatCardProps) {
  return (
    <div className="flex flex-1 items-center gap-3 rounded-xl border border-border bg-surface px-4 py-3.5">
      <div className={`flex size-9 shrink-0 items-center justify-center rounded-lg ${iconBgClass}`}>
        {icon}
      </div>
      <div className="flex flex-col gap-0.5">
        <span className="text-[13px] text-text-2">{label}</span>
        <span className="text-xl font-bold text-text">{value}</span>
        <span className="text-[11px] text-text-3">{sub}</span>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// TabBar — Semua / Profil / Bank  (mirrors Tabs in .pen Ckteo)
// ---------------------------------------------------------------------------

interface TabBarProps {
  active: ChangeRequestsSearch['tab'];
  onChange: (tab: ChangeRequestsSearch['tab']) => void;
}

function TabBar({ active, onChange }: TabBarProps) {
  const { t } = useTranslation();

  const tabs: Array<{ id: ChangeRequestsSearch['tab']; label: string }> = [
    { id: 'all', label: t('changeRequests.tabAll') },
    { id: 'profile', label: t('changeRequests.tabProfile') },
    { id: 'bank', label: t('changeRequests.tabBank') },
  ];

  return (
    <div className="flex gap-6 border-b border-border px-[18px]">
      {tabs.map((tab) => {
        const isActive = (active ?? 'all') === tab.id;
        return (
          <button
            key={tab.id}
            type="button"
            onClick={() => onChange(tab.id)}
            className={[
              'flex items-center gap-2 py-3 text-sm transition-colors',
              isActive
                ? 'border-b-2 border-primary font-semibold text-primary'
                : 'text-text-2 hover:text-text',
            ].join(' ')}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// TitleBand — top section matching .pen TitleBand
// ---------------------------------------------------------------------------

function TitleBand() {
  const { t } = useTranslation();
  return (
    <div className="flex items-start justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
      <div className="flex flex-col gap-1">
        <h1 className="text-xl font-semibold text-text">{t('changeRequests.pageTitle')}</h1>
        <p className="text-[13px] text-text-2">{t('changeRequests.pageSubtitle')}</p>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// ChangeRequestsScreen
// ---------------------------------------------------------------------------

export function ChangeRequestsScreen() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const navigate = useNavigate();
  const search = useSearch({ from: '/authed/change-requests' });

  const [prevCursors, setPrevCursors] = useState<string[]>([]);
  const [detailId, setDetailId] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  const approveMutation = useApproveChangeRequest();

  const activeTab = search.tab ?? 'all';

  // Derive request_type from tab if URL param not set
  const resolvedType: ChangeRequestRequestType | undefined =
    search.request_type ??
    (activeTab === 'profile'
      ? ChangeRequestRequestType.PHONE
      : activeTab === 'bank'
        ? ChangeRequestRequestType.BANK_ACCOUNT
        : undefined);

  const params: ListPendingChangeRequestsParams = {
    limit: PAGE_SIZE,
    q: search.q || undefined,
    request_type: resolvedType,
    cursor: search.cursor,
    status: ChangeRequestStatus.PENDING,
  };

  const query = useListPendingChangeRequests(params);

  const hasFilters = Boolean(search.q || search.request_type || activeTab !== 'all');

  function setSearch(patch: ChangeRequestsSearch) {
    void navigate({
      to: '/change-requests',
      search: { ...search, cursor: undefined, ...patch },
    });
    setPrevCursors([]);
  }

  // ---------------------------------------------------------------------------
  // Inline approve handler (quick-approve without opening drawer)
  // ---------------------------------------------------------------------------

  function handleInlineApprove(cr: ChangeRequest, e: React.MouseEvent) {
    e.stopPropagation();
    approveMutation.mutate(
      { changeRequestId: cr.id },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('changeRequests.approveSuccess') });
          void query.refetch();
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t(message as never) ?? message });
        },
      },
    );
  }

  // ---------------------------------------------------------------------------
  // Table columns — mirrors .pen THead: AGEN · TIPE · FIELD · DIUBAH · DIAJUKAN · AKSI
  // ---------------------------------------------------------------------------

  const columns: Column<ChangeRequest>[] = [
    {
      id: 'employee',
      header: t('changeRequests.colAgent'),
      width: 240,
      cell: (cr) => (
        <div className="flex items-center gap-2.5">
          <Avatar initials={idInitials(cr.employee_id)} size={32} />
          <div className="flex flex-col">
            <span className="font-mono text-[12px] font-medium text-text">{cr.employee_id}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'request_type',
      header: t('changeRequests.colType'),
      width: 140,
      cell: (cr) => (
        <StatusBadge dot tone="info">
          {requestTypeLabel(cr.request_type)}
        </StatusBadge>
      ),
    },
    {
      id: 'fields',
      header: t('changeRequests.colFields'),
      width: 170,
      cell: (cr) => {
        const keys = Object.keys(cr.changes ?? {});
        const labels = keys.map((fk) =>
          fk === 'bank_account' ? 'Rekening' : fk === 'phone' ? 'Telepon' : 'Alamat',
        );
        return <span className="text-sm text-text-2">{labels.join(', ') || '—'}</span>;
      },
    },
    {
      id: 'changes',
      header: t('changeRequests.colChanges'),
      width: 260,
      cell: (cr) => {
        const changes = cr.changes ?? {};
        const parts: string[] = [];
        if (changes.phone) parts.push(changes.phone);
        if (changes.address) parts.push(changes.address);
        if (changes.bank_account) {
          const ba = changes.bank_account;
          const baStr = [ba.bank_name, ba.account_number].filter(Boolean).join(' · ');
          if (baStr) parts.push(baStr);
        }
        return <span className="line-clamp-2 text-sm text-text-2">{parts.join(' | ') || '—'}</span>;
      },
    },
    {
      id: 'submitted_at',
      header: t('changeRequests.colSubmitted'),
      width: 140,
      cell: (cr) => {
        const days = daysSince(cr.submitted_at);
        return (
          <div className="flex flex-col gap-0.5">
            <DateText kind="date" value={cr.submitted_at} className="text-sm text-text-2" />
            {days > 3 && (
              <span className="flex items-center gap-1 text-[11px] text-warn-tx">
                <AlarmClock className="size-3 shrink-0" aria-hidden />
                {t('changeRequests.staleLabel', { days })}
              </span>
            )}
          </div>
        );
      },
    },
    {
      id: 'actions',
      header: t('changeRequests.colActions'),
      width: 180,
      cell: (cr) => (
        <div className="flex items-center gap-1.5">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={(e) => {
              e.stopPropagation();
              setDetailId(cr.id);
              setDetailOpen(true);
            }}
          >
            {t('changeRequests.actionReview')}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            aria-label={t('changeRequests.approveAction')}
            disabled={approveMutation.isPending}
            onClick={(e) => handleInlineApprove(cr, e)}
          >
            <Check className="size-4" aria-hidden />
          </Button>
        </div>
      ),
    },
  ];

  // ---------------------------------------------------------------------------
  // Data
  // ---------------------------------------------------------------------------

  const page = query.data?.data as ListPendingChangeRequests200 | undefined;
  const rows = (page?.data ?? []) as ChangeRequest[];

  const pendingCount = rows.length;
  const staleCount = rows.filter((r) => daysSince(r.submitted_at) > 3).length;

  // ---------------------------------------------------------------------------
  // Error / permission states (full-page takeover, title band preserved)
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    return (
      <div className="flex flex-col gap-[18px]">
        <TitleBand />
        {kind === 'forbidden' || kind === 'unauthenticated' ? (
          <EmptyState
            variant="no-permission"
            title={t('changeRequests.noPermissionTitle')}
            description={t('changeRequests.noPermissionBody')}
          />
        ) : (
          <StateView
            kind="error"
            title={t('changeRequests.errorTitle')}
            description={t('changeRequests.errorBody')}
            onRetry={() => query.refetch()}
            retryLabel={t('changeRequests.retry')}
          />
        )}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[18px]">
      <TitleBand />

      {/* Stat cards — mirrors .pen Stats row (W9AbJp / gIpeE / HapCk / H7YTL) */}
      <div className="flex flex-wrap gap-4">
        <StatCard
          icon={<Clock className="size-4 text-warn-tx" aria-hidden />}
          iconBgClass="bg-warn-bg"
          label={t('changeRequests.statPending')}
          value={pendingCount}
          sub={t('changeRequests.statPendingSub')}
        />
        <StatCard
          icon={<AlarmClock className="size-4 text-text-3" aria-hidden />}
          iconBgClass="bg-surface-2"
          label={t('changeRequests.statStale')}
          value={staleCount}
          sub={t('changeRequests.statStaleSub')}
        />
      </div>

      {/* Table card — mirrors .pen TableCard (Mf1FA) */}
      <div className="overflow-hidden rounded-xl border border-border bg-surface">
        {/* Inner title band */}
        <div className="flex items-center justify-between px-5 py-[18px]">
          <div className="flex flex-col gap-1">
            <h2 className="text-base font-semibold text-text">{t('changeRequests.tableTitle')}</h2>
            <p className="text-[13px] text-text-2">{t('changeRequests.tableSubtitle')}</p>
          </div>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => {
              toast({ tone: 'info', title: t('changeRequests.bulkApproveToast') });
            }}
          >
            <CheckCheck aria-hidden />
            {t('changeRequests.bulkApprove')}
          </Button>
        </div>

        {/* Tab bar — mirrors .pen Tabs (RyErE) */}
        <TabBar
          active={activeTab}
          onChange={(tab) => setSearch({ tab, request_type: undefined })}
        />

        {/* Filter row — mirrors .pen FilterRow (FT48h) */}
        <div className="flex flex-wrap items-center justify-between gap-2.5 border-b border-border-soft px-[18px] py-3.5">
          <div className="flex flex-wrap items-center gap-2.5">
            <SearchField
              placeholder={t('changeRequests.searchPlaceholder')}
              defaultValue={search.q ?? ''}
              containerClassName="w-64"
              onChange={(e) => setSearch({ q: e.target.value || undefined })}
            />
          </div>
          <div className="flex items-center gap-2">
            <FilterSelect
              aria-label={t('changeRequests.filterTypeLabel')}
              value={activeTab === 'all' ? (search.request_type ?? '') : ''}
              disabled={activeTab !== 'all'}
              title={activeTab !== 'all' ? t('changeRequests.filterTypeTabLocked') : undefined}
              onChange={(e) =>
                setSearch({
                  tab: 'all',
                  request_type: (e.target.value as ChangeRequestRequestType) || undefined,
                })
              }
            >
              <option value="">{t('changeRequests.filterTypeAll')}</option>
              {Object.values(ChangeRequestRequestType).map((rt) => (
                <option key={rt} value={rt}>
                  {requestTypeLabel(rt)}
                </option>
              ))}
            </FilterSelect>
          </div>
        </div>

        {/* Data table */}
        <DataTable
          aria-label={t('changeRequests.tableTitle')}
          columns={columns}
          data={rows}
          getRowId={(cr) => cr.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          onRowClick={(cr) => {
            setDetailId(cr.id);
            setDetailOpen(true);
          }}
          empty={
            hasFilters ? (
              <EmptyState
                variant="filtered"
                title={t('changeRequests.filteredTitle')}
                description={t('changeRequests.filteredBody')}
              />
            ) : (
              <EmptyState
                variant="fresh"
                title={t('changeRequests.emptyTitle')}
                description={t('changeRequests.emptyBody')}
              />
            )
          }
          footer={
            rows.length > 0 ? (
              <CursorPagination
                rangeLabel={t('changeRequests.resultRange', { count: rows.length })}
                hasPrev={prevCursors.length > 0}
                hasNext={Boolean(page?.has_more)}
                prevLabel={t('changeRequests.prev')}
                nextLabel={t('changeRequests.next')}
                onPrev={() => {
                  const next = [...prevCursors];
                  const cursor = next.pop();
                  setPrevCursors(next);
                  void navigate({
                    to: '/change-requests',
                    search: { ...search, cursor: cursor || undefined },
                  });
                }}
                onNext={() => {
                  const nextCursor = page?.next_cursor;
                  if (!nextCursor) return;
                  setPrevCursors((s) => [...s, search.cursor ?? '']);
                  void navigate({
                    to: '/change-requests',
                    search: { ...search, cursor: nextCursor },
                  });
                }}
              />
            ) : undefined
          }
        />
      </div>

      {/* Detail drawer + reject modal — driven by detailId / detailOpen */}
      <ChangeRequestDetailDrawer
        open={detailOpen}
        onOpenChange={setDetailOpen}
        changeRequestId={detailId}
        onDone={() => {
          void query.refetch();
        }}
      />
    </div>
  );
}
