import type { AdminAuthorStatus } from '@bookhaven/api-client';

/**
 * Link to an author on the moderation page: its status tab plus a name
 * search (the queue has no per-author route).
 */
export function authorModerationHref(author: { name: string; status: AdminAuthorStatus }): string {
  return `/moderation/authors?${new URLSearchParams({ status: author.status, search: author.name })}`;
}
