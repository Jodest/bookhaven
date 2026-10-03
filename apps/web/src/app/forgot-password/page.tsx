import { ForgotPasswordView } from '@/views/forgot-password';
import type { Metadata } from 'next';
import { Suspense } from 'react';

export const metadata: Metadata = {
  title: 'Восстановление пароля — BookHaven',
  robots: { index: false, follow: false },
  referrer: 'no-referrer',
};

export default function Page() {
  return (
    <Suspense>
      <ForgotPasswordView />
    </Suspense>
  );
}
