import { render, screen } from '@testing-library/react-native';
import i18n from '../lib/i18n';
import { Text } from '../ui/Text';

describe('jest harness smoke', () => {
  it('renders an RN Text via the design-system Text component', async () => {
    await render(<Text testID="smoke">hello</Text>);
    expect(screen.getByTestId('smoke')).toHaveTextContent('hello');
  });

  it('resolves mobile i18n copy', () => {
    expect(i18n.t('m:approvals.setujui')).toBe('Setujui');
  });
});
