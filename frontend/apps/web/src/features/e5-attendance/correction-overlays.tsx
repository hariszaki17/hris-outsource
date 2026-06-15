/**
 * E5 · Koreksi Kehadiran — overlay layer for the (READ-ONLY) correction detail drawer.
 *
 * .pen frames implemented:
 *   sSKtK  HR · Koreksi · Detail  (CorrectionDetailDrawer)
 *
 * Approve/Reject moved to the E11 Kotak Masuk (approval inbox) — corrections route through the
 * generic approval engine exactly like LEAVE / OVERTIME, so the HR/SL `/corrections` surface is
 * now a read-only history. The drawer links to the correction's `approval_instance` for the live
 * approval chain.
 *
 * ENGINEERING.md F5.4 · BR-1..BR-5 · INV-1.
 */

import { classifyError } from '@/lib/api-error.ts';
import {
  type Correction,
  CorrectionStatus,
  CorrectionType,
  type GetCorrection200,
  useGetCorrection,
} from '@swp/api-client/e5';
import type { StatusTone } from '@swp/design-tokens';
import {
  Button,
  DateText,
  Drawer,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  StateView,
  StatusBadge,
} from '@swp/ui';
import { AlertTriangle, CheckCircle2, ExternalLink, FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

export function correctionStatusTone(status: CorrectionStatus): StatusTone {
  switch (status) {
    case CorrectionStatus.PENDING:
      return 'warn';
    case CorrectionStatus.APPROVED:
    case CorrectionStatus.APPLIED:
      return 'ok';
    case CorrectionStatus.REJECTED:
      return 'bad';
    case CorrectionStatus.CANCELLED:
      return 'neutral';
    default:
      return 'neutral';
  }
}

export function correctionTypeLabel(type: CorrectionType, t: (key: string) => string): string {
  switch (type) {
    case CorrectionType.CHECK_IN:
      return t('corrections.typeCheckIn');
    case CorrectionType.CHECK_OUT:
      return t('corrections.typeCheckOut');
    case CorrectionType.CODE:
      return t('corrections.typeCode');
    case CorrectionType.OTHER:
      return t('corrections.typeOther');
    case CorrectionType.NEW_ENTRY:
      return t('corrections.typeNewEntry');
    default:
      return type;
  }
}

function formatDiffValue(value: unknown): string {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'string') {
    if (/^\d{4}-\d{2}-\d{2}T/.test(value)) {
      try {
        return new Date(value).toLocaleString('id-ID', { timeZone: 'Asia/Jakarta' });
      } catch {
        return value;
      }
    }
    return value;
  }
  return String(value);
}

// ---------------------------------------------------------------------------
// 1) DiffTable — before/after field diff from correction.diff[] or proposed fields
// ---------------------------------------------------------------------------

interface DiffRow {
  field: string;
  before: unknown;
  after: unknown;
}

function buildDiffRows(correction: Correction, t: (key: string) => string): DiffRow[] {
  if (correction.diff && correction.diff.length > 0) {
    return correction.diff.map((item) => ({
      field: item.field,
      before: item.before,
      after: item.after,
    }));
  }
  const rows: DiffRow[] = [];
  const snap = correction.original_snapshot as Record<string, unknown>;
  if (correction.proposed_check_in_at !== undefined) {
    rows.push({
      field: t('corrections.fieldCheckIn'),
      before: snap?.check_in_at,
      after: correction.proposed_check_in_at,
    });
  }
  if (correction.proposed_check_out_at !== undefined) {
    rows.push({
      field: t('corrections.fieldCheckOut'),
      before: snap?.check_out_at,
      after: correction.proposed_check_out_at,
    });
  }
  if (correction.proposed_attendance_code_id !== undefined) {
    rows.push({
      field: t('corrections.fieldAttendanceCode'),
      before: snap?.attendance_code_id,
      after: correction.proposed_attendance_code_id,
    });
  }
  return rows;
}

interface DiffTableProps {
  correction: Correction;
}

function DiffTable({ correction }: DiffTableProps) {
  const { t } = useTranslation();
  const rows = buildDiffRows(correction, t);

  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="border-b border-border-soft">
          <th className="py-2 pr-4 text-left font-medium text-text-3">
            {t('corrections.diffField')}
          </th>
          <th className="py-2 pr-4 text-left font-medium text-text-3">
            {t('corrections.diffBefore')}
          </th>
          <th className="py-2 text-left font-medium text-text-3">{t('corrections.diffAfter')}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.field} className="border-b border-border-soft last:border-0">
            <td className="py-2.5 pr-4 font-medium text-text">{row.field}</td>
            <td className="py-2.5 pr-4 text-text-2 line-through opacity-60">
              {formatDiffValue(row.before)}
            </td>
            <td className="py-2.5 font-medium text-ok">{formatDiffValue(row.after)}</td>
          </tr>
        ))}
        {rows.length === 0 && (
          <tr>
            <td colSpan={3} className="py-4 text-center text-text-3">
              —
            </td>
          </tr>
        )}
      </tbody>
    </table>
  );
}

// ---------------------------------------------------------------------------
// 2) CorrectionDetailDrawer  (.pen frame sSKtK — HR · Koreksi · Detail)
//    READ-ONLY: decisions are made in the E11 Kotak Masuk. The drawer surfaces a link to
//    the correction's approval_instance (the live chain) instead of inline approve/reject.
// ---------------------------------------------------------------------------

export interface CorrectionDetailDrawerProps {
  open: boolean;
  correctionId: string | null;
  onOpenChange: (open: boolean) => void;
  onDone: () => void;
  /**
   * Optional: open the E11 approval-instance (chain) view. Integration wires it to the typed
   * inbox/request-detail route. When omitted, the link is hidden.
   */
  onOpenInstance?: (instanceId: string) => void;
}

export function CorrectionDetailDrawer({
  open,
  correctionId,
  onOpenChange,
  onOpenInstance,
}: CorrectionDetailDrawerProps) {
  const { t } = useTranslation();

  const query = useGetCorrection(correctionId ?? '', {
    query: { enabled: open && Boolean(correctionId) },
  });

  const page = query.data?.data as GetCorrection200 | undefined;
  const correction = page?.data;
  const isPending = correction?.status === CorrectionStatus.PENDING;

  return (
    <>
      <Drawer open={open} onOpenChange={onOpenChange} width={560}>
        <DrawerHeader
          title={
            correction
              ? `${t('corrections.detailTitle')} · ${correction.id}`
              : t('corrections.detailTitle')
          }
        />

        <DrawerBody>
          {query.isLoading && <StateView kind="loading" title={t('corrections.loading')} />}

          {query.isError &&
            (() => {
              const { kind } = classifyError(query.error);
              if (kind === 'forbidden' || kind === 'unauthenticated') {
                return (
                  <StateView
                    kind="empty"
                    title={t('errors.forbidden')}
                    description={t('corrections.noPermissionBody')}
                  />
                );
              }
              return (
                <StateView
                  kind="error"
                  title={t('corrections.errorTitle')}
                  description={t('errors.network')}
                  onRetry={() => query.refetch()}
                  retryLabel={t('common.retry')}
                />
              );
            })()}

          {correction && (
            <div className="flex flex-col gap-6">
              {/* Header — requester + status */}
              <div className="flex items-center justify-between">
                <div className="flex flex-col gap-0.5">
                  <span className="font-semibold text-text">{correction.requester_id}</span>
                  <span className="font-mono text-xs text-text-3">{correction.attendance_id}</span>
                </div>
                <StatusBadge dot tone={correctionStatusTone(correction.status)}>
                  {t(`corrections.status.${correction.status}`)}
                </StatusBadge>
              </div>

              {/* Meta grid */}
              <div className="grid grid-cols-2 gap-4 rounded-xl border border-border bg-surface p-4 text-sm">
                <div className="flex flex-col gap-1">
                  <span className="text-xs text-text-3">{t('corrections.metaType')}</span>
                  <span className="font-medium text-text">
                    {correctionTypeLabel(correction.type, t)}
                  </span>
                </div>
                <div className="flex flex-col gap-1">
                  <span className="text-xs text-text-3">{t('corrections.metaSubmitted')}</span>
                  <DateText
                    kind="instant"
                    value={correction.created_at}
                    className="font-medium text-text"
                  />
                </div>
                {correction.decided_at && (
                  <div className="flex flex-col gap-1">
                    <span className="text-xs text-text-3">{t('corrections.metaDecided')}</span>
                    <DateText
                      kind="instant"
                      value={correction.decided_at as string}
                      className="font-medium text-text"
                    />
                  </div>
                )}
                {correction.decided_by && (
                  <div className="flex flex-col gap-1">
                    <span className="text-xs text-text-3">{t('corrections.metaDecidedBy')}</span>
                    <span className="font-medium text-text">{correction.decided_by as string}</span>
                  </div>
                )}
              </div>

              {/* Requester reason */}
              <div className="flex flex-col gap-2">
                <span className="text-sm font-medium text-text-2">
                  {t('corrections.requesterReason')}
                </span>
                <p className="rounded-lg border border-border-soft bg-surface p-3 text-sm text-text">
                  {correction.reason}
                </p>
              </div>

              {/* Before → After diff */}
              <div className="flex flex-col gap-2">
                <span className="text-sm font-medium text-text-2">
                  {t('corrections.diffLabel')}
                </span>
                <div className="overflow-x-auto rounded-xl border border-border bg-surface p-4">
                  <DiffTable correction={correction} />
                </div>
              </div>

              {/* Evidence file */}
              {correction.evidence_file_id && (
                <div className="flex items-center gap-2 rounded-lg border border-border-soft bg-surface p-3">
                  <FileText aria-hidden className="size-4 text-text-3" />
                  <span className="text-sm text-text-2">{t('corrections.evidenceFile')}</span>
                  <span className="font-mono text-xs text-text-3">
                    {correction.evidence_file_id as string}
                  </span>
                </div>
              )}

              {/* Reject reason */}
              {correction.reject_reason && (
                <div className="flex items-start gap-2 rounded-lg border border-error/20 bg-error/5 p-3">
                  <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0 text-error" />
                  <div className="flex flex-col gap-0.5">
                    <span className="text-sm font-medium text-error">
                      {t('corrections.rejectReasonLabel')}
                    </span>
                    <span className="text-sm text-text-2">{correction.reject_reason}</span>
                  </div>
                </div>
              )}

              {/* Applied notice */}
              {correction.status === CorrectionStatus.APPLIED && (
                <div className="flex items-center gap-2 rounded-lg border border-ok/20 bg-ok/5 p-3">
                  <CheckCircle2 aria-hidden className="size-4 text-ok" />
                  <span className="text-sm font-medium text-ok">
                    {t('corrections.appliedNote')}
                  </span>
                </div>
              )}

              {/* Pending decisions are made in the E11 Kotak Masuk — this surface is read-only. */}
              {isPending && (
                <div className="flex items-start gap-2 rounded-lg border border-border-soft bg-surface-2 p-3">
                  <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0 text-text-3" />
                  <span className="text-sm text-text-2">{t('corrections.decideInInbox')}</span>
                </div>
              )}
            </div>
          )}
        </DrawerBody>

        {/* Read-only footer: close + (optional) jump to the approval chain in the inbox. */}
        <DrawerFooter>
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            {t('corrections.close')}
          </Button>
          {correction?.approval_instance_id && onOpenInstance && (
            <Button
              type="button"
              variant="secondary"
              onClick={() => onOpenInstance(correction.approval_instance_id as string)}
            >
              <ExternalLink aria-hidden className="size-4" />
              {t('corrections.viewApproval')}
            </Button>
          )}
        </DrawerFooter>
      </Drawer>
    </>
  );
}
