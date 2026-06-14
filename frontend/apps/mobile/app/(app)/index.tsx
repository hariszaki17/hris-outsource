// Entry route. The agent's Beranda (home) tab is the Absen clock-in screen (attendance.tsx),
// not a dashboard — so `index` just bounces to the right home per role. It is hidden from the
// tab bar (href:null in _layout); the visible "Beranda" tab points at /attendance (agent) or
// /leader-beranda (shift leader).
import { Redirect } from 'expo-router';
import { useSession } from '../../src/providers/session';

export default function Index() {
  const { user } = useSession();
  return <Redirect href={user?.role === 'shift_leader' ? '/leader-beranda' : '/attendance'} />;
}
