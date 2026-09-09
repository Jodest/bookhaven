'use client';

import { Suspense } from 'react';
import { LoginForm } from './login-form';

export default function LoginPage() {
  return (
    <main className="flex min-h-screen items-center justify-center p-4">
      <Suspense
        fallback={
          <div className="w-full max-w-md rounded-lg border bg-card p-8 shadow-sm">
            Загрузка...
          </div>
        }
      >
        <LoginForm />
      </Suspense>
    </main>
  );
}
