export const shelfKeys = {
  all: ['shelves'] as const,
  user: (userId?: string) => [...shelfKeys.all, userId] as const,
  list: (userId?: string) => [...shelfKeys.user(userId), 'list'] as const,
  forBook: (userId: string | undefined, bookId: string) =>
    [...shelfKeys.user(userId), 'book', bookId] as const,
  readsForBook: (userId: string | undefined, bookId: string) =>
    [...shelfKeys.user(userId), 'reads', bookId] as const,
  detail: (userId: string | undefined, shelfId: string) =>
    [...shelfKeys.user(userId), 'detail', shelfId] as const,
  detailPage: (userId: string | undefined, shelfId: string, page: number, limit: number) =>
    [...shelfKeys.detail(userId, shelfId), { page, limit }] as const,
};
