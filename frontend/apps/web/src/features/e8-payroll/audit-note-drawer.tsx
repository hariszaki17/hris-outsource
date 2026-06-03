/**
 * E8 · HR Audit-Note Drawer — append-only annotation panel for a single payslip.
 *
 * .pen frame `BDHMZ` · "HR Audit-Note · Drawer & states" · width 480.
 * F8 / PA-7 · §8 decision "immutable; HR annotates via audited note, not edits".
 *
 * States covered: loading · empty (no notes yet) · error+retry · saving · default (list + form).
 * Notes are append-only — no edit/delete affordances are rendered.
 * The drawer does NOT mount unless `open && !!payslipId`.
 */

import { classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type PayslipAuditNote,
  type PayslipAuditNoteListResponse,
  useCreatePayslipAuditNote,
  useListPayslipAuditNotes,
} from '@swp/api-client/e8';
import {
  Button,
  DateText,
  Drawer,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  Skeleton,
  StateView,
  useToast,
} from '@swp/ui';
import { Lock, ShieldCheck } from 'lucide-react';
import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Zod schema — mirrors the design's "Minimal 8 karakter · maksimal 1000" hint
// and the API model @minLength 1 @maxLength 4000. We use 8 min per the .pen
// helper text; max 1000 per the .pen counter "0 / 1000".
// ---------------------------------------------------------------------------

const auditNoteSchema = z.object({
  text: z
    .string()
    .min(8, 'auditNotes.validation.minLength')
    .max(1000, 'auditNotes.validation.maxLength')
    .refine((v) => v.trim().length > 0, 'auditNotes.validation.required'),
});
type AuditNoteFormValues = z.infer<typeof auditNoteSchema>;

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

/** Single note item — author + timestamp + text. */
function NoteItem({ note }: { note: PayslipAuditNote }) {
  const { t } = useTranslation('payroll');
  const displayName = note.author_name ?? note.author_id;

  return (
    <div className="flex flex-col gap-1.5 rounded-lg border border-border-soft bg-surface-2 px-3.5 py-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-[13px] font-semibold text-text">{displayName}</span>
        <DateText
          kind="instant"
          value={note.created_at}
          className="shrink-0 font-mono text-[11px] text-text-3"
        />
      </div>
      <p className="whitespace-pre-wrap text-[13px] leading-[1.5] text-text-2">{note.text}</p>
      {/* Append-only lock chip — no edit/delete controls. */}
      <div className="flex items-center gap-1 text-text-3">
        <Lock className="size-3 shrink-0" aria-hidden="true" />
        <span className="text-[11px]">{t('auditNotes.immutableNote')}</span>
      </div>
    </div>
  );
}

/** Skeleton placeholder for the list loading state. */
function NoteListSkeleton() {
  return (
    <div className="flex flex-col gap-2.5" aria-hidden="true">
      {[0, 1, 2].map((i) => (
        <div
          key={i}
          className="flex flex-col gap-2 rounded-lg border border-border-soft bg-surface-2 px-3.5 py-3"
        >
          <div className="flex items-center justify-between gap-2">
            <Skeleton className="h-3 w-28" />
            <Skeleton className="h-2.5 w-24" />
          </div>
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-3/4" />
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Append-note form
// ---------------------------------------------------------------------------

interface AppendNoteFormProps {
  payslipId: string;
  onSuccess: () => void;
}

function AppendNoteForm({ payslipId, onSuccess }: AppendNoteFormProps) {
  const { t } = useTranslation('payroll');
  const { toast } = useToast();
  const createMutation = useCreatePayslipAuditNote();

  const {
    register,
    handleSubmit,
    watch,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<AuditNoteFormValues>({
    resolver: zodResolver(auditNoteSchema),
    defaultValues: { text: '' },
  });

  const textValue = watch('text');
  const charCount = textValue.length;

  async function onSubmit(values: AuditNoteFormValues) {
    try {
      await createMutation.mutateAsync({ id: payslipId, data: { text: values.text.trim() } });
      toast({
        tone: 'success',
        title: t('auditNotes.saveSuccess'),
        description: t('auditNotes.saveSuccessBody'),
      });
      reset();
      onSuccess();
    } catch (err) {
      const { kind } = classifyError(err);
      if (kind === 'forbidden' || kind === 'unauthenticated') {
        toast({ tone: 'error', title: t('auditNotes.errorForbidden') });
      } else {
        toast({ tone: 'error', title: t('auditNotes.errorGeneric') });
      }
    }
  }

  const textareaId = 'audit-note-text';
  const isDisabled = isSubmitting;

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate>
      {/* Section heading */}
      <div className="mb-3 flex items-center gap-1.5">
        <ShieldCheck className="size-3.5 shrink-0 text-ok-tx" aria-hidden="true" />
        <span className="text-[11px] font-bold tracking-wide text-text-3 uppercase">
          {t('auditNotes.addSectionTitle')}
        </span>
      </div>

      {/* Immutable notice — mirrors .pen ImmutableNotice (JYJSY) */}
      <div className="mb-3 flex items-center gap-2 rounded-lg border border-bad-bd bg-bad-bg px-3 py-2.5">
        <Lock className="size-3.5 shrink-0 text-bad-tx" aria-hidden="true" />
        <p className="text-[11px] font-semibold leading-[1.4] text-bad-tx">
          {t('auditNotes.immutableBanner')}
        </p>
      </div>

      {/* Textarea */}
      <div className="flex flex-col gap-1.5">
        <label htmlFor={textareaId} className="text-[12px] font-semibold text-text-2">
          {t('auditNotes.textLabel')}
          <span aria-hidden className="ml-0.5 text-error">
            *
          </span>
        </label>
        <textarea
          id={textareaId}
          {...register('text')}
          rows={5}
          disabled={isDisabled}
          aria-describedby={errors.text ? `${textareaId}-error` : `${textareaId}-hint`}
          aria-invalid={Boolean(errors.text)}
          placeholder={t('auditNotes.textPlaceholder')}
          className={[
            'w-full resize-none rounded-lg border px-3 py-2.5 text-[13px] leading-[1.5] text-text outline-none',
            'border-border bg-surface placeholder:text-text-3',
            'focus:border-primary focus:ring-1 focus:ring-primary',
            'disabled:cursor-not-allowed disabled:opacity-60',
            errors.text ? 'border-error focus:border-error focus:ring-error' : '',
          ]
            .filter(Boolean)
            .join(' ')}
        />
        {/* Helper row: validation hint + char counter */}
        <div className="flex items-center justify-between">
          {errors.text?.message ? (
            <p id={`${textareaId}-error`} role="alert" className="text-[11px] text-error">
              {t(errors.text.message)}
            </p>
          ) : (
            <p id={`${textareaId}-hint`} className="text-[11px] text-text-3">
              {t('auditNotes.textHint')}
            </p>
          )}
          <span
            className={[
              'font-mono text-[11px]',
              charCount > 1000 ? 'text-error' : 'text-text-3',
            ].join(' ')}
          >
            {charCount} / 1000
          </span>
        </div>
      </div>

      {/* Audit-log preview — mirrors .pen AuditPreview (e7Rrd) */}
      <div className="mt-3 flex flex-col gap-1.5 rounded-lg border border-border-soft bg-surface-2 px-3 py-2.5">
        <div className="flex items-center gap-1.5">
          <ShieldCheck className="size-3 shrink-0 text-ok-tx" aria-hidden="true" />
          <span className="text-[10px] font-bold tracking-[0.4px] text-text-3 uppercase">
            {t('auditNotes.auditPreviewLabel')}
          </span>
        </div>
        <p className="font-mono text-[11px] leading-[1.5] text-text-2">
          {t('auditNotes.auditPreviewLine', { payslipId })}
        </p>
      </div>

      {/* Submit row — sits inside the form, footer button references this */}
      <div className="mt-4 flex justify-end">
        <Button type="submit" disabled={isDisabled}>
          {isSubmitting ? t('auditNotes.saving') : t('auditNotes.submit')}
        </Button>
      </div>
    </form>
  );
}

// ---------------------------------------------------------------------------
// Main export
// ---------------------------------------------------------------------------

export interface AuditNoteDrawerProps {
  /** Payslip to annotate — `SWP-PS-{n}`. */
  payslipId: string;
  /** Payslip display label shown in the drawer subtitle (e.g. "Budi Santoso · Slip Mei 2026"). */
  payslipLabel?: string;
  open: boolean;
  onClose: () => void;
}

/**
 * AuditNoteDrawer — append-only HR annotation panel.
 * Frame: `.pen` `BDHMZ` · width 480.
 *
 * Opens as a right-side sheet. Fetches existing notes on mount.
 * Append-note form below; submits via `useCreatePayslipAuditNote`.
 */
export function AuditNoteDrawer({ payslipId, payslipLabel, open, onClose }: AuditNoteDrawerProps) {
  const { t } = useTranslation('payroll');

  const query = useListPayslipAuditNotes(
    payslipId,
    {},
    { query: { enabled: open && Boolean(payslipId) } },
  );

  // Re-fetch on open so the list is fresh each time. `query.refetch` is stable (TanStack Query).
  const refetch = query.refetch;
  useEffect(() => {
    if (open && payslipId) {
      void refetch();
    }
  }, [open, payslipId, refetch]);

  const page = query.data?.data as PayslipAuditNoteListResponse | undefined;
  const notes = page?.data ?? [];

  const headerSubtitle = payslipLabel ? `${t('auditNotes.forLabel')} ${payslipLabel}` : payslipId;

  return (
    <Drawer open={open} onOpenChange={(v) => !v && onClose()} width={480}>
      <DrawerHeader
        title={t('auditNotes.drawerTitle')}
        subtitle={headerSubtitle}
        closeLabel={t('auditNotes.close')}
        onClose={onClose}
      />

      <DrawerBody>
        {/* ---- Loading ---- */}
        {query.isLoading && <NoteListSkeleton />}

        {/* ---- Error ---- */}
        {query.isError &&
          (() => {
            const { kind } = classifyError(query.error);
            if (kind === 'forbidden' || kind === 'unauthenticated') {
              return (
                <StateView
                  kind="no-permission"
                  title={t('auditNotes.errorForbidden')}
                  description={t('auditNotes.errorForbiddenBody')}
                />
              );
            }
            return (
              <StateView
                kind="error"
                title={t('auditNotes.errorTitle')}
                onRetry={() => query.refetch()}
                retryLabel={t('auditNotes.retry')}
              />
            );
          })()}

        {/* ---- Notes list (empty or populated) ---- */}
        {!query.isLoading && !query.isError && (
          <>
            {/* List section header */}
            <div className="mb-1 flex items-center gap-1.5">
              <span className="text-[11px] font-bold tracking-wide text-text-3 uppercase">
                {t('auditNotes.listSectionTitle')}
              </span>
              {notes.length > 0 && (
                <span className="rounded-full bg-surface-2 px-2 py-0.5 font-mono text-[11px] font-medium text-text-3">
                  {notes.length}
                </span>
              )}
            </div>

            {notes.length === 0 ? (
              /* Empty: no notes yet */
              <div className="flex flex-col items-center justify-center gap-2 rounded-lg border border-border-soft bg-surface px-6 py-8 text-center">
                <ShieldCheck className="size-8 text-text-3" aria-hidden="true" />
                <p className="text-[13px] font-semibold text-text">{t('auditNotes.emptyTitle')}</p>
                <p className="text-[12px] text-text-3">{t('auditNotes.emptyBody')}</p>
              </div>
            ) : (
              /* Populated: chronological note list */
              <div className="flex flex-col gap-2.5">
                {notes.map((note) => (
                  <NoteItem key={note.id} note={note} />
                ))}
              </div>
            )}

            {/* Divider */}
            <div className="border-t border-border-soft" aria-hidden="true" />

            {/* Append-note form */}
            <AppendNoteForm payslipId={payslipId} onSuccess={() => void query.refetch()} />
          </>
        )}
      </DrawerBody>

      <DrawerFooter>
        {/* Left — immutable lock caption */}
        <div className="mr-auto flex items-center gap-1.5 text-text-3">
          <Lock className="size-3.5 shrink-0" aria-hidden="true" />
          <span className="text-[12px]">{t('auditNotes.appendOnlyFooter')}</span>
        </div>
        <Button type="button" variant="secondary" onClick={onClose}>
          {t('auditNotes.close')}
        </Button>
      </DrawerFooter>
    </Drawer>
  );
}
