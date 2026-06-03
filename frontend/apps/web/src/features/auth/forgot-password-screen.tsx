import { AuthLayout } from '@/features/auth/auth-layout.tsx';
import { Button, FormField, Input } from '@swp/ui';
import { Link, useNavigate } from '@tanstack/react-router';
import { ArrowLeft, Mail, MailCheck } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

/**
 * Forgot-password flow (E1 / Lupa Kata Sandi). Built from .pen frames `etsMo` (form)
 * and `vz7oI` (link sent) per G0. Toggled by local `stage` state; no real API yet —
 * TODO(E1): replace onSubmit stub with the generated useForgotPassword() mutation
 * once the Go /auth/forgot-password endpoint is available (WEB-STACK §6).
 */
const forgotSchema = z.object({
  email: z.string().email(),
});
type ForgotValues = z.infer<typeof forgotSchema>;

type Stage = 'form' | 'sent';

export function ForgotPasswordScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [stage, setStage] = useState<Stage>('form');
  const [submittedEmail, setSubmittedEmail] = useState('');

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ForgotValues>();

  const onSubmit = handleSubmit(async (values) => {
    const parsed = forgotSchema.safeParse(values);
    if (!parsed.success) return;
    // TODO(E1): replace with the generated useForgotPassword() mutation once the
    // Go /auth/forgot-password endpoint is designed. On success → setStage('sent').
    setSubmittedEmail(parsed.data.email);
    setStage('sent');
  });

  if (stage === 'sent') {
    return (
      <AuthLayout>
        {/* Row 1 — back link */}
        <Link to="/login" className="flex items-center gap-1.5 font-medium text-[13px] text-text-2">
          <ArrowLeft className="size-3.5 text-text-2" />
          {t('auth.backToLogin')}
        </Link>

        {/* Row 2 — icon circle */}
        <div className="flex size-[60px] items-center justify-center rounded-full bg-ok-bg">
          <MailCheck className="size-7 text-ok-tx" />
        </div>

        {/* Row 3 — title */}
        <h2 className="font-display font-bold text-2xl text-text">{t('forgot.sentTitle')}</h2>

        {/* Row 4 — body */}
        <p className="text-sm text-text-3 leading-relaxed">
          {t('forgot.sentBody', { email: submittedEmail })}
        </p>

        {/* Row 5 — back to login button */}
        <Button className="w-full" onClick={() => navigate({ to: '/login' })}>
          {t('forgot.backToLogin')}
        </Button>

        {/* Row 6 — resend row */}
        <div className="flex items-center justify-center gap-1.5">
          <span className="text-xs text-text-3">{t('forgot.resendQuestion')}</span>
          <button
            type="button"
            className="font-semibold text-xs text-primary"
            onClick={() => setStage('form')}
          >
            {t('forgot.resend')}
          </button>
        </div>
      </AuthLayout>
    );
  }

  // stage === 'form'
  return (
    <AuthLayout>
      {/* Row 1 — back link */}
      <Link to="/login" className="flex items-center gap-1.5 font-medium text-[13px] text-text-2">
        <ArrowLeft className="size-3.5 text-text-2" />
        {t('auth.backToLogin')}
      </Link>

      {/* Row 2 — title */}
      <h2 className="font-display font-bold text-[28px] text-text">{t('forgot.title')}</h2>

      {/* Row 3 — subtitle */}
      <p className="text-sm text-text-3 leading-relaxed">{t('forgot.subtitle')}</p>

      {/* Row 4–6 — form */}
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

        <Button type="submit" className="w-full" disabled={isSubmitting}>
          {t('forgot.submit')}
        </Button>

        <p className="text-xs text-text-3 leading-snug">{t('forgot.note')}</p>
      </form>
    </AuthLayout>
  );
}
