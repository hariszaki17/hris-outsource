import type { SessionUser } from '@/lib/auth.ts';
import { TopbarUser, cn } from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { LogOut, Settings } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Topbar user menu (feature organism — knows about auth + routes, so it lives in the app, not
 * packages/ui per ENGINEERING.md G2). `TopbarUser` is the trigger; a lightweight popover holds
 * Settings (admins) + Sign out. Closes on outside-click and Escape.
 */
export function UserMenu({ user, onLogout }: { user: SessionUser; onLogout: () => void }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onPointerDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const isAdmin = user.role === 'super_admin' || user.role === 'hr_admin';

  function handleLogout() {
    setOpen(false);
    onLogout();
  }

  return (
    <div ref={ref} className="relative">
      <TopbarUser
        name={user.name}
        roleLabel={user.companyName ?? t(`role.${user.role}`)}
        initials={user.initials}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      />

      {open && (
        <div className="absolute right-0 z-50 mt-1 w-52 overflow-hidden rounded-md border border-border bg-surface py-1 shadow-overlay">
          <div className="border-border-soft border-b px-3 py-2">
            <p className="font-semibold text-sm text-text">{user.name}</p>
            <p className="text-text-3 text-xs">{t(`role.${user.role}`)}</p>
          </div>
          {isAdmin && (
            <MenuItem
              icon={Settings}
              label={t('nav.settings')}
              onClick={() => {
                setOpen(false);
                void navigate({ to: '/settings' });
              }}
            />
          )}
          <MenuItem icon={LogOut} label={t('common.logout')} onClick={handleLogout} tone="danger" />
        </div>
      )}
    </div>
  );
}

function MenuItem({
  icon: Icon,
  label,
  onClick,
  tone,
}: {
  icon: typeof Settings;
  label: string;
  onClick: () => void;
  tone?: 'danger';
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm hover:bg-surface-2',
        tone === 'danger' ? 'text-bad-tx' : 'text-text',
      )}
    >
      <Icon className="size-4" aria-hidden />
      {label}
    </button>
  );
}
