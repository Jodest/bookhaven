'use client';

import {
  ResetPasswordForm,
  isResetLinkFailure,
  useResetPassword,
  type ResetLinkFailure,
} from '@/features/reset-password';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import type { ReactNode } from 'react';

const LINK_FAILURE_COPY: Record<ResetLinkFailure | 'missing_token', { title: string; body: string }> =
  {
    missing_token: {
      title: 'Ссылка неполная',
      body: 'В ссылке нет кода сброса. Откройте ссылку из письма целиком или запросите новую.',
    },
    token_expired: {
      title: 'Срок действия ссылки истёк',
      body: 'Ссылка действительна 1 час. Запросите новую.',
    },
    invalid_token: {
      title: 'Ссылка недействительна',
      body: 'Ссылка повреждена или устарела. Если вы запрашивали сброс повторно, используйте ссылку из последнего письма.',
    },
    token_used: {
      title: 'Ссылка уже использована',
      body: 'По этой ссылке пароль уже был изменён. Если нужно, запросите новую.',
    },
  };

function Card({ title, body, children }: { title: string; body?: string; children: ReactNode }) {
  return (
    <main className="container mx-auto flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-md space-y-6 rounded-xl border bg-card p-8 text-card-foreground shadow-sm">
        <div className="text-center" aria-live="polite">
          <h1 className="font-display text-2xl font-semibold tracking-tight">{title}</h1>
          {body && <p className="mt-2 text-sm text-muted-foreground">{body}</p>}
        </div>
        {children}
      </div>
    </main>
  );
}

function LinkFailure({ kind }: { kind: ResetLinkFailure | 'missing_token' }) {
  const copy = LINK_FAILURE_COPY[kind];
  return (
    <Card title={copy.title} body={copy.body}>
      <div className="flex flex-col gap-2 sm:flex-row sm:justify-center">
        <Button asChild>
          <Link href="/forgot-password">Запросить новую ссылку</Link>
        </Button>
        {kind === 'token_used' && (
          <Button asChild variant="outline">
            <Link href="/login">Войти</Link>
          </Button>
        )}
      </div>
    </Card>
  );
}

function ResetPasswordContent({ token }: { token: string }) {
  const { state, submit } = useResetPassword(token);

  if (state === 'success') {
    return (
      <Card
        title="Пароль изменён"
        body="Теперь войдите с новым паролем. Все прежние сеансы завершены."
      >
        <div className="flex justify-center">
          <Button asChild>
            <Link href="/login">Войти</Link>
          </Button>
        </div>
      </Card>
    );
  }

  if (isResetLinkFailure(state)) {
    return <LinkFailure kind={state} />;
  }

  return (
    <Card title="Новый пароль">
      <ResetPasswordForm state={state} onSubmit={submit} />
    </Card>
  );
}

export function ResetPasswordView() {
  const searchParams = useSearchParams();
  const token = searchParams.get('token')?.trim() || null;

  if (!token) {
    return <LinkFailure kind="missing_token" />;
  }
  // Keyed by token so a different link starts from a fresh form.
  return <ResetPasswordContent key={token} token={token} />;
}
