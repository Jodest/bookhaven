import { describe, expect, it } from 'vitest';

import { middleware } from './middleware';

// `middleware()` only reads `request.cookies.get()` and `request.nextUrl` (plus
// `request.url` when building the redirect target), so a minimal stub matching
// that shape is enough -- no need to construct a real `NextRequest`.
function makeRequest(
  pathname: string,
  cookies: Record<string, string> = {},
) {
  const url = `http://localhost:3000${pathname}`;
  return {
    nextUrl: { pathname, search: '' },
    url,
    cookies: {
      get: (name: string) =>
        name in cookies ? { name, value: cookies[name] } : undefined,
    },
  } as unknown as Parameters<typeof middleware>[0];
}

describe('middleware', () => {
  it('redirects to /login for a protected route with no auth cookies', () => {
    const response = middleware(makeRequest('/profile'));

    expect(response.status).toBe(307);
    expect(response.headers.get('location')).toContain('/login');
  });

  it('lets a protected route through when access_token is present', () => {
    const response = middleware(
      makeRequest('/profile', { access_token: 'valid-access-token' }),
    );

    expect(response.status).not.toBe(307);
    expect(response.headers.get('location')).toBeNull();
  });

  it('lets a protected route through when only has_session is present (expired access token, valid refresh token)', () => {
    const response = middleware(
      makeRequest('/profile', { has_session: '1' }),
    );

    expect(response.status).not.toBe(307);
    expect(response.headers.get('location')).toBeNull();
  });

  it('redirects to /login for a protected route when neither cookie is present', () => {
    const response = middleware(makeRequest('/shelves'));

    expect(response.status).toBe(307);
    expect(response.headers.get('location')).toContain('/login');
  });
});
