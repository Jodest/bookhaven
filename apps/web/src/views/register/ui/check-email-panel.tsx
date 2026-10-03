'use client';

import { ResendVerificationButton } from '@/features/resend-verification';
import { loginHrefWithRedirect } from '@/shared/lib/safe-redirect';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';

export function CheckEmailPanel({ email }: { email: string }) {
  const searchParams = useSearchParams();
  const loginHref = loginHrefWithRedirect(searchParams.get('redirect'));

  return (
    <div className="space-y-6">
      <div className="text-center">
        <h1 className="font-display text-2xl font-semibold tracking-tight">Проверьте почту</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Мы отправили письмо со ссылкой для подтверждения на{' '}
          <strong className="font-semibold text-foreground">{email}</strong>. Ссылка
          действительна 24 часа. Войти можно после подтверждения.
        </p>
      </div>

      <div className="space-y-3 text-center">
        <p className="text-sm text-muted-foreground">
          Не пришло письмо? Проверьте папку «Спам» или отправьте его ещё раз.
        </p>
        <ResendVerificationButton email={email} variant="outline" className="justify-center" />
      </div>

      <Button asChild className="w-full">
        <Link href={loginHref as any}>Войти</Link>
      </Button>
    </div>
  );
}
