import { shouldInitSentry } from '@/shared/lib/sentry-scrub';

// Server-side error tracking (docs/features/error-tracking.md). Node.js
// runtime only, and only when SENTRY_DSN is set at runtime.
export async function register() {
  if (process.env.NEXT_RUNTIME === 'nodejs' && shouldInitSentry(process.env)) {
    await import('./sentry.server.config');
  }
}
