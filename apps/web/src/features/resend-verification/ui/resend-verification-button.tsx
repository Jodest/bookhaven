'use client';

import { Button, cn } from '@bookhaven/ui';
import type { ComponentProps } from 'react';
import { resendMessage } from '../model/resend-state';
import { useResendVerification } from '../model/use-resend-verification';

type ButtonProps = ComponentProps<typeof Button>;

interface ResendVerificationButtonProps {
  variant?: ButtonProps['variant'];
  size?: ButtonProps['size'];
  className?: string;
}

export function ResendVerificationButton({
  variant = 'outline',
  size,
  className,
}: ResendVerificationButtonProps) {
  const { state, resend, disabled } = useResendVerification();
  const message = resendMessage(state);
  const isError = state.kind === 'error' || state.kind === 'rate_limited';

  return (
    <span className={cn('inline-flex flex-wrap items-center gap-x-3 gap-y-1', className)}>
      <Button
        type="button"
        variant={variant}
        size={size}
        disabled={disabled}
        onClick={() => void resend()}
      >
        {state.kind === 'sending' ? 'Отправляем…' : 'Отправить письмо ещё раз'}
      </Button>
      <span
        aria-live="polite"
        className={cn('text-sm', isError ? 'text-destructive' : 'text-muted-foreground')}
      >
        {message}
      </span>
    </span>
  );
}
