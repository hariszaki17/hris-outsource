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
import { MoreVertical } from 'lucide-react';
import type * as React from 'react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ChangeRoleModal,
  DeactivateUserConfirm,
  EditUserDrawer,
  ReactivateUserConfirm,
  SendResetConfirm,
  UserRowActionsMenu,
} from './user-overlays.tsx';

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

// ---------------------------------------------------------------------------
// Overlay state types
// ---------------------------------------------------------------------------

type OverlayKind = 'edit' | 'change-role' | 'send-reset' | 'deactivate' | 'reactivate' | null;

export function UsersScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const search = useSearch({ from: '/authed/settings/users' });
  // Cursor history for the Prev button (cursors are forward-only).
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  // ---------------------------------------------------------------------------
  // Overlay state
  // ---------------------------------------------------------------------------
  const [overlayKind, setOverlayKind] = useState<OverlayKind>(null);
  const [activeUser, setActiveUser] = useState<User | null>(null);
  // Per-row menu anchor management
  const [openMenuUserId, setOpenMenuUserId] = useState<string | null>(null);
  // Ref map for kebab button anchors — one ref per rendered row
  const kebabRefs = useRef<Map<string, HTMLButtonElement>>(new Map());

  // Wrap anchor as a RefObject shape for UserRowActionsMenu
  const activeMenuAnchorRef = useRef<HTMLElement | null>(null);

  function openOverlay(kind: Exclude<OverlayKind, null>, user?: User) {
    setOverlayKind(kind);
    setActiveUser(user ?? null);
  }

  function closeOverlay() {
    setOverlayKind(null);
    // Do not clear activeUser immediately so closing animations can finish.
  }

  function handleDone() {
    closeOverlay();
    void query.refetch();
  }

  // ---------------------------------------------------------------------------
  // List query
  // ---------------------------------------------------------------------------

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
      // Any filter change resets pagination. Build from the typed `search` (this route's own
      // shape) rather than the navigate `prev` (a cross-route union) to keep the result typed.
      search: { ...search, cursor: undefined, ...patch },
    });
    setPrevCursors([]);
  };

  // ---------------------------------------------------------------------------
  // Columns
  // ---------------------------------------------------------------------------

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
            <span className="text-text-3 text-xs">{u.email ?? u.phone}</span>
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

  // ---------------------------------------------------------------------------
  // Title band
  // ---------------------------------------------------------------------------

  const titleBand = (
    <div className="flex items-start justify-between">
      <div className="flex flex-col gap-1">
        <h1 className="font-bold text-3xl text-text">{t('users.title')}</h1>
        <p className="max-w-[640px] text-sm text-text-3">{t('users.subtitle')}</p>
      </div>
      {/* Admins are now created via employee-create + change-role (D1); no standalone create-user. */}
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
        rowActions={(u) => {
          const isOpen = openMenuUserId === u.id;

          // Build a stable anchor ref holder for this row
          const setRef = (el: HTMLButtonElement | null) => {
            if (el) {
              kebabRefs.current.set(u.id, el);
            } else {
              kebabRefs.current.delete(u.id);
            }
          };

          // Wrap the per-row button ref as a RefObject so we can pass it to the menu
          const anchorRef: React.RefObject<HTMLElement | null> = {
            get current() {
              return kebabRefs.current.get(u.id) ?? null;
            },
          };

          return (
            <div className="relative">
              <button
                ref={setRef}
                type="button"
                aria-label={t('users.rowActions')}
                aria-expanded={isOpen}
                aria-haspopup="menu"
                className="flex size-[30px] items-center justify-center rounded-md text-text-3 hover:bg-surface-2"
                onClick={() => {
                  if (isOpen) {
                    setOpenMenuUserId(null);
                  } else {
                    activeMenuAnchorRef.current = kebabRefs.current.get(u.id) ?? null;
                    setOpenMenuUserId(u.id);
                  }
                }}
              >
                <MoreVertical className="size-4" aria-hidden />
              </button>

              <UserRowActionsMenu
                user={u}
                open={isOpen}
                anchorRef={anchorRef}
                onClose={() => setOpenMenuUserId(null)}
                onEdit={() => openOverlay('edit', u)}
                onChangeRole={() => openOverlay('change-role', u)}
                onSendReset={() => openOverlay('send-reset', u)}
                onToggleStatus={() =>
                  openOverlay(u.status === UserStatus.DISABLED ? 'reactivate' : 'deactivate', u)
                }
              />
            </div>
          );
        }}
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
                  search: { ...search, cursor: cursor || undefined },
                });
              }}
              onNext={() => {
                const nextCursor = page?.next_cursor;
                if (!nextCursor) return;
                setPrevCursors((s) => [...s, search.cursor ?? '']);
                void navigate({
                  to: '/settings/users',
                  search: { ...search, cursor: nextCursor },
                });
              }}
            />
          ) : undefined
        }
      />

      {/* ---------------------------------------------------------------------------
          Overlays — rendered once, driven by overlayKind + activeUser state
          --------------------------------------------------------------------------- */}

      <EditUserDrawer
        open={overlayKind === 'edit'}
        onOpenChange={(open) => {
          if (!open) closeOverlay();
        }}
        user={activeUser}
        onDone={handleDone}
        onRequestChangeRole={(u) => openOverlay('change-role', u)}
        onRequestToggleStatus={(u) =>
          openOverlay(u.status === UserStatus.DISABLED ? 'reactivate' : 'deactivate', u)
        }
      />

      <ChangeRoleModal
        open={overlayKind === 'change-role'}
        onOpenChange={(open) => {
          if (!open) closeOverlay();
        }}
        user={activeUser}
        onDone={handleDone}
      />

      <SendResetConfirm
        open={overlayKind === 'send-reset'}
        onOpenChange={(open) => {
          if (!open) closeOverlay();
        }}
        user={activeUser}
        onDone={handleDone}
      />

      <DeactivateUserConfirm
        open={overlayKind === 'deactivate'}
        onOpenChange={(open) => {
          if (!open) closeOverlay();
        }}
        user={activeUser}
        onDone={handleDone}
      />

      <ReactivateUserConfirm
        open={overlayKind === 'reactivate'}
        onOpenChange={(open) => {
          if (!open) closeOverlay();
        }}
        user={activeUser}
        onDone={handleDone}
      />
    </div>
  );
}
