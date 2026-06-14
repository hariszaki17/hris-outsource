/**
 * E6 Leave — public hook surface (`@swp/api-client/e6`). Hand-authored barrel over the Orval
 * `tags-split` output (no root barrel emitted), outside `src/gen` so regen does not wipe it.
 * (E6 generates typed react-query hooks only; MSW + Zod deferred — WEB-STACK §4.)
 *
 * Per-type quota ledger (2026-06-12):
 *   useGetEmployeeTypeBalances, useAdjustTypeQuota (leave-balances tag)
 */
export * from './gen/e6/leave-types/leave-types.ts';
export * from './gen/e6/leave-balances/leave-balances.ts';
export * from './gen/e6/leave-requests/leave-requests.ts';
export * from './gen/e6/leave-calendar/leave-calendar.ts';
export * from './gen/e6/model/index.ts';
