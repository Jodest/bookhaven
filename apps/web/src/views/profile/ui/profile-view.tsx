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
    <main className="container mx-auto px-4 py-8">
      <div className="mx-auto max-w-2xl rounded-lg border bg-card p-8 text-card-foreground shadow-sm">
        <h1 className="text-3xl font-bold tracking-tight">Профиль</h1>
        <UpdateProfileForm />
      </div>
    </main>
  );
}
