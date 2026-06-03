import {
  NAV_ITEMS,
  SETTINGS_ITEM,
  activeSection,
  hasPermission,
  subnavForSection,
  visibleNav,
} from '@/app/nav.ts';
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
} from '@swp/ui';
import { Link, Outlet, useNavigate, useRouterState } from '@tanstack/react-router';
import { Bell } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * Authenticated app shell (DESIGN-SYSTEM §5): dark 240px sidebar · 64px white topbar
 * (breadcrumb left, user right) · app-bg content area. Composed from comp/Sidebar `iCqTB`
 * and comp/Topbar `caFkE` (packages/ui). The sidebar holds the 8 primary modules only; a section
 * sub-nav strip surfaces a section's sub-pages under the topbar (nav.ts SECTION_SUBNAV). Nav is
 * filtered by role via the interim x-rbac map (ENGINEERING.md A2/C1 — defense-in-depth, not the gate).
 */
export function AppShell() {
  const { t } = useTranslation();
  const user = useCurrentUser();
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  // Defensive: the route guard requires a token, but a session may lack a user (e.g. mid
  // refresh). Render nothing rather than a broken chrome; the guard handles redirects.
  if (!user) return <Outlet />;

  const items = visibleNav(NAV_ITEMS, user.permissions);
  const showSettings = hasPermission(user.permissions, SETTINGS_ITEM.requires);

  const inSettings = pathname.startsWith('/settings');
  const section = inSettings ? '/settings' : activeSection(pathname);
  const subnav = inSettings ? [] : subnavForSection(section, user.permissions);

  // Active sub-nav tab = the longest sub-route that prefixes the current path.
  const activeSub = subnav.reduce<string | null>((best, s) => {
    const match = s.to === '/' ? pathname === '/' : pathname.startsWith(s.to);
    if (!match) return best;
    if (best === null || s.to.length > best.length) return s.to;
    return best;
  }, null);

  const activePrimary = [...items, SETTINGS_ITEM].find((i) => i.to === section);
  const crumbs = activePrimary ? [{ label: t(activePrimary.labelKey), current: true }] : [];

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
          <SidebarNavItem key={item.to} icon={item.icon} active={item.to === section} asChild>
            <Link to={item.to}>{t(item.labelKey)}</Link>
          </SidebarNavItem>
        ))}
        <SidebarSpacer />
        {showSettings && (
          <SidebarFooter>
            <SidebarNavItem icon={SETTINGS_ITEM.icon} active={inSettings} asChild>
              <Link to={SETTINGS_ITEM.to}>{t(SETTINGS_ITEM.labelKey)}</Link>
            </SidebarNavItem>
          </SidebarFooter>
        )}
      </Sidebar>

      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar
          left={<Breadcrumb items={crumbs} />}
          right={
            <div className="flex items-center gap-2">
              <TopbarIconButton
                icon={Bell}
                label={t('shell.notifications')}
                onClick={() => navigate({ to: '/notifications' })}
              />
              <UserMenu user={user} />
            </div>
          }
        />
        {subnav.length > 0 && (
          <nav
            aria-label={activePrimary ? t(activePrimary.labelKey) : undefined}
            className="flex items-center gap-1 border-b border-border bg-surface px-6"
          >
            {subnav.map((s) => {
              const isActive = s.to === activeSub;
              return (
                <Link
                  key={s.to}
                  to={s.to}
                  className={[
                    '-mb-px border-b-2 px-3 py-3 text-[13px] font-medium transition-colors',
                    isActive
                      ? 'border-primary text-primary'
                      : 'border-transparent text-text-2 hover:text-text',
                  ].join(' ')}
                >
                  {t(s.labelKey)}
                </Link>
              );
            })}
          </nav>
        )}
        <main className="flex-1 overflow-auto bg-app p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
