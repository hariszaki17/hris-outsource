import { color } from '@swp/design-tokens';
import { Tabs } from 'expo-router';
import { Bell, CalendarDays, Home, MapPin, User, Users } from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { useSession } from '../../src/providers/session';

// Agent tab set (brainstorm.pen, profil-saya/absen frames): Beranda / Kehadiran / Cuti / Profil.
// Notifications is NOT a tab — each screen renders its own in-body header with a top-right bell
// (the .pen header pattern), so the navigator header is hidden. Jadwal folds into Beranda/
// Kehadiran; payslip/lembur/settings overflow lives under Profil. (IA resolved 2026-06-12.)
// SL tab set (frame UMzuO): Beranda / Tim / Notifikasi / Profil.
export default function AppTabsLayout() {
  const { t } = useTranslation();
  const { user } = useSession();
  const isLeader = user?.role === 'shift_leader';

  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: color.primary,
        tabBarInactiveTintColor: color.text3,
      }}
    >
      {isLeader ? (
        // ── Shift Leader tab set (frame UMzuO): Beranda / Tim / Notifikasi / Profil ──
        <>
          <Tabs.Screen
            name="leader-beranda"
            options={{
              title: t('m:leaderBeranda.tabTitle'),
              tabBarIcon: ({ color: c, size }) => <Home color={c} size={size} />,
            }}
          />
          <Tabs.Screen
            name="sl-verifikasi"
            options={{
              title: t('m:tabs.team'),
              tabBarIcon: ({ color: c, size }) => <Users color={c} size={size} />,
            }}
          />
          <Tabs.Screen
            name="notifications"
            options={{
              title: t('m:tabs.notifications'),
              tabBarIcon: ({ color: c, size }) => <Bell color={c} size={size} />,
            }}
          />
          <Tabs.Screen
            name="profile"
            options={{
              title: t('m:tabs.profile'),
              tabBarIcon: ({ color: c, size }) => <User color={c} size={size} />,
            }}
          />
          {/* Hide agent-only + overflow routes from the SL tab bar */}
          <Tabs.Screen name="index" options={{ href: null }} />
          <Tabs.Screen name="attendance" options={{ href: null }} />
          <Tabs.Screen name="schedule" options={{ href: null }} />
          <Tabs.Screen name="leave" options={{ href: null }} />
          <Tabs.Screen name="more" options={{ href: null }} />
        </>
      ) : (
        // ── Agent tab set: Beranda / Kehadiran / Cuti / Profil ──────────────────────
        <>
          <Tabs.Screen
            name="index"
            options={{
              title: t('m:tabs.home'),
              tabBarIcon: ({ color: c, size }) => <Home color={c} size={size} />,
            }}
          />
          <Tabs.Screen
            name="attendance"
            options={{
              title: t('m:tabs.attendance'),
              tabBarIcon: ({ color: c, size }) => <MapPin color={c} size={size} />,
            }}
          />
          <Tabs.Screen
            name="leave"
            options={{
              title: t('m:tabs.leave'),
              tabBarIcon: ({ color: c, size }) => <CalendarDays color={c} size={size} />,
            }}
          />
          <Tabs.Screen
            name="profile"
            options={{
              title: t('m:tabs.profile'),
              tabBarIcon: ({ color: c, size }) => <User color={c} size={size} />,
            }}
          />
          {/* Hidden routes (reachable via push / overflow, not direct tabs) */}
          <Tabs.Screen name="schedule" options={{ href: null }} />
          <Tabs.Screen name="notifications" options={{ href: null }} />
          <Tabs.Screen name="more" options={{ href: null }} />
          <Tabs.Screen name="leader-beranda" options={{ href: null }} />
          <Tabs.Screen name="sl-verifikasi" options={{ href: null }} />
        </>
      )}
    </Tabs>
  );
}
