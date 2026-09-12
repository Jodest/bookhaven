import { LoginView } from '@/views/login';
import { Suspense } from 'react';

export const metadata = {
  title: 'Вход — BookHaven',
};

export default function Page() {
  return (
    <Suspense>
      <LoginView />
    </Suspense>
  );
}
