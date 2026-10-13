import type { AdminBookQueueItem } from '@bookhaven/api-client';

/** Minimum confidence of a «confident publish» verdict. */
export const CONFIDENT_PUBLISH_THRESHOLD = 0.9;

/**
 * Ids of the page rows the admin may select in one click as «уверенные
 * «Опубликовать»»: a fresh publish verdict with confidence >= 0.9 and no
 * duplicate hint. Selection only: publishing still needs the bulk action
 * and its confirmation.
 */
export function confidentPublishIds(items: AdminBookQueueItem[]): string[] {
  return items
    .filter(
      (item) =>
        item.ai_verdict?.verdict === 'publish' &&
        !item.ai_verdict.stale &&
        item.ai_verdict.confidence >= CONFIDENT_PUBLISH_THRESHOLD &&
        item.similar.length === 0
    )
    .map((item) => item.book.id);
}
