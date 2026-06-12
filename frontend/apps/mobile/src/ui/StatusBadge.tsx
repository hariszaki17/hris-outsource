import { View } from 'react-native';
import { Text } from './Text';

// Attendance status → semantic tone (DESIGN-SYSTEM §2: "present" is teal = ok, not brand green).
// TDK_LENGKAP / INCOMPLETE is orange ("onprogress"), distinct from warn (amber) — brainstorm.pen.
type Tone = 'ok' | 'warn' | 'bad' | 'info' | 'orange';

const statusTone: Record<string, Tone> = {
  PRESENT: 'ok',
  LATE: 'warn',
  INCOMPLETE: 'orange',
  ABSENT: 'bad',
  ON_LEAVE: 'info',
};

const bgClass: Record<Tone, string> = {
  ok: 'bg-ok-bg',
  warn: 'bg-warn-bg',
  bad: 'bg-bad-bg',
  info: 'bg-info-bg',
  orange: 'bg-orange-bg',
};
const textClass: Record<Tone, string> = {
  ok: 'text-ok-text',
  warn: 'text-warn-text',
  bad: 'text-bad-text',
  info: 'text-info-text',
  orange: 'text-orange-text',
};

export function StatusBadge({ status, label }: { status: string; label: string }) {
  const tone = statusTone[status] ?? 'info';
  return (
    <View className={`self-start rounded-pill px-2 py-1 ${bgClass[tone]}`}>
      <Text className={`text-xs font-semibold ${textClass[tone]}`}>{label}</Text>
    </View>
  );
}
