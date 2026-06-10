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
import { Button, StateView, StatusBadge } from '@swp/ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { formatMoney, formatPeriod } from '../e8-payroll/payroll-shared.tsx';
import { AgentPage } from './agent-page.tsx';

// ---------------------------------------------------------------------------
// Page size — small for the agent card list
// ---------------------------------------------------------------------------

const PAGE_SIZE = 12;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * paid_on is an ISO date string (YYYY-MM-DD). Format as "DD Mon YYYY".
 * Returns null when not paid yet.
 */
function formatPaidOn(paidOn: string | null): string | null {
  if (!paidOn) return null;
  // paidOn is YYYY-MM-DD; parse without timezone ambiguity
  const [y, m, d] = paidOn.split('-').map(Number);
  if (!y || !m || !d) return paidOn;
  return new Date(y, m - 1, d).toLocaleDateString('id-ID', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  });
}

// ---------------------------------------------------------------------------
// PayslipCard
// ---------------------------------------------------------------------------

function PayslipCard({ p, t }: { p: Payslip; t: ReturnType<typeof useTranslation<'agent'>>['t'] }) {
  const paidOn = formatPaidOn(p.paid_on);
  const tone = paidOn ? ('ok' as const) : ('neutral' as const);

  return (
    <div className="rounded-xl border border-border bg-surface p-4">
      {/* Header: period + payment status */}
      <div className="flex items-center justify-between gap-3">
        <span className="text-[15px] font-semibold text-text">{formatPeriod(p.period)}</span>
        <StatusBadge dot tone={tone}>
          {paidOn ? t('payslipPaid') : t('payslipNotPaid')}
        </StatusBadge>
      </div>

      {/* Money rows */}
      <div className="mt-3 flex flex-col gap-1.5">
        <div className="flex items-center justify-between">
          <span className="text-[12px] text-text-2">{t('payslipTakeHome')}</span>
          <span className="text-[14px] font-semibold tabular-nums text-text">
            {formatMoney(p.take_home_pay)}
          </span>
        </div>
        <div className="flex items-center justify-between">
          <span className="text-[12px] text-text-2">{t('payslipGross')}</span>
          <span className="text-[12px] tabular-nums text-text-2">
            {formatMoney(p.gross_earnings)}
          </span>
        </div>
      </div>

      {/* Paid-on date (shown only when available) */}
      {paidOn && (
        <p className="mt-2 text-[11px] text-text-3">
          {t('payslipPaid')} {paidOn}
        </p>
      )}
    </div>
  );
}

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

  function goNext() {
    const nextCursor = body?.next_cursor;
    if (!nextCursor) return;
    setPrevCursors((prev) => [...prev, cursor ?? '']);
    setCursor(nextCursor);
  }

  function goPrev() {
    const next = [...prevCursors];
    const prev = next.pop();
    setPrevCursors(next);
    setCursor(prev || undefined);
  }

  const hasPrev = prevCursors.length > 0;

  return (
    <AgentPage title={t('payslipTitle')}>
      {query.isLoading ? (
        <StateView kind="loading" title={t('loading')} />
      ) : query.isError ? (
        <StateView
          kind="error"
          title={t('errorGeneric')}
          onRetry={() => void query.refetch()}
          retryLabel={t('retry')}
        />
      ) : items.length === 0 ? (
        <StateView kind="empty" title={t('payslipEmpty')} />
      ) : (
        <div className="flex flex-col gap-3">
          {items.map((p) => (
            <PayslipCard key={p.id} p={p} t={t} />
          ))}

          {/* Cursor pagination — only shown when there's more than one page */}
          {(hasPrev || hasMore) && (
            <div className="flex items-center justify-between pt-1">
              <Button
                type="button"
                variant="secondary"
                size="sm"
                disabled={!hasPrev}
                onClick={goPrev}
              >
                {t('prev', { defaultValue: 'Sebelumnya' })}
              </Button>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                disabled={!hasMore}
                onClick={goNext}
              >
                {t('next', { defaultValue: 'Berikutnya' })}
              </Button>
            </div>
          )}
        </div>
      )}
    </AgentPage>
  );
}
