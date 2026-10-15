import type { PasteBook, PasteLine } from '@bookhaven/api-client';

export function pasteBook(id: string, over: Partial<PasteBook> = {}): PasteBook {
  return {
    id,
    title: `Книга ${id}`,
    slug: id,
    status: 'published',
    cover_url: null,
    published_year: null,
    authors: [{ id: `a-${id}`, name: `Автор ${id}` }],
    in_collection: false,
    ...over,
  };
}

export function pasteLine(line: number, status: PasteLine['status'], over: Partial<PasteLine> = {}): PasteLine {
  return {
    line,
    text: `строка ${line}`,
    title: `строка ${line}`,
    author: null,
    status,
    book: null,
    candidates: [],
    duplicate_of: null,
    ...over,
  };
}
