// Matches .pen "Agen · Tautan Reset Terkirim (Mobile)" (Kl6wT).
// Left-aligned confirmation: teal mail-check chip, section title, muted body,
// full-width primary "Kembali ke Masuk", centered "Tidak menerima email? Kirim ulang".
import { color } from '@swp/design-tokens';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { MailCheck } from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { Pressable, ScrollView, View } from 'react-native';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';

export default function ResetSent() {
  const { t } = useTranslation();
  const router = useRouter();
  const { email } = useLocalSearchParams<{ email?: string }>();

  return (
    <ScrollView className="flex-1 bg-surface" contentContainerStyle={{ flexGrow: 1 }}>
      <View className="flex-1 pb-7" style={{ paddingTop: 60 }}>
        <View className="flex-1 px-7">
          <View className="h-[72px] w-[72px] items-center justify-center rounded-full bg-ok-bg">
            <MailCheck size={34} color={color.ok.text} />
          </View>

          <View className="gap-3.5 pt-6">
            <Text variant="section">{t('m:reset.sentTitle')}</Text>
            <Text variant="secondary" className="text-text-3" style={{ lineHeight: 22 }}>
              {t('m:reset.sentBody', { email: email ?? 'nama@swp.id' })}
            </Text>

            <Button
              label={t('m:reset.backToLogin')}
              onPress={() => router.replace('/login')}
              className="mt-2"
            />

            <Pressable className="flex-row items-center justify-center gap-1.5 pt-2" hitSlop={8}>
              <Text className="text-[13px] text-text-3">{t('m:reset.resendNotReceived')}</Text>
              <Text className="text-[13px] font-semibold text-primary">
                {t('m:reset.resendAction')}
              </Text>
            </Pressable>
          </View>
        </View>

        <Text className="text-center text-[12px] text-text-3">{t('m:login.footer')}</Text>
      </View>
    </ScrollView>
  );
}
