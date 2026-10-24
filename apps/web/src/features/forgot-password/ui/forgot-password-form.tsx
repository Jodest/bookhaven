'use client';

import { Button } from '@bookhaven/ui';
import { zodResolver } from '@hookform/resolvers/zod';
import Link from 'next/link';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import {
  FORGOT_SENT_MESSAGE,
  INVALID_EMAIL_MESSAGE,
  forgotMessage,
} from '../model/forgot-state';
import { useForgotPassword } from '../model/use-forgot-password';

const forgotSchema = z.object({
  email: z.email(INVALID_EMAIL_MESSAGE),
});

type ForgotFormData = z.infer<typeof forgotSchema>;

const backToLoginLink = (
  <p className="text-center text-sm text-muted-foreground">
    <Link href="/login" className="text-primary hover:underline">
      Вернуться ко входу
    </Link>
  </p>
);

/**
 * Email form of /forgot-password. After a successful request the form is
 * replaced by a neutral panel: the backend answers the same way for unknown
 * addresses, so the copy never claims that an email was sent.
 */
export function ForgotPasswordForm() {
  const { state, submit, backToForm, disabled } = useForgotPassword();

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ForgotFormData>({
    resolver: zodResolver(forgotSchema),
  });

  if (state.kind === 'sent') {
    return (
      <div className="space-y-4">
        <p role="status" className="rounded-md border bg-secondary p-3 text-sm">
          {FORGOT_SENT_MESSAGE}
        </p>
        <Button type="button" variant="outline" className="w-full" disabled={disabled} onClick={backToForm}>
          Отправить ещё раз
        </Button>
        {backToLoginLink}
      </div>
    );
  }

  const onSubmit = (data: ForgotFormData) => submit(data.email.trim());
  const emailError =
    errors.email?.message ?? (state.kind === 'invalid_email' ? INVALID_EMAIL_MESSAGE : null);
  const message = forgotMessage(state);

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-4">
      <p className="text-center text-sm text-muted-foreground">
        Введите email, указанный при регистрации. Мы отправим ссылку для сброса пароля.
      </p>
      <div>
        <label htmlFor="forgot-email" className="block text-sm font-medium">
          Email
        </label>
        <input
          id="forgot-email"
          type="email"
          autoComplete="email"
          className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          {...register('email')}
        />
        {emailError && <p className="mt-1 text-sm text-destructive">{emailError}</p>}
      </div>

      {message && (
        <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {message}
        </div>
      )}

      <Button type="submit" className="w-full" disabled={disabled}>
        {state.kind === 'sending' ? 'Отправляем…' : 'Отправить ссылку'}
      </Button>

      {backToLoginLink}
    </form>
  );
}
