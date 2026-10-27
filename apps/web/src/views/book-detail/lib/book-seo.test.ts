import type { Book } from '@bookhaven/api-client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { buildBookJsonLd, buildBookMetadata } from './book-seo';

const author = {
  biography: null,
  photoUrl: null,
  birthDate: null,
  deathDate: null,
  createdAt: '',
  updatedAt: '',
};

const book: Book = {
  id: 'b1',
  title: 'Отцы и дети',
  slug: 'fathers-and-sons',
  description: null,
  coverUrl: null,
  isbn: null,
  publishedYear: 1862,
  pageCount: null,
  language: 'ru',
  rating: 4.25,
  ratingsCount: 0,
  reviewsCount: 0,
  authors: [{ ...author, id: 'a1', name: 'Иван Тургенев', slug: 'ivan-turgenev' }],
  genres: [],
  createdAt: '',
  updatedAt: '',
};

describe('buildBookMetadata', () => {
  it('canonical address is the book page without ?tab=', () => {
    vi.stubEnv('SITE_URL', 'https://bookhaven.example');
    expect(buildBookMetadata(book).alternates?.canonical).toBe(
      'https://bookhaven.example/books/fathers-and-sons'
    );
    vi.unstubAllEnvs();
  });

  it('title with and without authors', () => {
    expect(buildBookMetadata(book).title).toBe('Отцы и дети — Иван Тургенев — BookHaven');
    expect(buildBookMetadata({ ...book, authors: [] }).title).toBe('Отцы и дети — BookHaven');
  });

  it('cuts a long description at about 160 chars on a word boundary', () => {
    const long = 'слово '.repeat(60);
    const description = buildBookMetadata({ ...book, description: long }).description as string;
    expect(description.length).toBeLessThanOrEqual(161);
    expect(description.endsWith('…')).toBe(true);
    expect(description.endsWith(' …')).toBe(false);
  });

  it('keeps a short description and collapses whitespace', () => {
    expect(buildBookMetadata({ ...book, description: 'Первая\n\nвторая' }).description).toBe(
      'Первая вторая'
    );
  });

  it('falls back to a generic text when only an AI description exists', () => {
    const meta = buildBookMetadata({
      ...book,
      aiDescription: { text: 'Текст ИИ', generatedAt: '' },
    });
    expect(meta.description).toBe('Отцы и дети: оценки, рецензии и экранизации на BookHaven.');
  });

  it('adds an og image only for an absolute cover url', () => {
    expect(buildBookMetadata({ ...book, coverUrl: 'https://x.test/c.jpg' }).openGraph).toMatchObject({
      images: [{ url: 'https://x.test/c.jpg' }],
    });
    expect(
      (buildBookMetadata({ ...book, coverUrl: '/other/c.jpg' }).openGraph as { images?: unknown }).images
    ).toBeUndefined();
    expect((buildBookMetadata(book).openGraph as { images?: unknown }).images).toBeUndefined();
  });

  describe('self-hosted cover', () => {
    afterEach(() => {
      vi.unstubAllEnvs();
    });

    it('makes the og image absolute with SITE_URL', () => {
      vi.stubEnv('SITE_URL', 'https://bookhaven.example');
      const meta = buildBookMetadata({ ...book, coverUrl: '/covers/ab/abc.jpg' });
      expect(meta.openGraph).toMatchObject({
        images: [{ url: 'https://bookhaven.example/covers/ab/abc.jpg' }],
      });
    });

    it('uses the same absolute url in the structured data', () => {
      vi.stubEnv('SITE_URL', 'https://bookhaven.example');
      const ld = buildBookJsonLd({ ...book, coverUrl: '/covers/ab/abc.jpg' });
      expect(ld.image).toBe('https://bookhaven.example/covers/ab/abc.jpg');
    });
  });
});

describe('buildBookJsonLd', () => {
  it('is a Book with authors and no aggregateRating without ratings', () => {
    const ld = buildBookJsonLd(book);
    expect(ld['@type']).toBe('Book');
    expect(ld.author).toEqual([{ '@type': 'Person', name: 'Иван Тургенев' }]);
    expect(ld.datePublished).toBe('1862');
    expect(ld).not.toHaveProperty('aggregateRating');
  });

  it('adds aggregateRating when there are ratings', () => {
    const ld = buildBookJsonLd({ ...book, ratingsCount: 3 });
    expect(ld.aggregateRating).toMatchObject({ ratingValue: 4.25, ratingCount: 3, bestRating: 5 });
  });

  it('keeps an absolute cover and omits the image without a cover', () => {
    expect(buildBookJsonLd({ ...book, coverUrl: 'https://x.test/c.jpg' }).image).toBe('https://x.test/c.jpg');
    expect(buildBookJsonLd(book)).not.toHaveProperty('image');
  });

  it('never takes the description from aiDescription', () => {
    const ld = buildBookJsonLd({ ...book, aiDescription: { text: 'Текст ИИ', generatedAt: '' } });
    expect(ld).not.toHaveProperty('description');
  });
});
