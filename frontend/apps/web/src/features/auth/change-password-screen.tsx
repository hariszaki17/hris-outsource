import { AuthLayout } from '@/features/auth/auth-layout.tsx';
import { auth } from '@/lib/auth.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import { ApiError } from '@swp/api-client';
import { useAuthChangePassword } from '@swp/api-client/e1';
import { Banner, Button, FormField, Input } from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { Check, Circle, Eye, EyeOff, ShieldCheck } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

/**
 * Change Password screen (E1 / EP-3 forced temp-password rotation + voluntary change).
 * Authenticated: the access token is already set, so the mutation is authorized. The
 * authed router guard funnels temp-password users here until the flag clears.
 *
 * Calls useAuthChangePassword(). On success the BE invalidates ALL of the user's
 * sessions (AU-6), so we clear the local session and send the user back to /login to
 * sign in fresh with the new password. Error code:
 *   - WEAK_PASSWORD (422) → field error on newPassword
 * Password policy mirrors the BE (AU-4): min 10 chars + upper + lower + digit + symbol.
 */

type ScreenState = 'form' | 'success';

export function ChangePasswordScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [screen, setScreen] = useState<ScreenState>('form');
  const [showNew, setShowNew] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const changeMut = useAuthChangePassword();

  // Build schema inside the component so t() is in scope for the cross-field message.
  // minLength = 10 + char-class rules to match the BE platform password policy (AU-4).
  const changeSchema = z
    .object({
      newPassword: z
        .string()
        .min(10)
        .regex(/(?=.*[a-z])(?=.*[A-Z])/, 'Must contain upper and lower case')
        .regex(/[0-9]/, 'Must contain a digit')
        .regex(/[^a-zA-Z0-9]/, 'Must contain a symbol'),
      confirmPassword: z.string(),
    })
    .refine((d) => d.newPassword === d.confirmPassword, {
      message: t('changePassword.mismatch'),
      path: ['confirmPassword'],
    });

  type ChangeValues = z.infer<typeof changeSchema>;

  const {
    register,
    handleSubmit,
    setError,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<ChangeValues>({
    resolver: zodResolver(changeSchema),
    mode: 'onChange',
  });

  const newPasswordValue = watch('newPassword') ?? '';

  // Live requirement flags — mirror the BE policy exactly.
  const reqMin = newPasswordValue.length >= 10;
  const reqCase = /[a-z]/.test(newPasswordValue) && /[A-Z]/.test(newPasswordValue);
  const reqNum = /[0-9]/.test(newPasswordValue);
  const reqSym = /[^a-zA-Z0-9]/.test(newPasswordValue);
  const allRulesMet = reqMin && reqCase && reqNum && reqSym;

  const onSubmit = handleSubmit(async (values) => {
    try {
      await changeMut.mutateAsync({ data: { new_password: values.newPassword } });
      setScreen('success');
    } catch (e) {
      if (e instanceof ApiError && e.code === 'WEAK_PASSWORD') {
        setError('newPassword', {
          message: t('changePassword.weakPassword', {
            defaultValue: 'Password does not meet requirements',
          }),
        });
      } else {
        setError('newPassword', {
          message: t('changePassword.genericError', {
            defaultValue: 'Gagal mengubah kata sandi. Silakan coba lagi.',
          }),
        });
      }
    }
  });

  if (screen === 'success') {
    return (
      <AuthLayout>
        <div className="flex flex-col items-center gap-[18px] text-center">
          <div className="flex size-[60px] items-center justify-center rounded-full bg-ok-bg">
            <ShieldCheck className="size-7 text-ok-tx" />
          </div>

          <div className="flex flex-col gap-1">
            <h2 className="font-display font-bold text-2xl text-text">
              {t('changePassword.successTitle')}
            </h2>
            <p className="text-sm text-text-3 leading-relaxed">{t('changePassword.successBody')}</p>
          </div>

          <Button
            className="w-full"
            onClick={() => {
              // BE revoked all sessions (AU-6); drop the local session and re-login.
              auth.clear();
              void navigate({ to: '/login' });
            }}
          >
            {t('changePassword.goLogin')}
          </Button>
        </div>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout>
      <div className="flex flex-col gap-1">
        <h2 className="font-display font-bold text-[28px] text-text">
          {t('changePassword.title')}
        </h2>
        <p className="text-sm text-text-3 leading-relaxed">{t('changePassword.subtitle')}</p>
      </div>

      <Banner
        tone="info"
        title={t('changePassword.noticeTitle')}
        description={t('changePassword.noticeBody')}
      />

      <form onSubmit={onSubmit} className="flex flex-col gap-[18px]">
        <FormField
          label={t('changePassword.newPassword')}
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
          label={t('changePassword.confirmPassword')}
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

        {/* Live requirements checklist — mirrors BE policy (AU-4). */}
        <div className="flex flex-col gap-1 pt-2">
          <RequirementRow met={reqMin} label={t('changePassword.reqMin')} />
          <RequirementRow met={reqCase} label={t('changePassword.reqCase')} />
          <RequirementRow met={reqNum} label={t('changePassword.reqNum')} />
          <RequirementRow met={reqSym} label={t('changePassword.reqSym')} />
        </div>

        <Button
          type="submit"
          className="w-full"
          disabled={!allRulesMet || isSubmitting || changeMut.isPending}
        >
          {t('changePassword.submit')}
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
