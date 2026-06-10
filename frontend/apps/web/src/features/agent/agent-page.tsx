/**
 * AgentPage — shared layout wrapper for the agent self-service screens (/me/*).
 *
 * Agent screens render inside the standard AppShell (which provides the agent nav backbone),
 * so this is just the content column: a centered, comfortable-width stack with an optional
 * header (title + subtitle + optional back link) and optional header actions. Keeps every
 * /me/* screen visually consistent without a new packages/ui component (G2 — promote only on
 * a second domain-agnostic reuse). docs/eng/AGENT-WEB-ACCESS.md §6.
 */
import { Link } from '@tanstack/react-router';
import { ArrowLeft } from 'lucide-react';
import type { ReactNode } from 'react';

interface AgentPageProps {
  title: string;
  subtitle?: string;
  /** Optional back link target (e.g. '/me/leave'); renders a "back" affordance above the title. */
  backTo?: string;
  backLabel?: string;
  /** Optional right-aligned header actions (buttons, links). */
  actions?: ReactNode;
  children: ReactNode;
}

export function AgentPage({
  title,
  subtitle,
  backTo,
  backLabel,
  actions,
  children,
}: AgentPageProps) {
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-5">
      {backTo && (
        <Link
          to={backTo as never}
          className="flex w-fit items-center gap-[7px] text-[13px] font-medium text-text-2 hover:text-text"
        >
          <ArrowLeft size={16} aria-hidden />
          {backLabel ?? 'Kembali'}
        </Link>
      )}
      <div className="flex items-start justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-[20px] font-bold text-text">{title}</h1>
          {subtitle && <p className="text-[13px] text-text-2">{subtitle}</p>}
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
      {children}
    </div>
  );
}
