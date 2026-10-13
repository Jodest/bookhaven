import type { AdminAuthor, AdminAuthorInput } from '@bookhaven/api-client';

/**
 * Initial AuthorForm state for editing an existing author. auto_harvest is
 * left out on purpose: the form adds it only when the admin changes the
 * checkbox (an unchanged flag is never re-sent).
 */
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
