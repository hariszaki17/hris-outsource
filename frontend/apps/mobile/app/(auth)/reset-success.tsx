// Matches .pen "Agen · Reset Berhasil (Mobile)" (gNzLP)
import { useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ShieldCheck } from 'lucide-react-native';
import { ScrollView, View } from 'react-native';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';

export default function ResetSuccess() {
  const { t } = useTranslation();
  const router = useRouter();

  return (
    <ScrollView className="flex-1 bg-app-bg" contentContainerStyle={{ flexGrow: 1 }}>
      <View className="flex-1 justify-between px-7 pb-7" style={{ paddingTop: 40 }}>
        <View className="items-center gap-5">
          <View className="h-6 w-full" />
          <View className="h-[72px] w-[72px] items-center justify-center rounded-full bg-ok-bg">
            <ShieldCheck size={34} color="#0F766E" />
          </View>
          <View className="items-center gap-2">
            <Text className="text-2xl font-bold text-text">{t('m:reset.successTitle')}</Text>
            <Text variant="caption" className="text-center text-text-3" style={{ lineHeight: 20 }}>
              {t('m:reset.successBody')}
            </Text>
          </View>
          <Button label={t('m:reset.successBtn')} onPress={() => router.replace('/login')} />
        </View>
        <Text variant="caption" className="text-center text-text-3">{t('m:login.footer')}</Text>
      </View>
    </ScrollView>
  );
}
