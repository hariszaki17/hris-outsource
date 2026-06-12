// Matches .pen "Agen · Lupa Kata Sandi (Mobile)" (wHKXQ).
// Brand chip (logo emblem) + wordmark, section title, muted subtitle, email field,
// primary "Kirim Tautan Reset", "← Kembali ke masuk" primary link, footer.
import { color } from '@swp/design-tokens';
import { useRouter } from 'expo-router';
import { ArrowLeft, Mail } from 'lucide-react-native';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Image, Pressable, ScrollView, View } from 'react-native';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';
import { TextField } from '../../src/ui/TextField';

export default function ForgotPassword() {
  const { t } = useTranslation();
  const router = useRouter();
  const [email, setEmail] = useState('');

  async function onSubmit() {
    // TODO(E1 /auth): wire to the password-reset request endpoint.
    router.push({ pathname: '/reset-sent', params: { email } });
  }

  return (
    <ScrollView className="flex-1 bg-surface" contentContainerStyle={{ flexGrow: 1 }}>
      <View className="flex-1 pb-7" style={{ paddingTop: 60 }}>
        <View className="flex-1 px-7">
          {/* Brand: chip 64×64 r16 + wordmark (Poppins 22/700) + subtitle */}
          <View className="items-center gap-3.5">
            <View className="h-16 w-16 items-center justify-center rounded-2xl border border-border bg-surface">
              <Image
                source={require('../../assets/swp-logo.png')}
                className="h-11 w-11"
                resizeMode="contain"
              />
            </View>
            <Text className="font-display text-[22px] font-bold text-text">SaranaWisesa</Text>
            <Text className="text-[13px] text-text-3">HRIS Outsource</Text>
          </View>

          <View className="gap-3.5 pt-8">
            <Text variant="section">{t('m:reset.title')}</Text>
            <Text variant="secondary" className="text-text-3" style={{ lineHeight: 22 }}>
              {t('m:reset.subtitle')}
            </Text>

            <TextField
              label={t('m:reset.emailLabel')}
              value={email}
              onChangeText={setEmail}
              placeholder={t('m:reset.emailPlaceholder')}
              keyboardType="email-address"
              autoCapitalize="none"
              autoCorrect={false}
            >
              <Mail size={16} color={color.text3} />
            </TextField>

            <Button label={t('m:reset.submit')} onPress={() => void onSubmit()} disabled={!email} />

            <Pressable
              onPress={() => router.back()}
              className="flex-row items-center justify-center gap-1.5 pt-1.5"
              hitSlop={8}
            >
              <ArrowLeft size={16} color={color.primary} />
              <Text className="text-[13px] font-semibold text-primary">
                {t('m:reset.backToLoginLink')}
              </Text>
            </Pressable>
          </View>
        </View>

        <Text className="text-center text-[12px] text-text-3">{t('m:login.footer')}</Text>
      </View>
    </ScrollView>
  );
}
