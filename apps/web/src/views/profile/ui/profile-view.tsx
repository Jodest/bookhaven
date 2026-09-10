'use client';

import { useAuthStore } from '@/entities/session';
import { UpdateProfileForm } from '@/features/update-profile';

export function ProfileView() {
  const { isLoading, isAuthenticated, user } = useAuthStore();

  if (isLoading) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка профиля...</p>
      </main>
    );
  }

  if (!isAuthenticated || !user) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Необходимо войти в аккаунт.</p>
      </main>
    );
  }

  return (
    <main className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
      <div className="mx-auto max-w-2xl rounded-xl border bg-card p-8 text-card-foreground shadow-sm">
        <h1 className="font-display text-2xl font-semibold tracking-tight md:text-3xl">
          Профиль
        </h1>
        <UpdateProfileForm />
      </div>
    </main>
  );
}
