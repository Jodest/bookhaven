'use client';

import { LoginForm } from '@/features/auth';
import { useState } from 'react';
import { EmailNotVerifiedPanel } from './email-not-verified-panel';

export function LoginView() {
  // Set when the password was correct but the email is not verified (403
  // email_not_verified). The form stays usable so the user can log in again
  // after following the link; a later successful login navigates away.
  const [unverifiedEmail, setUnverifiedEmail] = useState<string | null>(null);

  return (
    <main className="container mx-auto flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-md space-y-6 rounded-xl border bg-card p-8 text-card-foreground shadow-sm">
        <div className="text-center">
          <h1 className="font-display text-2xl font-semibold tracking-tight">
            Вход в BookHaven
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Введите email и пароль, чтобы продолжить
          </p>
        </div>
        {unverifiedEmail && (
          // Keyed by email so the resend state/cool-down resets per address.
          <EmailNotVerifiedPanel key={unverifiedEmail} email={unverifiedEmail} />
        )}
        <LoginForm
          onEmailNotVerified={setUnverifiedEmail}
          onSubmitStart={() => setUnverifiedEmail(null)}
        />
      </div>
    </main>
  );
}
