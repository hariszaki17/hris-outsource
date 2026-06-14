// SL Persetujuan + Verifikasi (DxK66 · viUFF · F11.3/F11.2 + preserved E5 F5.3).
//
// Integration test over the screen with the E11 + E5 data/mutation hooks mocked. We assert:
//   - 2 pending instances render as cards with the "Baris N / M" chain pill
//   - Setujui opens the sheet; approving calls useApproveApprovalInstance with the instance id
//   - Reject path calls useRejectApprovalInstance with the typed reason
//   - empty list → empty state
//   - the preserved "Kehadiran" (E5) segment toggle still renders + switches
//
// The @swp/api-client/* barrels pull the generated axios+msw client; we mock them entirely and
// supply the wire enums inline (tiny, stable) so the screen's value-level enum reads still work.
import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import i18n from '../../../src/lib/i18n';
import { withProviders } from '../../../src/test/test-utils';

// ── mocks ───────────────────────────────────────────────────────────────────
// jest hoists jest.mock() above imports; only `mock`-prefixed vars may be referenced in a factory.
const mockApproveMutate = jest.fn().mockResolvedValue({});
const mockRejectMutate = jest.fn().mockResolvedValue({});
let mockInstances: unknown[] = [];

jest.mock('@swp/api-client', () => ({
  ApiError: class ApiError extends Error {
    status: number;
    code?: string;
    constructor(status: number, code?: string) {
      super(code ?? 'ApiError');
      this.status = status;
      this.code = code;
    }
  },
}));

jest.mock('@swp/api-client/e11', () => ({
  RequestType: { LEAVE: 'LEAVE', OVERTIME: 'OVERTIME' },
  InstanceStatus: { PENDING: 'PENDING', APPROVED: 'APPROVED', REJECTED: 'REJECTED' },
  useListApprovalInstances: () => ({
    data: { data: { data: mockInstances } },
    isLoading: false,
    isError: false,
  }),
  useApproveApprovalInstance: () => ({ mutateAsync: mockApproveMutate, isPending: false }),
  useRejectApprovalInstance: () => ({ mutateAsync: mockRejectMutate, isPending: false }),
}));

// E5 attendance section — no pending records (keeps the Kehadiran section to its empty state).
jest.mock('@swp/api-client/e5', () => ({
  useListAttendance: () => ({ data: { data: { data: [] } }, isLoading: false, isError: false }),
  useVerifyAttendance: () => ({ mutateAsync: jest.fn(), isPending: false }),
  useRejectAttendance: () => ({ mutateAsync: jest.fn(), isPending: false }),
  useBulkVerifyAttendance: () => ({ mutateAsync: jest.fn(), isPending: false }),
}));

// Silence the success/error Alert.alert toasts.
jest.spyOn(require('react-native').Alert, 'alert').mockImplementation(() => {});

// Imported AFTER the mocks are registered.
import SlVerifikasiScreen from '../sl-verifikasi';

function makeInstance(over: Record<string, unknown> = {}) {
  return {
    id: 'inst-1',
    request_type: 'LEAVE',
    request_id: 'SWP-LR-1042',
    company_id: 'co-1',
    current_line: 1,
    line_count: 2,
    status: 'PENDING',
    requester_id: 'Budi Santoso',
    summary: 'Cuti Tahunan · 3 hari',
    created_at: new Date().toISOString(),
    ...over,
  };
}

async function renderScreen() {
  return render(<SlVerifikasiScreen />, { wrapper: withProviders() });
}

beforeEach(() => {
  mockInstances = [];
  mockApproveMutate.mockClear();
  mockRejectMutate.mockClear();
});

describe('sl-verifikasi — E11 approvals inbox', () => {
  it('renders a card per pending instance with the Baris N / M chain pill', async () => {
    mockInstances = [
      makeInstance({ id: 'inst-1', request_id: 'SWP-LR-1042' }),
      makeInstance({ id: 'inst-2', request_id: 'SWP-OT-2001', request_type: 'OVERTIME' }),
    ];
    await renderScreen();
    expect(screen.getByTestId('approval-card-inst-1')).toBeOnTheScreen();
    expect(screen.getByTestId('approval-card-inst-2')).toBeOnTheScreen();
    // "Baris 1 / 2" chain pill (current_line=1, line_count=2). Two cards → two pills.
    expect(screen.getAllByText('Baris 1 / 2')).toHaveLength(2);
  });

  it('tapping Setujui opens the sheet and approving calls approve with the instance id', async () => {
    mockInstances = [makeInstance({ id: 'inst-1' })];
    await renderScreen();
    await fireEvent.press(screen.getByTestId('approval-card-inst-1-approve'));
    // Sheet opened in approve mode.
    expect(await screen.findByTestId('sheet-approve-btn')).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId('sheet-approve-btn'));
    await waitFor(() =>
      expect(mockApproveMutate).toHaveBeenCalledWith({ id: 'inst-1', data: { note: undefined } }),
    );
  });

  it('reject path calls reject with the typed reason', async () => {
    mockInstances = [makeInstance({ id: 'inst-1' })];
    await renderScreen();
    await fireEvent.press(screen.getByTestId('approval-card-inst-1-reject'));
    expect(await screen.findByTestId('sheet-reason-input')).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByTestId('sheet-reason-input'), 'kuota habis');
    await fireEvent.press(screen.getByTestId('sheet-reject-btn'));
    await waitFor(() =>
      expect(mockRejectMutate).toHaveBeenCalledWith({
        id: 'inst-1',
        data: { reason: 'kuota habis' },
      }),
    );
  });

  it('shows the empty state when there are no pending instances', async () => {
    mockInstances = [];
    await renderScreen();
    expect(screen.getByText(i18n.t('m:approvals.emptyTitle'))).toBeOnTheScreen();
  });
});

describe('sl-verifikasi — preserved E5 attendance segment', () => {
  it('renders both segment toggles and switches to the Kehadiran (E5) queue', async () => {
    mockInstances = [makeInstance()];
    await renderScreen();
    expect(screen.getByTestId('segment-approvals')).toBeOnTheScreen();
    const attendanceSeg = screen.getByTestId('segment-attendance');
    expect(attendanceSeg).toBeOnTheScreen();
    await fireEvent.press(attendanceSeg);
    // E5 section empty-state copy proves we switched to the attendance queue.
    expect(await screen.findByText(i18n.t('m:slVerif.empty'))).toBeOnTheScreen();
  });
});
