import { type LoginResponse, useAuthLogin } from '@swp/api-client/e1';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { TextInput, View } from 'react-native';
import { useSession } from '../../src/providers/session';
import { Button } from '../../src/ui/Button';
import { Screen } from '../../src/ui/Screen';
import { Text } from '../../src/ui/Text';

export default function Login() {
  const { t } = useTranslation();
  const { signIn } = useSession();
  const router = useRouter();
  const login = useAuthLogin();

  const [identifier, setIdentifier] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);

  async function onSubmit() {
    setError(null);
    try {
      const res = await login.mutateAsync({ data: { identifier, password } });
      // Non-2xx throws; on success res.data is the LoginResponse body.
      await signIn(res.data as LoginResponse);
      router.replace('/');
    } catch {
      setError(t('m:login.errorInvalid'));
    }
  }

  const inputClass = 'rounded-input border border-border bg-surface px-4 py-3 text-text';

  return (
    <Screen>
      <Text variant="title">{t('m:login.title')}</Text>
      <Text variant="caption" className="mb-8">
        {t('m:login.subtitle')}
      </Text>

      <View className="gap-4">
        <View>
          <Text variant="caption" className="mb-1">
            {t('m:login.identifier')}
          </Text>
          <TextInput
            value={identifier}
            onChangeText={setIdentifier}
            autoCapitalize="none"
            autoCorrect={false}
            className={inputClass}
          />
        </View>
        <View>
          <Text variant="caption" className="mb-1">
            {t('m:login.password')}
          </Text>
          <TextInput
            value={password}
            onChangeText={setPassword}
            secureTextEntry
            className={inputClass}
          />
        </View>

        {error ? <Text className="text-danger">{error}</Text> : null}

        <Button label={t('m:login.submit')} onPress={onSubmit} loading={login.isPending} />
      </View>
    </Screen>
  );
}
