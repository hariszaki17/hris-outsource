// approval-status — agent read-only approval chain (PGrLa · F11.3 · IB-4 · INV-3).
//
// Mocked useGetApprovalInstance returns a detail with lines + actions; we assert:
//   - a DONE line (OR-clearer) renders with the done node + "Disetujui … (OR)" result
//   - the CURRENT line is highlighted (current node) + "Menunggu" pill
//   - an UPCOMING line renders
//   - the action trail (RIWAYAT) renders submitted + approve rows
//   - the header status pill reflects PENDING / APPROVED / REJECTED
//   - pre-chain state (no approval_instance_id) → query disabled, "belum masuk antrean" empty state
import { render, screen, waitFor } from '@testing-library/react-native';
import i18n from '../../src/lib/i18n';
import { withProviders } from '../../src/test/test-utils';

// expo-router — only useLocalSearchParams (params) + useRouter (back) are used.
let mockParams: Record<string, string | undefined> = {};
jest.mock('expo-router', () => ({
  useLocalSearchParams: () => mockParams,
  useRouter: () => ({ back: jest.fn(), push: jest.fn() }),
}));

let mockQuery: Record<string, unknown> = { data: undefined, isLoading: false, isError: false };
jest.mock('@swp/api-client/e11', () => ({
  RequestType: { LEAVE: 'LEAVE', OVERTIME: 'OVERTIME' },
  InstanceStatus: { PENDING: 'PENDING', APPROVED: 'APPROVED', REJECTED: 'REJECTED' },
  ApprovalActionAction: { APPROVE: 'APPROVE', REJECT: 'REJECT', BYPASS: 'BYPASS' },
  useGetApprovalInstance: () => mockQuery,
}));

// Imported AFTER the mocks.
import ApprovalStatusScreen from '../approval-status';

function detail(over: Record<string, unknown> = {}) {
  return {
    id: 'inst-1',
    request_type: 'LEAVE',
    request_id: 'SWP-LR-1042',
    company_id: 'co-1',
    current_line: 2,
    line_count: 3,
    status: 'PENDING',
    created_at: '2026-07-09T07:00:00Z',
    lines: [
      { id: 'l1', line_no: 1, members: [{ user_id: 'u1', display_name: 'Rudi' }] },
      { id: 'l2', line_no: 2, members: [{ user_id: 'u2', display_name: 'Budi' }] },
      { id: 'l3', line_no: 3, members: [{ user_id: 'u3', display_name: 'Dewi' }] },
    ],
    actions: [
      {
        id: 'a1',
        line_no: 1,
        actor_user_id: 'u1',
        actor_name: 'Rudi',
        action: 'APPROVE',
        created_at: '2026-07-09T07:20:00Z',
      },
    ],
    ...over,
  };
}

beforeEach(() => {
  mockParams = {};
  mockQuery = { data: undefined, isLoading: false, isError: false };
});

describe('approval-status — chain timeline', () => {
  it('renders done / current / upcoming lines + action trail for a PENDING instance', async () => {
    mockParams = { approval_instance_id: 'inst-1', request_type: 'LEAVE' };
    mockQuery = { data: { data: detail() }, isLoading: false, isError: false };
    await render(<ApprovalStatusScreen />, { wrapper: withProviders() });

    // Line 1 cleared (the OR-clearer) → done node + approved result.
    expect(await screen.findByTestId('chain-node-1-done')).toBeOnTheScreen();
    expect(screen.getByText(/Disetujui oleh Rudi/)).toBeOnTheScreen();
    // Line 2 is current (highlighted), line 3 upcoming.
    expect(screen.getByTestId('chain-node-2-current')).toBeOnTheScreen();
    expect(screen.getByTestId('chain-node-3-upcoming')).toBeOnTheScreen();
    // RIWAYAT trail — submitted + "Baris 1 disetujui".
    expect(screen.getByText(i18n.t('m:approvalStatus.trailHeader'))).toBeOnTheScreen();
    expect(screen.getByText(/Baris 1 disetujui/)).toBeOnTheScreen();
  });

  it('header pill shows Menunggu · Baris X/Y for PENDING', async () => {
    mockParams = { approval_instance_id: 'inst-1' };
    mockQuery = { data: { data: detail({ status: 'PENDING' }) }, isLoading: false, isError: false };
    await render(<ApprovalStatusScreen />, { wrapper: withProviders() });
    expect(
      await screen.findByText(i18n.t('m:approvalStatus.statusPending', { cur: 2, total: 3 })),
    ).toBeOnTheScreen();
  });

  it('header pill shows Disetujui for an APPROVED instance', async () => {
    mockParams = { approval_instance_id: 'inst-1' };
    mockQuery = {
      data: { data: detail({ status: 'APPROVED', current_line: 3 }) },
      isLoading: false,
      isError: false,
    };
    await render(<ApprovalStatusScreen />, { wrapper: withProviders() });
    expect(await screen.findByText(i18n.t('m:approvalStatus.statusApproved'))).toBeOnTheScreen();
  });

  it('header pill shows Ditolak + a rejected line for a REJECTED instance', async () => {
    mockParams = { approval_instance_id: 'inst-1' };
    mockQuery = {
      data: {
        data: detail({
          status: 'REJECTED',
          actions: [
            {
              id: 'a1',
              line_no: 1,
              actor_user_id: 'u1',
              actor_name: 'Rudi',
              action: 'REJECT',
              reason: 'kuota habis',
              created_at: '2026-07-09T07:20:00Z',
            },
          ],
        }),
      },
      isLoading: false,
      isError: false,
    };
    await render(<ApprovalStatusScreen />, { wrapper: withProviders() });
    // "Ditolak" appears on the header pill AND the rejected line label.
    expect(await screen.findByTestId('chain-node-1-rejected')).toBeOnTheScreen();
    expect(
      screen.getAllByText(i18n.t('m:approvalStatus.statusRejected')).length,
    ).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/kuota habis/)).toBeOnTheScreen();
  });
});

describe('approval-status — pre-chain state', () => {
  it('shows "belum masuk antrean" empty state when no approval_instance_id param', async () => {
    mockParams = {}; // query disabled (hasInstance=false)
    await render(<ApprovalStatusScreen />, { wrapper: withProviders() });
    await waitFor(() =>
      expect(screen.getByText(i18n.t('m:approvalStatus.preChainTitle'))).toBeOnTheScreen(),
    );
    // No chain nodes rendered in the pre-chain state.
    expect(screen.queryByTestId('chain-node-1-done')).toBeNull();
  });
});
