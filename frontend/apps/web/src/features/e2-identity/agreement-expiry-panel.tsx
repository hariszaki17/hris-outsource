/**
 * E2 · F2.7 — Contract-expiry decision panel (Class-A offboarding trigger).
 *
 * Surfaces PKWT agreements within 30d of end_date (status EXPIRING) as HR decisions:
 *   • Continue → renew (EA-3 / F3.1) — no revoke.
 *   • End      → offboard (END_OF_TERM) — closes agreement + revokes login (OB-4).
 * Grace (OB-6): once end_date passes the row stays here and the login stays valid
 * until HR acts — nothing auto-ends.
 *
 * DEMO DATA below — wires to GET /agreements?status=EXPIRING when the list endpoint
 * lands. The Continue path is a stub toast until the renew flow is wired (F3.1/EA-3).
 */

import { type Employee, EmployeeStatus } from '@swp/api-client/e2';
import { Button, StatusBadge, useToast } from '@swp/ui';
import { CalendarClock, FileWarning } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { OffboardEmployeeConfirm } from './employee-overlays.tsx';

interface ExpiringAgreement {
  agreement_id: string;
  agreement_no: string;
  employee_id: string;
  employee_name: string;
  end_date: string;
  /** Negative = past end_date, i.e. in the grace window (OB-6). */
  days_left: number;
}

// DEMO DATA (F2.7 OB-4) — replace with GET /agreements?status=EXPIRING.
const DEMO_EXPIRING: ExpiringAgreement[] = [
  {
    agreement_id: 'SWP-EA-2041',
    agreement_no: 'PKWT/2025/0412',
    employee_id: 'SWP-EMP-1042',
    employee_name: 'Budi Santoso',
    end_date: '2026-06-21',
    days_left: 15,
  },
  {
    agreement_id: 'SWP-EA-2038',
    agreement_no: 'PKWT/2025/0387',
    employee_id: 'SWP-EMP-1039',
    employee_name: 'Siti Nurhaliza',
    end_date: '2026-06-09',
    days_left: 3,
  },
  {
    agreement_id: 'SWP-EA-2033',
    agreement_no: 'PKWT/2025/0351',
    employee_id: 'SWP-EMP-1031',
    employee_name: 'Agus Pratama',
    end_date: '2026-06-02',
    days_left: -4,
  },
];

export function AgreementExpiryPanel() {
  const { t } = useTranslation('employees');
  const { toast } = useToast();
  const rows = DEMO_EXPIRING;

  // The employee targeted by an in-flight "End" decision (drives the offboard modal).
  const [endTarget, setEndTarget] = useState<Employee | null>(null);
  const [showOffboard, setShowOffboard] = useState(false);

  function handleContinue(row: ExpiringAgreement) {
    toast({ tone: 'info', title: t('expiryContinueToast', { name: row.employee_name }) });
  }

  function handleEnd(row: ExpiringAgreement) {
    // Minimal Employee shape — the offboard modal only reads id + full_name + status.
    setEndTarget({
      id: row.employee_id,
      full_name: row.employee_name,
      status: EmployeeStatus.ACTIVE,
    } as Employee);
    setShowOffboard(true);
  }

  return (
    <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border-soft px-[18px] py-[14px]">
        <div className="flex items-center gap-2">
          <FileWarning aria-hidden className="size-4 text-warn-tx" />
          <span className="text-[15px] font-bold text-text">{t('expiryTitle')}</span>
        </div>
        <div className="flex h-[22px] min-w-[24px] items-center justify-center rounded-full bg-bad-bg px-1.5">
          <span className="text-[12px] font-bold text-bad-tx">{rows.length}</span>
        </div>
      </div>

      {rows.length === 0 ? (
        <div className="px-[18px] py-6 text-center text-[13px] text-text-3">{t('expiryEmpty')}</div>
      ) : (
        rows.map((row, idx) => {
          const grace = row.days_left < 0;
          return (
            <div
              key={row.agreement_id}
              className={[
                'flex flex-col gap-3 px-[18px] py-[14px] sm:flex-row sm:items-center sm:justify-between',
                idx === rows.length - 1 ? '' : 'border-b border-border-soft',
              ]
                .filter(Boolean)
                .join(' ')}
            >
              {/* Left: who + when */}
              <div className="flex flex-col gap-1">
                <span className="text-[14px] font-semibold text-text">{row.employee_name}</span>
                <div className="flex flex-wrap items-center gap-2 text-[12px] text-text-3">
                  <span className="font-mono">{row.agreement_no}</span>
                  <span aria-hidden>·</span>
                  <span className="inline-flex items-center gap-1">
                    <CalendarClock className="size-3" aria-hidden />
                    {t('expiryEndsOn', { date: row.end_date })}
                  </span>
                  <StatusBadge dot tone={grace ? 'bad' : 'warn'}>
                    {grace ? t('expiryGrace') : t('expiryDaysLeft', { days: row.days_left })}
                  </StatusBadge>
                </div>
              </div>

              {/* Right: the decision */}
              <div className="flex shrink-0 items-center gap-2">
                <Button variant="secondary" size="sm" onClick={() => handleContinue(row)}>
                  {t('expiryContinue')}
                </Button>
                <Button variant="destructive" size="sm" onClick={() => handleEnd(row)}>
                  {t('expiryEnd')}
                </Button>
              </div>
            </div>
          );
        })
      )}

      {/* End → offboard (pre-seeded END_OF_TERM) */}
      <OffboardEmployeeConfirm
        open={showOffboard}
        onOpenChange={setShowOffboard}
        employee={endTarget}
        defaultReason="END_OF_TERM"
        onDone={() => setShowOffboard(false)}
      />
    </div>
  );
}
