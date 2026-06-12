// Matches .pen "Agen · Reset Berhasil (Mobile)" (gNzLP).
// Left-aligned success: teal shield-check chip, section title, muted body,
// full-width primary "Masuk Sekarang", footer.
import { color } from '@swp/design-tokens';
import { useRouter } from 'expo-router';
import { ShieldCheck } from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';

export default function ResetSuccess() {
  const { t } = useTranslation();
  const router = useRouter();

  return (
    <ScrollView className="flex-1 bg-surface" contentContainerStyle={{ flexGrow: 1 }}>
      <View className="flex-1 pb-7" style={{ paddingTop: 60 }}>
        <View className="flex-1 px-7">
          <View className="h-[72px] w-[72px] items-center justify-center rounded-full bg-ok-bg">
            <ShieldCheck size={34} color={color.ok.text} />
          </View>

          <View className="gap-3.5 pt-6">
            <Text variant="section">{t('m:reset.successTitle')}</Text>
            <Text variant="secondary" className="text-text-3" style={{ lineHeight: 22 }}>
              {t('m:reset.successBody')}
            </Text>

            <Button
              label={t('m:reset.successBtn')}
              onPress={() => router.replace('/login')}
              className="mt-2"
            />
          </View>
        </View>

        <Text className="text-center text-[12px] text-text-3">{t('m:login.footer')}</Text>
      </View>
    </ScrollView>
  );
}
