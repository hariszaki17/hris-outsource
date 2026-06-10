// Matches .pen "Agen · Lupa Kata Sandi (Mobile)" (wHKXQ)
import { Mail } from 'lucide-react-native';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';
import { TextField } from '../../src/ui/TextField';

export default function ForgotPassword() {
  const { t } = useTranslation();
  const [email, setEmail] = useState('');

  async function onSubmit() {
    // TODO: wire to auth API
  }

  return (
    <ScrollView className="flex-1 bg-app-bg" contentContainerStyle={{ flexGrow: 1 }}>
      <View className="flex-1 justify-between px-7 pb-7" style={{ paddingTop: 60 }}>
        <View className="items-center gap-3.5">
          <View className="h-14 w-14 items-center justify-center rounded-2xl border border-border bg-surface">
            <View className="h-10 w-10 items-center justify-center rounded-xl bg-primary">
              <Text className="text-lg font-bold text-surface">S</Text>
            </View>
          </View>
          <View className="items-center gap-1">
            <Text className="text-2xl font-bold text-text">SaranaWisesa</Text>
            <Text variant="caption" className="text-text-3">HRIS Outsource</Text>
          </View>
        </View>

        <View className="gap-3.5">
          <Text className="text-2xl font-bold text-text">{t('m:reset.title')}</Text>
          <Text variant="caption" className="text-text-3" style={{ lineHeight: 20 }}>
            {t('m:reset.subtitle')}
          </Text>

          <TextField
            label={t('m:reset.emailLabel')}
            value={email}
            onChangeText={setEmail}
            placeholder={t('m:reset.emailPlaceholder')}
            keyboardType="email-address"
            autoCapitalize="none"
          >
            <Mail size={16} color="#9CA3AF" />
          </TextField>

          <Button label={t('m:reset.submit')} onPress={() => void onSubmit()} disabled={!email} />
        </View>

        <Text variant="caption" className="text-center text-text-3">
          {t('m:login.footer')}
        </Text>
      </View>
    </ScrollView>
  );
}
