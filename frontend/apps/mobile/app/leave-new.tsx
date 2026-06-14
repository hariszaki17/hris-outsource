// Agen · Ajukan Cuti — built from brainstorm.pen frame QT92D.
// Per-type contract: type chips + a per-type balance hint banner for the selected type
// (useGetEmployeeTypeBalances / LeaveTypeBalance) + a standing quota-rejection warning.
// Deferred vs the frame (capability gaps, not omissions): native date picker (ISO text inputs
// for now), working-days count, delegation picker, and document upload (doc-required types are
// filtered out — no attachment flow yet).
import { ApiError } from '@swp/api-client';
import {
  type LeaveType,
  type LeaveTypeBalance,
  LeaveTypeStatus,
  useCreateLeaveRequest,
  useGetEmployeeTypeBalances,
  useListLeaveTypes,
} from '@swp/api-client/e6';
import { formatDate } from '@swp/shared/datetime';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Pressable, TextInput, View } from 'react-native';
import { useSession } from '../src/providers/session';
import { Button } from '../src/ui/Button';
import { Screen } from '../src/ui/Screen';
import { Text } from '../src/ui/Text';

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

export default function LeaveNew() {
  const { t } = useTranslation();
  const router = useRouter();
  const { user } = useSession();
  const employeeId = user?.employee_id ?? '';
  const typesQ = useListLeaveTypes();
  const create = useCreateLeaveRequest();

  const balancesQ = useGetEmployeeTypeBalances(employeeId, { query: { enabled: !!employeeId } });
  const balances = (balancesQ.data?.data as { data?: LeaveTypeBalance[] } | undefined)?.data ?? [];

  const allTypes = (typesQ.data?.data as { data?: LeaveType[] } | undefined)?.data ?? [];
  // Document-required types are deferred (no attachment upload yet).
  const types = allTypes.filter(
    (lt) => lt.status === LeaveTypeStatus.ACTIVE && !lt.requires_document,
  );

  const [typeId, setTypeId] = useState('');
  const [start, setStart] = useState('');
  const [end, setEnd] = useState('');
  const [reason, setReason] = useState('');
  const [err, setErr] = useState<string | null>(null);

  async function onSubmit() {
    setErr(null);
    if (!typeId) return setErr(t('m:leave.pickType'));
    if (!DATE_RE.test(start) || !DATE_RE.test(end)) return setErr(t('m:leave.badDate'));
    if (end < start) return setErr(t('m:leave.badRange'));
    if (reason.trim().length < 5) return setErr(t('m:leave.badReason'));
    try {
      await create.mutateAsync({
        data: {
          leave_type_id: typeId,
          start_date: start,
          end_date: end,
          reason: reason.trim(),
          submit: true,
        },
      });
      Alert.alert(t('m:leave.title'), t('m:leave.success'));
      router.back();
    } catch (e) {
      if (e instanceof ApiError) {
        const map: Record<string, string> = {
          OVERLAPPING_LEAVE: 'overlap',
          QUOTA_EXCEEDED: 'quota',
          BACKDATED_LEAVE: 'backdated',
          MISSING_REQUIRED_DOCUMENT: 'needDoc',
          INVALID_DATE_RANGE: 'badRange',
        };
        const key = map[e.code];
        if (key) return setErr(t(`m:leave.${key}`));
      }
      setErr(t('m:leave.error'));
    }
  }

  const inputClass = 'rounded-input border border-border bg-surface px-4 py-3 text-text';

  // Per-type balance hint for the selected type (frame QT92D's green banner). Only types with a
  // tracked depleting pool (ANNUAL_POOL / PER_YEAR_COUNT → entitled + remaining present) show it.
  const selectedBal = balances.find((b) => b.leave_type_id === typeId);
  const showHint =
    selectedBal != null && selectedBal.entitled_days != null && selectedBal.remaining_days != null;
  const hintDesc = selectedBal?.expires_at
    ? t('m:leave.bal.expires', { date: formatDate(selectedBal.expires_at) })
    : t('m:leave.bal.annual');

  return (
    <Screen>
      <View className="mb-6 flex-row items-center justify-between">
        <Text variant="title">{t('m:leave.newBtn')}</Text>
        <Pressable onPress={() => router.back()}>
          <Text variant="strong" className="text-primary">
            {t('m:clock.cancel')}
          </Text>
        </Pressable>
      </View>

      <View className="gap-4">
        <View>
          <Text variant="caption" className="mb-1">
            {t('m:leave.type')}
          </Text>
          <View className="flex-row flex-wrap gap-2">
            {types.map((lt) => (
              <Pressable
                key={lt.id}
                onPress={() => setTypeId(lt.id)}
                className={`rounded-input border px-3 py-2 ${
                  typeId === lt.id ? 'border-primary bg-primary-soft' : 'border-border bg-surface'
                }`}
              >
                <Text
                  weight={typeId === lt.id ? 'semibold' : 'regular'}
                  className={typeId === lt.id ? 'text-primary' : 'text-text-2'}
                >
                  {lt.name}
                </Text>
              </Pressable>
            ))}
          </View>
        </View>

        {showHint && selectedBal ? (
          <View className="rounded-input border border-ok-border bg-ok-bg px-4 py-3">
            <Text variant="strong" className="text-ok-text">
              {t('m:leave.balHint', {
                name: selectedBal.name,
                code: selectedBal.code,
                remaining: selectedBal.remaining_days,
              })}
            </Text>
            <Text variant="caption" className="text-ok-text mt-0.5">
              {t('m:leave.balHintSub', { entitled: selectedBal.entitled_days, desc: hintDesc })}
            </Text>
          </View>
        ) : null}

        <View className="flex-row gap-3">
          <View className="flex-1">
            <Text variant="caption" className="mb-1">
              {t('m:leave.startDate')}
            </Text>
            <TextInput
              value={start}
              onChangeText={setStart}
              placeholder="2026-06-10"
              autoCapitalize="none"
              className={inputClass}
            />
          </View>
          <View className="flex-1">
            <Text variant="caption" className="mb-1">
              {t('m:leave.endDate')}
            </Text>
            <TextInput
              value={end}
              onChangeText={setEnd}
              placeholder="2026-06-12"
              autoCapitalize="none"
              className={inputClass}
            />
          </View>
        </View>

        <View>
          <Text variant="caption" className="mb-1">
            {t('m:leave.reason')}
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

        <View className="rounded-input border border-warn-border bg-warn-bg px-4 py-3">
          <Text variant="caption" className="text-warn-text">
            {t('m:leave.quotaWarn')}
          </Text>
        </View>

        {err ? <Text className="text-danger">{err}</Text> : null}

        <Button label={t('m:leave.submit')} onPress={onSubmit} loading={create.isPending} />
      </View>
    </Screen>
  );
}
