// profile-edit — E2 instant self-edit (frame n465cT, EPICS §8 E11 2026-06-14).
//
// Profile change-requests were REMOVED: every editable field is applied INSTANTLY via
// PATCH /me/profile (useUpdateMyProfile). There is no approval tier and no
// "menunggu persetujuan" copy. We assert:
//   - editing fields + Simpan calls useUpdateMyProfile ONCE with the changed fields
//   - an invalid phone shows inline validation and blocks submit
//   - NO "menunggu persetujuan" / pending-approval text anywhere
import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import i18n from '../../src/lib/i18n';
import { withProviders } from '../../src/test/test-utils';

const mockUpdateMutate = jest.fn().mockResolvedValue({});
const mockBack = jest.fn();

jest.mock('expo-router', () => ({
  useRouter: () => ({ back: mockBack, push: jest.fn() }),
}));

// Session provider — supply the logged-in agent's employee_id without loading the real provider
// (which pulls @swp/api-client + /e1).
jest.mock('../../src/providers/session', () => ({
  useSession: () => ({ user: { employee_id: 'emp-1' }, status: 'authed' }),
}));

const mockEmployee = {
  id: 'emp-1',
  full_name: 'Budi Santoso',
  nik: '3201010101900001',
  phone: '+628111111111',
  address: 'Jl. Mawar 1',
  emergency_contact: { name: 'Siti', phone: '+628122222222' },
  bank_account: {
    bank_name: 'BCA',
    account_number: '1234567890',
    account_holder_name: 'Budi Santoso',
  },
  app_language: 'id',
};

jest.mock('@swp/api-client/e2', () => ({
  useGetEmployee: () => ({ data: { data: mockEmployee }, isLoading: false, isError: false }),
  useUpdateMyProfile: () => ({ mutateAsync: mockUpdateMutate, isPending: false }),
}));

// Suppress the success Alert.
jest.spyOn(require('react-native').Alert, 'alert').mockImplementation(() => {});

import ProfileEditScreen from '../profile-edit';

beforeEach(() => {
  mockUpdateMutate.mockClear();
  mockBack.mockClear();
});

describe('profile-edit — instant self-edit', () => {
  it('saving changed fields calls useUpdateMyProfile exactly once with those fields', async () => {
    await render(<ProfileEditScreen />, { wrapper: withProviders() });

    await fireEvent.changeText(screen.getByTestId('profile-phone'), '+628199999999');
    await fireEvent.changeText(screen.getByTestId('profile-emergency-name'), 'Rina');
    await fireEvent.changeText(screen.getByTestId('profile-bank-name'), 'Mandiri');
    await fireEvent.changeText(screen.getByTestId('profile-address'), 'Jl. Melati 5');

    await fireEvent.press(screen.getByTestId('profile-save'));

    await waitFor(() => expect(mockUpdateMutate).toHaveBeenCalledTimes(1));
    const arg = mockUpdateMutate.mock.calls[0][0];
    expect(arg.data.phone).toBe('+628199999999');
    expect(arg.data.address).toBe('Jl. Melati 5');
    expect(arg.data.emergency_contact).toEqual({ name: 'Rina', phone: '+628122222222' });
    expect(arg.data.bank_account.bank_name).toBe('Mandiri');
  });

  it('changing language is part of the instant patch body', async () => {
    await render(<ProfileEditScreen />, { wrapper: withProviders() });
    await fireEvent.press(screen.getByText(i18n.t('m:profileEdit.langEn')));
    await fireEvent.press(screen.getByTestId('profile-save'));
    await waitFor(() => expect(mockUpdateMutate).toHaveBeenCalledTimes(1));
    expect(mockUpdateMutate.mock.calls[0][0].data.app_language).toBe('en');
  });

  it('an invalid phone shows inline validation and blocks submit', async () => {
    await render(<ProfileEditScreen />, { wrapper: withProviders() });
    await fireEvent.changeText(screen.getByTestId('profile-phone'), '0812-not-e164');
    // Inline error renders (phone is the unique login identifier — D2).
    expect(await screen.findByText(i18n.t('m:profileEdit.phoneInvalid'))).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId('profile-save'));
    expect(mockUpdateMutate).not.toHaveBeenCalled();
  });

  it('renders NO "menunggu persetujuan" / pending-approval copy (instant edit)', async () => {
    await render(<ProfileEditScreen />, { wrapper: withProviders() });
    expect(screen.queryByText(/menunggu persetujuan/i)).toBeNull();
    expect(screen.queryByText(/persetujuan/i)).toBeNull();
    expect(screen.queryByText(/awaiting approval/i)).toBeNull();
  });
});
