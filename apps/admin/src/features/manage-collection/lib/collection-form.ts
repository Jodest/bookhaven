import { ApiError } from '@bookhaven/api-client';
import type { AdminCuratedCollection, AdminCuratedCollectionInput } from '@bookhaven/api-client';

/** Form state; every field is controlled, so optional ones are plain strings. */
export interface CollectionFormValues {
  title: string;
  slug: string;
  description: string;
  size: number;
  source: string;
  source_url: string;
  /** '' = no genre. */
  genre_id: string;
  is_published: boolean;
}

export const EMPTY_COLLECTION_FORM: CollectionFormValues = {
  title: '',
  slug: '',
  description: '',
  size: 100,
  source: '',
  source_url: '',
  genre_id: '',
  is_published: false,
};

export function collectionToForm(c: AdminCuratedCollection): CollectionFormValues {
  return {
    title: c.title,
    slug: c.slug,
    description: c.description,
    size: c.size,
    source: c.source ?? '',
    source_url: c.source_url ?? '',
    genre_id: c.genre?.id ?? '',
    is_published: c.is_published,
  };
}

/** Create body: empty optional fields are left out (an empty slug is derived from the title). */
export function formToCreateInput(v: CollectionFormValues): AdminCuratedCollectionInput {
  const input: AdminCuratedCollectionInput = {
    title: v.title.trim(),
    size: v.size,
    is_published: v.is_published,
  };
  if (v.slug.trim()) input.slug = v.slug.trim();
  if (v.description.trim()) input.description = v.description;
  if (v.source.trim()) input.source = v.source;
  if (v.source_url.trim()) input.source_url = v.source_url;
  if (v.genre_id) input.genre_id = v.genre_id;
  return input;
}

/** Update body: sends every field so an emptied one clears (genre_id null, '' for texts). */
export function formToUpdateInput(v: CollectionFormValues): AdminCuratedCollectionInput {
  return {
    title: v.title.trim(),
    slug: v.slug.trim(),
    size: v.size,
    description: v.description,
    source: v.source,
    source_url: v.source_url,
    genre_id: v.genre_id || null,
    is_published: v.is_published,
  };
}

/** A 409 on a collection create/update is always the slug. */
export function isSlugConflict(err: unknown): boolean {
  return err instanceof ApiError && err.status === 409;
}

/** Text of a create/update/delete error (409 slug becomes «Такой адрес уже занят»). */
export function collectionErrorText(err: unknown): string | undefined {
  if (!err) return undefined;
  if (isSlugConflict(err)) return 'Такой адрес уже занят';
  return err instanceof Error ? err.message : undefined;
}
