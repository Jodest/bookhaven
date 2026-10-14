import { api } from '@bookhaven/api-client';

export const adminCollectionsApi = {
  list: api.admin.collections.list,
  get: api.admin.collections.get,
  create: api.admin.collections.create,
  update: api.admin.collections.update,
  remove: api.admin.collections.remove,
  recompute: api.admin.collections.recompute,
  addBook: api.admin.collections.addBook,
  removeBook: api.admin.collections.removeBook,
  moveBook: api.admin.collections.moveBook,
  previewPaste: api.admin.collections.previewPaste,
  bulkAddBooks: api.admin.collections.bulkAddBooks,
};

/** Prefix of every admin collections query (list and detail). */
export const COLLECTIONS_QUERY_KEY = ['admin', 'collections'] as const;

export const collectionDetailKey = (id: string) => ['admin', 'collections', id] as const;
