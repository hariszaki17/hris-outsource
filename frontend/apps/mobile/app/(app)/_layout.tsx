import { color } from '@swp/design-tokens';
import { Tabs } from 'expo-router';
import { useTranslation } from 'react-i18next';

export default function AppTabsLayout() {
  const { t } = useTranslation();
  return (
    <Tabs
      screenOptions={{
        headerShown: true,
        tabBarActiveTintColor: color.primary,
        tabBarInactiveTintColor: color.text3,
      }}
    >
      <Tabs.Screen name="index" options={{ title: t('m:tabs.home') }} />
      <Tabs.Screen name="attendance" options={{ title: t('m:tabs.attendance') }} />
      <Tabs.Screen name="notifications" options={{ title: t('m:tabs.notifications') }} />
      <Tabs.Screen name="more" options={{ title: t('m:tabs.more') }} />
    </Tabs>
  );
}
