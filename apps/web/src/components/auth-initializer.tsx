'use client';

import { useAuthStore } from '@/lib/auth-store';
import { useEffect } from 'react';

export function AuthInitializer() {
  const fetchUser = useAuthStore((state) => state.fetchUser);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  return null;
}
