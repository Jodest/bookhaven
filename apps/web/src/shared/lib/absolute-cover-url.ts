import { getSiteUrl } from '@/shared/config/site';

/**
 * Absolute URL of a book cover for crawlers: a self-hosted cover
 * (`/covers/...`) is prefixed with the public site origin, an absolute
 * http(s) link is kept, anything else (no cover, odd value) gives null.
 */
export function absoluteCoverUrl(cover: string | null | undefined): string | null {
  if (!cover) return null;
  if (/^https?:\/\//.test(cover)) return cover;
  if (cover.startsWith('/covers/')) return `${getSiteUrl()}${cover}`;
  return null;
}
