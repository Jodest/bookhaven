'use client';

import { useAuthStore } from '@/entities/session';
import { ResendVerificationButton } from '@/features/resend-verification';
import { useVerifyEmail, type VerifyState } from '@/features/verify-email';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';

const COPY: Record<VerifyState, { title: string; body?: string }> = {
  loading: { title: 'Подтверждаем email…' },
  verified: {
    title: 'Email подтверждён',
    body: 'Спасибо! Адрес электронной почты подтверждён.',
  },
  already_verified: {
    title: 'Email уже подтверждён',
    body: 'Этот адрес уже был подтверждён ранее.',
  },
  token_expired: {
    title: 'Срок действия ссылки истёк',
    body: 'Ссылка действительна 24 часа. Запросите новое письмо.',
  },
  invalid_token: {
    title: 'Ссылка недействительна',
    body: 'Ссылка повреждена или устарела. Если вы запрашивали письмо повторно, используйте ссылку из последнего письма.',
  },
  token_used: {
    title: 'Ссылка уже использована',
    body: 'Эта ссылка уже была использована.',
  },
  missing_token: {
    title: 'Ссылка неполная',
    body: 'В ссылке нет кода подтверждения. Откройте ссылку из письма целиком.',
  },
  error: {
    title: 'Не удалось подтвердить email',
    body: 'Произошла ошибка. Проверьте подключение и попробуйте снова.',
  },
};

function SuccessActions() {
  const { isAuthenticated, isLoading } = useAuthStore();
  if (isLoading) return null;
  if (isAuthenticated) {
    return (
      <div className="flex flex-col gap-2 sm:flex-row sm:justify-center">
        <Button asChild>
          <Link href="/profile">Перейти в профиль</Link>
        </Button>
        <Button asChild variant="outline">
          <Link href="/books">К каталогу</Link>
        </Button>
      </div>
    );
  }
  return (
    <div className="flex justify-center">
      <Button asChild>
        <Link href="/login">Войти</Link>
      </Button>
    </div>
  );
}

function ResendBlock() {
  const { user, isAuthenticated, isLoading } = useAuthStore();
  if (isLoading) return null;

  if (isAuthenticated && user) {
    if (!user.emailVerified) {
      return (
        <div className="flex justify-center">
          <ResendVerificationButton className="justify-center" />
        </div>
      );
    }
    return (
      <div className="space-y-3 text-center">
        <p className="text-sm text-muted-foreground">Ваш email уже подтверждён.</p>
        <Button asChild>
          <Link href="/profile">Перейти в профиль</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-3 text-center">
      <p className="text-sm text-muted-foreground">Войдите, чтобы получить новое письмо.</p>
      <Button asChild>
        <Link href="/login">Войти</Link>
      </Button>
    </div>
  );
}

export function VerifyEmailView() {
  const searchParams = useSearchParams();
  const token = searchParams.get('token')?.trim() || null;
  const { state, retry } = useVerifyEmail(token);
  const copy = COPY[state];

  return (
    <main className="container mx-auto flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-md space-y-6 rounded-xl border bg-card p-8 text-card-foreground shadow-sm">
        <div className="text-center" aria-live="polite">
          <h1 className="font-display text-2xl font-semibold tracking-tight">{copy.title}</h1>
          {copy.body && <p className="mt-2 text-sm text-muted-foreground">{copy.body}</p>}
        </div>

        {(state === 'verified' || state === 'already_verified') && <SuccessActions />}

        {(state === 'token_expired' ||
          state === 'invalid_token' ||
          state === 'token_used' ||
          state === 'missing_token') && <ResendBlock />}

        {state === 'error' && (
          <div className="flex justify-center">
            <Button type="button" onClick={retry}>
              Попробовать снова
            </Button>
          </div>
        )}
      </div>
    </main>
  );
}
