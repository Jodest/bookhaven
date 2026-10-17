import { describe, expect, it } from 'vitest';
import { buildCatalogPageHref } from './page-href';

describe('buildCatalogPageHref', () => {
  it('preserves repeated genre_id, author_name and sort', () => {
    const params = new URLSearchParams(
      'genre_id=a&genre_id=b&author_name=%D0%A2%D0%BE%D0%BB%D1%81%D1%82%D0%BE%D0%B9&sort=title&page=2'
    );
    const href = buildCatalogPageHref('/books', params, 3);
    const out = new URL(href, 'http://x');
    expect(out.pathname).toBe('/books');
    expect(out.searchParams.getAll('genre_id')).toEqual(['a', 'b']);
    expect(out.searchParams.get('author_name')).toBe('Толстой');
    expect(out.searchParams.get('sort')).toBe('title');
    expect(out.searchParams.get('page')).toBe('3');
  });

  it('removes page for page 1', () => {
    expect(buildCatalogPageHref('/books', new URLSearchParams('page=4&sort=new'), 1)).toBe(
      '/books?sort=new'
    );
    expect(buildCatalogPageHref('/books', new URLSearchParams('page=4'), 1)).toBe('/books');
  });
});
