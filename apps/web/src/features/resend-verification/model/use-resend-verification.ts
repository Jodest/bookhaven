'use client';

import { api } from '@bookhaven/api-client';
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  DEFAULT_RESEND_COOLDOWN_SECONDS,
  resendErrorToState,
  type ResendState,
} from './resend-state';

export function useResendVerification(): {
  state: ResendState;
  resend: (email: string) => Promise<void>;
  disabled: boolean;
} {
  const [state, setState] = useState<ResendState>({ kind: 'idle' });
  const [coolingDown, setCoolingDown] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, []);

  const startCooldown = useCallback((seconds: number) => {
    if (timerRef.current) clearTimeout(timerRef.current);
    setCoolingDown(true);
    timerRef.current = setTimeout(() => {
      timerRef.current = null;
      if (mountedRef.current) setCoolingDown(false);
    }, seconds * 1000);
  }, []);

  const resend = useCallback(async (email: string) => {
    setState({ kind: 'sending' });
    try {
      await api.auth.resendVerification(email);
      if (!mountedRef.current) return;
      setState({ kind: 'sent' });
      startCooldown(DEFAULT_RESEND_COOLDOWN_SECONDS);
    } catch (err) {
      const next = resendErrorToState(err);
      if (!mountedRef.current) return;
      setState(next);
      if (next.kind === 'rate_limited') {
        startCooldown(next.retryAfterSeconds);
      }
    }
  }, [startCooldown]);

  const disabled = state.kind === 'sending' || coolingDown;

  return { state, resend, disabled };
}
