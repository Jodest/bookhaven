import { api } from '@bookhaven/api-client';

export const booksApi = {
  list: api.books.list,
  get: api.books.get,
};
