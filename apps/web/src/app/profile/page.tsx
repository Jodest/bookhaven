'use client';

import { useAuthStore } from '@/lib/auth-store';
import { Button } from '@bookhaven/ui';
import { useRouter } from 'next/navigation';

export default function ProfilePage() {
  const { user, isLoading, isAuthenticated, logout } = useAuthStore();
  const router = useRouter();

  const handleLogout = async () => {
    await logout();
    router.push('/');
  };

  if (isLoading) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка...</p>
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

        <div className="mt-6 space-y-4">
          <div>
            <dt className="text-sm text-muted-foreground">Имя</dt>
            <dd className="text-lg font-medium">{user.name}</dd>
          </div>

          <div>
            <dt className="text-sm text-muted-foreground">Email</dt>
            <dd className="text-lg font-medium">{user.email}</dd>
          </div>

          <div>
            <dt className="text-sm text-muted-foreground">Роль</dt>
            <dd className="text-lg font-medium">{user.role}</dd>
          </div>

          <div>
            <dt className="text-sm text-muted-foreground">Тип профиля</dt>
            <dd className="text-lg font-medium">{user.profileType}</dd>
          </div>
        </div>

        <div className="mt-8">
          <Button variant="outline" onClick={handleLogout}>
            Выйти
          </Button>
        </div>
      </div>
    </main>
  );
}
