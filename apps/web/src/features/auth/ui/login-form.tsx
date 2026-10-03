'use client';

import { useAuthStore } from '@/entities/session';
import { isSafeRedirectPath } from '@/shared/lib/safe-redirect';
import { isEmailNotVerifiedError } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { zodResolver } from '@hookform/resolvers/zod';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

const loginSchema = z.object({
  email: z.string().email('Введите корректный email'),
  password: z.string().min(1, 'Введите пароль'),
});

type LoginFormData = z.infer<typeof loginSchema>;

interface LoginFormProps {
  // Called with the typed email when the password is correct but the email
  // is not verified (403 email_not_verified); the login view shows its own
  // panel with a resend button. Without it the form shows a short hint.
  onEmailNotVerified?: (email: string) => void;
  // Called at the start of every submit so the view can drop a panel left
  // over from a previous attempt.
  onSubmitStart?: () => void;
}

export function LoginForm({ onEmailNotVerified, onSubmitStart }: LoginFormProps = {}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { login, error, clearError } = useAuthStore();
  const [notVerifiedHint, setNotVerifiedHint] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
  });

  const onSubmit = async (data: LoginFormData) => {
    clearError();
    setNotVerifiedHint(false);
    onSubmitStart?.();
    try {
      await login(data.email, data.password);
      const redirect = searchParams.get('redirect');
      router.push((isSafeRedirectPath(redirect) ? redirect : '/profile') as any);
    } catch (err) {
      if (isEmailNotVerifiedError(err)) {
        if (onEmailNotVerified) {
          onEmailNotVerified(data.email);
        } else {
          setNotVerifiedHint(true);
        }
      }
      // остальные ошибки уже в store
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <div>
        <label htmlFor="email" className="block text-sm font-medium">
          Email
        </label>
        <input
          id="email"
          type="email"
          autoComplete="email"
          className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          {...register('email')}
        />
        {errors.email && (
          <p className="mt-1 text-sm text-destructive">{errors.email.message}</p>
        )}
      </div>

      <div>
        <label htmlFor="password" className="block text-sm font-medium">
          Пароль
        </label>
        <input
          id="password"
          type="password"
          autoComplete="current-password"
          className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          {...register('password')}
        />
        {errors.password && (
          <p className="mt-1 text-sm text-destructive">{errors.password.message}</p>
        )}
      </div>

      {error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {notVerifiedHint && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          Email не подтверждён. Проверьте почту.
        </div>
      )}

      <Button type="submit" className="w-full" disabled={isSubmitting}>
        {isSubmitting ? 'Вход...' : 'Войти'}
      </Button>

      <p className="text-center text-sm text-muted-foreground">
        Нет аккаунта?{' '}
        <Link href="/register" className="text-primary hover:underline">
          Зарегистрироваться
        </Link>
      </p>
    </form>
  );
}
