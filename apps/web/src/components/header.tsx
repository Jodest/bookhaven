'use client';

import { useAuthStore } from '@/lib/auth-store';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useRouter } from 'next/navigation';

export function Header() {
  const { user, isAuthenticated, isLoading, logout } = useAuthStore();
  const router = useRouter();

  const handleLogout = async () => {
    await logout();
    router.push('/');
  };

  return (
    <header className="border-b bg-background">
      <div className="container mx-auto flex h-16 items-center justify-between px-4">
        <Link href="/" className="text-xl font-bold tracking-tight">
          BookHaven
        </Link>

        <nav className="flex items-center gap-4">
          <Button variant="ghost" asChild>
            <Link href="/books">Каталог</Link>
          </Button>

          {!isLoading && (
            <>
              {isAuthenticated && user ? (
                <>
                  <Button variant="ghost" asChild>
                    <Link href="/shelves">Мои полки</Link>
                  </Button>
                  <Button variant="ghost" asChild>
                    <Link href="/lists">Мои списки</Link>
                  </Button>
                  <Button variant="ghost" asChild>
                    <Link href="/challenge">Читательский вызов</Link>
                  </Button>
                  <Button variant="ghost" asChild>
                    <Link href="/profile">{user.name || user.email}</Link>
                  </Button>
                  <Button variant="outline" onClick={handleLogout}>
                    Выйти
                  </Button>
                </>
              ) : (
                <>
                  <Button variant="ghost" asChild>
                    <Link href="/login">Войти</Link>
                  </Button>
                  <Button asChild>
                    <Link href="/register">Регистрация</Link>
                  </Button>
                </>
              )}
            </>
          )}
        </nav>
      </div>
    </header>
  );
}
