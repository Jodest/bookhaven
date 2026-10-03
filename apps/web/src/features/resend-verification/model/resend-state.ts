import { formatRetryAfter } from '@/shared/lib/format-retry-after';
import { isApiError } from '@bookhaven/api-client';

export type ResendState =
  | { kind: 'idle' }
  | { kind: 'sending' }
  | { kind: 'sent' }
  | { kind: 'rate_limited'; retryAfterSeconds: number }
  | { kind: 'error' };

// Used when a 429 arrives without `retry_after_seconds` in the body, and as
// the cool-down after a successful send (backend min resend interval).
export const DEFAULT_RESEND_COOLDOWN_SECONDS = 60;

export function resendErrorToState(err: unknown): ResendState {
  if (!isApiError(err)) return { kind: 'error' };

  if (err.status === 429 || err.code === 'rate_limited') {
    const retry = err.retryAfterSeconds;
    return {
      kind: 'rate_limited',
      retryAfterSeconds:
        typeof retry === 'number' && Number.isFinite(retry) && retry > 0
          ? Math.ceil(retry)
          : DEFAULT_RESEND_COOLDOWN_SECONDS,
    };
  }

  return { kind: 'error' };
}

export function resendMessage(state: ResendState): string | null {
  switch (state.kind) {
    case 'sent':
      // The public endpoint does not reveal whether the address exists.
      return 'Если этот адрес зарегистрирован и не подтверждён, мы отправили на него новое письмо.';
    case 'rate_limited':
      return `Слишком много запросов. Попробуйте снова через ${formatRetryAfter(
        state.retryAfterSeconds
      )}.`;
    case 'error':
      return 'Не удалось отправить письмо. Попробуйте позже.';
    default:
      return null;
  }
}
