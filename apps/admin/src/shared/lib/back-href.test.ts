import { describe, expect, it } from 'vitest';
import { buildEditHref, safeBackHref } from './back-href';

const ALLOWED = ['/catalog/books', '/moderation/books'];
const FALLBACK = '/catalog/books';

const resolve = (raw: string | null) => safeBackHref(raw, ALLOWED, FALLBACK);

describe('safeBackHref', () => {
  it('accepts an allowlisted list URL with its query', () => {
    expect(resolve('/catalog/books?status=pending&page=2')).toBe('/catalog/books?status=pending&page=2');
    expect(resolve('/moderation/books')).toBe('/moderation/books');
  });

  it('accepts the value URL-decoded from a back param', () => {
    const encoded = encodeURIComponent('/catalog/books?status=pending&page=2');
    const decoded = new URLSearchParams(`back=${encoded}`).get('back');
    expect(resolve(decoded)).toBe('/catalog/books?status=pending&page=2');
  });

  it('keeps an encoded search value', () => {
    const back = `/catalog/books?search=${encodeURIComponent('Лев Толстой')}`;
    expect(resolve(back)).toBe(back);
  });

  it('drops the hash', () => {
    expect(resolve('/catalog/books?page=2#top')).toBe('/catalog/books?page=2');
  });

  it.each([
    null,
    '',
    'https://evil.com',
    'http://local/catalog/books',
    '//evil.com',
    '//evil.com/catalog/books',
    '/\\evil.com',
    '\\\\evil.com',
    '/%5Cevil.com',
    '/catalog/books?x=%5c',
    'javascript:alert(1)',
    'catalog/books',
    '/catalog/books/../../x',
    '/catalog/books/%2e%2e/%2e%2e/login',
    '/catalog/booksX',
    '/catalog/books/',
    '/catalog/books/1/edit',
    '/catalog/authors',
    '/login',
    '/catalog/books?search=a%0ab',
    '/catalog/books?search=a%0D%0Ab',
    '/catalog/books\n',
    '/catalog/books?x=\u0000',
    '/catalog/books\t',
    '/%2F%2Fevil.com',
  ])('rejects %j', (raw) => {
    expect(resolve(raw)).toBe(FALLBACK);
  });
});

describe('buildEditHref', () => {
  it('encodes the current pathname and query as back', () => {
    const href = buildEditHref('/catalog/books/b1/edit', '/catalog/books', new URLSearchParams('status=pending&page=2'));
    expect(href).toBe(`/catalog/books/b1/edit?back=${encodeURIComponent('/catalog/books?status=pending&page=2')}`);
  });

  it('omits the ? when there is no query', () => {
    expect(buildEditHref('/catalog/books/b1/edit', '/catalog/books', new URLSearchParams())).toBe(
      `/catalog/books/b1/edit?back=${encodeURIComponent('/catalog/books')}`
    );
  });

  it('round-trips: the produced back decodes and passes validation', () => {
    const params = new URLSearchParams();
    params.set('search', 'Мастер & Маргарита');
    params.set('author_id', 'a1');
    params.set('page', '3');
    const href = buildEditHref('/catalog/books/b1/edit', '/catalog/books', params);
    const back = new URL(href, 'http://x').searchParams.get('back');
    expect(resolve(back)).toBe(`/catalog/books?${params.toString()}`);
  });
});
