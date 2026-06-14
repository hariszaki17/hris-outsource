/**
 * Thin Pressable wrapper that accepts a `className` (NativeWind) plus a style override.
 * Used by the status filter chips so the chip surface itself is the tap target.
 */
import type { ReactNode } from 'react';
import { Pressable, type StyleProp, type ViewStyle } from 'react-native';

export function ChipPressable({
  children,
  className,
  style,
  onPress,
}: {
  children: ReactNode;
  className?: string;
  style?: StyleProp<ViewStyle>;
  onPress: () => void;
}) {
  return (
    <Pressable className={className} style={style} onPress={onPress} hitSlop={4}>
      {children}
    </Pressable>
  );
}
