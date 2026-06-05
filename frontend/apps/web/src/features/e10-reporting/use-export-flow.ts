/**
 * useExportFlow — generic export-job lifecycle hook for the ExportModal.
 *
 * .pen frames: EF8AZ (Ekspor button → opens modal), FJ6hX (Export modal screen).
 * Comp wiring: PN3mn → Q3dllJ → lJ2iU / zOpT1 (step1/2/3/error).
 *
 * Design decisions:
 *   - Uses generic `useCreateExport` + `useGetExport` + `useCancelExport` (not the
 *     report-specific `useExportBillableAttendanceReport`) so the same hook can be
 *     reused by any report screen that passes the correct ExportRequest.
 *   - Polling via `refetchInterval` while QUEUED or PROCESSING; interval disabled once
 *     terminal (COMPLETED / FAILED / CANCELLED). Interval: 2 000 ms.
 *   - `start()` fires the createExport mutation, stores the returned job id, then polling
 *     begins automatically through `useGetExport` with the stored id.
 *   - `exportStatusToStep` from e10-shared maps job status → ExportModal `step`.
 *
 * F10.3, F10.4 · EX-1..EX-6 · EPICS §8 D5 (XLSX only in v1).
 */

import {
  type ExportFormat,
  type ExportJob,
  type ExportRequest,
  ExportStatus,
  useCancelExport,
  useCreateExport,
  useGetExport,
} from '@swp/api-client/e10';
import type { ExportFilterChip, ExportModalProps, ExportStep } from '@swp/ui';
import { useCallback, useRef, useState } from 'react';
import { exportStatusToStep } from './e10-shared.tsx';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const POLL_INTERVAL_MS = 2_000;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function isTerminal(status: ExportStatus): boolean {
  return (
    status === ExportStatus.COMPLETED ||
    status === ExportStatus.FAILED ||
    status === ExportStatus.CANCELLED
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * Unwrap an ExportJob from a query/mutation result. Orval's customFetch wraps the HTTP
 * body in { data, status, headers } and the BE handler wraps the ExportJob in
 * { data: <ExportJob> } (dataResponse) even though the E10 openapi declares the bare
 * ExportJob. So the real job lives at result.data.data — peel both, with a bare fallback
 * (recurring {data}-unwrap finding; cf. [08-04]/[10-04]). `result` here is already the
 * customFetch envelope, so we read result.data (HTTP body) then its inner .data.
 */
function unwrapExportJob(result: unknown): ExportJob | undefined {
  const body = (result as { data?: { data?: ExportJob } | ExportJob } | undefined)?.data;
  return ((body as { data?: ExportJob } | undefined)?.data ?? (body as ExportJob | undefined)) as
    | ExportJob
    | undefined;
}

// ---------------------------------------------------------------------------
// Public API types
// ---------------------------------------------------------------------------

export interface ExportFlowStartOptions {
  request: ExportRequest;
  /** Period start ISO date string — shown in ExportModal rangeStart. */
  rangeStart?: string;
  /** Period end ISO date string — shown in ExportModal rangeEnd. */
  rangeEnd?: string;
  /** Chips summarising active filters shown in the format step. */
  filterChips?: ExportFilterChip[];
}

export interface ExportFlowResult {
  /** Whether the ExportModal is open. */
  open: boolean;
  /** Current modal step (format / progress / success / error). */
  step: ExportStep;
  /** Spread these onto `<ExportModal>` (excludes `open` — pass that separately). */
  modalProps: Omit<ExportModalProps, 'open'>;
  /** Kick off the export flow. Opens the modal at step=format, then on confirm calls POST /exports. */
  start: (opts: ExportFlowStartOptions) => void;
  /** Close the modal and reset state (safe to call at any step). */
  close: () => void;
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

/**
 * useExportFlow
 *
 * Manages the full ExportModal lifecycle: format → progress → success/error.
 * Returns `{ open, step, modalProps, start, close }`.
 *
 * Usage:
 * ```tsx
 * const ef = useExportFlow();
 * <Button type="button" onClick={() => ef.start({ request, rangeStart, rangeEnd, filterChips })}>
 *   Ekspor
 * </Button>
 * <ExportModal open={ef.open} {...ef.modalProps} />
 * ```
 */
export function useExportFlow(): ExportFlowResult {
  // ----------- UI state
  const [open, setOpen] = useState(false);
  const [step, setStep] = useState<ExportStep>('format');
  const [jobId, setJobId] = useState<string | null>(null);

  // ----------- Pending-format state (stored when user opens modal before firing)
  const pendingOpts = useRef<ExportFlowStartOptions | null>(null);

  // ----------- Mutations
  const createExport = useCreateExport({
    mutation: {
      onSuccess(res) {
        const job = unwrapExportJob(res);
        if (!job?.id) return;
        setJobId(job.id);
        setStep('progress');
      },
      onError() {
        setStep('error');
      },
    },
  });

  const cancelExport = useCancelExport();

  // ----------- Polling query — only active when we have a jobId and not yet terminal
  const pollQuery = useGetExport(jobId ?? '', {
    query: {
      enabled: Boolean(jobId),
      refetchInterval(query) {
        const job = unwrapExportJob(query.state.data);
        if (!job) return POLL_INTERVAL_MS;
        return isTerminal(job.status) ? false : POLL_INTERVAL_MS;
      },
    },
  });

  const job = unwrapExportJob(pollQuery.data);

  // Sync step from job status whenever polling resolves
  if (job && step !== 'format') {
    const derived = exportStatusToStep(job.status);
    if (derived !== step) {
      // Direct (non-render) mutation is intentional here — we're inside the render
      // but we only update if the value actually changed, preventing infinite loops.
      // This pattern is acceptable per the React docs for "derive during render".
      // (setStep would schedule a re-render — do it via the callback below instead.)
    }
  }

  // ---------------------------------------------------------------------------
  // Derive step from live job without triggering extra renders inside the body:
  // use a computed local that replaces `step` when job data is authoritative.
  // ---------------------------------------------------------------------------
  const effectiveStep: ExportStep =
    job && step !== 'format' ? exportStatusToStep(job.status) : step;

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  const start = useCallback((opts: ExportFlowStartOptions) => {
    pendingOpts.current = opts;
    setOpen(true);
    setStep('format');
    setJobId(null);
  }, []);

  const close = useCallback(() => {
    setOpen(false);
    setStep('format');
    setJobId(null);
    pendingOpts.current = null;
  }, []);

  const handleExport = useCallback(() => {
    const opts = pendingOpts.current;
    if (!opts) return;
    setStep('progress');
    createExport.mutate({ data: opts.request });
  }, [createExport]);

  const handleAbort = useCallback(() => {
    if (jobId) {
      cancelExport.mutate({ exportId: jobId });
    }
    close();
  }, [jobId, cancelExport, close]);

  const handleRetry = useCallback(() => {
    setStep('format');
    setJobId(null);
    // Re-open at format step so user can fire again
  }, []);

  const handleDownload = useCallback(() => {
    if (job?.file_url) {
      window.open(job.file_url, '_blank', 'noopener,noreferrer');
    }
  }, [job]);

  const handleCopyLink = useCallback(() => {
    if (job?.file_url) {
      void navigator.clipboard.writeText(job.file_url);
    }
  }, [job]);

  const handleCopyError = useCallback(() => {
    const code = job?.error?.code ?? 'UNKNOWN';
    void navigator.clipboard.writeText(code);
  }, [job]);

  // ---------------------------------------------------------------------------
  // Build ExportModal props from current state
  // ---------------------------------------------------------------------------

  const opts = pendingOpts.current;
  const format = opts?.request.format as ExportFormat | undefined;

  const fileInfo =
    job?.status === ExportStatus.COMPLETED && job.filename
      ? {
          name: job.filename,
          size: job.size_bytes != null ? formatBytes(job.size_bytes) : '—',
          rows: '—',
          format: format ?? 'EXCEL',
        }
      : undefined;

  const auditLine = job?.audit_log_entry_id
    ? `Audit: ${job.audit_log_entry_id} · ${job.requested_at}`
    : undefined;

  const modalProps: Omit<ExportModalProps, 'open'> = {
    onOpenChange: (v) => {
      if (!v) close();
    },
    step: effectiveStep,

    // Step 1 — format
    rangeStart: opts?.rangeStart,
    rangeEnd: opts?.rangeEnd,
    filterChips: opts?.filterChips ?? [],
    onExport: handleExport,
    exporting: createExport.isPending,

    // Step 2 — progress
    progressPct: job?.progress_percent ?? 0,
    progressLabel:
      job?.status === ExportStatus.QUEUED
        ? 'Menunggu antrean…'
        : job?.status === ExportStatus.PROCESSING
          ? `Memproses… ${job.progress_percent ?? 0}%`
          : undefined,
    onAbort: handleAbort,

    // Step 3 — success
    file: fileInfo,
    auditLine,
    onDownload: handleDownload,
    onCopyLink: handleCopyLink,

    // Error
    errorReason: job?.error?.message ?? undefined,
    errorCode: job?.error?.code ?? undefined,
    onRetry: handleRetry,
    onCopyError: handleCopyError,
  };

  return {
    open,
    step: effectiveStep,
    modalProps,
    start,
    close,
  };
}
