/**
 * E8 · Payroll Empty & Access-Denied State Blocks.
 *
 * .pen frame `dRfK9` · "Empty & Access-denied · State-cards" · width 430.
 * Three reusable blocks for the payroll surface:
 *
 *  1. `PayrollEmptyHistory`   — agent's "no payslips yet" fresh state (mobile + web).
 *                               Variant: MOBILE (agent self-scope) per .pen sMENT.
 *  2. `PayrollEmptyFiltered`  — archive filter returned zero results. Per .pen XdGri.
 *  3. `PayrollAccessDenied`   — non-HR / shift-leader access denied. Per .pen wmJUD.
 *                               Maps Gherkin: agent-deny + shift-leader/agent → archive.
 *
 * All three are thin wrappers over `EmptyState` from `@swp/ui`. Copy is supplied via
 * i18n `payroll` namespace so it merges with the other E8 agent's keys.
 */

import { EmptyState } from '@swp/ui';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// 1. PayrollEmptyHistory
//    .pen: EmptyFresh (mrACi) override — wallet icon, no action button (ZQo1D disabled).
//    Copy: "Belum ada slip gaji" / "Riwayat slip akan muncul…"
// ---------------------------------------------------------------------------

export interface PayrollEmptyHistoryProps {
  className?: string;
}

/**
 * PayrollEmptyHistory — shown to an agent who has no payslip history yet.
 * Variant: fresh (primary-soft tint, wallet icon override).
 * Frame: `.pen` `sMENT` (EmptyFresh override inside dRfK9).
 */
export function PayrollEmptyHistory({ className }: PayrollEmptyHistoryProps) {
  const { t } = useTranslation('payroll');

  return (
    <EmptyState
      variant="fresh"
      title={t('states.emptyHistoryTitle')}
      description={t('states.emptyHistoryBody')}
      className={className}
    />
  );
}

// ---------------------------------------------------------------------------
// 2. PayrollEmptyFiltered
//    .pen: EmptyFilteredZero (BNr4w) override.
//    Copy: "Tidak ada payroll cocok" / "Tidak ada payslip pada filter aktif…"
// ---------------------------------------------------------------------------

export interface PayrollEmptyFilteredProps {
  className?: string;
}

/**
 * PayrollEmptyFiltered — shown in the payroll archive when the active
 * Year / Month / search filter combination returns no results.
 * Frame: `.pen` `XdGri` (EmptyFilteredZero override inside dRfK9).
 */
export function PayrollEmptyFiltered({ className }: PayrollEmptyFilteredProps) {
  const { t } = useTranslation('payroll');

  return (
    <EmptyState
      variant="filtered"
      title={t('states.emptyFilteredTitle')}
      description={t('states.emptyFilteredBody')}
      className={className}
    />
  );
}

// ---------------------------------------------------------------------------
// 3. PayrollAccessDenied
//    .pen: EmptyNoPermission (MRbzz) override.
//    Copy: "Akses ditolak" / "Arsip payroll hanya untuk HR…" / "Hubungi HR…"
// ---------------------------------------------------------------------------

export interface PayrollAccessDeniedProps {
  className?: string;
}

/**
 * PayrollAccessDenied — shown when a non-HR / shift-leader user navigates to
 * the payroll archive or a payslip detail they are not permitted to see.
 * RBAC: PA-2, INV-4. Gherkin: agent-deny, shift-leader/agent → archive.
 * Frame: `.pen` `wmJUD` (EmptyNoPermission override inside dRfK9).
 *
 * Access attempt is recorded in the audit log server-side; this block surfaces
 * the denial copy including that note to inform the user their attempt is audited.
 */
export function PayrollAccessDenied({ className }: PayrollAccessDeniedProps) {
  const { t } = useTranslation('payroll');

  return (
    <EmptyState
      variant="no-permission"
      title={t('states.accessDeniedTitle')}
      description={t('states.accessDeniedBody')}
      hint={t('states.accessDeniedHint')}
      className={className}
    />
  );
}
