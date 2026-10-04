import { describe, expect, it } from 'vitest';
import {
  isAuthPagePath,
  isSafeRedirectPath,
  loginHrefWithRedirect,
  postLoginRedirectPath,
} from './safe-redirect';

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

describe('loginHrefWithRedirect', () => {
  it('keeps a safe redirect', () => {
    expect(loginHrefWithRedirect('/books/abc')).toBe('/login?redirect=%2Fbooks%2Fabc');
  });

  it.each(['//evil.com', '', null, undefined])('drops unsafe redirect %j', (value) => {
    expect(loginHrefWithRedirect(value)).toBe('/login');
  });
});

describe('isAuthPagePath', () => {
  it.each(['/login', '/register', '/register?redirect=%2Fbooks', '/register/', '/reset-password#x'])(
    'treats %s as an auth page',
    (value) => {
      expect(isAuthPagePath(value)).toBe(true);
    }
  );

  it.each(['/', '/profile', '/books/register', '/registered'])(
    'does not treat %s as an auth page',
    (value) => {
      expect(isAuthPagePath(value)).toBe(false);
    }
  );
});

describe('postLoginRedirectPath', () => {
  it('keeps a safe non-auth redirect', () => {
    expect(postLoginRedirectPath('/books/abc')).toBe('/books/abc');
  });

  it.each(['/register', '/login?redirect=%2Fbooks', '//evil.com', null])(
    'falls back to /profile for %j',
    (value) => {
      expect(postLoginRedirectPath(value)).toBe('/profile');
    }
  );
});
