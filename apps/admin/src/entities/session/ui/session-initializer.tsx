'use client';

import { useEffect } from 'react';
import { useAuthStore } from '../model/auth-store';

export function SessionInitializer() {
  const fetchUser = useAuthStore((state) => state.fetchUser);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  useEffect(() => {
    // Recover promptly if a proactive refresh timer was throttled while the
    // tab was hidden/backgrounded (e.g. laptop sleep) — re-validate the
    // session as soon as the tab becomes visible again.
    function handleVisibilityChange() {
      if (document.visibilityState === 'visible') {
        fetchUser();
      }
    }
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [fetchUser]);

  return null;
}
