'use client';

import { isPasswordWithinByteLimit, PASSWORD_MAX_MESSAGE } from '@/shared/lib/password-rules';
import { Button } from '@bookhaven/ui';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { PASSWORD_MIN_MESSAGE, type ResetState } from '../model/reset-state';

// Same password rule as registration.
const resetSchema = z
  .object({
    password: z
      .string()
      .min(8, PASSWORD_MIN_MESSAGE)
      .refine(isPasswordWithinByteLimit, PASSWORD_MAX_MESSAGE),
    confirm: z.string(),
  })
  .refine((data) => data.password === data.confirm, {
    message: 'Пароли не совпадают',
    path: ['confirm'],
  });

type ResetFormData = z.infer<typeof resetSchema>;

interface ResetPasswordFormProps {
  state: ResetState;
  onSubmit: (password: string) => void | Promise<void>;
}

const inputClassName =
  'mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary';

/**
 * New password + confirmation. Shows the server-side `invalid_password`
 * (normally prevented by the client rule) under the password field and a
 * generic error for 500/network failures; the link is not consumed by
 * either, so the form stays usable.
 */
export function ResetPasswordForm({ state, onSubmit }: ResetPasswordFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ResetFormData>({
    resolver: zodResolver(resetSchema),
  });

  const passwordError =
    errors.password?.message ?? (state === 'invalid_password' ? PASSWORD_MIN_MESSAGE : null);
  const submitting = state === 'submitting';

  return (
    <form
      onSubmit={handleSubmit((data) => onSubmit(data.password))}
      noValidate
      className="space-y-4"
    >
      <div>
        <label htmlFor="reset-password" className="block text-sm font-medium">
          Новый пароль
        </label>
        <input
          id="reset-password"
          type="password"
          autoComplete="new-password"
          className={inputClassName}
          {...register('password')}
        />
        {passwordError && <p className="mt-1 text-sm text-destructive">{passwordError}</p>}
      </div>

      <div>
        <label htmlFor="reset-password-confirm" className="block text-sm font-medium">
          Повторите пароль
        </label>
        <input
          id="reset-password-confirm"
          type="password"
          autoComplete="new-password"
          className={inputClassName}
          {...register('confirm')}
        />
        {errors.confirm && (
          <p className="mt-1 text-sm text-destructive">{errors.confirm.message}</p>
        )}
      </div>

      {state === 'error' && (
        <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          Не удалось сменить пароль. Попробуйте снова.
        </div>
      )}

      <Button type="submit" className="w-full" disabled={submitting}>
        {submitting ? 'Сохраняем…' : 'Сохранить пароль'}
      </Button>
    </form>
  );
}
