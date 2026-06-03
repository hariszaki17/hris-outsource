import { classifyError } from '@/lib/api-error.ts';
import { type AuditLogEntry, useGetAuditLogEntry } from '@swp/api-client/e1';
import {
  Button,
  DateText,
  Drawer,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  Skeleton,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { Copy, ExternalLink, Lock, Maximize2, Shield } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * E1 · Audit Detail Drawer — full before/after diff for a single audit entry (F1.3 / AL-2).
 * Built from `.pen` frame `x5wrt` (width 560).
 *
 * Deep-link "Lihat entitas sumber" is a no-op info toast — target screens are in later epics.
 * Code-surface dark bg uses token `bg-code-surface` if available; falls back to the ONE
 * allowed literal `#0B0F19` for the dark viewer surface (noted deviation).
 * "Salin JSON" copies after-snapshot to clipboard.
 */

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Bahasa label for entity_type snake-case values (shared with screen). */
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

/** Map action verb family to a StatusTone. Mirrors audit-log-screen logic. */
type StatusTone = 'ok' | 'bad' | 'neutral' | 'info' | 'warn' | 'onprogress';
function actionTone(action: string): StatusTone {
  const v = action.toLowerCase();
  if (/approve|verify|create/.test(v)) return 'ok';
  if (/reject|deactivate|delete/.test(v)) return 'bad';
  if (/auto|system|migrate/.test(v)) return 'neutral';
  if (/change|transfer|update/.test(v)) return 'info';
  return 'neutral';
}

/** Serialise a value to a compact JSON snippet for the diff viewer. */
function jsonVal(v: unknown): string {
  return JSON.stringify(v);
}

// ---------------------------------------------------------------------------
// Diff computation
// ---------------------------------------------------------------------------

type DiffLine =
  | { kind: 'equal'; key: string; value: unknown }
  | { kind: 'removed'; key: string; value: unknown }
  | { kind: 'added'; key: string; value: unknown }
  | { kind: 'changed'; key: string; before: unknown; after: unknown };

function computeDiff(before: Record<string, unknown>, after: Record<string, unknown>): DiffLine[] {
  const allKeys = Array.from(new Set([...Object.keys(before), ...Object.keys(after)]));
  const lines: DiffLine[] = [];
  for (const key of allKeys) {
    const inBefore = Object.prototype.hasOwnProperty.call(before, key);
    const inAfter = Object.prototype.hasOwnProperty.call(after, key);
    if (inBefore && inAfter) {
      if (jsonVal(before[key]) === jsonVal(after[key])) {
        lines.push({ kind: 'equal', key, value: before[key] });
      } else {
        lines.push({ kind: 'changed', key, before: before[key], after: after[key] });
      }
    } else if (inBefore) {
      lines.push({ kind: 'removed', key, value: before[key] });
    } else {
      lines.push({ kind: 'added', key, value: after[key] });
    }
  }
  return lines;
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

/** A single key/value row in the meta grid card. */
function MetaRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-[12px] text-text-3">{label}</span>
      <span
        className={
          mono ? 'font-mono text-[12px] font-medium text-text' : 'text-[12px] font-medium text-text'
        }
      >
        {value}
      </span>
    </div>
  );
}

/** Single card in the 2-column meta grid. */
function MetaCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-1 flex-col gap-2.5 rounded-[10px] border border-border-soft bg-surface-2 p-[14px]">
      <span className="text-[11px] font-bold tracking-wide text-text-3 uppercase">{title}</span>
      {children}
    </div>
  );
}

/** Diff viewer code block — renders before/after diff with line highlights. */
function DiffViewer({
  before,
  after,
}: {
  before: Record<string, unknown>;
  after: Record<string, unknown>;
}) {
  const lines = computeDiff(before, after);

  return (
    /*
     * Code surface: dark viewer bg. The design system does not yet define a `bg-code-surface`
     * token — using the ONE permitted literal `#0B0F19` per the build instructions.
     * DEVIATION: raw hex on this element only, pending a token addition to design-tokens.
     */
    <div
      className="overflow-x-auto rounded-[10px] p-3 text-[12px] leading-5"
      style={{ background: '#0B0F19' }}
    >
      <pre className="m-0 font-mono">
        {lines.map((line, idx) => {
          const key = `${line.key}-${idx}`;
          if (line.kind === 'equal') {
            return (
              <div key={key} className="text-text-3">
                {`  "${line.key}": ${jsonVal(line.value)},`}
              </div>
            );
          }
          if (line.kind === 'removed') {
            return (
              <div key={key} className="bg-red-950/50 text-rose-400">
                {`- "${line.key}": ${jsonVal(line.value)},`}
              </div>
            );
          }
          if (line.kind === 'added') {
            return (
              <div key={key} className="bg-emerald-950/50 text-emerald-400">
                {`+ "${line.key}": ${jsonVal(line.value)},`}
              </div>
            );
          }
          // changed: emit a removed line then an added line
          return (
            <span key={key}>
              <div className="bg-red-950/50 text-rose-400">
                {`- "${line.key}": ${jsonVal(line.before)},`}
              </div>
              <div className="bg-emerald-950/50 text-emerald-400">
                {`+ "${line.key}": ${jsonVal(line.after)},`}
              </div>
            </span>
          );
        })}
      </pre>
    </div>
  );
}

/** Skeleton body for the loading state. */
function DrawerBodySkeleton() {
  return (
    <div className="flex flex-col gap-4" aria-hidden="true">
      <div className="flex gap-3">
        <div className="flex flex-1 flex-col gap-2.5 rounded-[10px] border border-border-soft bg-surface-2 p-[14px]">
          <Skeleton className="h-2.5 w-16" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-3/4" />
          <Skeleton className="h-3 w-1/2" />
        </div>
        <div className="flex flex-1 flex-col gap-2.5 rounded-[10px] border border-border-soft bg-surface-2 p-[14px]">
          <Skeleton className="h-2.5 w-16" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-2/3" />
        </div>
      </div>
      <Skeleton className="h-2.5 w-28" />
      <Skeleton className="h-32 w-full rounded-[10px]" />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main export
// ---------------------------------------------------------------------------

export interface AuditDetailDrawerProps {
  entryId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * AuditDetailDrawer — renders the full before/after diff for a single audit log entry.
 * Frame: `.pen` `x5wrt` · width 560.
 * Fetches via `useGetAuditLogEntry`; gated by `open && !!entryId`.
 */
export function AuditDetailDrawer({ entryId, open, onOpenChange }: AuditDetailDrawerProps) {
  const { t } = useTranslation();
  const { toast } = useToast();

  const query = useGetAuditLogEntry(entryId, {
    query: { enabled: open && Boolean(entryId) },
  });

  // The Orval wrapper puts the actual body at `query.data?.data`.
  const entry = query.data?.data as AuditLogEntry | undefined;

  // --- Build header title from entry fields ---
  const headerTitle = entry
    ? `${entityLabel(entry.entity_type)} #${entry.entity_id} · ${entry.change_summary}`
    : t('auditLog.drawerTitle');

  const headerSubtitle = entry
    ? [
        entry.created_at
          ? undefined // DateText rendered inline below
          : null,
        entry.ip ? `IP ${entry.ip}` : null,
      ]
        .filter(Boolean)
        .join(' · ')
    : undefined;

  function handleCopyJson() {
    if (!entry) return;
    const json = JSON.stringify(entry.after, null, 2);
    navigator.clipboard.writeText(json).then(
      () => toast({ tone: 'success', title: t('auditLog.copiedToast') }),
      () => toast({ tone: 'error', title: t('auditLog.copyFailedToast') }),
    );
  }

  function handleDeepLink() {
    // Target screen belongs to a later epic. No-op with info toast.
    toast({
      tone: 'info',
      title: t('auditLog.deepLinkToastTitle'),
      description: t('auditLog.deepLinkToastBody'),
    });
  }

  return (
    <Drawer open={open} onOpenChange={onOpenChange} width={560}>
      <DrawerHeader
        title={headerTitle}
        subtitle={
          entry
            ? undefined // subtitle rendered with DateText component — see leading slot note
            : (headerSubtitle ?? t('auditLog.drawerTitle'))
        }
        leading={
          entry ? (
            <StatusBadge dot tone={actionTone(entry.action)}>
              <span className="font-mono text-[11px]">{entry.action}</span>
            </StatusBadge>
          ) : undefined
        }
        closeLabel={t('common.close')}
        onClose={() => onOpenChange(false)}
      />

      <DrawerBody>
        {/* ---- Loading state ---- */}
        {query.isLoading && <DrawerBodySkeleton />}

        {/* ---- Error state ---- */}
        {query.isError &&
          (() => {
            const { kind } = classifyError(query.error);
            if (kind === 'forbidden' || kind === 'unauthenticated') {
              return (
                <StateView
                  kind="no-permission"
                  title={t('errors.forbidden')}
                  description={t('auditLog.noPermissionBody')}
                />
              );
            }
            return (
              <StateView
                kind="error"
                title={t('auditLog.errorTitle')}
                description={t('errors.network')}
                onRetry={() => query.refetch()}
                retryLabel={t('common.retry')}
              />
            );
          })()}

        {/* ---- Loaded state ---- */}
        {entry && (
          <>
            {/* Subtitle with DateText + IP (full mono line) */}
            <div className="flex items-center gap-1.5 font-mono text-[11px] text-text-3">
              <DateText kind="instant" value={entry.created_at} className="font-mono text-[11px]" />
              <span>· WIB</span>
              {entry.ip && <span>· IP {entry.ip}</span>}
            </div>

            {/* Meta grid — 2 cards side by side */}
            <div className="flex gap-3">
              <MetaCard title={t('auditLog.metaAktor')}>
                <MetaRow
                  label={t('auditLog.metaAktorNama')}
                  value={entry.actor_label ?? t('auditLog.systemActor')}
                />
                {entry.actor_user_id && (
                  <MetaRow label={t('auditLog.metaAktorId')} value={entry.actor_user_id} mono />
                )}
              </MetaCard>
              <MetaCard title={t('auditLog.metaEntitas')}>
                <MetaRow
                  label={t('auditLog.metaEntitasTipe')}
                  value={entityLabel(entry.entity_type)}
                />
                <MetaRow label={t('auditLog.metaEntitasId')} value={entry.entity_id} mono />
              </MetaCard>
            </div>

            {/* Diff section */}
            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-bold tracking-wide text-text-3 uppercase">
                  {t('auditLog.diffTitle')}
                </span>
                <div className="flex items-center gap-1.5">
                  <button
                    type="button"
                    aria-label={t('auditLog.copyJson')}
                    onClick={handleCopyJson}
                    className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] text-text-3 hover:bg-surface-2 hover:text-text"
                  >
                    <Copy className="size-3.5" aria-hidden="true" />
                    {t('auditLog.copyJson')}
                  </button>
                  <button
                    type="button"
                    aria-label={t('auditLog.showFull')}
                    onClick={() => {
                      /* Expand / full-screen diff is a future enhancement */
                    }}
                    className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] text-text-3 hover:bg-surface-2 hover:text-text"
                  >
                    <Maximize2 className="size-3.5" aria-hidden="true" />
                    {t('auditLog.showFull')}
                  </button>
                </div>
              </div>
              <DiffViewer
                before={entry.before as Record<string, unknown>}
                after={entry.after as Record<string, unknown>}
              />
            </div>

            {/* Masked value note */}
            <div className="flex items-start gap-2.5 rounded-md border border-warn-bd bg-warn-bg px-3.5 py-3">
              <Shield className="mt-0.5 size-4 shrink-0 text-warn-tx" aria-hidden="true" />
              <p className="text-[12px] leading-[1.5] text-warn-tx">{t('auditLog.maskedNote')}</p>
            </div>

            {/* Deep link row — no-op, target screen is in a later epic */}
            <button
              type="button"
              onClick={handleDeepLink}
              className="flex w-full items-center justify-between rounded-[8px] bg-primary-soft px-3.5 py-3 text-left"
            >
              <div className="flex items-center gap-2 text-primary">
                <ExternalLink className="size-4 shrink-0" aria-hidden="true" />
                <span className="text-[13px] font-medium">
                  {t('auditLog.deepLinkLabel', {
                    entity: entityLabel(entry.entity_type),
                    id: entry.entity_id,
                  })}
                </span>
              </div>
              <ExternalLink className="size-3.5 text-primary" aria-hidden="true" />
            </button>
          </>
        )}
      </DrawerBody>

      <DrawerFooter>
        {/* Left — append-only lock chip */}
        <div className="mr-auto flex items-center gap-1.5 text-text-3">
          <Lock className="size-3.5 shrink-0" aria-hidden="true" />
          <span className="text-[12px]">{t('auditLog.appendOnlyNote')}</span>
        </div>
        {/* Right — close button */}
        <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
          {t('common.close')}
        </Button>
      </DrawerFooter>
    </Drawer>
  );
}
