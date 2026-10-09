import type { AdminGenre, AdminGenreInput } from '@bookhaven/api-client';

/** Initial GenreForm state for editing an existing genre. */
export function genreToInput(genre: AdminGenre): AdminGenreInput {
  return {
    name: genre.name,
    slug: genre.slug,
    status: genre.status,
    wikidata_qid: genre.wikidata_qid ?? '',
  };
}
