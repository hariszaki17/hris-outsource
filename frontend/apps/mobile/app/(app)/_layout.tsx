import { color } from '@swp/design-tokens';
import { Tabs } from 'expo-router';
import { Bell, CalendarCheck, FileText, House, User, Users } from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { useSession } from '../../src/providers/session';

// Bottom-tab nav styled to brainstorm.pen comp/AgentMobileNav (gfptk) + comp/SLMobileNav (fdVo7):
// white bar, 1px top border, icon 20, label Inter 10/500, active = brand primary, inactive = text-3.
// Agent tab set (AgentMobileNav): Beranda · Jadwal · Pengajuan · Profil. The agent's "Beranda" is
// the Absen (clock-in) screen — attendance.tsx — NOT a dashboard. Cuti is reached via Pengajuan.
// SL tab set (SLMobileNav): Beranda · Tim · Notifikasi · Profil.
//
// IMPORTANT: expo-router's <Tabs> only honours DIRECT <Tabs.Screen> children — wrapping them in a
// fragment or `{cond ? <>…</> : <>…</>}` makes it ignore them and auto-generate default tabs. So
// this is a flat list with role-conditional `href` (null = hidden). `index` is hidden and only
// redirects to the role's home (see index.tsx). Declaration order yields the correct visible
// sequence for both roles.
const ICON_SIZE = 20;

export default function AppTabsLayout() {
  const { t } = useTranslation();
  const { user } = useSession();
  const isLeader = user?.role === 'shift_leader';
  const agentOnly = isLeader ? null : undefined; // href: show for agent, hide for leader
  const leaderOnly = isLeader ? undefined : null; // href: show for leader, hide for agent

  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: color.primary,
        tabBarInactiveTintColor: color.text3,
        tabBarStyle: {
          backgroundColor: color.surface,
          borderTopColor: color.border,
          borderTopWidth: 1,
          paddingTop: 8,
        },
        tabBarLabelStyle: { fontFamily: 'Inter_500Medium', fontSize: 10 },
      }}
    >
      {/* Agent tabs — Beranda = Absen (attendance.tsx) */}
      <Tabs.Screen
        name="attendance"
        options={{
          href: agentOnly,
          title: t('m:tabs.home'),
          tabBarIcon: ({ color: c }) => <House color={c} size={ICON_SIZE} />,
        }}
      />
      <Tabs.Screen
        name="kehadiran"
        options={{
          href: agentOnly,
          title: t('m:tabs.attendance'),
          tabBarIcon: ({ color: c }) => <CalendarCheck color={c} size={ICON_SIZE} />,
        }}
      />
      <Tabs.Screen
        name="pengajuan"
        options={{
          href: agentOnly,
          title: t('m:tabs.requests'),
          tabBarIcon: ({ color: c }) => <FileText color={c} size={ICON_SIZE} />,
        }}
      />
      {/* Shift Leader tabs */}
      <Tabs.Screen
        name="leader-beranda"
        options={{
          href: leaderOnly,
          title: t('m:leaderBeranda.tabTitle'),
          tabBarIcon: ({ color: c }) => <House color={c} size={ICON_SIZE} />,
        }}
      />
      <Tabs.Screen
        name="sl-verifikasi"
        options={{
          href: leaderOnly,
          title: t('m:tabs.team'),
          tabBarIcon: ({ color: c }) => <Users color={c} size={ICON_SIZE} />,
        }}
      />
      <Tabs.Screen
        name="notifications"
        options={{
          href: leaderOnly,
          title: t('m:tabs.notifications'),
          tabBarIcon: ({ color: c }) => <Bell color={c} size={ICON_SIZE} />,
        }}
      />
      {/* Profil — both roles, always last */}
      <Tabs.Screen
        name="profile"
        options={{
          title: t('m:tabs.profile'),
          tabBarIcon: ({ color: c }) => <User color={c} size={ICON_SIZE} />,
        }}
      />
      {/* Always-hidden routes (index redirects; the rest are reached via push / Pengajuan) */}
      <Tabs.Screen name="index" options={{ href: null }} />
      <Tabs.Screen name="schedule" options={{ href: null }} />
      <Tabs.Screen name="leave" options={{ href: null }} />
      <Tabs.Screen name="more" options={{ href: null }} />
    </Tabs>
  );
}
