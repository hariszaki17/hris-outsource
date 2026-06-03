import { NAV_ITEMS, SETTINGS_ITEM, navForRole } from '@/app/nav.ts';
import { UserMenu } from '@/app/user-menu.tsx';
import { useCurrentUser } from '@/lib/use-auth.ts';
import {
  Breadcrumb,
  Sidebar,
  SidebarBrand,
  SidebarFooter,
  SidebarNavItem,
  SidebarSectionLabel,
  SidebarSpacer,
  Topbar,
  TopbarIconButton,
  TopbarSearch,
} from '@swp/ui';
import { Link, Outlet, useRouterState } from '@tanstack/react-router';
import { Bell } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * Authenticated app shell (DESIGN-SYSTEM §5): dark 240px sidebar · 64px white topbar
 * (breadcrumb left, user right) · app-bg content area. Composed from comp/Sidebar `iCqTB`
 * and comp/Topbar `caFkE` (packages/ui). Nav is filtered by the signed-in user's role via the
 * interim x-rbac map (ENGINEERING.md A2/C1 — client gating is defense-in-depth, not the gate).
 */
export function AppShell() {
  const { t } = useTranslation();
  const user = useCurrentUser();
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  // Defensive: the route guard requires a token, but a session may lack a user (e.g. mid
  // refresh). Render nothing rather than a broken chrome; the guard handles redirects.
  if (!user) return <Outlet />;

  const items = navForRole(NAV_ITEMS, user.role);
  const showSettings = SETTINGS_ITEM.roles.includes(user.role);

  const isActive = (to: string) => (to === '/' ? pathname === '/' : pathname.startsWith(to));
  const active = [...items, SETTINGS_ITEM].find((i) => isActive(i.to));
  const crumbs = active ? [{ label: t(active.labelKey), current: true }] : [];

  return (
    <div className="flex h-full">
      <Sidebar>
        <SidebarBrand
          logo={<img src="/swp-logo.png" alt="SWP" className="size-full object-contain" />}
          title={t('shell.brandTitle')}
          subtitle={t('shell.brandSubtitle')}
        />
        <SidebarSectionLabel>{t('shell.menu')}</SidebarSectionLabel>
        {items.map((item) => (
          <SidebarNavItem key={item.to} icon={item.icon} active={isActive(item.to)} asChild>
            <Link to={item.to}>{t(item.labelKey)}</Link>
          </SidebarNavItem>
        ))}
        <SidebarSpacer />
        {showSettings && (
          <SidebarFooter>
            <SidebarNavItem icon={SETTINGS_ITEM.icon} active={isActive(SETTINGS_ITEM.to)} asChild>
              <Link to={SETTINGS_ITEM.to}>{t(SETTINGS_ITEM.labelKey)}</Link>
            </SidebarNavItem>
          </SidebarFooter>
        )}
      </Sidebar>

      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar
          left={<Breadcrumb items={crumbs} />}
          right={
            <>
              <TopbarSearch placeholder={t('shell.search')} aria-label={t('shell.search')} />
              <TopbarIconButton icon={Bell} label={t('shell.notifications')} />
              <UserMenu user={user} />
            </>
          }
        />
        <main className="flex-1 overflow-auto bg-app p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
