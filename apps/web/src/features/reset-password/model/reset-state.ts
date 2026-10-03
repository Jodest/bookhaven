import { isApiError } from '@bookhaven/api-client';

// Outcome of a failed POST /auth/reset-password.
export type ResetFailure =
  | 'invalid_token'
  | 'token_expired'
  | 'token_used'
  | 'invalid_password'
  | 'error';

export type ResetState = 'idle' | 'submitting' | 'success' | ResetFailure;

// States in which the link is unusable: the form is replaced by an
// explanation and a link to request a new one.
export type ResetLinkFailure = Extract<ResetFailure, 'invalid_token' | 'token_expired' | 'token_used'>;

export function isResetLinkFailure(state: ResetState): state is ResetLinkFailure {
  return state === 'invalid_token' || state === 'token_expired' || state === 'token_used';
}

export function resetErrorToState(err: unknown): ResetFailure {
  if (!isApiError(err) || err.status !== 400) return 'error';
  switch (err.code) {
    case 'invalid_password':
      return 'invalid_password';
    case 'token_expired':
      return 'token_expired';
    case 'token_used':
      return 'token_used';
    default:
      // 'invalid_token' as well as an unknown/missing code on a 400.
      return 'invalid_token';
  }
}

export const PASSWORD_MIN_MESSAGE = 'Пароль должен быть не короче 8 символов';
