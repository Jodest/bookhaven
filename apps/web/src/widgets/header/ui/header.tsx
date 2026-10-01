'use client';

import { useAuthStore } from '@/entities/session';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Suspense, type ReactNode } from 'react';
import { HeaderLoginLink } from './header-login-link';
import { HeaderSearch } from './header-search';

interface HeaderProps {
  // The notification bell lives in its own `widgets/notifications` slice -
  // FSD's layering rules (see apps/web/.eslintrc.json boundaries/element-types)
  // disallow one widget importing another, so the app-root composition
  // (app/layout.tsx) injects it here rather than Header importing it
  // directly. Still renders exactly where the feature contract calls for:
  // next to the profile link, only for authenticated users.
  notificationSlot?: ReactNode;
}

export function Header({ notificationSlot }: HeaderProps) {
  const { user, isAuthenticated, isLoading, logout } = useAuthStore();
  const router = useRouter();

  const handleLogout = async () => {
    await logout();
    router.push('/');
  };

  return (
    <header className="bg-foreground text-background">
      <div className="container mx-auto flex h-16 items-center gap-4 px-4 sm:px-6 lg:px-8">
        <Link href="/" className="font-display text-xl font-semibold tracking-tight shrink-0">
          BookHaven
        </Link>

        <nav className="flex items-center gap-1 sm:gap-2">
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
                </>
              ) : null}
            </>
          )}
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <HeaderSearch />

          {!isLoading && (
            <>
              {isAuthenticated && user ? (
                <>
                  {notificationSlot}
                  <Button variant="ghost" asChild>
                    <Link href="/profile">{user.name || user.email}</Link>
                  </Button>
                  <Button variant="secondary" onClick={handleLogout}>
                    Выйти
                  </Button>
                </>
              ) : (
                <>
                  <Suspense
                    fallback={
                      <Button variant="ghost" asChild>
                        <Link href="/login">Войти</Link>
                      </Button>
                    }
                  >
                    <HeaderLoginLink />
                  </Suspense>
                  <Button variant="secondary" asChild>
                    <Link href="/register">Регистрация</Link>
                  </Button>
                </>
              )}
            </>
          )}
        </div>
      </div>
    </header>
  );
}
