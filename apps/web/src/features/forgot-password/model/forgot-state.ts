import { formatRetryAfter } from '@/shared/lib/format-retry-after';
import { isApiError } from '@bookhaven/api-client';

export type ForgotState =
  | { kind: 'idle' }
  | { kind: 'sending' }
  | { kind: 'sent' }
  | { kind: 'rate_limited'; retryAfterSeconds: number }
  | { kind: 'invalid_email' }
  | { kind: 'error' };

// Cool-down of "Отправить ещё раз" after a send (backend: 1 request per
// minute per email), and the fallback wait for a 429 without
// `retry_after_seconds`.
export const FORGOT_COOLDOWN_SECONDS = 60;

export function forgotErrorToState(err: unknown): ForgotState {
  if (!isApiError(err)) return { kind: 'error' };

  if (err.status === 429 || err.code === 'rate_limited') {
    const retry = err.retryAfterSeconds;
    return {
      kind: 'rate_limited',
      retryAfterSeconds:
        typeof retry === 'number' && Number.isFinite(retry) && retry > 0
          ? Math.ceil(retry)
          : FORGOT_COOLDOWN_SECONDS,
    };
  }

  if (err.status === 400 || err.code === 'invalid_email') {
    return { kind: 'invalid_email' };
  }

  return { kind: 'error' };
}

export const FORGOT_SENT_MESSAGE =
  'Если аккаунт с таким email существует, мы отправили на него письмо со ссылкой для сброса пароля. Ссылка действительна 1 час.';

export const INVALID_EMAIL_MESSAGE = 'Введите корректный email';

// Form-level message for the states that keep the form on screen
// (invalid_email is shown as a field error instead).
export function forgotMessage(state: ForgotState): string | null {
  switch (state.kind) {
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
