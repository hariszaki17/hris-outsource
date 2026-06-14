import type { ReactNode } from 'react';
import { View } from 'react-native';
import { type Edge, SafeAreaView } from 'react-native-safe-area-context';

// Base screen container — app background + safe-area insets.
// `edges` defaults to all sides. Tab screens pass ['top','left','right'] so the
// bottom safe-area inset isn't double-counted with the tab bar (which already
// sits in the home-indicator region) — otherwise content floats above the bar.
export function Screen({
  children,
  edges = ['top', 'right', 'bottom', 'left'],
}: {
  children: ReactNode;
  edges?: readonly Edge[];
}) {
  return (
    <SafeAreaView edges={edges} className="flex-1 bg-app-bg">
      <View className="flex-1 px-6 py-8">{children}</View>
    </SafeAreaView>
  );
}
