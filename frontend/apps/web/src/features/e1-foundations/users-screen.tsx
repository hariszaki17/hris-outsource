import { classifyError } from '@/lib/api-error.ts';
import {
  type ListUsers200,
  type ListUsersParams,
  Role,
  type User,
  UserStatus,
  useListUsers,
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
} from '@swp/ui';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { MoreVertical, UserPlus } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * E1 · Pengguna & Peran — RBAC user management list (F1.2 / rbac-roles.md). Built from `.pen`
 * frame `kHNWT` (+ empty `tg1Cf`, no-permission `TqMQ6`). Data via the generated `useListUsers`
 * hook over MSW; filters live in typed URL search params (D1, shareable + stable cache key);
 * cursor pagination only (D1). Client gating is defense-in-depth — the API is the gate (C1).
 */
const PAGE_SIZE = 50;

/** Typed filter/cursor search params for `/settings/users` (mirrors the route's validateSearch). */
type UsersSearch = { q?: string; role?: Role; status?: UserStatus; cursor?: string };

const roleTone: Record<string, StatusTone> = {
  super_admin: 'info',
  hr_admin: 'info',
  shift_leader: 'neutral',
  agent: 'neutral',
};

function initials(name: string): string {
  return name
    .split(' ')
    .slice(0, 2)
    .map((p) => p[0] ?? '')
    .join('')
    .toUpperCase();
}

export function UsersScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const search = useSearch({ from: '/authed/settings/users' });
  // Cursor history for the Prev button (cursors are forward-only).
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  const params: ListUsersParams = {
    limit: PAGE_SIZE,
    q: search.q || undefined,
    role: search.role,
    status: search.status,
    cursor: search.cursor,
  };
  const query = useListUsers(params);

  const hasFilters = Boolean(search.q || search.role || search.status);

  const setSearch = (patch: UsersSearch) => {
    void navigate({
      to: '/settings/users',
      // Any filter change resets pagination.
      search: (prev) => ({ ...prev, cursor: undefined, ...patch }),
    });
    setPrevCursors([]);
  };

  const columns: Column<User>[] = [
    {
      id: 'user',
      header: t('users.colUser'),
      width: 310,
      cell: (u) => (
        <div className="flex items-center gap-2.5">
          <Avatar initials={initials(u.full_name)} size={34} />
          <div className="flex flex-col">
            <span className="font-medium text-text">{u.full_name}</span>
            <span className="text-text-3 text-xs">{u.email}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'role',
      header: t('users.colRole'),
      width: 170,
      cell: (u) => (
        <StatusBadge dot tone={roleTone[u.role] ?? 'neutral'}>
          {t(`role.${u.role}`)}
        </StatusBadge>
      ),
    },
    {
      id: 'company',
      header: t('users.colCompany'),
      width: 220,
      cell: (u) => u.company_name ?? '—',
    },
    {
      id: 'status',
      header: t('users.colStatus'),
      width: 120,
      cell: (u) => (
        <StatusBadge dot tone={u.status === UserStatus.ACTIVE ? 'ok' : 'bad'}>
          {u.status === UserStatus.ACTIVE ? t('users.statusActive') : t('users.statusDisabled')}
        </StatusBadge>
      ),
    },
    {
      id: 'lastLogin',
      header: t('users.colLastLogin'),
      width: 180,
      cell: (u) =>
        u.last_login_at ? (
          <DateText kind="instant" value={u.last_login_at} className="text-text-2 text-sm" />
        ) : (
          <span className="text-text-3 text-sm">{t('users.neverLoggedIn')}</span>
        ),
    },
  ];

  const titleBand = (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="font-bold text-3xl text-text">{t('users.title')}</h1>
        <p className="max-w-[640px] text-sm text-text-3">{t('users.subtitle')}</p>
      </div>
      <Button>
        <UserPlus />
        {t('users.add')}
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
            description={t('users.noPermissionBody')}
          />
        ) : (
          <StateView
            kind="error"
            title={t('users.errorTitle')}
            description={t('errors.network')}
            onRetry={() => query.refetch()}
            retryLabel={t('common.retry')}
          />
        )}
      </div>
    );
  }

  // Orval fetch-client wraps the response: `query.data` is `{ data, status, headers }` and the
  // body lives under `.data`. Non-2xx threw above, so on success the body is `ListUsers200`.
  const page = query.data?.data as ListUsers200 | undefined;
  const rows = (page?.data ?? []) as User[];

  return (
    <div className="flex flex-col gap-[18px]">
      {titleBand}

      <div className="flex flex-wrap items-center gap-2.5">
        <SearchField
          placeholder={t('users.searchPlaceholder')}
          defaultValue={search.q ?? ''}
          containerClassName="w-64"
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
        />
        <FilterSelect
          aria-label={t('users.colRole')}
          value={search.role ?? ''}
          onChange={(e) => setSearch({ role: (e.target.value as Role) || undefined })}
        >
          <option value="">{t('users.filterRole')}</option>
          {Object.values(Role).map((r) => (
            <option key={r} value={r}>
              {t(`role.${r}`)}
            </option>
          ))}
        </FilterSelect>
        <FilterSelect
          aria-label={t('users.colStatus')}
          value={search.status ?? ''}
          onChange={(e) => setSearch({ status: (e.target.value as UserStatus) || undefined })}
        >
          <option value="">{t('users.filterStatus')}</option>
          <option value={UserStatus.ACTIVE}>{t('users.statusActive')}</option>
          <option value={UserStatus.DISABLED}>{t('users.statusDisabled')}</option>
        </FilterSelect>
      </div>

      <DataTable
        aria-label={t('users.title')}
        columns={columns}
        data={rows}
        getRowId={(u) => u.id}
        isLoading={query.isLoading}
        skeletonRows={6}
        empty={
          hasFilters ? (
            <EmptyState
              variant="filtered"
              title={t('users.filteredTitle')}
              description={t('users.filteredBody')}
            />
          ) : (
            <EmptyState
              variant="fresh"
              title={t('users.emptyTitle')}
              description={t('users.emptyBody')}
            />
          )
        }
        rowActions={() => (
          <button
            type="button"
            aria-label={t('users.rowActions')}
            className="flex size-[30px] items-center justify-center rounded-md text-text-3 hover:bg-surface-2"
          >
            <MoreVertical className="size-4" aria-hidden />
          </button>
        )}
        footer={
          rows.length > 0 ? (
            <CursorPagination
              rangeLabel={t('users.resultRange', { count: rows.length })}
              hasPrev={prevCursors.length > 0}
              hasNext={Boolean(page?.has_more)}
              prevLabel={t('common.prev')}
              nextLabel={t('common.next')}
              onPrev={() => {
                const next = [...prevCursors];
                const cursor = next.pop();
                setPrevCursors(next);
                void navigate({
                  to: '/settings/users',
                  search: (prev) => ({ ...prev, cursor: cursor || undefined }),
                });
              }}
              onNext={() => {
                const nextCursor = page?.next_cursor;
                if (!nextCursor) return;
                setPrevCursors((s) => [...s, search.cursor ?? '']);
                void navigate({
                  to: '/settings/users',
                  search: (prev) => ({ ...prev, cursor: nextCursor }),
                });
              }}
            />
          ) : undefined
        }
      />
    </div>
  );
}
