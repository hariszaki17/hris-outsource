/**
 * E8 Payroll (read-only) / Payroll — public hook surface (`@swp/api-client/e8`). Hand-authored
 * barrel over the Orval `tags-split` output (no root barrel emitted), outside `src/gen` so regen
 * does not wipe it. Typed react-query hooks + MSW mocks; Zod deferred (WEB-STACK §4).
 */
export * from './gen/e8/payslips/payslips.ts';
export * from './gen/e8/payslip-audit-notes/payslip-audit-notes.ts';
export * from './gen/e8/payslip-export/payslip-export.ts';
export * from './gen/e8/model/index.ts';
