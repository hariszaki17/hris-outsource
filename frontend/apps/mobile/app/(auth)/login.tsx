// Matches brainstorm.pen "Agen · Login (Mobile)" (Y09E0) + the 3 error-state variants
// (failed / locked / deactivated). Brand wordmark = font-display (Poppins 22/700).
// Icons from comp/TextField: mail on right side, eye toggles password visibility.
import { type LoginResponse, useAuthLogin } from '@swp/api-client/e1';
import { color } from '@swp/design-tokens';
import { useRouter } from 'expo-router';
import { CircleX, Eye, EyeOff, Lock, Mail, ShieldX } from 'lucide-react-native';
import type { ReactNode } from 'react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Image, Pressable, ScrollView, View } from 'react-native';
import { useSession } from '../../src/providers/session';
import { Button } from '../../src/ui/Button';
import { Text } from '../../src/ui/Text';
import { TextField } from '../../src/ui/TextField';

const MAX_ATTEMPTS = 5;
const DEFAULT_LOCK_MINUTES = 15;

type ErrorKind = 'invalid' | 'locked' | 'disabled' | 'generic';

interface LoginError {
  kind: ErrorKind;
  /** invalid: attempts left before lockout. */
  remainingAttempts?: number;
  /** locked: minutes until retry. */
  lockMinutes?: number;
}

// Error banner — bad tone (DESIGN-SYSTEM §2). Icon varies by kind (CircleX/Lock/ShieldX).
function ErrorBox({
  icon,
  title,
  subtitle,
}: {
  icon: ReactNode;
  title: string;
  subtitle?: string;
}) {
  return (
    <View className="flex-row items-start gap-2 rounded-input border border-bad-border bg-bad-bg px-3 py-2.5">
      {icon}
      <View className="shrink gap-0.5">
        <Text className="text-[13px] font-bold leading-[1.4] text-bad-text">{title}</Text>
        {subtitle ? <Text className="text-xs leading-[1.4] text-bad-text">{subtitle}</Text> : null}
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
  const [error, setError] = useState<LoginError | null>(null);

  async function onSubmit() {
    setError(null);
    try {
      const res = await login.mutateAsync({ data: { identifier, password } });
      await signIn(res.data as LoginResponse);
      router.replace('/');
    } catch (e) {
      const err = e as {
        status?: number;
        body?: { remaining_attempts?: number; retry_after_minutes?: number };
      };
      const status = err.status;
      if (status === 401) {
        setError({ kind: 'invalid', remainingAttempts: err.body?.remaining_attempts });
      } else if (status === 423) {
        setError({
          kind: 'locked',
          lockMinutes: err.body?.retry_after_minutes ?? DEFAULT_LOCK_MINUTES,
        });
      } else if (status === 410) {
        setError({ kind: 'disabled' });
      } else {
        setError({ kind: 'generic' });
      }
    }
  }

  const invalid = error !== null;
  const locked = error?.kind === 'locked';

  // Map error kind → banner content (icon + i18n title/subtitle).
  function renderErrorBox() {
    if (!error) return null;
    if (error.kind === 'locked') {
      return (
        <ErrorBox
          icon={<Lock size={14} color={color.bad.text} />}
          title={t('m:login.errorLockedTitle')}
          subtitle={t('m:login.errorLockedSub', { minutes: error.lockMinutes })}
        />
      );
    }
    if (error.kind === 'disabled') {
      return (
        <ErrorBox
          icon={<ShieldX size={14} color={color.bad.text} />}
          title={t('m:login.errorDisabledTitle')}
          subtitle={t('m:login.errorDisabledSub')}
        />
      );
    }
    if (error.kind === 'invalid') {
      return (
        <ErrorBox
          icon={<CircleX size={14} color={color.bad.text} />}
          title={t('m:login.errorInvalidTitle')}
          subtitle={
            error.remainingAttempts != null
              ? t('m:login.errorInvalidAttempts', {
                  count: error.remainingAttempts,
                  max: MAX_ATTEMPTS,
                })
              : undefined
          }
        />
      );
    }
    return (
      <ErrorBox
        icon={<CircleX size={14} color={color.bad.text} />}
        title={t('m:login.errorGenericTitle')}
        subtitle={t('m:common.errorGeneric')}
      />
    );
  }

  return (
    <ScrollView className="flex-1 bg-surface" contentContainerStyle={{ flexGrow: 1 }}>
      <View className="flex-1 pb-7" style={{ paddingTop: 60 }}>
        {/* Vertically-center the form block */}
        <View className="flex-1 justify-center px-7">
          <View className="gap-[22px]">
            {/* Brand: chip 64×64 r16 + wordmark (Poppins 22/700) + subtitle, 14px gaps */}
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
                invalid={invalid}
              >
                <Mail size={16} color={color.text3} />
              </TextField>

              <TextField
                label={t('m:login.passwordLabel')}
                value={password}
                onChangeText={setPassword}
                placeholder={'••••••••'}
                secureTextEntry={!showPassword}
                invalid={invalid}
              >
                <Pressable onPress={() => setShowPassword(!showPassword)} hitSlop={8}>
                  {showPassword ? (
                    <EyeOff size={16} color={color.text3} />
                  ) : (
                    <Eye size={16} color={color.text3} />
                  )}
                </Pressable>
              </TextField>

              {renderErrorBox()}

              {/* Locked accounts can't retry now → hide the forgot link + show a disabled
                  "Coba lagi nanti" secondary button instead of "Masuk" (matches .pen). */}
              {locked ? (
                <Button
                  variant="secondary"
                  label={t('m:login.errorLockedRetry')}
                  disabled
                  onPress={() => {}}
                />
              ) : (
                <>
                  <Pressable onPress={() => router.push('/forgot-password')}>
                    <Text className="text-[13px] font-semibold text-primary">
                      {t('m:login.forgotPassword')}
                    </Text>
                  </Pressable>

                  <Button
                    label={t('m:login.submit')}
                    onPress={() => void onSubmit()}
                    loading={login.isPending}
                  />
                </>
              )}
            </View>
          </View>
        </View>

        {/* Footer */}
        <Text className="text-center text-[12px] text-text-3">{t('m:login.footer')}</Text>
      </View>
    </ScrollView>
  );
}
