'use client';

import { api } from '@bookhaven/api-client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { verifyErrorToState, verifyResultToState, type VerifyState } from './verify-state';

/**
 * Posts the verification token exactly once per page load.
 *
 * StrictMode double-mounts effects in dev; a ref remembers the token already
 * submitted so the second run is a no-op. There is deliberately no
 * "cancelled" cleanup flag: under StrictMode it would discard the result of
 * the only in-flight request and leave the page stuck on `loading`.
 */
export function useVerifyEmail(token: string | null): {
  state: VerifyState;
  retry: () => void;
} {
  const [state, setState] = useState<VerifyState>(token ? 'loading' : 'missing_token');
  const submittedRef = useRef<string | null>(null);

  const submit = useCallback((value: string) => {
    setState('loading');
    api.auth
      .verifyEmail(value)
      .then((res) => {
        // No session to refresh: unverified users cannot be logged in, and
        // verification does not log anyone in.
        setState(verifyResultToState(res));
      })
      .catch((err: unknown) => {
        setState(verifyErrorToState(err));
      });
  }, []);

  useEffect(() => {
    if (!token) {
      setState('missing_token');
      return;
    }
    if (submittedRef.current === token) return;
    submittedRef.current = token;
    submit(token);
  }, [token, submit]);

  const retry = useCallback(() => {
    if (token) submit(token);
  }, [token, submit]);

  return { state, retry };
}
