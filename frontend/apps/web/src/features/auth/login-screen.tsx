import { AuthLayout } from '@/features/auth/auth-layout.tsx';
import { auth, buildSessionUser } from '@/lib/auth.ts';
import { ApiError } from '@swp/api-client';
import type { LoginResponse } from '@swp/api-client/e1';
import { useAuthLogin } from '@swp/api-client/e1';
import { Banner, Button, Checkbox, FormField, Input } from '@swp/ui';
import { Link, useNavigate, useSearch } from '@tanstack/react-router';
import { Eye, Lock, Mail, ShieldX } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

/**
 * Login (F1.1 / authentication.md). Built from the .pen frames `E1 · Login (Web)` (lKRjr),
 * `… — Gagal` (JRq3Z), `… — Terkunci sementara` (N2IdlJ), `… — Akun nonaktif` (QVifb) per G0.
 * Split-screen via the shared `AuthLayout`. The error state is taken from the typed `error`
 * search param (set by the login mutation redirect). Calls the real useAuthLogin() hook.
 * Error codes: INVALID_CREDENTIALS→'invalid', ACCOUNT_DISABLED→'disabled',
 * ACCOUNT_LOCKED/429→'locked' (ENGINEERING.md B1 / authentication.md AU-5).
 */
const loginSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});
type LoginValues = z.infer<typeof loginSchema>;

export function LoginScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { error } = useSearch({ from: '/login' });
  const [rememberMe, setRememberMe] = useState(true);
  const loginMut = useAuthLogin();
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginValues>();

  const locked = error === 'locked';
  const disabled = error === 'disabled';

  const onSubmit = handleSubmit(async (values) => {
    const parsed = loginSchema.safeParse(values);
    if (!parsed.success) return;
    try {
      const res = await loginMut.mutateAsync({
        data: { email: parsed.data.email, password: parsed.data.password, stay_signed_in: rememberMe },
      });
      const body = res.data as LoginResponse;
      auth.login(body.access_token, buildSessionUser(body.user));
      await navigate({ to: '/' });
    } catch (e) {
      if (e instanceof ApiError) {
        if (e.code === 'ACCOUNT_DISABLED') {
          void navigate({ to: '/login', search: { error: 'disabled' } });
        } else if (e.code === 'ACCOUNT_LOCKED' || e.status === 429) {
          void navigate({ to: '/login', search: { error: 'locked' } });
        } else {
          // INVALID_CREDENTIALS or any other auth error → generic invalid banner
          void navigate({ to: '/login', search: { error: 'invalid' } });
        }
      } else {
        void navigate({ to: '/login', search: { error: 'invalid' } });
      }
    }
  });

  return (
    <AuthLayout>
      <div className="flex flex-col gap-1">
        <h2 className="font-display font-bold text-[28px] text-text">{t('auth.title')}</h2>
        <p className="text-sm text-text-3">{t('auth.subtitle')}</p>
      </div>

      {error === 'invalid' && (
        <Banner tone="bad" title={t('auth.invalidTitle')} description={t('auth.invalidBody')} />
      )}
      {locked && (
        <Banner
          tone="bad"
          icon={Lock}
          title={t('auth.lockedTitle')}
          description={t('auth.lockedBody')}
        />
      )}
      {disabled && (
        <Banner
          tone="bad"
          icon={ShieldX}
          title={t('auth.disabledTitle')}
          description={t('auth.disabledBody')}
        />
      )}

      <form onSubmit={onSubmit} className="flex flex-col gap-[18px]">
        <FormField label={t('auth.email')} htmlFor="email" error={errors.email?.message} span={2}>
          <div className="relative">
            <Input
              id="email"
              type="email"
              autoComplete="email"
              placeholder={t('auth.emailPlaceholder')}
              aria-invalid={!!errors.email}
              className="pr-10"
              {...register('email', { required: true })}
            />
            <Mail className="-translate-y-1/2 pointer-events-none absolute top-1/2 right-3 size-4 text-text-3" />
          </div>
        </FormField>

        <FormField
          label={t('auth.password')}
          htmlFor="password"
          error={errors.password?.message}
          span={2}
        >
          <div className="relative">
            <Input
              id="password"
              type="password"
              autoComplete="current-password"
              aria-invalid={!!errors.password}
              className="pr-10"
              {...register('password', { required: true })}
            />
            <Eye className="-translate-y-1/2 pointer-events-none absolute top-1/2 right-3 size-4 text-text-3" />
          </div>
        </FormField>

        <div className="flex items-center justify-between">
          <Checkbox
            id="remember"
            label={t('auth.rememberMe')}
            checked={rememberMe}
            onChange={(e) => setRememberMe(e.target.checked)}
          />
          <Link to="/forgot-password" className="font-semibold text-[13px] text-primary">
            {t('auth.forgot')}
          </Link>
        </div>

        {locked ? (
          <Button type="button" variant="secondary" className="w-full" disabled>
            {t('auth.lockedRetry')}
          </Button>
        ) : (
          <Button type="submit" className="w-full" disabled={disabled || isSubmitting || loginMut.isPending}>
            {t('auth.login')}
          </Button>
        )}

        <p className="text-xs text-text-3 leading-snug">{t('auth.note')}</p>
      </form>
    </AuthLayout>
  );
}
