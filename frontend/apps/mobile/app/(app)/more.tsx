import { useTranslation } from 'react-i18next';
import { Alert, View } from 'react-native';
import { useSession } from '../../src/providers/session';
import { Button } from '../../src/ui/Button';
import { Card } from '../../src/ui/Card';
import { Text } from '../../src/ui/Text';

export default function More() {
  const { t } = useTranslation();
  const { user, signOut } = useSession();

  return (
    <View className="flex-1 gap-4 bg-app-bg p-6">
      <Card>
        <Text variant="title">{user?.full_name ?? ''}</Text>
        <Text variant="caption" className="mt-1">
          {user?.phone ?? ''}
        </Text>
      </Card>

      <Button
        variant="secondary"
        label={t('m:more.changePassword')}
        onPress={() => Alert.alert(t('m:more.changePassword'), t('m:common.comingSoon'))}
      />
      <Button label={t('m:more.signOut')} onPress={() => void signOut()} />
    </View>
  );
}
