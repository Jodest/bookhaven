import { ApiError } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import {
  collectionErrorText,
  EMPTY_COLLECTION_FORM,
  formToCreateInput,
  formToUpdateInput,
  isSlugConflict,
} from './collection-form';

describe('collectionErrorText', () => {
  it('maps 409 to the slug message', () => {
    const err = new ApiError(409, 'Conflict', 'slug already exists');
    expect(isSlugConflict(err)).toBe(true);
    expect(collectionErrorText(err)).toBe('Такой адрес уже занят');
  });

  it('passes other messages through', () => {
    expect(collectionErrorText(new ApiError(400, 'Bad Request', 'title: must be 2 to 255 characters'))).toBe(
      'title: must be 2 to 255 characters'
    );
    expect(collectionErrorText(null)).toBeUndefined();
  });
});

describe('form to input', () => {
  it('create leaves empty optional fields out', () => {
    expect(formToCreateInput({ ...EMPTY_COLLECTION_FORM, title: ' Топ ' })).toEqual({
      title: 'Топ',
      size: 100,
      is_published: false,
    });
  });

  it('update clears emptied fields', () => {
    expect(formToUpdateInput({ ...EMPTY_COLLECTION_FORM, title: 'Топ', slug: 'top' })).toEqual({
      title: 'Топ',
      slug: 'top',
      size: 100,
      description: '',
      source: '',
      source_url: '',
      genre_id: null,
      is_published: false,
    });
  });
});
