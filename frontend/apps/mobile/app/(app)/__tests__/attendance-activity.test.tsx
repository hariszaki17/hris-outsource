// Attendance Activity Log (E5 F5.8) — mobile clock-out gate + activity CRUD.
//
// Integration test over the Absen screen with all of its data/mutation hooks (e3/e4/e5),
// expo-* native modules, and the session/capture/location libs mocked. We assert the F5.8
// behavior layered onto the existing clock screen:
//   - TC-9  delete own activity while open → calls useDeleteAttendanceActivity with {id, activityId}
//   - TC-3  clock-out blocked when the open record has 0 activities → guard Alert, NO clock-out call
//   - TC-4  clock-out succeeds when ≥1 activity exists → calls useClockOut
//   - server 422 ACTIVITY_REQUIRED is surfaced (the server is the real gate; client guard is UX)
//   - adding a note calls useCreateAttendanceActivity with {id, data:{note}}
//
// jest hoists jest.mock() above imports; only `mock`-prefixed vars may be referenced in a factory.
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import i18n from '../../../src/lib/i18n';
import { withProviders } from '../../../src/test/test-utils';

// ── mocks ───────────────────────────────────────────────────────────────────

jest.mock('expo-router', () => ({
  useRouter: () => ({ push: jest.fn(), back: jest.fn() }),
}));

jest.mock('expo-location', () => ({
  getForegroundPermissionsAsync: jest.fn().mockResolvedValue({ status: 'granted' }),
}));

jest.mock('expo-linking', () => ({ openSettings: jest.fn() }));

// Device GPS — always resolves to the site centroid so the geofence pill is "inside".
jest.mock('../../../src/lib/location', () => ({
  getCurrentCoords: jest.fn().mockResolvedValue({ lat: -6.2, lng: 106.8 }),
}));

jest.mock('../../../src/lib/capture', () => ({
  captureClockInPhoto: jest.fn().mockResolvedValue({ status: 'cancelled' }),
}));

jest.mock('../../../src/providers/session', () => ({
  useSession: () => ({ user: { employee_id: 'emp-1' }, status: 'authed' }),
}));

// usePullToRefresh returns a live <RefreshControl> element via a lazy RN getter that throws
// if it re-evaluates after the Jest env tears down (the screen's GPS setState can trigger a
// stray re-render). It is irrelevant to F5.8 — stub it to a no-op control.
jest.mock('../../../src/ui/usePullToRefresh', () => ({
  usePullToRefresh: () => undefined,
}));

// Plain safe-area mock (insets only). The global jest.setup mock re-exposes the lib's
// SafeAreaProvider, which nativewind's css-interop tries to "hijack" during commit — on this
// heavy screen that hijack hits an undefined component type and crashes the tree. The screen
// only needs useSafeAreaInsets, so provide a minimal stub with no hijack surface.
jest.mock('react-native-safe-area-context', () => ({
  useSafeAreaInsets: () => ({ top: 0, bottom: 0, left: 0, right: 0 }),
  SafeAreaProvider: ({ children }: { children: unknown }) => children,
  SafeAreaView: ({ children }: { children: unknown }) => children,
}));

class MockApiError extends Error {
  status: number;
  code?: string;
  fields?: Record<string, unknown>;
  constructor(status: number, code?: string, fields?: Record<string, unknown>) {
    super(code ?? 'ApiError');
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}
jest.mock('@swp/api-client', () => ({ ApiError: MockApiError }));

// E3 — active placement so the screen resolves "hasPlacement". We deliberately omit
// `site_geofence`: with no geofence the screen's GPS-distance effect bails early (it requires
// a geofence), so there is no on-mount async `getCurrentCoords()` re-render to leak across
// tests. The geofence pill / distance is not what F5.8 exercises.
jest.mock('@swp/api-client/e3', () => ({
  useGetMyPlacement: () => ({
    data: {
      data: {
        placement: {
          client_company_name: 'PT Klien',
          site_name: 'Menara A',
          site_geofence: null,
        },
      },
    },
    isLoading: false,
    isError: false,
  }),
}));

// E4 — weekly schedule (empty is fine; not under test here).
jest.mock('@swp/api-client/e4', () => ({
  useGetScheduleByAgent: () => ({ data: { data: { data: [] } }, isLoading: false, isError: false }),
}));

// E5 — attendance list + clock mutations + the F5.8 activity hooks. The open record and the
// activity list are swapped per-test via the module-level `mock*` vars.
const mockClockOut = jest.fn().mockResolvedValue({});
const mockCreateActivity = jest.fn().mockResolvedValue({});
const mockDeleteActivity = jest.fn().mockResolvedValue({});
let mockAttendanceItems: unknown[] = [];
let mockActivities: unknown[] = [];

jest.mock('@swp/api-client/e5', () => ({
  useListAttendance: () => ({
    data: { data: { data: mockAttendanceItems } },
    isLoading: false,
    isError: false,
  }),
  useListAttendanceActivities: () => ({
    data: { data: { data: mockActivities, has_more: false } },
    isLoading: false,
    isError: false,
  }),
  useClockIn: () => ({ mutateAsync: jest.fn(), isPending: false }),
  useClockOut: () => ({ mutateAsync: mockClockOut, isPending: false }),
  useCreateAttendanceActivity: () => ({ mutateAsync: mockCreateActivity, isPending: false }),
  useDeleteAttendanceActivity: () => ({ mutateAsync: mockDeleteActivity, isPending: false }),
  useUploadAttendancePhoto: () => ({ mutateAsync: jest.fn(), isPending: false }),
}));

// Capture the Alert.alert calls so the guard/error copy can be asserted.
const mockAlert = jest.spyOn(require('react-native').Alert, 'alert').mockImplementation(() => {});

// An OPEN attendance record (clocked in, not out, checkout window open).
function openRecord(over: Record<string, unknown> = {}) {
  const now = new Date();
  return {
    id: 'SWP-ATT-1',
    check_in_at: now.toISOString(),
    check_out_at: null,
    can_check_out: true,
    shift_start_at: now.toISOString(),
    shift_end_at: now.toISOString(),
    company_name: 'PT Klien',
    site_name: 'Menara A',
    ...over,
  };
}

function activity(over: Record<string, unknown> = {}) {
  return {
    id: 'SWP-ACT-1',
    attendance_id: 'SWP-ATT-1',
    employee_id: 'emp-1',
    note: 'Patroli lantai 1',
    recorded_at: '2026-06-18T07:30:00Z',
    created_at: '2026-06-18T07:30:00Z',
    ...over,
  };
}

// Required (not import) AFTER the mocks so the screen binds the mocked `@swp/api-client`
// ApiError at module-eval time — a hoisted ESM import would evaluate the screen before the
// mock factory and leave `ApiError` undefined (breaks the `e instanceof ApiError` branch).
const AttendanceScreen = require('../attendance').default;

async function renderSettled() {
  let utils: ReturnType<typeof render> | undefined;
  // Render AND settle the on-mount permission/GPS effects inside ONE act scope. Rendering
  // outside act then flushing in a second act() nests/overlaps with React 19's render act and
  // crashes nativewind's css-interop commit hook; doing both in a single act keeps the commit
  // contained and the act env clean for later tests.
  await act(async () => {
    utils = render(<AttendanceScreen />, { wrapper: withProviders() });
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
  });
  if (!utils) throw new Error('render did not run');
  return utils;
}

// The screen runs a 1-second live-clock `setInterval` (setNow). Left real, its callback fires
// after the Jest env tears down ("import after env torn down") and keeps the worker alive.
// Stub setInterval to a no-op for this suite — the live clock is irrelevant to F5.8 — so no
// timer handle is ever created and RNTL's auto-cleanup fully tears the tree down between tests.
let realSetInterval: typeof global.setInterval;
beforeAll(() => {
  realSetInterval = global.setInterval;
  // @ts-expect-error test stub — returns a dummy handle id.
  global.setInterval = () => 0 as unknown as ReturnType<typeof setInterval>;
});
afterAll(() => {
  global.setInterval = realSetInterval;
});

beforeEach(() => {
  mockAttendanceItems = [];
  mockActivities = [];
  mockClockOut.mockClear();
  mockCreateActivity.mockClear();
  mockDeleteActivity.mockClear();
  mockAlert.mockClear();
});

afterEach(() => {
  cleanup();
});

describe('attendance activity log — F5.8', () => {
  it('shows the activity panel + an existing note while the record is open', async () => {
    mockAttendanceItems = [openRecord()];
    mockActivities = [activity({ note: 'Patroli lantai 1' })];
    await renderSettled();

    expect(await screen.findByText(i18n.t('m:activity.title'))).toBeOnTheScreen();
    expect(screen.getByText('Patroli lantai 1')).toBeOnTheScreen();
  });

  it('shows the empty state when the open record has no activities', async () => {
    mockAttendanceItems = [openRecord()];
    mockActivities = [];
    await renderSettled();

    expect(await screen.findByText(i18n.t('m:activity.empty'))).toBeOnTheScreen();
  });

  it('adding a note calls useCreateAttendanceActivity with {id, data:{note}} (trimmed)', async () => {
    mockAttendanceItems = [openRecord()];
    mockActivities = [];
    await renderSettled();

    const input = await screen.findByTestId('activity-input');
    await act(async () => {
      fireEvent.changeText(input, '  Cek APAR  ');
    });
    // Drive the press + its full async handler (mutateAsync → setNoteDraft) inside act so the
    // trailing state update is flushed within the act window (no stray act warning).
    await act(async () => {
      fireEvent.press(screen.getByTestId('activity-add'));
    });

    // note is trimmed (AA-6) before send.
    expect(mockCreateActivity).toHaveBeenCalledWith({
      id: 'SWP-ATT-1',
      data: { note: 'Cek APAR' },
    });
  });

  it('TC-9 deleting an activity calls useDeleteAttendanceActivity with {id, activityId}', async () => {
    mockAttendanceItems = [openRecord()];
    mockActivities = [activity({ id: 'SWP-ACT-9' })];
    await renderSettled();

    fireEvent.press(await screen.findByTestId('activity-delete-SWP-ACT-9'));

    await waitFor(() =>
      expect(mockDeleteActivity).toHaveBeenCalledWith({
        id: 'SWP-ATT-1',
        activityId: 'SWP-ACT-9',
      }),
    );
  });

  it('TC-3 clock-out is blocked (no mutation) when the open record has 0 activities', async () => {
    mockAttendanceItems = [openRecord()];
    mockActivities = [];
    await renderSettled();

    // The action button is "Clock Out" while open.
    fireEvent.press(screen.getByTestId('clock-action'));

    // Guard Alert shown with the activity-required copy; clock-out never called.
    await waitFor(() =>
      expect(mockAlert).toHaveBeenCalledWith(
        i18n.t('m:activity.title'),
        i18n.t('m:activity.required'),
      ),
    );
    expect(mockClockOut).not.toHaveBeenCalled();
  });

  it('TC-4 clock-out succeeds (calls useClockOut) when ≥1 activity exists', async () => {
    mockAttendanceItems = [openRecord()];
    mockActivities = [activity()];
    await renderSettled();

    await act(async () => {
      fireEvent.press(screen.getByTestId('clock-action'));
    });

    expect(mockClockOut).toHaveBeenCalledTimes(1);
  });

  it('TC-3 server 422 ACTIVITY_REQUIRED is surfaced even if the client guard passes', async () => {
    // Client sees 1 activity (guard passes) but the server rejects (e.g. stale/raced delete).
    mockAttendanceItems = [openRecord()];
    mockActivities = [activity()];
    mockClockOut.mockRejectedValueOnce(
      new MockApiError(422, 'ACTIVITY_REQUIRED', { activity_count: '0' }),
    );
    await renderSettled();

    await act(async () => {
      fireEvent.press(screen.getByTestId('clock-action'));
    });

    expect(mockClockOut).toHaveBeenCalledTimes(1);
    // Server is the real gate: the 422 ACTIVITY_REQUIRED is caught and surfaced as the Alert.
    expect(mockAlert).toHaveBeenCalledWith(
      i18n.t('m:activity.title'),
      i18n.t('m:activity.required'),
    );
  });

  it('the activity panel is hidden when there is no open record', async () => {
    mockAttendanceItems = []; // no open record → pre-clock state
    await renderSettled();

    expect(screen.queryByText(i18n.t('m:activity.title'))).toBeNull();
    expect(screen.queryByTestId('activity-input')).toBeNull();
  });
});
