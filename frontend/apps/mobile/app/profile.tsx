import { type Employee, useCreateChangeRequest, useGetEmployee } from '@swp/api-client/e2';
import { useRouter } from 'expo-router';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Alert, Pressable, TextInput, View } from 'react-native';
import { useSession } from '../src/providers/session';
import { Button } from '../src/ui/Button';
import { Screen } from '../src/ui/Screen';
import { Text } from '../src/ui/Text';

export default function ProfileScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { user } = useSession();
  const employeeId = user?.employee_id ?? '';
  const q = useGetEmployee(employeeId, { query: { enabled: !!employeeId } });
  const create = useCreateChangeRequest();

  const emp = q.data?.data as Employee | undefined;
  const [phone, setPhone] = useState('');
  const [address, setAddress] = useState('');
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (emp) {
      setPhone(emp.phone ?? '');
      setAddress(emp.address ?? '');
    }
  }, [emp]);

  async function onSave() {
    setErr(null);
    const changes: { phone?: string; address?: string } = {};
    if (emp && phone !== (emp.phone ?? '')) changes.phone = phone;
    if (emp && address !== (emp.address ?? '')) changes.address = address;
    if (!changes.phone && !changes.address) return setErr(t('m:profile.noChange'));
    try {
      await create.mutateAsync({ employeeId, data: { changes } });
      Alert.alert(t('m:profile.title'), t('m:profile.success'));
      router.back();
    } catch {
      setErr(t('m:profile.error'));
    }
  }

  const inputClass = 'rounded-input border border-border bg-surface px-4 py-3 text-text';

  if (q.isLoading) {
    return (
      <Screen>
        <View className="items-center py-10">
          <ActivityIndicator />
        </View>
      </Screen>
    );
  }

  return (
    <Screen>
      <View className="mb-6 flex-row items-center justify-between">
        <Text variant="title">{t('m:profile.title')}</Text>
        <Pressable onPress={() => router.back()}>
          <Text className="text-primary font-semibold">{t('m:clock.cancel')}</Text>
        </Pressable>
      </View>

      <View className="gap-4">
        <View>
          <Text variant="caption" className="mb-1">
            {t('m:profile.name')}
          </Text>
          <Text variant="body">{emp?.full_name ?? '—'}</Text>
        </View>
        <View>
          <Text variant="caption" className="mb-1">
            {t('m:profile.phone')}
          </Text>
          <TextInput
            value={phone}
            onChangeText={setPhone}
            keyboardType="phone-pad"
            className={inputClass}
          />
        </View>
        <View>
          <Text variant="caption" className="mb-1">
            {t('m:profile.address')}
          </Text>
          <TextInput
            value={address}
            onChangeText={setAddress}
            multiline
            className={`${inputClass} h-24`}
            style={{ textAlignVertical: 'top' }}
          />
        </View>

        {err ? <Text className="text-danger">{err}</Text> : null}

        <Button label={t('m:profile.save')} onPress={onSave} loading={create.isPending} />
      </View>
    </Screen>
  );
}
