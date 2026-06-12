// Thin RN primitive layer. The web @swp/ui package is DOM/shadcn/Tailwind and is NOT
// reusable in React Native, so mobile keeps its own primitives backed by the same tokens.
// Promote to a shared @swp/ui-native package on the 2nd domain-agnostic reuse.
import { Text as RNText, type TextProps } from 'react-native';

// Type ramp — mirrors @swp/design-tokens `type` (DESIGN-SYSTEM §3).
// pageTitle 30/700 · section 22/700 · cardTitle 19/700 · strong 14/600
// · body 14/400 · secondary 13/400 · caption 12/400.
type Variant =
  | 'pageTitle'
  | 'section'
  | 'cardTitle'
  | 'strong'
  | 'body'
  | 'secondary'
  | 'caption'
  // legacy alias kept for existing call sites (= section).
  | 'title';

// Each variant is split into (typography, default color). Weight comes from the font FAMILY
// (Inter_*), not fontWeight — RN can't synthesize weights from a single static family.
const variantType: Record<Variant, string> = {
  pageTitle: 'text-[30px] font-sans-bold',
  section: 'text-[22px] font-sans-bold',
  cardTitle: 'text-[19px] font-sans-bold',
  strong: 'text-sm font-sans-semibold',
  body: 'text-sm font-sans',
  secondary: 'text-[13px] font-sans',
  caption: 'text-xs font-sans',
  title: 'text-[22px] font-sans-bold',
};
const variantColor: Record<Variant, string> = {
  pageTitle: 'text-text',
  section: 'text-text',
  cardTitle: 'text-text',
  strong: 'text-text',
  body: 'text-text',
  secondary: 'text-text-2',
  caption: 'text-text-3',
  title: 'text-text',
};

// NativeWind v4 does NOT guarantee a later className beats the variant's color for
// same-specificity `text-<color>` utilities — the variant default was winning, so any
// `className="… text-primary"` silently rendered as `text-text`. So we detect a caller
// text-color override and DROP the variant color, letting the override apply cleanly.
// Matches design-system color tokens only (never sizes like text-sm / text-[13px] or
// alignment like text-center). Keep in sync with src/theme/tokens.ts color keys.
const TEXT_COLOR_OVERRIDE =
  /(?:^|\s)!?text-(?:primary(?:-strong|-soft)?|text(?:-2|-3)?|muted|surface(?:-2)?|app-bg|border(?:-soft)?|success|warning|danger|scrim|control-off|avatar-neutral|sidebar(?:-hover|-text)?|(?:ok|warn|bad|info|orange)-(?:bg|border|text)|accent-(?:gold|green|blue|purple)|white|black)\b/;

export function Text({
  variant = 'body',
  className,
  ...props
}: TextProps & { variant?: Variant; className?: string }) {
  const overridesColor = className ? TEXT_COLOR_OVERRIDE.test(className) : false;
  const color = overridesColor ? '' : variantColor[variant];
  return (
    <RNText className={`${variantType[variant]} ${color} ${className ?? ''}`.trim()} {...props} />
  );
}
