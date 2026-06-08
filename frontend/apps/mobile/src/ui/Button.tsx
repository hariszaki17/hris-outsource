import { color } from '@swp/design-tokens';
import { ActivityIndicator, Pressable, type PressableProps } from 'react-native';
import { Text } from './Text';

type Variant = 'primary' | 'secondary';

export function Button({
  label,
  loading,
  disabled,
  variant = 'primary',
  className,
  ...props
}: PressableProps & { label: string; loading?: boolean; variant?: Variant; className?: string }) {
  const isDisabled = disabled || loading;
  const base = variant === 'primary' ? 'bg-primary' : 'bg-surface border border-border';
  const labelColor = variant === 'primary' ? 'text-surface' : 'text-text';
  return (
    <Pressable
      disabled={isDisabled}
      accessibilityRole="button"
      className={`flex-row items-center justify-center rounded-input px-5 py-3 ${base} ${isDisabled ? 'opacity-60' : ''} ${className ?? ''}`}
      {...props}
    >
      {loading ? (
        <ActivityIndicator color={variant === 'primary' ? color.surface : color.primary} />
      ) : (
        <Text className={`${labelColor} font-semibold`}>{label}</Text>
      )}
    </Pressable>
  );
}
