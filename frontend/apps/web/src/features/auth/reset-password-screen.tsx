import { AuthLayout } from '@/features/auth/auth-layout.tsx';
import { zodResolver } from '@hookform/resolvers/zod';
import { ApiError } from '@swp/api-client';
import { useAuthResetPassword } from '@swp/api-client/e1';
import { Banner, Button, FormField, Input } from '@swp/ui';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { Check, Circle, Eye, EyeOff, ShieldCheck } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

/**
 * Reset Password screen (F1.x / E1 · Reset Kata Sandi). Built from .pen frames:
 *   - `N1c1X` — password-entry form with live requirements checklist
 *   - `b8BGef` — success confirmation with navigate-to-login CTA
 *
 * Two local states: 'form' (default) → 'success' after submit.
 *
 * Calls useAuthResetPassword() with the token from `?token=…` (typed via router.tsx
 * validateSearch on /reset-password). Error codes:
 *   - RESET_TOKEN_EXPIRED (401) → banner on the form
 *   - WEAK_PASSWORD (422) → field error on newPassword
 * Password minLength = 10 per BE policy (AU-4 / platform password policy).
 */

type ScreenState = 'form' | 'success';

export function ResetPasswordScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [screen, setScreen] = useState<ScreenState>('form');
  const [tokenExpired, setTokenExpired] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const resetMut = useAuthResetPassword();

  // Read the typed token from the URL search params (validated in router.tsx).
  const { token } = useSearch({ from: '/reset-password' });

  // Build schema inside component so t() is in scope for the cross-field message.
  // minLength = 10 to match the BE platform password policy (AU-4).
  const resetSchema = z
    .object({
      newPassword: z
        .string()
        .min(10)
        .regex(/(?=.*[a-z])(?=.*[A-Z])/, 'Must contain upper and lower case')
        .regex(/[^a-zA-Z]/, 'Must contain a number or symbol'),
      confirmPassword: z.string(),
    })
    .refine((d) => d.newPassword === d.confirmPassword, {
      message: t('reset.mismatch'),
      path: ['confirmPassword'],
    });

  type ResetValues = z.infer<typeof resetSchema>;

  const {
    register,
    handleSubmit,
    setError,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<ResetValues>({
    resolver: zodResolver(resetSchema),
    mode: 'onChange',
  });

  const newPasswordValue = watch('newPassword') ?? '';

  // Live requirement flags (minLength now 10 to match BE policy)
  const reqMin = newPasswordValue.length >= 10;
  const reqCase = /[a-z]/.test(newPasswordValue) && /[A-Z]/.test(newPasswordValue);
  const reqNum = /[^a-zA-Z]/.test(newPasswordValue);
  const allRulesMet = reqMin && reqCase && reqNum;

  const onSubmit = handleSubmit(async (values) => {
    setTokenExpired(false);
    try {
      await resetMut.mutateAsync({
        data: { reset_token: token ?? '', new_password: values.newPassword },
      });
      setScreen('success');
    } catch (e) {
      if (e instanceof ApiError) {
        if (e.code === 'RESET_TOKEN_EXPIRED') {
          setTokenExpired(true);
        } else if (e.code === 'WEAK_PASSWORD') {
          setError('newPassword', {
            message: t('reset.weakPassword', {
              defaultValue: 'Password does not meet requirements',
            }),
          });
        } else {
          // Generic error — surface as token-expired banner (safe fallback)
          setTokenExpired(true);
        }
      }
    }
  });

  if (screen === 'success') {
    return (
      <AuthLayout>
        {/* b8BGef — success state */}
        <div className="flex flex-col items-center gap-[18px] text-center">
          <div className="flex size-[60px] items-center justify-center rounded-full bg-ok-bg">
            <ShieldCheck className="size-7 text-ok-tx" />
          </div>

          <div className="flex flex-col gap-1">
            <h2 className="font-display font-bold text-2xl text-text">{t('reset.successTitle')}</h2>
            <p className="text-sm text-text-3 leading-relaxed">{t('reset.successBody')}</p>
          </div>

          <Button className="w-full" onClick={() => navigate({ to: '/login' })}>
            {t('reset.goLogin')}
          </Button>
        </div>
      </AuthLayout>
    );
  }

  // N1c1X — form state
  return (
    <AuthLayout>
      <div className="flex flex-col gap-1">
        <h2 className="font-display font-bold text-[28px] text-text">{t('reset.title')}</h2>
        <p className="text-sm text-text-3 leading-relaxed">{t('reset.subtitle')}</p>
      </div>

      {tokenExpired && (
        <Banner
          tone="bad"
          title={t('reset.expiredTitle', { defaultValue: 'Link telah kedaluwarsa' })}
          description={t('reset.expiredBody', {
            defaultValue:
              'Tautan reset kata sandi ini sudah tidak berlaku. Silakan minta tautan baru.',
          })}
        />
      )}

      <form onSubmit={onSubmit} className="flex flex-col gap-[18px]">
        <FormField
          label={t('reset.newPassword')}
          htmlFor="new-password"
          error={errors.newPassword?.message}
          span={2}
        >
          <div className="relative">
            <Input
              id="new-password"
              type={showNew ? 'text' : 'password'}
              autoComplete="new-password"
              aria-invalid={!!errors.newPassword}
              className="pr-10"
              {...register('newPassword')}
            />
            <button
              type="button"
              onClick={() => setShowNew((v) => !v)}
              aria-label={showNew ? t('auth.hidePassword') : t('auth.showPassword')}
              aria-pressed={showNew}
              className="-translate-y-1/2 absolute top-1/2 right-3 text-text-3 hover:text-text"
            >
              {showNew ? (
                <EyeOff aria-hidden className="size-4" />
              ) : (
                <Eye aria-hidden className="size-4" />
              )}
            </button>
          </div>
        </FormField>

        <FormField
          label={t('reset.confirmPassword')}
          htmlFor="confirm-password"
          error={errors.confirmPassword?.message}
          span={2}
        >
          <div className="relative">
            <Input
              id="confirm-password"
              type={showConfirm ? 'text' : 'password'}
              autoComplete="new-password"
              aria-invalid={!!errors.confirmPassword}
              className="pr-10"
              {...register('confirmPassword')}
            />
            <button
              type="button"
              onClick={() => setShowConfirm((v) => !v)}
              aria-label={showConfirm ? t('auth.hidePassword') : t('auth.showPassword')}
              aria-pressed={showConfirm}
              className="-translate-y-1/2 absolute top-1/2 right-3 text-text-3 hover:text-text"
            >
              {showConfirm ? (
                <EyeOff aria-hidden className="size-4" />
              ) : (
                <Eye aria-hidden className="size-4" />
              )}
            </button>
          </div>
        </FormField>

        {/* Live requirements checklist */}
        <div className="flex flex-col gap-1 pt-2">
          <RequirementRow met={reqMin} label={t('reset.reqMin')} />
          <RequirementRow met={reqCase} label={t('reset.reqCase')} />
          <RequirementRow met={reqNum} label={t('reset.reqNum')} />
        </div>

        <Button
          type="submit"
          className="w-full"
          disabled={!allRulesMet || isSubmitting || resetMut.isPending}
        >
          {t('reset.submit')}
        </Button>
      </form>
    </AuthLayout>
  );
}

// ---------------------------------------------------------------------------
// Internal helper — not exported (single use here, no promotion needed yet)
// ---------------------------------------------------------------------------

function RequirementRow({ met, label }: { met: boolean; label: string }) {
  return (
    <div className="flex items-center gap-2">
      {met ? (
        <Check className="size-3.5 text-ok-tx" aria-hidden="true" />
      ) : (
        <Circle className="size-3.5 text-text-3" aria-hidden="true" />
      )}
      <span className={`text-xs ${met ? 'text-ok-tx' : 'text-text-3'}`}>{label}</span>
    </div>
  );
}
