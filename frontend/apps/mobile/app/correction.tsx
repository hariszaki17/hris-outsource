import { ApiError } from '@swp/api-client';
import {
  type CorrectionType,
  type CorrectionWriteRequest,
  useCreateCorrection,
} from '@swp/api-client/e5';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Pressable, TextInput, View } from 'react-native';
import { Button } from '../src/ui/Button';
import { Screen } from '../src/ui/Screen';
import { Text } from '../src/ui/Text';

// CODE corrections need an attendance-code picker — deferred; MVP covers the common
// missed/wrong clock-in/out case.
const TYPES: { value: CorrectionType; key: 'typeCheckIn' | 'typeCheckOut' }[] = [
  { value: 'CHECK_IN', key: 'typeCheckIn' },
  { value: 'CHECK_OUT', key: 'typeCheckOut' },
];

export default function CorrectionForm() {
  const { t } = useTranslation();
  const router = useRouter();
  const { attendanceId, date } = useLocalSearchParams<{ attendanceId: string; date: string }>();
  const create = useCreateCorrection();

  const [type, setType] = useState<CorrectionType>('CHECK_OUT');
  const [time, setTime] = useState('');
  const [reason, setReason] = useState('');
  const [err, setErr] = useState<string | null>(null);

  async function onSubmit() {
    setErr(null);
    if (!attendanceId) return;
    if (!/^([01]\d|2[0-3]):([0-5]\d)$/.test(time.trim())) {
      setErr(t('m:correction.badTime'));
      return;
    }
    if (reason.trim().length < 5) {
      setErr(t('m:correction.badReason'));
      return;
    }
    // Asia/Jakarta is a fixed +07:00 offset (no DST) — safe to build the instant directly.
    const iso = `${(date ?? '').slice(0, 10)}T${time.trim()}:00+07:00`;
    const data: CorrectionWriteRequest = {
      attendance_id: attendanceId,
      type,
      reason: reason.trim(),
      proposed_check_in_at: type === 'CHECK_IN' ? iso : undefined,
      proposed_check_out_at: type === 'CHECK_OUT' ? iso : undefined,
    };
    try {
      await create.mutateAsync({ data });
      Alert.alert(t('m:correction.title'), t('m:correction.success'));
      router.back();
    } catch (e) {
      if (e instanceof ApiError) {
        if (e.code === 'OUTSIDE_CORRECTION_WINDOW') {
          setErr(t('m:correction.outsideWindow'));
          return;
        }
        if (e.code === 'CORRECTION_ALREADY_PENDING') {
          setErr(t('m:correction.alreadyPending'));
          return;
        }
      }
      setErr(t('m:correction.error'));
    }
  }

  const inputClass = 'rounded-input border border-border bg-surface px-4 py-3 text-text';

  return (
    <Screen>
      <View className="mb-6 flex-row items-center justify-between">
        <Text variant="title">{t('m:correction.title')}</Text>
        <Pressable onPress={() => router.back()}>
          <Text className="text-primary font-semibold">{t('m:clock.cancel')}</Text>
        </Pressable>
      </View>

      <View className="gap-4">
        {/* Type toggle */}
        <View className="flex-row gap-3">
          {TYPES.map((opt) => (
            <Pressable
              key={opt.value}
              onPress={() => setType(opt.value)}
              className={`flex-1 rounded-input border px-4 py-3 ${
                type === opt.value ? 'border-primary bg-primary-soft' : 'border-border bg-surface'
              }`}
            >
              <Text
                className={`text-center ${type === opt.value ? 'text-primary font-semibold' : 'text-text-2'}`}
              >
                {t(`m:correction.${opt.key}`)}
              </Text>
            </Pressable>
          ))}
        </View>

        <View>
          <Text variant="caption" className="mb-1">
            {t('m:correction.time')}
          </Text>
          <TextInput
            value={time}
            onChangeText={setTime}
            placeholder="15:10"
            keyboardType="numbers-and-punctuation"
            className={inputClass}
          />
        </View>

        <View>
          <Text variant="caption" className="mb-1">
            {t('m:correction.reason')}
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

        <Button label={t('m:correction.submit')} onPress={onSubmit} loading={create.isPending} />
      </View>
    </Screen>
  );
}
