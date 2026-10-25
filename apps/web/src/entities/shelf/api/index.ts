import { api } from '@bookhaven/api-client';

export const readsApi = api.reads;

export const shelvesApi = {
  list: api.shelves.list,
  get: api.shelves.get,
  getPublic: api.shelves.getPublic,
  create: api.shelves.create,
  update: api.shelves.update,
  remove: api.shelves.remove,
  addBook: api.shelves.addBook,
  removeBook: api.shelves.removeBook,
};
