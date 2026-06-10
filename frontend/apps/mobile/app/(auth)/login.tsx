// Matches brainstorm.pen "Agen · Login (Mobile)" (Y09E0) + error state variants.
// Icons from comp/TextField: mail on right side, eye toggles password visibility.
// Logo from swp-logo.png. Brand section: 3 siblings at 14px gap (no inner wrapper).
import { type LoginResponse, useAuthLogin } from '@swp/api-client/e1';
import { color } from '@swp/design-tokens';
import { useRouter } from 'expo-router';
import { CircleX, Eye, EyeOff, Mail } from 'lucide-react-native';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Image, Pressable, ScrollView, View } from 'react-native';
import { useSession } from '../../src/providers/session';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';
import { TextField } from '../../src/ui/TextField';


function ErrorBox({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <View className="flex-row items-start gap-2 rounded-input border border-bad-border bg-bad-bg px-3 py-2.5">
      <CircleX size={14} color={color.bad.text} />
      <View className="shrink gap-0.5">
        <Text
          variant="caption"
          className="font-bold leading-[1.4] text-bad-text"
        >
          {title}
        </Text>
        {subtitle ? (
          <Text
            variant="caption"
            className="text-[11px] leading-[1.4] text-bad-text"
          >
            {subtitle}
          </Text>
        ) : null}
      </View>
    </View>
  );
}

export default function Login() {
  const { t } = useTranslation();
  const { signIn } = useSession();
  const router = useRouter();
  const login = useAuthLogin();
  const [identifier, setIdentifier] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [remainingAttempts, setRemainingAttempts] = useState<number | null>(
    null,
  );

  async function onSubmit() {
    setError(null);
    setRemainingAttempts(null);
    try {
      const res = await login.mutateAsync({ data: { identifier, password } });
      await signIn(res.data as LoginResponse);
      router.replace('/');
    } catch (e) {
      const err = e as { status?: number; body?: { remaining_attempts?: number } };
      const status = err.status;
      if (status === 401) {
        setError(t('m:login.errorInvalid'));
        setRemainingAttempts(err.body?.remaining_attempts ?? null);
      } else if (status === 423) setError(t('m:login.errorLocked'));
      else if (status === 410) setError(t('m:login.errorDisabled'));
      else setError(t('m:common.errorGeneric'));
    }
  }

  const showError = error !== null;

  return (
    <ScrollView
      className="flex-1 bg-surface"
      contentContainerStyle={{ flexGrow: 1 }}
    >
      <View className="flex-1 pb-7" style={{ paddingTop: 60 }}>
        {/* Vertically-center the form block */}
        <View className="flex-1 justify-center px-7">
          <View className="gap-[22px]">
            {/* Brand: logo + title + subtitle at 14px gap each, mirrors design */}
            <View className="items-center gap-3.5">
              <View className="h-16 w-16 items-center justify-center rounded-2xl border border-border bg-surface">
                <Image
                  source={require('../../assets/swp-logo.png')}
                  className="h-11 w-11"
                  resizeMode="contain"
                />
              </View>
              <Text className="font-['Poppins'] text-2xl font-bold text-text">
                SaranaWisesa
              </Text>
              <Text className="font-sans text-[11px] text-text-3">
                HRIS Outsource
              </Text>
            </View>

            {/* Form */}
            <View className="gap-3.5">
              <TextField
                label={t('m:login.emailLabel')}
                value={identifier}
                onChangeText={setIdentifier}
                placeholder={t('m:login.emailPlaceholder')}
                autoCapitalize="none"
                autoCorrect={false}
                keyboardType="email-address"
                invalid={showError}
              >
                <Mail size={16} color={color.text3} />
              </TextField>

              <TextField
                label={t('m:login.passwordLabel')}
                value={password}
                onChangeText={setPassword}
                placeholder={'••••••••'}
                secureTextEntry={!showPassword}
                invalid={showError}
              >
                <Pressable
                  onPress={() => setShowPassword(!showPassword)}
                  hitSlop={8}
                >
                  {showPassword ? (
                    <EyeOff size={16} color={color.text3} />
                  ) : (
                    <Eye size={16} color={color.text3} />
                  )}
                </Pressable>
              </TextField>

              {showError ? (
                <ErrorBox
                  title={error}
                  subtitle={
                    remainingAttempts != null
                      ? `Tersisa ${remainingAttempts} dari 5 percobaan.`
                      : undefined
                  }
                />
              ) : null}

              <Pressable onPress={() => router.push('/forgot-password')}>
                <Text
                  className="text-[13px] font-semibold"
                  style={{ color: color.primary }}
                >
                  {t('m:login.forgotPassword')}
                </Text>
              </Pressable>

              <Button
                label={t('m:login.submit')}
                onPress={() => void onSubmit()}
                loading={login.isPending}
                // Button always at full primary opacity (matches comp/BtnPrimary design)
              />
            </View>
          </View>
        </View>

        {/* Footer */}
        <Text className="text-center text-[12px] text-text-3">
          {t('m:login.footer')}
        </Text>
      </View>
    </ScrollView>
  );
}
