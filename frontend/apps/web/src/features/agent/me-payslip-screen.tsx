/**
 * /me/payslip — Agent's own payslips (list). Web port of apps/mobile/app/payslip.tsx.
 *
 * F8 agent self-service: lists the authenticated agent's payslip history with period,
 * take-home pay, gross earnings, and payment status. Cursor pagination (no offset).
 * Server enforces `scope: self` — no employee_id is sent.
 *
 * RBAC: agent scope only (INV-3 / PH-5). No component breakdown; summary totals only.
 */
import { type Payslip, type PayslipListResponse, useListPayslips } from '@swp/api-client/e8';
import {
  type Column,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  StateView,
  StatusBadge,
} from '@swp/ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  formatMoney,
  formatPeriod,
  payslipStatusKey,
  payslipStatusTone,
} from '../e8-payroll/payroll-shared.tsx';
import { AgentPage } from './agent-page.tsx';

// ---------------------------------------------------------------------------
// Page size
// ---------------------------------------------------------------------------

const PAGE_SIZE = 20;

// ---------------------------------------------------------------------------
// AgentPayslipScreen
// ---------------------------------------------------------------------------

export function AgentPayslipScreen() {
  const { t } = useTranslation('agent');

  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  const query = useListPayslips({ limit: PAGE_SIZE, cursor });

  const body = query.data?.data as PayslipListResponse | undefined;
  const items: Payslip[] = body?.data ?? [];
  const hasMore = body?.has_more ?? false;

  // -------------------------------------------------------------------------
  // Columns
  // -------------------------------------------------------------------------

  const columns: Column<Payslip>[] = [
    {
      id: 'period',
      header: t('payslipPeriodCol', { defaultValue: 'Periode' }),
      width: 160,
      cell: (r) => <span className="font-medium text-text">{formatPeriod(r.period)}</span>,
    },
    {
      id: 'takeHome',
      header: t('payslipTakeHome', { defaultValue: 'Gaji Bersih' }),
      width: 180,
      cell: (r) => (
        <span className="font-semibold tabular-nums text-text">{formatMoney(r.take_home_pay)}</span>
      ),
    },
    {
      id: 'gross',
      header: t('payslipGross', { defaultValue: 'Penghasilan Bruto' }),
      width: 180,
      cell: (r) => (
        <span className="text-sm tabular-nums text-text-2">{formatMoney(r.gross_earnings)}</span>
      ),
    },
    {
      id: 'status',
      header: t('payslipStatusCol', { defaultValue: 'Status' }),
      width: 140,
      cell: (r) => (
        <StatusBadge dot tone={payslipStatusTone(r.status)}>
          {t(payslipStatusKey(r.status), { ns: 'payroll', defaultValue: r.status })}
        </StatusBadge>
      ),
    },
    {
      id: 'paidOn',
      header: t('payslipPaidOnCol', { defaultValue: 'Tanggal Bayar' }),
      width: 140,
      cell: (r) =>
        r.paid_on ? (
          <DateText kind="date" value={r.paid_on} className="text-sm text-text-2" />
        ) : (
          <span className="text-sm italic text-text-3">—</span>
        ),
    },
  ];

  // -------------------------------------------------------------------------
  // Render
  // -------------------------------------------------------------------------

  if (query.isError) {
    return (
      <AgentPage title={t('payslipTitle')}>
        <StateView
          kind="error"
          title={t('errorGeneric')}
          onRetry={() => void query.refetch()}
          retryLabel={t('retry')}
        />
      </AgentPage>
    );
  }

  return (
    <AgentPage title={t('payslipTitle')}>
      <DataTable
        aria-label={t('payslipTitle')}
        columns={columns}
        data={items}
        getRowId={(r) => r.id}
        isLoading={query.isLoading}
        skeletonRows={8}
        empty={
          <EmptyState
            variant="fresh"
            title={t('payslipEmpty', { defaultValue: 'Belum ada slip gaji' })}
          />
        }
        footer={
          items.length > 0 && (hasMore || prevCursors.length > 0) ? (
            <CursorPagination
              rangeLabel={t('resultRange', {
                defaultValue: '{{count}} entri',
                count: items.length,
              })}
              hasPrev={prevCursors.length > 0}
              hasNext={hasMore}
              prevLabel={t('prev', { defaultValue: 'Sebelumnya' })}
              nextLabel={t('next', { defaultValue: 'Berikutnya' })}
              onPrev={() => {
                const next = [...prevCursors];
                const prev = next.pop();
                setPrevCursors(next);
                setCursor(prev || undefined);
              }}
              onNext={() => {
                const nextCursor = body?.next_cursor;
                if (!nextCursor) return;
                setPrevCursors((prev) => [...prev, cursor ?? '']);
                setCursor(nextCursor);
              }}
            />
          ) : undefined
        }
      />
    </AgentPage>
  );
}
