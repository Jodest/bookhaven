import type { ErrorEvent } from '@sentry/nextjs';
import { describe, expect, it } from 'vitest';

import { scrubEvent, shouldInitSentry } from './sentry-scrub';

const JWT = 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig_nature-1';

describe('shouldInitSentry', () => {
  it.each([undefined, '', '   '])('is false for %j', (dsn) => {
    expect(shouldInitSentry({ SENTRY_DSN: dsn })).toBe(false);
  });
  it('is true for a DSN', () => {
    expect(shouldInitSentry({ SENTRY_DSN: ' https://k@o0.ingest.invalid/0 ' })).toBe(true);
  });
});

describe('scrubEvent', () => {
  const makeEvent = (): ErrorEvent =>
    ({
      type: undefined,
      message: `fail for a.b+c@mail.example.org Bearer abc.def-123 ${JWT} ?token=xyz&x=1 password=hunter2`,
      exception: { values: [{ type: 'Error', value: `user bob@example.com secret=s3 ${JWT}` }] },
      request: {
        url: 'https://h/reset-password?token=tok123&next=a@b.com#frag',
        cookies: { sid: '1' },
        headers: { authorization: 'x' },
        data: 'body',
        query_string: 'q=1',
      },
      transaction: 'GET /verify-email?token=tok456',
      user: { id: '1', email: 'a@b.com', ip_address: '1.2.3.4' },
    }) as ErrorEvent;

  it('strips request and user data and redacts strings', () => {
    const out = scrubEvent(makeEvent(), {});
    expect(out).not.toBeNull();
    expect(out!.request).toEqual({ url: 'https://h/reset-password' });
    expect(out!.transaction).toBe('GET /verify-email');
    expect(out!.user).toBeUndefined();
    const text = `${out!.message} ${out!.exception!.values![0].value}`;
    for (const bad of ['@', 'abc.def-123', 'eyJ', 'xyz', 'hunter2', 's3']) {
      expect(text).not.toContain(bad);
    }
    for (const good of ['[email]', 'Bearer [redacted]', '[jwt]', 'token=[redacted]', 'password=[redacted]']) {
      expect(out!.message).toContain(good);
    }
  });

  it.each([404, 401, 499])('drops events whose original exception has status %i', (status) => {
    expect(scrubEvent(makeEvent(), { originalException: { status } })).toBeNull();
    expect(scrubEvent(makeEvent(), { originalException: { statusCode: status } })).toBeNull();
  });

  it('keeps status 500, 399 and plain errors', () => {
    expect(scrubEvent(makeEvent(), { originalException: { status: 500 } })).not.toBeNull();
    expect(scrubEvent(makeEvent(), { originalException: { status: 399 } })).not.toBeNull();
    expect(scrubEvent(makeEvent(), { originalException: new Error('boom') })).not.toBeNull();
    expect(scrubEvent(makeEvent())).not.toBeNull();
  });
});
