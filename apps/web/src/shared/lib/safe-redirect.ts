/**
 * Validates that a redirect target is a safe, same-origin, relative path.
 *
 * Prevents open-redirect vulnerabilities by rejecting:
 * - empty/missing values
 * - control characters (tab, newline, carriage return) anywhere in the
 *   value — the WHATWG URL spec (used internally by `new URL()`, which
 *   Next.js's router relies on to resolve `router.push()` targets) strips
 *   these before parsing, so a value like `/\t/evil.com` would otherwise
 *   pass the `//` check here but still resolve to `https://evil.com`
 * - protocol-relative URLs (`//evil.com`)
 * - browser-normalized backslash variants (`/\evil.com`)
 * - absolute URLs with a scheme (`https://evil.com`, `javascript:alert(1)`)
 * - any value that does not start with a single `/`
 */
export function isSafeRedirectPath(
  value: string | null | undefined
): value is string {
  if (!value) return false;
  if (/[\t\n\r]/.test(value)) return false;
  if (!value.startsWith('/')) return false;
  if (value.startsWith('//')) return false;
  if (value.startsWith('/\\')) return false;

  return true;
}

/**
 * `/login`, carrying `redirect` along only when it is a safe path (e.g. after
 * registration, so the user lands where they started once they log in).
 */
export function loginHrefWithRedirect(redirect: string | null | undefined): string {
  return isSafeRedirectPath(redirect)
    ? `/login?${new URLSearchParams({ redirect }).toString()}`
    : '/login';
}
