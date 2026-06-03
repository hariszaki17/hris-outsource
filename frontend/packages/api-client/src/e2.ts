/**
 * E2 Identity / Karyawan & Master Data — public hook surface (`@swp/api-client/e2`).
 *
 * Hand-authored barrel over the Orval `tags-split` output (one file per tag, no root barrel).
 * Lives OUTSIDE `src/gen` so `clean: true` regeneration never wipes it. Generated files stay
 * untouched (ENGINEERING.md E2).
 */
export * from './gen/e2/employees/employees.ts';
export * from './gen/e2/agreements/agreements.ts';
export * from './gen/e2/client-companies/client-companies.ts';
export * from './gen/e2/sites/sites.ts';
export * from './gen/e2/service-lines-positions/service-lines-positions.ts';
export * from './gen/e2/master-data/master-data.ts';
export * from './gen/e2/model/index.ts';

// The change-request sub-resource ops (`/employees/{id}/change-requests`) are tagged
// `[change-requests, employees]`, so Orval emits identical members into BOTH the employees and
// change-requests files. `employees.ts` (above) is the source for the two shared ops
// (createChangeRequest / listOwnChangeRequests); here we re-export only the members UNIQUE to the
// change-requests tag (approve / reject / get / listPending) to avoid duplicate-export ambiguity.
export {
  approveChangeRequest,
  getApproveChangeRequestMutationOptions,
  getApproveChangeRequestUrl,
  useApproveChangeRequest,
  rejectChangeRequest,
  getRejectChangeRequestMutationOptions,
  getRejectChangeRequestUrl,
  useRejectChangeRequest,
  getChangeRequest,
  getGetChangeRequestQueryKey,
  getGetChangeRequestQueryOptions,
  getGetChangeRequestUrl,
  useGetChangeRequest,
  listPendingChangeRequests,
  getListPendingChangeRequestsQueryKey,
  getListPendingChangeRequestsQueryOptions,
  getListPendingChangeRequestsUrl,
  useListPendingChangeRequests,
} from './gen/e2/change-requests/change-requests.ts';
