// Matches .pen "Agen · Reset Berhasil (Mobile)" (gNzLP).
// Left-aligned success: teal shield-check chip, section title, muted body,
// full-width primary "Masuk Sekarang", footer.
import { color } from '@swp/design-tokens';
import { useRouter } from 'expo-router';
import { ShieldCheck } from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';

export default function ResetSuccess() {
  const { t } = useTranslation();
  const router = useRouter();
  const insets = useSafeAreaInsets();

  return (
    <ScrollView className="flex-1 bg-surface" contentContainerStyle={{ flexGrow: 1 }}>
      <View
        className="flex-1"
        style={{ paddingTop: insets.top + 8, paddingBottom: insets.bottom + 28 }}
      >
        <View className="flex-1 px-7">
          <View className="h-[72px] w-[72px] items-center justify-center rounded-full bg-ok-bg">
            <ShieldCheck size={34} color={color.ok.text} />
          </View>

          {/* Centered headline + body (.pen Txt: alignItems center, gap 8). Icon chip stays
              left; the full-width button must NOT inherit centering, so it sits outside. */}
          <View className="items-center gap-2 pt-[22px]">
            <Text variant="displayTitle" className="text-center">
              {t('m:reset.successTitle')}
            </Text>
            <Text
              variant="secondary"
              className="text-center text-text-3"
              style={{ lineHeight: 22 }}
            >
              {t('m:reset.successBody')}
            </Text>
          </View>

          <Button
            label={t('m:reset.successBtn')}
            onPress={() => router.replace('/login')}
            className="mt-[22px]"
          />
        </View>

        <Text variant="caption" className="text-center text-text-3">
          {t('m:login.footer')}
        </Text>
      </View>
    </ScrollView>
  );
}
