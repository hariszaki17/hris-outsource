import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Split-screen auth layout (.pen `E1 · Login`/`Lupa`/`Reset` frames): dark brand panel (560,
 * hidden < lg) + white form panel with a centered 380px card. Shared by every auth screen
 * (login, forgot, reset) so the brand panel + card chrome live in one place (reuse, ENGINEERING.md E3).
 * The card body is supplied via `children`.
 */
export function AuthLayout({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <div className="flex h-full">
      <aside className="hidden w-[560px] flex-col justify-between bg-sidebar p-12 lg:flex">
        <div className="flex items-center gap-3">
          <span className="flex size-12 items-center justify-center rounded-xl bg-white">
            <img src="/swp-logo.png" alt="SWP" className="size-[30px] object-contain" />
          </span>
          <div className="flex flex-col gap-0.5">
            <span className="font-display font-bold text-white text-xl">{t('auth.wordmark')}</span>
            <span className="text-sidebar-text text-xs">{t('auth.wordmarkSub')}</span>
          </div>
        </div>

        <div className="flex max-w-[400px] flex-col gap-[18px]">
          <h1 className="font-display font-bold text-[34px] text-white leading-[1.2]">
            {t('auth.brandHeadline')}
          </h1>
          <p className="text-sidebar-text text-sm leading-relaxed">{t('auth.brandSub')}</p>
        </div>

        <p className="text-text-3 text-xs">{t('auth.brandFoot')}</p>
      </aside>

      <div className="flex flex-1 items-center justify-center bg-surface p-10">
        <div className="flex w-[380px] flex-col gap-[18px]">{children}</div>
      </div>
    </div>
  );
}
