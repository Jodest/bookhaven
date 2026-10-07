import { describe, expect, it } from 'vitest';
import { isWikimediaUrl } from './is-wikimedia-url';

describe('isWikimediaUrl', () => {
  it('accepts https Wikimedia thumbnail hosts', () => {
    expect(isWikimediaUrl('https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/X.jpg/500px-X.jpg')).toBe(true);
    expect(isWikimediaUrl('https://thumb.wikimedia.org/wikipedia/commons/thumb/a/ab/X.jpg/500px-X.jpg')).toBe(true);
  });

  it('rejects other hosts, http and invalid values', () => {
    expect(isWikimediaUrl('https://covers.openlibrary.org/b/id/1-L.jpg')).toBe(false);
    expect(isWikimediaUrl('http://upload.wikimedia.org/x.jpg')).toBe(false);
    expect(isWikimediaUrl('not a url')).toBe(false);
    expect(isWikimediaUrl(null)).toBe(false);
  });
});
