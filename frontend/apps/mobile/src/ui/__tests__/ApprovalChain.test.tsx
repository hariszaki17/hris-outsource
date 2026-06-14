// ApprovalChain — pure read-only timeline primitive (F11.3 · IB-4).
// Verifies the per-state node rendering: done=check, rejected=X, current=outlined number,
// upcoming=neutral number. testIDs encode lineNo + state (chain-node-N-state).
import { render, screen } from '@testing-library/react-native';
import '../../lib/i18n';
import { ApprovalChain, type ApprovalChainStep } from '../ApprovalChain';

const STEPS: ApprovalChainStep[] = [
  {
    lineNo: 1,
    state: 'done',
    members: 'Rudi Wijaya · Sari',
    statusLabel: 'Selesai',
    result: 'Disetujui oleh Rudi · 9 Jul 14:20 (OR)',
  },
  {
    lineNo: 2,
    state: 'current',
    members: 'Budi Hartono',
    statusLabel: 'Menunggu',
    result: 'Menunggu keputusan.',
  },
  {
    lineNo: 3,
    state: 'upcoming',
    members: 'Dewi Lestari',
    statusLabel: 'Menunggu',
    result: 'Belum tercapai.',
  },
];

describe('ApprovalChain', () => {
  it('renders one node per step with the correct state encoded in its testID', async () => {
    await render(<ApprovalChain steps={STEPS} />);
    expect(screen.getByTestId('chain-node-1-done')).toBeOnTheScreen();
    expect(screen.getByTestId('chain-node-2-current')).toBeOnTheScreen();
    expect(screen.getByTestId('chain-node-3-upcoming')).toBeOnTheScreen();
  });

  it('renders a row + member names + result line per line', async () => {
    await render(<ApprovalChain steps={STEPS} />);
    expect(screen.getByTestId('chain-line-1')).toBeOnTheScreen();
    expect(screen.getByTestId('chain-line-2')).toBeOnTheScreen();
    expect(screen.getByText('Baris 1')).toBeOnTheScreen();
    expect(screen.getByText('Rudi Wijaya · Sari')).toBeOnTheScreen();
    expect(screen.getByText('Menunggu keputusan.')).toBeOnTheScreen();
  });

  it('renders a rejected node (X) for a rejected line', async () => {
    await render(
      <ApprovalChain
        steps={[
          {
            lineNo: 1,
            state: 'rejected',
            members: 'Budi',
            statusLabel: 'Ditolak',
            result: 'Ditolak oleh Budi · 9 Jul — kuota habis',
          },
        ]}
      />,
    );
    expect(screen.getByTestId('chain-node-1-rejected')).toBeOnTheScreen();
    expect(screen.queryByTestId('chain-node-1-done')).toBeNull();
  });
});
