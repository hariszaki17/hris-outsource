import { type Href, useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { Alert, Pressable, View } from 'react-native';
import { useSession } from '../../src/providers/session';
import { Button } from '../../src/ui/Button';
import { Card } from '../../src/ui/Card';
import { Text } from '../../src/ui/Text';

// Feature sections reachable from More (added per phase: leave=17, OT=18, payslip=19, profile=20).
const MENU: { key: 'leave' | 'overtime'; href: Href }[] = [
  { key: 'leave', href: '/leave' },
  { key: 'overtime', href: '/overtime' },
];

export default function More() {
  const { t } = useTranslation();
  const router = useRouter();
  const { user, signOut } = useSession();

  return (
    <View className="flex-1 gap-4 bg-app-bg p-6">
      <Card>
        <Text variant="title">{user?.full_name ?? ''}</Text>
        <Text variant="caption" className="mt-1">
          {user?.phone ?? ''}
        </Text>
      </Card>

      <View className="gap-2">
        {MENU.map((m) => (
          <Pressable key={m.key} onPress={() => router.push(m.href)}>
            <Card>
              <Text variant="body" className="font-semibold">
                {t(`m:menu.${m.key}`)}
              </Text>
            </Card>
          </Pressable>
        ))}
      </View>

      <Button
        variant="secondary"
        label={t('m:more.changePassword')}
        onPress={() => Alert.alert(t('m:more.changePassword'), t('m:common.comingSoon'))}
      />
      <Button label={t('m:more.signOut')} onPress={() => void signOut()} />
    </View>
  );
}
