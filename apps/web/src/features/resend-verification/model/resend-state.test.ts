import { ApiError } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { formatRetryAfter, resendErrorToState, resendMessage } from './resend-state';

describe('resendErrorToState', () => {
  it('maps 429 with retryAfterSeconds to rate_limited with that value', () => {
    const err = new ApiError(429, 'Too Many Requests', 'x', '/p', 'rate_limited', 90);
    expect(resendErrorToState(err)).toEqual({ kind: 'rate_limited', retryAfterSeconds: 90 });
  });

  it('falls back to 60 seconds for 429 without retryAfterSeconds', () => {
    const err = new ApiError(429, 'Too Many Requests', 'x', '/p', 'rate_limited');
    expect(resendErrorToState(err)).toEqual({ kind: 'rate_limited', retryAfterSeconds: 60 });
  });

  it('maps 400 invalid_email to error', () => {
    const err = new ApiError(400, 'Bad Request', 'x', '/p', 'invalid_email');
    expect(resendErrorToState(err)).toEqual({ kind: 'error' });
  });

  it('maps 500 email_send_failed to error', () => {
    const err = new ApiError(500, 'Internal Server Error', 'x', '/p', 'email_send_failed');
    expect(resendErrorToState(err)).toEqual({ kind: 'error' });
  });

  it('maps a non-ApiError (network) to error', () => {
    expect(resendErrorToState(new TypeError('Failed to fetch'))).toEqual({ kind: 'error' });
  });
});

describe('formatRetryAfter', () => {
  it.each([
    [1, '1 сек'],
    [59, '59 сек'],
    [60, '1 мин'],
    [61, '2 мин'],
    [3599, '60 мин'],
    [3600, '1 ч'],
    [86400, '24 ч'],
  ])('%i -> %s', (seconds, expected) => {
    expect(formatRetryAfter(seconds)).toBe(expected);
  });
});

describe('resendMessage', () => {
  it('renders the rate-limit message with formatted wait', () => {
    expect(resendMessage({ kind: 'rate_limited', retryAfterSeconds: 90 })).toBe(
      'Слишком много запросов. Попробуйте снова через 2 мин.'
    );
  });

  it('renders the enumeration-safe sent message', () => {
    expect(resendMessage({ kind: 'sent' })).toBe(
      'Если этот адрес зарегистрирован и не подтверждён, мы отправили на него новое письмо.'
    );
  });

  it('returns null for idle and sending', () => {
    expect(resendMessage({ kind: 'idle' })).toBeNull();
    expect(resendMessage({ kind: 'sending' })).toBeNull();
  });
});
