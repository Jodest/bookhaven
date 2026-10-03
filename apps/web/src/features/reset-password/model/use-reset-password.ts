'use client';

import { useAuthStore } from '@/entities/session';
import { api } from '@bookhaven/api-client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { resetErrorToState, type ResetState } from './reset-state';

/**
 * Submits the new password with the emailed token. Nothing is sent on page
 * load: the token is checked only on submit.
 */
export function useResetPassword(token: string): {
  state: ResetState;
  submit: (password: string) => Promise<void>;
} {
  const [state, setState] = useState<ResetState>('idle');
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const submit = useCallback(
    async (password: string) => {
      setState('submitting');
      try {
        await api.auth.resetPassword(token, password);
      } catch (err) {
        if (mountedRef.current) setState(resetErrorToState(err));
        return;
      }
      // The server revoked every session and cleared the auth cookies. If
      // this browser was logged in, drop the in-memory user and the refresh
      // timer too; the logout request itself is expected to fail (no access
      // cookie any more), which is fine.
      const { isAuthenticated, logout } = useAuthStore.getState();
      if (isAuthenticated) {
        void logout().catch(() => undefined);
      }
      if (mountedRef.current) setState('success');
    },
    [token]
  );

  return { state, submit };
}
