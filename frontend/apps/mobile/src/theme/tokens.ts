// Bridge the shared design tokens (@swp/design-tokens) into a Tailwind/NativeWind theme.
// Single source of truth for color/spacing lives in the design-tokens package — this file
// only reshapes it for Tailwind. App code uses className tokens (e.g. `text-primary`),
// never raw hex (mirrors the web ENGINEERING.md token rule).
import { color, radius, space } from '@swp/design-tokens';

export const theme = {
  colors: {
    primary: color.primary,
    'primary-strong': color.primaryStrong,
    'primary-soft': color.primarySoft,
    text: color.text,
    'text-2': color.text2,
    'text-3': color.text3,
    muted: color.text3,
    'app-bg': color.appBg,
    surface: color.surface,
    'surface-2': color.surface2,
    border: color.border,
    'border-soft': color.borderSoft,
    // Status tones (DESIGN-SYSTEM §2) — "present" is teal (ok), not brand green.
    'ok-bg': color.ok.bg,
    'ok-border': color.ok.border,
    'ok-text': color.ok.text,
    'warn-bg': color.warn.bg,
    'warn-border': color.warn.border,
    'warn-text': color.warn.text,
    'bad-bg': color.bad.bg,
    'bad-border': color.bad.border,
    'bad-text': color.bad.text,
    'info-bg': color.info.bg,
    'info-border': color.info.border,
    'info-text': color.info.text,
    // Convenience text aliases.
    success: color.ok.text,
    warning: color.warn.text,
    danger: color.bad.text,
  },
  // space is the shared step scale [2,4,6,...] → spacing keys "1".."N" in px.
  spacing: Object.fromEntries(space.map((v, i) => [String(i + 1), `${v}px`])),
  borderRadius: {
    control: `${radius.control}px`,
    input: `${radius.input}px`,
    card: `${radius.card}px`,
    pill: `${radius.pill}px`,
  },
} as const;
