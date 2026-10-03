'use client';

import { api } from '@bookhaven/api-client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { FORGOT_COOLDOWN_SECONDS, forgotErrorToState, type ForgotState } from './forgot-state';

export function useForgotPassword(): {
  state: ForgotState;
  submit: (email: string) => Promise<void>;
  // Back from the "sent" panel to the form.
  backToForm: () => void;
  // True while a request is in flight or during the cool-down after a send
  // or a 429.
  disabled: boolean;
} {
  const [state, setState] = useState<ForgotState>({ kind: 'idle' });
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

  const submit = useCallback(
    async (email: string) => {
      setState({ kind: 'sending' });
      try {
        await api.auth.forgotPassword(email);
        if (!mountedRef.current) return;
        setState({ kind: 'sent' });
        startCooldown(FORGOT_COOLDOWN_SECONDS);
      } catch (err) {
        const next = forgotErrorToState(err);
        if (!mountedRef.current) return;
        setState(next);
        if (next.kind === 'rate_limited') {
          startCooldown(next.retryAfterSeconds);
        }
      }
    },
    [startCooldown]
  );

  const backToForm = useCallback(() => setState({ kind: 'idle' }), []);

  const disabled = state.kind === 'sending' || coolingDown;

  return { state, submit, backToForm, disabled };
}
