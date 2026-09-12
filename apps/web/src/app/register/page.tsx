import { RegisterView } from '@/views/register';
import { Suspense } from 'react';

export const metadata = {
  title: 'Регистрация — BookHaven',
};

export default function Page() {
  return (
    <Suspense>
      <RegisterView />
    </Suspense>
  );
}
