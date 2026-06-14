/**
 * E11 Approvals — public hook surface (`@swp/api-client/e11`). Hand-authored barrel over the
 * Orval `tags-split` output (no root barrel emitted), outside `src/gen` so regen does not wipe it.
 * Configurable per-company multi-line approval engine; single source of truth for leave/overtime
 * approval. Typed react-query hooks + MSW mocks; Zod deferred (WEB-STACK §4).
 */
export * from './gen/e11/approval-templates/approval-templates.ts';
export * from './gen/e11/approval-instances/approval-instances.ts';
export * from './gen/e11/model/index.ts';
