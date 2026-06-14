/**
 * AgreementField — read-only auto-resolver for an employee's active employment agreement.
 *
 * Per domain (EA-2) an employee has at most ONE active employment agreement at a time,
 * so once the agent is chosen the agreement is uniquely determined — there is nothing
 * for the user to pick. This component fetches the agent's ACTIVE/EXPIRING agreement(s)
 * and auto-selects the single active one (BR-1/BR-1b: the placement period must sit
 * within the agreement's validity; comp/leave terms are read from it), rendering the
 * resolved agreement read-only.
 *
 * Used on the Create Placement form (g3OzZz), replacing the former AgreementPicker.
 *
 * Render states (all read-only — no dropdown):
 *   - no employee selected → muted placeholder
 *   - loading → spinner + "Memuat…"
 *   - resolved → read-only chip: agreement_no (fallback id) · type · validity
 *   - none found → advisory box (value stays null). Since EPICS §8 2026-06-11 the
 *     agreement is OPTIONAL: the placement can be created "menunggu perjanjian"
 *     (awaiting_agreement) and backfilled later, so this no longer gates submit.
 *
 * i18n namespace: `pickers`.
 */

import { useListAgreements } from '@swp/api-client/e2';
import { AlertTriangle, FileText, Loader2 } from 'lucide-react';
import { useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface AgreementFieldProps {
  employeeId: string | null;
  value: string | null;
  onChange: (id: string | null) => void;
  error?: boolean;
}

type AgreementItem = {
  id: string;
  agreement_no?: string | null;
  type?: string;
  end_date?: string | null;
};

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function AgreementField({ employeeId, value, onChange, error }: AgreementFieldProps) {
  const { t } = useTranslation('pickers');

  const result = useListAgreements(
    {
      employee_id: employeeId ?? undefined,
      status__in: 'ACTIVE,EXPIRING',
      limit: 50,
    },
    {
      query: {
        enabled: !!employeeId,
        staleTime: 30_000,
      },
    },
  );

  const agreements = (result.data?.data as { data?: AgreementItem[] } | undefined)?.data ?? [];
  // EA-2: at most one active agreement — the first is the canonical one.
  const resolved = agreements[0] ?? null;

  // Auto-resolve the agreement_id from the fetched result. We track the previous
  // employeeId (mirroring the picker's guard) so StrictMode's double-invoked effect
  // and the initial mount don't clobber the form's value, and we only call onChange
  // when the target id actually differs from the current value to avoid loops.
  const prevEmployeeId = useRef(employeeId);
  // biome-ignore lint/correctness/useExhaustiveDependencies: onChange is a stable form setter; keying on resolved/value is intentional
  useEffect(() => {
    if (!employeeId) {
      // No agent → ensure no stale agreement remains selected.
      if (value !== null) onChange(null);
      prevEmployeeId.current = employeeId;
      return;
    }

    // Wait for the query to settle before reconciling.
    if (result.isLoading) return;

    const targetId = resolved?.id ?? null;
    if (value !== targetId) {
      onChange(targetId);
    }
    prevEmployeeId.current = employeeId;
  }, [employeeId, resolved?.id, value, result.isLoading]);

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  // No employee chosen yet.
  if (!employeeId) {
    return (
      <div className="flex h-[38px] items-center rounded-lg border border-border bg-surface-2 px-3">
        <p className="text-[13px] text-text-3">{t('agreement.disabledPlaceholder')}</p>
      </div>
    );
  }

  // Loading the agent's agreement.
  if (result.isLoading) {
    return (
      <div className="flex h-[38px] items-center gap-2 rounded-lg border border-border bg-surface-2 px-3">
        <Loader2 className="size-4 animate-spin text-text-3" aria-hidden />
        <p className="text-[13px] text-text-3">{t('agreement.loading')}</p>
      </div>
    );
  }

  // No active agreement — advisory only (value stays null). Submit is allowed:
  // the placement is created awaiting an agreement, backfilled later (EPICS §8 2026-06-11).
  if (!resolved) {
    return (
      <div className="flex items-start gap-2 rounded-lg border border-warn-bd bg-warn-bg px-3 py-[9px]">
        <AlertTriangle className="mt-[1px] size-[15px] shrink-0 text-warn-tx" aria-hidden />
        <p className="text-[12px] font-medium leading-[1.4] text-warn-tx">
          {t('agreement.noneAdvisory')}
        </p>
      </div>
    );
  }

  // Resolved — read-only chip.
  const label = resolved.agreement_no ?? resolved.id;
  const validity = resolved.end_date
    ? t('agreement.validUntil', { date: resolved.end_date })
    : t('agreement.openEnded');
  const parts = [resolved.type, validity].filter(Boolean).join(' · ');

  return (
    <div
      className={`flex h-[38px] items-center gap-2 rounded-lg border bg-surface-2 px-3 ${
        error ? 'border-bad-bd' : 'border-border'
      }`}
    >
      <FileText className="size-4 shrink-0 text-text-3" aria-hidden />
      <span className="truncate text-[13px] font-medium text-text">{label}</span>
      {parts && <span className="truncate text-[12px] text-text-3">· {parts}</span>}
    </div>
  );
}
