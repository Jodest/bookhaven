import type { ErrorEvent, EventHint } from '@sentry/nextjs';

// Pure helpers for server-side error tracking (docs/features/error-tracking.md).
// Identical copy in apps/web and apps/admin; keep both in sync.

/** Reporting is on iff SENTRY_DSN is non-empty after trimming. */
export function shouldInitSentry(env: Record<string, string | undefined>): boolean {
  return (env.SENTRY_DSN ?? '').trim() !== '';
}

const EMAIL_RE = /[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/g;
const BEARER_RE = /\bBearer\s+[A-Za-z0-9._~+/=-]+/gi;
const JWT_RE = /eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*/g;
const KV_RE = /\b(token|password|secret)=[^&\s"']+/gi;

export function redact(s: string): string {
  return s
    .replace(EMAIL_RE, '[email]')
    .replace(BEARER_RE, 'Bearer [redacted]')
    .replace(JWT_RE, '[jwt]')
    .replace(KV_RE, '$1=[redacted]');
}

function httpStatusOf(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  const e = error as { status?: unknown; statusCode?: unknown };
  for (const v of [e.status, e.statusCode]) {
    if (typeof v === 'number') return v;
  }
  return undefined;
}

/** Drops the query string and fragment (e.g. ?token= on /reset-password). */
function stripUrl(url: string): string {
  const cut = url.search(/[?#]/);
  return redact(cut === -1 ? url : url.slice(0, cut));
}

/**
 * beforeSend: drops expected 4xx errors (original exception with a numeric
 * status/statusCode in 400..499), otherwise strips request/user data, the
 * URL's query string and fragment, and redacts the message, transaction and
 * exception values. Never drops for privacy reasons.
 */
export function scrubEvent<T extends ErrorEvent>(event: T, hint?: EventHint): T | null {
  const status = httpStatusOf(hint?.originalException);
  if (status !== undefined && status >= 400 && status <= 499) return null;

  if (event.message) event.message = redact(event.message);
  if (event.transaction) event.transaction = stripUrl(event.transaction);
  for (const ex of event.exception?.values ?? []) {
    if (ex.value) ex.value = redact(ex.value);
  }
  if (event.request) {
    delete event.request.cookies;
    delete event.request.headers;
    delete event.request.data;
    delete event.request.query_string;
    if (event.request.url) event.request.url = stripUrl(event.request.url);
  }
  delete event.user;
  return event;
}
