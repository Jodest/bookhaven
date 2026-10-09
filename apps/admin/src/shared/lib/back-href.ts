// Return-URL handling for the catalog edit pages
// (docs/features/catalog-unified-lists.md, Business Rule 6). The `back` query
// param is untrusted input: it is only ever used after passing safeBackHref.

const LOCAL_ORIGIN = 'http://local';
// ASCII control characters (raw).
const CONTROL_CHARS = /[\x00-\x1F\x7F]/;
// Percent-encoded control characters and backslashes.
const ENCODED_UNSAFE = /%(?:[01][0-9a-f]|7f|5c)/i;

/**
 * Returns `pathname + search` of `raw` when it is a same-origin relative path
 * whose pathname is exactly one of `allowed` (no prefix matching); otherwise
 * `fallback`. The hash is dropped.
 */
export function safeBackHref(raw: string | null | undefined, allowed: string[], fallback: string): string {
  if (typeof raw !== 'string' || raw.length === 0 || raw.length > 2048) return fallback;
  // Exactly one leading slash: rejects `//host`, `/\host` and absolute URLs.
  if (!raw.startsWith('/') || raw.startsWith('//')) return fallback;
  if (raw.includes('\\') || CONTROL_CHARS.test(raw) || ENCODED_UNSAFE.test(raw)) return fallback;

  try {
    const url = new URL(raw, LOCAL_ORIGIN);
    if (url.origin !== LOCAL_ORIGIN) return fallback;
    if (!allowed.includes(url.pathname)) return fallback;
    return url.pathname + url.search;
  } catch {
    return fallback;
  }
}

/**
 * Edit-page href carrying the current list location as `back`:
 * `${base}?back=<encodeURIComponent(pathname?qs)>`.
 */
export function buildEditHref(base: string, pathname: string, searchParams: { toString(): string }): string {
  const qs = searchParams.toString();
  const current = qs ? `${pathname}?${qs}` : pathname;
  return `${base}?back=${encodeURIComponent(current)}`;
}
