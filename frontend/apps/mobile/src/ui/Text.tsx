// Thin RN primitive layer. The web @swp/ui package is DOM/shadcn/Tailwind and is NOT
// reusable in React Native, so mobile keeps its own primitives backed by the same tokens.
// Promote to a shared @swp/ui-native package on the 2nd domain-agnostic reuse.
import { Text as RNText, type TextProps } from 'react-native';

type Variant = 'body' | 'title' | 'caption';

const variantClass: Record<Variant, string> = {
  title: 'text-text text-2xl font-semibold',
  body: 'text-text text-base',
  caption: 'text-text-3 text-sm',
};

export function Text({
  variant = 'body',
  className,
  ...props
}: TextProps & { variant?: Variant; className?: string }) {
  return <RNText className={`${variantClass[variant]} ${className ?? ''}`} {...props} />;
}
