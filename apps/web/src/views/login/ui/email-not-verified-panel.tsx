'use client';

import { ResendVerificationButton } from '@/features/resend-verification';

export function EmailNotVerifiedPanel({ email }: { email: string }) {
  return (
    <div role="status" className="space-y-3 rounded-md border bg-secondary p-3 text-sm">
      <p>
        Email <strong className="font-semibold">{email}</strong> не подтверждён. Перейдите по
        ссылке из письма, которое мы отправили при регистрации, и войдите снова.
      </p>
      <ResendVerificationButton email={email} variant="outline" size="sm" />
    </div>
  );
}
