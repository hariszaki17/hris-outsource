import * as Dialog from '@radix-ui/react-dialog';
import {
  CircleCheck,
  Download,
  FileSpreadsheet,
  FileText,
  Info,
  Link as LinkIcon,
  Loader2,
  OctagonAlert,
  RotateCcw,
  Sheet,
  TriangleAlert,
  X,
} from 'lucide-react';
import { cn } from '../lib/cn.ts';
import { Button } from '../primitives/button.tsx';

/**
 * ExportModal family — maps to .pen `comp/ModalExportStep1Format` `PN3mn`,
 * `…Step2Progress` `Q3dllJ`, `…Step3Success` `lJ2iU`, `…Error` `zOpT1` (G3 — one canonical
 * multi-step modal, the step is a `step` prop). Owned by E10; instanced by every Ekspor button.
 *
 * XLSX-only in v1 (D5 / EPICS §8) — PDF is shown disabled. Copy carries Bahasa defaults baked in
 * (matching the .pen); override any label via `labels`. (Externalising to i18n is a follow-up.)
 */
export type ExportStep = 'format' | 'progress' | 'success' | 'error';
export type ExportQuickRange = '7d' | '30d' | 'month' | 'custom';

export interface ExportFilterChip {
  label: string;
  tone?: 'info' | 'ok' | 'neutral';
}

export interface ExportFileInfo {
  name: string;
  size: string;
  rows: string;
  format: string;
}

export interface ExportModalLabels {
  title: string;
  stepFormat: string;
  stepProgress: string;
  stepDone: string;
  pdfDeferredNote: string;
  formatLabel: string;
  optExcel: string;
  optExcelHint: string;
  optPdf: string;
  pdfSoon: string;
  rangeLabel: string;
  quick7d: string;
  quick30d: string;
  quickMonth: string;
  quickCustom: string;
  filterRecapLabel: string;
  confidentialLabel: string;
  confidentialHint: string;
  cancel: string;
  exportBtn: string;
  processingSubtitle: string;
  queueHint: string;
  abort: string;
  close: string;
  successSubtitle: string;
  successTitle: string;
  copyLink: string;
  download: string;
  errorSubtitle: string;
  errorTitle: string;
  errorTechnical: string;
  errorRoleNote: string;
  copyError: string;
  retry: string;
}

const DEFAULT_LABELS: ExportModalLabels = {
  title: 'Ekspor data',
  stepFormat: 'Format',
  stepProgress: 'Memproses',
  stepDone: 'Selesai',
  pdfDeferredNote: 'PDF ditunda (EPICS §8) — hanya Excel untuk v1.',
  formatLabel: 'Format',
  optExcel: 'Excel (.xlsx)',
  optExcelHint: 'Direkomendasikan untuk data tabular besar',
  optPdf: 'PDF (.pdf)',
  pdfSoon: 'Segera',
  rangeLabel: 'Periode',
  quick7d: '7 hari',
  quick30d: '30 hari',
  quickMonth: 'Bulan ini',
  quickCustom: 'Custom',
  filterRecapLabel: 'Filter aktif (mengikuti layar)',
  confidentialLabel: 'Tandai sebagai rahasia (RAHASIA)',
  confidentialHint: 'Watermark RAHASIA akan ditambahkan ke file.',
  cancel: 'Batal',
  exportBtn: 'Ekspor',
  processingSubtitle: 'Sedang memproses…',
  queueHint: 'Anda bisa menutup modal — kami akan beri tahu saat selesai.',
  abort: 'Batalkan ekspor',
  close: 'Tutup',
  successSubtitle: 'File siap diunduh',
  successTitle: 'Ekspor selesai',
  copyLink: 'Salin tautan',
  download: 'Unduh',
  errorSubtitle: 'Terjadi kesalahan',
  errorTitle: 'Ekspor gagal',
  errorTechnical: 'Detail teknis',
  errorRoleNote: 'HR / Super Admin only',
  copyError: 'Salin error',
  retry: 'Coba lagi',
};

const chipToneClass: Record<NonNullable<ExportFilterChip['tone']>, string> = {
  info: 'bg-info-bg text-info-tx border-info-bd',
  ok: 'bg-ok-bg text-ok-tx border-ok-bd',
  neutral: 'bg-surface text-text-2 border-border',
};

export interface ExportModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  step: ExportStep;
  labels?: Partial<ExportModalLabels>;

  // Step 1 — format
  rangeStart?: string;
  rangeEnd?: string;
  quickRange?: ExportQuickRange;
  onQuickRangeChange?: (r: ExportQuickRange) => void;
  filterChips?: ExportFilterChip[];
  /** When set, renders the amber size-warning banner. */
  sizeWarn?: { estimate: string; hint: string };
  onExport?: () => void;
  /** Disables the Ekspor button + shows it busy. */
  exporting?: boolean;

  // Step 2 — progress
  progressPct?: number;
  progressLabel?: string;
  onAbort?: () => void;

  // Step 3 — success
  file?: ExportFileInfo;
  auditLine?: string;
  onDownload?: () => void;
  onCopyLink?: () => void;

  // Error
  errorReason?: string;
  /** Raw technical code — only render for HR/Super Admin callers. */
  errorCode?: string;
  onRetry?: () => void;
  onCopyError?: () => void;
}

type StepState = 'todo' | 'active' | 'done';

function stepStates(step: ExportStep): [StepState, StepState, StepState] {
  switch (step) {
    case 'format':
      return ['active', 'todo', 'todo'];
    case 'progress':
      return ['done', 'active', 'todo'];
    case 'success':
      return ['done', 'done', 'active'];
    case 'error':
      return ['done', 'active', 'todo'];
  }
}

function StepperNode({ state, n, label }: { state: StepState; n: number; label: string }) {
  return (
    <div
      className={cn(
        'flex items-center gap-1.5 rounded-full border px-2.5 py-1',
        state === 'active' ? 'border-primary bg-primary-soft' : 'border-border bg-surface-2',
      )}
    >
      <span
        className={cn(
          'flex size-[18px] items-center justify-center rounded-full text-[11px] font-bold',
          state === 'active' && 'bg-primary text-white',
          state === 'done' && 'bg-ok-bg text-ok-tx',
          state === 'todo' && 'border border-border bg-surface text-text-3',
        )}
      >
        {state === 'done' ? <CircleCheck className="size-3" aria-hidden="true" /> : n}
      </span>
      <span
        className={cn(
          'text-[12px] font-semibold',
          state === 'active' ? 'text-primary-strong' : 'text-text-3',
        )}
      >
        {label}
      </span>
    </div>
  );
}

export function ExportModal(props: ExportModalProps) {
  const { open, onOpenChange, step } = props;
  const L = { ...DEFAULT_LABELS, ...props.labels };
  const [s1, s2, s3] = stepStates(step);

  const subtitle =
    step === 'progress'
      ? L.processingSubtitle
      : step === 'success'
        ? L.successSubtitle
        : step === 'error'
          ? L.errorSubtitle
          : undefined;

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-scrim" />
        <Dialog.Content
          className={cn(
            'fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2',
            'flex max-h-[calc(100vh-2rem)] w-[520px] max-w-[calc(100vw-2rem)] flex-col overflow-hidden',
            'rounded-[14px] border border-border bg-surface shadow-overlay',
          )}
        >
          {/* Header */}
          <div className="flex items-center justify-between gap-3 border-b border-border-soft px-5 py-4">
            <div className="flex flex-col gap-0.5">
              <Dialog.Title className="text-[16px] font-bold text-text">{L.title}</Dialog.Title>
              {subtitle ? (
                <Dialog.Description
                  className={cn(
                    'text-[12px]',
                    step === 'success'
                      ? 'font-semibold text-ok-tx'
                      : step === 'error'
                        ? 'font-semibold text-bad-tx'
                        : 'text-text-3',
                  )}
                >
                  {subtitle}
                </Dialog.Description>
              ) : (
                <Dialog.Description className="text-[12px] text-text-3">
                  Pilih format dan opsi
                </Dialog.Description>
              )}
            </div>
            <Dialog.Close
              className="flex size-[30px] items-center justify-center rounded-md bg-surface-2 text-text-2 hover:bg-surface"
              aria-label={L.close}
            >
              <X className="size-4" aria-hidden="true" />
            </Dialog.Close>
          </div>

          {/* Stepper */}
          <div className="flex items-center gap-2 px-5 pt-4">
            <StepperNode state={s1} n={1} label={L.stepFormat} />
            <span className="h-px w-4 bg-border" aria-hidden="true" />
            <StepperNode state={s2} n={2} label={L.stepProgress} />
            <span className="h-px w-4 bg-border" aria-hidden="true" />
            <StepperNode state={s3} n={3} label={L.stepDone} />
          </div>

          {/* Body + Footer per step */}
          {step === 'format' && <FormatStep {...props} L={L} />}
          {step === 'progress' && <ProgressStep {...props} L={L} />}
          {step === 'success' && <SuccessStep {...props} L={L} />}
          {step === 'error' && <ErrorStep {...props} L={L} />}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

type StepProps = ExportModalProps & { L: ExportModalLabels };

function FormatStep({
  rangeStart,
  rangeEnd,
  quickRange = '30d',
  onQuickRangeChange,
  filterChips = [],
  sizeWarn,
  onExport,
  exporting,
  onOpenChange,
  L,
}: StepProps) {
  const quickOptions: { key: ExportQuickRange; label: string }[] = [
    { key: '7d', label: L.quick7d },
    { key: '30d', label: L.quick30d },
    { key: 'month', label: L.quickMonth },
    { key: 'custom', label: L.quickCustom },
  ];
  return (
    <>
      <div className="flex flex-col gap-[18px] overflow-y-auto px-5 py-5">
        {/* PDF deferred note */}
        <div className="flex items-center gap-2 rounded-md border border-warn-bd bg-warn-bg px-2.5 py-2">
          <Info className="size-3.5 shrink-0 text-warn-tx" aria-hidden="true" />
          <span className="text-[11px] font-semibold text-warn-tx">{L.pdfDeferredNote}</span>
        </div>

        {/* Format options */}
        <div className="flex flex-col gap-2">
          <span className="text-[12px] font-semibold text-text-2">{L.formatLabel}</span>
          <div className="flex items-center gap-3 rounded-[10px] border-[1.5px] border-primary bg-primary-soft px-3.5 py-3">
            <span className="flex size-[18px] items-center justify-center rounded-full border-2 border-primary bg-surface">
              <span className="size-2 rounded-full bg-primary" aria-hidden="true" />
            </span>
            <span className="flex size-8 items-center justify-center rounded-lg bg-surface text-primary">
              <Sheet className="size-[18px]" aria-hidden="true" />
            </span>
            <span className="flex flex-col">
              <span className="text-[13px] font-bold text-text">{L.optExcel}</span>
              <span className="text-[11px] text-text-2">{L.optExcelHint}</span>
            </span>
          </div>
          <div className="flex items-center gap-3 rounded-[10px] border border-border bg-surface-2 px-3.5 py-3 opacity-70">
            <span className="size-[18px] rounded-full border-[1.5px] border-text-3 bg-surface" />
            <span className="flex size-8 items-center justify-center rounded-lg bg-surface text-text-3">
              <FileText className="size-[18px]" aria-hidden="true" />
            </span>
            <span className="flex flex-1 items-center justify-between gap-2">
              <span className="text-[13px] font-bold text-text-3">{L.optPdf}</span>
              <span className="rounded-full border border-info-bd bg-info-bg px-1.5 py-0.5 text-[10px] font-semibold text-info-tx">
                {L.pdfSoon}
              </span>
            </span>
          </div>
        </div>

        {/* Range + quick pick */}
        <div className="flex flex-col gap-2">
          <span className="text-[12px] font-semibold text-text-2">{L.rangeLabel}</span>
          <div className="flex items-center gap-2 text-[13px] text-text-2">
            <span className="flex-1 rounded-md border border-border bg-surface px-3 py-2">
              {rangeStart ?? '—'}
            </span>
            <span className="text-text-3">–</span>
            <span className="flex-1 rounded-md border border-border bg-surface px-3 py-2">
              {rangeEnd ?? '—'}
            </span>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {quickOptions.map((opt) => {
              const active = opt.key === quickRange;
              return (
                <button
                  key={opt.key}
                  type="button"
                  onClick={() => onQuickRangeChange?.(opt.key)}
                  className={cn(
                    'rounded-full border px-2.5 py-1 text-[11px] font-semibold',
                    active
                      ? 'border-primary bg-primary-soft text-primary-strong'
                      : 'border-border bg-surface-2 text-text-2 hover:bg-surface',
                  )}
                >
                  {opt.label}
                </button>
              );
            })}
          </div>
        </div>

        {/* Filter recap strip */}
        {filterChips.length > 0 && (
          <div className="flex flex-col gap-1.5 rounded-lg border border-border-soft bg-surface-2 px-3 py-2.5">
            <span className="text-[10px] font-semibold text-text-3">{L.filterRecapLabel}</span>
            <div className="flex flex-wrap gap-1.5">
              {filterChips.map((chip) => (
                <span
                  key={chip.label}
                  className={cn(
                    'rounded-full border px-2 py-0.5 text-[11px] font-semibold',
                    chipToneClass[chip.tone ?? 'neutral'],
                  )}
                >
                  {chip.label}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Size-warning banner */}
        {sizeWarn && (
          <div className="flex items-center gap-2.5 rounded-lg border border-warn-bd bg-warn-bg px-3 py-2.5">
            <TriangleAlert className="size-4 shrink-0 text-warn-tx" aria-hidden="true" />
            <span className="flex flex-col">
              <span className="text-[12px] font-bold text-warn-tx">{sizeWarn.estimate}</span>
              <span className="text-[11px] text-text-2">{sizeWarn.hint}</span>
            </span>
          </div>
        )}
      </div>
      <div className="flex justify-end gap-2.5 border-t border-border-soft bg-surface-2 px-5 py-3.5">
        <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
          {L.cancel}
        </Button>
        <Button type="button" onClick={onExport} disabled={exporting}>
          <Download className="size-4" aria-hidden="true" />
          {L.exportBtn}
        </Button>
      </div>
    </>
  );
}

function ProgressStep({ progressPct = 0, progressLabel, onAbort, onOpenChange, L }: StepProps) {
  const pct = Math.max(0, Math.min(100, progressPct));
  return (
    <>
      <div className="flex flex-col gap-5 px-5 py-6">
        <div className="flex justify-center">
          <span className="flex size-[42px] items-center justify-center rounded-full bg-primary-soft text-primary">
            <Loader2 className="size-6 animate-spin" aria-hidden="true" />
          </span>
        </div>
        <div className="h-2 w-full overflow-hidden rounded-full border border-border-soft bg-surface-2">
          <div className="h-full rounded-full bg-primary" style={{ width: `${pct}%` }} />
        </div>
        <div className="flex items-center justify-between gap-2">
          <span className="text-[13px] text-text-2">{progressLabel}</span>
          <span className="font-mono text-[13px] font-bold text-primary-strong">{pct}%</span>
        </div>
        <div className="flex items-center gap-2 rounded-lg border border-info-bd bg-info-bg px-3 py-2.5">
          <Info className="size-3.5 shrink-0 text-info-tx" aria-hidden="true" />
          <span className="text-[12px] font-semibold text-info-tx">{L.queueHint}</span>
        </div>
      </div>
      <div className="flex justify-between border-t border-border-soft bg-surface-2 px-5 py-3.5">
        <Button type="button" variant="ghost" onClick={onAbort}>
          {L.abort}
        </Button>
        <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
          {L.close}
        </Button>
      </div>
    </>
  );
}

function SuccessStep({ file, auditLine, onDownload, onCopyLink, onOpenChange, L }: StepProps) {
  return (
    <>
      <div className="flex flex-col gap-[18px] px-5 py-5">
        <div className="flex flex-col items-center gap-2 pt-2">
          <span className="flex size-14 items-center justify-center rounded-full border border-ok-bd bg-ok-bg text-ok-tx">
            <CircleCheck className="size-[30px]" aria-hidden="true" />
          </span>
          <span className="text-[18px] font-bold text-text">{L.successTitle}</span>
        </div>
        {file && (
          <div className="flex items-center gap-3.5 rounded-[10px] border border-border-soft bg-surface-2 px-3.5 py-3">
            <span className="flex size-10 items-center justify-center rounded-lg border border-border bg-surface text-primary">
              <FileSpreadsheet className="size-[22px]" aria-hidden="true" />
            </span>
            <span className="flex min-w-0 flex-col gap-0.5">
              <span className="truncate font-mono text-[13px] font-semibold text-text">
                {file.name}
              </span>
              <span className="text-[11px] font-medium text-text-2">
                {file.size} · {file.rows} · {file.format}
              </span>
            </span>
          </div>
        )}
        {auditLine && <span className="text-[11px] font-medium text-text-3">{auditLine}</span>}
      </div>
      <div className="flex justify-between border-t border-border-soft bg-surface-2 px-5 py-3.5">
        <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
          {L.close}
        </Button>
        <div className="flex gap-2">
          <Button type="button" variant="secondary" onClick={onCopyLink}>
            <LinkIcon className="size-4" aria-hidden="true" />
            {L.copyLink}
          </Button>
          <Button type="button" onClick={onDownload}>
            <Download className="size-4" aria-hidden="true" />
            {L.download}
          </Button>
        </div>
      </div>
    </>
  );
}

function ErrorStep({ errorReason, errorCode, onRetry, onCopyError, onOpenChange, L }: StepProps) {
  return (
    <>
      <div className="flex flex-col gap-4 px-5 py-5">
        <div className="flex flex-col items-center gap-2 pt-2">
          <span className="flex size-14 items-center justify-center rounded-full border border-bad-bd bg-bad-bg text-bad-tx">
            <OctagonAlert className="size-[30px]" aria-hidden="true" />
          </span>
          <span className="text-[18px] font-bold text-text">{L.errorTitle}</span>
        </div>
        {errorReason && (
          <p className="text-center text-[13px] leading-relaxed text-text-2">{errorReason}</p>
        )}
        {errorCode && (
          <div className="flex flex-col gap-1.5 rounded-lg border border-border-soft bg-surface-2 px-3 py-2.5">
            <div className="flex items-center justify-between">
              <span className="text-[10px] font-semibold text-text-3">{L.errorTechnical}</span>
              <span className="font-mono text-[9px] font-medium text-text-3">
                {L.errorRoleNote}
              </span>
            </div>
            <code className="font-mono text-[11px] leading-relaxed text-text-2">{errorCode}</code>
          </div>
        )}
      </div>
      <div className="flex justify-between border-t border-border-soft bg-surface-2 px-5 py-3.5">
        <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
          {L.close}
        </Button>
        <div className="flex gap-2">
          <Button type="button" variant="secondary" onClick={onCopyError}>
            {L.copyError}
          </Button>
          <Button type="button" onClick={onRetry}>
            <RotateCcw className="size-4" aria-hidden="true" />
            {L.retry}
          </Button>
        </div>
      </div>
    </>
  );
}
