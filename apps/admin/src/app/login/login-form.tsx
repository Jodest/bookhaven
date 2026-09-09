'use client';

import { useAuthStore } from '@/lib/auth-store';
import { isSafeInternalRedirect } from '@/lib/redirect';
import { Button, Input, Label } from '@bookhaven/ui';
import { useRouter, useSearchParams } from 'next/navigation';
import { useForm } from 'react-hook-form';

interface LoginFormData {
  email: string;
  password: string;
}

export function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { login, error, clearError, isAuthenticated, user } = useAuthStore();

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormData>();

  const origin = typeof window !== 'undefined' ? window.location.origin : '';
  const rawRedirect = searchParams.get('redirect') || '/';
  const redirect = isSafeInternalRedirect(rawRedirect, origin) ? rawRedirect : '/';

  const onSubmit = async (data: LoginFormData) => {
    clearError();
    try {
      await login(data.email, data.password);
      router.push(redirect as any);
    } catch {
      // ошибка уже в store
    }
  };

  if (isAuthenticated && (user?.role === 'admin' || user?.role === 'moderator')) {
    router.push(redirect as any);
    return null;
  }

  return (
    <div className="w-full max-w-md space-y-6 rounded-lg border bg-card p-8 shadow-sm">
      <div className="text-center">
        <h1 className="text-2xl font-bold">BookHaven Admin</h1>
        <p className="text-sm text-muted-foreground">Вход в панель управления</p>
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            {...register('email', { required: 'Введите email' })}
          />
          {errors.email && (
            <p className="text-sm text-destructive">{errors.email.message}</p>
          )}
        </div>

        <div className="space-y-2">
          <Label htmlFor="password">Пароль</Label>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            {...register('password', { required: 'Введите пароль' })}
          />
          {errors.password && (
            <p className="text-sm text-destructive">{errors.password.message}</p>
          )}
        </div>

        {error && (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {error}
          </div>
        )}

        <Button type="submit" className="w-full" disabled={isSubmitting}>
          {isSubmitting ? 'Вход...' : 'Войти'}
        </Button>
      </form>
    </div>
  );
}
