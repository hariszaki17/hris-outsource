// Expo + NativeWind + pnpm monorepo Metro config.
// node-linker=hoisted (frontend/.npmrc) keeps node_modules flat, so the only monorepo
// tweaks needed are watching the workspace root and adding its node_modules to the
// resolver path — Expo's metro-config handles the rest. (The pnpm symlink resolver
// overrides flagged by expo-doctor are unnecessary under hoisted linking.)
const { getDefaultConfig } = require('expo/metro-config');
const { withNativeWind } = require('nativewind/metro');
const path = require('node:path');

const projectRoot = __dirname;
const workspaceRoot = path.resolve(projectRoot, '../..');

const config = getDefaultConfig(projectRoot);

// Watch the whole monorepo so changes in packages/* trigger reloads.
config.watchFolders = [workspaceRoot];

// Resolve modules from the app first, then the hoisted workspace root.
config.resolver.nodeModulesPaths = [
  path.resolve(projectRoot, 'node_modules'),
  path.resolve(workspaceRoot, 'node_modules'),
];

module.exports = withNativeWind(config, { input: './global.css' });
