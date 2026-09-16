import { describe, expect, it } from 'vitest';
import { isSafeInternalRedirect, sanitizeRedirect } from './redirect';

const baseOrigin = 'https://admin.example.com';

describe('isSafeInternalRedirect', () => {
  it.each(['/dashboard', '/books/some-slug', '/users?tab=active'])(
    'accepts %s',
    (value) => {
      expect(isSafeInternalRedirect(value, baseOrigin)).toBe(true);
    }
  );

  it.each(['//evil.com', 'https://evil.com', 'javascript:alert(1)', ''])(
    'rejects %s',
    (value) => {
      expect(isSafeInternalRedirect(value, baseOrigin)).toBe(false);
    }
  );

  it('rejects null', () => {
    expect(isSafeInternalRedirect(null, baseOrigin)).toBe(false);
  });

  it('rejects undefined', () => {
    expect(isSafeInternalRedirect(undefined, baseOrigin)).toBe(false);
  });

  it('rejects raw backslash variants', () => {
    expect(isSafeInternalRedirect('/\\evil.com', baseOrigin)).toBe(false);
  });

  it.each(['/%5Cevil.com', '/%5cevil.com'])(
    'rejects percent-encoded backslash variant %s',
    (value) => {
      expect(isSafeInternalRedirect(value, baseOrigin)).toBe(false);
    }
  );

  it.each(['/\t/evil.com', '/\n/evil.com', '/\r/evil.com'])(
    'rejects control-character bypass %j',
    (value) => {
      expect(isSafeInternalRedirect(value, baseOrigin)).toBe(false);
    }
  );

  it('rejects when baseOrigin cannot be resolved against', () => {
    expect(isSafeInternalRedirect('/dashboard', 'not-a-valid-origin')).toBe(
      false
    );
  });

  it('rejects values over the max length', () => {
    const longPath = '/' + 'a'.repeat(2048);
    expect(isSafeInternalRedirect(longPath, baseOrigin)).toBe(false);
  });
});

describe('sanitizeRedirect', () => {
  it('returns the redirect value when safe', () => {
    expect(sanitizeRedirect('/dashboard', baseOrigin)).toBe('/dashboard');
  });

  it('returns the default fallback ("/") when unsafe', () => {
    expect(sanitizeRedirect('//evil.com', baseOrigin)).toBe('/');
  });

  it('returns a custom fallback when unsafe', () => {
    expect(sanitizeRedirect('https://evil.com', baseOrigin, '/login')).toBe(
      '/login'
    );
  });

  it('returns the custom fallback when redirect is null', () => {
    expect(sanitizeRedirect(null, baseOrigin, '/login')).toBe('/login');
  });
});
