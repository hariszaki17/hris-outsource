// comp/BtnPrimary anatomy (REFERENCE §2): cornerRadius 8 (rounded-input), fill $primary,
// padding [10,16] (py-2.5 px-4), gap 7 (gap-[7px]), label font-sans 14/600 #FFFFFF, optional
// lucide icon 16×16. Variants mirror comp/BtnSecondary / BtnGhost / BtnDanger.
import { color } from '@swp/design-tokens';
import type { ReactNode } from 'react';
import { ActivityIndicator, Pressable, type PressableProps } from 'react-native';
import { Text } from './Text';

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger';

const containerClass: Record<Variant, string> = {
  primary: 'bg-primary',
  secondary: 'bg-surface border border-border',
  ghost: 'bg-transparent',
  danger: 'bg-bad-text',
};

const labelColor: Record<Variant, string> = {
  primary: color.surface,
  secondary: color.text,
  ghost: color.primary,
  danger: color.surface,
};

export function Button({
  label,
  loading,
  disabled,
  variant = 'primary',
  icon,
  className,
  ...props
}: PressableProps & {
  label: string;
  loading?: boolean;
  variant?: Variant;
  /** Optional leading lucide icon (16×16), tinted to the label color. */
  icon?: ReactNode;
  className?: string;
}) {
  const isDisabled = disabled || loading;
  return (
    <Pressable
      disabled={isDisabled}
      accessibilityRole="button"
      className={`flex-row items-center justify-center gap-[7px] rounded-input px-4 py-2.5 ${containerClass[variant]} ${isDisabled ? 'opacity-60' : ''} ${className ?? ''}`}
      {...props}
    >
      {loading ? (
        <ActivityIndicator color={labelColor[variant]} />
      ) : (
        <>
          {icon ?? null}
          <Text variant="strong" style={{ color: labelColor[variant] }}>
            {label}
          </Text>
        </>
      )}
    </Pressable>
  );
}
