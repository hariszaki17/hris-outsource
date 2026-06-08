import { ApiError } from '@swp/api-client';
import { useCreateOvertimeRequest } from '@swp/api-client/e7';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Pressable, TextInput, View } from 'react-native';
import { useSession } from '../src/providers/session';
import { Button } from '../src/ui/Button';
import { Screen } from '../src/ui/Screen';
import { Text } from '../src/ui/Text';

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;
const TIME_RE = /^([01]\d|2[0-3]):([0-5]\d)$/;

export default function OvertimeNew() {
  const { t } = useTranslation();
  const router = useRouter();
  const { user } = useSession();
  const create = useCreateOvertimeRequest();

  const [date, setDate] = useState('');
  const [startT, setStartT] = useState('');
  const [endT, setEndT] = useState('');
  const [reason, setReason] = useState('');
  const [err, setErr] = useState<string | null>(null);

  async function onSubmit() {
    setErr(null);
    if (!DATE_RE.test(date)) return setErr(t('m:overtime.badDate'));
    if (!TIME_RE.test(startT) || !TIME_RE.test(endT)) return setErr(t('m:overtime.badTime'));
    if (reason.trim().length < 5) return setErr(t('m:overtime.badReason'));
    try {
      await create.mutateAsync({
        data: {
          employee_id: user?.employee_id ?? '',
          work_date: date,
          planned_start_time: startT,
          planned_end_time: endT,
          reason: reason.trim(),
        },
      });
      Alert.alert(t('m:overtime.title'), t('m:overtime.success'));
      router.back();
    } catch (e) {
      if (e instanceof ApiError) {
        if (e.code === 'OT_OVERLAPS_LEAVE') return setErr(t('m:overtime.overlapLeave'));
        if (e.code === 'NO_ACTIVE_PLACEMENT') return setErr(t('m:overtime.noPlacement'));
      }
      setErr(t('m:overtime.error'));
    }
  }

  const inputClass = 'rounded-input border border-border bg-surface px-4 py-3 text-text';

  return (
    <Screen>
      <View className="mb-6 flex-row items-center justify-between">
        <Text variant="title">{t('m:overtime.newBtn')}</Text>
        <Pressable onPress={() => router.back()}>
          <Text className="text-primary font-semibold">{t('m:clock.cancel')}</Text>
        </Pressable>
      </View>

      <View className="gap-4">
        <View>
          <Text variant="caption" className="mb-1">
            {t('m:overtime.workDate')}
          </Text>
          <TextInput
            value={date}
            onChangeText={setDate}
            placeholder="2026-06-10"
            autoCapitalize="none"
            className={inputClass}
          />
        </View>
        <View className="flex-row gap-3">
          <View className="flex-1">
            <Text variant="caption" className="mb-1">
              {t('m:overtime.startTime')}
            </Text>
            <TextInput
              value={startT}
              onChangeText={setStartT}
              placeholder="18:00"
              className={inputClass}
            />
          </View>
          <View className="flex-1">
            <Text variant="caption" className="mb-1">
              {t('m:overtime.endTime')}
            </Text>
            <TextInput
              value={endT}
              onChangeText={setEndT}
              placeholder="20:00"
              className={inputClass}
            />
          </View>
        </View>
        <View>
          <Text variant="caption" className="mb-1">
            {t('m:overtime.reason')}
          </Text>
          <TextInput
            value={reason}
            onChangeText={setReason}
            multiline
            numberOfLines={3}
            className={`${inputClass} h-24`}
            style={{ textAlignVertical: 'top' }}
          />
        </View>

        {err ? <Text className="text-danger">{err}</Text> : null}

        <Button label={t('m:overtime.submit')} onPress={onSubmit} loading={create.isPending} />
      </View>
    </Screen>
  );
}
