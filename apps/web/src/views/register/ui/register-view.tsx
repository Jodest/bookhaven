'use client';

import { useAuthStore } from '@/entities/session';
import { RegisterForm } from '@/features/auth';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { CheckEmailPanel } from './check-email-panel';

export function RegisterView() {
  const [registeredEmail, setRegisteredEmail] = useState<string | null>(null);
  const { isAuthenticated } = useAuthStore();
  const router = useRouter();

  // Registration is guest-only: a signed-in user has nothing to do here.
  useEffect(() => {
    if (isAuthenticated) router.replace('/profile');
  }, [isAuthenticated, router]);

  if (isAuthenticated) return null;

  return (
    <main className="container mx-auto flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-md space-y-6 rounded-xl border bg-card p-8 text-card-foreground shadow-sm">
        {registeredEmail ? (
          <CheckEmailPanel email={registeredEmail} />
        ) : (
          <>
            <div className="text-center">
              <h1 className="font-display text-2xl font-semibold tracking-tight">Регистрация</h1>
              <p className="mt-2 text-sm text-muted-foreground">
                Создайте аккаунт, чтобы отслеживать книги и оставлять рецензии
              </p>
            </div>
            <RegisterForm onRegistered={setRegisteredEmail} />
          </>
        )}
      </div>
    </main>
  );
}
