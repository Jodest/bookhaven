import type { AdminGenreStatus } from '@bookhaven/api-client';

/**
 * Link to a genre on the moderation page: its status tab plus a name search
 * (the queue has no per-genre route).
 */
export function genreModerationHref(genre: { name: string; status: AdminGenreStatus }): string {
  return `/moderation/genres?${new URLSearchParams({ status: genre.status, search: genre.name })}`;
}
