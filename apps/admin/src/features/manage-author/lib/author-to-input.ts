import type { AdminAuthor, AdminAuthorInput } from '@bookhaven/api-client';

/** Initial AuthorForm state for editing an existing author. */
export function authorToInput(author: AdminAuthor): AdminAuthorInput {
  return {
    name: author.name,
    slug: author.slug,
    biography: author.biography ?? '',
    photo_url: author.photo_url ?? '',
    birth_date: author.birth_date ?? '',
    death_date: author.death_date ?? '',
    status: author.status,
  };
}
