'use client';

import { ForgotPasswordForm } from '@/features/forgot-password';

export function ForgotPasswordView() {
  return (
    <main className="container mx-auto flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-md space-y-6 rounded-xl border bg-card p-8 text-card-foreground shadow-sm">
        <div className="text-center">
          <h1 className="font-display text-2xl font-semibold tracking-tight">
            Восстановление пароля
          </h1>
        </div>
        <ForgotPasswordForm />
      </div>
    </main>
  );
}
