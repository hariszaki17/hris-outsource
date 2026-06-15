/**
 * Riwayat Koreksi — the agent's own correction history (Pengajuan · Koreksi tab, F5.6).
 *
 * Self-scoped via useListCorrections (server resolves the principal; no requester_id sent).
 * Rows show type, the proposed change summary, a StatusBadge (Pending / Disetujui / Ditolak /
 * Diterapkan / Dibatalkan) and a payable indicator for NEW_ENTRY / no-shift days. Clicking a row
 * opens the status modal (proposed change + collapsed approval chain + withdraw).
 *
 * "Ajukan Koreksi" launches the searchable attendance picker → either the CHECK_IN/CHECK_OUT
 * form (existing row) or the NEW_ENTRY form (day with no record). No dead-flow states:
 * loading / empty / error all handled.
 */
import {
  type Correction,
  type CorrectionPage,
  CorrectionStatus,
  CorrectionType,
  useListCorrections,
} from '@swp/api-client/e5';
import type { StatusTone } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared';
import { Button, type Column, DataTable, EmptyState, StateView, StatusBadge } from '@swp/ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CorrectionAttendancePicker } from './correction-attendance-picker.tsx';
import { AgentCorrectionModal } from './me-correction-screen.tsx';
import { AgentCorrectionStatusModal } from './me-correction-status-modal.tsx';

function correctionStatusTone(status: CorrectionStatus): StatusTone {
  switch (status) {
    case CorrectionStatus.APPROVED:
    case CorrectionStatus.APPLIED:
      return 'ok';
    case CorrectionStatus.REJECTED:
      return 'bad';
    case CorrectionStatus.PENDING:
      return 'warn';
    default:
      return 'neutral';
  }
}

function timeOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { timeStyle: 'short' }) : '—';
}

/** A short summary of the proposed change for the list row. */
function changeSummary(c: Correction): string {
  const parts: string[] = [];
  if (c.proposed_check_in_at) parts.push(`↓ ${timeOf(c.proposed_check_in_at)}`);
  if (c.proposed_check_out_at) parts.push(`↑ ${timeOf(c.proposed_check_out_at)}`);
  return parts.join('  ');
}

/** Flow stage within the create wizard. */
type Stage =
  | { kind: 'none' }
  | { kind: 'picker' }
  | { kind: 'existing'; attendanceId: string; date: string }
  | { kind: 'new-entry' };

export function AgentCorrectionPanel() {
  const { t } = useTranslation('agent');

  const query = useListCorrections({ limit: 50 });
  const page = query.data?.data as CorrectionPage | undefined;
  const items: Correction[] = page?.data ?? [];

  const [stage, setStage] = useState<Stage>({ kind: 'none' });
  const [selected, setSelected] = useState<Correction | null>(null);

  function refetch() {
    void query.refetch();
  }

  const columns: Column<Correction>[] = [
    {
      id: 'date',
      header: t('corrColDate'),
      width: 150,
      cell: (c) => (
        <span className="font-medium text-text tabular-nums">
          {c.work_date
            ? formatInstant(c.work_date, { dateStyle: 'medium' })
            : formatInstant(c.created_at, { dateStyle: 'medium' })}
        </span>
      ),
    },
    {
      id: 'type',
      header: t('corrColType'),
      width: 170,
      cell: (c) => <span className="text-sm text-text-2">{t(`corrType.${c.type}`)}</span>,
    },
    {
      id: 'change',
      header: t('corrColChange'),
      cell: (c) => {
        const summary = changeSummary(c);
        return summary ? (
          <span className="text-sm text-text-2 tabular-nums">{summary}</span>
        ) : (
          <span className="line-clamp-1 text-sm text-text-3">{c.reason}</span>
        );
      },
    },
    {
      id: 'status',
      header: t('leaveStatus'),
      width: 220,
      cell: (c) => (
        <div className="flex items-center gap-2">
          <StatusBadge dot tone={correctionStatusTone(c.status as CorrectionStatus)}>
            {t(`corrStatus.${c.status}`, { defaultValue: c.status })}
          </StatusBadge>
          {/* NEW_ENTRY no-shift days await a payable decision by HR/SL after they're applied. */}
          {c.type === CorrectionType.NEW_ENTRY && c.status === CorrectionStatus.APPLIED && (
            <StatusBadge tone="info">{t('corrPayablePending')}</StatusBadge>
          )}
        </div>
      ),
    },
    {
      id: 'detail',
      header: '',
      width: 96,
      align: 'right',
      cell: (c) => (
        <Button variant="secondary" size="sm" onClick={() => setSelected(c)}>
          {t('common.detail', { ns: 'translation' })}
        </Button>
      ),
    },
  ];

  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="text-[15px] font-bold text-text">{t('corrHistoryTitle')}</h2>
        <Button variant="primary" size="sm" onClick={() => setStage({ kind: 'picker' })}>
          {t('corrNewBtn')}
        </Button>
      </div>

      {query.isError ? (
        <StateView kind="error" title={t('errorGeneric')} onRetry={refetch} />
      ) : (
        <DataTable
          aria-label={t('corrHistoryTitle')}
          columns={columns}
          data={items}
          getRowId={(c) => c.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          empty={<EmptyState variant="fresh" title={t('corrEmpty')} />}
        />
      )}

      {/* Step 1: searchable attendance picker */}
      <CorrectionAttendancePicker
        open={stage.kind === 'picker'}
        onOpenChange={(o) => {
          if (!o) setStage({ kind: 'none' });
        }}
        onSelect={({ attendanceId, date }) => setStage({ kind: 'existing', attendanceId, date })}
        onNewEntry={() => setStage({ kind: 'new-entry' })}
      />

      {/* Step 2a: correct an existing row (CHECK_IN / CHECK_OUT) */}
      {stage.kind === 'existing' && (
        <AgentCorrectionModal
          open
          mode="existing"
          attendanceId={stage.attendanceId}
          date={stage.date}
          onOpenChange={(o) => {
            if (!o) setStage({ kind: 'none' });
          }}
          onSuccess={() => {
            setStage({ kind: 'none' });
            refetch();
          }}
        />
      )}

      {/* Step 2b: create a record for a day with none (NEW_ENTRY) */}
      {stage.kind === 'new-entry' && (
        <AgentCorrectionModal
          open
          mode="new-entry"
          onOpenChange={(o) => {
            if (!o) setStage({ kind: 'none' });
          }}
          onSuccess={() => {
            setStage({ kind: 'none' });
            refetch();
          }}
        />
      )}

      {/* Detail / status modal */}
      {selected && (
        <AgentCorrectionStatusModal
          correction={selected}
          open
          onOpenChange={(o) => {
            if (!o) setSelected(null);
          }}
          onChanged={refetch}
        />
      )}
    </section>
  );
}
