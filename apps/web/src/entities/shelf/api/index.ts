import { api } from '@bookhaven/api-client';

export const shelvesApi = {
  list: api.shelves.list,
  get: api.shelves.get,
  create: api.shelves.create,
  rename: api.shelves.rename,
  remove: api.shelves.remove,
  addBook: api.shelves.addBook,
  removeBook: api.shelves.removeBook,
};
