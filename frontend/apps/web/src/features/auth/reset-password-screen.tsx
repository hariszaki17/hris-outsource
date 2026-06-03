import { AuthLayout } from '@/features/auth/auth-layout.tsx';
import { zodResolver } from '@hookform/resolvers/zod';
import { Button, FormField, Input } from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { Check, Circle, Eye, ShieldCheck } from 'lucide-react';
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
 * TODO(E1): replace onSubmit stub with the generated `useAuthResetPassword()` mutation.
 *   The reset token must be read from the URL search params (e.g. `?token=…`) using
 *   TanStack Router's `useSearch({ from: '/reset-password' })` once the route is typed.
 *   On success → switch to 'success'. On 400/expired-token → surface a Banner (tone="bad").
 */

type ScreenState = 'form' | 'success';

export function ResetPasswordScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [screen, setScreen] = useState<ScreenState>('form');

  // Build schema inside component so t() is in scope for the cross-field message.
  const resetSchema = z
    .object({
      newPassword: z
        .string()
        .min(8)
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
    watch,
    formState: { errors, isSubmitting },
  } = useForm<ResetValues>({
    resolver: zodResolver(resetSchema),
    mode: 'onChange',
  });

  const newPasswordValue = watch('newPassword') ?? '';

  // Live requirement flags
  const reqMin = newPasswordValue.length >= 8;
  const reqCase = /[a-z]/.test(newPasswordValue) && /[A-Z]/.test(newPasswordValue);
  const reqNum = /[^a-zA-Z]/.test(newPasswordValue);
  const allRulesMet = reqMin && reqCase && reqNum;

  const onSubmit = handleSubmit(async (_values) => {
    // TODO(E1): call useAuthResetPassword() mutation with token from URL + _values.newPassword.
    setScreen('success');
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
              type="password"
              autoComplete="new-password"
              aria-invalid={!!errors.newPassword}
              className="pr-10"
              {...register('newPassword')}
            />
            <Eye className="-translate-y-1/2 pointer-events-none absolute top-1/2 right-3 size-4 text-text-3" />
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
              type="password"
              autoComplete="new-password"
              aria-invalid={!!errors.confirmPassword}
              className="pr-10"
              {...register('confirmPassword')}
            />
            <Eye className="-translate-y-1/2 pointer-events-none absolute top-1/2 right-3 size-4 text-text-3" />
          </div>
        </FormField>

        {/* Live requirements checklist */}
        <div className="flex flex-col gap-1 pt-2">
          <RequirementRow met={reqMin} label={t('reset.reqMin')} />
          <RequirementRow met={reqCase} label={t('reset.reqCase')} />
          <RequirementRow met={reqNum} label={t('reset.reqNum')} />
        </div>

        <Button type="submit" className="w-full" disabled={!allRulesMet || isSubmitting}>
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
