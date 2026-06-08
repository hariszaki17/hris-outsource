import nativewindPreset from 'nativewind/preset';
import type { Config } from 'tailwindcss';
// Theme is generated from @swp/design-tokens (see src/theme/tokens.ts) — tokens only, no raw hex.
import { theme } from './src/theme/tokens';

export default {
  content: ['./app/**/*.{ts,tsx}', './src/**/*.{ts,tsx}'],
  presets: [nativewindPreset],
  theme: { extend: theme },
  plugins: [],
} satisfies Config;
