import type { StatusTone } from '@swp/design-tokens';
import { AlertTriangle, CircleX, Info, type LucideIcon } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * Banner (molecule) — page/section-level alert (DESIGN-SYSTEM §6). Tone-driven, token colors only.
 * Used e.g. for the login "email/sandi salah" inline error (.pen `E1 · Login — Gagal`).
 */
const toneClass: Record<'bad' | 'warn' | 'info', string> = {
  bad: 'bg-bad-bg text-bad-tx border-bad-bd',
  warn: 'bg-warn-bg text-warn-tx border-warn-bd',
  info: 'bg-info-bg text-info-tx border-info-bd',
};

const toneIcon: Record<'bad' | 'warn' | 'info', React.ComponentType<{ className?: string }>> = {
  bad: CircleX,
  warn: AlertTriangle,
  info: Info,
};

export interface BannerProps {
  tone: Extract<StatusTone, 'bad' | 'warn' | 'info'>;
  title: string;
  description?: string;
  /** Override the tone's default icon (e.g. `lock` for a locked-account error). */
  icon?: LucideIcon;
  className?: string;
}

export function Banner({ tone, title, description, icon, className }: BannerProps) {
  const Icon = icon ?? toneIcon[tone];
  return (
    <div
      role="alert"
      className={cn('flex gap-2 rounded-md border px-3 py-2.5', toneClass[tone], className)}
    >
      <Icon className="mt-0.5 size-3.5 shrink-0" />
      <div className="flex flex-col gap-0.5">
        <p className="font-semibold text-[13px] leading-snug">{title}</p>
        {description && <p className="text-xs leading-normal">{description}</p>}
      </div>
    </div>
  );
}
