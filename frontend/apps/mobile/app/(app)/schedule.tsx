import {
  type GetScheduleByAgent200,
  type ScheduleEntry,
  useGetScheduleByAgent,
} from '@swp/api-client/e4';
import { LOCALE_ID } from '@swp/shared/datetime';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';
import { useSession } from '../../src/providers/session';
import { Card } from '../../src/ui/Card';
import { Text } from '../../src/ui/Text';

// Local (device = WIB) YYYY-MM-DD — avoids the UTC shift of toISOString().
function ymd(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${m}-${day}`;
}

function currentWeek(): Date[] {
  const now = new Date();
  const mondayOffset = (now.getDay() + 6) % 7; // 0 = Monday
  const monday = new Date(now);
  monday.setDate(now.getDate() - mondayOffset);
  return Array.from({ length: 7 }, (_, i) => {
    const d = new Date(monday);
    d.setDate(monday.getDate() + i);
    return d;
  });
}

const weekdayFmt = new Intl.DateTimeFormat(LOCALE_ID, { weekday: 'short' });

function DayCard({ date, entry }: { date: Date; entry?: ScheduleEntry }) {
  const { t } = useTranslation();
  let detail: string;
  let tone = 'text-text-2';
  if (!entry || entry.is_day_off) {
    detail = entry?.is_day_off ? t('m:schedule.dayOff') : t('m:schedule.noShift');
    tone = 'text-text-3';
  } else if (entry.status === 'CANCELLED_BY_LEAVE') {
    detail = t('m:schedule.cancelledLeave');
    tone = 'text-info-text';
  } else {
    detail = `${entry.start_time ?? '—'}–${entry.end_time ?? '—'}`;
    tone = 'text-text';
  }
  return (
    <Card>
      <View className="flex-row items-center justify-between">
        <View>
          <Text variant="caption">{weekdayFmt.format(date)}</Text>
          <Text variant="body" className="font-semibold">
            {date.getDate()}
          </Text>
        </View>
        <View className="items-end">
          <Text variant="body" className={tone}>
            {detail}
          </Text>
          {entry?.company_name ? (
            <Text variant="caption" className="text-text-3">
              {entry.company_name}
            </Text>
          ) : null}
        </View>
      </View>
    </Card>
  );
}

export default function ScheduleScreen() {
  const { t } = useTranslation();
  const { user } = useSession();
  const employeeId = user?.employee_id ?? '';
  const days = currentWeek();

  const q = useGetScheduleByAgent(
    employeeId,
    { start_date: ymd(days[0]), end_date: ymd(days[6]), include_company: true },
    { query: { enabled: !!employeeId } },
  );
  const body = q.data?.data as GetScheduleByAgent200 | undefined;
  const entries: ScheduleEntry[] = body?.data ?? [];
  const byDate = (d: Date) => entries.find((e) => e.work_date === ymd(d));

  return (
    <ScrollView className="flex-1 bg-app-bg">
      <View className="gap-3 p-6">
        <Text variant="title">{t('m:schedule.title')}</Text>
        {q.isLoading ? (
          <View className="items-center py-10">
            <ActivityIndicator />
          </View>
        ) : q.isError ? (
          <Card>
            <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
          </Card>
        ) : (
          days.map((d) => <DayCard key={ymd(d)} date={d} entry={byDate(d)} />)
        )}
      </View>
    </ScrollView>
  );
}
