import { SettingsSubnav, SettingsSubnavItem } from '@swp/ui';
import { Link, Outlet, useRouterState } from '@tanstack/react-router';
import { LayoutDashboard, ScrollText, Settings, UsersRound } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * Settings section layout (.pen `E1 · Pengaturan` frames): left `SettingsSubnav` rail (220) +
 * the active sub-page in an `<Outlet>`. Rendered inside the app shell's content area. The
 * sub-nav is the shared chrome; each sub-page supplies its own title band + content.
 */
const SUBNAV: { to: string; labelKey: string; icon: LucideIcon }[] = [
  { to: '/settings', labelKey: 'settings.overview', icon: LayoutDashboard },
  { to: '/settings/users', labelKey: 'settings.users', icon: UsersRound },
  { to: '/settings/audit-log', labelKey: 'settings.auditLog', icon: ScrollText },
  { to: '/settings/general', labelKey: 'settings.general', icon: Settings },
];

export function SettingsLayout() {
  const { t } = useTranslation();
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  return (
    <div className="flex gap-6">
      <div className="w-[220px] shrink-0">
        <SettingsSubnav label={t('nav.settings')}>
          {SUBNAV.map((item) => {
            const active =
              item.to === '/settings' ? pathname === '/settings' : pathname.startsWith(item.to);
            return (
              <SettingsSubnavItem key={item.to} icon={item.icon} active={active} asChild>
                <Link to={item.to}>{t(item.labelKey)}</Link>
              </SettingsSubnavItem>
            );
          })}
        </SettingsSubnav>
      </div>
      <div className="min-w-0 flex-1">
        <Outlet />
      </div>
    </div>
  );
}
