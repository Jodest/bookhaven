'use client';

import { useAuthStore, isAdmin, isModerator } from '@/entities/session';
import {
  getHomeHref,
  getHomeTab,
  getSectionHref,
  getVisibleSections,
  getVisibleTabs,
  canAccessTab,
  resolveNavLocation,
} from '@/shared/config';
import { isSafeInternalRedirect } from '@/shared/lib';
import { AccessDenied } from './access-denied';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect } from 'react';
import { LogOut } from 'lucide-react';

export function Shell({ children }: { children: React.ReactNode }) {
  const { user, logout, isLoading, forbiddenPath, clearForbiddenPath } = useAuthStore();
  const pathname = usePathname();
  const router = useRouter();

  const canAccessAdmin = isAdmin(user);
  const canAccessModerator = isModerator(user);

  // Config-driven guard. Computed before any early return so the redirect
  // effect below is always registered (hooks order).
  const role = user?.role;
  const loc = resolveNavLocation(pathname);
  const isAllowed = loc
    ? loc.tab
      ? canAccessTab(loc.tab, role)
      : getVisibleTabs(loc.section, role).length > 0
    : canAccessAdmin;
  const homeHref = getHomeHref(role);
  // A staff user whose home is not `/` (moderator) is sent home from `/`.
  const shouldRedirectHome =
    !!user && canAccessModerator && pathname === '/' && !isAllowed && homeHref !== '/';

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

  useEffect(() => {
    if (shouldRedirectHome) {
      router.replace(homeHref as any);
    }
  }, [shouldRedirectHome, homeHref, router]);

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

  const homeLabel = getHomeTab(role)?.homeLabel ?? 'На главную';

  if (forbiddenPath === pathname) {
    return (
      <AccessDenied message="Сервер отклонил запрос: у вас недостаточно прав для выполнения этого действия.">
        <Link href={homeHref as any}>
          <Button variant="outline">{homeLabel}</Button>
        </Link>
      </AccessDenied>
    );
  }

  if (shouldRedirectHome) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-muted-foreground">Перенаправление...</p>
      </div>
    );
  }

  if (!isAllowed) {
    return (
      <AccessDenied message="Этот раздел доступен только администраторам.">
        <Link href={homeHref as any}>
          <Button variant="outline">{homeLabel}</Button>
        </Link>
      </AccessDenied>
    );
  }

  const sections = getVisibleSections(role);

  return (
    <div className="flex min-h-screen flex-col md:flex-row">
      <aside className="w-full border-b bg-card md:w-64 md:border-b-0 md:border-r">
        <div className="flex h-14 items-center border-b px-4 font-semibold">
          BookHaven Admin
        </div>
        <nav className="space-y-1 p-4">
          {sections.map((section) => {
            const href = getSectionHref(section, role) ?? '/';
            const active = loc?.section.id === section.id;
            const Icon = section.icon;
            return (
              <Link
                key={section.id}
                href={href as any}
                aria-current={active ? 'page' : undefined}
                className={`flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                  active
                    ? 'bg-primary text-primary-foreground'
                    : 'text-foreground hover:bg-muted'
                }`}
              >
                <Icon className="h-4 w-4" />
                {section.label}
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
