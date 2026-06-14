// Legacy route — attendance history moved to the agent "Kehadiran" tab ((app)/kehadiran.tsx)
// when the Jadwal tab was repurposed. Kept as a redirect so any lingering deep links resolve.
import { Redirect } from 'expo-router';

export default function AttendanceHistoryRedirect() {
  return <Redirect href="/kehadiran" />;
}
