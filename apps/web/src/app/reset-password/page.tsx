import { ResetPasswordView } from '@/views/reset-password';
import type { Metadata } from 'next';
import { Suspense } from 'react';

// The token travels in the URL: keep it out of Referer headers and search
// indexes.
export const metadata: Metadata = {
  title: 'Новый пароль — BookHaven',
  robots: { index: false, follow: false },
  referrer: 'no-referrer',
};

export default function Page() {
  return (
    <Suspense>
      <ResetPasswordView />
    </Suspense>
  );
}
