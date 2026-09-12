import { describe, expect, it } from 'vitest';
import { isSafeRedirectPath } from './safe-redirect';

describe('isSafeRedirectPath', () => {
  it.each(['/books/some-slug', '/profile', '/shelves?tab=custom'])(
    'accepts %s',
    (value) => {
      expect(isSafeRedirectPath(value)).toBe(true);
    }
  );

  it.each(['//evil.com', 'https://evil.com', 'javascript:alert(1)', ''])(
    'rejects %s',
    (value) => {
      expect(isSafeRedirectPath(value)).toBe(false);
    }
  );

  it('rejects null', () => {
    expect(isSafeRedirectPath(null)).toBe(false);
  });

  it('rejects undefined', () => {
    expect(isSafeRedirectPath(undefined)).toBe(false);
  });

  it('rejects browser-normalized backslash variants', () => {
    expect(isSafeRedirectPath('/\\evil.com')).toBe(false);
  });

  it.each(['/\t/evil.com', '/\n/evil.com', '/\r/evil.com'])(
    'rejects control-character bypass %j',
    (value) => {
      expect(isSafeRedirectPath(value)).toBe(false);
    }
  );
});
