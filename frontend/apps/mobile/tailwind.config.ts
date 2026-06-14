import nativewindPreset from 'nativewind/preset';
import type { Config } from 'tailwindcss';
// Theme is generated from @swp/design-tokens (see src/theme/tokens.ts) — tokens only, no raw hex.
import { theme } from './src/theme/tokens';

export default {
  // App is light-only (no dark: variants), but NativeWind-web defaults darkMode to 'media'
  // and then throws "Cannot manually set color scheme". 'class' disables the auto-media path.
  darkMode: 'class',
  content: ['./app/**/*.{ts,tsx}', './src/**/*.{ts,tsx}'],
  presets: [nativewindPreset],
  theme: { extend: theme },
  plugins: [],
} satisfies Config;
