/**
 * E4 · Jadwal Mingguan — overlay / popover layer for F4.2 cell-assign flows.
 *
 * .pen frames:
 *   BfUbA  E4 · Shift Picker Popover (cell click)
 *   (Bulk-apply modal is inline here — no separate frame, designed from BfUbA context strip
 *    and the "Terapkan ke rentang" button on Rubba.)
 *
 * Exports:
 *   ShiftPickerPopover  — anchored lightweight popover for cell-assign (F4.2)
 *   BulkApplyModal      — modal for "Terapkan ke rentang" (SA-3)
 *
 * Conflict codes surfaced as toasts / inline warnings:
 *   DOUBLE_SHIFT · SHIFT_OVER_LEAVE · OUTSIDE_PLACEMENT_PERIOD ·
 *   OUT_OF_SCOPE · SHIFT_NOT_FOR_SERVICE_LINE · SHIFT_DEACTIVATED
 *   COVERAGE_BELOW_MIN (informational warn toast, non-blocking)
 *
 * INV-1/3/4 · SA-1/2/3/4/5 · SM-3/5
 * i18n namespace: schedule
 */

import { classifyError } from '@/lib/api-error.ts';
import {
  BulkApplyRequestKind,
  type ConflictDetails,
  ScheduleEntryWriteRequestKind,
  type ShiftMaster,
  useBulkApplySchedule,
  useCheckScheduleConflicts,
  useCreateScheduleEntry,
  useDeleteScheduleEntry,
  useListShiftMasters,
  useUpdateScheduleEntry,
} from '@swp/api-client/e4';
import { Button, Modal, ModalBody, ModalFooter, useToast } from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { Copy } from 'lucide-react';
import {
  CalendarOff,
  Check,
  ChevronDown,
  CirclePlus,
  Loader2,
  Search,
  Trash2,
  Users,
} from 'lucide-react';
import * as React from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CellTarget {
  employeeId: string;
  employeeName: string;
  placementId: string;
  date: string;
  existingEntryId?: string;
  existingShiftName?: string;
  isDayOff?: boolean;
}

interface ShiftPickerPopoverProps {
  target: CellTarget | null;
  anchorRef: React.RefObject<HTMLElement | null>;
  onClose: () => void;
  onMutated: () => void;
  scheduleQueryKey: readonly unknown[];
}

export interface BulkApplyModalProps {
  open: boolean;
  onClose: () => void;
  companyId: string;
  /** Agents placed at the company — the selectable targets for the bulk apply. */
  agents: { id: string; name: string }[];
  onMutated: () => void;
  scheduleQueryKey: readonly unknown[];
}

// ---------------------------------------------------------------------------
// Conflict → human-readable message helper
// ---------------------------------------------------------------------------

type ConflictCode =
  | 'DOUBLE_SHIFT'
  | 'SHIFT_OVER_LEAVE'
  | 'OUTSIDE_PLACEMENT_PERIOD'
  | 'OUT_OF_SCOPE'
  | 'SHIFT_NOT_FOR_SERVICE_LINE'
  | 'SHIFT_DEACTIVATED'
  | 'COVERAGE_BELOW_MIN';

function conflictMessage(
  code: string,
  t: (k: string, opts?: Record<string, string | number | undefined>) => string,
  details?: ConflictDetails,
): string {
  switch (code as ConflictCode) {
    case 'DOUBLE_SHIFT':
      return t('conflict.doubleShift', { shift: details?.existing_shift_name ?? '' });
    case 'SHIFT_OVER_LEAVE':
      return t('conflict.shiftOverLeave', { leaveId: details?.leave_request_id ?? '' });
    case 'OUTSIDE_PLACEMENT_PERIOD':
      return t('conflict.outsidePlacementPeriod');
    case 'OUT_OF_SCOPE':
      return t('conflict.outOfScope');
    case 'SHIFT_DEACTIVATED':
      return t('conflict.shiftDeactivated');
    case 'COVERAGE_BELOW_MIN':
      return t('conflict.coverageBelowMin');
    default:
      return t('conflict.unknown');
  }
}

// ---------------------------------------------------------------------------
// ShiftPickerPopover
// ---------------------------------------------------------------------------

export function ShiftPickerPopover({
  target,
  anchorRef,
  onClose,
  onMutated,
  scheduleQueryKey,
}: ShiftPickerPopoverProps) {
  const { t } = useTranslation('schedule');
  const { toast } = useToast();
  const qc = useQueryClient();

  const [search, setSearch] = React.useState('');
  const [saving, setSaving] = React.useState(false);

  const popoverRef = React.useRef<HTMLDivElement>(null);

  // Position the popover at the clicked cell. It must be `fixed` (viewport-anchored):
  // the grid container is `relative overflow-hidden`, so an `absolute` popover with no
  // offsets pins to the grid's top-left corner and gets clipped — i.e. clicking "+"
  // appeared to do nothing. Compute from the anchor cell's rect, clamped to the viewport.
  const POPOVER_W = 360;
  const POPOVER_MAXH = 360;
  const [pos, setPos] = React.useState<{ top: number; left: number } | null>(null);
  React.useLayoutEffect(() => {
    if (!target || !anchorRef.current) {
      setPos(null);
      return;
    }
    const r = anchorRef.current.getBoundingClientRect();
    const top = Math.max(8, Math.min(r.bottom + 4, window.innerHeight - POPOVER_MAXH - 8));
    const left = Math.max(8, Math.min(r.left, window.innerWidth - POPOVER_W - 8));
    setPos({ top, left });
  }, [target, anchorRef]);

  // Close on outside mousedown (ENGINEERING.md combobox pattern)
  React.useEffect(() => {
    if (!target) return;
    const handle = (e: MouseEvent) => {
      const node = e.target as Node;
      if (
        popoverRef.current &&
        !popoverRef.current.contains(node) &&
        !anchorRef.current?.contains(node)
      ) {
        onClose();
      }
    };
    document.addEventListener('mousedown', handle);
    return () => document.removeEventListener('mousedown', handle);
  }, [target, onClose, anchorRef]);

  // Shift list — all active shift masters (shift catalog is service-line-independent).
  const shiftsQuery = useListShiftMasters(
    {
      status: 'ACTIVE',
      q: search || undefined,
      limit: 50,
    },
    { query: { enabled: !!target, staleTime: 30_000 } },
  );

  const shifts: ShiftMaster[] =
    (shiftsQuery.data?.data as { data?: ShiftMaster[]; next_cursor?: string | null } | undefined)
      ?.data ?? [];

  const checkConflicts = useCheckScheduleConflicts();
  const createEntry = useCreateScheduleEntry();
  const updateEntry = useUpdateScheduleEntry();
  const deleteEntry = useDeleteScheduleEntry();

  if (!target) return null;

  const invalidate = () => qc.invalidateQueries({ queryKey: scheduleQueryKey });

  const handleAssignShift = async (shift: ShiftMaster) => {
    setSaving(true);
    try {
      // Dry-run check first (SA-1 pre-flight)
      const checkResult = await checkConflicts.mutateAsync({
        data: {
          kind: ScheduleEntryWriteRequestKind.single,
          employee_id: target.employeeId,
          shift_master_id: shift.id,
          date: target.date,
          is_day_off: false,
          force_replace: false,
        },
      });

      const result = checkResult.data as
        | {
            succeeded?: unknown[];
            failed?: Array<{ error?: { code?: string; details?: ConflictDetails } }>;
          }
        | undefined;
      const failures = result?.failed ?? [];

      if (failures.length > 0) {
        const first = failures[0];
        const code = first?.error?.code ?? 'UNKNOWN';
        // The real BE envelope nests conflict details under `error.details` (the
        // bulk/:check `failed[].error` location, mirroring the Phase-5 error.details
        // precedent), NOT a top-level `conflict_details`. Read it from there so the
        // DOUBLE_SHIFT / SHIFT_OVER_LEAVE / over-leave block messages render.
        const details = first?.error?.details;

        if (code === 'DOUBLE_SHIFT') {
          // SA-2: double-shift asks for force_replace confirmation
          await handleForceReplace(shift, details);
          return;
        }

        toast({
          tone: 'error',
          title: t('conflict.blockedTitle'),
          description: conflictMessage(code, t, details),
        });
        setSaving(false);
        return;
      }

      // Actual write
      if (target.existingEntryId) {
        await updateEntry.mutateAsync({
          id: target.existingEntryId,
          data: { shift_master_id: shift.id, is_day_off: false },
        });
      } else {
        await createEntry.mutateAsync({
          data: {
            kind: ScheduleEntryWriteRequestKind.single,
            employee_id: target.employeeId,
            shift_master_id: shift.id,
            date: target.date,
            is_day_off: false,
          },
        });
      }

      await invalidate();
      onMutated();
      onClose();
      toast({ tone: 'success', title: t('toast.published') });
    } catch (err) {
      const { kind } = classifyError(err);
      if (kind === 'forbidden') {
        toast({ tone: 'error', title: t('conflict.outOfScope') });
      } else {
        toast({ tone: 'error', title: t('toast.saveFailed') });
      }
    } finally {
      setSaving(false);
    }
  };

  const handleForceReplace = async (shift: ShiftMaster, _details?: ConflictDetails) => {
    // SA-2: user already triggered shift picker on an existing cell — treat as replace
    try {
      if (target.existingEntryId) {
        await updateEntry.mutateAsync({
          id: target.existingEntryId,
          data: { shift_master_id: shift.id, is_day_off: false },
        });
      } else {
        await createEntry.mutateAsync({
          data: {
            kind: ScheduleEntryWriteRequestKind.single,
            employee_id: target.employeeId,
            shift_master_id: shift.id,
            date: target.date,
            is_day_off: false,
            force_replace: true,
          },
        });
      }
      await invalidate();
      onMutated();
      onClose();
      toast({ tone: 'success', title: t('toast.published') });
    } catch (err) {
      classifyError(err);
      toast({ tone: 'error', title: t('toast.saveFailed') });
    } finally {
      setSaving(false);
    }
  };

  const handleMarkDayOff = async () => {
    setSaving(true);
    try {
      if (target.existingEntryId) {
        await updateEntry.mutateAsync({
          id: target.existingEntryId,
          data: { is_day_off: true, shift_master_id: null },
        });
      } else {
        await createEntry.mutateAsync({
          data: {
            kind: ScheduleEntryWriteRequestKind.single,
            employee_id: target.employeeId,
            shift_master_id: undefined,
            date: target.date,
            is_day_off: true,
          },
        });
      }
      await invalidate();
      onMutated();
      onClose();
      toast({ tone: 'success', title: t('toast.published') });
    } catch (err) {
      classifyError(err);
      toast({ tone: 'error', title: t('toast.saveFailed') });
    } finally {
      setSaving(false);
    }
  };

  const handleClearCell = async () => {
    if (!target.existingEntryId) {
      onClose();
      return;
    }
    setSaving(true);
    try {
      await deleteEntry.mutateAsync({ id: target.existingEntryId });
      await invalidate();
      onMutated();
      onClose();
      toast({ tone: 'success', title: t('toast.cleared') });
    } catch (err) {
      classifyError(err);
      toast({ tone: 'error', title: t('toast.saveFailed') });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div
      ref={popoverRef}
      className="fixed z-50 w-[360px] rounded-xl border border-border bg-surface shadow-overlay"
      style={{ top: pos?.top ?? -9999, left: pos?.left ?? -9999 }}
      aria-label={t('picker.title', { name: target.employeeName })}
    >
      {/* Header */}
      <div className="border-b border-border-soft px-3.5 py-3">
        <p className="text-sm font-bold text-text">
          {t('picker.title', { name: target.employeeName })}
        </p>
        <p className="text-xs text-text-3">{target.date}</p>
      </div>

      {/* Search + filter */}
      <div className="flex items-center gap-2 border-b border-border-soft bg-surface-2 px-3.5 py-2.5">
        <div className="flex flex-1 items-center gap-1.5 rounded-[7px] border border-border bg-surface px-2.5 py-1.5">
          <Search aria-hidden className="size-3 text-text-3" />
          <input
            type="text"
            placeholder={t('picker.searchPlaceholder')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="flex-1 bg-transparent text-xs text-text placeholder:text-text-3 outline-none"
          />
        </div>
      </div>

      {/* Shift list */}
      <div className="max-h-[280px] overflow-y-auto">
        {shiftsQuery.isLoading && (
          <div className="flex items-center justify-center py-6">
            <Loader2 aria-hidden className="size-4 animate-spin text-text-3" />
          </div>
        )}

        {!shiftsQuery.isLoading &&
          shifts.map((shift) => (
            <ShiftRow key={shift.id} shift={shift} onSelect={handleAssignShift} disabled={saving} />
          ))}

        {!shiftsQuery.isLoading && shifts.length === 0 && (
          <p className="px-3.5 py-4 text-xs text-text-3">{t('picker.noShifts')}</p>
        )}
      </div>

      {/* Quick actions */}
      <div className="border-t border-border-soft bg-surface-2">
        <button
          type="button"
          disabled={saving}
          onClick={handleMarkDayOff}
          className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-left text-sm text-text-2 transition-colors hover:bg-surface disabled:opacity-50"
        >
          <CalendarOff aria-hidden className="size-3.5" />
          <div>
            <p className="text-sm font-medium text-text">{t('picker.actionDayOff')}</p>
            <p className="text-xs text-text-3">{t('picker.actionDayOffDesc')}</p>
          </div>
        </button>

        {target.existingEntryId && (
          <button
            type="button"
            disabled={saving}
            onClick={handleClearCell}
            className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-left text-sm text-text-2 transition-colors hover:bg-surface disabled:opacity-50"
          >
            <Trash2 aria-hidden className="size-3.5" />
            <div>
              <p className="text-sm font-medium text-text">{t('picker.actionClear')}</p>
              <p className="text-xs text-text-3">{t('picker.actionClearDesc')}</p>
            </div>
          </button>
        )}

        <button
          type="button"
          disabled
          className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-left opacity-50"
        >
          <CirclePlus aria-hidden className="size-3.5 text-text-2" />
          <div>
            <p className="text-sm font-medium text-text">{t('picker.actionNewShift')}</p>
            <p className="text-xs text-text-3">{t('picker.actionNewShiftDesc')}</p>
          </div>
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// ShiftRow — reusable row inside the picker list
// ---------------------------------------------------------------------------

interface ShiftRowProps {
  shift: ShiftMaster;
  onSelect: (shift: ShiftMaster) => void;
  disabled?: boolean;
}

function ShiftRow({ shift, onSelect, disabled }: ShiftRowProps) {
  const hours = React.useMemo(() => {
    if (!shift.start_time || !shift.end_time) return '';
    return `${shift.start_time.slice(0, 5)} – ${shift.end_time.slice(0, 5)}`;
  }, [shift.start_time, shift.end_time]);

  return (
    <button
      type="button"
      disabled={disabled}
      onClick={() => onSelect(shift)}
      className="flex w-full items-center gap-2.5 border-b border-border-soft px-3.5 py-2.5 text-left transition-colors hover:bg-surface-2 disabled:opacity-50 last:border-b-0"
    >
      <span aria-hidden className="mt-0.5 size-2.5 shrink-0 rounded-full bg-accent-gold" />
      <span className="flex-1 min-w-0">
        <span className="block text-[13px] font-semibold text-text">{shift.name}</span>
        {shift.break_start && (
          <span className="block text-[11px] text-text-3">
            istirahat {shift.break_start}
            {shift.break_end ? ` – ${shift.break_end}` : ''}
          </span>
        )}
        {shift.cross_midnight && (
          <span className="ml-1 rounded bg-warn-bg px-1.5 py-0.5 text-[10px] font-bold text-warn-tx">
            +1
          </span>
        )}
      </span>
      <span className="shrink-0 font-mono text-[11px] font-semibold text-text-2">{hours}</span>
    </button>
  );
}

// ---------------------------------------------------------------------------
// BulkApplyModal — "Terapkan ke rentang" (SA-3)
// ---------------------------------------------------------------------------

export function BulkApplyModal({
  open,
  onClose,
  companyId: _companyId,
  agents,
  onMutated,
  scheduleQueryKey,
}: BulkApplyModalProps) {
  const { t } = useTranslation('schedule');
  const { toast } = useToast();
  const qc = useQueryClient();

  const [agentSearch, setAgentSearch] = React.useState('');
  const [selectedAgent, setSelectedAgent] = React.useState<{ id: string; name: string } | null>(
    null,
  );
  const [agentMenuOpen, setAgentMenuOpen] = React.useState(false);
  const [shiftSearch, setShiftSearch] = React.useState('');
  const [selectedShift, setSelectedShift] = React.useState<ShiftMaster | null>(null);
  const [startDate, setStartDate] = React.useState('');
  const [endDate, setEndDate] = React.useState('');
  const [weekdays, setWeekdays] = React.useState<number[]>([1, 2, 3, 4, 5, 6, 7]);
  const [overrideExisting, setOverrideExisting] = React.useState(false);
  const [previewDone, setPreviewDone] = React.useState(false);
  const [previewSucceeded, setPreviewSucceeded] = React.useState(0);
  const [previewFailed, setPreviewFailed] = React.useState(0);
  const [confirming, setConfirming] = React.useState(false);

  const shiftsQuery = useListShiftMasters(
    { q: shiftSearch || undefined, status: 'ACTIVE', limit: 50 },
    { query: { enabled: open, staleTime: 30_000 } },
  );
  const shifts: ShiftMaster[] =
    (shiftsQuery.data?.data as { data?: ShiftMaster[] } | undefined)?.data ?? [];

  const checkConflicts = useCheckScheduleConflicts();
  const bulkApply = useBulkApplySchedule();

  const canPreview =
    !!selectedShift && !!startDate && !!endDate && startDate <= endDate && weekdays.length > 0;
  const targetEmployeeIds = selectedAgent ? [selectedAgent.id] : agents.map((a) => a.id);
  const filteredAgents = agents.filter((a) =>
    a.name.toLowerCase().includes(agentSearch.toLowerCase()),
  );

  const handlePreview = async () => {
    if (!selectedShift || !startDate || !endDate) return;
    try {
      const result = await checkConflicts.mutateAsync({
        data: {
          kind: BulkApplyRequestKind.bulk,
          shift_master_id: selectedShift.id,
          start_date: startDate,
          end_date: endDate,
          employee_ids: targetEmployeeIds,
          weekdays_mask: weekdays,
          override_existing: overrideExisting,
        },
      });
      const r = result.data as { succeeded?: unknown[]; failed?: unknown[] } | undefined;
      setPreviewSucceeded(r?.succeeded?.length ?? 0);
      setPreviewFailed(r?.failed?.length ?? 0);
      setPreviewDone(true);
    } catch {
      toast({ tone: 'error', title: t('toast.previewFailed') });
    }
  };

  const handleApply = async () => {
    if (!selectedShift || !startDate || !endDate) return;
    setConfirming(true);
    try {
      await bulkApply.mutateAsync({
        data: {
          kind: BulkApplyRequestKind.bulk,
          shift_master_id: selectedShift.id,
          start_date: startDate,
          end_date: endDate,
          employee_ids: targetEmployeeIds,
          weekdays_mask: weekdays,
          override_existing: overrideExisting,
        },
      });
      await qc.invalidateQueries({ queryKey: scheduleQueryKey });
      onMutated();
      onClose();
      toast({
        tone: 'success',
        title: t('toast.bulkApplied', { count: previewSucceeded }),
      });
    } catch {
      toast({ tone: 'error', title: t('toast.saveFailed') });
    } finally {
      setConfirming(false);
    }
  };

  const handleClose = () => {
    setSelectedShift(null);
    setSelectedAgent(null);
    setAgentSearch('');
    setAgentMenuOpen(false);
    setStartDate('');
    setEndDate('');
    setPreviewDone(false);
    onClose();
  };

  const weekdayLabels: { n: number; label: string }[] = [
    { n: 1, label: 'Sen' },
    { n: 2, label: 'Sel' },
    { n: 3, label: 'Rab' },
    { n: 4, label: 'Kam' },
    { n: 5, label: 'Jum' },
    { n: 6, label: 'Sab' },
    { n: 7, label: 'Min' },
  ];

  const toggleWeekday = (n: number) => {
    setWeekdays((prev) => (prev.includes(n) ? prev.filter((d) => d !== n) : [...prev, n].sort()));
    setPreviewDone(false);
  };

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose();
      }}
      size="md"
    >
      {/* header */}
      <div className="flex items-center justify-between border-b border-border-soft px-5 py-4">
        <div className="flex items-center gap-3">
          <span className="inline-flex size-9 items-center justify-center rounded-full bg-primary-soft text-primary">
            <Copy aria-hidden className="size-[18px]" />
          </span>
          <p className="text-base font-bold text-text">{t('bulk.title')}</p>
        </div>
        <button
          type="button"
          aria-label={t('common.close')}
          onClick={handleClose}
          className="inline-flex size-[30px] items-center justify-center rounded-md bg-surface-2 text-text-2 hover:bg-surface"
        >
          ×
        </button>
      </div>
      <ModalBody>
        <div className="flex flex-col gap-4">
          {/* Agent selector — scope the bulk apply to one agent or all */}
          <div className="relative">
            <span className="mb-1.5 block text-sm font-semibold text-text">
              {t('bulk.agentLabel')}
            </span>
            <button
              type="button"
              onClick={() => setAgentMenuOpen((o) => !o)}
              className="flex w-full items-center gap-2 rounded-lg border border-border bg-surface px-3 py-2 text-left text-sm text-text"
            >
              <Users aria-hidden className="size-3.5 shrink-0 text-text-3" />
              <span className="flex-1 truncate">
                {selectedAgent ? selectedAgent.name : t('bulk.allAgents', { count: agents.length })}
              </span>
              <ChevronDown aria-hidden className="size-3.5 shrink-0 text-text-3" />
            </button>
            {agentMenuOpen && (
              <div className="absolute z-10 mt-1 w-full rounded-lg border border-border bg-surface shadow-overlay">
                <div className="flex items-center gap-2 border-b border-border-soft px-3 py-2">
                  <Search aria-hidden className="size-3.5 text-text-3" />
                  <input
                    type="text"
                    // biome-ignore lint/a11y/noAutofocus: opening the menu intends focus
                    autoFocus
                    placeholder={t('bulk.agentSearchPlaceholder')}
                    value={agentSearch}
                    onChange={(e) => setAgentSearch(e.target.value)}
                    className="flex-1 bg-transparent text-sm text-text placeholder:text-text-3 outline-none"
                  />
                </div>
                <div className="max-h-44 overflow-y-auto">
                  <button
                    type="button"
                    onClick={() => {
                      setSelectedAgent(null);
                      setAgentMenuOpen(false);
                      setAgentSearch('');
                      setPreviewDone(false);
                    }}
                    className={`flex w-full items-center px-3 py-2 text-left text-sm hover:bg-surface-2 ${
                      !selectedAgent ? 'bg-primary-soft text-primary' : 'text-text'
                    }`}
                  >
                    {t('bulk.allAgents', { count: agents.length })}
                  </button>
                  {filteredAgents.map((a) => (
                    <button
                      key={a.id}
                      type="button"
                      onClick={() => {
                        setSelectedAgent(a);
                        setAgentMenuOpen(false);
                        setAgentSearch('');
                        setPreviewDone(false);
                      }}
                      className={`flex w-full items-center px-3 py-2 text-left text-sm hover:bg-surface-2 ${
                        selectedAgent?.id === a.id ? 'bg-primary-soft text-primary' : 'text-text'
                      }`}
                    >
                      {a.name}
                    </button>
                  ))}
                  {filteredAgents.length === 0 && (
                    <p className="px-3 py-2 text-xs text-text-3">{t('bulk.noAgents')}</p>
                  )}
                </div>
              </div>
            )}
          </div>

          {/* Shift selector */}
          <div>
            <span className="mb-1.5 block text-sm font-semibold text-text">
              {t('bulk.shiftLabel')}
            </span>
            <div className="flex items-center gap-2 rounded-lg border border-border bg-surface px-3 py-2">
              <Search aria-hidden className="size-3.5 text-text-3" />
              <input
                type="text"
                placeholder={t('picker.searchPlaceholder')}
                value={shiftSearch}
                onChange={(e) => {
                  setShiftSearch(e.target.value);
                  setPreviewDone(false);
                }}
                className="flex-1 bg-transparent text-sm text-text placeholder:text-text-3 outline-none"
              />
            </div>
            {shifts.length > 0 && (
              <div className="mt-1 max-h-40 overflow-y-auto rounded-lg border border-border bg-surface">
                {shifts.map((s) => (
                  <button
                    key={s.id}
                    type="button"
                    onClick={() => {
                      setSelectedShift(s);
                      setShiftSearch(s.name);
                      setPreviewDone(false);
                    }}
                    className={`flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-surface-2 ${
                      selectedShift?.id === s.id ? 'bg-primary-soft text-primary' : 'text-text'
                    }`}
                  >
                    <span className="font-semibold">{s.name}</span>
                    <span className="ml-auto font-mono text-xs text-text-3">
                      {s.start_time?.slice(0, 5)} – {s.end_time?.slice(0, 5)}
                    </span>
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Date range */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <span className="mb-1.5 block text-sm font-semibold text-text">
                {t('bulk.startDate')}
              </span>
              <input
                type="date"
                value={startDate}
                onChange={(e) => {
                  setStartDate(e.target.value);
                  setPreviewDone(false);
                }}
                className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text outline-none focus:border-primary"
              />
            </div>
            <div>
              <span className="mb-1.5 block text-sm font-semibold text-text">
                {t('bulk.endDate')}
              </span>
              <input
                type="date"
                value={endDate}
                min={startDate}
                onChange={(e) => {
                  setEndDate(e.target.value);
                  setPreviewDone(false);
                }}
                className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text outline-none focus:border-primary"
              />
            </div>
          </div>

          {/* Weekday mask */}
          <div>
            <p className="text-sm font-semibold text-text">{t('bulk.weekdays')}</p>
            <p className="mb-2 text-xs text-text-3">{t('bulk.weekdaysHint')}</p>
            <div className="flex flex-wrap gap-1.5">
              {weekdayLabels.map(({ n, label }) => {
                const on = weekdays.includes(n);
                return (
                  <button
                    key={n}
                    type="button"
                    aria-pressed={on}
                    onClick={() => toggleWeekday(n)}
                    className={`inline-flex items-center gap-1 rounded-md border px-2.5 py-1 text-xs font-semibold transition-colors ${
                      on
                        ? 'border-primary bg-primary text-white'
                        : 'border-dashed border-border bg-surface text-text-3 hover:border-text-3 hover:text-text-2'
                    }`}
                  >
                    {on && <Check aria-hidden className="size-3" />}
                    {label}
                  </button>
                );
              })}
            </div>
            <p className="mt-1.5 text-xs text-text-3">
              {weekdays.length > 0
                ? t('bulk.weekdaysCount', { count: weekdays.length })
                : t('bulk.weekdaysNone')}
            </p>
          </div>

          {/* Override toggle */}
          <label className="flex cursor-pointer items-center gap-2">
            <input
              type="checkbox"
              checked={overrideExisting}
              onChange={(e) => {
                setOverrideExisting(e.target.checked);
                setPreviewDone(false);
              }}
              className="rounded border-border"
            />
            <span className="text-sm text-text">{t('bulk.overrideExisting')}</span>
          </label>

          {/* Preview result */}
          {previewDone && (
            <div className="rounded-lg border border-info-bd bg-info-bg px-3 py-2 text-sm">
              <p className="font-semibold text-info-tx">
                {t('bulk.previewResult', {
                  succeeded: previewSucceeded,
                  failed: previewFailed,
                })}
              </p>
              {previewFailed > 0 && (
                <p className="text-xs text-text-3">{t('bulk.previewFailedHint')}</p>
              )}
            </div>
          )}
        </div>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={handleClose} disabled={confirming}>
          {t('bulk.cancel')}
        </Button>
        {!previewDone ? (
          <Button
            variant="primary"
            onClick={handlePreview}
            disabled={!canPreview || checkConflicts.isPending}
          >
            {checkConflicts.isPending ? (
              <Loader2 aria-hidden className="mr-1.5 size-3.5 animate-spin" />
            ) : null}
            {t('bulk.preview')}
          </Button>
        ) : (
          <Button
            variant="primary"
            onClick={handleApply}
            disabled={confirming || previewSucceeded === 0}
          >
            {confirming ? <Loader2 aria-hidden className="mr-1.5 size-3.5 animate-spin" /> : null}
            {t('bulk.apply', { count: previewSucceeded })}
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}
