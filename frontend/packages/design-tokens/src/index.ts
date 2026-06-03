/**
 * SWP HRIS design tokens — single source of truth in code.
 * Mirrors docs/design/DESIGN-SYSTEM.md §2 (color), §3 (type), §4 (spacing/radius/elevation).
 *
 * The web app consumes these via Tailwind (`theme.css`). This TS export exists for
 * non-CSS consumers (React Native, charts, runtime status→color mapping). Keep the two in sync.
 */

export const color = {
  // Brand — green is reserved for brand/primary actions (DESIGN-SYSTEM §1).
  primary: '#188E4D',
  primaryStrong: '#0E6033',
  primarySoft: '#E7F4EC',
  // Text
  text: '#18181B',
  text2: '#52525B',
  text3: '#9CA3AF',
  // Surfaces
  appBg: '#F3F4F6',
  surface: '#FFFFFF',
  surface2: '#F9FAFB',
  // Borders
  border: '#E5E7EB',
  borderSoft: '#EEF0F2',
  // Dark sidebar
  sidebar: '#18181B',
  sidebarHover: '#27272A',
  sidebarText: '#A1A1AA',
  // Neutral avatar chip (identity initials — Topbar user / comp/Avatar neutral tone)
  avatarNeutral: '#DDE3EA',
  // Unchecked toggle/switch track (comp/Toggle)
  controlOff: '#D1D5DB',
  // Status — "present" is TEAL, not green (green is brand-only).
  ok: { bg: '#F0FDFA', border: '#99F6E4', text: '#0F766E' },
  warn: { bg: '#FFFAEB', border: '#FEDF89', text: '#B54708' },
  bad: { bg: '#FBEAE9', border: '#F1C5C0', text: '#BF4A40' },
  info: { bg: '#EFF8FF', border: '#B2DDFF', text: '#175CD3' },
  // "Tdk Lengkap" / transfer orange (brainstorm.pen orange-* variables)
  orange: { bg: '#FFF3E8', border: '#FDD9B5', text: '#C2410C' },
  // Accents from the logo (service-line / category coding, charts)
  accent: { gold: '#F5A800', green: '#5E8C2A', blue: '#0B5FAE', purple: '#8E0E8E' },
  scrim: 'rgba(24,24,27,0.70)',
} as const;

export const font = {
  ui: "'Inter', system-ui, sans-serif",
  mono: "'IBM Plex Mono', ui-monospace, monospace", // IDs, times, coordinates
  display: "'Poppins', 'Inter', sans-serif", // login & marketing only
} as const;

/** Type ramp: [fontSize px, fontWeight]. DESIGN-SYSTEM §3. */
export const type = {
  pageTitle: { size: 30, weight: 700 },
  section: { size: 22, weight: 700 },
  cardTitle: { size: 19, weight: 700 },
  strong: { size: 14, weight: 600 },
  body: { size: 14, weight: 400 },
  secondary: { size: 13, weight: 400 },
  caption: { size: 12, weight: 400 },
  tableHeader: { size: 11, weight: 600, letterSpacing: 0.5 },
} as const;

/** Spacing scale (px). DESIGN-SYSTEM §4. */
export const space = [2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 24, 32] as const;

export const radius = {
  control: 6,
  input: 8,
  card: 12,
  pill: 999,
} as const;

export const elevation = {
  card: '0 0 0 1px #E5E7EB',
  overlay: '0 8px 24px rgba(0,0,0,0.08)',
} as const;

/** Semantic status buckets — the only sanctioned source for status coloring (StatusBadge). */
export type StatusTone = 'ok' | 'warn' | 'bad' | 'info' | 'neutral' | 'onprogress';

/**
 * Attendance status → tone. DESIGN-SYSTEM §2 "Status → semantic mapping (attendance)".
 * Keys are the API enum / Bahasa labels used in the contract.
 */
export const attendanceTone: Record<string, StatusTone> = {
  HADIR: 'ok',
  TERLAMBAT: 'warn',
  TDK_LENGKAP: 'onprogress',
  ABSEN: 'bad',
  // verification sub-states
  OTOMATIS: 'neutral',
  MENUNGGU: 'warn',
  TERVERIFIKASI: 'info',
  DITOLAK: 'bad',
};

/** Placement status → tone (E3). DESIGN-SYSTEM §2 "Status → semantic mapping (placement)". */
export const placementTone: Record<string, StatusTone> = {
  DRAFT: 'neutral',
  SCHEDULED: 'info',
  ACTIVE: 'ok',
  EXPIRING: 'warn',
  ENDED: 'neutral',
  TERMINATED: 'bad',
  RESIGNED: 'bad',
  SUPERSEDED: 'neutral',
  TRANSFERRED: 'onprogress',
};
