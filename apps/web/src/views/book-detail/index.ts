import { getBook } from './api/get-book';
import { buildBookMetadata } from './lib/book-seo';
import type { Metadata } from 'next';

export { BookDetailView } from './ui/book-detail-view';

export async function generateBookMetadata(slug: string): Promise<Metadata> {
  const book = await getBook(slug);
  if (!book) return { title: 'Книга не найдена — BookHaven' };
  return buildBookMetadata(book);
}
