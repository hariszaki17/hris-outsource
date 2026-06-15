/**
 * Pilih Kehadiran — searchable attendance picker for filing a correction (F5.6).
 *
 * The agent can't always remember which day was wrong, so step 1 of "Ajukan Koreksi" is to
 * find the problematic attendance row. This modal lists the agent's own attendance
 * (useListAttendance — server resolves the principal; no employee_id sent) with a free-text
 * search + date range, then selecting a row hands its id + date up to the correction form
 * (CHECK_IN/CHECK_OUT). A secondary affordance opens the NEW_ENTRY form for a day that has no
 * record at all (forgot entirely / no configured shift).
 *
 * States: loading (skeleton rows) · empty (no attendance) · no-results (filtered) · error (retry).
 * All times via the Asia/Jakarta TZ layer (@swp/shared).
 */
import {
  type Attendance,
  type AttendancePage,
  AttendanceStatus,
  useListAttendance,
} from '@swp/api-client/e5';
import type { StatusTone } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared';
import {
  Button,
  EmptyState,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  StateView,
  StatusBadge,
} from '@swp/ui';
import { CalendarPlus, Search } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

function attendanceStatusTone(status: AttendanceStatus): StatusTone {
  switch (status) {
    case AttendanceStatus.PRESENT:
      return 'ok';
    case AttendanceStatus.LATE:
    case AttendanceStatus.INCOMPLETE:
      return 'warn';
    case AttendanceStatus.ABSENT:
      return 'bad';
    case AttendanceStatus.ON_LEAVE:
      return 'info';
    default:
      return 'neutral';
  }
}

function timeOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { timeStyle: 'short' }) : '—';
}
function dateOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { dateStyle: 'medium' }) : '—';
}

export interface CorrectionAttendancePickerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Selecting a row → hands id + date to the CHECK_IN/CHECK_OUT form. */
  onSelect: (row: { attendanceId: string; date: string }) => void;
  /** "Buat untuk tanggal tanpa jadwal" → opens the NEW_ENTRY form. */
  onNewEntry: () => void;
}

export function CorrectionAttendancePicker({
  open,
  onOpenChange,
  onSelect,
  onNewEntry,
}: CorrectionAttendancePickerProps) {
  const { t } = useTranslation('agent');

  const [q, setQ] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');

  const params = {
    limit: 30,
    q: q.trim() || undefined,
    date_from: dateFrom || undefined,
    date_to: dateTo || undefined,
    sort: 'shift_start_at:desc',
  };

  const query = useListAttendance(params, { query: { enabled: open } });

  const body = query.data?.data as AttendancePage | undefined;
  const rows: Attendance[] = useMemo(() => body?.data ?? [], [body]);

  const hasFilters = Boolean(q.trim() || dateFrom || dateTo);

  function resetFilters() {
    setQ('');
    setDateFrom('');
    setDateTo('');
  }

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg" className="w-[620px]">
      <ModalHeader
        icon={Search}
        tone="brand"
        title={t('corrPickerTitle')}
        closeLabel={t('cancel')}
      />

      <ModalBody className="gap-4">
        <p className="text-[13px] text-text-3">{t('corrPickerSubtitle')}</p>

        {/* Filters: search + date range */}
        <div className="flex flex-wrap items-end gap-3">
          <label className="flex flex-1 flex-col gap-1" htmlFor="corr-picker-q">
            <span className="text-xs font-medium text-text-3">{t('corrPickerSearch')}</span>
            <Input
              id="corr-picker-q"
              value={q}
              placeholder={t('corrPickerSearch')}
              onChange={(e) => setQ(e.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1" htmlFor="corr-picker-from">
            <span className="text-xs font-medium text-text-3">{t('corrPickerDateFrom')}</span>
            <Input
              id="corr-picker-from"
              type="date"
              value={dateFrom}
              onChange={(e) => setDateFrom(e.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1" htmlFor="corr-picker-to">
            <span className="text-xs font-medium text-text-3">{t('corrPickerDateTo')}</span>
            <Input
              id="corr-picker-to"
              type="date"
              value={dateTo}
              onChange={(e) => setDateTo(e.target.value)}
            />
          </label>
        </div>

        {/* Rows */}
        <div className="min-h-[220px]">
          {query.isLoading ? (
            <StateView kind="loading" title={t('loading')} />
          ) : query.isError ? (
            <StateView
              kind="error"
              title={t('errorGeneric')}
              onRetry={() => void query.refetch()}
            />
          ) : rows.length === 0 ? (
            hasFilters ? (
              <EmptyState
                variant="filtered"
                title={t('corrPickerNoResults')}
                description={t('corrPickerNoResultsBody')}
              />
            ) : (
              <EmptyState variant="fresh" title={t('corrPickerEmpty')} />
            )
          ) : (
            <ul className="flex flex-col divide-y divide-border-soft rounded-lg border border-border">
              {rows.map((r) => (
                <li key={r.id}>
                  <button
                    type="button"
                    onClick={() =>
                      onSelect({
                        attendanceId: r.id,
                        date: r.check_in_at ?? r.shift_start_at ?? '',
                      })
                    }
                    className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-surface-2"
                  >
                    <span className="w-28 shrink-0 text-[13px] font-semibold text-text">
                      {dateOf(r.check_in_at ?? r.shift_start_at)}
                    </span>
                    <span className="flex flex-1 items-center gap-3 text-[13px] tabular-nums text-text-3">
                      <span>↓ {timeOf(r.check_in_at)}</span>
                      <span>↑ {timeOf(r.check_out_at)}</span>
                    </span>
                    <StatusBadge dot tone={attendanceStatusTone(r.status)}>
                      {t(`attendance.status.${r.status}`, {
                        ns: 'common',
                        defaultValue: r.status,
                      })}
                    </StatusBadge>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </ModalBody>

      <ModalFooter>
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={resetFilters}>
            {t('corrPickerReset')}
          </Button>
        )}
        <Button variant="secondary" size="sm" onClick={onNewEntry}>
          <CalendarPlus className="size-4" aria-hidden />
          {t('corrPickerNewEntry')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
