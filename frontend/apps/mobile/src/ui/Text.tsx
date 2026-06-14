// Canonical mobile typography — the SINGLE source of truth for font size/weight/family.
//
// Why style-driven (not className): NativeWind v4 lets a component's own variant class win
// over a same-specificity caller class, so `<Text className="text-[13px] font-mono-medium">`
// silently rendered at the variant's size/family. Worse, arbitrary `text-[Npx]` and the
// `font-mono-*` families were unreliable. So this component applies fontSize / lineHeight /
// fontFamily via the `style` prop (always highest priority in RN) — every screen uses a
// `variant` (+ optional `weight` / `mono`) and NEVER hand-rolls font size/family.
//
// Ramp mirrors DESIGN-SYSTEM §3 + the brainstorm.pen type styles (faithful, zero visual
// shift). Weight comes from the font FAMILY (Inter_*/IBMPlexMono_*/Poppins) — RN can't
// synthesize weights from one static family. Families are loaded in app/_layout.tsx.
import { Text as RNText, type StyleProp, type TextProps, type TextStyle } from 'react-native';

export type FontWeight = 'regular' | 'medium' | 'semibold' | 'bold';

export type Variant =
  | 'pageTitle' // 30/700 — biggest screen title
  | 'section' // 22/700
  | 'displayTitle' // 22/700 Poppins — brand/auth headline (DESIGN-SYSTEM §3)
  | 'screenTitle' // 20/700 — tab/app-bar title
  | 'cardTitle' // 19/700
  | 'subtitle' // 15/700 — card/date headers (e.g. "Sel, 9 Jun 2026")
  | 'strong' // 14/600
  | 'body' // 14/400
  | 'label' // 13/700 — field/card section labels (e.g. "Clock-in")
  | 'secondary' // 13/400
  | 'caption' // 12/400
  | 'badge' // 11/600 — flag/pill chips, table header
  | 'micro' // 10/500 — tab-bar labels
  | 'metric' // 28/700 — stat numbers
  | 'buttonLg' // 16/700 — primary buttons
  | 'monoLg' // 20/700 mono — masuk/keluar tiles
  | 'monoHero' // 46/700 mono — live clock
  | 'title'; // legacy alias = section

const RAMP: Record<
  Variant,
  { size: number; lh: number; weight: FontWeight; mono?: boolean; display?: boolean }
> = {
  pageTitle: { size: 30, lh: 36, weight: 'bold' },
  section: { size: 22, lh: 28, weight: 'bold' },
  title: { size: 22, lh: 28, weight: 'bold' },
  displayTitle: { size: 22, lh: 28, weight: 'bold', display: true },
  screenTitle: { size: 20, lh: 26, weight: 'bold' },
  cardTitle: { size: 19, lh: 25, weight: 'bold' },
  subtitle: { size: 15, lh: 20, weight: 'bold' },
  strong: { size: 14, lh: 20, weight: 'semibold' },
  body: { size: 14, lh: 21, weight: 'regular' },
  label: { size: 13, lh: 18, weight: 'bold' },
  secondary: { size: 13, lh: 19, weight: 'regular' },
  caption: { size: 12, lh: 17, weight: 'regular' },
  badge: { size: 11, lh: 15, weight: 'semibold' },
  micro: { size: 10, lh: 13, weight: 'medium' },
  metric: { size: 28, lh: 34, weight: 'bold' },
  buttonLg: { size: 16, lh: 22, weight: 'bold' },
  monoLg: { size: 20, lh: 26, weight: 'bold', mono: true },
  monoHero: { size: 46, lh: 52, weight: 'bold', mono: true },
};

const SANS: Record<FontWeight, string> = {
  regular: 'Inter_400Regular',
  medium: 'Inter_500Medium',
  semibold: 'Inter_600SemiBold',
  bold: 'Inter_700Bold',
};
// IBM Plex Mono ships 400/500/700 — semibold maps to 500 (no 600 face).
const MONO: Record<FontWeight, string> = {
  regular: 'IBMPlexMono_400Regular',
  medium: 'IBMPlexMono_500Medium',
  semibold: 'IBMPlexMono_500Medium',
  bold: 'IBMPlexMono_700Bold',
};
const DISPLAY = 'Poppins_700Bold';

// Default color per variant (overridable). Most text is text-text; muted ramps differ.
const defaultColor: Partial<Record<Variant, string>> = {
  secondary: 'text-text-2',
  caption: 'text-text-3',
  micro: 'text-text-3',
};

// NativeWind v4 doesn't guarantee a later same-specificity `text-<color>` beats the default,
// so when the caller passes a color token we drop our default and let theirs win. Matches
// design-system COLOR tokens only (never sizes/alignment). Keep in sync with theme color keys.
const TEXT_COLOR_OVERRIDE =
  /(?:^|\s)!?text-(?:primary(?:-strong|-soft)?|text(?:-2|-3)?|muted|surface(?:-2)?|app-bg|border(?:-soft)?|success|warning|danger|scrim|control-off|avatar-neutral|sidebar(?:-hover|-text)?|(?:ok|warn|bad|info|orange)-(?:bg|border|text)|accent-(?:gold|green|blue|purple)|white|black)\b/;

export function Text({
  variant = 'body',
  weight,
  mono,
  className,
  style,
  ...props
}: TextProps & {
  variant?: Variant;
  /** Override the variant's default weight (family). */
  weight?: FontWeight;
  /** Render in IBM Plex Mono (IDs, times, coordinates) at the variant's size. */
  mono?: boolean;
  className?: string;
  style?: StyleProp<TextStyle>;
}) {
  const r = RAMP[variant];
  const w = weight ?? r.weight;
  const family = r.display ? DISPLAY : (mono ?? r.mono) ? MONO[w] : SANS[w];

  const overridesColor = className ? TEXT_COLOR_OVERRIDE.test(className) : false;
  const colorClass = overridesColor ? '' : (defaultColor[variant] ?? 'text-text');

  return (
    <RNText
      className={`${colorClass} ${className ?? ''}`.trim()}
      style={[{ fontSize: r.size, lineHeight: r.lh, fontFamily: family }, style]}
      {...props}
    />
  );
}
