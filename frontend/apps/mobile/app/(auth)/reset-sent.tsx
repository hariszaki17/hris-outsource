// Matches .pen "Agen · Tautan Reset Terkirim (Mobile)" (Kl6wT)
import { useTranslation } from 'react-i18next';
import { MailCheck } from 'lucide-react-native';
import { Pressable, ScrollView, View } from 'react-native';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';

export default function ResetSent() {
  const { t } = useTranslation();

  return (
    <ScrollView className="flex-1 bg-app-bg" contentContainerStyle={{ flexGrow: 1 }}>
      <View className="flex-1 justify-between px-7 pb-7" style={{ paddingTop: 40 }}>
        <View className="items-center gap-5">
          <View className="h-6 w-full" />
          <View className="h-[72px] w-[72px] items-center justify-center rounded-full bg-ok-bg">
            <MailCheck size={34} color="#0F766E" />
          </View>
          <View className="items-center gap-2">
            <Text className="text-2xl font-bold text-text">{t('m:reset.sentTitle')}</Text>
            <Text variant="caption" className="text-center text-text-3" style={{ lineHeight: 20 }}>
              {t('m:reset.sentBody', { email: 'nama@swp.id' })}
            </Text>
          </View>
          <Button label={t('m:reset.backToLogin')} onPress={() => {}} />
          <Pressable className="flex-row items-center gap-1.5 pt-1">
            <Text variant="caption" className="text-text-3">{t('m:reset.resendNotReceived')}</Text>
            <Text variant="caption" className="font-semibold text-primary">{t('m:reset.resendAction')}</Text>
          </Pressable>
        </View>
        <Text variant="caption" className="text-center text-text-3">{t('m:login.footer')}</Text>
      </View>
    </ScrollView>
  );
}
