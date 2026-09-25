import { api } from '@bookhaven/api-client';

export const adminBooksApi = {
  list: api.admin.books.list,
  get: api.admin.books.get,
  create: api.admin.books.create,
  update: api.admin.books.update,
  lookup: api.admin.books.lookup,
  import: api.admin.books.import,
};
