import '../global.css';
import { useFonts } from 'expo-font';
import { Stack, useRouter, useSegments } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { useEffect } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { checkForJsUpdate } from '../src/lib/update-gate';
import { AppProviders } from '../src/providers/app-providers';
import { useSession } from '../src/providers/session';

// Auth gate: redirect by session status + which route group we're in.
function RootNavigator() {
  const { status } = useSession();
  const segments = useSegments();
  const router = useRouter();

  useEffect(() => {
    if (status === 'restoring') return;
    const inAuthGroup = segments[0] === '(auth)';
    if (status === 'unauthed' && !inAuthGroup) {
      router.replace('/login');
    } else if (status === 'authed' && inAuthGroup) {
      router.replace('/');
    }
  }, [status, segments, router]);

  if (status === 'restoring') {
    return (
      <View className="flex-1 items-center justify-center bg-app-bg">
        <ActivityIndicator />
      </View>
    );
  }

  return <Stack screenOptions={{ headerShown: false }} />;
}

export default function RootLayout() {
  const [fontsLoaded] = useFonts({
    Poppins: require('../assets/fonts/Poppins-Bold.ttf'),
  });

  // Force-update gate (OTA) on launch — no-op in dev / until EAS Update is configured.
  useEffect(() => {
    void checkForJsUpdate();
  }, []);

  if (!fontsLoaded) {
    return (
      <View className="flex-1 items-center justify-center bg-surface">
        <ActivityIndicator />
      </View>
    );
  }

  return (
    <AppProviders>
      <StatusBar style="auto" />
      <RootNavigator />
    </AppProviders>
  );
}
