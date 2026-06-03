import { classifyError } from '@/lib/api-error.ts';
import {
  type AuditLogEntrySummary,
  type ListAuditLog200,
  type ListAuditLogParams,
  useListAuditLog,
} from '@swp/api-client/e1';
import type { StatusTone } from '@swp/design-tokens';
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
import { Cpu, Download } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AuditDetailDrawer } from './audit-detail-drawer.tsx';

/**
 * E1 · Audit Log — append-only, immutable audit trail (F1.3 / audit-log.md).
 * Built from `.pen` frame `N3EBSr`. Data via generated `useListAuditLog` hook over MSW.
 * Filters live in typed URL search params (shareable + stable cache key).
 * Cursor pagination only (ENGINEERING.md D1). Client gating is defense-in-depth.
 *
 * Export (Ekspor) is a no-op info toast — real export is deferred to E10.
 * Date filter is a preset-select placeholder — real DateRangePicker deferred
 * (no DateRangePicker in packages/ui yet).
 */
const PAGE_SIZE = 50;

/** Typed filter/cursor search params for `/settings/audit-log` (mirrors the route's validateSearch). */
type AuditLogSearch = {
  q?: string;
  entity_type?: string;
  action?: string;
  date_preset?: string;
  cursor?: string;
};

/** Map action verb family to a StatusTone for the AKSI badge. */
function actionTone(action: string): StatusTone {
  const v = action.toLowerCase();
  if (/approve|verify|create/.test(v)) return 'ok';
  if (/reject|deactivate|delete/.test(v)) return 'bad';
  if (/auto|system|migrate/.test(v)) return 'neutral';
  if (/change|transfer|update/.test(v)) return 'info';
  return 'neutral';
}

/** Bahasa label for entity_type snake-case values. */
function entityLabel(entityType: string): string {
  switch (entityType) {
    case 'user':
      return 'Pengguna';
    case 'leave_request':
      return 'Cuti';
    case 'attendance':
      return 'Kehadiran';
    case 'placement':
      return 'Penempatan';
    case 'payslip':
      return 'Payslip';
    default:
      return entityType;
  }
}

/** Derive single-char initials from a display label. */
function actorInitials(label: string): string {
  return (label.trim()[0] ?? '?').toUpperCase();
}

export function AuditLogScreen() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const navigate = useNavigate();
  const search = useSearch({ from: '/authed/settings/audit-log' });
  // Cursor history for the Prev button (cursors are forward-only).
  const [prevCursors, setPrevCursors] = useState<string[]>([]);
  // Track which entry is open in the detail drawer.
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const params: ListAuditLogParams = {
    limit: PAGE_SIZE,
    q: search.q || undefined,
    entity_type: search.entity_type || undefined,
    action: search.action || undefined,
    cursor: search.cursor,
  };
  const query = useListAuditLog(params);

  const hasFilters = Boolean(search.q || search.entity_type || search.action || search.date_preset);

  const setSearch = (patch: AuditLogSearch) => {
    void navigate({
      to: '/settings/audit-log',
      // Any filter change resets pagination.
      search: (prev) => ({ ...prev, cursor: undefined, ...patch }),
    });
    setPrevCursors([]);
  };

  function handleExport() {
    // Real export modal is deferred to E10. Push an info toast for now.
    toast({
      tone: 'info',
      title: t('auditLog.exportToastTitle'),
      description: t('auditLog.exportToastBody'),
    });
  }

  function openDrawer(id: string) {
    setSelectedId(id);
    setDrawerOpen(true);
  }

  const columns: Column<AuditLogEntrySummary>[] = [
    {
      id: 'waktu',
      header: t('auditLog.colWaktu'),
      width: 150,
      cell: (row) => (
        <DateText kind="instant" value={row.created_at} className="font-mono text-xs text-text-2" />
      ),
    },
    {
      id: 'aktor',
      header: t('auditLog.colAktor'),
      width: 230,
      cell: (row) => {
        // System actor: actor_user_id is null / absent.
        if (!row.actor_user_id) {
          return (
            <div className="flex items-center gap-2">
              <span className="inline-flex size-[28px] shrink-0 items-center justify-center rounded-md bg-surface-2 text-text-3">
                <Cpu className="size-3.5" aria-hidden="true" />
              </span>
              <span className="text-xs text-text-2">
                {row.actor_label ?? t('auditLog.systemActor')}
              </span>
            </div>
          );
        }
        return (
          <div className="flex items-center gap-2">
            <Avatar
              initials={actorInitials(row.actor_label ?? row.actor_user_id)}
              size={28}
              tone="neutral"
            />
            <span className="text-xs text-text">{row.actor_label ?? row.actor_user_id}</span>
          </div>
        );
      },
    },
    {
      id: 'aksi',
      header: t('auditLog.colAksi'),
      width: 160,
      cell: (row) => (
        <StatusBadge dot tone={actionTone(row.action)}>
          <span className="font-mono">{row.action}</span>
        </StatusBadge>
      ),
    },
    {
      id: 'entitas',
      header: t('auditLog.colEntitas'),
      width: 230,
      cell: (row) => (
        <span className="font-mono text-xs text-text-2">
          {entityLabel(row.entity_type)} #{row.entity_id}
        </span>
      ),
    },
    {
      id: 'perubahan',
      header: t('auditLog.colPerubahan'),
      // fill — no fixed width
      cell: (row) => (
        <span
          className="block max-w-[320px] truncate font-mono text-xs text-text-2"
          title={row.change_summary}
        >
          {row.change_summary}
        </span>
      ),
    },
  ];

  // Title band — shared across error + normal paths.
  const titleBand = (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="font-bold text-3xl text-text">{t('auditLog.title')}</h1>
        <p className="max-w-[640px] text-sm text-text-3">{t('auditLog.subtitle')}</p>
      </div>
      <Button type="button" variant="secondary" onClick={handleExport}>
        <Download aria-hidden="true" />
        {t('auditLog.export')}
      </Button>
    </div>
  );

  // Error states take over the whole content (no table chrome).
  if (query.isError) {
    const { kind } = classifyError(query.error);
    return (
      <div className="flex flex-col gap-[18px]">
        {titleBand}
        {kind === 'forbidden' || kind === 'unauthenticated' ? (
          <EmptyState
            variant="no-permission"
            title={t('errors.forbidden')}
            description={t('auditLog.noPermissionBody')}
          />
        ) : (
          <StateView
            kind="error"
            title={t('auditLog.errorTitle')}
            description={t('errors.network')}
            onRetry={() => query.refetch()}
            retryLabel={t('common.retry')}
          />
        )}
      </div>
    );
  }

  // Orval fetch-client wraps the response: `query.data` is `{ data, status, headers }` and the
  // body lives under `.data`. Non-2xx threw above, so on success the body is `ListAuditLog200`.
  const page = query.data?.data as ListAuditLog200 | undefined;
  const rows = (page?.data ?? []) as AuditLogEntrySummary[];

  return (
    <div className="flex flex-col gap-[18px]">
      {titleBand}

      {/* Filters row */}
      <div className="flex flex-wrap items-center gap-2.5">
        <SearchField
          placeholder={t('auditLog.searchPlaceholder')}
          defaultValue={search.q ?? ''}
          containerClassName="w-64"
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
        />
        <FilterSelect
          aria-label={t('auditLog.filterEntityLabel')}
          value={search.entity_type ?? ''}
          onChange={(e) => setSearch({ entity_type: e.target.value || undefined })}
        >
          <option value="">{t('auditLog.filterEntityAll')}</option>
          <option value="user">{t('auditLog.entityUser')}</option>
          <option value="leave_request">{t('auditLog.entityLeaveRequest')}</option>
          <option value="attendance">{t('auditLog.entityAttendance')}</option>
          <option value="placement">{t('auditLog.entityPlacement')}</option>
          <option value="payslip">{t('auditLog.entityPayslip')}</option>
        </FilterSelect>
        <FilterSelect
          aria-label={t('auditLog.filterActionLabel')}
          value={search.action ?? ''}
          onChange={(e) => setSearch({ action: e.target.value || undefined })}
        >
          <option value="">{t('auditLog.filterActionAll')}</option>
          <option value="user.create">{t('auditLog.actionUserCreate')}</option>
          <option value="user.role.change">{t('auditLog.actionRoleChange')}</option>
          <option value="user.deactivate">{t('auditLog.actionUserDeactivate')}</option>
          <option value="placement.transfer">{t('auditLog.actionPlacementTransfer')}</option>
          <option value="attendance.verify">{t('auditLog.actionAttendanceVerify')}</option>
          <option value="leave_request.approve">{t('auditLog.actionLeaveApprove')}</option>
          <option value="leave_request.reject">{t('auditLog.actionLeaveReject')}</option>
        </FilterSelect>
        {/*
          Date filter — placeholder select with presets. A real DateRangePicker component
          (which would set created_at__gte / created_at__lte on the API params) is deferred
          until a DateRangePicker primitive lands in packages/ui.
        */}
        <FilterSelect
          aria-label={t('auditLog.filterDateLabel')}
          value={search.date_preset ?? ''}
          onChange={(e) => setSearch({ date_preset: e.target.value || undefined })}
        >
          <option value="">{t('auditLog.filterDateAll')}</option>
          <option value="today">{t('auditLog.filterDateToday')}</option>
          <option value="last7">{t('auditLog.filterDateLast7')}</option>
          <option value="last30">{t('auditLog.filterDateLast30')}</option>
        </FilterSelect>
      </div>

      {/* Data table — rows are clickable (no row actions; append-only log) */}
      <DataTable
        aria-label={t('auditLog.title')}
        columns={columns}
        data={rows}
        getRowId={(row) => row.id}
        isLoading={query.isLoading}
        skeletonRows={8}
        onRowClick={(row) => openDrawer(row.id)}
        empty={
          hasFilters ? (
            <EmptyState
              variant="filtered"
              title={t('auditLog.filteredTitle')}
              description={t('auditLog.filteredBody')}
            />
          ) : (
            <EmptyState
              variant="fresh"
              title={t('auditLog.emptyTitle')}
              description={t('auditLog.emptyBody')}
            />
          )
        }
        footer={
          rows.length > 0 ? (
            <div className="flex items-center justify-between px-4 py-2.5">
              <span className="text-xs text-text-3">
                {t('auditLog.rowCount', { count: rows.length })}
              </span>
              <CursorPagination
                rangeLabel={t('auditLog.rowCount', { count: rows.length })}
                hasPrev={prevCursors.length > 0}
                hasNext={Boolean(page?.has_more)}
                prevLabel={t('common.prev')}
                nextLabel={t('common.next')}
                onPrev={() => {
                  const next = [...prevCursors];
                  const cursor = next.pop();
                  setPrevCursors(next);
                  void navigate({
                    to: '/settings/audit-log',
                    search: (prev) => ({ ...prev, cursor: cursor || undefined }),
                  });
                }}
                onNext={() => {
                  const nextCursor = page?.next_cursor;
                  if (!nextCursor) return;
                  setPrevCursors((s) => [...s, search.cursor ?? '']);
                  void navigate({
                    to: '/settings/audit-log',
                    search: (prev) => ({ ...prev, cursor: nextCursor }),
                  });
                }}
              />
            </div>
          ) : undefined
        }
      />

      {/* Audit detail drawer — fetches individual entry on open */}
      <AuditDetailDrawer
        entryId={selectedId ?? ''}
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
      />
    </div>
  );
}
