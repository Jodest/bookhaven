import { ApiError } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { verifyErrorToState, verifyResultToState } from './verify-state';

describe('verifyResultToState', () => {
  it('maps verified', () => {
    expect(verifyResultToState({ status: 'verified' })).toBe('verified');
  });

  it('maps already_verified', () => {
    expect(verifyResultToState({ status: 'already_verified' })).toBe('already_verified');
  });

  it('maps an unexpected body to error', () => {
    expect(verifyResultToState({ status: 'weird' })).toBe('error');
  });
});

describe('verifyErrorToState', () => {
  it.each(['invalid_token', 'token_expired', 'token_used'] as const)('maps 400 %s', (code) => {
    expect(verifyErrorToState(new ApiError(400, 'Bad Request', 'x', '/p', code))).toBe(code);
  });

  it('maps 400 without a code to invalid_token', () => {
    expect(verifyErrorToState(new ApiError(400, 'Bad Request'))).toBe('invalid_token');
  });

  it('maps 400 with an unknown code to invalid_token', () => {
    expect(verifyErrorToState(new ApiError(400, 'Bad Request', 'x', '/p', 'something'))).toBe(
      'invalid_token'
    );
  });

  it('maps 500 to error', () => {
    expect(verifyErrorToState(new ApiError(500, 'Internal Server Error'))).toBe('error');
  });

  it('maps a network error to error', () => {
    expect(verifyErrorToState(new TypeError('Failed to fetch'))).toBe('error');
  });
});
