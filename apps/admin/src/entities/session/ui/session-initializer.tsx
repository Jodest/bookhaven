'use client';

import { useEffect } from 'react';
import { useAuthStore } from '../model/auth-store';

export function SessionInitializer() {
  const fetchUser = useAuthStore((state) => state.fetchUser);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  return null;
}
