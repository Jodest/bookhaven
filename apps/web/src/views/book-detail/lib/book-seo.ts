import type { Book } from '@bookhaven/api-client';
import type { Metadata } from 'next';

const DESCRIPTION_LIMIT = 160;

function ownDescription(book: Book): string | null {
  return book.description?.trim() ? book.description : null;
}

function truncateOnWord(text: string, limit: number): string {
  if (text.length <= limit) return text;
  const cut = text.slice(0, limit);
  const lastSpace = cut.lastIndexOf(' ');
  const base = lastSpace > limit * 0.5 ? cut.slice(0, lastSpace) : cut;
  return `${base.replace(/[\s,.;:!?-]+$/, '')}…`;
}

function metaDescription(book: Book): string {
  const own = ownDescription(book);
  if (own) return truncateOnWord(own.replace(/\s+/g, ' ').trim(), DESCRIPTION_LIMIT);
  return `${book.title}: оценки, рецензии и экранизации на BookHaven.`;
}

export function buildBookMetadata(book: Book): Metadata {
  const authors = book.authors.map((a) => a.name);
  const title =
    authors.length > 0
      ? `${book.title} — ${authors.join(', ')} — BookHaven`
      : `${book.title} — BookHaven`;
  const description = metaDescription(book);
  const hasImage = !!book.coverUrl && /^https?:\/\//.test(book.coverUrl);

  return {
    title,
    description,
    openGraph: {
      type: 'book',
      title,
      description,
      ...(book.isbn ? { isbn: book.isbn } : {}),
      ...(authors.length > 0 ? { authors } : {}),
      ...(hasImage ? { images: [{ url: book.coverUrl as string }] } : {}),
    },
  };
}

export function buildBookJsonLd(book: Book): Record<string, unknown> {
  const own = ownDescription(book);
  return {
    '@context': 'https://schema.org',
    '@type': 'Book',
    name: book.title,
    author: book.authors.map((a) => ({ '@type': 'Person', name: a.name })),
    ...(book.isbn ? { isbn: book.isbn } : {}),
    ...(book.pageCount ? { numberOfPages: book.pageCount } : {}),
    ...(book.publishedYear ? { datePublished: String(book.publishedYear) } : {}),
    ...(book.language ? { inLanguage: book.language } : {}),
    ...(book.coverUrl ? { image: book.coverUrl } : {}),
    ...(own ? { description: own } : {}),
    ...(book.ratingsCount > 0
      ? {
          aggregateRating: {
            '@type': 'AggregateRating',
            ratingValue: book.rating,
            ratingCount: book.ratingsCount,
            bestRating: 5,
            worstRating: 1,
          },
        }
      : {}),
  };
}
