// Matches .pen "Agen · Tautan Reset Terkirim (Mobile)" (Kl6wT).
// Left-aligned confirmation: teal mail-check chip, section title, muted body,
// full-width primary "Kembali ke Masuk", centered "Tidak menerima email? Kirim ulang".
import { color } from '@swp/design-tokens';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { MailCheck } from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { Pressable, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';

export default function ResetSent() {
  const { t } = useTranslation();
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { email } = useLocalSearchParams<{ email?: string }>();

  return (
    <ScrollView className="flex-1 bg-surface" contentContainerStyle={{ flexGrow: 1 }}>
      <View
        className="flex-1"
        style={{ paddingTop: insets.top + 8, paddingBottom: insets.bottom + 28 }}
      >
        <View className="flex-1 px-7">
          <View className="h-[72px] w-[72px] items-center justify-center rounded-full bg-ok-bg">
            <MailCheck size={34} color={color.ok.text} />
          </View>

          {/* Centered headline + body (.pen Txt: alignItems center, gap 8). Icon chip stays
              left; the full-width button must NOT inherit centering, so it sits outside. */}
          <View className="items-center gap-2 pt-[22px]">
            <Text variant="displayTitle" className="text-center">
              {t('m:reset.sentTitle')}
            </Text>
            <Text
              variant="secondary"
              className="text-center text-text-3"
              style={{ lineHeight: 22 }}
            >
              {t('m:reset.sentBody', { email: email ?? 'nama@swp.id' })}
            </Text>
          </View>

          <Button
            label={t('m:reset.backToLogin')}
            onPress={() => router.replace('/login')}
            className="mt-[22px]"
          />

          <Pressable className="flex-row items-center justify-center gap-1.5 pt-[22px]" hitSlop={8}>
            <Text variant="caption" className="text-text-3">
              {t('m:reset.resendNotReceived')}
            </Text>
            <Text variant="caption" weight="semibold" className="text-primary">
              {t('m:reset.resendAction')}
            </Text>
          </Pressable>
        </View>

        <Text variant="caption" className="text-center text-text-3">
          {t('m:login.footer')}
        </Text>
      </View>
    </ScrollView>
  );
}
