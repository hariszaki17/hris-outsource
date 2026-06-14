// Jest config for the mobile app (Expo + React Native). Component/integration tests for the
// E11 Approvals screens live next to the screens as *.test.tsx (and under __tests__/).
//
// Preset `jest-expo` wires the RN/Expo transform, mocks, and jsdom-free RN environment.
// We extend transformIgnorePatterns so our workspace packages (@swp/*) and the RN/Expo ESM
// modules are transformed (node_modules ship untranspiled ESM that Jest can't parse raw).
/** @type {import('jest').Config} */
module.exports = {
  preset: 'jest-expo',
  setupFilesAfterEnv: ['<rootDir>/jest.setup.tsx'],
  // jest-expo's default ignore allowlists a fixed set of RN/Expo scopes. Append our monorepo
  // workspace packages (@swp/*) plus the libs the screens import, so they get Babel-transformed.
  transformIgnorePatterns: [
    'node_modules/(?!((jest-)?react-native|@react-native(-community)?|expo(nent)?|@expo(nent)?/.*|@expo-google-fonts/.*|react-navigation|@react-navigation/.*|@unimodules/.*|unimodules|sentry-expo|native-base|react-native-svg|nativewind|react-native-css-interop|lucide-react-native|@swp/.*|@js-temporal/.*))',
  ],
  testMatch: ['<rootDir>/**/*.test.{ts,tsx}'],
  // .expo/types and build artifacts hold no tests.
  testPathIgnorePatterns: ['/node_modules/', '/.expo/', '/dist/'],
  clearMocks: true,
};
