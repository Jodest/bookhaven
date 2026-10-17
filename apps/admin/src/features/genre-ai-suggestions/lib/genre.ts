import type { AdminGenre, AdminGenreAISuggestionGenre } from '@bookhaven/api-client';

/**
 * A genre of a suggestion as the AdminGenre the merge preview and the genre
 * search work with. Only open, non-stale suggestions reach the dialogs, so
 * every genre exists and has a real status.
 */
export function toAdminGenre(genre: AdminGenreAISuggestionGenre): AdminGenre {
  return {
    id: genre.id,
    name: genre.name,
    slug: '',
    status: genre.status === '' ? 'pending' : genre.status,
    wikidata_qid: genre.wikidata_qid,
  };
}
