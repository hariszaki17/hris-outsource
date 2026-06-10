// Matches .pen "Agen · Reset Kata Sandi (Mobile)" (Y6feM)
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';
import { TextField } from '../../src/ui/TextField';

function ReqRow({ met, label }: { met: boolean; label: string }) {
  return (
    <View className="flex-row items-center gap-2">
      <Text className={met ? 'text-ok-text' : 'text-text-3'}>{met ? '✓' : '○'}</Text>
      <Text variant="caption" className={met ? 'text-text' : 'text-text-3'}>
        {label}
      </Text>
    </View>
  );
}

export default function ResetPassword() {
  const { t } = useTranslation();
  const [newPass, setNewPass] = useState('');
  const [confirmPass, setConfirmPass] = useState('');

  const minChars = newPass.length >= 8;
  const hasUpper = /[A-Z]/.test(newPass);
  const hasNumber = /[0-9]/.test(newPass);

  async function onSubmit() {
    // TODO: wire to auth API
  }

  return (
    <ScrollView className="flex-1 bg-app-bg" contentContainerStyle={{ flexGrow: 1 }}>
      <View className="flex-1 justify-between px-7 pb-7" style={{ paddingTop: 40 }}>
        <View className="gap-5">
          <Text className="text-2xl font-bold text-text">{t('m:reset.newPasswordLabel')}</Text>
          <Text variant="caption" className="text-text-3" style={{ lineHeight: 20 }}>
            {t('m:reset.subtitle')}
          </Text>

          <View className="gap-3.5">
            <TextField
              label={t('m:reset.newPasswordLabel')}
              value={newPass}
              onChangeText={setNewPass}
              placeholder={t('m:reset.newPasswordPlaceholder')}
              secureTextEntry
            />
            <TextField
              label={t('m:reset.confirmPasswordLabel')}
              value={confirmPass}
              onChangeText={setConfirmPass}
              placeholder={t('m:reset.confirmPasswordPlaceholder')}
              secureTextEntry
            />
          </View>

          <View className="gap-1 pt-1">
            <ReqRow met={minChars} label={t('m:reset.reqMinChars')} />
            <ReqRow met={hasUpper} label={t('m:reset.reqUpper')} />
            <ReqRow met={hasNumber} label={t('m:reset.reqNumber')} />
          </View>

          <Button
            label={t('m:reset.saveBtn')}
            onPress={() => void onSubmit()}
            disabled={!minChars || !hasUpper || !hasNumber || newPass !== confirmPass}
          />
        </View>

        <Text variant="caption" className="text-center text-text-3">
          {t('m:login.footer')}
        </Text>
      </View>
    </ScrollView>
  );
}
