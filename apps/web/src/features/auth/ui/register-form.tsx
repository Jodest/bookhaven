'use client';

import { useAuthStore } from '@/entities/session';
import { isPasswordWithinByteLimit, PASSWORD_MAX_MESSAGE } from '@/shared/lib/password-rules';
import { loginHrefWithRedirect } from '@/shared/lib/safe-redirect';
import { Button } from '@bookhaven/ui';
import { zodResolver } from '@hookform/resolvers/zod';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

const registerSchema = z.object({
  name: z.string().min(2, 'Имя должно быть не короче 2 символов'),
  email: z.string().email('Введите корректный email'),
  password: z
    .string()
    .min(8, 'Пароль должен быть не короче 8 символов')
    .refine(isPasswordWithinByteLimit, PASSWORD_MAX_MESSAGE),
});

type RegisterFormData = z.infer<typeof registerSchema>;

interface RegisterFormProps {
  // When provided, called with the registered email instead of navigating
  // away (the register view shows a "check your email" panel in place).
  // Without it the form navigates to /login: registration issues no session.
  onRegistered?: (email: string) => void;
}

export function RegisterForm({ onRegistered }: RegisterFormProps = {}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { register: registerUser, error, clearError } = useAuthStore();

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<RegisterFormData>({
    resolver: zodResolver(registerSchema),
  });

  const onSubmit = async (data: RegisterFormData) => {
    clearError();
    try {
      await registerUser(data.email, data.password, data.name);
      if (onRegistered) {
        onRegistered(data.email);
        return;
      }
      router.push(loginHrefWithRedirect(searchParams.get('redirect')) as any);
    } catch {
      // ошибка уже в store
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <div>
        <label htmlFor="name" className="block text-sm font-medium">
          Имя
        </label>
        <input
          id="name"
          type="text"
          autoComplete="name"
          className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          {...register('name')}
        />
        {errors.name && (
          <p className="mt-1 text-sm text-destructive">{errors.name.message}</p>
        )}
      </div>

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
          autoComplete="new-password"
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

      <Button type="submit" className="w-full" disabled={isSubmitting}>
        {isSubmitting ? 'Регистрация...' : 'Зарегистрироваться'}
      </Button>

      <p className="text-center text-sm text-muted-foreground">
        Уже есть аккаунт?{' '}
        <Link href="/login" className="text-primary hover:underline">
          Войти
        </Link>
      </p>
    </form>
  );
}
