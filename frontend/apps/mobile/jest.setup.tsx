// Jest setup (setupFilesAfterEach). Runs before every test file.
//
// 1. i18n — initialize the real mobile catalog so t('m:approvals.setujui') etc. resolve to the
//    actual Bahasa/English copy the screens render. Tests assert on visible copy, so we want the
//    real strings, not key passthrough. DEFAULT_LOCALE is Bahasa ('id').
// 2. Native-module mocks — safe-area-context ships a jest mock; lucide icons render fine as RN
//    Views under the jest-expo preset. We stub the bits that touch real native bridges.
import '@testing-library/react-native'; // RNTL v14 ships its own matchers (toBeOnTheScreen, …)
import './src/lib/i18n';

// react-native-safe-area-context ships a first-party jest mock (fixed insets, no native call),
// but it exports the API under `default`. Re-expose it as named exports so the screens'
// `import { useSafeAreaInsets, SafeAreaView } from 'react-native-safe-area-context'` resolves.
jest.mock('react-native-safe-area-context', () => {
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const mock = require('react-native-safe-area-context/jest/mock').default;
  const { View } = require('react-native');
  return {
    ...mock,
    // The Screen primitive uses <SafeAreaView>; render it as a plain View in tests.
    SafeAreaView: ({ children }: { children: unknown }) => <View>{children}</View>,
  };
});

// Silence the act()/animation noise that RN emits in the jsdom-free test env; keep real errors.
const origWarn = console.warn;
beforeAll(() => {
  jest.spyOn(console, 'warn').mockImplementation((...args: unknown[]) => {
    const msg = String(args[0] ?? '');
    if (msg.includes('useNativeDriver') || msg.includes('VirtualizedList')) return;
    origWarn(...(args as []));
  });
});
