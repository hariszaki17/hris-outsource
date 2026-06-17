// Change Password (E1 / EP-3 forced temp-password rotation + voluntary change).
// Mirrors the "Reset Kata Sandi (Mobile)" layout: title + muted subtitle, an info notice,
// two password fields (eye toggle), a live requirement checklist (teal check met /
// gray circle unmet), and a full-width "Simpan Kata Sandi".
//
// Authenticated screen — the access token is already set, so the mutation is authorized.
// On success the BE invalidates ALL sessions (AU-6), so we sign out and bounce to login to
// re-authenticate with the new password. Password policy mirrors the BE (AU-4): min 10
// chars + upper + lower + digit + symbol.
import { useAuthChangePassword } from '@swp/api-client/e1';
import { color } from '@swp/design-tokens';
import { Check, Circle, Eye, EyeOff } from 'lucide-react-native';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Pressable, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { useSession } from '../src/providers/session';
import { Button } from '../src/ui/Button';
import { Text } from '../src/ui/Text';
import { TextField } from '../src/ui/TextField';

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

export default function ChangePassword() {
  const { t } = useTranslation();
  const insets = useSafeAreaInsets();
  const { signOut } = useSession();
  const changePassword = useAuthChangePassword();
  const [newPass, setNewPass] = useState('');
  const [confirmPass, setConfirmPass] = useState('');
  const [showNew, setShowNew] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const reqMin = newPass.length >= 10;
  const reqCase = /[a-z]/.test(newPass) && /[A-Z]/.test(newPass);
  const reqNum = /[0-9]/.test(newPass);
  const reqSym = /[^A-Za-z0-9]/.test(newPass);
  const matches = newPass.length > 0 && newPass === confirmPass;
  const canSubmit = reqMin && reqCase && reqNum && reqSym && matches;

  async function onSubmit() {
    setError(null);
    try {
      await changePassword.mutateAsync({ data: { new_password: newPass } });
      Alert.alert(t('m:changePassword.successTitle'), t('m:changePassword.successBody'), [
        // Sessions are revoked server-side — sign out locally and return to login.
        { text: 'OK', onPress: () => void signOut() },
      ]);
    } catch (e) {
      const status = (e as { status?: number }).status;
      setError(
        status === 422 ? t('m:changePassword.errorWeak') : t('m:changePassword.errorGeneric'),
      );
    }
  }

  return (
    <ScrollView className="flex-1 bg-surface" contentContainerStyle={{ flexGrow: 1 }}>
      <View
        className="flex-1"
        style={{ paddingTop: insets.top + 8, paddingBottom: insets.bottom + 28 }}
      >
        <View className="flex-1 px-7">
          <View className="gap-3.5">
            <Text variant="displayTitle">{t('m:changePassword.title')}</Text>
            <Text variant="secondary" className="text-text-3" style={{ lineHeight: 22 }}>
              {t('m:changePassword.subtitle')}
            </Text>

            {/* Info notice — explains the forced temp-password rotation. */}
            <View className="gap-0.5 rounded-input border border-info-border bg-info-bg px-3 py-2.5">
              <Text variant="caption" weight="bold" className="text-info-text">
                {t('m:changePassword.noticeTitle')}
              </Text>
              <Text variant="badge" weight="regular" className="text-info-text">
                {t('m:changePassword.noticeBody')}
              </Text>
            </View>

            <TextField
              label={t('m:changePassword.newPasswordLabel')}
              value={newPass}
              onChangeText={setNewPass}
              placeholder={t('m:changePassword.newPasswordPlaceholder')}
              secureTextEntry={!showNew}
              autoCapitalize="none"
              autoCorrect={false}
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
              label={t('m:changePassword.confirmPasswordLabel')}
              value={confirmPass}
              onChangeText={setConfirmPass}
              placeholder={t('m:changePassword.confirmPasswordPlaceholder')}
              secureTextEntry={!showConfirm}
              autoCapitalize="none"
              autoCorrect={false}
            >
              <Pressable onPress={() => setShowConfirm(!showConfirm)} hitSlop={8}>
                {showConfirm ? (
                  <EyeOff size={16} color={color.text3} />
                ) : (
                  <Eye size={16} color={color.text3} />
                )}
              </Pressable>
            </TextField>

            {error ? (
              <Text variant="caption" className="text-bad-text">
                {error}
              </Text>
            ) : null}
          </View>

          <View className="gap-1 pt-[22px]">
            <ReqRow met={reqMin} label={t('m:changePassword.reqMinChars')} />
            <ReqRow met={reqCase} label={t('m:changePassword.reqCase')} />
            <ReqRow met={reqNum} label={t('m:changePassword.reqNumber')} />
            <ReqRow met={reqSym} label={t('m:changePassword.reqSymbol')} />
          </View>

          <Button
            label={t('m:changePassword.saveBtn')}
            onPress={() => void onSubmit()}
            disabled={!canSubmit}
            loading={changePassword.isPending}
            className="mt-[22px]"
          />
        </View>
      </View>
    </ScrollView>
  );
}
