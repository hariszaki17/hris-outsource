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
    'orange-bg': color.orange.bg,
    'orange-border': color.orange.border,
    'orange-text': color.orange.text,
    // Dark sidebar (web shell; exposed for parity / shared organisms).
    sidebar: color.sidebar,
    'sidebar-hover': color.sidebarHover,
    'sidebar-text': color.sidebarText,
    // Overlay scrim + control/avatar neutrals (comp/Toggle off, comp/Avatar neutral).
    scrim: color.scrim,
    'control-off': color.controlOff,
    'avatar-neutral': color.avatarNeutral,
    // Logo accents (service-line / category coding, charts) — used by the brand emblem.
    'accent-gold': color.accent.gold,
    'accent-green': color.accent.green,
    'accent-blue': color.accent.blue,
    'accent-purple': color.accent.purple,
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
  // Font families (DESIGN-SYSTEM §3). Static per-weight families from @expo-google-fonts,
  // loaded in app/_layout.tsx. RN doesn't synthesize weights from a single static family,
  // so each weight is its own className token (e.g. `font-sans-bold`). The canonical
  // src/ui/Text component maps the type ramp onto these.
  //   sans = Inter · mono = IBM Plex Mono (IDs/times) · display = Poppins (brand wordmark).
  fontFamily: {
    sans: ['Inter_400Regular'],
    'sans-medium': ['Inter_500Medium'],
    'sans-semibold': ['Inter_600SemiBold'],
    'sans-bold': ['Inter_700Bold'],
    mono: ['IBMPlexMono_400Regular'],
    'mono-medium': ['IBMPlexMono_500Medium'],
    display: ['Poppins_700Bold'],
  },
} as const;
