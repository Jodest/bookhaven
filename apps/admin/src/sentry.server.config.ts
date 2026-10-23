import * as Sentry from '@sentry/nextjs';

import { scrubEvent } from '@/shared/lib/sentry-scrub';

Sentry.init({
  dsn: process.env.SENTRY_DSN?.trim(),
  environment: process.env.SENTRY_ENVIRONMENT?.trim() || process.env.NODE_ENV,
  release: process.env.SENTRY_RELEASE?.trim() || undefined,
  // @sentry/nextjs 11 has no sendDefaultPii option (PII collection is off by
  // default); scrubEvent additionally strips request and user data.
  maxBreadcrumbs: 0,
  beforeSend: scrubEvent,
  initialScope: { tags: { component: 'admin' } },
});
