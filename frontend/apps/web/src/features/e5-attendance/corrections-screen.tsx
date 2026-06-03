/**
 * E5 · Koreksi Kehadiran — HR corrections approval queue (F5.4).
 *
 * .pen frames implemented:
 *   QfamL  HR · Koreksi · Antrian  (this file — CorrectionsScreen)
 *   sSKtK  HR · Koreksi · Detail   (CorrectionDetailDrawer in correction-overlays.tsx)
 *
 * Route:   /corrections  (add to router.tsx — see RETURN section in task)
 * Hooks:   useListCorrections, useGetCorrection, useApproveCorrection, useRejectCorrection
 *          (all from @swp/api-client/e5)
 * Data:    query.data?.data = { data: Correction[], next_cursor?, has_more }
 * RBAC:    Client-side gating is defense-in-depth; API is the gate.
 */

import { classifyError } from '@/lib/api-error.ts';
import {
  type Correction,
  type CorrectionPage,
  CorrectionStatus,
  CorrectionType,
  type ListCorrectionsParams,
  useListCorrections,
} from '@swp/api-client/e5';
import {
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
} from '@swp/ui';
import { RotateCcw } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CorrectionDetailDrawer } from './correction-overlays.tsx';
import { correctionStatusTone, correctionTypeLabel } from './correction-overlays.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** Typed URL search params for /corrections (used in router.tsx validateSearch). */
export type CorrectionsSearch = {
  q?: string;
  status?: CorrectionStatus;
  type?: CorrectionType;
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function hasActiveFilters(s: CorrectionsSearch): boolean {
  return Boolean(s.q || s.status || s.type);
}

// ---------------------------------------------------------------------------
// Main screen — consumes typed search props passed from the route component
// ---------------------------------------------------------------------------

interface CorrectionsScreenProps {
  search: CorrectionsSearch;
  onSearch: (patch: CorrectionsSearch) => void;
  prevCursors: string[];
  onPrevCursorsChange: (c: string[]) => void;
}

function CorrectionsScreenInner({
  search,
  onSearch,
  prevCursors,
  onPrevCursorsChange,
}: CorrectionsScreenProps) {
  const { t } = useTranslation();

  const [drawerCorrectionId, setDrawerCorrectionId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  // ---------------------------------------------------------------------------
  // Query
  // ---------------------------------------------------------------------------

  const params: ListCorrectionsParams = {
    limit: PAGE_SIZE,
    cursor: search.cursor,
    status: search.status ? [search.status] : undefined,
    type: search.type ? [search.type] : undefined,
  };

  const query = useListCorrections(params);

  const hasFilters = hasActiveFilters(search);

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  function setSearch(patch: CorrectionsSearch) {
    // Any filter change resets pagination
    onSearch({ ...search, cursor: undefined, ...patch });
    onPrevCursorsChange([]);
  }

  function openDrawer(id: string) {
    setDrawerCorrectionId(id);
    setDrawerOpen(true);
  }

  function handleDone() {
    setDrawerOpen(false);
    void query.refetch();
  }

  // ---------------------------------------------------------------------------
  // Error state
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    return (
      <div className="flex flex-col gap-[18px]">
        <TitleBand />
        {kind === 'forbidden' || kind === 'unauthenticated' ? (
          <EmptyState
            variant="no-permission"
            title={t('errors.forbidden')}
            description={t('corrections.noPermissionBody')}
          />
        ) : (
          <StateView
            kind="error"
            title={t('corrections.errorTitle')}
            description={t('errors.network')}
            onRetry={() => query.refetch()}
            retryLabel={t('common.retry')}
          />
        )}
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Data
  // ---------------------------------------------------------------------------

  // Orval wraps response: query.data = { data: CorrectionPage, status, headers }
  const page = query.data?.data as CorrectionPage | undefined;
  const rows = page?.data ?? [];

  // ---------------------------------------------------------------------------
  // Columns
  // ---------------------------------------------------------------------------

  const columns: Column<Correction>[] = [
    {
      id: 'requester',
      header: t('corrections.colRequester'),
      width: 200,
      cell: (c) => (
        <div className="flex flex-col">
          <span className="font-medium text-text">{c.requester_id}</span>
          <span className="font-mono text-xs text-text-3">{c.attendance_id}</span>
        </div>
      ),
    },
    {
      id: 'type',
      header: t('corrections.colType'),
      width: 140,
      cell: (c) => <span className="text-sm text-text-2">{correctionTypeLabel(c.type, t)}</span>,
    },
    {
      id: 'summary',
      header: t('corrections.colSummary'),
      width: 280,
      cell: (c) => (
        <p className="max-w-[260px] truncate text-sm text-text-2" title={c.reason}>
          {c.reason}
        </p>
      ),
    },
    {
      id: 'submitted',
      header: t('corrections.colSubmitted'),
      width: 170,
      cell: (c) => <DateText kind="instant" value={c.created_at} className="text-sm text-text-2" />,
    },
    {
      id: 'status',
      header: t('corrections.colStatus'),
      width: 130,
      cell: (c) => (
        <StatusBadge dot tone={correctionStatusTone(c.status)}>
          {t(`corrections.status.${c.status}`)}
        </StatusBadge>
      ),
    },
  ];

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      <TitleBand />

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-2.5">
        <SearchField
          placeholder={t('corrections.searchPlaceholder')}
          defaultValue={search.q ?? ''}
          containerClassName="w-64"
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
        />
        <FilterSelect
          aria-label={t('corrections.filterStatus')}
          value={search.status ?? ''}
          onChange={(e) => setSearch({ status: (e.target.value as CorrectionStatus) || undefined })}
        >
          <option value="">{t('corrections.filterStatus')}</option>
          {Object.values(CorrectionStatus).map((s) => (
            <option key={s} value={s}>
              {t(`corrections.status.${s}`)}
            </option>
          ))}
        </FilterSelect>
        <FilterSelect
          aria-label={t('corrections.filterType')}
          value={search.type ?? ''}
          onChange={(e) => setSearch({ type: (e.target.value as CorrectionType) || undefined })}
        >
          <option value="">{t('corrections.filterType')}</option>
          {Object.values(CorrectionType).map((ty) => (
            <option key={ty} value={ty}>
              {correctionTypeLabel(ty, t)}
            </option>
          ))}
        </FilterSelect>

        {hasFilters && (
          <>
            <div className="h-6 w-px bg-border" />
            <Button
              type="button"
              variant="ghost"
              onClick={() =>
                setSearch({ q: undefined, status: undefined, type: undefined, cursor: undefined })
              }
            >
              <RotateCcw aria-hidden className="size-3.5" />
              {t('corrections.resetFilters')}
            </Button>
          </>
        )}
      </div>

      {/* Table */}
      <DataTable
        aria-label={t('corrections.tableAriaLabel')}
        columns={columns}
        data={rows}
        getRowId={(c) => c.id}
        isLoading={query.isLoading}
        skeletonRows={8}
        onRowClick={(c) => openDrawer(c.id)}
        empty={
          hasFilters ? (
            <EmptyState
              variant="filtered"
              title={t('corrections.filteredTitle')}
              description={t('corrections.filteredBody')}
            />
          ) : (
            <EmptyState
              variant="fresh"
              title={t('corrections.emptyTitle')}
              description={t('corrections.emptyBody')}
            />
          )
        }
        footer={
          rows.length > 0 ? (
            <CursorPagination
              rangeLabel={t('corrections.resultRange', { count: rows.length })}
              hasPrev={prevCursors.length > 0}
              hasNext={Boolean(page?.has_more)}
              prevLabel={t('common.prev')}
              nextLabel={t('common.next')}
              onPrev={() => {
                const next = [...prevCursors];
                const cursor = next.pop();
                onPrevCursorsChange(next);
                onSearch({ ...search, cursor: cursor || undefined });
              }}
              onNext={() => {
                const nextCursor = page?.next_cursor;
                if (!nextCursor) return;
                onPrevCursorsChange([...prevCursors, search.cursor ?? '']);
                onSearch({ ...search, cursor: nextCursor });
              }}
            />
          ) : undefined
        }
      />

      {/* Detail drawer + reject modal */}
      <CorrectionDetailDrawer
        open={drawerOpen}
        correctionId={drawerCorrectionId}
        onOpenChange={(open) => {
          if (!open) setDrawerOpen(false);
        }}
        onDone={handleDone}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public export — manages search state internally; router wires search via
// useSearch + navigate and passes the props down (see router.tsx snippet below).
// Exported as a self-contained screen so it can also be used standalone.
// ---------------------------------------------------------------------------

/**
 * Exported screen component. When added to the router the route component should
 * call useSearch / useNavigate and pass `search` + `onSearch` as props.
 * When used without the router (e.g. Storybook / tests) it falls back to local state.
 */
export function CorrectionsScreen() {
  const [search, setSearch] = useState<CorrectionsSearch>({});
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  return (
    <CorrectionsScreenInner
      search={search}
      onSearch={(patch) => setSearch(patch)}
      prevCursors={prevCursors}
      onPrevCursorsChange={setPrevCursors}
    />
  );
}

// ---------------------------------------------------------------------------
// TitleBand
// ---------------------------------------------------------------------------

function TitleBand() {
  const { t } = useTranslation();
  return (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="font-bold text-3xl text-text">{t('corrections.title')}</h1>
        <p className="max-w-[640px] text-sm text-text-3">{t('corrections.subtitle')}</p>
      </div>
    </div>
  );
}
