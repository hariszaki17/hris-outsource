// Matches .pen "Agen · Reset Kata Sandi (Mobile)" (Y6feM).
// Section title + muted subtitle, two password fields (eye toggle), live requirement
// checklist (teal check met / gray circle unmet), full-width "Simpan Kata Sandi".
import { color } from '@swp/design-tokens';
import { useRouter } from 'expo-router';
import { Check, Circle, Eye, EyeOff } from 'lucide-react-native';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Pressable, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';
import { TextField } from '../../src/ui/TextField';

function ReqRow({ met, label }: { met: boolean; label: string }) {
  return (
    <View className="flex-row items-center gap-2">
      {met ? <Check size={14} color={color.ok.text} /> : <Circle size={14} color={color.text3} />}
      <Text variant="caption" className={met ? 'text-ok-text' : 'text-text-3'}>
        {label}
      </Text>
    </View>
  );
}

export default function ResetPassword() {
  const { t } = useTranslation();
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const [newPass, setNewPass] = useState('');
  const [confirmPass, setConfirmPass] = useState('');
  const [showNew, setShowNew] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);

  const minChars = newPass.length >= 8;
  const hasUpperLower = /[a-z]/.test(newPass) && /[A-Z]/.test(newPass);
  const hasNumberSymbol = /[0-9]/.test(newPass) || /[^A-Za-z0-9]/.test(newPass);
  const matches = newPass.length > 0 && newPass === confirmPass;

  async function onSubmit() {
    // TODO(E1 /auth): wire to the password-reset confirm endpoint.
    router.replace('/reset-success');
  }

  return (
    <ScrollView className="flex-1 bg-surface" contentContainerStyle={{ flexGrow: 1 }}>
      <View
        className="flex-1"
        style={{ paddingTop: insets.top + 8, paddingBottom: insets.bottom + 28 }}
      >
        <View className="flex-1 px-7">
          {/* .pen Form: title, subtitle, both fields share one uniform 14px gap. */}
          <View className="gap-3.5">
            <Text variant="displayTitle">{t('m:reset.setNewTitle')}</Text>
            <Text variant="secondary" className="text-text-3" style={{ lineHeight: 22 }}>
              {t('m:reset.setNewSubtitle')}
            </Text>

            <TextField
              label={t('m:reset.newPasswordLabel')}
              value={newPass}
              onChangeText={setNewPass}
              placeholder={t('m:reset.newPasswordPlaceholder')}
              secureTextEntry={!showNew}
            >
              <Pressable onPress={() => setShowNew(!showNew)} hitSlop={8}>
                {showNew ? (
                  <EyeOff size={16} color={color.text3} />
                ) : (
                  <Eye size={16} color={color.text3} />
                )}
              </Pressable>
            </TextField>
            <TextField
              label={t('m:reset.confirmPasswordLabel')}
              value={confirmPass}
              onChangeText={setConfirmPass}
              placeholder={t('m:reset.confirmPasswordPlaceholder')}
              secureTextEntry={!showConfirm}
            >
              <Pressable onPress={() => setShowConfirm(!showConfirm)} hitSlop={8}>
                {showConfirm ? (
                  <EyeOff size={16} color={color.text3} />
                ) : (
                  <Eye size={16} color={color.text3} />
                )}
              </Pressable>
            </TextField>
          </View>

          <View className="gap-1 pt-[22px]">
            <ReqRow met={minChars} label={t('m:reset.reqMinChars')} />
            <ReqRow met={hasUpperLower} label={t('m:reset.reqUpper')} />
            <ReqRow met={hasNumberSymbol} label={t('m:reset.reqNumber')} />
          </View>

          <Button
            label={t('m:reset.saveBtn')}
            onPress={() => void onSubmit()}
            disabled={!minChars || !hasUpperLower || !hasNumberSymbol || !matches}
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
