import { ApiError } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { isResetLinkFailure, resetErrorToState } from './reset-state';

describe('resetErrorToState', () => {
  it.each(['invalid_token', 'token_expired', 'token_used', 'invalid_password'] as const)(
    'maps 400 %s',
    (code) => {
      expect(resetErrorToState(new ApiError(400, 'Bad Request', 'x', '/p', code))).toBe(code);
    }
  );

  it('maps 400 without a code to invalid_token', () => {
    expect(resetErrorToState(new ApiError(400, 'Bad Request'))).toBe('invalid_token');
  });

  it('maps 400 with an unknown code to invalid_token', () => {
    expect(resetErrorToState(new ApiError(400, 'Bad Request', 'x', '/p', 'something'))).toBe(
      'invalid_token'
    );
  });

  it('maps 500 to error', () => {
    expect(resetErrorToState(new ApiError(500, 'Internal Server Error'))).toBe('error');
  });

  it('maps 429 to error', () => {
    expect(
      resetErrorToState(new ApiError(429, 'Too Many Requests', 'x', '/p', 'rate_limited', 5))
    ).toBe('error');
  });

  it('maps a non-ApiError (network) to error', () => {
    expect(resetErrorToState(new TypeError('Failed to fetch'))).toBe('error');
  });
});

describe('isResetLinkFailure', () => {
  it('is true only for link failures', () => {
    expect(isResetLinkFailure('invalid_token')).toBe(true);
    expect(isResetLinkFailure('token_expired')).toBe(true);
    expect(isResetLinkFailure('token_used')).toBe(true);
    expect(isResetLinkFailure('invalid_password')).toBe(false);
    expect(isResetLinkFailure('error')).toBe(false);
    expect(isResetLinkFailure('success')).toBe(false);
  });
});
