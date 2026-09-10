/**
 * Validates that a redirect value is a safe, same-origin internal path.
 *
 * Rules:
 * - Must be a non-empty string.
 * - Must not exceed 2048 characters.
 * - Must start with a single `/` and not `//`.
 * - Must not contain backslashes (decoded or percent-encoded `%5C`/`%5c`).
 * - Must not contain control characters or whitespace.
 * - When resolved against `baseOrigin`, the resulting URL must share that origin.
 *
 * Callers should pass `window.location.origin` (or another trusted origin).
 */
export function isSafeInternalRedirect(
  redirect: string | null | undefined,
  baseOrigin: string
): boolean {
  if (typeof redirect !== 'string') return false;
  if (redirect.length === 0 || redirect.length > 2048) return false;
  if (!redirect.startsWith('/')) return false;
  if (redirect.startsWith('//')) return false;

  // Reject backslashes, including encoded forms that could decode to a backslash.
  if (redirect.includes('\\') || /%5[cC]/.test(redirect)) return false;

  // Reject control characters and whitespace.
  if (/[\x00-\x1F\x7F\s]/.test(redirect)) return false;

  try {
    const resolved = new URL(redirect, baseOrigin);
    if (resolved.origin !== baseOrigin) return false;
    if (!resolved.pathname.startsWith('/')) return false;
    return true;
  } catch {
    return false;
  }
}

/**
 * Sanitizes a redirect value, returning the value itself if safe or the
 * fallback path otherwise.
 */
export function sanitizeRedirect(
  redirect: string | null | undefined,
  baseOrigin: string,
  fallback: string = '/'
): string {
  return isSafeInternalRedirect(redirect, baseOrigin) ? redirect! : fallback;
}
