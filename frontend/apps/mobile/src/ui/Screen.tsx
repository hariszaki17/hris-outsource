import type { ReactNode } from 'react';
import { View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

// Base screen container — app background + safe-area insets.
export function Screen({ children }: { children: ReactNode }) {
  return (
    <SafeAreaView className="flex-1 bg-app-bg">
      <View className="flex-1 px-6 py-8">{children}</View>
    </SafeAreaView>
  );
}
