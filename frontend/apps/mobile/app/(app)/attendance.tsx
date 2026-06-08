import { ApiError } from '@swp/api-client';
import {
  type Attendance,
  type AttendancePage,
  useClockIn,
  useClockOut,
  useListAttendance,
} from '@swp/api-client/e5';
import { formatInstant } from '@swp/shared/datetime';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Alert, ScrollView, View } from 'react-native';
import { getCurrentCoords } from '../../src/lib/location';
import { Button } from '../../src/ui/Button';
import { Card } from '../../src/ui/Card';
import { StatusBadge } from '../../src/ui/StatusBadge';
import { Text } from '../../src/ui/Text';

function timeOf(iso: string): string {
  return formatInstant(iso, { timeStyle: 'short' });
}
function dateOf(iso: string): string {
  return formatInstant(iso, { dateStyle: 'medium' });
}

function HistoryRow({ item }: { item: Attendance }) {
  const { t } = useTranslation();
  return (
    <Card>
      <View className="flex-row items-center justify-between">
        <Text variant="body" className="font-semibold">
          {dateOf(item.check_in_at)}
        </Text>
        <StatusBadge status={item.status} label={t(`m:attendance.status.${item.status}`)} />
      </View>
      <Text variant="caption" className="mt-1">
        {t('m:attendance.in')} {timeOf(item.check_in_at)}
        {item.check_out_at ? ` · ${t('m:attendance.out')} ${timeOf(item.check_out_at)}` : ''}
      </Text>
      {item.flags.length > 0 ? (
        <Text variant="caption" className="mt-1 text-warn-text">
          {item.flags.map((f) => t(`m:attendance.flag.${f}`)).join(' · ')}
        </Text>
      ) : null}
    </Card>
  );
}

export default function AttendanceScreen() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const list = useListAttendance({ limit: 20 });
  const clockIn = useClockIn();
  const clockOut = useClockOut();
  const [busy, setBusy] = useState(false);

  const body = list.data?.data as AttendancePage | undefined;
  const items: Attendance[] = body?.data ?? [];
  const open = items.find((a) => !a.check_out_at);

  async function refresh() {
    await qc.invalidateQueries({ queryKey: ['/attendance'] });
  }

  function handleError(e: unknown, onForce?: () => void) {
    if (e instanceof ApiError) {
      if (e.code === 'OUT_OF_GEOFENCE' && onForce) {
        Alert.alert(
          t('m:clock.outsideTitle'),
          t('m:clock.outsideMsg', {
            distance: e.fields?.distance_m ?? '?',
            radius: e.fields?.radius_m ?? '?',
          }),
          [
            { text: t('m:clock.cancel'), style: 'cancel' },
            { text: t('m:clock.clockAnyway'), onPress: onForce },
          ],
        );
        return;
      }
      if (e.code === 'ALREADY_CLOCKED_IN') {
        void refresh();
        Alert.alert(t('m:clock.title'), t('m:clock.alreadyIn'));
        return;
      }
      if (e.code === 'NOT_CLOCKED_IN') {
        Alert.alert(t('m:clock.title'), t('m:clock.notIn'));
        return;
      }
      if (e.code === 'GPS_UNAVAILABLE') {
        Alert.alert(t('m:clock.title'), t('m:clock.gpsUnavailable'));
        return;
      }
    }
    Alert.alert(t('m:clock.title'), t('m:clock.error'));
  }

  async function doClockIn(force: boolean) {
    setBusy(true);
    try {
      const coords = await getCurrentCoords();
      if (!coords) {
        Alert.alert(t('m:clock.title'), t('m:clock.gpsDenied'));
        return;
      }
      try {
        await clockIn.mutateAsync({
          data: {
            lat: coords.lat,
            lng: coords.lng,
            gps_available: true,
            wfo: true,
            force_outside_geofence: force,
          },
        });
        await refresh();
        Alert.alert(t('m:clock.title'), t('m:clock.successIn'));
      } catch (e) {
        handleError(e, () => void doClockIn(true));
      }
    } finally {
      setBusy(false);
    }
  }

  async function doClockOut() {
    setBusy(true);
    try {
      const coords = await getCurrentCoords();
      if (!coords) {
        Alert.alert(t('m:clock.title'), t('m:clock.gpsDenied'));
        return;
      }
      try {
        await clockOut.mutateAsync({
          data: { lat: coords.lat, lng: coords.lng, gps_available: true },
        });
        await refresh();
        Alert.alert(t('m:clock.title'), t('m:clock.successOut'));
      } catch (e) {
        handleError(e);
      }
    } finally {
      setBusy(false);
    }
  }

  const pending = busy || clockIn.isPending || clockOut.isPending;

  return (
    <ScrollView className="flex-1 bg-app-bg">
      <View className="gap-4 p-6">
        {/* Clock card */}
        <Card>
          <Text variant="caption">{t('m:clock.title')}</Text>
          <Text variant="title" className="mt-1">
            {open
              ? t('m:clock.clockedInAt', { time: timeOf(open.check_in_at) })
              : t('m:clock.notClockedIn')}
          </Text>
          <View className="mt-4">
            {open ? (
              <Button
                label={t('m:clock.clockOut')}
                onPress={() => void doClockOut()}
                loading={pending}
              />
            ) : (
              <Button
                label={t('m:clock.clockIn')}
                onPress={() => void doClockIn(false)}
                loading={pending}
              />
            )}
          </View>
        </Card>

        {/* History */}
        <Text variant="body" className="mt-2 font-semibold">
          {t('m:attendance.historyTitle')}
        </Text>
        {list.isLoading ? (
          <View className="items-center py-10">
            <ActivityIndicator />
          </View>
        ) : list.isError ? (
          <Card>
            <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
          </Card>
        ) : items.length === 0 ? (
          <Card>
            <Text variant="caption">{t('m:attendance.empty')}</Text>
          </Card>
        ) : (
          <View className="gap-3">
            {items.map((a) => (
              <HistoryRow key={a.id} item={a} />
            ))}
          </View>
        )}
      </View>
    </ScrollView>
  );
}
