'use client';

import { useAuthStore } from '@/entities/session';
import { ResendVerificationButton } from '@/features/resend-verification';
import { Button } from '@bookhaven/ui';
import { usePathname } from 'next/navigation';
import { useEffect, useState } from 'react';

const DISMISS_KEY_PREFIX = 'bookhaven:verify-banner-dismissed:';
const HIDDEN_ON_PATHS = ['/register', '/verify-email'];

export function dismissKey(userId: string): string {
  return `${DISMISS_KEY_PREFIX}${userId}`;
}

function readDismissed(userId: string): boolean {
  try {
    return window.sessionStorage.getItem(dismissKey(userId)) === '1';
  } catch {
    return false;
  }
}

export function EmailVerificationBanner() {
  const { user, isAuthenticated, isLoading } = useAuthStore();
  const pathname = usePathname();
  const userId = user?.id ?? null;
  // null = not yet read from sessionStorage (client-only, avoids hydration
  // mismatch: nothing renders until the effect runs).
  const [dismissedFor, setDismissedFor] = useState<{ userId: string; dismissed: boolean } | null>(
    null
  );

  useEffect(() => {
    if (!userId) {
      setDismissedFor(null);
      return;
    }
    setDismissedFor({ userId, dismissed: readDismissed(userId) });
  }, [userId]);

  if (
    isLoading ||
    !isAuthenticated ||
    !user ||
    user.emailVerified !== false ||
    HIDDEN_ON_PATHS.includes(pathname ?? '') ||
    !dismissedFor ||
    dismissedFor.userId !== user.id ||
    dismissedFor.dismissed
  ) {
    return null;
  }

  const dismiss = () => {
    try {
      window.sessionStorage.setItem(dismissKey(user.id), '1');
    } catch {
      // Storage unavailable: hide for this render tree only.
    }
    setDismissedFor({ userId: user.id, dismissed: true });
  };

  return (
    <div
      role="region"
      aria-label="Подтверждение email"
      className="border-b bg-secondary text-secondary-foreground"
    >
      <div className="container mx-auto flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-2 text-sm sm:px-6 lg:px-8">
        <p className="flex-1">
          Подтвердите адрес электронной почты: мы отправили ссылку на{' '}
          <strong className="font-semibold">{user.email}</strong>.
        </p>
        <ResendVerificationButton size="sm" variant="outline" />
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label="Скрыть"
          className="h-8 w-8"
          onClick={dismiss}
        >
          ×
        </Button>
      </div>
    </div>
  );
}
