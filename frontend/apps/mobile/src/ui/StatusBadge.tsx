import { View } from 'react-native';
import { Text } from './Text';

// Attendance status → semantic tone (DESIGN-SYSTEM §2: "present" is teal = ok, not brand green).
type Tone = 'ok' | 'warn' | 'bad' | 'info';

const statusTone: Record<string, Tone> = {
  PRESENT: 'ok',
  LATE: 'warn',
  INCOMPLETE: 'warn',
  ABSENT: 'bad',
  ON_LEAVE: 'info',
};

const bgClass: Record<Tone, string> = {
  ok: 'bg-ok-bg',
  warn: 'bg-warn-bg',
  bad: 'bg-bad-bg',
  info: 'bg-info-bg',
};
const textClass: Record<Tone, string> = {
  ok: 'text-ok-text',
  warn: 'text-warn-text',
  bad: 'text-bad-text',
  info: 'text-info-text',
};

export function StatusBadge({ status, label }: { status: string; label: string }) {
  const tone = statusTone[status] ?? 'info';
  return (
    <View className={`self-start rounded-pill px-2 py-1 ${bgClass[tone]}`}>
      <Text className={`text-xs font-semibold ${textClass[tone]}`}>{label}</Text>
    </View>
  );
}
