// Bridge the shared design tokens (@swp/design-tokens) into a Tailwind/NativeWind theme.
// Single source of truth for color/spacing lives in the design-tokens package — this file
// only reshapes it for Tailwind. App code uses className tokens (e.g. `text-primary`),
// never raw hex (mirrors the web ENGINEERING.md token rule).
import { color, space } from '@swp/design-tokens';

export const theme = {
  colors: {
    primary: color.primary,
    'primary-strong': color.primaryStrong,
    'primary-soft': color.primarySoft,
    text: color.text,
    'text-2': color.text2,
    'text-3': color.text3,
    'app-bg': color.appBg,
    surface: color.surface,
  },
  // space is the shared step scale [2,4,6,...] → spacing keys "1".."N" in px.
  spacing: Object.fromEntries(space.map((v, i) => [String(i + 1), `${v}px`])),
} as const;
