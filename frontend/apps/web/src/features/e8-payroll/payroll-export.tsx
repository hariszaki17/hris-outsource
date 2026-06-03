/**
 * E8 · Payroll Export — thin trigger button + inline job-status card.
 *
 * .pen frame `i1uLk` · "Export Flow · State-cards (from RFJJj)" · width 600.
 * F8 / PA-5 · PA-7 · §8 D5 (Excel-only, PDF deferred) · confidential lock ON.
 *
 * Scope for E8:
 *  - `PayrollExportButton` — trigger button that calls `useExportPayslips`.
 *  - `ExportJobCard` — inline status card mapping QUEUED / RUNNING / DONE / FAILED.
 *
 * TODO (E10): The full reusable multi-step Export Modal family
 *   (comp/ModalExportStep1Format `PN3mn`, comp/ModalExportStep2Progress `Q3dllJ`,
 *   comp/ModalExportStep3Success `lJ2iU`, comp/ModalExportError `zOpT1`)
 *   is OWNED by E10 and has NOT been built yet. Once E10 ships those components,
 *   this file should be replaced: swap `PayrollExportButton` + `ExportJobCard` for
 *   a composition of the E10 modal family wired to `useExportPayslips`. The current
 *   inline card approach is intentionally minimal and self-contained.
 */

import { classifyError } from '@/lib/api-error.ts';
import {
  type PayslipExportJob,
  PayslipExportJobStatus,
  useExportPayslips,
} from '@swp/api-client/e8';
import { Button, useToast } from '@swp/ui';
import { AlertCircle, CheckCircle2, Download, Loader2, LockKeyhole, RefreshCw } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// ExportJobCard — inline status card for a queued/running/done/failed job
// ---------------------------------------------------------------------------

export interface ExportJobCardProps {
  job: PayslipExportJob;
  onRetry?: () => void;
  onDismiss?: () => void;
}

/**
 * ExportJobCard renders the result of a payroll export submission.
 * Maps the four job statuses from `.pen` i1uLk states to a compact inline card.
 *
 * TODO (E10): Replace with E10 modal family (PN3mn → Q3dllJ → lJ2iU / zOpT1).
 */
export function ExportJobCard({ job, onRetry, onDismiss }: ExportJobCardProps) {
  const { t } = useTranslation('payroll');

  type CardConfig = {
    icon: React.ReactNode;
    borderClass: string;
    bgClass: string;
    titleKey: string;
    descKey: string;
    showRetry: boolean;
  };

  const config: Record<PayslipExportJobStatus, CardConfig> = {
    [PayslipExportJobStatus.QUEUED]: {
      icon: <Loader2 className="size-4 shrink-0 animate-spin text-info-tx" aria-hidden="true" />,
      borderClass: 'border-info-bd',
      bgClass: 'bg-info-bg',
      titleKey: 'export.statusQueued',
      descKey: 'export.statusQueuedBody',
      showRetry: false,
    },
    [PayslipExportJobStatus.RUNNING]: {
      icon: <Loader2 className="size-4 shrink-0 animate-spin text-info-tx" aria-hidden="true" />,
      borderClass: 'border-info-bd',
      bgClass: 'bg-info-bg',
      titleKey: 'export.statusRunning',
      descKey: 'export.statusRunningBody',
      showRetry: false,
    },
    [PayslipExportJobStatus.DONE]: {
      icon: <CheckCircle2 className="size-4 shrink-0 text-ok-tx" aria-hidden="true" />,
      borderClass: 'border-ok-bd',
      bgClass: 'bg-ok-bg',
      titleKey: 'export.statusDone',
      descKey: 'export.statusDoneBody',
      showRetry: false,
    },
    [PayslipExportJobStatus.FAILED]: {
      icon: <AlertCircle className="size-4 shrink-0 text-bad-tx" aria-hidden="true" />,
      borderClass: 'border-bad-bd',
      bgClass: 'bg-bad-bg',
      titleKey: 'export.statusFailed',
      descKey: 'export.statusFailedBody',
      showRetry: true,
    },
  };

  const { icon, borderClass, bgClass, titleKey, descKey, showRetry } =
    config[job.status] ?? config[PayslipExportJobStatus.QUEUED];

  return (
    <div
      aria-live="polite"
      className={[
        'flex items-start gap-3 rounded-lg border px-3.5 py-3',
        borderClass,
        bgClass,
      ].join(' ')}
    >
      {icon}
      <div className="flex flex-1 flex-col gap-0.5">
        <p className="text-[13px] font-bold leading-snug text-text">{t(titleKey)}</p>
        <p className="text-[12px] leading-[1.4] text-text-2">{t(descKey, { jobId: job.id })}</p>
        {/* Confidential watermark notice */}
        <div className="mt-1.5 flex items-center gap-1 text-text-3">
          <LockKeyhole className="size-3 shrink-0" aria-hidden="true" />
          <span className="text-[11px] font-semibold">{t('export.confidentialLockNote')}</span>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        {showRetry && onRetry && (
          <button
            type="button"
            aria-label={t('export.retry')}
            onClick={onRetry}
            className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] text-bad-tx hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <RefreshCw className="size-3.5" aria-hidden="true" />
            {t('export.retry')}
          </button>
        )}
        {onDismiss && (
          <button
            type="button"
            aria-label={t('export.dismiss')}
            onClick={onDismiss}
            className="rounded-md px-2 py-1 text-[12px] text-text-3 hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {t('export.dismiss')}
          </button>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// PayrollExportButton — trigger + inline job state
// ---------------------------------------------------------------------------

export interface PayrollExportButtonProps {
  /**
   * Scope for the export. Either `period` (`YYYY-MM`) or `year` is required by the API.
   * Pass the current active filter from the archive screen.
   */
  period?: string;
  year?: number;
  /** Optional employee filter — when omitted, exports all visible payslips in scope. */
  employeeIds?: string[];
}

/**
 * PayrollExportButton — "Ekspor (RAHASIA)" trigger button with inline job-status card.
 *
 * Renders a single Button. On click, calls `useExportPayslips` with the given scope and
 * `confidential: true` (always enforced server-side per Wave 2.8 / OptConfidential lock).
 * The returned `PayslipExportJob` is shown inline via `ExportJobCard`.
 *
 * TODO (E10): Replace this component entirely with E10's `ExportModalTrigger` once the
 * multi-step modal family (PN3mn → Q3dllJ → lJ2iU / zOpT1) is built. This E8 implementation
 * is intentionally a thin in-place stand-in.
 *
 * Frame: `.pen` `i1uLk` · states QUEUED → RUNNING → DONE / FAILED.
 */
export function PayrollExportButton({ period, year, employeeIds }: PayrollExportButtonProps) {
  const { t } = useTranslation('payroll');
  const { toast } = useToast();

  const exportMutation = useExportPayslips();

  const [latestJob, setLatestJob] = useState<PayslipExportJob | null>(null);

  async function handleExport() {
    // Guard: need at least one scope dimension.
    if (!period && !year) {
      toast({
        tone: 'warn',
        title: t('export.scopeRequiredTitle'),
        description: t('export.scopeRequiredBody'),
      });
      return;
    }

    try {
      const res = await exportMutation.mutateAsync({
        data: {
          ...(period ? { period } : {}),
          ...(year ? { year } : {}),
          ...(employeeIds && employeeIds.length > 0 ? { employee_ids: employeeIds } : {}),
          format: 'XLSX',
          confidential: true,
        },
      });

      const job = res.data as PayslipExportJob;

      // Surface QUEUED toast so the user can close the area and still track the job.
      toast({
        tone: 'queued',
        title: t('export.queuedToastTitle'),
        description: t('export.queuedToastBody', { jobId: job.id }),
      });

      setLatestJob(job);
    } catch (err) {
      const { kind } = classifyError(err);

      if (kind === 'forbidden' || kind === 'unauthenticated') {
        toast({ tone: 'error', title: t('export.errorForbidden') });
      } else {
        toast({
          tone: 'error',
          title: t('export.errorTitle'),
          description: t('export.errorBody'),
        });
      }
    }
  }

  function handleRetry() {
    setLatestJob(null);
    void handleExport();
  }

  const isLoading = exportMutation.isPending;

  return (
    <div className="flex flex-col gap-2.5">
      {/* Primary trigger button */}
      <Button
        type="button"
        variant="secondary"
        disabled={isLoading}
        onClick={() => void handleExport()}
      >
        {isLoading ? (
          <Loader2 className="size-4 animate-spin" aria-hidden="true" />
        ) : (
          <Download className="size-4" aria-hidden="true" />
        )}
        {isLoading ? t('export.exporting') : t('export.triggerLabel')}
        {/* Confidential badge */}
        <span className="ml-1 flex items-center gap-1 rounded-full border border-warn-bd bg-warn-bg px-2 py-0.5 text-[10px] font-bold tracking-wide text-warn-tx">
          <LockKeyhole className="size-3" aria-hidden="true" />
          {t('export.confidentialBadge')}
        </span>
      </Button>

      {/* Inline job status card — only when a job has been queued */}
      {latestJob && (
        <ExportJobCard job={latestJob} onRetry={handleRetry} onDismiss={() => setLatestJob(null)} />
      )}
    </div>
  );
}
