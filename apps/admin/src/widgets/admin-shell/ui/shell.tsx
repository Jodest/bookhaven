'use client';

import { useAuthStore, isAdmin, isModerator } from '@/entities/session';
import { isSafeInternalRedirect } from '@/shared/lib';
import { AccessDenied } from './access-denied';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect } from 'react';
import { BookOpen, Users, Tag, MessageSquare, LayoutDashboard, LogOut } from 'lucide-react';

const adminNav = [
  { href: '/', label: 'Обзор', icon: LayoutDashboard },
  { href: '/books', label: 'Книги', icon: BookOpen },
  { href: '/authors', label: 'Авторы', icon: Users },
  { href: '/genres', label: 'Жанры', icon: Tag },
];

const moderatorNav = [
  { href: '/reviews', label: 'Отзывы', icon: MessageSquare },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const { user, logout, isLoading, forbiddenPath, clearForbiddenPath } = useAuthStore();
  const pathname = usePathname();
  const router = useRouter();

  const canAccessAdmin = isAdmin(user);
  const canAccessModerator = isModerator(user);

  useEffect(() => {
    if (pathname === '/login') return;
    if (!isLoading && !user) {
      const origin = typeof window !== 'undefined' ? window.location.origin : '';
      const currentUrl = typeof window !== 'undefined' ? window.location.href : '';
      const loginUrl = new URL('/login', origin || 'http://localhost');
      const redirectPath = currentUrl.replace(origin, '') || pathname;
      loginUrl.searchParams.set(
        'redirect',
        isSafeInternalRedirect(redirectPath, origin) ? redirectPath : '/'
      );
      router.push(loginUrl.toString() as any);
    }
  }, [isLoading, user, pathname, router]);

  useEffect(() => {
    if (forbiddenPath && forbiddenPath !== pathname) {
      clearForbiddenPath();
    }
  }, [forbiddenPath, pathname, clearForbiddenPath]);

  const handleLogout = async () => {
    await logout();
    router.push('/login' as any);
  };

  if (pathname === '/login') {
    return <>{children}</>;
  }

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-muted-foreground">Загрузка...</p>
      </div>
    );
  }

  if (!user) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-muted-foreground">Перенаправление на страницу входа...</p>
      </div>
    );
  }

  if (!canAccessAdmin && !canAccessModerator) {
    return (
      <AccessDenied message="У вас недостаточно прав для доступа к панели администратора." />
    );
  }

  if (forbiddenPath === pathname) {
    return (
      <AccessDenied message="Сервер отклонил запрос: у вас недостаточно прав для выполнения этого действия.">
        <Link href={"/" as any}>
          <Button variant="outline">На главную</Button>
        </Link>
      </AccessDenied>
    );
  }

  const navItems = [
    ...(canAccessAdmin ? adminNav : []),
    ...(canAccessModerator ? moderatorNav : []),
  ];

  // Admin-only route guard: if a moderator navigates to an admin-only path, show forbidden.
  const adminOnlyPaths = ['/books', '/authors', '/genres'];
  const isAdminOnlyPath = pathname === '/' || adminOnlyPaths.some((p) => pathname === p || pathname.startsWith(`${p}/`));
  if (isAdminOnlyPath && !canAccessAdmin) {
    return (
      <AccessDenied message="Этот раздел доступен только администраторам. Модераторы могут работать только с отзывами.">
        <Link href={"/reviews" as any}>
          <Button variant="outline">К отзывам</Button>
        </Link>
      </AccessDenied>
    );
  }

  return (
    <div className="flex min-h-screen flex-col md:flex-row">
      <aside className="w-full border-b bg-card md:w-64 md:border-b-0 md:border-r">
        <div className="flex h-14 items-center border-b px-4 font-semibold">
          BookHaven Admin
        </div>
        <nav className="space-y-1 p-4">
          {navItems.map((item) => {
            const active = pathname === item.href || pathname.startsWith(`${item.href}/`);
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href as any}
                className={`flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                  active
                    ? 'bg-primary text-primary-foreground'
                    : 'text-foreground hover:bg-muted'
                }`}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>
        <div className="border-t p-4">
          <div className="mb-2 text-sm text-muted-foreground">
            {user?.email}
          </div>
          <Button variant="outline" size="sm" className="w-full" onClick={handleLogout}>
            <LogOut className="mr-2 h-4 w-4" />
            Выйти
          </Button>
        </div>
      </aside>
      <main className="flex-1 overflow-auto p-4 md:p-8">{children}</main>
    </div>
  );
}
