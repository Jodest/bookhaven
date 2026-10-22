const DEFAULT_SITE_URL = 'http://localhost:3000';

/**
 * Public origin of the site, server-side only (no NEXT_PUBLIC_ prefix). Set as
 * SITE_URL in production; used to make self-hosted `/covers/...` links absolute
 * in Open Graph and structured data.
 */
export function getSiteUrl(): string {
  const raw = process.env.SITE_URL?.trim();
  return (raw || DEFAULT_SITE_URL).replace(/\/+$/, '');
}
