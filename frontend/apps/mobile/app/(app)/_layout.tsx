import { color } from '@swp/design-tokens';
import { Tabs } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { useSession } from '../../src/providers/session';

// TODO(DB-8): When E4/E6/E7 screens exist for the shift_leader role, replace the
// placeholder routes (attendance / notifications / more) with a dedicated Tim, Notifikasi,
// and Profil tab set. For now only "Beranda" is fully implemented; the rest reuse the
// agent screens as stand-ins so the leader can still reach them.
export default function AppTabsLayout() {
  const { t } = useTranslation();
  const { user } = useSession();
  const isLeader = user?.role === 'shift_leader';

  return (
    <Tabs
      screenOptions={{
        headerShown: true,
        tabBarActiveTintColor: color.primary,
        tabBarInactiveTintColor: color.text3,
      }}
    >
      {isLeader ? (
        // ── Shift Leader tab set (DB-8 · frame UMzuO): Beranda / Notifikasi / Lainnya ──
        <>
          <Tabs.Screen name="leader-beranda" options={{ title: t('m:leaderBeranda.tabTitle') }} />
          <Tabs.Screen name="notifications" options={{ title: t('m:tabs.notifications') }} />
          <Tabs.Screen name="more" options={{ title: t('m:tabs.more') }} />
          {/* Hide agent-only tabs from the shift_leader tab bar */}
          <Tabs.Screen name="index" options={{ href: null }} />
          <Tabs.Screen name="attendance" options={{ href: null }} />
          <Tabs.Screen name="schedule" options={{ href: null }} />
        </>
      ) : (
        // ── Agent tab set (existing) ──────────────────────────────────────────────────
        <>
          <Tabs.Screen name="index" options={{ title: t('m:tabs.home') }} />
          <Tabs.Screen name="attendance" options={{ title: t('m:tabs.attendance') }} />
          <Tabs.Screen name="schedule" options={{ title: t('m:tabs.schedule') }} />
          <Tabs.Screen name="notifications" options={{ title: t('m:tabs.notifications') }} />
          <Tabs.Screen name="more" options={{ title: t('m:tabs.more') }} />
          {/* Hide shift_leader tab from agent tab bar */}
          <Tabs.Screen name="leader-beranda" options={{ href: null }} />
        </>
      )}
    </Tabs>
  );
}
