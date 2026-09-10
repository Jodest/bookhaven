import { api } from '@bookhaven/api-client';

export const listsApi = {
  list: api.lists.list,
  get: api.lists.get,
  getPublic: api.lists.getPublic,
  create: api.lists.create,
  update: api.lists.update,
  remove: api.lists.remove,
  addBook: api.lists.addBook,
  removeBook: api.lists.removeBook,
};
