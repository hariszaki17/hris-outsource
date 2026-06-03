import { defineConfig } from 'orval';

/**
 * Contract-first codegen. Source of truth = the per-epic openapi.yaml under docs/api
 * (CONVENTIONS.md). Output under src/gen is GENERATED — never hand-edit (ENGINEERING.md E2).
 * Run `pnpm gen`.
 *
 * Specs are self-contained per epic (each redefines ErrorEnvelope, etc.), so we generate
 * one project per epic into its own folder to avoid schema name collisions.
 *
 * NOTE (caveat, WEB-STACK.md §4): epics using oneOf/discriminator (incl. E6) can produce
 * imperfect Zod from Orval on OpenAPI 3.1. We generate Zod for clean specs (E1) and defer
 * Zod for union-heavy ones until validated; react-query + MSW mocks still generate for all.
 */
const SPECS = '../../../docs/api';

const reactQuery = (epic: string, file: string, mock: boolean) => ({
  input: { target: `${SPECS}/${file}/openapi.yaml` },
  output: {
    mode: 'tags-split' as const,
    target: `./src/gen/${epic}/${epic}.ts`,
    schemas: `./src/gen/${epic}/model`,
    client: 'react-query' as const,
    httpClient: 'fetch' as const,
    mock: mock ? { type: 'msw' as const, useExamples: true } : false,
    clean: true,
    override: {
      mutator: { path: './src/mutator.ts', name: 'customFetch' },
      query: { useQuery: true, signal: true },
    },
  },
});

const zod = (epic: string, file: string) => ({
  input: { target: `${SPECS}/${file}/openapi.yaml` },
  output: {
    mode: 'tags-split' as const,
    target: `./src/gen/${epic}/${epic}.zod.ts`,
    client: 'zod' as const,
    fileExtension: '.zod.ts',
    clean: false,
  },
});

export default defineConfig({
  // E1 Foundations — auth, users, audit, platform. No oneOf → full Zod + MSW mocks.
  e1: reactQuery('e1', 'E1-foundations', true),
  'e1-zod': zod('e1', 'E1-foundations'),

  // E2 Identity — employees, change-requests, agreements, client-companies, service-lines/
  // positions, master-data. Typed react-query hooks + MSW mocks. Zod DEFERRED: Orval's zod
  // emitter mis-references a regex-pattern const (`…ServiceLineIdRegExpOne`) on the overtime-rule
  // schemas, producing undefined identifiers (the documented Orval-zod caveat, WEB-STACK §4).
  // Forms hand-author their zod schemas; re-enable when the spec/Orval is reconciled.
  e2: reactQuery('e2', 'E2-identity', true),

  // E3 Placement — placements (+ transfer/renew/end/terminate/resign actions), shift-leader
  // assignments, company roster. Typed hooks + MSW mocks; Zod deferred (1 oneOf union + forms
  // hand-author zod, WEB-STACK §4).
  e3: reactQuery('e3', 'E3-placement', true),

  // E4 Shift Scheduling — shift masters + weekly schedule grid (+ bulk-apply / conflict-check
  // actions). Typed hooks + MSW mocks; Zod deferred (3 unions, WEB-STACK §4).
  e4: reactQuery('e4', 'E4-shift-scheduling', true),

  // E6 Leave — typed react-query hooks only. MSW mocks + Zod DEFERRED: Orval's faker
  // mocks emit `string | undefined` against required fields on E6's union/nullable
  // schemas under strict null checks (the documented oneOf/discriminator caveat,
  // WEB-STACK §4). Hand-author E6 fixtures or post-process before enabling.
  e6: reactQuery('e6', 'E6-leave', false),
});
