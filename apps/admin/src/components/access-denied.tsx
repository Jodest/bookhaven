'use client';

import { useAuthStore } from '@/lib/auth-store';
import { Button } from '@bookhaven/ui';
import { LogOut } from 'lucide-react';
import { ReactNode } from 'react';

interface AccessDeniedProps {
  title?: string;
  message?: string;
  children?: ReactNode;
}

export function AccessDenied({
  title = 'Доступ запрещен',
  message = 'У вас недостаточно прав для доступа к этому разделу.',
  children,
}: AccessDeniedProps) {
  const { logout } = useAuthStore();

  const handleLogout = async () => {
    await logout();
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 p-4 text-center">
      <h1 className="text-2xl font-bold">{title}</h1>
      <p className="max-w-md text-muted-foreground">{message}</p>
      <div className="flex flex-wrap items-center justify-center gap-2">
        {children}
        <Button onClick={handleLogout} variant="outline">
          <LogOut className="mr-2 h-4 w-4" />
          Выйти
        </Button>
      </div>
    </div>
  );
}
