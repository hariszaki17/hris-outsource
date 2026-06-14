/**
 * BottomSheet — shared bottom-sheet shell (dim scrim + sliding surface).
 *
 * Why this exists: a plain <Modal animationType="slide"> slides the ENTIRE modal,
 * scrim included, so the dim layer travels up from the bottom edge instead of
 * fading in — a visible dark band sweeping the screen. Here the scrim fades
 * (Modal animationType="fade") while only the surface slides up (reanimated
 * SlideInDown), which is the native bottom-sheet feel.
 *
 * Scrim #18181B66 · surface top corners radius 16 · padding [12,16,24+safe,16].
 * Tapping the scrim closes; taps on the surface are swallowed.
 */
import type { ReactNode } from 'react';
import { Modal, Pressable, type ViewStyle } from 'react-native';
import Animated, { SlideInDown, SlideOutDown } from 'react-native-reanimated';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

export function BottomSheet({
  visible,
  onClose,
  style,
  children,
}: {
  visible: boolean;
  onClose: () => void;
  /** Extra style merged onto the surface (e.g. row gap). */
  style?: ViewStyle;
  children: ReactNode;
}) {
  const insets = useSafeAreaInsets();
  return (
    <Modal
      visible={visible}
      transparent
      animationType="fade"
      statusBarTranslucent
      onRequestClose={onClose}
    >
      <Pressable
        style={{ flex: 1, justifyContent: 'flex-end', backgroundColor: '#18181B66' }}
        onPress={onClose}
      >
        <Animated.View entering={SlideInDown.duration(260)} exiting={SlideOutDown.duration(200)}>
          <Pressable
            onPress={() => {}}
            className="bg-surface border border-border"
            style={{
              borderTopLeftRadius: 16,
              borderTopRightRadius: 16,
              paddingTop: 12,
              paddingHorizontal: 16,
              paddingBottom: 24 + insets.bottom,
              ...style,
            }}
          >
            {children}
          </Pressable>
        </Animated.View>
      </Pressable>
    </Modal>
  );
}
