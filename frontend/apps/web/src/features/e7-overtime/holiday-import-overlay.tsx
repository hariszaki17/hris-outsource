/**
 * E7 · Impor Hari Libur — yearly bootstrap import (EPICS §8 D4).
 *
 * Prefills public-holiday candidates for a year from the offline `date-holidays`
 * library (computes movable dates: Idul Fitri/Adha, Nyepi, Imlek, Waisak…), dedupes
 * against the existing `/holidays` calendar (recurring holidays project to the year,
 * so re-importing skips them), and lets HR review/edit before a bulk create.
 *
 * Deliberately NOT a live sync: HR confirms, and the SKB 3 Menteri decree stays the
 * authoritative source for cuti bersama (the auto source lags) — surfaced as a note.
 * The library is lazy-imported so it stays out of the main bundle.
 */
import { classifyError } from '@/lib/api-error.ts';
import {
  HolidayCategory,
  type HolidayPage,
  type HolidayWriteRequest,
  useCreateHoliday,
  useListHolidays,
} from '@swp/api-client/e7';
import { TZ } from '@swp/shared';
import {
  Banner,
  Button,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  StateView,
  useToast,
} from '@swp/ui';
import { CalendarPlus, ChevronLeft, ChevronRight, Info } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types + helpers
// ---------------------------------------------------------------------------

type Candidate = {
  id: string; // `${date}|${name}` — stable selection key
  date: string; // YYYY-MM-DD
  name: string;
  recurring: boolean;
};

/** Current calendar year in Asia/Jakarta. */
function currentJakartaYear(): number {
  return Number(new Date().toLocaleDateString('en-CA', { timeZone: TZ }).slice(0, 4));
}

/** Load public-holiday candidates for a year from the lazy `date-holidays` lib. */
async function loadCandidates(year: number): Promise<Candidate[]> {
  const mod = await import('date-holidays');
  const Holidays = mod.default;
  const hd = new Holidays('ID');
  return hd
    .getHolidays(year)
    .filter((h) => h.type === 'public')
    .map((h) => {
      const date = h.date.slice(0, 10);
      // Fixed MM-DD rule → recurring (projects across years); movable → per-year.
      const recurring = /^\d{2}-\d{2}$/.test(h.rule);
      return { id: `${date}|${h.name}`, date, name: h.name, recurring };
    })
    .sort((a, b) => a.date.localeCompare(b.date));
}

type ImportResult = { created: number; skipped: number; failed: number };

// ---------------------------------------------------------------------------
// Modal
// ---------------------------------------------------------------------------

export interface HolidayImportModalProps {
  open: boolean;
  onClose: () => void;
  onImported: () => void;
}

export function HolidayImportModal({ open, onClose, onImported }: HolidayImportModalProps) {
  const { t } = useTranslation('overtime');
  const { toast } = useToast();
  const create = useCreateHoliday();

  const [year, setYear] = useState<number>(currentJakartaYear);
  const [candidates, setCandidates] = useState<Candidate[] | null>(null);
  const [libError, setLibError] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [nameOverrides, setNameOverrides] = useState<Record<string, string>>({});
  const [importing, setImporting] = useState(false);
  const [progress, setProgress] = useState(0);
  const [result, setResult] = useState<ImportResult | null>(null);

  // Existing holidays for the selected year (recurring ones project to this year's date).
  const existingQuery = useListHolidays({ year }, { query: { enabled: open } });
  const existingDates = useMemo(() => {
    const page = existingQuery.data?.data as HolidayPage | undefined;
    return new Set((page?.data ?? []).map((h) => h.date));
  }, [existingQuery.data]);

  // Load candidates whenever the modal opens or the year changes.
  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setCandidates(null);
    setLibError(false);
    setResult(null);
    setProgress(0);
    loadCandidates(year)
      .then((list) => {
        if (!cancelled) setCandidates(list);
      })
      .catch(() => {
        if (!cancelled) setLibError(true);
      });
    return () => {
      cancelled = true;
    };
  }, [open, year]);

  // Default selection = every candidate not already on the calendar.
  // biome-ignore lint/correctness/useExhaustiveDependencies: re-seed only when data changes, not on every selection edit
  useEffect(() => {
    if (!candidates || importing || result) return;
    setSelected(new Set(candidates.filter((c) => !existingDates.has(c.date)).map((c) => c.id)));
  }, [candidates, existingDates]);

  // Reset transient year on close so the next open starts fresh-ish but keeps the year.
  const handleClose = useCallback(() => {
    if (importing) return;
    onClose();
  }, [importing, onClose]);

  const importable = useMemo(
    () => (candidates ?? []).filter((c) => !existingDates.has(c.date)),
    [candidates, existingDates],
  );
  const selectedImportable = useMemo(
    () => importable.filter((c) => selected.has(c.id)),
    [importable, selected],
  );

  const toggle = (c: Candidate) => {
    if (existingDates.has(c.date)) return;
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(c.id)) next.delete(c.id);
      else next.add(c.id);
      return next;
    });
  };

  const allSelected = importable.length > 0 && selectedImportable.length === importable.length;
  const toggleAll = () => {
    setSelected(allSelected ? new Set() : new Set(importable.map((c) => c.id)));
  };

  const runImport = async () => {
    if (selectedImportable.length === 0) return;
    setImporting(true);
    setProgress(0);
    let created = 0;
    let skipped = 0;
    let failed = 0;
    for (const c of selectedImportable) {
      const body: HolidayWriteRequest = {
        name: (nameOverrides[c.id] ?? c.name).trim() || c.name,
        date: c.date,
        category: HolidayCategory.NATIONAL,
        recurring: c.recurring,
      };
      try {
        await create.mutateAsync({ data: body });
        created += 1;
      } catch (err) {
        // Duplicate (same date already present) → skip; anything else → failure.
        if (classifyError(err).kind === 'conflict') skipped += 1;
        else failed += 1;
      }
      setProgress((p) => p + 1);
    }
    setImporting(false);
    setResult({ created, skipped, failed });
    onImported();
    toast({
      tone: failed > 0 ? 'warn' : 'success',
      title: t('holidays.import.toastDone', { created, skipped, failed }),
    });
  };

  return (
    <Modal open={open} onOpenChange={(o) => !o && handleClose()} size="lg">
      <ModalHeader
        icon={CalendarPlus}
        tone="info"
        title={t('holidays.import.title')}
        onClose={handleClose}
      />
      <ModalBody>
        <div className="flex flex-col gap-3">
          {/* Year stepper */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1 rounded-lg border border-border bg-surface px-2 py-1">
              <button
                type="button"
                aria-label={t('holidays.import.prevYear')}
                onClick={() => setYear((y) => y - 1)}
                disabled={importing}
                className="flex size-7 items-center justify-center rounded-md text-text-2 hover:bg-surface-2 disabled:opacity-40"
              >
                <ChevronLeft className="size-4" aria-hidden />
              </button>
              <span className="min-w-[56px] text-center text-[15px] font-bold text-text">{year}</span>
              <button
                type="button"
                aria-label={t('holidays.import.nextYear')}
                onClick={() => setYear((y) => y + 1)}
                disabled={importing}
                className="flex size-7 items-center justify-center rounded-md text-text-2 hover:bg-surface-2 disabled:opacity-40"
              >
                <ChevronRight className="size-4" aria-hidden />
              </button>
            </div>
            {candidates && !result && (
              <button
                type="button"
                onClick={toggleAll}
                disabled={importing || importable.length === 0}
                className="text-[13px] font-medium text-primary hover:underline disabled:opacity-40"
              >
                {allSelected ? t('holidays.import.deselectAll') : t('holidays.import.selectAll')}
              </button>
            )}
          </div>

          {/* SKB note */}
          <div className="flex items-start gap-2 rounded-lg border border-info-bd bg-info-bg px-3 py-2">
            <Info className="mt-0.5 size-3.5 shrink-0 text-info-tx" aria-hidden />
            <p className="text-[11px] text-info-tx">{t('holidays.import.skbNote')}</p>
          </div>

          {/* Result summary */}
          {result && (
            <Banner
              tone={result.failed > 0 ? 'warn' : 'info'}
              title={t('holidays.import.resultSummary', {
                created: result.created,
                skipped: result.skipped,
                failed: result.failed,
              })}
            />
          )}

          {/* Candidate list */}
          {libError ? (
            <StateView
              kind="error"
              title={t('holidays.import.loadFailed')}
              description={t('holidays.import.loadFailedBody')}
              onRetry={() => setYear((y) => y)}
              retryLabel={t('common.retry')}
            />
          ) : !candidates || existingQuery.isLoading ? (
            <StateView kind="loading" title={t('common.loading')} />
          ) : candidates.length === 0 ? (
            <p className="py-6 text-center text-[13px] text-text-3">
              {t('holidays.import.empty')}
            </p>
          ) : (
            <ul className="max-h-[340px] overflow-y-auto rounded-lg border border-border-soft">
              {candidates.map((c) => {
                const exists = existingDates.has(c.date);
                const checked = !exists && selected.has(c.id);
                return (
                  <li
                    key={c.id}
                    className="flex items-center gap-3 border-b border-border-soft px-3 py-2 last:border-b-0"
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      disabled={exists || importing}
                      onChange={() => toggle(c)}
                      aria-label={c.name}
                      className="size-4 shrink-0 accent-primary disabled:opacity-40"
                    />
                    <span className="w-[88px] shrink-0 font-mono text-[12px] text-text-2">
                      {c.date}
                    </span>
                    <Input
                      value={nameOverrides[c.id] ?? c.name}
                      onChange={(e) =>
                        setNameOverrides((m) => ({ ...m, [c.id]: e.target.value }))
                      }
                      disabled={exists || importing}
                      className="h-8 flex-1 text-[13px]"
                    />
                    {c.recurring && (
                      <span className="shrink-0 rounded bg-surface-2 px-1.5 py-0.5 text-[10px] font-medium text-text-3">
                        {t('holidays.import.recurringTag')}
                      </span>
                    )}
                    {exists && (
                      <span className="shrink-0 rounded bg-ok-bg px-1.5 py-0.5 text-[10px] font-semibold text-ok-tx">
                        {t('holidays.import.exists')}
                      </span>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </ModalBody>
      <ModalFooter>
        <span className="flex-1 text-[12px] text-text-3">
          {importing
            ? t('holidays.import.importingProgress', { done: progress, total: selectedImportable.length })
            : candidates
              ? t('holidays.import.selectedCount', { count: selectedImportable.length })
              : ''}
        </span>
        <Button type="button" variant="secondary" onClick={handleClose} disabled={importing}>
          {result ? t('common.close') : t('common.cancel')}
        </Button>
        {!result && (
          <Button
            type="button"
            onClick={runImport}
            disabled={importing || selectedImportable.length === 0}
          >
            {importing
              ? t('common.processing')
              : t('holidays.import.importBtn', { count: selectedImportable.length })}
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}
