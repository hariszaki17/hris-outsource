// ApprovalActionSheet — E11 approve/reject bottom sheet (viUFF · F11.2/F11.3).
//   - approve mode: OPTIONAL note label, Setujui calls onApprove(note)
//   - reject mode: REQUIRED reason label, Tolak blocked until non-empty, then onReject(reason)
//   - mode toggle: Tolak (in approve) → reject mode; Setujui (in reject) → approve mode
//   - chain progress: done node(s) before the current node
//
// NOTE — RNTL v14: both `render` and `fireEvent.*` are async (they wrap their work in act()).
// Every call is awaited so act() scopes don't overlap.
import { fireEvent, render, screen } from '@testing-library/react-native';
import i18n from '../../lib/i18n';
import {
  ApprovalActionSheet,
  type ApprovalSheetMode,
  type ApprovalSheetTarget,
} from '../ApprovalActionSheet';

const TARGET: ApprovalSheetTarget = {
  summaryLine: 'Cuti Tahunan · 3 hari',
  detailLine: 'Budi Santoso · SWP-EMP-1042 · 12–14 Jul 2026',
  currentLine: 2,
  lineCount: 3,
};

async function setup(
  mode: ApprovalSheetMode,
  overrides: Partial<Parameters<typeof ApprovalActionSheet>[0]> = {},
) {
  const onApprove = jest.fn();
  const onReject = jest.fn();
  const onModeChange = jest.fn();
  const onClose = jest.fn();
  await render(
    <ApprovalActionSheet
      mode={mode}
      target={TARGET}
      submitting={false}
      onApprove={onApprove}
      onReject={onReject}
      onModeChange={onModeChange}
      onClose={onClose}
      {...overrides}
    />,
  );
  return { onApprove, onReject, onModeChange, onClose };
}

describe('ApprovalActionSheet — approve mode', () => {
  it('shows the OPTIONAL note label and the approve title', async () => {
    await setup('approve');
    expect(screen.getByText(i18n.t('m:approvals.sheetTitleApprove'))).toBeOnTheScreen();
    expect(screen.getByText(i18n.t('m:approvals.noteLabel'))).toBeOnTheScreen();
    expect(screen.getByTestId('sheet-note-input')).toBeOnTheScreen();
  });

  it('Setujui submits with the typed note via onApprove', async () => {
    const { onApprove } = await setup('approve');
    await fireEvent.changeText(screen.getByTestId('sheet-note-input'), 'lihat dokumen');
    await fireEvent.press(screen.getByTestId('sheet-approve-btn'));
    expect(onApprove).toHaveBeenCalledTimes(1);
    expect(onApprove).toHaveBeenCalledWith('lihat dokumen');
  });

  it('Setujui with empty note submits undefined (note is optional)', async () => {
    const { onApprove } = await setup('approve');
    await fireEvent.press(screen.getByTestId('sheet-approve-btn'));
    expect(onApprove).toHaveBeenCalledWith(undefined);
  });

  it('tapping Tolak in approve mode switches to reject mode (does not reject)', async () => {
    const { onModeChange, onReject } = await setup('approve');
    await fireEvent.press(screen.getByTestId('sheet-reject-btn'));
    expect(onModeChange).toHaveBeenCalledWith('reject');
    expect(onReject).not.toHaveBeenCalled();
  });
});

describe('ApprovalActionSheet — reject mode', () => {
  it('shows the REQUIRED reason label and the reject title', async () => {
    await setup('reject');
    expect(screen.getByText(i18n.t('m:approvals.sheetTitleReject'))).toBeOnTheScreen();
    expect(screen.getByText(i18n.t('m:approvals.reasonLabel'))).toBeOnTheScreen();
    expect(screen.getByTestId('sheet-reason-input')).toBeOnTheScreen();
  });

  it('Tolak is guarded while the reason is empty (no onReject)', async () => {
    const { onReject } = await setup('reject');
    await fireEvent.press(screen.getByTestId('sheet-reject-btn'));
    expect(onReject).not.toHaveBeenCalled();
  });

  it('Tolak submits the trimmed reason once it is non-empty', async () => {
    const { onReject } = await setup('reject');
    await fireEvent.changeText(screen.getByTestId('sheet-reason-input'), '  kuota habis  ');
    await fireEvent.press(screen.getByTestId('sheet-reject-btn'));
    expect(onReject).toHaveBeenCalledTimes(1);
    expect(onReject).toHaveBeenCalledWith('kuota habis');
  });

  it('tapping Setujui in reject mode switches back to approve mode', async () => {
    const { onModeChange, onApprove } = await setup('reject');
    await fireEvent.press(screen.getByTestId('sheet-approve-btn'));
    expect(onModeChange).toHaveBeenCalledWith('approve');
    expect(onApprove).not.toHaveBeenCalled();
  });
});

describe('ApprovalActionSheet — chain progress', () => {
  it('renders done node(s) before the current node and an upcoming node after', async () => {
    await setup('approve');
    // currentLine=2 of 3 → line 1 done, line 2 current, line 3 upcoming.
    expect(screen.getByTestId('sheet-chain-node-1')).toBeOnTheScreen();
    expect(screen.getByLabelText('chain-node-1-done')).toBeOnTheScreen();
    expect(screen.getByLabelText('chain-node-2-current')).toBeOnTheScreen();
    expect(screen.getByLabelText('chain-node-3-upcoming')).toBeOnTheScreen();
  });
});
