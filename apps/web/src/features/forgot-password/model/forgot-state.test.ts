import { ApiError } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { forgotErrorToState, forgotMessage } from './forgot-state';

describe('forgotErrorToState', () => {
  it('maps 429 with retryAfterSeconds to rate_limited with that value', () => {
    const err = new ApiError(429, 'Too Many Requests', 'x', '/p', 'rate_limited', 90);
    expect(forgotErrorToState(err)).toEqual({ kind: 'rate_limited', retryAfterSeconds: 90 });
  });

  it('falls back to 60 seconds for 429 without retryAfterSeconds', () => {
    const err = new ApiError(429, 'Too Many Requests', 'x', '/p', 'rate_limited');
    expect(forgotErrorToState(err)).toEqual({ kind: 'rate_limited', retryAfterSeconds: 60 });
  });

  it('maps 400 invalid_email to invalid_email', () => {
    const err = new ApiError(400, 'Bad Request', 'x', '/p', 'invalid_email');
    expect(forgotErrorToState(err)).toEqual({ kind: 'invalid_email' });
  });

  it('maps 500 email_send_failed to error', () => {
    const err = new ApiError(500, 'Internal Server Error', 'x', '/p', 'email_send_failed');
    expect(forgotErrorToState(err)).toEqual({ kind: 'error' });
  });

  it('maps a non-ApiError (network) to error', () => {
    expect(forgotErrorToState(new TypeError('Failed to fetch'))).toEqual({ kind: 'error' });
  });
});

describe('forgotMessage', () => {
  it('renders the rate-limit message with formatted wait', () => {
    expect(forgotMessage({ kind: 'rate_limited', retryAfterSeconds: 90 })).toBe(
      'Слишком много запросов. Попробуйте снова через 2 мин.'
    );
  });

  it('renders the generic error message', () => {
    expect(forgotMessage({ kind: 'error' })).toBe('Не удалось отправить письмо. Попробуйте позже.');
  });

  it('returns null for the other states', () => {
    expect(forgotMessage({ kind: 'idle' })).toBeNull();
    expect(forgotMessage({ kind: 'sending' })).toBeNull();
    expect(forgotMessage({ kind: 'sent' })).toBeNull();
    expect(forgotMessage({ kind: 'invalid_email' })).toBeNull();
  });
});
