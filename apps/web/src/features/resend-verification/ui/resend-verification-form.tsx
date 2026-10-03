'use client';

import { Button, cn } from '@bookhaven/ui';
import { useState, type FormEvent } from 'react';
import { resendMessage } from '../model/resend-state';
import { useResendVerification } from '../model/use-resend-verification';

interface ResendVerificationFormProps {
  className?: string;
}

/**
 * Email input + resend button for people without a session (e.g. an expired
 * link on /verify-email). Only a light client-side check; the backend
 * validates the address and answers the same way for unknown emails.
 */
export function ResendVerificationForm({ className }: ResendVerificationFormProps) {
  const [email, setEmail] = useState('');
  const [invalid, setInvalid] = useState(false);
  const { state, resend, disabled } = useResendVerification();
  const message = invalid ? 'Введите корректный email' : resendMessage(state);
  const isError = invalid || state.kind === 'error' || state.kind === 'rate_limited';

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const value = email.trim();
    if (!value || !value.includes('@')) {
      setInvalid(true);
      return;
    }
    setInvalid(false);
    void resend(value);
  };

  return (
    <form onSubmit={onSubmit} noValidate className={cn('space-y-3', className)}>
      <div>
        <label htmlFor="resend-email" className="block text-sm font-medium">
          Email
        </label>
        <input
          id="resend-email"
          type="email"
          required
          autoComplete="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
        />
      </div>
      <Button type="submit" className="w-full" disabled={disabled}>
        {state.kind === 'sending' ? 'Отправляем…' : 'Отправить письмо ещё раз'}
      </Button>
      <p
        aria-live="polite"
        className={cn('text-sm', isError ? 'text-destructive' : 'text-muted-foreground')}
      >
        {message}
      </p>
    </form>
  );
}
