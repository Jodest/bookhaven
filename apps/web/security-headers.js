// HTTP security headers for the Next.js app (docs/features/security-headers.md).
// HSTS is intentionally absent: Caddy adds it, only over TLS.

function apiOrigin(apiUrl) {
  if (!apiUrl) return null;
  try {
    return new URL(apiUrl).origin;
  } catch {
    return null;
  }
}

function buildCsp({ isDev, apiUrl }) {
  const origin = apiOrigin(apiUrl);
  const connect = ["'self'"];
  if (origin) connect.push(origin);
  if (isDev) connect.push('ws:', 'wss:');
  const script = ["'self'", "'unsafe-inline'"];
  if (isDev) script.push("'unsafe-eval'");
  return [
    "default-src 'self'",
    `script-src ${script.join(' ')}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob: https:",
    "font-src 'self' data:",
    `connect-src ${connect.join(' ')}`,
    "frame-src 'none'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
  ].join('; ');
}

function securityHeaders({ isDev = false, apiUrl = '', noindex = false } = {}) {
  const headers = [
    { key: 'Content-Security-Policy', value: buildCsp({ isDev, apiUrl }) },
    { key: 'X-Frame-Options', value: 'DENY' },
    { key: 'X-Content-Type-Options', value: 'nosniff' },
    { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
    {
      key: 'Permissions-Policy',
      value: 'camera=(), microphone=(), geolocation=(), payment=(), usb=(), browsing-topics=()',
    },
    { key: 'Cross-Origin-Opener-Policy', value: 'same-origin' },
  ];
  if (noindex) headers.push({ key: 'X-Robots-Tag', value: 'noindex, nofollow' });
  return headers;
}

module.exports = { securityHeaders };
