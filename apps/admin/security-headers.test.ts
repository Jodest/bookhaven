import { describe, expect, it } from 'vitest';

// eslint-disable-next-line @typescript-eslint/no-require-imports
const { securityHeaders } = require('./security-headers') as {
  securityHeaders: (o?: { isDev?: boolean; apiUrl?: string; noindex?: boolean }) => { key: string; value: string }[];
};

const get = (h: { key: string; value: string }[], k: string) => h.find((x) => x.key === k)?.value;

describe('securityHeaders', () => {
  it('prod values', () => {
    const h = securityHeaders({ apiUrl: 'https://api.example.test/v1', noindex: true });
    expect(get(h, 'X-Frame-Options')).toBe('DENY');
    expect(get(h, 'X-Content-Type-Options')).toBe('nosniff');
    expect(get(h, 'Referrer-Policy')).toBe('strict-origin-when-cross-origin');
    expect(get(h, 'Cross-Origin-Opener-Policy')).toBe('same-origin');
    expect(get(h, 'Permissions-Policy')).toBe(
      'camera=(), microphone=(), geolocation=(), payment=(), usb=(), browsing-topics=()',
    );
    expect(get(h, 'Content-Security-Policy')).toBe(
      "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; " +
        "img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self' https://api.example.test; " +
        "frame-src 'none'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'",
    );
    expect(get(h, 'Strict-Transport-Security')).toBeUndefined();
  });

  it('dev adds unsafe-eval and ws', () => {
    const csp = get(securityHeaders({ isDev: true, apiUrl: 'http://localhost:8080' }), 'Content-Security-Policy')!;
    expect(csp).toContain("script-src 'self' 'unsafe-inline' 'unsafe-eval'");
    expect(csp).toContain("connect-src 'self' http://localhost:8080 ws: wss:");
  });

  it('omits API origin when empty or invalid', () => {
    for (const apiUrl of ['', 'not a url']) {
      expect(get(securityHeaders({ apiUrl }), 'Content-Security-Policy')).toContain("connect-src 'self'; frame-src");
    }
  });

  it('X-Robots-Tag only with noindex', () => {
    expect(get(securityHeaders(), 'X-Robots-Tag')).toBeUndefined();
    expect(get(securityHeaders({ noindex: true }), 'X-Robots-Tag')).toBe('noindex, nofollow');
  });
});
