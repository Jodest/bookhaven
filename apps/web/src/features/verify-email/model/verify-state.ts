import { isApiError } from '@bookhaven/api-client';

export type VerifyState =
  | 'loading'
  | 'verified'
  | 'already_verified'
  | 'invalid_token'
  | 'token_expired'
  | 'token_used'
  | 'missing_token'
  | 'error';

export type VerifyFailureState = Extract<
  VerifyState,
  'invalid_token' | 'token_expired' | 'token_used' | 'missing_token'
>;

export function verifyResultToState(res: { status: string } | null | undefined): VerifyState {
  if (res?.status === 'verified') return 'verified';
  if (res?.status === 'already_verified') return 'already_verified';
  return 'error';
}

export function verifyErrorToState(err: unknown): VerifyState {
  if (!isApiError(err) || err.status !== 400) return 'error';
  switch (err.code) {
    case 'token_expired':
      return 'token_expired';
    case 'token_used':
      return 'token_used';
    default:
      // 'invalid_token' as well as an unknown/missing code on a 400.
      return 'invalid_token';
  }
}
