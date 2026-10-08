import type { AdminDashboard } from '@bookhaven/api-client';

/**
 * Items waiting for admin moderation: harvested books on review plus pending
 * authors. pending_reviews is deliberately excluded - it counts reviews that
 * are already hidden, not a work queue
 * (docs/features/admin-navigation-restructure.md, phase 5).
 */
export function pendingModerationTotal(stats: AdminDashboard): number {
  return stats.pending_import_review + stats.pending_authors;
}
